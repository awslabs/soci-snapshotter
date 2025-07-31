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

package fs

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/awslabs/soci-snapshotter/config"
	"github.com/containerd/log"
	"golang.org/x/sync/semaphore"
)

const (
	alphanumeric              = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	defaultUnpackDirLen       = 10 // Arbitrary length
	unlimited           int64 = 0
	maxRetriesUniqueID        = 10 // Arbitrary amount
	unpackDir                 = "unpack"
	unpackDirPerm             = 0700
	layerUnpackDir            = "fs"
	layerUnpackDirPerm        = 0700
)

var (
	// TODO: pipe through garbage collection frequency to configuration.
	// The default frequency for garbage collection. This value was arbitrarily chosen.
	garbageCollectionInterval = 10 * time.Second

	// TODO: pipe through garbage collection job expiration.
	// The default expiration time for in progress jobs to be garbage collected.
	garbageCollectionJobExpiration = 2 * time.Hour
)

var (
	ErrParallelPullIsDisabled = errors.New("the given config does not allow parallel pulling")

	ErrImageUnpackJobNotFound    = errors.New("image unpack job not found")
	ErrImageUnpackJobHasNoLayers = errors.New("image unpack job has no layers")
	ErrImageUnpackJobExpired     = errors.New("image unpack job has expired")
	ErrLayerHasNoJobs            = errors.New("layer has no jobs")

	ErrNoClaimableLayerJobs    = errors.New("no claimable jobs")
	ErrLayerJobNotFound        = errors.New("specified layer job not found")
	ErrLayerJobCannotBeCleaned = errors.New("layer job cannot be cleaned (is not claimed or cancelled)")

	// ErrLayerIngestDoesNotExist can occur during layer unpack operations when parallel layer download has disabled.
	ErrLayerIngestDoesNotExist = errors.New("layer ingest does not exist")

	// ErrLayerUnpackDestinationHasContent can occur during layer unpack operations. Before writing content to disk,
	// the unpacker verifies the destination directory has no pre-existing content as a layer of protection from
	// container image layer poisoning attacks.
	ErrLayerUnpackDestinationHasContent = errors.New("layer unpack destination has content")
)

type SemaphoreWithNil struct {
	smp *semaphore.Weighted
}

func NewSemaphoreWithNil(n int64) *SemaphoreWithNil {
	s := &SemaphoreWithNil{}
	if n > unlimited {
		s.smp = semaphore.NewWeighted(n)
	}
	return s
}

func (s *SemaphoreWithNil) Acquire(ctx context.Context, n int64) error {
	if s.smp != nil {
		return s.smp.Acquire(ctx, n)
	}
	return nil
}

func (s *SemaphoreWithNil) Release(n int64) {
	if s.smp != nil {
		s.smp.Release(n)
	}
}

type unpackJobs struct {
	imagePullCfg *config.ParallelConfig

	globalConcurrentDownloadsLimiter *SemaphoreWithNil
	globalConcurrentUnpacksLimiter   *SemaphoreWithNil

	storage LayerUnpackJobStorage

	images map[string]*imageUnpackJob
	mu     sync.Mutex
}

func newUnpackJobs(ctx context.Context, parallelConfig *config.Parallel, storage LayerUnpackJobStorage) (*unpackJobs, error) {
	var (
		globalConcurrentDownloadsLimit = parallelConfig.MaxConcurrentDownloads
		globalConcurrentUnpacksLimit   = parallelConfig.MaxConcurrentUnpacks
	)

	if err := checkParallelPullUnpack(parallelConfig); err != nil {
		return nil, fmt.Errorf("error validating image pull config: %w", err)
	}

	log.G(ctx).WithFields(log.Fields{
		"max_concurrent_downloads":           parallelConfig.MaxConcurrentDownloads,
		"max_concurrent_downloads_per_image": parallelConfig.MaxConcurrentDownloadsPerImage,
		"max_concurrent_unpacks":             parallelConfig.MaxConcurrentUnpacks,
		"max_concurrent_unpacks_per_image":   parallelConfig.MaxConcurrentUnpacksPerImage,
		"discard_unpack_layers":              parallelConfig.DiscardUnpackedLayers,
		"concurrent_download_chunk_size":     parallelConfig.ConcurrentDownloadChunkSize,
	}).Info("Parallel image pull enabled")

	if parallelConfig.MaxConcurrentDownloads == config.Unbounded {
		globalConcurrentDownloadsLimit = unlimited
	}

	if parallelConfig.MaxConcurrentUnpacks == config.Unbounded {
		globalConcurrentUnpacksLimit = unlimited
	}

	jobs := &unpackJobs{
		imagePullCfg:                     &parallelConfig.ParallelConfig,
		globalConcurrentDownloadsLimiter: NewSemaphoreWithNil(globalConcurrentDownloadsLimit),
		globalConcurrentUnpacksLimiter:   NewSemaphoreWithNil(globalConcurrentUnpacksLimit),
		images:                           make(map[string]*imageUnpackJob),
		storage:                          storage,
		mu:                               sync.Mutex{},
	}

	garbageCollector := newGarbageCollector(garbageCollectionInterval, jobs, storage)
	go garbageCollector.Run(ctx)

	return jobs, nil
}

