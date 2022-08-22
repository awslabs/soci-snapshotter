/*
   Copyright The Soci Snapshotter Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

/*
   Copyright 2019 The Go Authors. All rights reserved.
   Use of this source code is governed by a BSD-style
   license that can be found in the NOTICE.md file.
*/

//
// Implementation of FileSystem of SOCI snapshotter
//

package fs

import (
	"context"
	"fmt"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/awslabs/soci-snapshotter/fs/config"
	"github.com/awslabs/soci-snapshotter/fs/layer"
	commonmetrics "github.com/awslabs/soci-snapshotter/fs/metrics/common"
	layermetrics "github.com/awslabs/soci-snapshotter/fs/metrics/layer"
	"github.com/awslabs/soci-snapshotter/fs/remote"
	"github.com/awslabs/soci-snapshotter/fs/source"
	"github.com/awslabs/soci-snapshotter/metadata"
	"github.com/awslabs/soci-snapshotter/snapshot"
	"github.com/awslabs/soci-snapshotter/soci"
	"github.com/awslabs/soci-snapshotter/task"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/reference"
	"github.com/containerd/containerd/remotes/docker"
	metrics "github.com/docker/go-metrics"
	fusefs "github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	orascontent "oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/content/oci"
	"oras.land/oras-go/v2/errdef"
)

const (
	defaultFuseTimeout    = time.Second
	defaultMaxConcurrency = 2
	fusermountBin         = "fusermount"
)

type Option func(*options)

type options struct {
	getSources        source.GetSources
	resolveHandlers   map[string]remote.Handler
	metadataStore     metadata.Store
	overlayOpaqueType layer.OverlayOpaqueType
}

func WithGetSources(s source.GetSources) Option {
	return func(opts *options) {
		opts.getSources = s
	}
}

func WithResolveHandler(name string, handler remote.Handler) Option {
	return func(opts *options) {
		if opts.resolveHandlers == nil {
			opts.resolveHandlers = make(map[string]remote.Handler)
		}
		opts.resolveHandlers[name] = handler
	}
}

func WithMetadataStore(metadataStore metadata.Store) Option {
	return func(opts *options) {
		opts.metadataStore = metadataStore
	}
}

func WithOverlayOpaqueType(overlayOpaqueType layer.OverlayOpaqueType) Option {
	return func(opts *options) {
		opts.overlayOpaqueType = overlayOpaqueType
	}
}

func NewFilesystem(root string, cfg config.Config, opts ...Option) (_ snapshot.FileSystem, err error) {
	var fsOpts options
	for _, o := range opts {
		o(&fsOpts)
	}
	maxConcurrency := cfg.MaxConcurrency
	if maxConcurrency == 0 {
		maxConcurrency = defaultMaxConcurrency
	}

	attrTimeout := time.Duration(cfg.FuseConfig.AttrTimeout) * time.Second
	if attrTimeout == 0 {
		attrTimeout = defaultFuseTimeout
	}

	entryTimeout := time.Duration(cfg.FuseConfig.EntryTimeout) * time.Second
	if entryTimeout == 0 {
		entryTimeout = defaultFuseTimeout
	}

	metadataStore := fsOpts.metadataStore

	getSources := fsOpts.getSources
	if getSources == nil {
		getSources = source.FromDefaultLabels(func(refspec reference.Spec) (hosts []docker.RegistryHost, _ error) {
			return docker.ConfigureDefaultRegistries(docker.WithPlainHTTP(docker.MatchLocalhost))(refspec.Hostname())
		})
	}

	store, err := oci.New(config.SociContentStorePath)
	if err != nil {
		return nil, fmt.Errorf("cannot create local store: %w", err)
	}

	tm := task.NewBackgroundTaskManager(maxConcurrency, 5*time.Second)
	r, err := layer.NewResolver(root, tm, cfg, fsOpts.resolveHandlers, metadataStore, store, fsOpts.overlayOpaqueType)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to setup resolver")
	}

	var ns *metrics.Namespace
	if !cfg.NoPrometheus {
		ns = metrics.NewNamespace("soci", "fs", nil)
		commonmetrics.Register() // Register common metrics. This will happen only once.
	}
	c := layermetrics.NewLayerMetrics(ns)
	if ns != nil {
		metrics.Register(ns) // Register layer metrics.
	}
	return &filesystem{
		resolver:              r,
		getSources:            getSources,
		noBackgroundFetch:     cfg.NoBackgroundFetch,
		debug:                 cfg.Debug,
		layer:                 make(map[string]layer.Layer),
		backgroundTaskManager: tm,
		allowNoVerification:   cfg.AllowNoVerification,
		disableVerification:   true,
		metricsController:     c,
		attrTimeout:           attrTimeout,
		entryTimeout:          entryTimeout,
		imageLayerToSociDesc:  make(map[string]ocispec.Descriptor),
		orasStore:             store,
	}, nil
}

