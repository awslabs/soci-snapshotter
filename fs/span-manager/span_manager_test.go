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
	"compress/gzip"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/awslabs/soci-snapshotter/cache"
	commonmetrics "github.com/awslabs/soci-snapshotter/fs/metrics/common"
	"github.com/awslabs/soci-snapshotter/util/testutil"
	"github.com/awslabs/soci-snapshotter/ztoc"
	"github.com/awslabs/soci-snapshotter/ztoc/compression"
	"github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/assert"
)

func TestSpanManager(t *testing.T) {
	var spanSize compression.Offset = 65536 // 64 KiB
	fileName := "span-manager-test"
	testCases := []struct {
		name          string
		maxSpans      compression.SpanID
		sectionReader *io.SectionReader
		expectedError error
	}{
		{
			name:     "a file from 1 span",
			maxSpans: 1,
		},
		{
			name:     "a file from 100 spans",
			maxSpans: 100,
		},
		{
			name:          "bad MaxSpanID",
			maxSpans:      5,
			expectedError: ErrIncorrectMaxSpanID,
		},
		{
			name:          "bad span start/end",
			maxSpans:      3,
			expectedError: ErrNonMonotonicCheckpoints,
		},
		{
			name:     "header verification fails",
			maxSpans: 100,
			sectionReader: io.NewSectionReader(readerFn(func(b []byte, _ int64) (int, error) {
				sz := compression.Offset(len(b))
				r := testutil.NewTestRand(t)
				copy(b, r.RandomByteData(int64(sz))) // populate with garbage data
				return len(b), nil
			}), 0, 10000000),
			expectedError: gzip.ErrHeader,
		},
		{
			name:     "span digest verification fails",
			maxSpans: 100,
			sectionReader: io.NewSectionReader(readerFn(func(b []byte, _ int64) (int, error) {
				var r bytes.Buffer
				w := gzip.NewWriter(&r)
				w.Write([]byte("failing digest verification"))
				w.Close()

				gz, err := io.ReadAll(&r)
				if err != nil {
					t.Fatalf("error creating SectionReader: %v", err)
				}

				copy(b, gz)
				return len(b), nil
			}), 0, 10000000),
			expectedError: ErrIncorrectSpanDigest,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var err error
			defer func() {
				if err != nil && !errors.Is(err, tc.expectedError) {
					t.Fatal(err)
				}
			}()

			fileContent := []byte{}
			tRand := testutil.NewTestRand(t)
			for i := 0; i < int(tc.maxSpans); i++ {
				fileContent = append(fileContent, tRand.RandomByteData(int64(spanSize))...)
			}
			tarEntries := []testutil.TarEntry{
				testutil.File(fileName, string(fileContent)),
			}

			toc, r, err := ztoc.BuildZtocReader(t, tarEntries, gzip.BestCompression, int64(spanSize))
			if err != nil {
				err = fmt.Errorf("failed to create ztoc: %w", err)
				return
			}

			// Make TOC mutations as needed
			if errors.Is(tc.expectedError, ErrIncorrectMaxSpanID) {
				toc.MaxSpanID++
			} else if errors.Is(tc.expectedError, ErrNonMonotonicCheckpoints) {
				corruptCheckpoints(toc)
			}

			if tc.sectionReader != nil {
				r = tc.sectionReader
			}

			cache := cache.NewMemoryCache()
			defer cache.Close()
			m, err := New(toc, r, cache, 0, digest.FromString(""))
			if err != nil {
				return
			}

			// Test GetContent
			fileContentFromSpans, err := getFileContentFromSpans(m, toc, fileName)
			if err != nil {
				return
			}
			if !bytes.Equal(fileContent, fileContentFromSpans) {
				err = fmt.Errorf("file contents are not the same as span contents")
				return
			}

			// Test resolving all spans
			var i compression.SpanID
			for i = 0; i <= toc.MaxSpanID; i++ {
				err := m.resolveSpan(i)
				if err != nil {
					t.Fatalf("error resolving span %d. error: %v", i, err)
				}
			}

			// Test resolveSpan returning ErrExceedMaxSpan for span id larger than max span id
			resolveSpanErr := m.resolveSpan(toc.MaxSpanID + 1)
			if !errors.Is(resolveSpanErr, ErrExceedMaxSpan) {
				t.Fatalf("failed returning ErrExceedMaxSpan for span id larger than max span id")
			}
		})
	}
}