func checkParallelPullUnpack(cfg *config.Parallel) error {
	if cfg == nil {
		return errors.New("parallel pull config is nil")
	}
	if !cfg.Enable {
		return ErrParallelPullIsDisabled
	}
	// If global concurrent downloads/unpacks are unlimited, any value for per-image concurrent downloads/unpacks are valid
	var err error
	if cfg.MaxConcurrentDownloads > unlimited && cfg.MaxConcurrentDownloadsPerImage > cfg.MaxConcurrentDownloads {
		err = errors.New("global download limit less than per-image download limit")
	}

	// If global concurrent downloads/unpacks are limited, per-image concurrent downloads/unpacks should be <= the global value
	if cfg.MaxConcurrentUnpacks > unlimited && cfg.MaxConcurrentUnpacksPerImage > cfg.MaxConcurrentUnpacks {
		err = errors.Join(err, errors.New("global unpack limit less than per-image unpack limit"))
	}
	return err
}

// GetOrAddImageJob adds the requisite image job to unpackJobs.
// If the job already exists, return nil.
// Else, return the newly created imageUnpackJob.
func (jobs *unpackJobs) GetOrAddImageJob(imageDigest string, cancel context.CancelCauseFunc) *imageUnpackJob {
	jobs.mu.Lock()
	defer jobs.mu.Unlock()

	if jobs.imageExists(imageDigest) {
		return jobs.images[imageDigest]
	}

	jobs.images[imageDigest] = newImageUnpackJob(imageDigest,
		withImageDownloadsLimit(jobs.imagePullCfg.MaxConcurrentDownloadsPerImage),
		withImageUnpacksLimit(jobs.imagePullCfg.MaxConcurrentUnpacksPerImage),
		withGlobalConcurrentDownloadsLimiter(jobs.globalConcurrentDownloadsLimiter),
		withGlobalConcurrentUnpacksLimiter(jobs.globalConcurrentUnpacksLimiter),
		withCancelFunc(cancel),
	)

	return jobs.images[imageDigest]
}

// AddLayerJob both adds the job to the in-memory store and creates the requisite folder on disk
func (jobs *unpackJobs) AddLayerJob(imageJob *imageUnpackJob, layerDigest string) (*layerUnpackJob, error) {
	jobs.mu.Lock()
	defer jobs.mu.Unlock()

	layerJob, err := newLayerUnpackJob(layerDigest, jobs.storage, withImageUnpackJob(imageJob))
	if err != nil {
		return nil, err
	}

	jobs.images[imageJob.imageDigest].layers[layerDigest] =
		append(jobs.images[imageJob.imageDigest].layers[layerDigest], layerJob)

	return layerJob, nil
}

func (jobs *unpackJobs) checkLayerExists(imageDigest, layerDigest string) error {
	if !jobs.imageExists(imageDigest) {
		return fmt.Errorf("%w: %s", ErrImageUnpackJobNotFound, imageDigest)
	}

	if len(jobs.images[imageDigest].layers) == 0 {
		return fmt.Errorf("%w: %s", ErrImageUnpackJobHasNoLayers, imageDigest)
	}

	if len(jobs.images[imageDigest].layers[layerDigest]) == 0 {
		return fmt.Errorf("%w: %s", ErrLayerHasNoJobs, layerDigest)
	}

	return nil
}

func (jobs *unpackJobs) ImageExists(imageDigest string) bool {
	jobs.mu.Lock()
	defer jobs.mu.Unlock()

	return jobs.imageExists(imageDigest)
}

