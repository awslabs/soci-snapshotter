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
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/awslabs/soci-snapshotter/cache"
	"github.com/awslabs/soci-snapshotter/soci"
	"github.com/opencontainers/go-digest"
	"golang.org/x/sync/errgroup"
)

type spanState int

const (
	// A span is in Unrequested state when it's not requested from remote.
	unrequested spanState = iota
	// A span is in Requested state when it's requested from remote but its content hasn't been returned.
	requested
	// A span is in Fetched state when its content is fetched from remote.
	fetched
	// A span is in Uncompressed state when it's uncompressed and its uncompressed content is cached.
	uncompressed
)

// map of valid span transtions. Key is the current state and value is valid new states.
var stateTransitionMap = map[spanState][]spanState{
	unrequested:  {unrequested, requested},
	requested:    {requested, fetched},
	fetched:      {fetched, uncompressed},
	uncompressed: {uncompressed},
}

var (
	ErrSpanNotAvailable           = errors.New("span not available in cache")
	ErrIncorrectSpanDigest        = errors.New("span digests do not match")
	ErrExceedMaxSpan              = errors.New("span id larger than max span id")
	errInvalidSpanStateTransition = errors.New("invalid span state transition")
)

type span struct {
	id                soci.SpanID
	startCompOffset   soci.FileSize
	endCompOffset     soci.FileSize
	startUncompOffset soci.FileSize
	endUncompOffset   soci.FileSize
	state             atomic.Value
	mu                sync.Mutex
}

func (s *span) setState(state spanState) error {
	err := s.validateStateTransition(state)
	if err != nil {
		return err
	}
	s.state.Store(state)
	return nil
}

func (s *span) validateStateTransition(newState spanState) error {
	state := s.state.Load().(spanState)
	for _, s := range stateTransitionMap[state] {
		if newState == s {
			return nil
		}
	}
	return errInvalidSpanStateTransition
}

type SpanManager struct {
	cache    cache.BlobCache
	cacheOpt []cache.Option
	index    *soci.GzipZinfo
	r        *io.SectionReader // reader for contents of the spans managed by SpanManager
	spans    []*span
	ztoc     *soci.Ztoc
}

type spanInfo struct {
	// starting span id of the requested contents
	spanStart soci.SpanID
	// ending span id of the requested contents
	spanEnd soci.SpanID
	// start offsets of the requested contents within the spans
	startOffInSpan []soci.FileSize
	// end offsets the requested contents within the spans
	endOffInSpan []soci.FileSize
	// indexes of the spans in the buffer
	spanIndexInBuf []soci.FileSize
}

func New(ztoc *soci.Ztoc, r *io.SectionReader, cache cache.BlobCache, cacheOpt ...cache.Option) *SpanManager {
	index, err := soci.NewGzipZinfo(ztoc.CompressionInfo.Checkpoints)
	if err != nil {
		return nil
	}
	spans := make([]*span, ztoc.CompressionInfo.MaxSpanID+1)
	m := &SpanManager{
		cache:    cache,
		cacheOpt: cacheOpt,
		index:    index,
		r:        r,
		spans:    spans,
		ztoc:     ztoc,
	}
	m.buildAllSpans()
	runtime.SetFinalizer(m, func(m *SpanManager) {
		m.Close()
	})

	return m
}

func (m *SpanManager) buildAllSpans() {
	m.spans[0] = &span{
		id:                0,
		startCompOffset:   m.index.SpanIDToCompressedOffset(soci.SpanID(0)),
		endCompOffset:     m.getEndCompressedOffset(0),
		startUncompOffset: m.index.SpanIDToUncompressedOffset(soci.SpanID(0)),
		endUncompOffset:   m.getEndUncompressedOffset(0),
	}
	m.spans[0].state.Store(unrequested)
	var i soci.SpanID
	for i = 1; i <= m.ztoc.CompressionInfo.MaxSpanID; i++ {
		startCompOffset := m.spans[i-1].endCompOffset
		hasBits := m.index.HasBits(i)
		if hasBits {
			startCompOffset--
		}
		s := span{
			id:                i,
			startCompOffset:   startCompOffset,
			endCompOffset:     m.getEndCompressedOffset(i),
			startUncompOffset: m.spans[i-1].endUncompOffset,
			endUncompOffset:   m.getEndUncompressedOffset(i),
		}
		m.spans[i] = &s
		m.spans[i].state.Store(unrequested)
	}
}

