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

package reader

import (
	"archive/tar"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"github.com/awslabs/soci-snapshotter/cache"
	commonmetrics "github.com/awslabs/soci-snapshotter/fs/metrics/common"
	spanmanager "github.com/awslabs/soci-snapshotter/fs/span-manager"
	"github.com/awslabs/soci-snapshotter/metadata"
	"github.com/awslabs/soci-snapshotter/util/ioutils"
	"github.com/awslabs/soci-snapshotter/ztoc"
	"github.com/awslabs/soci-snapshotter/ztoc/compression"
	digest "github.com/opencontainers/go-digest"
)

type Reader interface {
	OpenFile(id uint32) (io.ReaderAt, error)
	Metadata() metadata.Reader
	Close() error
	LastOnDemandReadTime() time.Time
}

// VerifiableReader produces a Reader with a given verifier.
type VerifiableReader struct {
	r *reader

	lastVerifyErr           atomic.Value
	prohibitVerifyFailure   bool
	prohibitVerifyFailureMu sync.RWMutex

	closed   bool
	closedMu sync.Mutex

	verifier func(uint32, string) (digest.Verifier, error)
}

func (vr *VerifiableReader) SkipVerify() Reader {
	return vr.r
}

func (vr *VerifiableReader) VerifyTOC(tocDigest digest.Digest) (Reader, error) {
	if vr.isClosed() {
		return nil, fmt.Errorf("reader is already closed")
	}
	vr.prohibitVerifyFailureMu.Lock()
	vr.prohibitVerifyFailure = true
	lastVerifyErr := vr.lastVerifyErr.Load()
	vr.prohibitVerifyFailureMu.Unlock()
	if err := lastVerifyErr; err != nil {
		return nil, fmt.Errorf("content error occures during caching contents: %w", err.(error))
	}
	vr.r.verify = true
	return vr.r, nil
}

// nolint:revive
func (vr *VerifiableReader) GetReader() *reader {
	return vr.r
}

func (vr *VerifiableReader) Metadata() metadata.Reader {
	// TODO: this shouldn't be called before verified
	return vr.r.r
}

func (vr *VerifiableReader) Close() error {
	vr.closedMu.Lock()
	defer vr.closedMu.Unlock()
	if vr.closed {
		return nil
	}
	vr.closed = true
	return vr.r.Close()
}

func (vr *VerifiableReader) isClosed() bool {
	vr.closedMu.Lock()
	closed := vr.closed
	vr.closedMu.Unlock()
	return closed
}

// NewReader creates a Reader based on the given soci blob and Span Manager.
func NewReader(r metadata.Reader, layerSha digest.Digest, spanManager *spanmanager.SpanManager, disableVerification bool) (*VerifiableReader, error) {
	vr := &reader{
		spanManager:         spanManager,
		r:                   r,
		layerSha:            layerSha,
		verifier:            digestVerifier,
		disableVerification: disableVerification,
	}
	return &VerifiableReader{r: vr, verifier: digestVerifier}, nil
}

type reader struct {
	spanManager *spanmanager.SpanManager
	r           metadata.Reader
	layerSha    digest.Digest

	lastReadTime   time.Time
	lastReadTimeMu sync.Mutex

	closed   bool
	closedMu sync.Mutex

	verify              bool
	verifier            func(uint32, string) (digest.Verifier, error)
	disableVerification bool
}

func (gr *reader) Metadata() metadata.Reader {
	return gr.r
}

func (gr *reader) setLastReadTime(lastReadTime time.Time) {
	gr.lastReadTimeMu.Lock()
	gr.lastReadTime = lastReadTime
	gr.lastReadTimeMu.Unlock()
}

func (gr *reader) LastOnDemandReadTime() time.Time {
	gr.lastReadTimeMu.Lock()
	t := gr.lastReadTime
	gr.lastReadTimeMu.Unlock()
	return t
}

func (gr *reader) OpenFile(id uint32) (io.ReaderAt, error) {
	if gr.isClosed() {
		return nil, fmt.Errorf("reader is already closed")
	}
	var fr metadata.File
	fr, err := gr.r.OpenFile(id)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %d: %w", id, err)
	}
	return &file{
		id: id,
		fr: fr,
		gr: gr,
	}, nil
}

func (gr *reader) Close() (retErr error) {
	gr.closedMu.Lock()
	defer gr.closedMu.Unlock()
	if gr.closed {
		return nil
	}
	gr.closed = true
	if err := gr.r.Close(); err != nil {
		retErr = errors.Join(retErr, err)
	}
	return
}

func (gr *reader) isClosed() bool {
	gr.closedMu.Lock()
	closed := gr.closed
	gr.closedMu.Unlock()
	return closed
}

type file struct {
	id       uint32
	fr       metadata.File
	gr       *reader
	verified atomic.Bool
	lock     sync.Mutex
}

