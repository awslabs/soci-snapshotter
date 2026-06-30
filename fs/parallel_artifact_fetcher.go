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
	"os"
	"path/filepath"
	"time"

	"github.com/awslabs/soci-snapshotter/soci/store"
	"github.com/containerd/containerd/v2/pkg/reference"
	"github.com/containerd/log"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"golang.org/x/sync/errgroup"
)

// artifactFetcher is responsible for fetching and storing artifacts in the provided artifact store.
type parallelArtifactFetcher struct {
	*artifactFetcher
	layerUnpackJob *layerUnpackJob
	chunkSize      int64
	verifier       *asyncVerifier
}

// Constructs a new artifact fetcher
// Takes in the image reference, the local store and the resolver
func newParallelArtifactFetcher(
	refspec reference.Spec, localStore store.BasicStore, remoteStore resolverStorage,
	layerUnpackJob *layerUnpackJob, chunkSize int64, verifier *asyncVerifier,
) (*parallelArtifactFetcher, error) {
	if chunkSize <= 0 {
		chunkSize = unlimited
	}
	return &parallelArtifactFetcher{
		artifactFetcher: &artifactFetcher{
			localStore:  localStore,
			remoteStore: remoteStore,
			refspec:     refspec,
		},
		layerUnpackJob: layerUnpackJob,
		chunkSize:      chunkSize,
		verifier:       verifier,
	}, nil
}

// Fetches the artifact identified by the descriptor.
// It first checks the local store for the artifact.
// If not found, if constructs the ref and writes to a temporary file on disk,
// then returns the readcloser for that file.
func (f *parallelArtifactFetcher) Fetch(ctx context.Context, desc ocispec.Descriptor) (io.ReadCloser, bool, error) {
	// Check local store first
	rc, err := f.localStore.Fetch(ctx, desc)
	if err == nil {
		// Content from local store is already verified by containerd.
		// Mark the verifier as not needing verification to avoid false "digest mismatch" errors.
		f.verifier.SkipVerification()
		log.G(ctx).WithField("digest", desc.Digest.String()).Debug("fetched artifact from local store, skipping compressed verification")
		return rc, true, nil
	}

	log.G(ctx).WithField("digest", desc.Digest.String()).Infof("fetching artifact from remote")
	startTime := time.Now()
	if desc.Size == 0 {
		// Digest verification fails is desc.Size == 0
		// Therefore, we try to use the resolver to resolve the descriptor
		// and hopefully get the size.
		// Note that the resolve would fail for size > 4MiB, since that's the limit
		// for the manifest size when using the Docker resolver.
		log.G(ctx).WithField("digest", desc.Digest).Warnf("size of descriptor is 0, trying to resolve it...")
		desc, err = f.resolve(ctx, desc)
		if err != nil {
			return nil, false, fmt.Errorf("size of descriptor is 0; unable to resolve: %w", err)
		}
	}

	// If it doesn't exist locally, pull concurrently in chunks and write to temporary file
	rc, err = f.fetchFromRemoteAndWriteToTempDir(ctx, desc)
	if err != nil {
		return nil, false, fmt.Errorf("unable to fetch descriptor (%v) from remote store: %w", desc.Digest, err)
	}

	log.G(ctx).WithFields(
		log.Fields{
			"digest":     desc.Digest.String(),
			"size":       desc.Size,
			"latency_ms": time.Since(startTime).Milliseconds(),
		}).Debug("Artifact successfully fetched from remote")
	return rc, false, nil
}

// Returns [lower, upper] of the content we want to read.
func (f *parallelArtifactFetcher) getRange(i, descSize int64) (int64, int64) {
	if f.chunkSize <= unlimited {
		return 0, descSize - 1
	}

	lower := i * f.chunkSize
	upper := min(lower+f.chunkSize, descSize)
	upper-- // Range is inclusive
	return lower, upper
}

// Returns the number of loops needed to fetch a resource
// given a descriptor and the artifactFetcher's chunk size
func (f *parallelArtifactFetcher) calcNumLoops(size int64) int64 {
	if f.chunkSize <= 0 {
		return 1
	}

	// Return ceiling of size / f.chunkSize
	numLoops := size / f.chunkSize
	if size%f.chunkSize != 0 {
		numLoops++
	}
	return numLoops
}

