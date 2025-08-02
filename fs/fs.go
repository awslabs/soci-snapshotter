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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	gofs "io/fs"
	golog "log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/awslabs/soci-snapshotter/config"
	bf "github.com/awslabs/soci-snapshotter/fs/backgroundfetcher"
	"github.com/awslabs/soci-snapshotter/fs/layer"
	commonmetrics "github.com/awslabs/soci-snapshotter/fs/metrics/common"
	layermetrics "github.com/awslabs/soci-snapshotter/fs/metrics/layer"
	"github.com/awslabs/soci-snapshotter/fs/remote"
	"github.com/awslabs/soci-snapshotter/fs/source"
	"github.com/awslabs/soci-snapshotter/idtools"
	"github.com/awslabs/soci-snapshotter/internal/archive/compression"
	socihttp "github.com/awslabs/soci-snapshotter/internal/http"
	"github.com/awslabs/soci-snapshotter/metadata"
	"github.com/awslabs/soci-snapshotter/service/resolver"
	"github.com/awslabs/soci-snapshotter/snapshot"
	"github.com/awslabs/soci-snapshotter/soci"
	"github.com/awslabs/soci-snapshotter/soci/store"
	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/mount"
	"github.com/containerd/containerd/namespaces"
	ctdsnapshotters "github.com/containerd/containerd/pkg/snapshotters"
	"github.com/containerd/containerd/reference"
	"github.com/containerd/containerd/remotes/docker"
	"github.com/containerd/errdefs"
	"github.com/containerd/log"
	metrics "github.com/docker/go-metrics"
	fusefs "github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/semaphore"
	orasremote "oras.land/oras-go/v2/registry/remote"
)

var (
	defaultIndexSelectionPolicy = SelectFirstPolicy
	fusermountBin               = "fusermount"
	preresolverQueueBufferSize  = 1024 // arbitrarily chosen buffer size

	ErrAllLazyPullModesDisabled = errors.New("all lazy pull modes are disabled")
)

// Preresolver will resolve a number of layers in parallel,
// up to the amount specified by MaxConcurrency.
type preresolver struct {
	queue chan func(context.Context) string
	cache *sync.Map
	smp   *semaphore.Weighted
}

func newPreresolver(maxConcurrency int64) *preresolver {
	pr := &preresolver{}
	pr.queue = make(chan func(context.Context) string, preresolverQueueBufferSize)
	pr.cache = &sync.Map{}
	if maxConcurrency > 0 {
		pr.smp = semaphore.NewWeighted(maxConcurrency)
	}
	return pr
}

func (pr *preresolver) Start(ctx context.Context) error {
	go func() {
		for {
			select {
			case <-ctx.Done():
				log.G(ctx).Info("exiting preresolver")
				return
			default:
				resolveFn := <-pr.queue
				// If concurrency limits are disabled,
				// we don't need to wait for a semaphore
				if pr.smp != nil {
					pr.smp.Acquire(ctx, 1)
				}

				go func() {
					digest := resolveFn(ctx)
					pr.cache.Delete(digest)
					if pr.smp != nil {
						pr.smp.Release(1)
					}
				}()
			}
		}
	}()

	return nil
}

func (pr *preresolver) Enqueue(imgNameAndDigest string, fn func(context.Context) string) {
	if _, ok := pr.cache.Load(imgNameAndDigest); !ok {
		select {
		case pr.queue <- fn:
			pr.cache.Store(imgNameAndDigest, struct{}{})
		default:
			return
		}
	}

}

type Option func(*options)

