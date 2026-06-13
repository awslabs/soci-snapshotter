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

package compression_test

import (
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/awslabs/soci-snapshotter/util/testutil"
	"github.com/awslabs/soci-snapshotter/ztoc/compression"
	"github.com/awslabs/soci-snapshotter/ztoc/compression/purego"
)

type compatTestCase struct {
	name             string
	entries          []testutil.TarEntry
	compressionLevel int
	spanSize         int64
	opts             []testutil.BuildTarOption
}

func getCompatTestCases(t *testing.T) []compatTestCase {
	rng := testutil.NewTestRand(t)

	return []compatTestCase{
		{
			name: "small file (< 1 span)",
			entries: []testutil.TarEntry{
				testutil.File("small.txt", "Hello, World!"),
			},
			compressionLevel: gzip.DefaultCompression,
			spanSize:         1 << 20,
		},
		{
			name: "medium file (5-10 spans)",
			entries: []testutil.TarEntry{
				testutil.File("medium.bin", string(rng.RandomByteData(200000))),
			},
			compressionLevel: gzip.DefaultCompression,
			spanSize:         32768,
		},
		{
			name: "large file (50+ spans)",
			entries: []testutil.TarEntry{
				testutil.File("large.bin", string(rng.RandomByteData(500000))),
			},
			compressionLevel: gzip.DefaultCompression,
			spanSize:         8192,
		},
		{
			name: "highly compressible data",
			entries: []testutil.TarEntry{
				testutil.File("zeros.bin", strings.Repeat("\x00", 100000)),
			},
			compressionLevel: gzip.DefaultCompression,
			spanSize:         32768,
		},
		{
			name: "compression level 1",
			entries: []testutil.TarEntry{
				testutil.File("data.bin", string(rng.RandomByteData(100000))),
			},
			compressionLevel: gzip.BestSpeed,
			spanSize:         32768,
		},
		{
			name: "compression level 9",
			entries: []testutil.TarEntry{
				testutil.File("data.bin", string(rng.RandomByteData(100000))),
			},
			compressionLevel: gzip.BestCompression,
			spanSize:         32768,
		},
		{
			name: "empty tar entry",
			entries: []testutil.TarEntry{
				testutil.File("empty.txt", ""),
				testutil.File("notempty.txt", "some content"),
			},
			compressionLevel: gzip.DefaultCompression,
			spanSize:         32768,
		},
		{
			name: "single byte file",
			entries: []testutil.TarEntry{
				testutil.File("one.txt", "x"),
			},
			compressionLevel: gzip.DefaultCompression,
			spanSize:         32768,
		},
		{
			name: "multiple files",
			entries: []testutil.TarEntry{
				testutil.Dir("dir/"),
				testutil.File("dir/a.txt", string(rng.RandomByteData(50000))),
				testutil.File("dir/b.txt", string(rng.RandomByteData(50000))),
				testutil.File("dir/c.txt", string(rng.RandomByteData(50000))),
			},
			compressionLevel: gzip.DefaultCompression,
			spanSize:         32768,
		},
		{
			name: "with gzip extra fields",
			entries: []testutil.TarEntry{
				testutil.File("data.txt", string(rng.RandomByteData(50000))),
			},
			compressionLevel: gzip.DefaultCompression,
			spanSize:         32768,
			opts: []testutil.BuildTarOption{
				testutil.WithGzipComment("test comment"),
				testutil.WithGzipFilename("testfile.tar"),
				testutil.WithGzipExtra([]byte("extra")),
			},
		},
	}
}

func writeTarGzToTemp(t *testing.T, tc compatTestCase) (string, []byte, []byte) {
	t.Helper()
	tarGzReader := testutil.BuildTarGz(tc.entries, tc.compressionLevel, tc.opts...)
	tmpFile, gzData, err := testutil.WriteTarToTempFile("compat-*.tar.gz", tarGzReader)
	if err != nil {
		t.Fatal(err)
	}

	gz, err := gzip.NewReader(bytes.NewReader(gzData))
	if err != nil {
		t.Fatal(err)
	}
	uncompressed, err := io.ReadAll(gz)
	gz.Close()
	if err != nil {
		t.Fatal(err)
	}

	return tmpFile, gzData, uncompressed
}