// fetchFromRemoteAndWriteToTempDir pulls the artifact from the remote repository,
// writes it to a file on disk, and returns a ReadCloser for the new file.
// This way we can control when exactly the content is being fetched,
// and any further reads to this content will be fetched from disk,
// not from upstream.
func (f *parallelArtifactFetcher) fetchFromRemoteAndWriteToTempDir(ctx context.Context, desc ocispec.Descriptor) (io.ReadCloser, error) {
	ingestPath := f.layerUnpackJob.GetIngestLocation()
	layerUnpackID := f.layerUnpackJob.layerUnpackID
	parentDir := filepath.Dir(ingestPath)

	logger := log.G(ctx).WithFields(log.Fields{
		"ingestPath":    ingestPath,
		"layerUnpackID": layerUnpackID,
		"parentDir":     parentDir,
		"digest":        desc.Digest.String(),
	})

	// Verify parent directory exists
	if parentInfo, parentErr := os.Stat(parentDir); parentErr != nil {
		logger.WithError(parentErr).Error("parent directory does not exist before file creation")
		return nil, fmt.Errorf("parent directory does not exist at %s: %w", parentDir, parentErr)
	} else {
		logger.WithField("parentIsDir", parentInfo.IsDir()).Debug("parent directory check passed")
	}

	// List contents of parent directory for debugging
	entries, listErr := os.ReadDir(parentDir)
	if listErr != nil {
		logger.WithError(listErr).Warn("failed to list parent directory contents")
	} else {
		entryNames := make([]string, len(entries))
		for i, e := range entries {
			entryNames[i] = e.Name()
		}
		logger.WithField("parentContents", entryNames).Debug("parent directory contents before file creation")
	}

	// Refuse to unpack if file already exists
	_, err := os.Stat(ingestPath)
	if err == nil {
		err = fs.ErrExist
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("error setting up temporary file: %w", err)
	}

	logger.Debug("creating ingest file")
	file, err := os.Create(ingestPath)
	if err != nil {
		return nil, fmt.Errorf("error creating temp ingest file at %s: %w", ingestPath, err)
	}
	logger.WithField("fd", file.Fd()).Debug("file created successfully")

	// Verify file exists immediately after creation
	if statInfo, statErr := os.Stat(ingestPath); statErr != nil {
		logger.WithError(statErr).Error("file does not exist immediately after os.Create")
	} else {
		logger.WithField("size", statInfo.Size()).Debug("file exists after creation")
	}

	defer func() {
		if err != nil {
			file.Close()
		}
	}()

	// Use fallocate to pre-allocate blocks instead of Truncate (which creates sparse files).
	// Sparse files cause block allocation on each WriteAt, which can be slow on some filesystems.
	// fallocate pre-allocates all blocks upfront, making subsequent WriteAt calls faster.
	err = preallocateFile(file, desc.Size)
	if err != nil {
		// Fall back to Truncate if fallocate is not supported (e.g., on some filesystems)
		logger.WithError(err).Warn("fallocate failed, falling back to Truncate")
		err = file.Truncate(desc.Size)
		if err != nil {
			return nil, fmt.Errorf("error truncating temp ingest file at %s: %w", ingestPath, err)
		}
	}
	logger.WithField("targetSize", desc.Size).Debug("file space pre-allocated")

	doMultipleFetches := false
	numLoops := f.calcNumLoops(desc.Size)
	logger.WithFields(log.Fields{
		"chunkSize": f.chunkSize,
		"layerSize": desc.Size,
		"numLoops":  numLoops,
	}).Info("chunk calculation for layer download")
	if numLoops > 1 {
		if rs, ok := f.remoteStore.(*orasBlobStore); ok {
			// If this layer does not support ranged GET, it is very likely
			// all other layers of this image do not either.
			doMultipleFetches, err = rs.doInitialFetch(ctx, f.constructRef(desc))
			if err != nil {
				return nil, fmt.Errorf("error doing initial authorization for layer: %w", err)
			}
		} else {
			logger.Warn("remoteStore is not *orasBlobStore, cannot use range requests")
		}
	} else {
		logger.Info("numLoops <= 1, using single request (set concurrent_download_chunk_size in config to enable chunking)")
	}
	logger.WithField("doMultipleFetches", doMultipleFetches).Info("starting fetch write")
	if doMultipleFetches {
		err = f.multiRequestFetchWrite(ctx, desc, file, numLoops)
	} else {
		err = f.oneRequestFetchWrite(ctx, desc, file)
	}

	if err != nil {
		// Check if file still exists after write error
		if statInfo, statErr := os.Stat(ingestPath); statErr != nil {
			logger.WithError(statErr).Error("file does not exist after write error")
		} else {
			logger.WithField("size", statInfo.Size()).Debug("file exists after write error")
		}
		return nil, fmt.Errorf("error writing to temp ingest file at %s: %w", ingestPath, err)
	}

	logger.Debug("fetch write completed, checking file state before sync")

	// Check file state before sync
	if statInfo, statErr := os.Stat(ingestPath); statErr != nil {
		logger.WithError(statErr).Error("file does not exist after write but before sync")
		// Also check parent directory
		if _, parentErr := os.Stat(parentDir); parentErr != nil {
			logger.WithError(parentErr).Error("parent directory also does not exist")
		} else {
			// List what's in parent directory
			if entries, listErr := os.ReadDir(parentDir); listErr == nil {
				entryNames := make([]string, len(entries))
				for i, e := range entries {
					entryNames[i] = e.Name()
				}
				logger.WithField("parentContents", entryNames).Error("parent directory contents at time of disappearance")
			}
		}
		return nil, fmt.Errorf("file disappeared before sync at %s: %w", ingestPath, statErr)
	} else {
		logger.WithField("size", statInfo.Size()).Debug("file exists before sync")
	}

	// Sync file to disk before verification
	logger.Debug("starting file sync")
	if err = file.Sync(); err != nil {
		return nil, fmt.Errorf("error syncing file at %s: %w", ingestPath, err)
	}
	logger.Debug("file sync completed")

	n, err := file.Seek(0, io.SeekStart)
	if n != 0 {
		err = errors.Join(err, errors.New("seek position != zero"))
	}
	if err != nil {
		return nil, fmt.Errorf("error reopening file at %s after writing: %w", ingestPath, err)
	}
	logger.Debug("file seeked to start")

	// Verify the file exists before async verification
	if statInfo, statErr := os.Stat(ingestPath); statErr != nil {
		logger.WithError(statErr).Error("file does not exist after sync - possible race condition")
		// Also check parent directory
		if _, parentErr := os.Stat(parentDir); parentErr != nil {
			logger.WithError(parentErr).Error("parent directory also does not exist after sync")
		} else {
			// List what's in parent directory
			if entries, listErr := os.ReadDir(parentDir); listErr == nil {
				entryNames := make([]string, len(entries))
				for i, e := range entries {
					entryNames[i] = e.Name()
				}
				logger.WithField("parentContents", entryNames).Error("parent directory contents at time of disappearance")
			}
		}
		return nil, fmt.Errorf("file disappeared after write at %s: %w", ingestPath, statErr)
	} else {
		logger.WithField("sizeAfterSync", statInfo.Size()).Debug("file exists after sync, proceeding to verification")
	}

	// start async verification of the blob digest
	if err = f.asyncVerifyBlobDigest(ctx, ingestPath); err != nil {
		return nil, fmt.Errorf("error starting async verification of blob digest: %w", err)
	}

	return file, nil
}