func TestSpanManagerCache(t *testing.T) {
	tRand := testutil.NewTestRand(t)
	var spanSize compression.Offset = 65536 // 64 KiB
	content := tRand.RandomByteData(int64(spanSize))
	tarEntries := []testutil.TarEntry{
		testutil.File("span-manager-cache-test", string(content)),
	}
	toc, r, err := ztoc.BuildZtocReader(t, tarEntries, gzip.BestCompression, int64(spanSize))
	if err != nil {
		t.Fatalf("failed to create ztoc: %v", err)
	}
	cache := cache.NewMemoryCache()
	defer cache.Close()
	m, err := New(toc, r, cache, 0, digest.FromString(""))
	assert.Nil(t, err)
	spanID := 0
	err = m.resolveSpan(compression.SpanID(spanID))
	if err != nil {
		t.Fatalf("failed to resolve span 0: %v", err)
	}

	testCases := []struct {
		name   string
		offset compression.Offset
		size   compression.Offset
	}{
		{
			name:   "offset 0",
			offset: 0,
			size:   100,
		},
		{
			name:   "offset 20000",
			offset: 20000,
			size:   500,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Test resolveSpanFromCache
			spanR, err := m.getSpanContent(compression.SpanID(spanID), tc.offset, tc.offset+tc.size)
			if err != nil {
				t.Fatalf("error resolving span from cache")
			}
			spanContent, err := io.ReadAll(spanR)
			if err != nil && !errors.Is(err, io.EOF) {
				t.Fatalf("error reading span content")
			}
			if tc.size != compression.Offset(len(spanContent)) {
				t.Fatalf("size of span content from cache is not expected")
			}
		})
	}
}

func TestStateTransition(t *testing.T) {
	tRand := testutil.NewTestRand(t)
	var spanSize compression.Offset = 65536 // 64 KiB
	content := tRand.RandomByteData(int64(spanSize))
	tarEntries := []testutil.TarEntry{
		testutil.File("set-span-test", string(content)),
	}
	toc, r, err := ztoc.BuildZtocReader(t, tarEntries, gzip.BestCompression, int64(spanSize))
	if err != nil {
		t.Fatalf("failed to create ztoc: %v", err)
	}
	cache := cache.NewMemoryCache()
	defer cache.Close()
	m, err := New(toc, r, cache, 0, digest.FromString(""))
	assert.Nil(t, err)

	// check initial span states
	for i := uint32(0); i <= uint32(toc.MaxSpanID); i++ {
		state := m.spans[i].state.Load().(spanState)
		if state != unrequested {
			t.Fatalf("failed initializing span states to Unrequested")
		}
	}

	testCases := []struct {
		name      string
		spanID    compression.SpanID
		isBgFetch bool
	}{
		{
			name:      "span 0 - bgfetch",
			spanID:    0,
			isBgFetch: true,
		},
		{
			name:   "span 0 - on demand fetch",
			spanID: 0,
		},
		{
			name:      "max span - bgfetch",
			spanID:    m.ztoc.MaxSpanID,
			isBgFetch: true,
		},
		{
			name:   "max span - on demand fetch",
			spanID: m.ztoc.MaxSpanID,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			s := m.spans[tc.spanID]
			if tc.isBgFetch {
				err := m.FetchSingleSpan(tc.spanID)
				if err != nil {
					t.Fatalf("failed resolving the span for prefetch: %v", err)
				}
				state := s.state.Load().(spanState)
				if state != fetched {
					t.Fatalf("failed transitioning to Fetched state")
				}
			} else {
				_, err := m.getSpanContent(tc.spanID, 0, s.endUncompOffset-s.startUncompOffset)
				if err != nil {
					t.Fatalf("failed getting the span for on-demand fetch: %v", err)
				}
				state := s.state.Load().(spanState)
				if state != uncompressed {
					t.Fatalf("failed transitioning to Uncompressed state")
				}
			}
		})
	}
}