func TestCompatIndexGeneration(t *testing.T) {
	t.Parallel()
	for _, tc := range getCompatTestCases(t) {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tmpFile, _, _ := writeTarGzToTemp(t, tc)
			defer os.Remove(tmpFile)

			// C implementation (via exported factory).
			cZinfo, err := compression.NewZinfoFromFile(compression.Gzip, tmpFile, tc.spanSize)
			if err != nil {
				t.Fatalf("C NewZinfoFromFile failed: %v", err)
			}
			defer cZinfo.Close()

			// Go implementation.
			goZinfo, err := purego.NewGzipZinfoFromFile(tmpFile, tc.spanSize)
			if err != nil {
				t.Fatalf("Go NewGzipZinfoFromFile failed: %v", err)
			}
			defer goZinfo.Close()

			if cZinfo.MaxSpanID() != goZinfo.MaxSpanID() {
				t.Fatalf("MaxSpanID mismatch: C=%d Go=%d", cZinfo.MaxSpanID(), goZinfo.MaxSpanID())
			}
			if cZinfo.SpanSize() != goZinfo.SpanSize() {
				t.Fatalf("SpanSize mismatch: C=%d Go=%d", cZinfo.SpanSize(), goZinfo.SpanSize())
			}

			cBlob, err := cZinfo.Bytes()
			if err != nil {
				t.Fatalf("C Bytes() failed: %v", err)
			}
			goBlob, err := goZinfo.Bytes()
			if err != nil {
				t.Fatalf("Go Bytes() failed: %v", err)
			}

			if !bytes.Equal(cBlob, goBlob) {
				// Log details for debugging.
				t.Logf("C blob len=%d, Go blob len=%d", len(cBlob), len(goBlob))
				t.Logf("C  MaxSpanID=%d, Go MaxSpanID=%d", cZinfo.MaxSpanID(), goZinfo.MaxSpanID())
				for sid := compression.SpanID(0); sid <= cZinfo.MaxSpanID(); sid++ {
					t.Logf("C  span %d: startComp=%d startUncomp=%d",
						sid, cZinfo.StartCompressedOffset(sid), cZinfo.StartUncompressedOffset(sid))
				}
				for sid := compression.SpanID(0); sid <= goZinfo.MaxSpanID(); sid++ {
					t.Logf("Go span %d: startComp=%d startUncomp=%d",
						sid, goZinfo.StartCompressedOffset(sid), goZinfo.StartUncompressedOffset(sid))
				}
				t.Fatalf("serialized blobs differ")
			}
		})
	}
}

func TestCompatDeserialization(t *testing.T) {
	t.Parallel()
	for _, tc := range getCompatTestCases(t) {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tmpFile, _, _ := writeTarGzToTemp(t, tc)
			defer os.Remove(tmpFile)

			cZinfo, err := compression.NewZinfoFromFile(compression.Gzip, tmpFile, tc.spanSize)
			if err != nil {
				t.Fatalf("C NewZinfoFromFile failed: %v", err)
			}
			defer cZinfo.Close()

			cBlob, err := cZinfo.Bytes()
			if err != nil {
				t.Fatalf("C Bytes() failed: %v", err)
			}

			goZinfo, err := purego.NewGzipZinfo(cBlob)
			if err != nil {
				t.Fatalf("Go NewGzipZinfo from C blob failed: %v", err)
			}
			defer goZinfo.Close()

			if cZinfo.MaxSpanID() != goZinfo.MaxSpanID() {
				t.Fatalf("MaxSpanID mismatch: C=%d Go=%d", cZinfo.MaxSpanID(), goZinfo.MaxSpanID())
			}
			if cZinfo.SpanSize() != goZinfo.SpanSize() {
				t.Fatalf("SpanSize mismatch: C=%d Go=%d", cZinfo.SpanSize(), goZinfo.SpanSize())
			}

			for sid := compression.SpanID(0); sid <= cZinfo.MaxSpanID(); sid++ {
				if cZinfo.StartCompressedOffset(sid) != goZinfo.StartCompressedOffset(sid) {
					t.Fatalf("span %d StartCompressedOffset: C=%d Go=%d",
						sid, cZinfo.StartCompressedOffset(sid), goZinfo.StartCompressedOffset(sid))
				}
				if cZinfo.StartUncompressedOffset(sid) != goZinfo.StartUncompressedOffset(sid) {
					t.Fatalf("span %d StartUncompressedOffset: C=%d Go=%d",
						sid, cZinfo.StartUncompressedOffset(sid), goZinfo.StartUncompressedOffset(sid))
				}
			}

			for _, off := range []compression.Offset{0, 1, 100, 1000, 10000, 50000} {
				cSid := cZinfo.UncompressedOffsetToSpanID(off)
				goSid := goZinfo.UncompressedOffsetToSpanID(off)
				if cSid != goSid {
					t.Fatalf("UncompressedOffsetToSpanID(%d): C=%d Go=%d", off, cSid, goSid)
				}
			}
		})
	}
}

