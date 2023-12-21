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
	"bytes"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const (
	testURL            = "http://testdummy.com/v2/library/test/blobs/sha256:deadbeaf"
	rangeHeaderPrefix  = "bytes="
	sampleChunkSize    = 3
	sampleMiddleOffset = sampleChunkSize / 2
	sampleData1        = "0123456789"
)

// Tests ReadAt method of each file.
func TestReadAt(t *testing.T) {
	sizeCond := map[string]int64{
		"single_chunk": sampleChunkSize - sampleMiddleOffset,
		"multi_chunks": 2*sampleChunkSize + sampleMiddleOffset,
	}
	innerOffsetCond := map[string]int64{
		"at_top":    0,
		"at_middle": sampleMiddleOffset,
	}
	baseOffsetCond := map[string]int64{
		"of_1st_chunk":  sampleChunkSize * 0,
		"of_2nd_chunk":  sampleChunkSize * 1,
		"of_last_chunk": sampleChunkSize * (int64(len(sampleData1)) / sampleChunkSize),
	}
	blobSizeCond := map[string]int64{
		"in_1_chunk_blob":   sampleChunkSize * 1,
		"in_3_chunks_blob":  sampleChunkSize * 3,
		"in_max_size_chunk": int64(len(sampleData1)),
	}
	transportCond := map[string]struct {
		allowMultiRange bool
	}{
		"with_multi_reg": {
			allowMultiRange: true,
		},
		"with_single_reg": {
			allowMultiRange: false,
		},
	}

	for sn, size := range sizeCond {
		for in, innero := range innerOffsetCond {
			for bo, baseo := range baseOffsetCond {
				for bs, blobsize := range blobSizeCond {
					for tc, trCond := range transportCond {
						t.Run(fmt.Sprintf("reading_%s_%s_%s_%s_%s", sn, in, bo, bs, tc), func(t *testing.T) {
							if blobsize > int64(len(sampleData1)) {
								t.Fatal("sample file size is larger than sample data")
							}

							wantN := size //7

							offset := baseo + innero
							if remain := blobsize - offset; remain < wantN {
								if wantN = remain; wantN < 0 {
									wantN = 0
								}
							}

							// use constant string value as a data source.
							want := strings.NewReader(sampleData1)

							// data we want to get.
							wantData := make([]byte, wantN)
							_, err := want.ReadAt(wantData, offset)
							if err != nil && err != io.EOF {
								t.Fatalf("want.ReadAt (offset=%d,size=%d): %v", offset, wantN, err)
							}

							// data we get through a remote blob.
							blob := []byte(sampleData1)[:blobsize]

							// Check with allowing multi range requests
							tr := multiRoundTripper(t, blob, allowMultiRange(trCond.allowMultiRange))

							// Check ReadAt method
							bb1 := makeTestBlob(t, blobsize, tr)
							checkRead(t, wantData, bb1, offset, wantN)
						})
					}

				}
			}
		}
	}
}

func checkRead(t *testing.T, wantData []byte, r *blob, offset int64, wantSize int64) {
	respData := make([]byte, wantSize)
	t.Logf("reading offset:%d, size:%d", offset, wantSize)
	n, err := r.ReadAt(respData, offset)
	if err != nil {
		t.Errorf("failed to read off=%d, size=%d, blobsize=%d: %v", offset, wantSize, r.Size(), err)
		return
	}
	respData = respData[:n]

	if !bytes.Equal(wantData, respData) {
		t.Errorf("off=%d, blobsize=%d; read data{size=%d,data=%q}; want (size=%d,data=%q)",
			offset, r.Size(), len(respData), string(respData), len(wantData), string(wantData))
		return
	}
}

// Tests ReadAt method for failure cases.
func TestFailReadAt(t *testing.T) {

	// test failed http respose.
	r := makeTestBlob(t, int64(len(sampleData1)), failRoundTripper())
	respData := make([]byte, len(sampleData1))
	_, err := r.ReadAt(respData, 0)
	if err == nil || err == io.EOF {
		t.Errorf("must be fail for http failure but err=%v", err)
		return
	}

	// test broken body with allowing multi range
	checkBrokenBody(t, true)  // with allowing multi range
	checkBrokenBody(t, false) // with prohibiting multi range

	// test broken header
	checkBrokenHeader(t, true)  // with allowing multi range
	checkBrokenHeader(t, false) // with prohibiting multi range
}