type options struct {
	getSources        source.GetSources
	resolveHandlers   map[string]remote.Handler
	metadataStore     metadata.Store
	overlayOpaqueType layer.OverlayOpaqueType
	maxConcurrency    int64
	pullModes         config.PullModes
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

func WithMaxConcurrency(maxConcurrency int64) Option {
	return func(opts *options) {
		opts.maxConcurrency = maxConcurrency
	}
}

func WithPullModes(pullModes config.PullModes) Option {
	return func(opts *options) {
		opts.pullModes = pullModes
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
		getSources = source.FromDefaultLabels(func(imgRefSpec reference.Spec) (hosts []docker.RegistryHost, _ error) {
			return docker.ConfigureDefaultRegistries(docker.WithPlainHTTP(docker.MatchLocalhost))(imgRefSpec.Hostname())
		})
	}

	pullModes := fsOpts.pullModes
	// disable_lazy_loading should only work for containerd content store, unless we skip content store ingestion entirely
	if pullModes.Parallel.Enable &&
		cfg.ContentStoreConfig.Type != config.ContainerdContentStoreType &&
		!pullModes.Parallel.DiscardUnpackedLayers {
		return nil, errors.New("parallel_pull_unpack mode requires containerd content store (type=\"containerd\" under [content_store])")
	}
	client := store.NewContainerdClient(cfg.ContentStoreConfig.ContainerdAddress)

	store, err := store.NewContentStore(
		store.WithType(cfg.ContentStoreConfig.Type),
		store.WithContainerdAddress(cfg.ContentStoreConfig.ContainerdAddress),
		store.WithClient(client),
	)
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

	pr := newPreresolver(fsOpts.maxConcurrency)
	pr.Start(ctx)

	var ns *metrics.Namespace
	if !cfg.NoPrometheus {
		ns = metrics.NewNamespace("soci", "fs", nil)
		commonmetrics.Register() // Register common metrics. This will happen only once.
	}
	c := layermetrics.NewLayerMetrics(ns)
	if ns != nil {
		metrics.Register(ns) // Register layer metrics.
	}

	go commonmetrics.ListenForFuseFailure(ctx)

	storage, err := newLayerUnpackDiskStorage(filepath.Dir(root))
	if err != nil {
		return nil, fmt.Errorf("error creating unpack directory on disk: %w", err)
	}

	unpackJobs, err := createParallelPullStructs(ctx, storage, &pullModes.Parallel)
	if err != nil {
		return nil, err
	}

	return &filesystem{
		// it's generally considered bad practice to store a context in a struct,
		// however `filesystem` has it's own lifecycle as well as a per-request lifecycle.
		// Some operations (e.g. remote calls) exist within a per-request lifecycle and use
		// the context passed to the specific function, but some operations (e.g. fuse operation counts)
		// are tied to the lifecycle of the filesystem itself. In order to avoid leaking goroutines,
		// we store the snapshotter's lifecycle in the struct itself so that we can tie new goroutines
		// to it later.
		ctx:                         ctx,
		resolver:                    r,
		getSources:                  getSources,
		debug:                       cfg.Debug,
		layer:                       make(map[string]layer.Layer),
		disableVerification:         cfg.DisableVerification,
		metricsController:           c,
		attrTimeout:                 attrTimeout,
		entryTimeout:                entryTimeout,
		negativeTimeout:             negativeTimeout,
		contentStore:                store,
		bgFetcher:                   bgFetcher,
		mountTimeout:                mountTimeout,
		fuseMetricsEmitWaitDuration: fuseMetricsEmitWaitDuration,
		pr:                          pr,
		pullModes:                   pullModes,
		containerd:                  client,
		inProgressImageUnpacks:      unpackJobs,
	}, nil
}

func createParallelPullStructs(ctx context.Context, storage LayerUnpackJobStorage, parallelConfig *config.Parallel) (*unpackJobs, error) {
	if !parallelConfig.Enable {
		return nil, nil
	}

	if err := compression.InitializeDecompressStreams(parallelConfig.DecompressStreams); err != nil {
		return nil, fmt.Errorf("error initializing decompress streams: %w", err)
	}

	unpackJobs, err := newUnpackJobs(ctx, parallelConfig, storage)
	if err != nil {
		return nil, fmt.Errorf("error creating unpack jobs: %w", err)
	}
	return unpackJobs, nil
}

type sociContext struct {
	cachedErr            error
	bgFetchPauseOnce     sync.Once
	fetchOnce            sync.Once
	sociIndex            *soci.Index
	imageLayerToSociDesc map[string]ocispec.Descriptor
	fuseOperationCounter *layer.FuseOperationCounter
}

func (c *sociContext) Init(ctx context.Context, fs *filesystem, imageRef, indexDigest, imageManifestDigest string, client *http.Client) error {
	c.fetchOnce.Do(func() {
		index, err := fs.fetchSociIndex(ctx, imageRef, indexDigest, imageManifestDigest, client)
		if err != nil {
			c.cachedErr = err
			return
		}
		c.sociIndex = index
		c.populateImageLayerToSociMapping(index)

		// Create the FUSE operation counter.
		// Metrics are emitted after a wait time of fuseOpEmitWaitDuration.
		c.fuseOperationCounter = layer.NewFuseOperationCounter(digest.Digest(imageManifestDigest), fs.fuseMetricsEmitWaitDuration)
		go c.fuseOperationCounter.Run(fs.ctx)
	})
	return c.cachedErr
}

func (c *sociContext) populateImageLayerToSociMapping(sociIndex *soci.Index) {
	c.imageLayerToSociDesc = make(map[string]ocispec.Descriptor, len(sociIndex.Blobs))
	for _, desc := range sociIndex.Blobs {
		ociDigest := desc.Annotations[soci.IndexAnnotationImageLayerDigest]
		c.imageLayerToSociDesc[ociDigest] = desc
	}
}

type filesystem struct {
	ctx                         context.Context
	resolver                    *layer.Resolver
	debug                       bool
	layer                       map[string]layer.Layer
	layerMu                     sync.Mutex
	disableVerification         bool
	getSources                  source.GetSources
	metricsController           *layermetrics.Controller
	attrTimeout                 time.Duration
	entryTimeout                time.Duration
	negativeTimeout             time.Duration
	sociContexts                sync.Map
	contentStore                store.Store
	bgFetcher                   *bf.BackgroundFetcher
	mountTimeout                time.Duration
	fuseMetricsEmitWaitDuration time.Duration
	pr                          *preresolver
	pullModes                   config.PullModes
	containerd                  *store.ContainerdClient
	inProgressImageUnpacks      *unpackJobs
}

func (fs *filesystem) MountParallel(ctx context.Context, mountpoint string, labels map[string]string, mounts []mount.Mount) error {
	if !fs.pullModes.Parallel.Enable {
		return ErrParallelPullIsDisabled
	}

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
	client := s.Hosts[0].Client
	refspec, err := reference.Parse(imageRef)
	if err != nil {
		return fmt.Errorf("cannot parse image ref (%s): %w", imageRef, err)
	}
	desc := s.Target
	imageDigest, ok := labels[ctdsnapshotters.TargetManifestDigestLabel]
	if !ok {
		return errors.New("layer has no image manifest attached")
	}
	// If lazy-loading is disabled and the image has no jobs associated with it, start premounting all jobs
	if !fs.inProgressImageUnpacks.ImageExists(imageDigest) {
		err := fs.preloadAllLayers(ctx, desc, imageDigest, refspec, client)
		if err != nil {
			return fmt.Errorf("failed to preload layers for image manifest digest %s: %w", imageDigest, err)
		}
	}

	err = fs.rebase(ctx, desc.Digest, imageDigest, mountpoint)
	if err != nil {
		return fmt.Errorf("failed to rebase layer %s: %w", desc.Digest, err)
	}
	return nil
}

func (fs *filesystem) preloadAllLayers(ctx context.Context, desc ocispec.Descriptor, imageDigest string, refspec reference.Spec, cachedClient *http.Client) error {
	manifest, err := fs.getImageManifest(ctx, imageDigest)
	if err != nil {
		return fmt.Errorf("cannot get image manifest: %w", err)
	}
	diffIDMap, err := fs.getDiffIDMap(ctx, manifest)
	if err != nil {
		return fmt.Errorf("error getting uncompressed shasums for image %s: %v", desc.Digest, err)
	}

	ns, ok := namespaces.Namespace(ctx)
	if !ok {
		return errors.New("namespace not attached to context")
	}
	// Clone client if it's our internal [socihttp.AuthClient]
	// so that this image pull request has an isolated client reference.
	client := cachedClient
	if authClient, ok := cachedClient.Transport.(*socihttp.AuthClient); ok {
		retryClient := resolver.CloneRetryableClient(authClient.Client())
		// The clone will have a cleaned cache
		newAuthClient := authClient.CloneWithNewClient(retryClient)
		// It's worth noting we don't ever directly clear the cache after this.
		// This client is used to create the remoteStore, which falls
		// out of scope after all layers are finished premounting,
		// which should trigger Go's garbage collector, so it should
		// be safe to never clear the cache and let Go handle it.
		newAuthClient.CacheRedirects(true)
		client = &http.Client{
			Transport: newAuthClient,
		}
	}
	remoteStore, err := newRemoteBlobStore(refspec, client)
	if err != nil {
		return fmt.Errorf("cannot create remote store: %w", err)
	}

	premountCtx, cancel := context.WithCancelCause(context.Background())
	premountCtx = namespaces.WithNamespace(premountCtx, ns)
	imageJob := fs.inProgressImageUnpacks.GetOrAddImageJob(imageDigest, cancel)

	// If we fail anywhere after making the image job, we must remove the associated image job
	premountAll := func() error {
		// We only want to premount all layers that don't exist yet.
		// Since layer order is deterministic, we can safely assume that
		// every layer after this needs to be premounted as well.
		startPremounting := false
		for _, l := range manifest.Layers {
			if images.IsLayerType(l.MediaType) {
				if l.Digest.String() == desc.Digest.String() {
					startPremounting = true

					// We don't have to preauthorize if we only do one request at a time
					if fs.inProgressImageUnpacks.imagePullCfg.MaxConcurrentDownloadsPerImage != 1 {
						err = remoteStore.doInitialFetch(ctx, constructRef(refspec, desc))
						if err != nil {
							return fmt.Errorf("error doing initial client fetch: %w", err)
						}
					}
				}
				if startPremounting {
					layerJob, err := fs.inProgressImageUnpacks.AddLayerJob(imageJob, l.Digest.String())
					if err != nil {
						return fmt.Errorf("error adding layer job: %w", err)
					}
					go fs.premount(premountCtx, l, refspec, remoteStore, diffIDMap, layerJob)
				}
			}
		}
		return nil
	}

	if err := premountAll(); err != nil {
		fs.inProgressImageUnpacks.RemoveImageWithError(imageDigest, err)
	}
	return err
}

func (fs *filesystem) premount(ctx context.Context, desc ocispec.Descriptor, refspec reference.Spec, remoteStore resolverStorage, diffIDMap map[string]digest.Digest, layerJob *layerUnpackJob) error {
	var err error
	defer func() {
		// If there is a context error (usually context cancelled),
		// rebase will not get called for this layer,
		// so we need to make sure to remove this job.
		if cErr := ctx.Err(); cErr != nil {
			err = cErr
		}
		if err != nil {
			fs.inProgressImageUnpacks.RemoveImageWithError(layerJob.imageDigest, err)
		}
		layerJob.errCh <- err
		close(layerJob.errCh)
	}()

	uncompressedDigest, ok := diffIDMap[desc.Digest.String()]
	if !ok {
		return fmt.Errorf("digest %s not found in image manifest", desc.Digest.String())
	}

	var decompressStream compression.DecompressStream
	if ds, ok := compression.GetDecompressStream(desc.MediaType); ok {
		decompressStream = ds
	}

	// If we discard unpacked layers, we must verify layer integrity ourselves.
	var compressedVerifier digest.Verifier
	if fs.pullModes.Parallel.DiscardUnpackedLayers {
		compressedVerifier = desc.Digest.Verifier()
	}

	archive := NewLayerArchive(compressedVerifier, uncompressedDigest.Verifier(), decompressStream)
	chunkSize := fs.pullModes.Parallel.ConcurrentDownloadChunkSize
	fetcher, err := newParallelArtifactFetcher(refspec, fs.contentStore, remoteStore, layerJob, chunkSize)
	if err != nil {
		log.G(ctx).WithError(err).Error("cannot create fetcher")
		return err
	}

	unpacker := NewParallelLayerUnpacker(fetcher, archive, layerJob, fs.pullModes.Parallel.DiscardUnpackedLayers)
	fsPath := layerJob.GetUnpackUpperPath()
	err = unpacker.Unpack(ctx, desc, fsPath, []mount.Mount{})
	if err != nil {
		log.G(ctx).WithError(err).WithField("digest", desc.Digest).Error("cannot unpack layer")
	}
	return err
}

func (fs *filesystem) rebase(ctx context.Context, dgst digest.Digest, imageDigest, mountpoint string) error {
	layerJob, err := fs.inProgressImageUnpacks.Claim(imageDigest, dgst.String())
	if err != nil {
		fs.inProgressImageUnpacks.RemoveImageWithError(imageDigest, err)
		return fmt.Errorf("error attempting to claim job to rebase: %w", err)
	}
	defer func() {
		if err != nil {
			layerJob.Cancel(err)
			fs.inProgressImageUnpacks.RemoveImageWithError(layerJob.imageDigest, err)
		} else {
			fs.inProgressImageUnpacks.Remove(layerJob, err)
		}
	}()

	log.G(ctx).WithField("digest", dgst).Debug("claimed layer")

	select {
	case <-ctx.Done():
		err = ctx.Err()
		return err
	case err = <-layerJob.errCh:
		if err != nil {
			return err
		}
	}

	if layerJob.status.Load() == LayerUnpackJobCancelled {
		return errors.New("layer unpack job cancelled")
	}

	tempDir := layerJob.GetUnpackUpperPath()
	if _, err = os.Stat(tempDir); err != nil {
		return fmt.Errorf("error statting temporary unpack directory %s: %w", tempDir, err)
	}

	var file os.FileInfo
	if file, err = os.Stat(mountpoint); err == nil {
		if file.IsDir() {
			// Make sure directory is empty
			f, err := os.Open(mountpoint)
			if err != nil {
				return fmt.Errorf("error opening up preexisting mountpoint %s: %w", mountpoint, err)
			}

			_, err = f.Readdirnames(1)
			if err != io.EOF {
				// If mountpoint is not empty, refuse to rebase to location
				f.Close()
				if err == nil {
					err = gofs.ErrExist
				}
				return err
			}
			f.Close()
		}
		os.RemoveAll(mountpoint)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("error statting mountpoint %s: %w", mountpoint, err)
	}

	if err = os.MkdirAll(filepath.Dir(mountpoint), 0700); err != nil {
		return fmt.Errorf("error creating mountpoint %s on disk: %w", mountpoint, err)
	}

	if err = os.Rename(tempDir, mountpoint); err != nil {
		return fmt.Errorf("error moving temp unpack dir %s to mountpoint %s: %w", tempDir, mountpoint, err)
	}

	return nil
}

func (fs *filesystem) getDiffIDMap(ctx context.Context, imageManifest *ocispec.Manifest) (map[string]digest.Digest, error) {
	client, err := fs.containerd.Client()
	if err != nil {
		return nil, err
	}

	buf, err := content.ReadBlob(ctx, client.ContentStore(), imageManifest.Config)
	if err != nil {
		return nil, err
	}

	imgConfig := ocispec.Image{}
	err = json.Unmarshal(buf, &imgConfig)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling image config JSON: %v", err)
	}

	uncompressedShas := imgConfig.RootFS.DiffIDs
	compressedShas := imageManifest.Layers
	if len(uncompressedShas) != len(compressedShas) {
		return nil, fmt.Errorf("mismatch between manifest layers and diff IDs")
	}

	diffIDMap := map[string]digest.Digest{}
	for i := range len(uncompressedShas) {
		diffIDMap[compressedShas[i].Digest.String()] = uncompressedShas[i]
	}

	return diffIDMap, nil
}