func (jobs *unpackJobs) imageExists(imageDigest string) bool {
	_, ok := jobs.images[imageDigest]
	return ok
}

// Claim marks a job so that no other jobs can attempt to rebase it.
// Claim should always be followed by a remove on the same job.
func (jobs *unpackJobs) Claim(imageDigest, layerDigest string) (*layerUnpackJob, error) {
	jobs.mu.Lock()
	defer jobs.mu.Unlock()

	if err := jobs.checkLayerExists(imageDigest, layerDigest); err != nil {
		return nil, fmt.Errorf("claim job: %w", err)
	}

	// Loop through all available jobs until we can claim one
	for _, status := range jobs.images[imageDigest].layers[layerDigest] {
		if status.Claim() {
			return status, nil
		}
	}

	return nil, ErrNoClaimableLayerJobs
}

var (
	errEmptyJobID            = errors.New("job id is empty")
	errUniqueJobIDGenFailure = errors.New("failed to generate unique job id")
	errJobNotFound           = errors.New("job not found")
)

// LayerUnpackJobStorage defines an interface for persisting layer unpack job state
// to a durable storage medium.
type LayerUnpackJobStorage interface {
	// Create an unpack job in storage and return its unique identifier.
	Create() (string, error)

	// GetJobPath returns a path on disk to use for a specified unpack job.
	GetJobPath(string) (string, error)

	// Keys lists all jobs in storage.
	Keys() ([]string, error)

	// Delete a specified unpack job from storage.
	Delete(string) error
}

// LayerUnpackDiskStorage persists image unpack jobs to disk.
type LayerUnpackDiskStorage struct {
	unpackRoot string
}

func newLayerUnpackDiskStorage(root string) (LayerUnpackJobStorage, error) {
	directory := filepath.Join(root, unpackDir)

	// Check if unpack directory already exists, and remove it if found.
	if _, err := os.Stat(directory); err == nil {
		// Directory exists, remove it
		if err := os.RemoveAll(directory); err != nil {
			return nil, fmt.Errorf("failed to remove existing unpack directory: %w", err)
		}
		log.G(context.Background()).WithField("dir", directory).Debug("removed existing unpack directory")
	}

	// Create a fresh unpack directory
	if err := os.MkdirAll(directory, unpackDirPerm); err != nil {
		return nil, err
	}

	return &LayerUnpackDiskStorage{
		unpackRoot: directory,
	}, nil
}

// Create an unpack job on disk with a unique identifier.
func (disk LayerUnpackDiskStorage) Create() (string, error) {
	key, err := disk.generateUniqueKey()
	if err != nil {
		return "", err
	}

	err = os.MkdirAll(filepath.Join(disk.unpackRoot, key, layerUnpackDir), layerUnpackDirPerm)
	if err != nil {
		return "", err
	}
	return key, nil
}

func (disk LayerUnpackDiskStorage) generateUniqueKey() (string, error) {
	for attempt := range maxRetriesUniqueID {
		key := generateUniqueString(defaultUnpackDirLen)
		if len(key) > 0 && !disk.contains(key) {
			return key, nil
		}
		log.G(context.Background()).WithField("id", key).WithField("attempt", attempt+1).Debug("randomly generated id already exists in storage")
	}
	return "", errUniqueJobIDGenFailure
}

func generateUniqueString(n int) string {
	sb := strings.Builder{}
	sb.Grow(n)
	for range n {
		sb.WriteByte(alphanumeric[rand.Int64()%int64(len(alphanumeric))])
	}
	return sb.String()
}

func (disk LayerUnpackDiskStorage) GetJobPath(id string) (string, error) {
	if !disk.contains(id) {
		return "", errJobNotFound
	}
	return disk.getJobPath(id), nil
}

func (disk LayerUnpackDiskStorage) getJobPath(id string) string {
	return filepath.Join(disk.unpackRoot, id)
}

// Keys unpack jobs found on disk.
// If the root unpack directory does not exist, then an empty list will be returned with no error.
func (disk LayerUnpackDiskStorage) Keys() ([]string, error) {
	entries, err := os.ReadDir(disk.unpackRoot)
	if errors.Is(err, fs.ErrNotExist) {
		// No unpack jobs found on disk. Return an empty list; not an error.
		return []string{}, nil
	} else if err != nil {
		return []string{}, err
	}

	unpackJobs := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			unpackJobs = append(unpackJobs, entry.Name())
		}
	}

	return unpackJobs, nil
}