func TestValidateState(t *testing.T) {
	testCases := []struct {
		name         string
		currentState spanState
		newState     []spanState
		expectedErr  error
	}{
		{
			name:         "span in Unrequested state with valid new state",
			currentState: unrequested,
			newState:     []spanState{requested},
			expectedErr:  nil,
		},
		{
			name:         "span in Unrequested state with invalid new state",
			currentState: unrequested,
			newState:     []spanState{unrequested, fetched, uncompressed},
			expectedErr:  errInvalidSpanStateTransition,
		},
		{
			name:         "span in Requested state with valid new state",
			currentState: requested,
			newState:     []spanState{unrequested, fetched, uncompressed},
			expectedErr:  nil,
		},
		{
			name:         "span in Requested state with invalid new state",
			currentState: requested,
			newState:     []spanState{requested},
			expectedErr:  errInvalidSpanStateTransition,
		},
		{
			name:         "span in Fetched state with valid new state",
			currentState: fetched,
			newState:     []spanState{uncompressed},
			expectedErr:  nil,
		},
		{
			name:         "span in Fetched state with invalid new state",
			currentState: fetched,
			newState:     []spanState{unrequested, requested, fetched},
			expectedErr:  errInvalidSpanStateTransition,
		},
		{
			name:         "span in Uncompressed state with valid new state",
			currentState: uncompressed,
			newState:     []spanState{},
			expectedErr:  nil,
		},
		{
			name:         "span in Uncompressed state with invalid new state",
			currentState: uncompressed,
			newState:     []spanState{unrequested, requested, fetched, uncompressed},
			expectedErr:  errInvalidSpanStateTransition,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			for _, ns := range tc.newState {
				s := span{}
				s.state.Store(tc.currentState)
				err := s.validateStateTransition(ns)
				if !errors.Is(err, tc.expectedErr) {
					t.Fatalf("failed validateState. current state: %v, new state: %v", tc.currentState, ns)
				}
			}
		})
	}
}

func TestSpanManagerRetries(t *testing.T) {
	testCases := []struct {
		name               string
		spanManagerRetries int
		readerErrors       int
		expectedErr        error
	}{
		{
			name:               "reader returns correct data first time",
			spanManagerRetries: 3,
			readerErrors:       0,
		},
		{
			name:               "reader returns correct data the last time",
			spanManagerRetries: 3,
			readerErrors:       2,
		},
		{
			name:               "reader returns ErrIncorrectSpanDigest",
			spanManagerRetries: 3,
			readerErrors:       5,
			expectedErr:        ErrIncorrectSpanDigest,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r := testutil.NewTestRand(t)
			randStr := string(r.RandomByteData(10000000))

			entries := []testutil.TarEntry{
				testutil.File("test", randStr),
			}
			ztoc, sr, err := ztoc.BuildZtocReader(t, entries, gzip.DefaultCompression, 100000)
			if err != nil {
				t.Fatal(err)
			}
			rdr := newRetryableReaderAt(sr, tc.readerErrors)
			sr = io.NewSectionReader(rdr, 0, 10000000)
			sm, err := New(ztoc, sr, cache.NewMemoryCache(), tc.spanManagerRetries, digest.FromString(""))
			assert.Nil(t, err)

			for i := 0; i < int(ztoc.MaxSpanID); i++ {
				rdr.errCount = 0

				_, err := sm.fetchAndCacheSpan(compression.SpanID(i), true)
				if !errors.Is(err, tc.expectedErr) {
					t.Fatalf("unexpected err; expected %v, got %v", tc.expectedErr, err)
				}

				if rdr.errCount != min(tc.spanManagerRetries+1, tc.readerErrors) {
					t.Fatalf("retry count is unexpected; expected %d, got %d", min(tc.spanManagerRetries+1, tc.readerErrors), rdr.errCount)
				}
			}
		})
	}
}

// A retryableReaderAt returns incorrect data to the caller maxErrors - 1 times.
type retryableReaderAt struct {
	inner     *io.SectionReader
	errCount  int
	maxErrors int
}

func newRetryableReaderAt(inner *io.SectionReader, maxErrors int) *retryableReaderAt {
	return &retryableReaderAt{
		inner:     inner,
		maxErrors: maxErrors,
		errCount:  -1, // First read needs to succeed to create spanmanager
	}
}

func (r *retryableReaderAt) ReadAt(buf []byte, off int64) (int, error) {
	n, err := r.inner.ReadAt(buf, off)
	if (err != nil && err != io.EOF) || n != len(buf) {
		return n, err
	}
	if r.errCount < r.maxErrors {
		r.errCount++
		if r.errCount > 0 {
			buf[0] = buf[0] ^ 0xff
		}
	}
	return n, err
}