func (m *SpanManager) ResolveSpan(spanID soci.SpanID, r *io.SectionReader) error {
	if spanID > m.ztoc.CompressionInfo.MaxSpanID {
		return ErrExceedMaxSpan
	}

	// Check if the span exists in the cache
	s := m.spans[spanID]
	s.mu.Lock()
	defer s.mu.Unlock()
	state := s.state.Load().(spanState)
	if state == uncompressed {
		id := strconv.Itoa(int(spanID))
		_, err := m.cache.Get(id)
		if err == nil {
			// The span is already in cache.
			return nil
		}
	}

	// The span is not available in cache. Fetch the span and add it to cache
	_, err := m.fetchAndCacheSpan(spanID, r, true)
	if err != nil {
		return err
	}

	return nil
}

// GetContents returns a reader for the requested contents.
// offsetStart and offsetEnd are start and end uncompressed offsets of the file.
func (m *SpanManager) GetContents(offsetStart, offsetEnd soci.FileSize) (io.Reader, error) {
	si := m.getSpanInfo(offsetStart, offsetEnd)
	numSpans := si.spanEnd - si.spanStart + 1
	spanReaders := make([]io.Reader, numSpans)

	eg, _ := errgroup.WithContext(context.Background())
	var i soci.SpanID
	for i = 0; i < numSpans; i++ {
		j := i
		eg.Go(func() error {
			spanContentSize := si.endOffInSpan[j] - si.startOffInSpan[j]
			spanID := j + si.spanStart
			r, err := m.GetSpanContent(spanID, si.startOffInSpan[j], si.endOffInSpan[j], spanContentSize)
			if err != nil {
				return err
			}
			spanReaders[j] = r
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return nil, err
	}

	return io.MultiReader(spanReaders...), nil
}

// getSpanInfo returns spanInfo from the offsets of the requested file
func (m *SpanManager) getSpanInfo(offsetStart, offsetEnd soci.FileSize) *spanInfo {
	spanStart := m.index.UncompressedOffsetToSpanID(offsetStart)
	spanEnd := m.index.UncompressedOffsetToSpanID(offsetEnd)
	numSpans := spanEnd - spanStart + 1
	start := make([]soci.FileSize, numSpans)
	end := make([]soci.FileSize, numSpans)
	index := make([]soci.FileSize, numSpans)
	var bufSize soci.FileSize

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

func (m *SpanManager) GetSpanContent(spanID soci.SpanID, offsetStart, offsetEnd, size soci.FileSize) (io.Reader, error) {
	// Check if we can resolve the span from the cache
	s := m.spans[spanID]
	r, err := m.resolveSpanFromCache(s, offsetStart, size)
	if err == nil {
		return r, nil
	} else if !errors.Is(err, ErrSpanNotAvailable) {
		// if the span exists in the cache but resolveSpanFromCache fails, return the error to caller
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	// retry resolveSpanFromCache in case we raced with another thread
	r, err = m.resolveSpanFromCache(s, offsetStart, size)
	if err == nil {
		return r, nil
	} else if !errors.Is(err, ErrSpanNotAvailable) {
		// if the span exists in the cache but resolveSpanFromCache fails, return the error to caller
		return nil, err
	}
	uncompBuf, err := m.fetchAndCacheSpan(spanID, m.r, false)
	if err != nil {
		return nil, err
	}

	buf := bytes.NewBuffer(uncompBuf[offsetStart:offsetEnd])
	return io.Reader(buf), nil
}

// getSpanFromCache returns the reader for the contents of the span stored in the cache.
// offset is the offset of the requested contents within the span. size is the size of the requested contents.
func (m *SpanManager) getSpanFromCache(spanID string, offset, size soci.FileSize) (io.Reader, error) {
	r, err := m.cache.Get(spanID)
	if err != nil {
		return nil, ErrSpanNotAvailable
	}
	runtime.SetFinalizer(r, func(r cache.Reader) {
		r.Close()
	})
	return io.NewSectionReader(r, int64(offset), int64(size)), // doing integer type conversion to allow passing offset and size on the reader
		nil
}

func (m *SpanManager) verifySpanContents(compressedData []byte, id soci.SpanID) error {
	actual := digest.FromBytes(compressedData)
	expected := m.ztoc.CompressionInfo.SpanDigests[id]
	if actual != expected {
		return fmt.Errorf("expected %v but got %v: %w", expected, actual, ErrIncorrectSpanDigest)
	}
	return nil
}

// addSpanToCache adds contents of the span to the cache.
func (m *SpanManager) addSpanToCache(spanID string, contents []byte, opts ...cache.Option) {
	if w, err := m.cache.Add(spanID, opts...); err == nil {
		if n, err := w.Write(contents); err != nil || n != len(contents) {
			w.Abort()
		} else {
			w.Commit()
		}
		w.Close()
	}
}

// resolveSpanFromCache resolves the span (in Fetched/Uncompressed state) from the cache.
// This method returns the reader for the uncompressed span.
// For Uncompressed span, directly return the reader from the cache.
// For Fetched span, get the compressed span from the cache, uncompress it, cache the uncompressed span and
// returns the reader for the uncompressed span.
func (m *SpanManager) resolveSpanFromCache(s *span, offsetStart, size soci.FileSize) (io.Reader, error) {
	id := fmt.Sprintf("%d", s.id)
	state := s.state.Load().(spanState)
	if state == uncompressed {
		r, err := m.getSpanFromCache(id, offsetStart, size)
		if err != nil {
			return nil, err
		}
		return r, nil
	}
	if state == fetched {
		// get the compressed span from the cache
		compressedSize := s.endCompOffset - s.startCompOffset
		r, err := m.getSpanFromCache(id, 0, compressedSize)
		if err != nil {
			return nil, err
		}

		// read the compressed span
		compressedBuf, err := io.ReadAll(r)
		if err != nil {
			return nil, err
		}

		// uncompress the span
		uncompSpanBuf, err := m.uncompressSpan(s, compressedBuf)
		if err != nil {
			return nil, err
		}

		// cache the uncompressed span
		m.addSpanToCache(id, uncompSpanBuf, m.cacheOpt...)
		err = s.setState(uncompressed)
		if err != nil {
			return nil, err
		}
		return bytes.NewReader(uncompSpanBuf[offsetStart : offsetStart+size]), nil
	}
	return nil, ErrSpanNotAvailable
}

func (m *SpanManager) fetchSpan(buf []byte, spanID soci.SpanID, r *io.SectionReader) error {
	s := m.spans[spanID]
	err := s.setState(requested)
	if err != nil {
		return err
	}
	n, err := r.ReadAt(buf, int64(s.startCompOffset))
	if err != nil {
		return err
	}
	if n != len(buf) {
		return fmt.Errorf("unexpected data size for reading compressed span. read = %d, expected = %d", n, len(buf))
	}
	return nil
}

func (m *SpanManager) uncompressSpan(s *span, compressedBuf []byte) ([]byte, error) {
	uncompSize := s.endUncompOffset - s.startUncompOffset

	// Theoretically, a span can be empty. If that happens, just return an empty buffer.
	if uncompSize == 0 {
		return []byte{}, nil
	}

	bytes, err := m.index.ExtractDataFromBuffer(compressedBuf, uncompSize, s.startUncompOffset, s.id)
	if err != nil {
		return nil, err
	}
	return bytes, nil
}

func (m *SpanManager) fetchAndCacheSpan(spanID soci.SpanID, r *io.SectionReader, isPrefetch bool) ([]byte, error) {
	s := m.spans[spanID]
	compressedSize := s.endCompOffset - s.startCompOffset
	compressedBuf := make([]byte, compressedSize)
	err := m.fetchSpan(compressedBuf, spanID, r)
	if err != nil && err != io.EOF {
		return nil, err
	}

	if err := m.verifySpanContents(compressedBuf, spanID); err != nil {
		return nil, err
	}
	err = s.setState(fetched)
	if err != nil {
		return nil, err
	}

	id := strconv.Itoa(int(spanID))
	if isPrefetch {
		m.addSpanToCache(id, compressedBuf, m.cacheOpt...)
		if err != nil {
			return nil, err
		}
		return nil, nil

	}

	uncompSpanBuf, err := m.uncompressSpan(s, compressedBuf)
	if err != nil {
		return nil, err
	}

	// Cache the content of the whole span
	m.addSpanToCache(id, uncompSpanBuf, m.cacheOpt...)
	err = s.setState(uncompressed)
	if err != nil {
		return nil, err
	}
	return uncompSpanBuf, nil
}

func (m *SpanManager) getEndCompressedOffset(spanID soci.SpanID) soci.FileSize {
	var end soci.FileSize
	if spanID == m.ztoc.CompressionInfo.MaxSpanID {
		end = m.ztoc.CompressedArchiveSize
	} else {
		end = m.index.SpanIDToCompressedOffset(spanID + 1)
	}
	return end
}

func (m *SpanManager) getEndUncompressedOffset(spanID soci.SpanID) soci.FileSize {
	var end soci.FileSize
	if spanID == m.ztoc.CompressionInfo.MaxSpanID {
		end = m.ztoc.UncompressedArchiveSize
	} else {
		end = m.index.SpanIDToUncompressedOffset(spanID + 1)
	}
	return end
}

func (m *SpanManager) Close() {
	m.index.Close()
	m.cache.Close()
}