func (disk LayerUnpackDiskStorage) contains(id string) bool {
	_, err := os.Stat(disk.getJobPath(id))
	return err == nil
}

// Delete the specified layer unpack job from disk.
func (disk LayerUnpackDiskStorage) Delete(id string) error {
	if id == "" {
		return errEmptyJobID
	}
	return os.RemoveAll(disk.getJobPath(id))
}

type unpackJobsSnapshot struct {
	inMemory  map[string]*layerUnpackJobSnapshot
	inStorage []string
}

type layerUnpackJobSnapshot struct {
	imageID           string
	creationTimestamp int64
}

func (jobs *unpackJobs) Snapshot(ctx context.Context) (*unpackJobsSnapshot, error) {
	jobs.mu.Lock()
	defer jobs.mu.Unlock()

	snapshot := &unpackJobsSnapshot{
		inMemory: make(map[string]*layerUnpackJobSnapshot),
	}

	// All jobs in storage are keyed by layerUnpackID, so go through every single layerUnpackJob
	for _, image := range jobs.images {
		for _, layerJobs := range image.layers {
			for _, v := range layerJobs {
				snapshot.inMemory[v.layerUnpackID] = &layerUnpackJobSnapshot{
					imageID:           v.imageDigest,
					creationTimestamp: v.creationTimestamp,
				}
			}
		}
	}

	inStorage, err := jobs.storage.Keys()
	if err != nil {
		return nil, err
	}
	snapshot.inStorage = inStorage

	return snapshot, nil
}

// Remove will remove the specified layer digest from memory,
// indicating all fetch and decompress operations have concluded,
// successful or otherwise.
// This will only remove cancelled or claimed jobs.
// If no available jobs meet this criteria, it will return an error.
func (jobs *unpackJobs) Remove(job *layerUnpackJob, cause error) error {
	jobs.mu.Lock()
	defer jobs.mu.Unlock()

	return jobs.remove(job, cause)
}

func (jobs *unpackJobs) RemoveImageWithError(imageDigest string, cause error) error {
	jobs.mu.Lock()
	defer jobs.mu.Unlock()

	imageJob, ok := jobs.images[imageDigest]
	if !ok {
		return fmt.Errorf("%w: %s", ErrImageUnpackJobNotFound, imageDigest)
	}

	// Cancelling an image will cancel all layer unpack jobs.
	imageJob.Cancel(cause)
	delete(jobs.images, imageDigest)
	return nil
}

func (jobs *unpackJobs) remove(job *layerUnpackJob, cause error) error {
	imageDigest := job.imageDigest
	layerDigest := job.layerDigest
	layerUnpackID := job.layerUnpackID

	if err := jobs.checkLayerExists(imageDigest, layerDigest); err != nil {
		return fmt.Errorf("remove job: %w", err)
	}

	for i, status := range jobs.images[imageDigest].layers[layerDigest] {
		if status.layerUnpackID == layerUnpackID {
			// Proceed only if status is claimed or cancelled
			switch status.status.Load() {
			case LayerUnpackJobClaimed:
			case LayerUnpackJobCancelled:
			default:
				return fmt.Errorf("%w: %s", ErrLayerJobCannotBeCleaned, layerUnpackID)
			}

			jobs.images[imageDigest].layers[layerDigest] = slices.Delete(jobs.images[imageDigest].layers[layerDigest], i, i+1)
			if len(jobs.images[imageDigest].layers[layerDigest]) == 0 {
				delete(jobs.images[imageDigest].layers, layerDigest)
			}
			if len(jobs.images[imageDigest].layers) == 0 {
				// Call cancel to avoid context leak
				jobs.images[imageDigest].cancel(cause)
				delete(jobs.images, imageDigest)
			}
			return nil
		}
	}

	return ErrLayerJobNotFound
}

type imageUnpackJob struct {
	ctx    context.Context
	cancel context.CancelCauseFunc

	imageDigest       string
	creationTimestamp int64

	globalConcurrentDownloadsLimiter *SemaphoreWithNil
	globalConcurrentUnpacksLimiter   *SemaphoreWithNil
	concurrentDownloadsLimiter       *SemaphoreWithNil
	concurrentUnpacksLimiter         *SemaphoreWithNil

	layers map[string][]*layerUnpackJob
}