func (fs *filesystem) getImageManifest(ctx context.Context, dgst string) (*ocispec.Manifest, error) {
	client, err := fs.containerd.Client()
	if err != nil {
		return nil, err
	}

	manifestDigest, _ := digest.Parse(dgst)
	manifestDesc := ocispec.Descriptor{
		Digest: manifestDigest,
	}
	buf, err := content.ReadBlob(ctx, client.ContentStore(), manifestDesc)
	if err != nil {
		return nil, err
	}

	var manifest ocispec.Manifest
	if err := json.Unmarshal(buf, &manifest); err != nil {
		return nil, err
	}

	return &manifest, nil
}

// CleanImage stops all parallel operations for the specific image.
// Generally this will be called when removing a snapshot for an image.
func (fs *filesystem) CleanImage(ctx context.Context, imgDigest string) error {
	if !fs.pullModes.Parallel.Enable {
		return nil
	}

	err := fs.inProgressImageUnpacks.RemoveImageWithError(imgDigest, context.Canceled)
	if !errors.Is(err, ErrImageUnpackJobNotFound) && err != nil {
		return fmt.Errorf("error removing image: %w", err)
	}
	return nil
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
	client := s.Hosts[0].Client
	refspec, err := reference.Parse(imageRef)
	if err != nil {
		return fmt.Errorf("cannot parse image ref (%s): %w", imageRef, err)
	}
	remoteStore, err := newRemoteBlobStore(refspec, client)
	if err != nil {
		return fmt.Errorf("cannot create remote store: %w", err)
	}
	fetcher, err := newArtifactFetcher(refspec, fs.contentStore, remoteStore)
	if err != nil {
		return fmt.Errorf("cannot create fetcher: %w", err)
	}

	desc := s.Target

	// If the descriptor size is zero, the artifact fetcher will resolve it.
	// However, it never returns this resolved descriptor.
	// Since the unpacker is also in charge of storing the content and the
	// ORAS store requires an expected size, we need to resolve here.
	if desc.Size == 0 {
		// In remoteStore.Reference, Registry and Target should be correct.
		// However, we need Reference to point to the current layer.
		blobRef := remoteStore.Reference
		blobRef.Reference = s.Target.Digest.String()
		desc, err = remoteStore.Resolve(ctx, blobRef.String())
		if err != nil {
			return fmt.Errorf("cannot resolve size of layer (%s): %w", blobRef.String(), err)
		}
	}

	imageDigest, ok := labels[ctdsnapshotters.TargetManifestDigestLabel]
	if !ok {
		return errors.New("layer has no image manifest attached")
	}
	manifest, err := fs.getImageManifest(ctx, imageDigest)
	if err != nil {
		return fmt.Errorf("cannot get image manifest: %w", err)
	}
	diffIDMap, err := fs.getDiffIDMap(ctx, manifest)
	if err != nil {
		return fmt.Errorf("error getting uncompressed shasums for image %s: %v", desc.Digest, err)
	}
	uncompressedDigest, ok := diffIDMap[desc.Digest.String()]
	if !ok {
		return fmt.Errorf("digest %s not found in image manifest", desc.Digest.String())
	}

	archive := NewLayerArchive(nil, uncompressedDigest.Verifier(), nil)
	unpacker := NewLayerUnpacker(fetcher, archive)

	err = unpacker.Unpack(ctx, desc, mountpoint, mounts)
	if err != nil {
		return fmt.Errorf("cannot unpack the layer: %w", err)
	}

	return nil
}

