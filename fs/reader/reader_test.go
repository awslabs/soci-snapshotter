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
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/awslabs/soci-snapshotter/cache"
	spanmanager "github.com/awslabs/soci-snapshotter/fs/span-manager"
	"github.com/awslabs/soci-snapshotter/metadata"
	"github.com/awslabs/soci-snapshotter/util/testutil"
	"github.com/awslabs/soci-snapshotter/ztoc"
	digest "github.com/opencontainers/go-digest"
)

const (
	sampleSpanSize     = 3
	sampleMiddleOffset = sampleSpanSize / 2
	sampleData1        = "0123456789"
	lastSpanOffset1    = sampleSpanSize * (int64(len(sampleData1)) / sampleSpanSize)
)

var spanSizeCond = [3]int64{64, 128, 256}

func TestFsReader(t *testing.T) {
	testFileReadAt(t, metadata.NewTempDbStore)
	testFailReader(t, metadata.NewTempDbStore)
}

func testFileReadAt(t *testing.T, factory metadata.Store) {
	sizeCond := map[string]int64{
		"single_span": sampleSpanSize - sampleMiddleOffset,
		"multi_spans": sampleSpanSize + sampleMiddleOffset,
	}
	innerOffsetCond := map[string]int64{
		"at_top":    0,
		"at_middle": sampleMiddleOffset,
	}
	baseOffsetCond := map[string]int64{
		"of_1st_span":  sampleSpanSize * 0,
		"of_2nd_span":  sampleSpanSize * 1,
		"of_last_span": lastSpanOffset1,
	}
	fileSizeCond := map[string]int64{
		"in_1_span_file":   sampleSpanSize * 1,
		"in_2_spans_file":  sampleSpanSize * 2,
		"in_max_size_file": int64(len(sampleData1)),
	}
	for sn, size := range sizeCond {
		for in, innero := range innerOffsetCond {
			for bo, baseo := range baseOffsetCond {
				for fn, filesize := range fileSizeCond {
					for _, spanSize := range spanSizeCond {
						t.Run(fmt.Sprintf("reading_%s_%s_%s_%s_spansize_%d", sn, in, bo, fn, spanSize), func(t *testing.T) {
							if filesize > int64(len(sampleData1)) {
								t.Fatal("sample file size is larger than sample data")
							}

							wantN := size
							offset := baseo + innero
							if offset >= filesize {
								return
							}
							if remain := filesize - offset; remain < wantN {
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

							// data we get through a file.
							f, closeFn := makeFile(t, []byte(sampleData1)[:filesize], factory, spanSize)
							defer closeFn()

							// read the file
							respData := make([]byte, size)
							n, err := f.ReadAt(respData, offset)
							if err != nil && err != io.EOF {
								t.Fatalf("failed to read off=%d, size=%d, filesize=%d: %v", offset, size, filesize, err)
							}
							respData = respData[:n]
							if !bytes.Equal(wantData, respData) {
								t.Errorf("off=%d, filesize=%d; read data{size=%d,data=%q}; want (size=%d,data=%q)",
									offset, filesize, len(respData), string(respData), wantN, string(wantData))
							}
						})
					}
				}
			}
		}
	}
}

func makeFile(t *testing.T, contents []byte, factory metadata.Store, spanSize int64) (*file, func() error) {
	testName := "test"
	tarEntry := []testutil.TarEntry{
		testutil.File(testName, string(contents)),
	}
	ztoc, sr, err := ztoc.BuildZtocReader(t, tarEntry, gzip.DefaultCompression, spanSize)
	if err != nil {
		t.Fatalf("failed to build sample ztoc: %v", err)
	}

	mr, err := factory(sr, ztoc.TOC)
	if err != nil {
		t.Fatalf("failed to create reader: %v", err)
	}
	spanManager := spanmanager.New(ztoc, sr, cache.NewMemoryCache(), 0)
	vr, err := NewReader(mr, digest.FromString(""), spanManager)
	if err != nil {
		mr.Close()
		t.Fatalf("failed to make new reader: %v", err)
	}
	r := vr.GetReader()
	tid, _, err := mr.GetChild(mr.RootID(), testName)
	if err != nil {
		vr.Close()
		t.Fatalf("failed to get %q: %v", testName, err)
	}
	ra, err := r.OpenFile(tid)
	if err != nil {
		vr.Close()
		t.Fatalf("Failed to open testing file: %v", err)
	}
	f, ok := ra.(*file)
	if !ok {
		vr.Close()
		t.Fatalf("invalid type of file %q", tid)
	}
	return f, vr.Close
}

func testFailReader(t *testing.T, factory metadata.Store) {
	testFileName := "test"
	tarEntry := []testutil.TarEntry{
		testutil.File(testFileName, sampleData1),
	}
	for _, spanSize := range spanSizeCond {
		t.Run(fmt.Sprintf("reading_spansize_%d", spanSize), func(t *testing.T) {
			ztoc, sr, err := ztoc.BuildZtocReader(t, tarEntry, gzip.DefaultCompression, spanSize)
			if err != nil {
				t.Fatalf("failed to build sample ztoc: %v", err)
			}

			// build a metadata reader
			mr, err := factory(sr, ztoc.TOC)
			if err != nil {
				t.Fatalf("failed to prepare metadata reader")
			}
			defer mr.Close()

			// tests for opening non-existing file
			notexist := uint32(0)
			found := false
			for i := uint32(0); i < 1000000; i++ {
				if _, err := mr.GetAttr(i); err != nil {
					notexist, found = i, true
					break
				}
			}
			if !found {
				t.Fatalf("free ID not found")
			}
			spanManager := spanmanager.New(ztoc, sr, cache.NewMemoryCache(), 0)
			vr, err := NewReader(mr, digest.FromString(""), spanManager)
			if err != nil {
				mr.Close()
				t.Fatalf("failed to make new reader: %v", err)
			}
			r := vr.GetReader()

			_, err = r.OpenFile(notexist)
			if err == nil {
				t.Errorf("succeeded to open file but wanted to fail")
			}

			// tests failure behaviour of a file read
			tid, _, err := mr.GetChild(mr.RootID(), testFileName)
			if err != nil {
				t.Fatalf("failed to get %q: %v", testFileName, err)
			}
			fr, err := r.OpenFile(tid)
			if err != nil {
				t.Fatalf("failed to open file but wanted to succeed: %v", err)
			}

			// tests for reading file
			p := make([]byte, len(sampleData1))
			n, err := fr.ReadAt(p, 0)
			if (err != nil && err != io.EOF) || n != len(sampleData1) || !bytes.Equal([]byte(sampleData1), p) {
				t.Errorf("failed to read data but wanted to succeed: %v", err)
			}
		})
	}
}