// ReadAt reads the file when the file is requested by the container
func (sf *file) ReadAt(p []byte, offset int64) (int, error) {
	if !sf.gr.disableVerification {
		if err := sf.Verify(); err != nil {
			return 0, err
		}
	}
	if len(p) == 0 {
		return 0, nil
	}
	uncompFileSize := sf.fr.GetUncompressedFileSize()
	if compression.Offset(offset) >= uncompFileSize {
		return 0, io.EOF
	}
	expectedSize := uncompFileSize - compression.Offset(offset)
	if expectedSize > compression.Offset(len(p)) {
		expectedSize = compression.Offset(len(p))
	}
	fileOffsetStart := sf.fr.GetUncompressedOffset() + compression.Offset(offset)
	fileOffsetEnd := fileOffsetStart + expectedSize
	r, err := sf.gr.spanManager.GetContents(fileOffsetStart, fileOffsetEnd)
	if err != nil {
		return 0, fmt.Errorf("failed to read the file: %w", err)
	}
	defer r.Close()

	// TODO this is not the right place for this metric to be. It needs to go down the BlobReader, when the HTTP request is issued
	commonmetrics.IncOperationCount(commonmetrics.SynchronousReadRegistryFetchCount, sf.gr.layerSha) // increment the number of on demand file fetches from remote registry
	sf.gr.setLastReadTime(time.Now())

	n, err := io.ReadFull(r, p[0:expectedSize])
	if err != nil {
		return 0, fmt.Errorf("unexpected copied data size for on-demand fetch. read = %d, expected = %d", n, expectedSize)
	}

	commonmetrics.AddBytesCount(commonmetrics.SynchronousBytesServed, sf.gr.layerSha, int64(n)) // measure the number of bytes served synchronously

	return n, nil
}

// Verify verifies that the file's attributes match the tar header in the image layer
func (sf *file) Verify() (retErr error) {
	if sf.verified.Load() {
		return nil
	}
	sf.lock.Lock()
	defer sf.lock.Unlock()
	if sf.verified.Load() {
		return nil
	}
	defer func() {
		if retErr == nil {
			sf.verified.Store(true)
		}
	}()

	attr, err := sf.gr.r.GetAttr(sf.id)
	if err != nil {
		return err
	}

	tarHeaderOffset := sf.fr.TarHeaderOffset()
	tarHeaderSize := sf.fr.TarHeaderSize()
	if sf.fr.TarHeaderSize() < 0 {
		return fmt.Errorf("invalid tar header size: %d", sf.fr.TarHeaderSize())
	}
	tarHeaderReader, err := sf.gr.spanManager.GetContents(tarHeaderOffset, tarHeaderOffset+tarHeaderSize)
	if err != nil {
		return err
	}
	counterReader := ioutils.NewPositionTrackerReader(tarHeaderReader)
	tarReader := tar.NewReader(counterReader)
	tarHeader, err := tarReader.Next()
	if err != nil {
		return fmt.Errorf("error reading tar header at %d, size %d: %w", tarHeaderOffset, tarHeaderSize, err)
	}
	if counterReader.CurrentPos() != int64(tarHeaderSize) {
		return fmt.Errorf("incorrect tar header size: expected %d, actual %d", tarHeaderSize, counterReader.CurrentPos())
	}
	if !attrMatchesTarHeader(attr, tarHeader) {
		return errors.New("file attributes do not match tar header")
	}
	if sf.fr.TarName() != tarHeader.Name {
		return errors.New("file name does not match tar header")
	}

	return nil
}

func attrMatchesTarHeader(attr metadata.Attr, tarh *tar.Header) bool {
	// specifically, we don't look at attr.NumLink because it doesn't exist in a tar header
	if attr.Size != tarh.Size ||
		!attr.ModTime.Equal(tarh.ModTime) ||
		attr.LinkName != tarh.Linkname ||
		attr.Mode != tarh.FileInfo().Mode() ||
		attr.UID != tarh.Uid ||
		attr.GID != tarh.Gid ||
		attr.DevMajor != int(tarh.Devmajor) ||
		attr.DevMinor != int(tarh.Devminor) {
		return false
	}

	tarXattrs := ztoc.Xattrs(tarh.PAXRecords)
	if len(attr.Xattrs) != len(tarXattrs) {
		return false
	}
	for k := range attr.Xattrs {
		attrV := attr.Xattrs[k]
		tarV := tarXattrs[k]
		if len(attrV) != len(tarV) {
			return false
		}
		for i := 0; i < len(attrV); i++ {
			if attrV[i] != tarV[i] {
				return false
			}
		}
	}

	return true
}

type CacheOption func(*cacheOptions)

type cacheOptions struct {
	cacheOpts []cache.Option
	filter    func(int64) bool
	reader    *io.SectionReader
}

func WithCacheOpts(cacheOpts ...cache.Option) CacheOption {
	return func(opts *cacheOptions) {
		opts.cacheOpts = cacheOpts
	}
}

func WithFilter(filter func(int64) bool) CacheOption {
	return func(opts *cacheOptions) {
		opts.filter = filter
	}
}

func WithReader(sr *io.SectionReader) CacheOption {
	return func(opts *cacheOptions) {
		opts.reader = sr
	}
}

func digestVerifier(id uint32, digestStr string) (digest.Verifier, error) {
	digest, err := digest.Parse(digestStr)
	if err != nil {
		return nil, fmt.Errorf("no digset is recorded: %w", err)
	}
	return digest.Verifier(), nil
}