func (fs *filesystem) getSociContext(ctx context.Context, imageRef, indexDigest, imageManifestDigest string, client *http.Client) (*sociContext, error) {
	cAny, _ := fs.sociContexts.LoadOrStore(imageManifestDigest, &sociContext{})
	c, ok := cAny.(*sociContext)
	if !ok {
		return nil, fmt.Errorf("could not load index: fs soci context is invalid type for %s", indexDigest)
	}
	err := c.Init(ctx, fs, imageRef, indexDigest, imageManifestDigest, client)
	return c, err
}

func (fs *filesystem) fetchSociIndex(ctx context.Context, imageRef, indexDigest, imageManifestDigest string, client *http.Client) (*soci.Index, error) {
	refspec, err := reference.Parse(imageRef)
	if err != nil {
		return nil, err
	}

	remoteStore, err := newRemoteStore(refspec, client)
	if err != nil {
		return nil, err
	}

	indexDesc, err := fs.findSociIndexDesc(ctx, imageManifestDigest, indexDigest, remoteStore)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", snapshot.ErrNoIndex, err)
	}

	log.G(ctx).WithField("digest", indexDesc.Digest.String()).Infof("fetching SOCI artifacts using index descriptor")

	index, err := FetchSociArtifacts(ctx, refspec, indexDesc, fs.contentStore, remoteStore)
	if err != nil {
		return nil, fmt.Errorf("%w: error trying to fetch SOCI artifacts: %w", snapshot.ErrNoIndex, err)
	}
	return index, nil
}

