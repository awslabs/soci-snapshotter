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
	golog "log"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/awslabs/soci-snapshotter/config"
	bf "github.com/awslabs/soci-snapshotter/fs/backgroundfetcher"
	"github.com/awslabs/soci-snapshotter/fs/layer"
	m "github.com/awslabs/soci-snapshotter/fs/metrics"
	commonmetrics "github.com/awslabs/soci-snapshotter/fs/metrics/common"
	layermetrics "github.com/awslabs/soci-snapshotter/fs/metrics/layer"
	"github.com/awslabs/soci-snapshotter/fs/remote"
	"github.com/awslabs/soci-snapshotter/fs/source"
	"github.com/awslabs/soci-snapshotter/metadata"
	"github.com/awslabs/soci-snapshotter/snapshot"
	"github.com/awslabs/soci-snapshotter/soci"
	"github.com/awslabs/soci-snapshotter/soci/store"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/mount"
	ctdsnapshotters "github.com/containerd/containerd/pkg/snapshotters"
	"github.com/containerd/containerd/reference"
	"github.com/containerd/containerd/remotes/docker"
	metrics "github.com/docker/go-metrics"
	fusefs "github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/sirupsen/logrus"
)

var (
	defaultIndexSelectionPolicy = SelectFirstPolicy
	fusermountBin               = "fusermount"
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

func NewFilesystem(ctx context.Context, root string, cfg config.FSConfig, opts ...Option) (_ snapshot.FileSystem, err error) {
	var fsOpts options
	for _, o := range opts {
		o(&fsOpts)
	}

	var (
		mountTimeout                = time.Duration(cfg.MountTimeoutSec) * time.Second
		fuseMetricsEmitWaitDuration = time.Duration(cfg.FuseMetricsEmitWaitDurationSec) * time.Second
		attrTimeout                 = time.Duration(cfg.FuseConfig.AttrTimeout) * time.Second
		entryTimeout                = time.Duration(cfg.FuseConfig.EntryTimeout) * time.Second
		negativeTimeout             = time.Duration(cfg.FuseConfig.NegativeTimeout) * time.Second
		bgFetchPeriod               = time.Duration(cfg.BackgroundFetchConfig.FetchPeriodMsec) * time.Millisecond
		bgSilencePeriod             = time.Duration(cfg.BackgroundFetchConfig.SilencePeriodMsec) * time.Millisecond
		bgEmitMetricPeriod          = time.Duration(cfg.BackgroundFetchConfig.EmitMetricPeriodSec) * time.Second
		bgMaxQueueSize              = cfg.BackgroundFetchConfig.MaxQueueSize
	)

	metadataStore := fsOpts.metadataStore

	getSources := fsOpts.getSources
	if getSources == nil {
		getSources = source.FromDefaultLabels(func(refspec reference.Spec) (hosts []docker.RegistryHost, _ error) {
			return docker.ConfigureDefaultRegistries(docker.WithPlainHTTP(docker.MatchLocalhost))(refspec.Hostname())
		})
	}
	ctx, store, err := store.NewContentStore(ctx, store.WithType(store.ContentStoreType(cfg.ContentStoreConfig.Type)), store.WithNamespace(cfg.ContentStoreConfig.Namespace))
	if err != nil {
		return nil, fmt.Errorf("cannot create content store: %w", err)
	}

	var bgFetcher *bf.BackgroundFetcher

	if !cfg.BackgroundFetchConfig.Disable {
		log.G(context.Background()).WithFields(logrus.Fields{
			"fetchPeriod":      bgFetchPeriod,
			"silencePeriod":    bgSilencePeriod,
			"maxQueueSize":     bgMaxQueueSize,
			"emitMetricPeriod": bgEmitMetricPeriod,
		}).Info("constructing background fetcher")

		bgFetcher, err = bf.NewBackgroundFetcher(bf.WithFetchPeriod(bgFetchPeriod),
			bf.WithSilencePeriod(bgSilencePeriod),
			bf.WithMaxQueueSize(bgMaxQueueSize),
			bf.WithEmitMetricPeriod(bgEmitMetricPeriod))

		if err != nil {
			return nil, fmt.Errorf("cannot create background fetcher: %w", err)
		}
		go bgFetcher.Run(context.Background())
	} else {
		log.G(context.Background()).Info("background fetch is disabled")
	}

	r, err := layer.NewResolver(root, cfg, fsOpts.resolveHandlers, metadataStore, store, fsOpts.overlayOpaqueType, bgFetcher)
	if err != nil {
		return nil, fmt.Errorf("failed to setup resolver: %w", err)
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

	fob := m.NewFuseObservabilityManager(cfg.LogFuseOperations, fuseMetricsEmitWaitDuration)
	go fob.GlobalMonitor.RunFuseFailureListener(ctx)

	return &filesystem{
		// it's generally considered bad practice to store a context in a struct,
		// however `filesystem` has it's own lifecycle as well as a per-request lifecycle.
		// Some operations (e.g. remote calls) exist within a per-request lifecycle and use
		// the context passed to the specific function, but some operations (e.g. fuse operation counts)
		// are tied to the lifecycle of the filesystem itself. In order to avoid leaking goroutines,
		// we store the snapshotter's lifecycle in the struct itself so that we can tie new goroutines
		// to it later.
		ctx:                      ctx,
		resolver:                 r,
		getSources:               getSources,
		debug:                    cfg.Debug,
		layer:                    make(map[string]layer.Layer),
		allowNoVerification:      cfg.AllowNoVerification,
		disableVerification:      cfg.DisableVerification,
		metricsController:        c,
		attrTimeout:              attrTimeout,
		entryTimeout:             entryTimeout,
		negativeTimeout:          negativeTimeout,
		httpConfig:               cfg.RetryableHTTPClientConfig,
		contentStore:             store,
		bgFetcher:                bgFetcher,
		mountTimeout:             mountTimeout,
		fuseObservabilityManager: fob,
	}, nil
}

type sociContext struct {
	cachedErr            error
	cachedErrMu          sync.RWMutex
	bgFetchPauseOnce     sync.Once
	fetchOnce            sync.Once
	sociIndex            *soci.Index
	imageLayerToSociDesc map[string]ocispec.Descriptor
}

func (c *sociContext) Init(fsCtx context.Context, ctx context.Context, imageRef, indexDigest, imageManifestDigest string, store store.Store, fob *m.FuseObservabilityManager, httpConfig config.RetryableHTTPClientConfig) error {
	var retErr error
	c.fetchOnce.Do(func() {
		defer func() {
			if retErr != nil {
				c.cachedErrMu.Lock()
				c.cachedErr = retErr
				c.cachedErrMu.Unlock()
			}
		}()

		refspec, err := reference.Parse(imageRef)
		if err != nil {
			retErr = err
			return
		}

		remoteStore, err := newRemoteStore(refspec, httpConfig)
		if err != nil {
			retErr = err
			return
		}

		client := NewOCIArtifactClient(remoteStore)
		indexDesc := ocispec.Descriptor{
			Digest: digest.Digest(indexDigest),
		}

		if indexDigest == "" {
			log.G(ctx).Info("index digest not provided, making a Referrers API call to fetch list of indices")
			imgDigest, err := digest.Parse(imageManifestDigest)
			if err != nil {
				retErr = fmt.Errorf("unable to parse image digest: %w", err)
			}

			desc, err := client.SelectReferrer(ctx, ocispec.Descriptor{Digest: imgDigest}, defaultIndexSelectionPolicy)
			if err != nil {
				retErr = fmt.Errorf("cannot fetch list of referrers: %w", err)
				return
			}
			indexDesc = desc
		}

		log.G(ctx).WithField("digest", indexDesc.Digest.String()).Infof("fetching SOCI artifacts using index descriptor")

		index, err := FetchSociArtifacts(fsCtx, refspec, indexDesc, store, remoteStore)
		if err != nil {
			retErr = fmt.Errorf("error trying to fetch SOCI artifacts: %w", err)
			return
		}
		c.sociIndex = index
		c.populateImageLayerToSociMapping(index)

		go fob.ImageMonitor.InitOpCounter(ctx, digest.Digest(imageManifestDigest))

	})
	c.cachedErrMu.RLock()
	retErr = c.cachedErr
	c.cachedErrMu.RUnlock()
	return retErr
}

func (c *sociContext) populateImageLayerToSociMapping(sociIndex *soci.Index) {
	c.imageLayerToSociDesc = make(map[string]ocispec.Descriptor, len(sociIndex.Blobs))
	for _, desc := range sociIndex.Blobs {
		ociDigest := desc.Annotations[soci.IndexAnnotationImageLayerDigest]
		c.imageLayerToSociDesc[ociDigest] = desc
	}
}

type filesystem struct {
	ctx                      context.Context
	resolver                 *layer.Resolver
	debug                    bool
	layer                    map[string]layer.Layer
	layerMu                  sync.Mutex
	allowNoVerification      bool
	disableVerification      bool
	getSources               source.GetSources
	metricsController        *layermetrics.Controller
	attrTimeout              time.Duration
	entryTimeout             time.Duration
	negativeTimeout          time.Duration
	httpConfig               config.RetryableHTTPClientConfig
	sociContexts             sync.Map
	contentStore             store.Store
	bgFetcher                *bf.BackgroundFetcher
	mountTimeout             time.Duration
	fuseObservabilityManager *m.FuseObservabilityManager
}

func (fs *filesystem) MountLocal(ctx context.Context, mountpoint string, labels map[string]string, mounts []mount.Mount) error {
	imageRef, ok := labels[ctdsnapshotters.TargetRefLabel]
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
	remoteStore, err := newRemoteStore(refspec, fs.httpConfig)
	if err != nil {
		return fmt.Errorf("cannot create remote store: %w", err)
	}
	fetcher, err := newArtifactFetcher(refspec, fs.contentStore, remoteStore)
	if err != nil {
		return fmt.Errorf("cannot create fetcher: %w", err)
	}
	unpacker := NewLayerUnpacker(fetcher, archive)
	desc := s.Target
	err = unpacker.Unpack(ctx, desc, mountpoint, mounts)
	if err != nil {
		return fmt.Errorf("cannot unpack the layer: %w", err)
	}

	return nil
}

func (fs *filesystem) getSociContext(ctx context.Context, imageRef, indexDigest, imageManifestDigest string) (*sociContext, error) {
	cAny, _ := fs.sociContexts.LoadOrStore(imageManifestDigest, &sociContext{})
	c, ok := cAny.(*sociContext)
	if !ok {
		return nil, fmt.Errorf("could not load index: fs soci context is invalid type for %s", indexDigest)
	}
	err := c.Init(fs.ctx, ctx, imageRef, indexDigest, imageManifestDigest, fs.contentStore, fs.fuseObservabilityManager, fs.httpConfig)
	return c, err
}

func (fs *filesystem) Mount(ctx context.Context, mountpoint string, labels map[string]string) (retErr error) {
	// Setting the start time to measure the Mount operation duration.
	start := time.Now()
	ctx = log.WithLogger(ctx, log.G(ctx).WithField("mountpoint", mountpoint))

	// If this is empty or the label doesn't exist, then we will use the referrers API later
	// to get find an index digest.
	sociIndexDigest := labels[source.TargetSociIndexDigestLabel]
	imageRef, ok := labels[ctdsnapshotters.TargetRefLabel]
	if !ok {
		return fmt.Errorf("unable to get image ref from labels")
	}
	imgDigest, ok := labels[ctdsnapshotters.TargetManifestDigestLabel]
	if !ok {
		return fmt.Errorf("unable to get image digest from labels")
	}

	c, err := fs.getSociContext(ctx, imageRef, sociIndexDigest, imgDigest)
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
		var rErr error
		for _, s := range src {
			sociDesc, ok := c.imageLayerToSociDesc[s.Target.Digest.String()]
			if !ok {
				log.G(ctx).WithFields(logrus.Fields{
					"layerDigest": s.Target.Digest.String(),
					"image":       s.Name.String(),
				}).Infof("skipping mounting layer as FUSE mount: %v", snapshot.ErrNoZtoc)
				rErr = fmt.Errorf("skipping mounting layer %s as FUSE mount: %w", s.Target.Digest.String(), snapshot.ErrNoZtoc)
				break
			}

			l, err := fs.resolver.Resolve(ctx, s.Hosts, s.Name, s.Target, sociDesc, fs.fuseObservabilityManager, digest.Digest(imgDigest), fs.disableVerification)
			if err == nil {
				resultChan <- l
				return
			}
			rErr = fmt.Errorf("failed to resolve layer %q from %q: %w", s.Target.Digest, s.Name, err)
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
			sociDesc, ok := c.imageLayerToSociDesc[desc.Digest.String()]
			if !ok {
				log.G(ctx).WithError(snapshot.ErrNoZtoc).WithField("layerDigest", desc.Digest.String()).Debug("skipping layer pre-resolve")
				return
			}

			l, err := fs.resolver.Resolve(ctx, preResolve.Hosts, preResolve.Name, desc, sociDesc, fs.fuseObservabilityManager, digest.Digest(imgDigest), fs.disableVerification)
			if err != nil {
				log.G(ctx).WithError(err).Debug("failed to pre-resolve")
				return
			}
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
		retErr = err
		return
	case <-time.After(fs.mountTimeout):
		log.G(ctx).WithFields(logrus.Fields{
			"timeout":     fs.mountTimeout.String(),
			"layerDigest": labels[ctdsnapshotters.TargetLayerDigestLabel],
		}).Info("timeout waiting for layer to resolve")
		retErr = fmt.Errorf("timeout waiting for layer %s to resolve", labels[ctdsnapshotters.TargetLayerDigestLabel])
		return
	}
	defer func() {
		if retErr != nil {
			l.Done() // don't use this layer.
		}
	}()

	// Verification is needed to instantiate reader
	l.SkipVerify()
	log.G(ctx).Infof("Verification forcefully skipped")
	// Maybe we should reword the log here or remove it entirely,
	// since the old Verify() function no longer serves any purpose.

	node, err := l.RootNode(0)
	if err != nil {
		log.G(ctx).WithError(err).Warnf("Failed to get root node")
		retErr = fmt.Errorf("failed to get root node: %w", err)
		return
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
		NegativeTimeout: &fs.negativeTimeout,
		NullPermissions: true,
	})
	// Pass in a logger to go-fuse with the layer digest
	// The go-fuse logs are useful for tracing exactly what's happening at the fuse level.
	logger := log.L.
		WithField("layerDigest", labels[ctdsnapshotters.TargetLayerDigestLabel]).
		WriterLevel(logrus.TraceLevel)
	mountOpts := &fuse.MountOptions{
		AllowOther: true,   // allow users other than root&mounter to access fs
		FsName:     "soci", // name this filesystem as "soci"
		Debug:      fs.debug,
		Logger:     golog.New(logger, "", 0),
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
		retErr = err
		return
	}

	go server.Serve()

	// Send a signal to the background fetcher that a new image is being mounted
	// and to pause all background fetches.
	c.bgFetchPauseOnce.Do(func() {
		if fs.bgFetcher != nil {
			fs.bgFetcher.Pause()
		}
	})

	return server.WaitMount()
}

func (fs *filesystem) Check(ctx context.Context, mountpoint string, labels map[string]string) error {

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
			rErr = fmt.Errorf("failed(layer:%q, ref:%q): %v: %w",
				s.Target.Digest, s.Name, err, rErr)
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

// neighboringLayers returns layer descriptors except the `target` layer in the specified manifest.
func neighboringLayers(manifest ocispec.Manifest, target ocispec.Descriptor) (descs []ocispec.Descriptor) {
	for _, desc := range manifest.Layers {
		if desc.Digest.String() != target.Digest.String() {
			descs = append(descs, desc)
		}
	}
	return
}