// oneRequestFetchWrite does a normal fetch for the content from the repo and writes it to the given file descriptor
func (f *parallelArtifactFetcher) oneRequestFetchWrite(ctx context.Context, desc ocispec.Descriptor, file *os.File) error {
	err := f.layerUnpackJob.AcquireDownload(ctx, 1)
	if err != nil {
		return fmt.Errorf("error acquiring semaphore: %w", err)
	}
	defer f.layerUnpackJob.ReleaseDownload(1)

	rc, err := f.remoteStore.Fetch(ctx, desc)
	if err != nil {
		return fmt.Errorf("error fetching from remote: %w", err)
	}
	return writeToEntireFile(file, rc)
}

// multiRequestFetchWrite will make parallel calls to the upstream repo for chunks of the file and buffer it from the given file descriptor.
// This assumes that we will be fetching multiple chunks of the file and that we are using our own blob store implementation.
func (f *parallelArtifactFetcher) multiRequestFetchWrite(ctx context.Context, desc ocispec.Descriptor, file *os.File, numLoops int64) error {
	eg, egCtx := errgroup.WithContext(ctx)
	reference := f.constructRef(desc)

	blobStore, ok := f.remoteStore.(*orasBlobStore)
	if !ok {
		return errors.New("did not pass orasBlobStore type to parallel fetcher")
	}

	for i := range numLoops {
		semAcquireStart := time.Now()
		err := f.layerUnpackJob.AcquireDownload(ctx, 1)
		if err != nil {
			return fmt.Errorf("error acquiring semaphore: %w", err)
		}
		semWaitMs := time.Since(semAcquireStart).Milliseconds()

		eg.Go(func() error {
			lower, upper := f.getRange(i, desc.Size)
			chunkSize := upper - lower + 1

			// Time the HTTP fetch
			fetchStart := time.Now()
			rc, err := blobStore.FetchRange(egCtx, reference, lower, upper)
			fetchMs := time.Since(fetchStart).Milliseconds()

			// Release semaphore immediately after HTTP fetch completes.
			// This allows other downloads to start while we write to disk.
			f.layerUnpackJob.ReleaseDownload(1)

			if err != nil {
				return err
			}

			// Time the disk write (no longer holding download semaphore)
			writeStart := time.Now()
			errCh := make(chan error, 1)
			defer close(errCh)

			copyFunc := func() <-chan error {
				defer rc.Close()

				errCh <- writeToFileRange(file, rc, lower, upper)
				return errCh
			}

			select {
			case <-egCtx.Done():
				err = egCtx.Err()
			case err = <-copyFunc():
			}
			writeMs := time.Since(writeStart).Milliseconds()

			// Log timing for chunks that took longer than expected
			totalMs := fetchMs + writeMs
			if semWaitMs > 100 || totalMs > 500 {
				log.G(egCtx).WithFields(log.Fields{
					"chunk":       i,
					"chunkSize":   chunkSize,
					"semWait_ms":  semWaitMs,
					"fetch_ms":    fetchMs,
					"write_ms":    writeMs,
					"total_ms":    totalMs,
					"offset":      lower,
				}).Info("chunk download timing (slow)")
			}

			return err
		})
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- eg.Wait()
		close(errCh)
	}()

	var err error
	select {
	case <-ctx.Done():
		err = ctx.Err()
	case err = <-errCh:
	}
	return err
}