func (fs *filesystem) findSociIndexDesc(ctx context.Context, imageManifestDigest string, sociIndexDigest string, remoteStore *orasremote.Repository) (ocispec.Descriptor, error) {
	imgDigest, err := digest.Parse(imageManifestDigest)
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("unable to parse image digest: %w", err)
	}

	// 1. Try to find an explicit index digest
	if sociIndexDigest != "" {
		log.G(ctx).Debug("using provided soci index digest")
		return parseIndexDigest(sociIndexDigest)
	}
	log.G(ctx).Debug("index digest not provided")

	if !fs.pullModes.SOCIv1.Enable && !fs.pullModes.SOCIv2.Enable {
		return ocispec.Descriptor{}, ErrAllLazyPullModesDisabled
	}

	// 2. Try to find an index digest in the manifest labels if SOCI v2 is enabled.
	if fs.pullModes.SOCIv2.Enable {
		log.G(ctx).Debug("checking for soci v2 index annotation")
		desc, err := findSociIndexDescAnnotation(ctx, imgDigest, remoteStore)
		if err == nil {
			log.G(ctx).Debug("using soci v2 index annotation")
			return desc, nil
		}
		if !errors.Is(err, errdefs.ErrNotFound) {
			return ocispec.Descriptor{}, err
		}
		log.G(ctx).Debug("soci v2 index annotation not found")
	} else {
		log.G(ctx).Debug("soci v2 is disabled")
	}

	// 3. Try to find an index using the referrers API if SOCI v1 is enabled.
	if fs.pullModes.SOCIv1.Enable {
		log.G(ctx).Debug("checking for soci v1 index via referrers API")
		desc, err := findSociIndexDescReferrer(ctx, imgDigest, remoteStore)
		if err == nil {
			log.G(ctx).Debug("using soci v1 index via referrers API")
			return desc, nil
		}
		if !errors.Is(err, ErrNoReferrers) {
			return ocispec.Descriptor{}, err
		}
		log.G(ctx).Debug("soci v1 referrers not found")
	} else {
		log.G(ctx).Debug("soci v1 is disabled")
	}

	return ocispec.Descriptor{}, errdefs.ErrNotFound
}

