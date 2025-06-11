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
	"sync"
	"sync/atomic"

	"github.com/awslabs/soci-snapshotter/soci/store"
	"github.com/containerd/containerd/reference"
	"github.com/containerd/log"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"golang.org/x/sync/errgroup"
)

// artifactFetcher is responsible for fetching and storing artifacts in the provided artifact store.
type parallelArtifactFetcher struct {
	*artifactFetcher
	layerUnpackJob *layerUnpackJob
	chunkSize      int64
}

// Constructs a new artifact fetcher
// Takes in the image reference, the local store and the resolver
func newParallelArtifactFetcher(refspec reference.Spec, localStore store.BasicStore, remoteStore resolverStorage, layerUnpackJob *layerUnpackJob, chunkSize int64) (*parallelArtifactFetcher, error) {
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
		return rc, true, nil
	}

	log.G(ctx).WithField("digest", desc.Digest.String()).Infof("fetching artifact from remote")
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

	// Refuse to unpack if file already exists
	_, err := os.Stat(ingestPath)
	if err == nil {
		err = fs.ErrExist
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("error setting up temporary file: %w", err)
	}

	file, err := os.Create(ingestPath)
	if err != nil {
		return nil, fmt.Errorf("error creating temp ingest file at %s: %w", ingestPath, err)
	}

	// We want to close this file descriptor as soon as we run into an error.
	// This can either be during the goroutine or if the context is cancelled.
	// Closing a file descriptor multiple times is undefined behavior,
	// so this will enforce that the file is only closed once.
	closeFileOnce := sync.OnceFunc(func() {
		file.Close()
	})
	defer func() {
		if err != nil {
			closeFileOnce()
		}
	}()

	err = file.Truncate(desc.Size)
	if err != nil {
		return nil, fmt.Errorf("error truncating temp ingest file at %s: %w", ingestPath, err)
	}

	eg, egCtx := errgroup.WithContext(ctx)
	var isSeekable atomic.Bool
	isSeekable.Store(true)
	numLoops := f.calcNumLoops(desc.Size)

	for i := range numLoops {
		if !isSeekable.Load() {
			// This val only gets set after the whole readcloser has been
			// written to file, so safe to stop attempting this again.
			break
		}

		err := f.layerUnpackJob.AcquireDownload(ctx, 1)
		if err != nil {
			return nil, fmt.Errorf("error acquiring semaphore: %w", err)
		}

		eg.Go(func() error {
			defer f.layerUnpackJob.ReleaseDownload(1)

			var err error
			defer func() {
				if err != nil {
					closeFileOnce()
				}
			}()

			rc, err := f.remoteStore.Fetch(egCtx, desc)
			if err != nil {
				return err
			}

			errCh := make(chan error, 1)
			defer close(errCh)
			var copyFunc func() <-chan error

			rsc, ok := rc.(io.ReadSeekCloser)
			if numLoops == 1 {
				copyFunc = func() <-chan error {
					defer rc.Close()
					errCh <- writeToEntireFile(file, rc)
					return errCh
				}
			} else if !ok { // Upstream reader is not seekable
				log.G(egCtx).Debug("upstream reader is not seekable, reading entire descriptor")
				copyFunc = func() <-chan error {
					defer rc.Close()
					if !isSeekable.CompareAndSwap(true, false) {
						errCh <- nil
					} else {
						errCh <- writeToEntireFile(file, rc)
					}
					return errCh
				}
			} else {
				copyFunc = func() <-chan error {
					defer rsc.Close()

					lower, upper := f.getRange(i, desc.Size)
					errCh <- writeToFileRange(file, rsc, lower, upper)
					return errCh
				}
			}

			select {
			case <-egCtx.Done():
				err = egCtx.Err()
			case err = <-copyFunc():
			}
			return err
		})
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- eg.Wait()
		close(errCh)
	}()

	select {
	case <-ctx.Done():
		err = ctx.Err()
	case err = <-errCh:
	}

	if err != nil {
		return nil, fmt.Errorf("error writing to temp ingest file at %s: %w", ingestPath, err)
	}

	n, err := file.Seek(0, io.SeekStart)
	if n != 0 {
		err = errors.Join(err, errors.New("seek position != zero"))
	}
	if err != nil {
		return nil, fmt.Errorf("error reopening file at %s after writing: %w", ingestPath, err)
	}
	return file, nil
}

func writeToEntireFile(file *os.File, rc io.ReadCloser) error {
	_, err := io.Copy(file, rc)
	if err != nil {
		return fmt.Errorf("failed to write to temp file %s: %w", file.Name(), err)
	}

	return nil
}

func writeToFileRange(file *os.File, rsc io.ReadSeekCloser, lower, upper int64) error {
	w := io.NewOffsetWriter(file, lower)

	_, err := rsc.Seek(lower, io.SeekStart)
	if err != nil {
		return fmt.Errorf("failed to seek readcloser: %w", err)
	}

	readSize := upper - lower + 1
	// Reading any more or less will result in an incorrect file being created,
	// so use io.CopyN to guarantee we read exactly lower - upper + 1 bytes.
	_, err = io.CopyN(w, rsc, readSize)
	if err != nil {
		return fmt.Errorf("failed to write to temp file %s at offset %d: %w", file.Name(), lower, err)
	}

	return nil
}
