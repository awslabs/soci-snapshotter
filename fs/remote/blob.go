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

package remote

import (
	"context"
	"fmt"
	"io"
	"regexp"

	"sync"
	"time"

	"github.com/awslabs/soci-snapshotter/fs/source"
	"github.com/containerd/containerd/reference"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

var contentRangeRegexp = regexp.MustCompile(`bytes ([0-9]+)-([0-9]+)/([0-9]+|\\*)`)

type Blob interface {
	Check() error
	Size() int64
	FetchedSize() int64
	ReadAt(p []byte, offset int64, opts ...Option) (int, error)
	Refresh(ctx context.Context, host source.RegistryHosts, refspec reference.Spec, desc ocispec.Descriptor) error
	Close() error
}

type blob struct {
	fetcher   fetcher
	fetcherMu sync.Mutex

	size          int64
	lastCheck     time.Time
	lastCheckMu   sync.Mutex
	checkInterval time.Duration
	fetchTimeout  time.Duration

	fetchedRegionSet   regionSet
	fetchedRegionSetMu sync.Mutex

	resolver *Resolver

	closed   bool
	closedMu sync.Mutex
}

func makeBlob(fetcher fetcher, size int64, lastCheck time.Time, checkInterval time.Duration,
	r *Resolver, fetchTimeout time.Duration) *blob {
	return &blob{
		fetcher:       fetcher,
		size:          size,
		lastCheck:     lastCheck,
		checkInterval: checkInterval,
		resolver:      r,
		fetchTimeout:  fetchTimeout,
	}
}

func (b *blob) Close() error {
	b.closedMu.Lock()
	defer b.closedMu.Unlock()
	if !b.closed {
		b.closed = true
	}
	return nil
}

func (b *blob) isClosed() bool {
	b.closedMu.Lock()
	closed := b.closed
	b.closedMu.Unlock()
	return closed
}

func (b *blob) Refresh(ctx context.Context, hosts source.RegistryHosts, refspec reference.Spec, desc ocispec.Descriptor) error {
	if b.isClosed() {
		return fmt.Errorf("blob is already closed")
	}

	// refresh the fetcher
	f, newSize, err := b.resolver.resolveFetcher(ctx, hosts, refspec, desc)
	if err != nil {
		return err
	}
	if newSize != b.size {
		return fmt.Errorf("Invalid size of new blob %d; want %d", newSize, b.size)
	}

	// update the blob's fetcher with new one
	b.fetcherMu.Lock()
	b.fetcher = f
	b.fetcherMu.Unlock()
	b.lastCheckMu.Lock()
	b.lastCheck = time.Now()
	b.lastCheckMu.Unlock()

	return nil
}

func (b *blob) Check() error {
	if b.isClosed() {
		return fmt.Errorf("blob is already closed")
	}

	now := time.Now()
	b.lastCheckMu.Lock()
	lastCheck := b.lastCheck
	b.lastCheckMu.Unlock()
	if now.Sub(lastCheck) < b.checkInterval {
		// do nothing if not expired
		return nil
	}
	b.fetcherMu.Lock()
	fr := b.fetcher
	b.fetcherMu.Unlock()
	err := fr.check()
	if err == nil {
		// update lastCheck only if check succeeded.
		// on failure, we should check this layer next time again.
		b.lastCheckMu.Lock()
		b.lastCheck = now
		b.lastCheckMu.Unlock()
	}

	return err
}

func (b *blob) Size() int64 {
	return b.size
}

func (b *blob) FetchedSize() int64 {
	b.fetchedRegionSetMu.Lock()
	sz := b.fetchedRegionSet.totalSize()
	b.fetchedRegionSetMu.Unlock()
	return sz
}

// ReadAt reads remote blob from specified offset for the buffer size.
// We can configure this function with options.
func (b *blob) ReadAt(p []byte, offset int64, opts ...Option) (int, error) {
	if b.isClosed() {
		return 0, fmt.Errorf("blob is already closed")
	}

	if len(p) == 0 || offset > b.size {
		return 0, nil
	}

	reg := region{offset, offset + int64(len(p)) - 1}

	var readAtOpts options
	for _, o := range opts {
		o(&readAtOpts)
	}

	// Take it from remote registry.
	w := newBytesWriter(p, 0)

	// Read required data
	if err := b.fetchRange(reg, w, &readAtOpts); err != nil {
		return 0, err
	}

	// Adjust the buffer size according to the blob size
	if remain := b.size - offset; int64(len(p)) >= remain {
		if remain < 0 {
			remain = 0
		}
		p = p[:remain]
	}

	return len(p), nil
}

// fetchRegion fetches content from remote blob.
// It must be called from within fetchRange and need to ensure that it is inside the singleflight `Do` operation.
func (b *blob) fetchRegion(reg region, w io.Writer, fetched bool, opts *options) error {
	// Fetcher can be suddenly updated so we take and use the snapshot of it for
	// consistency.
	b.fetcherMu.Lock()
	fr := b.fetcher
	b.fetcherMu.Unlock()

	fetchCtx, cancel := context.WithTimeout(context.Background(), b.fetchTimeout)
	defer cancel()
	if opts.ctx != nil {
		fetchCtx = opts.ctx
	}

	var req []region
	req = append(req, reg)
	mr, err := fr.fetch(fetchCtx, req, true)

	if err != nil {
		return err
	}
	defer mr.Close()

	// Update the check timer because we succeeded to access the blob
	b.lastCheckMu.Lock()
	b.lastCheck = time.Now()
	b.lastCheckMu.Unlock()

	for {
		_, p, err := mr.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			return errors.Wrapf(err, "failed to read multipart resp")
		}

		if _, err := io.CopyN(w, p, reg.size()); err != nil {
			return err
		}

		b.fetchedRegionSetMu.Lock()
		b.fetchedRegionSet.add(reg)
		b.fetchedRegionSetMu.Unlock()
		fetched = true
	}

	if !fetched {
		return fmt.Errorf("failed to fetch region %v", reg)
	}

	return nil
}

// fetchRange fetches content from remote blob.
func (b *blob) fetchRange(reg region, w io.Writer, opts *options) error {
	return b.fetchRegion(reg, w, false, opts)
}

func newBytesWriter(dest []byte, destOff int64) io.Writer {
	return &bytesWriter{
		dest:    dest,
		destOff: destOff,
		current: 0,
	}
}

type bytesWriter struct {
	dest    []byte
	destOff int64
	current int64
}

func (bw *bytesWriter) Write(p []byte) (int, error) {
	defer func() { bw.current = bw.current + int64(len(p)) }()

	var (
		destBase = positive(bw.current - bw.destOff)
		pBegin   = positive(bw.destOff - bw.current)
		pEnd     = positive(bw.destOff + int64(len(bw.dest)) - bw.current)
	)

	if destBase > int64(len(bw.dest)) {
		return len(p), nil
	}
	if pBegin >= int64(len(p)) {
		return len(p), nil
	}
	if pEnd > int64(len(p)) {
		pEnd = int64(len(p))
	}

	copy(bw.dest[destBase:], p[pBegin:pEnd])

	return len(p), nil
}

func positive(n int64) int64 {
	if n < 0 {
		return 0
	}
	return n
}