func parseIndexDigest(sociIndexDigest string) (ocispec.Descriptor, error) {
	dg, err := digest.Parse(sociIndexDigest)
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("unable to parse SOCI index digest: %w", err)
	}
	return ocispec.Descriptor{
		Digest: dg,
	}, nil
}

func findSociIndexDescAnnotation(ctx context.Context, imgDigest digest.Digest, remoteStore *orasremote.Repository) (ocispec.Descriptor, error) {
	ref := remoteStore.Reference
	ref.Reference = imgDigest.String()
	_, r, err := remoteStore.Manifests().FetchReference(ctx, ref.Reference)
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("could not fetch manifest: %w", err)
	}
	var manifest ocispec.Manifest
	err = json.NewDecoder(r).Decode(&manifest)
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("could not unmarshal manifest: %w", err)
	}
	if manifest.Annotations == nil {
		return ocispec.Descriptor{}, errdefs.ErrNotFound
	}

	indexDigestStr := manifest.Annotations[soci.ImageAnnotationSociIndexDigest]
	if indexDigestStr != "" {
		return parseIndexDigest(indexDigestStr)
	}
	return ocispec.Descriptor{}, errdefs.ErrNotFound
}

func findSociIndexDescReferrer(ctx context.Context, imgDigest digest.Digest, remoteStore *orasremote.Repository) (ocispec.Descriptor, error) {
	artifactClient := NewOCIArtifactClient(remoteStore)

	desc, err := artifactClient.SelectReferrer(ctx, ocispec.Descriptor{Digest: imgDigest}, defaultIndexSelectionPolicy)
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("cannot fetch list of referrers: %w", err)
	}
	return desc, nil
}