type imageUnpackOption func(*imageUnpackJob)

func withImageDownloadsLimit(limit int64) imageUnpackOption {
	return func(job *imageUnpackJob) {
		if limit == config.Unbounded {
			limit = unlimited
		}
		job.concurrentDownloadsLimiter = NewSemaphoreWithNil(limit)
	}
}

func withImageUnpacksLimit(limit int64) imageUnpackOption {
	return func(job *imageUnpackJob) {
		if limit == config.Unbounded {
			limit = unlimited
		}
		job.concurrentUnpacksLimiter = NewSemaphoreWithNil(limit)
	}
}

func withGlobalConcurrentDownloadsLimiter(smp *SemaphoreWithNil) imageUnpackOption {
	return func(job *imageUnpackJob) {
		job.globalConcurrentDownloadsLimiter = smp
	}

}

func withGlobalConcurrentUnpacksLimiter(smp *SemaphoreWithNil) imageUnpackOption {
	return func(job *imageUnpackJob) {
		job.globalConcurrentUnpacksLimiter = smp
	}
}

func withCancelFunc(cancel context.CancelCauseFunc) imageUnpackOption {
	return func(job *imageUnpackJob) {
		job.cancel = cancel
	}
}

var now = func() time.Time {
	return time.Now()
}

func newImageUnpackJob(imageDigest string, opts ...imageUnpackOption) *imageUnpackJob {
	ctx, cancel := context.WithCancelCause(context.Background())
	job := &imageUnpackJob{
		ctx:                              ctx,
		cancel:                           cancel,
		imageDigest:                      imageDigest,
		creationTimestamp:                now().UnixNano(),
		globalConcurrentDownloadsLimiter: NewSemaphoreWithNil(unlimited),
		globalConcurrentUnpacksLimiter:   NewSemaphoreWithNil(unlimited),
		concurrentDownloadsLimiter:       NewSemaphoreWithNil(unlimited),
		concurrentUnpacksLimiter:         NewSemaphoreWithNil(unlimited),
		layers:                           make(map[string][]*layerUnpackJob),
	}

	for _, opt := range opts {
		opt(job)
	}

	return job
}

func (job *imageUnpackJob) Cancel(cause error) {
	job.cancel(cause)
}

type layerUnpackJobStatus int

const (
	LayerUnpackJobInProgress layerUnpackJobStatus = iota
	LayerUnpackJobClaimed
	LayerUnpackJobDone
	LayerUnpackJobFailed
	LayerUnpackJobCancelled
)

// LayerUnpackResourceController implements various controls for a layer unpack resources.
type LayerUnpackResourceController interface {
	// AcquireUnpackLease rate limits unpackers based on global and per image unpack concurrency limits.
	AcquireUnpackLease(context.Context) (func(), error)

	// GetUnpackIngestReader returns a reader for the compressed layer tarball on disk for unpacking.
	GetUnpackIngestReader() (io.ReadCloser, error)

	// VerifyUnpackDestinationIsReady verifies the destination for a layer unpack exists and has no pre-existing content.
	VerifyUnpackDestinationIsReady() error
}

type layerUnpackJobOption func(*layerUnpackJob)

type layerUnpackJob struct {
	// Inherit fields from parent via withImageUnpackJob
	imageDigest                      string
	cancel                           context.CancelCauseFunc
	globalConcurrentDownloadsLimiter *SemaphoreWithNil
	globalConcurrentUnpacksLimiter   *SemaphoreWithNil
	concurrentDownloadsLimiter       *SemaphoreWithNil
	concurrentUnpacksLimiter         *SemaphoreWithNil
	creationTimestamp                int64

	// Unique to layerUnpackJob struct
	layerUnpackID string
	layerDigest   string
	ingestPath    string
	upperPath     string
	errCh         chan error
	status        atomic.Value
}

func withImageUnpackJob(image *imageUnpackJob) layerUnpackJobOption {
	return func(luj *layerUnpackJob) {
		luj.imageDigest = image.imageDigest
		luj.cancel = image.cancel
		luj.globalConcurrentDownloadsLimiter = image.globalConcurrentDownloadsLimiter
		luj.globalConcurrentUnpacksLimiter = image.globalConcurrentUnpacksLimiter
		luj.concurrentDownloadsLimiter = image.concurrentDownloadsLimiter
		luj.concurrentUnpacksLimiter = image.concurrentUnpacksLimiter
		luj.creationTimestamp = image.creationTimestamp
	}
}