func checkBrokenBody(t *testing.T, allowMultiRange bool) {
	respData := make([]byte, len(sampleData1))
	r := makeTestBlob(t, int64(len(sampleData1)), brokenBodyRoundTripper(t, []byte(sampleData1), allowMultiRange))
	if _, err := r.ReadAt(respData, 0); err == nil {
		t.Errorf("must be fail for broken full body but err=%v (allowMultiRange=%v)", err, allowMultiRange)
		return
	}
	r = makeTestBlob(t, int64(len(sampleData1)), brokenBodyRoundTripper(t, []byte(sampleData1), allowMultiRange))
	if _, err := r.ReadAt(respData[0:len(sampleData1)/2], 0); err == nil {
		t.Errorf("must be fail for broken multipart body but err=%v (allowMultiRange=%v)", err, allowMultiRange)
		return
	}
}

func checkBrokenHeader(t *testing.T, allowMultiRange bool) {
	r := makeTestBlob(t, int64(len(sampleData1)), brokenHeaderRoundTripper(t, []byte(sampleData1), allowMultiRange))
	respData := make([]byte, len(sampleData1))
	if _, err := r.ReadAt(respData[0:len(sampleData1)/2], 0); err == nil || err == io.EOF {
		t.Errorf("must be fail for broken multipart header but err=%v (allowMultiRange=%v)", err, allowMultiRange)
		return
	}
}

func TestParallelDownloadingBehavior(t *testing.T) {
	type testData struct {
		name           string
		regions        [3]region
		roundtripCount int64
		content        string
	}

	tests := []testData{
		{
			name: "same_regions",
			regions: [3]region{
				{
					b: 0,
					e: 3,
				},
				{
					b: 0,
					e: 3,
				},
				{
					b: 0,
					e: 3,
				},
			},
			roundtripCount: 3,
			content:        "test",
		},
		{
			name: "different_regions",
			regions: [3]region{
				{
					b: 0,
					e: 3,
				},
				{
					b: 4,
					e: 7,
				},
				{
					b: 0,
					e: 9,
				},
			},
			roundtripCount: 3,
			content:        "test123456",
		},
	}

	var wg sync.WaitGroup
	// we always run 3 routines
	routines := 3

	for _, tst := range tests {
		var (
			tr = &callsCountRoundTripper{
				content: tst.content,
			}
			b = &blob{
				fetcher: &httpFetcher{
					realURL:      "test",
					roundTripper: tr,
				},
				size: int64(len(tst.content)),
			}
		)

		start := make(chan struct{})
		wg.Add(routines)
		var contentBytes [3][]byte

		for i := 0; i < routines; i++ {
			reg := tst.regions[i]
			contentBytes[i] = make([]byte, reg.size())
			w := newBytesWriter(contentBytes[i], 0)
			go func() {
				<-start // by blocking on channel start we can ensure that the goroutines will run at approximately the same time
				defer wg.Done()
				b.fetchRange(reg, w, &options{})
			}()
		}
		close(start) // starting
		wg.Wait()

		// We expect the number of round trip calls to be 1, since we are making 5 calls to fetchRange with
		// overlapping intervals.
		if tr.count != tst.roundtripCount {
			t.Errorf("%v test failed: the round trip count should be %v, but was %v", tst.name, tst.roundtripCount, tr.count)
		}
		// Check for contents
		for j := range contentBytes {
			for i := 0; i < int(tst.regions[j].size()); i++ {
				if contentBytes[j][i] != []byte(tst.content)[i] {
					t.Errorf("%v test failed: the output sequence is wrong, wanted %v, got %v", tst.name, []byte(tst.content)[i], contentBytes[j])
					break
				}
			}
		}
	}
}

func makeTestBlob(t *testing.T, size int64, fn RoundTripFunc) *blob {
	var (
		lastCheck     time.Time
		checkInterval time.Duration
	)

	return makeBlob(
		&httpFetcher{
			realURL:      testURL,
			roundTripper: fn,
		},
		size,
		lastCheck,
		checkInterval,
		&Resolver{})
}