func getIDMappedMountpoint(mountpoint, activeLayerID string) string {
	d := filepath.Dir(mountpoint)
	return filepath.Join(fmt.Sprintf("%s_%s", d, activeLayerID), "fs")
}

func (fs *filesystem) IDMapMount(ctx context.Context, mountpoint, activeLayerID string, idmapper idtools.IDMap) (string, error) {
	newMountpoint := getIDMappedMountpoint(mountpoint, activeLayerID)
	logger := log.G(ctx).WithField("mountpoint", newMountpoint)

	logger.Debug("creating remote id-mapped mount")
	if err := os.Mkdir(filepath.Dir(newMountpoint), 0700); err != nil {
		return "", err
	}
	if err := os.Mkdir(newMountpoint, 0755); err != nil {
		return "", err
	}

	fs.layerMu.Lock()
	l := fs.layer[mountpoint]
	if l == nil {
		fs.layerMu.Unlock()
		logger.Error("failed to create remote id-mapped mount")
		return "", errdefs.ErrNotFound
	}
	fs.layer[newMountpoint] = l
	fs.layerMu.Unlock()
	node, err := l.RootNode(0, idmapper)
	if err != nil {
		return "", err
	}

	fuseLogger := log.L.
		WithField("mountpoint", mountpoint).
		WriterLevel(logrus.TraceLevel)

	return newMountpoint, fs.setupFuseServer(ctx, newMountpoint, node, l, fuseLogger, nil)
}