func TestCompatExtraction(t *testing.T) {
	t.Parallel()
	for _, tc := range getCompatTestCases(t) {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tmpFile, gzData, uncompressed := writeTarGzToTemp(t, tc)
			defer os.Remove(tmpFile)

			cZinfo, err := compression.NewZinfoFromFile(compression.Gzip, tmpFile, tc.spanSize)
			if err != nil {
				t.Fatalf("C NewZinfoFromFile failed: %v", err)
			}
			defer cZinfo.Close()

			goZinfo, err := purego.NewGzipZinfoFromFile(tmpFile, tc.spanSize)
			if err != nil {
				t.Fatalf("Go NewGzipZinfoFromFile failed: %v", err)
			}
			defer goZinfo.Close()

			fileSize := compression.Offset(len(gzData))
			uncompressedSize := compression.Offset(len(uncompressed))

			for sid := compression.SpanID(0); sid <= cZinfo.MaxSpanID(); sid++ {
				startComp := cZinfo.StartCompressedOffset(sid)
				endComp := cZinfo.EndCompressedOffset(sid, fileSize)
				compBuf := gzData[startComp:endComp]

				startUncomp := cZinfo.StartUncompressedOffset(sid)
				endUncomp := cZinfo.EndUncompressedOffset(sid, uncompressedSize)
				size := endUncomp - startUncomp

				if size <= 0 {
					continue
				}

				cData, err := cZinfo.ExtractDataFromBuffer(compBuf, size, startUncomp, sid)
				if err != nil {
					t.Fatalf("span %d: C ExtractDataFromBuffer failed: %v", sid, err)
				}

				goData, err := goZinfo.ExtractDataFromBuffer(compBuf, size, startUncomp, sid)
				if err != nil {
					t.Fatalf("span %d: Go ExtractDataFromBuffer failed: %v", sid, err)
				}

				expected := uncompressed[startUncomp:endUncomp]
				if !bytes.Equal(cData[:len(expected)], expected) {
					t.Fatalf("span %d: C extraction doesn't match expected", sid)
				}
				if !bytes.Equal(goData[:len(expected)], expected) {
					t.Fatalf("span %d: Go extraction doesn't match expected", sid)
				}
			}
		})
	}
}

func TestCompatExtractionFromFile(t *testing.T) {
	t.Parallel()
	for _, tc := range getCompatTestCases(t) {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tmpFile, _, uncompressed := writeTarGzToTemp(t, tc)
			defer os.Remove(tmpFile)

			cZinfo, err := compression.NewZinfoFromFile(compression.Gzip, tmpFile, tc.spanSize)
			if err != nil {
				t.Fatalf("C NewZinfoFromFile failed: %v", err)
			}
			defer cZinfo.Close()

			goZinfo, err := purego.NewGzipZinfoFromFile(tmpFile, tc.spanSize)
			if err != nil {
				t.Fatalf("Go NewGzipZinfoFromFile failed: %v", err)
			}
			defer goZinfo.Close()

			offsets := []compression.Offset{0, 512, 1024}
			size := compression.Offset(256)

			for _, off := range offsets {
				if off+size > compression.Offset(len(uncompressed)) {
					continue
				}

				cData, err := cZinfo.ExtractDataFromFile(tmpFile, size, off)
				if err != nil {
					t.Fatalf("offset %d: C ExtractDataFromFile failed: %v", off, err)
				}

				goData, err := goZinfo.ExtractDataFromFile(tmpFile, size, off)
				if err != nil {
					t.Fatalf("offset %d: Go ExtractDataFromFile failed: %v", off, err)
				}

				expected := uncompressed[off : off+size]
				if !bytes.Equal(cData, expected) {
					t.Fatalf("offset %d: C extraction mismatch", off)
				}
				if !bytes.Equal(goData, expected) {
					t.Fatalf("offset %d: Go extraction mismatch", off)
				}
			}
		})
	}
}