func TestCheckInterval(t *testing.T) {
	var (
		tr        = &calledRoundTripper{}
		firstTime = time.Now()
		b         = &blob{
			fetcher: &httpFetcher{
				realURL:      "test",
				roundTripper: tr,
			},
			lastCheck: firstTime,
		}
	)

	check := func(name string, checkInterval time.Duration) (time.Time, bool) {
		beforeUpdate := time.Now()

		time.Sleep(time.Millisecond)

		tr.called = false
		b.checkInterval = checkInterval
		if err := b.Check(); err != nil {
			t.Fatalf("%q: check mustn't be failed", name)
		}

		time.Sleep(time.Millisecond)

		afterUpdate := time.Now()
		if !tr.called {
			return b.lastCheck, false
		}
		if !(b.lastCheck.After(beforeUpdate) && b.lastCheck.Before(afterUpdate)) {
			t.Errorf("%q: updated time must be after %q and before %q but %q", name, beforeUpdate, afterUpdate, b.lastCheck)
		}

		return b.lastCheck, true
	}

	// second time(not expired yet)
	secondTime, called := check("second time", time.Hour)
	if called {
		t.Error("mustn't be checked if not expired")
	}
	if !secondTime.Equal(firstTime) {
		t.Errorf("lastCheck time must be same as first time(%q) but %q", firstTime, secondTime)
	}

	// third time(expired, must be checked)
	if _, called := check("third time", 0); !called {
		t.Error("must be called for the third time")
	}
}

type callsCountRoundTripper struct {
	count   int64
	content string
}

func (c *callsCountRoundTripper) RoundTrip(req *http.Request) (res *http.Response, err error) {
	atomic.AddInt64(&c.count, 1)
	time.Sleep(50 * time.Millisecond) // sleep for 50 milliseconds to emulate the http call and to make sure that we can run tests on parallel goroutines
	convertBody := func(r io.ReadCloser) io.ReadCloser { return r }
	header := make(http.Header)
	header.Add("Content-Length", fmt.Sprintf("%d", len(c.content)))
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     header,
		Body:       convertBody(io.NopCloser(bytes.NewReader([]byte(c.content)))),
	}, nil
}

type calledRoundTripper struct {
	called bool
}

func (c *calledRoundTripper) RoundTrip(req *http.Request) (res *http.Response, err error) {
	c.called = true
	res = &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewReader([]byte("test"))),
	}
	return
}

type RoundTripFunc func(req *http.Request) *http.Response

func (f RoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req), nil
}

type bodyConverter func(r io.ReadCloser) io.ReadCloser
type exceptRegions []region
type allowMultiRange bool