type filesystem struct {
	resolver              *layer.Resolver
	noBackgroundFetch     bool
	debug                 bool
	layer                 map[string]layer.Layer
	layerMu               sync.Mutex
	backgroundTaskManager *task.BackgroundTaskManager
	allowNoVerification   bool
	disableVerification   bool
	getSources            source.GetSources
	metricsController     *layermetrics.Controller
	attrTimeout           time.Duration
	entryTimeout          time.Duration
	sociIndex             *soci.SociIndex
	imageLayerToSociDesc  map[string]ocispec.Descriptor
	loadIndexOnce         sync.Once
	orasStore             orascontent.Storage
}

func (fs *filesystem) fetchSociArtifacts(ctx context.Context, imageRef, indexDigest string) error {
	var retErr error
	fs.loadIndexOnce.Do(func() {
		index, err := FetchSociArtifacts(ctx, imageRef, indexDigest, fs.orasStore)
		if err != nil && !errors.Is(err, errdef.ErrAlreadyExists) {
			retErr = fmt.Errorf("error trying to fetch SOCI artifacts: %w", err)
			return
		}
		fs.sociIndex = index
		fs.populateImageLayerToSociMapping(index)
	})
	return retErr
}

func (fs *filesystem) populateImageLayerToSociMapping(sociIndex *soci.SociIndex) {
	for _, desc := range sociIndex.Blobs {
		ociDigest := desc.Annotations[soci.IndexAnnotationImageLayerDigest]
		fs.imageLayerToSociDesc[ociDigest] = desc
	}
}

func (fs *filesystem) MountLocal(ctx context.Context, mountpoint string, labels map[string]string) error {
	imageRef, ok := labels[source.TargetRefLabel]
	if !ok {
		return fmt.Errorf("unable to get image ref from labels")
	}
	// Get source information of this layer.
	src, err := fs.getSources(labels)
	if err != nil {
		return err
	} else if len(src) == 0 {
		return fmt.Errorf("blob info not found for any labels in %s", fmt.Sprint(labels))
	}
	// download the target layer
	s := src[0]
	archive := NewLayerArchive()
	refspec, err := reference.Parse(imageRef)
	if err != nil {
		return fmt.Errorf("cannot parse image ref (%s): %w", imageRef, err)
	}
	remoteStore, err := newRemoteStore(refspec)
	if err != nil {
		return fmt.Errorf("cannot create remote store: %w", err)
	}
	fetcher, err := newArtifactFetcher(refspec, fs.orasStore, remoteStore, newResolver())
	if err != nil {
		return fmt.Errorf("cannot create fetcher: %w", err)
	}
	unpacker := NewLayerUnpacker(fetcher, archive)
	desc := s.Target
	err = unpacker.Unpack(ctx, desc, mountpoint)
	if err != nil {
		return fmt.Errorf("cannot unpack the layer: %w", err)
	}

	return nil
}