// Note: unpacker needs to provide same errCh for all layers associated with a ingest so
// the channel is correctly sized.
func newLayerUnpackJob(layerDigest string, storage LayerUnpackJobStorage, opts ...layerUnpackJobOption) (*layerUnpackJob, error) {
	id, err := storage.Create()
	if err != nil {
		return nil, fmt.Errorf("failed to create new layer unpack job in storage for layer %s: %w", layerDigest, err)
	}

	path, err := storage.GetJobPath(id)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch unpack root for layer %s: %w", layerDigest, err)
	}

	luj := &layerUnpackJob{
		layerUnpackID: id,
		layerDigest:   layerDigest,
		ingestPath:    filepath.Join(path, layerDigest),
		upperPath:     filepath.Join(path, layerUnpackDir),
		status:        atomic.Value{},
		errCh:         make(chan error, 1),
	}

	for _, opt := range opts {
		opt(luj)
	}

	luj.status.Store(LayerUnpackJobInProgress)

	return luj, nil
}

func (job *layerUnpackJob) AcquireDownload(ctx context.Context, n int64) error {
	if err := job.concurrentDownloadsLimiter.Acquire(ctx, n); err != nil {
		return err
	}
	return job.globalConcurrentDownloadsLimiter.Acquire(ctx, n)
}

func (job *layerUnpackJob) ReleaseDownload(n int64) {
	job.concurrentDownloadsLimiter.Release(n)
	job.globalConcurrentDownloadsLimiter.Release(n)
}

// AcquireUnpackLease is a blocking call to acquire the bandwidth to unpack a layer in parallel.
// A closure function is returned for freeing resources along with an error if the allocation is unsuccessful.
func (job *layerUnpackJob) AcquireUnpackLease(ctx context.Context) (func(), error) {
	if err := job.concurrentUnpacksLimiter.Acquire(ctx, 1); err != nil {
		return nil, err
	}

	if err := job.globalConcurrentUnpacksLimiter.Acquire(ctx, 1); err != nil {
		// Context was cancelled before global lock was acquired; release the per image lock
		// that was already acquired.
		job.concurrentUnpacksLimiter.Release(1)
		return nil, err
	}

	return func() {
		job.concurrentUnpacksLimiter.Release(1)
		job.globalConcurrentUnpacksLimiter.Release(1)
	}, nil
}

func (job *layerUnpackJob) Cancel(cause error) {
	job.cancel(cause)
	job.status.Store(LayerUnpackJobCancelled)
}

// Claim claims this job and returns true if job is unclaimed,
// and false if the job is already claimed.
func (job *layerUnpackJob) Claim() bool {
	return job.status.CompareAndSwap(LayerUnpackJobInProgress, LayerUnpackJobClaimed)
}

// GetIngestLocation returns the filepath of the compressed tarball for a layer unpack job.
func (job *layerUnpackJob) GetIngestLocation() string {
	return job.ingestPath
}

// GetUnpackIngestReader returns the reader for the compressed tarball on disk.
func (job *layerUnpackJob) GetUnpackIngestReader() (io.ReadCloser, error) {
	// If ingest path is empty, then the compressed tarball was not pulled ahead of time.
	if job.ingestPath == "" {
		return nil, ErrLayerIngestDoesNotExist
	}

	return os.Open(job.GetIngestLocation())
}

// GetUnpackUpperPath returns the filepath for the temporary unpack directory for a layer unpack job.
func (job *layerUnpackJob) GetUnpackUpperPath() string {
	return job.upperPath
}

// VerifyUnpackDestinationIsReady verifies the unpack destination exists and has no pre-existing content.
func (job *layerUnpackJob) VerifyUnpackDestinationIsReady() error {
	f, err := os.Open(job.GetUnpackUpperPath())
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.Readdirnames(1)
	if errors.Is(err, io.EOF) {
		return nil
	}
	return ErrLayerUnpackDestinationHasContent
}

// garbageCollectionPolicy defines the interface for implementing different garbage collection
// strategies to manage unpack job resources. Implementations of this interface determine
// how and when resources should be cleaned up during the image unpacking process.
//
// The policy is enforced periodically by the garbage collector to maintain system resources
// and clean up stale or unused resources.
type garbageCollectionPolicy interface {
	MarkAndSweep(context.Context, *unpackJobsSnapshot)
}