func multiRoundTripper(t *testing.T, contents []byte, opts ...interface{}) RoundTripFunc {
	multiRangeEnable := true
	doNotFetch := []region{}
	convertBody := func(r io.ReadCloser) io.ReadCloser { return r }
	for _, opt := range opts {
		if v, ok := opt.(allowMultiRange); ok {
			multiRangeEnable = bool(v)
		} else if v, ok := opt.(exceptRegions); ok {
			doNotFetch = []region(v)
		} else if v, ok := opt.(bodyConverter); ok {
			convertBody = (func(r io.ReadCloser) io.ReadCloser)(v)
		}
	}
	emptyResponse := func(statusCode int) *http.Response {
		return &http.Response{
			StatusCode: statusCode,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewReader([]byte{})),
		}
	}

	return func(req *http.Request) *http.Response {
		// Validate request
		if req.Method != "GET" || req.URL.String() != testURL {
			return emptyResponse(http.StatusBadRequest)
		}
		ranges := req.Header.Get("Range")
		if ranges == "" {
			return emptyResponse(http.StatusBadRequest)
		}
		if !strings.HasPrefix(ranges, rangeHeaderPrefix) {
			return emptyResponse(http.StatusBadRequest)
		}
		rlist := strings.Split(ranges[len(rangeHeaderPrefix):], ",")
		if len(rlist) == 0 {
			return emptyResponse(http.StatusBadRequest)
		}

		// check this request can be served as one whole blob.
		var sorted []region
		for _, part := range rlist {
			begin, end := parseRangeString(t, part)
			sorted = append(sorted, region{begin, end})
		}
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].b < sorted[j].b
		})
		var sparse bool
		if sorted[0].b == 0 {
			var max int64
			for _, reg := range sorted {
				if reg.e > max {
					if max < reg.b-1 {
						sparse = true
						break
					}
					max = reg.e
				}
			}
			if max >= int64(len(contents)-1) && !sparse {
				t.Logf("serving whole range %q = %d", ranges, len(contents))
				header := make(http.Header)
				header.Add("Content-Length", fmt.Sprintf("%d", len(contents)))
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     header,
					Body:       convertBody(io.NopCloser(bytes.NewReader(contents))),
				}
			}
		}

		if !multiRangeEnable {
			if len(rlist) > 1 {
				return emptyResponse(http.StatusBadRequest) // prohibiting multi range
			}

			// serve as single part response
			begin, end := parseRangeString(t, rlist[0])
			target := region{begin, end}
			for _, reg := range doNotFetch {
				if target.b <= reg.b && reg.e <= target.e {
					t.Fatalf("Requested prohibited region (singlepart): (%d, %d) contained in fetching region (%d, %d)",
						reg.b, reg.e, target.b, target.e)
				}
			}
			header := make(http.Header)
			header.Add("Content-Length", fmt.Sprintf("%d", target.size()))
			header.Add("Content-Range",
				fmt.Sprintf("bytes %d-%d/%d", target.b, target.e, len(contents)))
			header.Add("Content-Type", "application/octet-stream")
			part := contents[target.b : target.e+1]
			return &http.Response{
				StatusCode: http.StatusPartialContent,
				Header:     header,
				Body:       convertBody(io.NopCloser(bytes.NewReader(part))),
			}
		}

		// Write multipart response.
		var buf bytes.Buffer
		mw := multipart.NewWriter(&buf)
		for _, part := range rlist {
			mh := make(textproto.MIMEHeader)
			mh.Set("Content-Range", fmt.Sprintf("bytes %s/%d", part, len(contents)))
			w, err := mw.CreatePart(mh)
			if err != nil {
				t.Fatalf("failed to create part: %v", err)
			}
			begin, end := parseRangeString(t, part)
			if begin >= int64(len(contents)) {
				// skip if out of range.
				continue
			}
			if end > int64(len(contents)-1) {
				end = int64(len(contents) - 1)
			}
			for _, reg := range doNotFetch {
				if begin <= reg.b && reg.e <= end {
					t.Fatalf("Requested prohibited region (multipart): (%d, %d) contained in fetching region (%d, %d)",
						reg.b, reg.e, begin, end)
				}
			}
			if n, err := w.Write(contents[begin : end+1]); err != nil || int64(n) != end+1-begin {
				t.Fatalf("failed to write to part(%d-%d): %v", begin, end, err)
			}
		}
		mw.Close()
		param := map[string]string{
			"boundary": mw.Boundary(),
		}
		header := make(http.Header)
		header.Add("Content-Type", mime.FormatMediaType("multipart/text", param))
		return &http.Response{
			StatusCode: http.StatusPartialContent,
			Header:     header,
			Body:       convertBody(io.NopCloser(&buf)),
		}
	}
}

func failRoundTripper() RoundTripFunc {
	return func(req *http.Request) *http.Response {
		return &http.Response{
			StatusCode: http.StatusInternalServerError,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewReader([]byte{})),
		}
	}
}

func brokenBodyRoundTripper(t *testing.T, contents []byte, multiRange bool) RoundTripFunc {
	breakReadCloser := func(r io.ReadCloser) io.ReadCloser {
		defer r.Close()
		data, err := io.ReadAll(r)
		if err != nil {
			t.Fatalf("failed to break read closer faild to read original: %v", err)
		}
		return io.NopCloser(bytes.NewReader(data[:len(data)/2]))
	}
	tr := multiRoundTripper(t, contents, allowMultiRange(multiRange), bodyConverter(breakReadCloser))
	return func(req *http.Request) *http.Response {
		return tr(req)
	}
}

func brokenHeaderRoundTripper(t *testing.T, contents []byte, multiRange bool) RoundTripFunc {
	tr := multiRoundTripper(t, contents, allowMultiRange(multiRange))
	return func(req *http.Request) *http.Response {
		res := tr(req)
		res.Header = make(http.Header)
		return res
	}
}

func parseRangeString(t *testing.T, rangeString string) (int64, int64) {
	rng := strings.Split(rangeString, "-")
	if len(rng) != 2 {
		t.Fatalf("falied to parse range %q", rng)
	}
	begin, err := strconv.ParseInt(rng[0], 10, 64)
	if err != nil {
		t.Fatalf("failed to parse beginning offset: %v", err)
	}
	end, err := strconv.ParseInt(rng[1], 10, 64)
	if err != nil {
		t.Fatalf("failed to parse ending offset: %v", err)
	}
	return begin, end
}