func (fs *filesystem) IDMapMountLocal(ctx context.Context, mountpoint, activeLayerID string, idmapper idtools.IDMap) (string, error) {
	newMountpoint := getIDMappedMountpoint(mountpoint, activeLayerID)
	logger := log.G(ctx).WithField("mountpoint", newMountpoint)

	logger.Debug("creating local id-mapped mount")
	if err := idtools.RemapDir(ctx, mountpoint, newMountpoint, idmapper); err != nil {
		logger.WithError(err).Error("failed to create local mount")
		return "", err
	}

	logger.Debug("successfully created local mountpoint")
	return newMountpoint, nil
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

	// Get source information of this layer.
	src, err := fs.getSources(labels)
	if err != nil {
		return err
	} else if len(src) == 0 {
		return fmt.Errorf("source must be passed")
	}
	client := src[0].Hosts[0].Client
	c, err := fs.getSociContext(ctx, imageRef, sociIndexDigest, imgDigest, client)
	if err != nil {
		return fmt.Errorf("unable to fetch SOCI artifacts for image %q: %w", imageRef, err)
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

			l, err := fs.resolver.Resolve(ctx, s.Hosts, s.Name, s.Target, sociDesc, c.fuseOperationCounter, fs.disableVerification)
			if err == nil {
				resultChan <- l
				return
			}
			rErr = fmt.Errorf("failed to resolve layer %q from %q: %w", s.Target.Digest, s.Name, err)
		}
		errChan <- rErr
	}()

	ns, ok := namespaces.Namespace(ctx)
	if !ok {
		return errors.New("could not find namespace attached to context")
	}
	// Also resolve and cache other layers in parallel
	preResolve := src[0] // TODO: should we pre-resolve blobs in other sources as well?
	for _, desc := range neighboringLayers(preResolve.Manifest, preResolve.Target) {
		imgNameAndDigest := preResolve.Name.String() + "/" + desc.Digest.String()
		fs.pr.Enqueue(imgNameAndDigest, func(ctx context.Context) string {
			// Use context from the preresolver, but append namespace from current ctx
			ctx = namespaces.WithNamespace(ctx, ns)
			sociDesc, ok := c.imageLayerToSociDesc[desc.Digest.String()]
			if !ok {
				log.G(ctx).WithError(snapshot.ErrNoZtoc).WithField("layerDigest", desc.Digest.String()).Debug("skipping layer pre-resolve")
				return imgNameAndDigest
			}

			l, err := fs.resolver.Resolve(ctx, preResolve.Hosts, preResolve.Name, desc, sociDesc, c.fuseOperationCounter, fs.disableVerification)
			if err != nil {
				log.G(ctx).WithError(err).Debug("failed to pre-resolve")
				return imgNameAndDigest
			}
			// Release this layer because this isn't target and we don't use it anymore here.
			// However, this will remain on the resolver cache until eviction.
			l.Done()

			return imgNameAndDigest
		})
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

	node, err := l.RootNode(0, idtools.IDMap{})
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

	// Pass in a logger to go-fuse with the layer digest
	// The go-fuse logs are useful for tracing exactly what's happening at the fuse level.
	fuseLogger := log.L.
		WithField("layerDigest", labels[ctdsnapshotters.TargetLayerDigestLabel]).
		WriterLevel(logrus.TraceLevel)

	retErr = fs.setupFuseServer(ctx, mountpoint, node, l, fuseLogger, c)
	return
}

func (fs *filesystem) setupFuseServer(ctx context.Context, mountpoint string, node fusefs.InodeEmbedder, l layer.Layer, logger *io.PipeWriter, c *sociContext) error {
	// mount the node to the specified mountpoint
	// TODO: bind mount the state directory as a read-only fs on snapshotter's side
	rawFS := fusefs.NewNodeFS(node, &fusefs.Options{
		AttrTimeout:     &fs.attrTimeout,
		EntryTimeout:    &fs.entryTimeout,
		NegativeTimeout: &fs.negativeTimeout,
		NullPermissions: true,
	})
	mountOpts := &fuse.MountOptions{
		AllowOther:    true,   // allow users other than root&mounter to access fs
		FsName:        "soci", // name this filesystem as "soci"
		Debug:         fs.debug,
		Logger:        golog.New(logger, "", 0),
		DisableXAttrs: l.DisableXAttrs(),
	}
	if _, err := exec.LookPath(fusermountBin); err == nil {
		mountOpts.Options = []string{"suid"} // option for fusermount; allow setuid inside container
	} else {
		log.G(ctx).WithField("binary", fusermountBin).WithError(err).Info("fusermount binary not installed; trying direct mount")
		mountOpts.DirectMount = true
	}
	server, err := fuse.NewServer(rawFS, mountpoint, mountOpts)
	if err != nil {
		log.G(ctx).WithError(err).Error("failed to make filesystem server")
		return err
	}

	go server.Serve()

	if c != nil {
		// Send a signal to the background fetcher that a new image is being mounted
		// and to pause all background fetches.
		c.bgFetchPauseOnce.Do(func() {
			if fs.bgFetcher != nil {
				fs.bgFetcher.Pause()
			}
		})
	}

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

	if l.Info().FetchedSize < l.Info().Size {
		// Image contents hasn't fully cached yet.
		// Check the blob connectivity and try to refresh the connection on failure
		if err := fs.check(ctx, l, labels); err != nil {
			log.G(ctx).WithError(err).Warn("check failed")
			return err
		}
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

func isIDMappedDir(mountpoint string) bool {
	dirName := filepath.Base(mountpoint)
	return len(strings.Split(dirName, "_")) > 1
}

func (fs *filesystem) Unmount(ctx context.Context, mountpoint string) error {
	fs.layerMu.Lock()
	l, ok := fs.layer[mountpoint]
	if !ok {
		fs.layerMu.Unlock()
		return fmt.Errorf("specified path %q isn't a mountpoint", mountpoint)
	}

	delete(fs.layer, mountpoint)
	// If the mountpoint is an id-mapped layer, it is pointing to the
	// underlying layer, so we cannot call done on it.
	if !isIDMappedDir(mountpoint) {
		l.Done()
	}
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
