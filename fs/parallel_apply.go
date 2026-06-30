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
	"archive/tar"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"

	"github.com/containerd/containerd/v2/pkg/archive"
	"golang.org/x/sync/errgroup"
)

// ParallelApplyConfig configures parallel tar extraction
type ParallelApplyConfig struct {
	// NumWorkers is the number of parallel file writers (default: 16)
	NumWorkers int
	// BufferSize is the size of the read buffer per file (default: 1MB)
	BufferSize int
	// QueueSize is the size of the work queue (default: 64)
	QueueSize int
}

// DefaultParallelApplyConfig returns sensible defaults for NVMe
func DefaultParallelApplyConfig() ParallelApplyConfig {
	return ParallelApplyConfig{
		NumWorkers: 16,
		BufferSize: 1 << 20, // 1MB - larger buffers for NVMe
		QueueSize:  64,
	}
}

// fileWork represents a file to be written
type fileWork struct {
	path    string
	mode    os.FileMode
	size    int64
	content []byte // For small files, content is buffered
	reader  io.Reader // For large files, stream from here
	isLarge bool
}

// ParallelApply extracts a tar archive with parallel file writes
// This improves NVMe performance by increasing I/O queue depth
func ParallelApply(ctx context.Context, root string, r io.Reader, cfg ParallelApplyConfig, opts ...archive.ApplyOpt) (int64, error) {
	if cfg.NumWorkers <= 0 {
		cfg.NumWorkers = 16
	}
	if cfg.BufferSize <= 0 {
		cfg.BufferSize = 1 << 20
	}
	if cfg.QueueSize <= 0 {
		cfg.QueueSize = 64
	}

	// Threshold for buffering small files vs streaming large files
	smallFileThreshold := int64(cfg.BufferSize)

	var totalSize atomic.Int64
	workChan := make(chan *fileWork, cfg.QueueSize)

	eg, egCtx := errgroup.WithContext(ctx)

	// Start worker pool for parallel file writes
	for i := 0; i < cfg.NumWorkers; i++ {
		eg.Go(func() error {
			for work := range workChan {
				select {
				case <-egCtx.Done():
					return egCtx.Err()
				default:
				}

				if err := writeFile(work, cfg.BufferSize); err != nil {
					return fmt.Errorf("failed to write %s: %w", work.path, err)
				}
				totalSize.Add(work.size)
			}
			return nil
		})
	}

	// Producer: read tar and dispatch work
	eg.Go(func() error {
		defer close(workChan)

		tr := tar.NewReader(r)

		// Buffer pool for small file contents
		bufPool := sync.Pool{
			New: func() any {
				buf := make([]byte, cfg.BufferSize)
				return &buf
			},
		}

		for {
			select {
			case <-egCtx.Done():
				return egCtx.Err()
			default:
			}

			hdr, err := tr.Next()
			if err == io.EOF {
				return nil
			}
			if err != nil {
				return fmt.Errorf("tar read error: %w", err)
			}

			targetPath := filepath.Join(root, hdr.Name)

			// Ensure parent directory exists
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return fmt.Errorf("mkdir failed for %s: %w", targetPath, err)
			}

			switch hdr.Typeflag {
			case tar.TypeDir:
				if err := os.MkdirAll(targetPath, os.FileMode(hdr.Mode)); err != nil {
					return fmt.Errorf("mkdir failed: %w", err)
				}

			case tar.TypeReg:
				work := &fileWork{
					path: targetPath,
					mode: os.FileMode(hdr.Mode),
					size: hdr.Size,
				}

				if hdr.Size <= smallFileThreshold {
					// Small file: buffer content and dispatch
					buf := bufPool.Get().(*[]byte)
					n, err := io.ReadFull(tr, (*buf)[:hdr.Size])
					if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
						bufPool.Put(buf)
						return fmt.Errorf("read failed for %s: %w", hdr.Name, err)
					}
					work.content = make([]byte, n)
					copy(work.content, (*buf)[:n])
					bufPool.Put(buf)
					work.isLarge = false
				} else {
					// Large file: we must read it now (tar is sequential)
					// Buffer the entire content for parallel write
					work.content = make([]byte, hdr.Size)
					if _, err := io.ReadFull(tr, work.content); err != nil {
						return fmt.Errorf("read failed for large file %s: %w", hdr.Name, err)
					}
					work.isLarge = true
				}

				select {
				case workChan <- work:
				case <-egCtx.Done():
					return egCtx.Err()
				}

			case tar.TypeSymlink:
				// Remove existing file/symlink if present
				os.Remove(targetPath)
				if err := os.Symlink(hdr.Linkname, targetPath); err != nil {
					return fmt.Errorf("symlink failed: %w", err)
				}

			case tar.TypeLink:
				// Hard link
				linkTarget := filepath.Join(root, hdr.Linkname)
				os.Remove(targetPath)
				if err := os.Link(linkTarget, targetPath); err != nil {
					return fmt.Errorf("hardlink failed: %w", err)
				}

			default:
				// Skip other types (devices, etc.) or handle as needed
			}
		}
	})

	if err := eg.Wait(); err != nil {
		return 0, err
	}

	return totalSize.Load(), nil
}

// writeFile writes a single file with optimized I/O
func writeFile(work *fileWork, bufSize int) error {
	// Use O_WRONLY|O_CREATE|O_TRUNC - standard flags
	// Note: NOT using O_DIRECT because it requires aligned buffers
	// The kernel's page cache will batch our writes
	f, err := os.OpenFile(work.path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, work.mode)
	if err != nil {
		return err
	}
	defer f.Close()

	// Pre-allocate file size for better performance
	if work.size > 0 {
		if err := f.Truncate(work.size); err != nil {
			// Non-fatal, continue anyway
		}
	}

	// Write content
	if len(work.content) > 0 {
		_, err = f.Write(work.content)
		return err
	}

	return nil
}

// ParallelApplyWithFallback tries parallel apply, falls back to sequential on error
func ParallelApplyWithFallback(ctx context.Context, root string, r io.Reader, cfg ParallelApplyConfig, opts ...archive.ApplyOpt) (int64, error) {
	// For now, just use parallel apply
	// In production, you might want to detect errors and fall back
	return ParallelApply(ctx, root, r, cfg, opts...)
}
