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

package compression

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/klauspost/compress/zstd"
)

func zstdFuzzSeeds(f *testing.F) [][]byte {
	data := compressibleData(7, 48<<10)
	var seeds [][]byte
	for _, opts := range [][]zstd.EOption{
		{zstd.WithWindowSize(1 << 10)},
		{zstd.WithWindowSize(1 << 16), zstd.WithEncoderLevel(zstd.SpeedBestCompression)},
		{zstd.WithEncoderCRC(false), zstd.WithSingleSegment(true)},
	} {
		enc, err := zstd.NewWriter(nil, opts...)
		if err != nil {
			f.Fatal(err)
		}
		seeds = append(seeds, enc.EncodeAll(data, nil))
		enc.Close()
	}
	return seeds
}

// FuzzZstdZinfoFromStream checks that a zinfo built from any stream either
// fails or extracts the same data as a full decompression.
func FuzzZstdZinfoFromStream(f *testing.F) {
	for _, s := range zstdFuzzSeeds(f) {
		f.Add(s, uint32(16<<10))
	}
	dec, err := zstd.NewReader(nil, zstd.WithDecoderMaxWindow(zstdMaxWindowSize))
	if err != nil {
		f.Fatal(err)
	}
	defer dec.Close()
	f.Fuzz(func(t *testing.T, stream []byte, span uint32) {
		spanSize := int64(span%(1<<20)) + 1<<10
		path := filepath.Join(t.TempDir(), "layer.zst")
		if err := os.WriteFile(path, stream, 0600); err != nil {
			t.Fatal(err)
		}
		built, err := newZstdZinfoFromFile(path, spanSize)
		if err != nil {
			return
		}
		expected, err := dec.DecodeAll(stream, nil)
		if err != nil {
			t.Fatalf("zinfo was built, but the stream does not decode: %v", err)
		}
		b, err := built.Bytes()
		if err != nil {
			t.Fatal(err)
		}
		zinfo, err := newZstdZinfo(b)
		if err != nil {
			t.Fatalf("cannot deserialize zinfo: %v", err)
		}
		fileSize, size := Offset(len(stream)), Offset(len(expected))
		for id := SpanID(0); id <= zinfo.MaxSpanID(); id++ {
			cs, ce := zinfo.StartCompressedOffset(id), zinfo.EndCompressedOffset(id, fileSize)
			us, ue := zinfo.StartUncompressedOffset(id), zinfo.EndUncompressedOffset(id, size)
			got, err := zinfo.ExtractDataFromBuffer(stream[cs:ce], ue-us, us, id)
			if err != nil {
				t.Fatalf("span %d: %v", id, err)
			}
			if !bytes.Equal(got, expected[us:ue]) {
				t.Fatalf("span %d: extracted data differs", id)
			}
		}
	})
}

// FuzzNewZstdZinfo checks that a corrupt serialized zinfo is rejected or
// fails to extract data, without a panic.
func FuzzNewZstdZinfo(f *testing.F) {
	seeds := zstdFuzzSeeds(f)
	for _, s := range seeds {
		path := filepath.Join(f.TempDir(), "layer.zst")
		if err := os.WriteFile(path, s, 0600); err != nil {
			f.Fatal(err)
		}
		built, err := newZstdZinfoFromFile(path, 16<<10)
		if err != nil {
			f.Fatal(err)
		}
		b, err := built.Bytes()
		if err != nil {
			f.Fatal(err)
		}
		f.Add(b)
	}
	stream := seeds[1]
	f.Fuzz(func(t *testing.T, b []byte) {
		zinfo, err := newZstdZinfo(b)
		if err != nil {
			return
		}
		fileSize := Offset(len(stream))
		for id := SpanID(0); id <= zinfo.MaxSpanID() && id < 64; id++ {
			cs, ce := zinfo.StartCompressedOffset(id), zinfo.EndCompressedOffset(id, fileSize)
			if cs < 0 || ce > fileSize || cs > ce {
				continue
			}
			us, ue := zinfo.StartUncompressedOffset(id), zinfo.EndUncompressedOffset(id, 1<<20)
			if ue < us || ue-us > 1<<20 {
				continue
			}
			_, _ = zinfo.ExtractDataFromBuffer(stream[cs:ce], ue-us, us, id)
		}
	})
}
