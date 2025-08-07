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
	"strings"
	"sync"

	socicompression "github.com/awslabs/soci-snapshotter/internal/archive/compression"
	"github.com/containerd/containerd/archive"
	"github.com/containerd/containerd/archive/compression"
	"github.com/containerd/containerd/mount"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type Unpacker interface {
	// Unpack takes care of getting the layer specified by descriptor `desc`,
	// decompressing it, putting it in the directory with the path `mountpoint`
	// and applying the difference to the parent layers if there is any.
	// After that the layer can be mounted as non-remote snapshot.
	Unpack(ctx context.Context, desc ocispec.Descriptor, mountpoint string, mounts []mount.Mount) error
}

type Archive interface {
	// Apply decompresses the compressed stream represented by reader `r` and
	// applies it to the directory `root`.
	Apply(ctx context.Context, root string, r io.Reader, opts ...archive.ApplyOpt) (int64, error)
}

type layerArchive struct {
	compressed       *asyncVerifier
	uncompressed     *asyncVerifier
	decompressStream socicompression.DecompressStream
	bufferPool       *bufferPool
}

type asyncVerifier struct {
	v       digest.Verifier
	waitCh  chan struct{}
	started bool
}

func newAsyncVerifier(v digest.Verifier) *asyncVerifier {
	return &asyncVerifier{
		v:       v,
		waitCh:  make(chan struct{}, 1),
		started: false,
	}
}

func (av *asyncVerifier) AsyncVerify(reader io.ReadCloser) {
	if av == nil || av.v == nil {
		reader.Close()
		return
	}
	go func() {
		io.Copy(av.v, reader)
		close(av.waitCh)
		reader.Close()
	}()
	av.started = true
}

func (av *asyncVerifier) Verified(ctx context.Context) bool {
	if av == nil || av.v == nil {
		return true
	}
	if !av.started {
		return false
	}
	select {
	case <-ctx.Done():
		return false
	case <-av.waitCh:
		return av.v.Verified()
	}
}

type bufferPool struct {
	pool *sync.Pool
}

func newbufferPool(size int64) *bufferPool {
	pool := &sync.Pool{
		New: func() any {
			buffer := make([]byte, size)
			return &buffer
		},
	}
	return &bufferPool{
		pool: pool,
	}
}

func (p *bufferPool) Get() *[]byte {
	buf := p.pool.Get().(*[]byte)
	return buf
}

func (p *bufferPool) Put(buffer *[]byte) {
	p.pool.Put(buffer)
}

func NewLayerArchive(compressedVerifier, uncompressedVerifier *asyncVerifier, decompressStream socicompression.DecompressStream, bufPool *bufferPool) Archive {
	// If no layer decompress stream was provided, then use containerd's decompress stream implementation.
	if decompressStream == nil {
		decompressStream = compression.DecompressStream
	}
	if bufPool == nil {
		bufPool = newbufferPool(64 * 1024)
	}
	return &layerArchive{
		compressed:       compressedVerifier,
		uncompressed:     uncompressedVerifier,
		decompressStream: decompressStream,
		bufferPool:       bufPool,
	}
}

func AsyncTeeReader(r io.Reader, w io.WriteCloser, bp *bufferPool) io.Reader {
	type chunk struct {
		data *[]byte
		n    int
	}

	var (
		ch     = make(chan *chunk, 16)
		pr, pw = io.Pipe()
	)

	// writer goroutine writing chunks to w
	go func() {
		for c := range ch {
			w.Write((*c.data)[:c.n])
			bp.Put(c.data)
		}
		w.Close()
	}()

	// reader goroutine reading from r and writing to pw
	go func() {
		defer close(ch)
		for {
			b := bp.Get()
			n, err := r.Read(*b)
			if n > 0 {
				c := &chunk{data: bp.Get(), n: n}
				copy((*c.data), (*b)[:n])
				ch <- c
				_, err = pw.Write((*b)[:n])
			}
			if err != nil {
				if err != io.EOF {
					pw.CloseWithError(err)
				} else {
					pw.Close()
				}
				bp.Put(b)
				return
			}
			bp.Put(b)
		}
	}()

	return pr
}

func (la *layerArchive) Apply(ctx context.Context, root string, r io.Reader, opts ...archive.ApplyOpt) (int64, error) {
	// Decompress first, then apply.
	decompressReader, err := la.decompressStream(r)
	if err != nil {
		return 0, fmt.Errorf("cannot decompress the stream: %w", err)
	}

	var (
		reader io.Reader = decompressReader
	)
	drain := func() {
		// Read any trailing data to ensure digest validation
		io.Copy(io.Discard, reader)
		decompressReader.Close()
	}

	if la.uncompressed != nil {
		pr, pw := io.Pipe()
		// Benchmark suggests that we should give the larger buffer to the faster consumer.
		// Digest calculation is faster than untar operation.
		reader = AsyncTeeReader(decompressReader, pw, la.bufferPool)
		la.uncompressed.AsyncVerify(pr)
	}

	n, err := archive.Apply(ctx, root, reader, opts...)
	drain()
	if err != nil {
		return 0, err
	}

	if !la.uncompressed.Verified(ctx) {
		return 0, errors.New("uncompressed digests did not match")
	}
	if !la.compressed.Verified(ctx) {
		return 0, errors.New("compressed digests did not match")
	}

	return n, nil
}

type layerUnpacker struct {
	fetcher Fetcher
	archive Archive
}

func NewLayerUnpacker(fetcher Fetcher, archive Archive) Unpacker {
	return &layerUnpacker{
		fetcher: fetcher,
		archive: archive,
	}
}

func (lu *layerUnpacker) Unpack(ctx context.Context, desc ocispec.Descriptor, mountpoint string, mounts []mount.Mount) error {
	rc, local, err := lu.fetcher.Fetch(ctx, desc)
	if err != nil {
		return fmt.Errorf("cannot fetch layer: %w", err)
	}

	if !local {
		err := lu.fetcher.Store(ctx, desc, rc)
		rc.Close()
		if err != nil {
			return fmt.Errorf("cannot store layer: %w", err)
		}
		rc, _, err = lu.fetcher.Fetch(ctx, desc)
		if err != nil {
			return fmt.Errorf("cannot fetch layer: %w", err)
		}
	}
	defer rc.Close()

	opts := []archive.ApplyOpt{
		archive.WithConvertWhiteout(archive.OverlayConvertWhiteout),
	}
	if len(mounts) > 0 {
		if parents := getLayerParents(mounts[0].Options); len(parents) > 0 {
			opts = append(opts, archive.WithParents(parents))
		}
	}
	_, err = lu.archive.Apply(ctx, mountpoint, rc, opts...)
	if err != nil {
		return fmt.Errorf("cannot apply layer: %w", err)
	}

	return nil
}

func getLayerParents(options []string) (lower []string) {
	const lowerdirPrefix = "lowerdir="

	for _, o := range options {
		if strings.HasPrefix(o, lowerdirPrefix) {
			lower = strings.Split(strings.TrimPrefix(o, lowerdirPrefix), ":")
		}
	}
	return
}