// corruptCheckpoint will corrupt a particular checkpoint to create a non-monotonic offset list
// (i.e. checkpoint 1 -> offset 0, checkpoint 2 -> offset 100, checkpoint 3 -> offset 10)
// This is separated out from the main test function just to put a lot of text
// without making it hard to read for all the other test cases.
func corruptCheckpoints(toc *ztoc.Ztoc) {
	// Numbers below are from gzip_zinfo.h, copy-pasting for clarity

	/* Since gzip is compressed with 32 KiB window size, WINDOW_SIZE is fixed
		#define WINSIZE 32768U

	    -  8 bytes, compressed offset
	    -  8 bytes, uncompressed offset
	    -  1 byte, bits
	    -  32768 bytes, window

		#define PACKED_CHECKPOINT_SIZE (8 + 8 + 1 + WINSIZE)

		-  4 bytes, number of checkpoints
	    -  8 bytes, span size
		#define BLOB_HEADER_SIZE (4 + 8)
	*/
	const (
		blobHeaderSize       = 4 + 8
		packedCheckpointSize = 8 + 8 + 1 + 32768
	)

	startChkpt2 := blobHeaderSize + 2*packedCheckpointSize
	// 0x1 is arbitrary, any value reasonably less than the previous checkpoint's offest will do
	binary.LittleEndian.PutUint64(toc.Checkpoints[startChkpt2:startChkpt2+8], 0x1)
}

func getFileContentFromSpans(m *SpanManager, toc *ztoc.Ztoc, fileName string) ([]byte, error) {
	metadata, err := toc.GetMetadataEntry(fileName)
	if err != nil {
		return nil, err
	}
	offsetStart := metadata.UncompressedOffset
	offsetEnd := offsetStart + metadata.UncompressedSize
	r, err := m.GetContents(offsetStart, offsetEnd)
	if err != nil {
		return nil, err
	}
	content, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return content, nil
}

type readerFn func([]byte, int64) (int, error)

func (f readerFn) ReadAt(b []byte, n int64) (int, error) {
	return f(b, n)
}

func TestSynchronousFetchMetricOnlyFiresOnNetworkFetch(t *testing.T) {
	tRand := testutil.NewTestRand(t)
	var spanSize compression.Offset = 65536
	// Create content spanning at least 2 spans
	content := tRand.RandomByteData(int64(spanSize) * 2)
	tarEntries := []testutil.TarEntry{
		testutil.File("metric-test", string(content)),
	}
	toc, r, err := ztoc.BuildZtocReader(t, tarEntries, gzip.BestCompression, int64(spanSize))
	if err != nil {
		t.Fatalf("failed to create ztoc: %v", err)
	}
	c := cache.NewMemoryCache()
	defer c.Close()

	layerDigest := digest.FromString("metric-test-layer")
	m, err := New(toc, r, c, 0, layerDigest)
	assert.Nil(t, err)

	getCount := func() float64 {
		return commonmetrics.GetOperationCount(commonmetrics.SynchronousReadRegistryFetchCount, layerDigest)
	}

	// Span 0: unrequested -> should increment metric
	before := getCount()
	s := m.spans[0]
	_, err = m.getSpanContent(0, 0, s.endUncompOffset-s.startUncompOffset)
	assert.Nil(t, err)
	after := getCount()
	assert.Equal(t, before+1, after, "metric should increment on network fetch")

	// Span 0 again: now uncompressed/cached -> should NOT increment
	before = getCount()
	_, err = m.getSpanContent(0, 0, s.endUncompOffset-s.startUncompOffset)
	assert.Nil(t, err)
	after = getCount()
	assert.Equal(t, before, after, "metric should not increment on cache hit")

	// Span 1: bg-fetch first, then getSpanContent -> should NOT increment
	if toc.MaxSpanID >= 1 {
		err = m.FetchSingleSpan(1)
		assert.Nil(t, err)

		before = getCount()
		s1 := m.spans[1]
		_, err = m.getSpanContent(1, 0, s1.endUncompOffset-s1.startUncompOffset)
		assert.Nil(t, err)
		after = getCount()
		assert.Equal(t, before, after, "metric should not increment when bg fetcher already fetched span")
	}
}
