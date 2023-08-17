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

package spanmanager

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"runtime"

	"github.com/awslabs/soci-snapshotter/cache"
	"github.com/awslabs/soci-snapshotter/ztoc"
	"github.com/awslabs/soci-snapshotter/ztoc/compression"
	"github.com/opencontainers/go-digest"
	"golang.org/x/sync/errgroup"
)

// Specific error types raised by SpanManager.
var (
	ErrSpanNotAvailable    = errors.New("span not available in cache")
	ErrIncorrectSpanDigest = errors.New("span digests do not match")
	ErrExceedMaxSpan       = errors.New("span id larger than max span id")
)

type MultiReaderCloser struct {
	c []io.Closer
	io.Reader
}

func (mrc *MultiReaderCloser) Close() error {
	errs := []error{}
	for _, c := range mrc.c {
		if err := c.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

type SectionReaderCloser struct {
	c io.Closer
	*io.SectionReader
}

func (src *SectionReaderCloser) Close() error {
	return src.c.Close()
}

// SpanManager fetches and caches spans of a given layer.
type SpanManager struct {
	cache                             cache.BlobCache
	cacheOpt                          []cache.Option
	zinfo                             compression.Zinfo
	r                                 *io.SectionReader // reader for contents of the spans managed by SpanManager
	spans                             []*span
	ztoc                              *ztoc.Ztoc
	maxSpanVerificationFailureRetries int
}

type spanInfo struct {
	// starting span id of the requested contents
	spanStart compression.SpanID
	// ending span id of the requested contents
	spanEnd compression.SpanID
	// start offsets of the requested contents within the spans
	startOffInSpan []compression.Offset
	// end offsets the requested contents within the spans
	endOffInSpan []compression.Offset
	// indexes of the spans in the buffer
	spanIndexInBuf []compression.Offset
}

// New creates a SpanManager with given ztoc and content reader, and builds all
// spans based on the ztoc.
func New(ztoc *ztoc.Ztoc, r *io.SectionReader, cache cache.BlobCache, retries int, cacheOpt ...cache.Option) *SpanManager {
	index, err := ztoc.Zinfo()
	if err != nil {
		return nil
	}
	spans := make([]*span, ztoc.MaxSpanID+1)
	m := &SpanManager{
		cache:                             cache,
		cacheOpt:                          cacheOpt,
		zinfo:                             index,
		r:                                 r,
		spans:                             spans,
		ztoc:                              ztoc,
		maxSpanVerificationFailureRetries: retries,
	}
	if m.maxSpanVerificationFailureRetries < 0 {
		m.maxSpanVerificationFailureRetries = defaultSpanVerificationFailureRetries
	}
	m.buildAllSpans()
	runtime.SetFinalizer(m, func(m *SpanManager) {
		m.Close()
	})

	return m
}

func (m *SpanManager) buildAllSpans() {
	var i compression.SpanID
	for i = 0; i <= m.ztoc.MaxSpanID; i++ {
		s := span{
			id:                i,
			startCompOffset:   m.zinfo.StartCompressedOffset(i),
			endCompOffset:     m.zinfo.EndCompressedOffset(i, m.ztoc.CompressedArchiveSize),
			startUncompOffset: m.zinfo.StartUncompressedOffset(i),
			endUncompOffset:   m.zinfo.EndUncompressedOffset(i, m.ztoc.UncompressedArchiveSize),
		}

		m.spans[i] = &s
		m.spans[i].state.Store(unrequested)
	}
}

// FetchSingleSpan invokes the reader to fetch the span in the background and cache
// the span without uncompressing. It is invoked by the BackgroundFetcher.
// span state change: unrequested -> requested -> fetched.
func (m *SpanManager) FetchSingleSpan(spanID compression.SpanID) error {
	if spanID > m.ztoc.MaxSpanID {
		return ErrExceedMaxSpan
	}

	// return directly if span is not in `unrequested`
	s := m.spans[spanID]
	if !s.checkState(unrequested) {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	// check again after acquiring Lock
	if !s.checkState(unrequested) {
		return nil
	}

	_, err := m.fetchAndCacheSpan(spanID, false)
	return err
}

// resolveSpan ensures the span exists in cache and is uncompressed by calling
// `getSpanContent`. Only for testing.
func (m *SpanManager) resolveSpan(spanID compression.SpanID) error {
	if spanID > m.ztoc.MaxSpanID {
		return ErrExceedMaxSpan
	}

	// this func itself doesn't use the returned span data
	_, err := m.getSpanContent(spanID, 0, m.spans[spanID].endUncompOffset)
	return err
}

// GetContents returns a reader for the requested contents. The contents may be
// across multiple spans.
func (m *SpanManager) GetContents(startUncompOffset, endUncompOffset compression.Offset) (io.ReadCloser, error) {
	si := m.getSpanInfo(startUncompOffset, endUncompOffset)
	numSpans := si.spanEnd - si.spanStart + 1
	spanReaders := make([]io.Reader, numSpans)
	spanClosers := make([]io.Closer, numSpans)

	eg, _ := errgroup.WithContext(context.Background())
	var i compression.SpanID
	for i = 0; i < numSpans; i++ {
		j := i
		eg.Go(func() error {
			spanID := j + si.spanStart
			r, err := m.getSpanContent(spanID, si.startOffInSpan[j], si.endOffInSpan[j])
			if err != nil {
				return err
			}
			spanReaders[j] = r
			spanClosers[j] = r
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return nil, err
	}
	return &MultiReaderCloser{spanClosers, io.MultiReader(spanReaders...)}, nil
}

// getSpanInfo returns spanInfo from the offsets of the requested file
func (m *SpanManager) getSpanInfo(offsetStart, offsetEnd compression.Offset) *spanInfo {
	spanStart := m.zinfo.UncompressedOffsetToSpanID(offsetStart)
	spanEnd := m.zinfo.UncompressedOffsetToSpanID(offsetEnd)
	numSpans := spanEnd - spanStart + 1
	start := make([]compression.Offset, numSpans)
	end := make([]compression.Offset, numSpans)
	index := make([]compression.Offset, numSpans)
	var bufSize compression.Offset

	for i := spanStart; i <= spanEnd; i++ {
		j := i - spanStart
		index[j] = bufSize
		s := m.spans[i]
		uncompSpanSize := s.endUncompOffset - s.startUncompOffset
		if offsetStart > s.startUncompOffset {
			start[j] = offsetStart - s.startUncompOffset
		}
		if offsetEnd < s.endUncompOffset {
			end[j] = offsetEnd - s.startUncompOffset
		} else {
			end[j] = uncompSpanSize
		}
		bufSize += end[j] - start[j]
	}
	spanInfo := spanInfo{
		spanStart:      spanStart,
		spanEnd:        spanEnd,
		startOffInSpan: start,
		endOffInSpan:   end,
		spanIndexInBuf: index,
	}
	return &spanInfo
}

// getSpanContent gets uncompressed span content (specified by [offsetStart:offsetEnd]),
// which is returned as an `io.Reader`.
//
// It resolves the span to ensure it exists and is uncompressed in cache:
//  1. For `uncompressed` span, directly return the reader from the cache.
//  2. For `fetched` span, read and uncompress the compressed span from cache, cache and
//     return the reader from the uncompressed span.
//  3. For `unrequested` span, fetch-uncompress-cache the span data, return the reader
//     from the uncompressed span
//  4. No span state lock will be acquired in `requested` state.
func (m *SpanManager) getSpanContent(spanID compression.SpanID, offsetStart, offsetEnd compression.Offset) (io.ReadCloser, error) {
	s := m.spans[spanID]
	size := offsetEnd - offsetStart

	// return from cache directly if cached and uncompressed
	if s.checkState(uncompressed) {
		return m.getSpanFromCache(s.id, offsetStart, size)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	// check again after acquiring lock
	if s.checkState(uncompressed) {
		return m.getSpanFromCache(s.id, offsetStart, size)
	}

	// if cached but not uncompressed, uncompress and cache the span content
	if s.checkState(fetched) {
		// get compressed span from the cache
		compressedSize := s.endCompOffset - s.startCompOffset
		r, err := m.getSpanFromCache(s.id, 0, compressedSize)
		if err != nil {
			return nil, err
		}
		defer r.Close()

		// read compressed span
		compressedBuf, err := io.ReadAll(r)
		if err != nil {
			return nil, err
		}

		// uncompress span
		uncompSpanBuf, err := m.uncompressSpan(s, compressedBuf)
		if err != nil {
			return nil, err
		}

		// cache uncompressed span
		if err := m.addSpanToCache(s.id, uncompSpanBuf, m.cacheOpt...); err != nil {
			return nil, err
		}
		if err := s.setState(uncompressed); err != nil {
			return nil, err
		}
		return io.NopCloser(bytes.NewReader(uncompSpanBuf[offsetStart : offsetStart+size])), nil
	}

	// fetch-uncompress-cache span: span state can only be `unrequested` since
	// no goroutine will release span state lock in `requested` state
	uncompBuf, err := m.fetchAndCacheSpan(s.id, true)
	if err != nil {
		return nil, err
	}
	buf := bytes.NewBuffer(uncompBuf[offsetStart : offsetStart+size])
	return io.NopCloser(buf), nil
}

// fetchAndCacheSpan fetches a span, uncompresses the span if `uncompress == true`,
// caches and returns the span content. The span state is set to `fetched/uncompressed`,
// depending on if `uncompress` is enabled.
// The caller needs to check the span state (e.g. `unrequested`) and acquires the
// span's state lock before calling.
func (m *SpanManager) fetchAndCacheSpan(spanID compression.SpanID, uncompress bool) (buf []byte, err error) {
	s := m.spans[spanID]

	// change to `requested`; if fetch/cache fails, change back to `unrequested`
	// so other goroutines can request again.
	if err := s.setState(requested); err != nil {
		return nil, err
	}
	defer func() {
		if err != nil && s.checkState(requested) {
			s.setState(unrequested)
		}
	}()

	// fetch compressed span
	compressedBuf, err := m.fetchSpanWithRetries(spanID)
	if err != nil {
		return nil, err
	}

	buf = compressedBuf
	var state = fetched

	if uncompress {
		// uncompress span
		uncompSpanBuf, err := m.uncompressSpan(s, compressedBuf)
		if err != nil {
			return nil, err
		}
		buf = uncompSpanBuf
		state = uncompressed
	}

	// cache span data
	if err := m.addSpanToCache(spanID, buf, m.cacheOpt...); err != nil {
		return nil, err
	}
	if err := s.setState(state); err != nil {
		return nil, err
	}
	return buf, nil
}

// fetchSpanWithRetries fetches the requested data and verifies that the span digest matches the one in the ztoc.
// It will retry the fetch and verification m.maxSpanVerificationFailureRetries times.
// It does not retry when there is an error fetching the data, because retries already happen lower in the stack in httpFetcher.
// If there is an error fetching data from remote, it is not an transient error.
func (m *SpanManager) fetchSpanWithRetries(spanID compression.SpanID) ([]byte, error) {
	s := m.spans[spanID]
	offset := s.startCompOffset
	compressedSize := s.endCompOffset - s.startCompOffset
	compressedBuf := make([]byte, compressedSize)

	var (
		err error
		n   int
	)
	for i := 0; i < m.maxSpanVerificationFailureRetries+1; i++ {
		n, err = m.r.ReadAt(compressedBuf, int64(offset))
		// if the n = len(p) bytes returned by ReadAt are at the end of the input source,
		// ReadAt may return either err == EOF or err == nil: https://pkg.go.dev/io#ReaderAt
		if err != nil && err != io.EOF {
			return []byte{}, err
		}

		if n != len(compressedBuf) {
			return []byte{}, fmt.Errorf("unexpected data size for reading compressed span. read = %d, expected = %d", n, len(compressedBuf))
		}

		if err = m.verifySpanContents(compressedBuf, spanID); err == nil {
			return compressedBuf, nil
		}
	}
	return []byte{}, err
}

// uncompressSpan uses zinfo to extract uncompressed span data from compressed
// span data.
func (m *SpanManager) uncompressSpan(s *span, compressedBuf []byte) ([]byte, error) {
	uncompSize := s.endUncompOffset - s.startUncompOffset

	// Theoretically, a span can be empty. If that happens, just return an empty buffer.
	if uncompSize == 0 {
		return []byte{}, nil
	}

	bytes, err := m.zinfo.ExtractDataFromBuffer(compressedBuf, uncompSize, s.startUncompOffset, s.id)
	if err != nil {
		return nil, err
	}
	return bytes, nil
}

// addSpanToCache adds contents of the span to the cache.
// A non-nil error is returned if the data is not written to the cache.
func (m *SpanManager) addSpanToCache(spanID compression.SpanID, contents []byte, opts ...cache.Option) error {
	w, err := m.cache.Add(fmt.Sprintf("%d", spanID), opts...)
	if err != nil {
		return err
	}
	defer w.Close()

	_, err = w.Write(contents)
	if err != nil {
		w.Abort()
		return err
	}

	w.Commit()
	return nil
}

// getSpanFromCache returns the cached span content as an `io.Reader`.
// `offset` is the offset of the requested contents within the span.
// `size` is the size of the requested contents.
func (m *SpanManager) getSpanFromCache(spanID compression.SpanID, offset, size compression.Offset) (io.ReadCloser, error) {
	r, err := m.cache.Get(fmt.Sprintf("%d", spanID))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrSpanNotAvailable, err)
	}
	return &SectionReaderCloser{r, io.NewSectionReader(r, int64(offset), int64(size))}, nil
}

// verifySpanContents calculates span digest from its compressed bytes, and compare
// with the digest stored in ztoc.
func (m *SpanManager) verifySpanContents(compressedData []byte, spanID compression.SpanID) error {
	actual := digest.FromBytes(compressedData)
	expected := m.ztoc.SpanDigests[spanID]
	if actual != expected {
		return fmt.Errorf("expected %v but got %v: %w", expected, actual, ErrIncorrectSpanDigest)
	}
	return nil
}

// Close closes both the underlying zinfo data and blob cache.
func (m *SpanManager) Close() {
	m.zinfo.Close()
	m.cache.Close()
}