func TestCompatCrossImplementationRoundTrip(t *testing.T) {
	t.Parallel()
	for _, tc := range getCompatTestCases(t) {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tmpFile, gzData, uncompressed := writeTarGzToTemp(t, tc)
			defer os.Remove(tmpFile)

			fileSize := compression.Offset(len(gzData))
			uncompressedSize := compression.Offset(len(uncompressed))

			// C → serialize → Go deserialize → Go extract.
			cZinfo, err := compression.NewZinfoFromFile(compression.Gzip, tmpFile, tc.spanSize)
			if err != nil {
				t.Fatalf("C NewZinfoFromFile failed: %v", err)
			}
			cBlob, err := cZinfo.Bytes()
			cZinfo.Close()
			if err != nil {
				t.Fatalf("C Bytes() failed: %v", err)
			}

			goZinfo, err := purego.NewGzipZinfo(cBlob)
			if err != nil {
				t.Fatalf("Go NewGzipZinfo from C blob failed: %v", err)
			}

			for sid := compression.SpanID(0); sid <= goZinfo.MaxSpanID(); sid++ {
				startComp := goZinfo.StartCompressedOffset(sid)
				endComp := goZinfo.EndCompressedOffset(sid, fileSize)
				compBuf := gzData[startComp:endComp]

				startUncomp := goZinfo.StartUncompressedOffset(sid)
				endUncomp := goZinfo.EndUncompressedOffset(sid, uncompressedSize)
				size := endUncomp - startUncomp
				if size <= 0 {
					continue
				}

				goData, err := goZinfo.ExtractDataFromBuffer(compBuf, size, startUncomp, sid)
				if err != nil {
					t.Fatalf("span %d: Go extract from C index failed: %v", sid, err)
				}

				expected := uncompressed[startUncomp:endUncomp]
				if !bytes.Equal(goData[:len(expected)], expected) {
					t.Fatalf("span %d: C→Go round-trip mismatch", sid)
				}
			}
			goZinfo.Close()

			// Go → serialize → C deserialize → C extract.
			goZinfo2, err := purego.NewGzipZinfoFromFile(tmpFile, tc.spanSize)
			if err != nil {
				t.Fatalf("Go NewGzipZinfoFromFile failed: %v", err)
			}
			goBlob, err := goZinfo2.Bytes()
			goZinfo2.Close()
			if err != nil {
				t.Fatalf("Go Bytes() failed: %v", err)
			}

			cZinfo2, err := compression.NewZinfo(compression.Gzip, goBlob)
			if err != nil {
				t.Fatalf("C NewZinfo from Go blob failed: %v", err)
			}
			defer cZinfo2.Close()

			for sid := compression.SpanID(0); sid <= cZinfo2.MaxSpanID(); sid++ {
				startComp := cZinfo2.StartCompressedOffset(sid)
				endComp := cZinfo2.EndCompressedOffset(sid, fileSize)
				compBuf := gzData[startComp:endComp]

				startUncomp := cZinfo2.StartUncompressedOffset(sid)
				endUncomp := cZinfo2.EndUncompressedOffset(sid, uncompressedSize)
				size := endUncomp - startUncomp
				if size <= 0 {
					continue
				}

				cData, err := cZinfo2.ExtractDataFromBuffer(compBuf, size, startUncomp, sid)
				if err != nil {
					t.Fatalf("span %d: C extract from Go index failed: %v", sid, err)
				}

				expected := uncompressed[startUncomp:endUncomp]
				if !bytes.Equal(cData[:len(expected)], expected) {
					t.Fatalf("span %d: Go→C round-trip mismatch", sid)
				}
			}
		})
	}
}