func (fs *filesystem) Mount(ctx context.Context, mountpoint string, labels map[string]string) (retErr error) {
	// Setting the start time to measure the Mount operation duration.
	start := time.Now()

	// This is a prioritized task and all background tasks will be stopped
	// execution so this can avoid being disturbed for NW traffic by background
	// tasks.
	fs.backgroundTaskManager.DoPrioritizedTask()
	defer fs.backgroundTaskManager.DonePrioritizedTask()
	ctx = log.WithLogger(ctx, log.G(ctx).WithField("mountpoint", mountpoint))

	sociIndexDigest, ok := labels[source.TargetSociIndexDigestLabel]
	if !ok {
		return fmt.Errorf("unable to get soci index digest from labels")
	}
	imageRef, ok := labels[source.TargetRefLabel]
	if !ok {
		return fmt.Errorf("unable to get image ref from labels")
	}

	err := fs.fetchSociArtifacts(ctx, imageRef, sociIndexDigest)
	if err != nil {
		return fmt.Errorf("unable to fetch SOCI artifacts: %w", err)
	}

	// Get source information of this layer.
	src, err := fs.getSources(labels)
	if err != nil {
		return err
	} else if len(src) == 0 {
		return fmt.Errorf("source must be passed")
	}

	// Resolve the target layer
	var (
		resultChan = make(chan layer.Layer)
		errChan    = make(chan error)
	)
	go func() {
		rErr := fmt.Errorf("failed to resolve target")
		for _, s := range src {
			sociDesc := ocispec.Descriptor{}
			if desc, ok := fs.imageLayerToSociDesc[s.Target.Digest.String()]; ok {
				sociDesc = desc
			}

			l, err := fs.resolver.Resolve(ctx, s.Hosts, s.Name, s.Target, sociDesc)
			if err == nil {
				resultChan <- l
				fs.backgroundFetch(ctx, l, start)
				return
			}
			rErr = errors.Wrapf(rErr, "failed to resolve layer %q from %q: %v", s.Target.Digest, s.Name, err)
		}
		errChan <- rErr
	}()

	// Also resolve and cache other layers in parallel
	preResolve := src[0] // TODO: should we pre-resolve blobs in other sources as well?
	for _, desc := range neighboringLayers(preResolve.Manifest, preResolve.Target) {
		desc := desc
		go func() {
			// Avoids to get canceled by client.
			ctx := log.WithLogger(context.Background(), log.G(ctx).WithField("mountpoint", mountpoint))
			sociDesc := ocispec.Descriptor{}
			if descriptor, ok := fs.imageLayerToSociDesc[desc.Digest.String()]; ok {
				sociDesc = descriptor
			}
			l, err := fs.resolver.Resolve(ctx, preResolve.Hosts, preResolve.Name, desc, sociDesc)
			if err != nil {
				log.G(ctx).WithError(err).Debug("failed to pre-resolve")
				return
			}
			fs.backgroundFetch(ctx, l, start)

			// Release this layer because this isn't target and we don't use it anymore here.
			// However, this will remain on the resolver cache until eviction.
			l.Done()
		}()
	}

	// Wait for resolving completion
	var l layer.Layer
	select {
	case l = <-resultChan:
	case err := <-errChan:
		log.G(ctx).WithError(err).Debug("failed to resolve layer")
		return errors.Wrapf(err, "failed to resolve layer")
	case <-time.After(30 * time.Second):
		log.G(ctx).Debug("failed to resolve layer (timeout)")
		return fmt.Errorf("failed to resolve layer (timeout)")
	}
	defer func() {
		if retErr != nil {
			l.Done() // don't use this layer.
		}
	}()

	// Verify layer's content
	if fs.disableVerification {
		// Skip if verification is disabled completely
		l.SkipVerify()
		log.G(ctx).Infof("Verification forcefully skipped")
	}

	node, err := l.RootNode(0)
	if err != nil {
		log.G(ctx).WithError(err).Warnf("Failed to get root node")
		return errors.Wrapf(err, "failed to get root node")
	}

	// Measuring duration of Mount operation for resolved layer.
	digest := l.Info().Digest // get layer sha
	defer commonmetrics.MeasureLatencyInMilliseconds(commonmetrics.Mount, digest, start)

	// Register the mountpoint layer
	fs.layerMu.Lock()
	fs.layer[mountpoint] = l
	fs.layerMu.Unlock()
	fs.metricsController.Add(mountpoint, l)

	// mount the node to the specified mountpoint
	// TODO: bind mount the state directory as a read-only fs on snapshotter's side
	rawFS := fusefs.NewNodeFS(node, &fusefs.Options{
		AttrTimeout:     &fs.attrTimeout,
		EntryTimeout:    &fs.entryTimeout,
		NullPermissions: true,
	})
	mountOpts := &fuse.MountOptions{
		AllowOther: true,   // allow users other than root&mounter to access fs
		FsName:     "soci", // name this filesystem as "soci"
		Debug:      fs.debug,
	}
	if _, err := exec.LookPath(fusermountBin); err == nil {
		mountOpts.Options = []string{"suid"} // option for fusermount; allow setuid inside container
	} else {
		log.G(ctx).WithError(err).Infof("%s not installed; trying direct mount", fusermountBin)
		mountOpts.DirectMount = true
	}
	server, err := fuse.NewServer(rawFS, mountpoint, mountOpts)
	if err != nil {
		log.G(ctx).WithError(err).Debug("failed to make filesystem server")
		return err
	}

	go server.Serve()
	return server.WaitMount()
}