func (f *parallelArtifactFetcher) asyncVerifyBlobDigest(ctx context.Context, path string) error {
	// Debug: check if directory exists
	dir := filepath.Dir(path)
	if _, dirErr := os.Stat(dir); dirErr != nil {
		log.G(ctx).WithError(dirErr).WithField("dir", dir).Error("parent directory does not exist for digest verification")
	}

	file, err := os.Open(path)
	if err != nil {
		log.G(ctx).WithError(err).WithFields(log.Fields{
			"path": path,
			"dir":  dir,
		}).Error("failed to open file for digest verification")
		return err
	}
	f.verifier.AsyncVerify(file)
	return err
}

func writeToEntireFile(file *os.File, rc io.ReadCloser) error {
	_, err := io.Copy(file, rc)
	if err != nil {
		return fmt.Errorf("failed to write to temp file %s: %w", file.Name(), err)
	}

	return nil
}

func writeToFileRange(file *os.File, rsc io.ReadCloser, lower, upper int64) error {
	w := io.NewOffsetWriter(file, lower)

	readSize := upper - lower + 1
	// Reading any more or less will result in an incorrect file being created,
	// so use io.CopyN to guarantee we read exactly lower - upper + 1 bytes.
	_, err := io.CopyN(w, rsc, readSize)
	io.Copy(io.Discard, rsc) // Drain remaining data
	if err != nil {
		return fmt.Errorf("failed to write to temp file %s at offset %d: %w", file.Name(), lower, err)
	}

	return nil
}