type diskSpaceGarbageCollectionPolicy interface {
	garbageCollectionPolicy
}

// GarbageCollectIfNotInMemory defines a garbage collection policy for freeing resources
// for unpacks found on-disk not being tracked in memory.
type garbageCollectIfNotFoundInMemory struct {
	storage LayerUnpackJobStorage
}

// MarkAndSweep all image unpack resources in persistent state which have no reference in-memory.
func (gcp garbageCollectIfNotFoundInMemory) MarkAndSweep(ctx context.Context, jobs *unpackJobsSnapshot) {
	logger := log.G(ctx).WithField("policy", "NotFoundInMemory")

	jobs.inStorage = slices.DeleteFunc(jobs.inStorage, func(id string) bool {
		if _, ok := jobs.inMemory[id]; ok {
			// Job is tracked in-memory, skip.
			return false
		}

		jobCtxLogger := logger.WithField("id", id)
		jobCtxLogger.Trace("Marked for cleanup")

		if err := gcp.storage.Delete(id); err != nil {
			jobCtxLogger.WithError(err).Error("Failed to free resources for untracked image unpack")
			return false
		}

		return true
	})
}

type memoryGarbageCollectionPolicy interface {
	garbageCollectionPolicy
}

type garbageCollectIfExpired struct {
	expiryTime time.Duration

	// cancelImageUnpack still cancel all layer unpack jobs associated with an image unpack.
	cancelImageUnpack func(string) error
}

// MarkAndSweep marks all in-progress jobs which that were created before the expiry time for garbage collection.
func (p garbageCollectIfExpired) MarkAndSweep(ctx context.Context, jobs *unpackJobsSnapshot) {
	logger := log.G(ctx).WithField("policy", "Expired")
	cancelledImages := map[string]struct{}{}

	maps.DeleteFunc(jobs.inMemory, func(id string, layer *layerUnpackJobSnapshot) bool {
		// All layers from the same image share the creation timestamp. Cancelling one layer will cancel all of them.
		// So just remove the layer from the snapshot if it has already been cancelled.
		if _, ok := cancelledImages[layer.imageID]; ok {
			return true
		}

		if time.Since(time.Unix(0, layer.creationTimestamp)) > p.expiryTime {
			jobCtxLogger := logger.WithField("id", id)
			jobCtxLogger.Trace("Marked for cleanup")

			if err := p.cancelImageUnpack(layer.imageID); err != nil {
				jobCtxLogger.WithError(err).Error("Failed to cancel job")
				return false
			}
			cancelledImages[layer.imageID] = struct{}{}

			jobCtxLogger.Trace("Reclaimed memory")
			return true
		}
		return false
	})
}

type garbageCollector struct {
	interval time.Duration
	snapshot func(context.Context) (*unpackJobsSnapshot, error)

	unusedMemory    []memoryGarbageCollectionPolicy
	unusedDiskSpace diskSpaceGarbageCollectionPolicy
}

func newGarbageCollector(interval time.Duration, jobs *unpackJobs, storage LayerUnpackJobStorage) *garbageCollector {
	return &garbageCollector{
		interval: interval,
		snapshot: func(ctx context.Context) (*unpackJobsSnapshot, error) {
			return jobs.Snapshot(ctx)
		},
		unusedMemory: []memoryGarbageCollectionPolicy{
			garbageCollectIfExpired{
				expiryTime: garbageCollectionJobExpiration,
				cancelImageUnpack: func(id string) error {
					return jobs.RemoveImageWithError(id, ErrImageUnpackJobExpired)
				},
			},
		},
		unusedDiskSpace: garbageCollectIfNotFoundInMemory{storage: storage},
	}
}

// Run starts the garbage collector and runs the lifetime of the provided context.
func (gc garbageCollector) Run(ctx context.Context) {
	ticker := time.NewTicker(gc.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			snapshot, err := gc.snapshot(ctx)
			if err != nil {
				log.G(ctx).WithError(err).Error("Failed to take snapshot of unpack jobs")
				continue
			}

			for _, policy := range gc.unusedMemory {
				policy.MarkAndSweep(ctx, snapshot)
			}

			// Free disk after memory for garbage collection efficiency.
			gc.unusedDiskSpace.MarkAndSweep(ctx, snapshot)
		case <-ctx.Done():
			return
		}
	}
}