func (fs *filesystem) Check(ctx context.Context, mountpoint string, labels map[string]string) error {
	// This is a prioritized task and all background tasks will be stopped
	// execution so this can avoid being disturbed for NW traffic by background
	// tasks.
	fs.backgroundTaskManager.DoPrioritizedTask()
	defer fs.backgroundTaskManager.DonePrioritizedTask()

	ctx = log.WithLogger(ctx, log.G(ctx).WithField("mountpoint", mountpoint))

	fs.layerMu.Lock()
	l := fs.layer[mountpoint]
	fs.layerMu.Unlock()
	if l == nil {
		log.G(ctx).Debug("layer not registered")
		return fmt.Errorf("layer not registered")
	}

	// Check the blob connectivity and try to refresh the connection on failure
	if err := fs.check(ctx, l, labels); err != nil {
		log.G(ctx).WithError(err).Warn("check failed")
		return err
	}

	return nil
}

func (fs *filesystem) check(ctx context.Context, l layer.Layer, labels map[string]string) error {
	err := l.Check()
	if err == nil {
		return nil
	}
	log.G(ctx).WithError(err).Warn("failed to connect to blob")

	// Check failed. Try to refresh the connection with fresh source information
	src, err := fs.getSources(labels)
	if err != nil {
		return err
	}
	var (
		retrynum = 1
		rErr     = fmt.Errorf("failed to refresh connection")
	)
	for retry := 0; retry < retrynum; retry++ {
		log.G(ctx).Warnf("refreshing(%d)...", retry)
		for _, s := range src {
			err := l.Refresh(ctx, s.Hosts, s.Name, s.Target)
			if err == nil {
				log.G(ctx).Debug("Successfully refreshed connection")
				return nil
			}
			log.G(ctx).WithError(err).Warnf("failed to refresh the layer %q from %q",
				s.Target.Digest, s.Name)
			rErr = errors.Wrapf(rErr, "failed(layer:%q, ref:%q): %v",
				s.Target.Digest, s.Name, err)
		}
	}

	return rErr
}

func (fs *filesystem) Unmount(ctx context.Context, mountpoint string) error {
	fs.layerMu.Lock()
	l, ok := fs.layer[mountpoint]
	if !ok {
		fs.layerMu.Unlock()
		return fmt.Errorf("specified path %q isn't a mountpoint", mountpoint)
	}
	delete(fs.layer, mountpoint) // unregisters the corresponding layer
	l.Done()
	fs.layerMu.Unlock()
	fs.metricsController.Remove(mountpoint)
	// The goroutine which serving the mountpoint possibly becomes not responding.
	// In case of such situations, we use MNT_FORCE here and abort the connection.
	// In the future, we might be able to consider to kill that specific hanging
	// goroutine using channel, etc.
	// See also: https://www.kernel.org/doc/html/latest/filesystems/fuse.html#aborting-a-filesystem-connection
	return syscall.Unmount(mountpoint, syscall.MNT_FORCE)
}

func (fs *filesystem) backgroundFetch(ctx context.Context, l layer.Layer, start time.Time) {
	// Fetch whole layer aggressively in background.
	if !fs.noBackgroundFetch {
		go func() {
			if err := l.BackgroundFetch(); err == nil {
				// write log record for the latency between mount start and last on demand fetch
				commonmetrics.LogLatencyForLastOnDemandFetch(ctx, l.Info().Digest, start, l.Info().ReadTime)
			}
		}()
	}
}

// neighboringLayers returns layer descriptors except the `target` layer in the specified manifest.
func neighboringLayers(manifest ocispec.Manifest, target ocispec.Descriptor) (descs []ocispec.Descriptor) {
	for _, desc := range manifest.Layers {
		if desc.Digest.String() != target.Digest.String() {
			descs = append(descs, desc)
		}
	}
	return
}
