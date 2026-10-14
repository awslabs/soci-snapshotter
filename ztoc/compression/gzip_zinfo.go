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
	"fmt"
	"io"
	"os"

	"github.com/awslabs/soci-snapshotter/ztoc/compression/internal/gzipinfo"
)

// GzipZinfo is a go struct wrapping a gzipinfo.Index: a pure-Go,
// cgo-free zran.c-style random-access index into a gzip stream. It
// replaces an earlier cgo/zlib-based implementation of the same
// algorithm; the on-disk Checkpoints blob format (see Bytes/newGzipZinfo)
// is unchanged, so existing ztocs remain readable.
type GzipZinfo struct {
	idx *gzipinfo.Index
}

// newGzipZinfo creates a new instance of `GzipZinfo` from the zinfo byte
// blob stored on a ztoc.
func newGzipZinfo(zinfoBytes []byte) (*GzipZinfo, error) {
	if len(zinfoBytes) == 0 {
		return nil, fmt.Errorf("empty checkpoints")
	}
	idx := &gzipinfo.Index{}
	if err := idx.UnmarshalBinary(zinfoBytes); err != nil {
		return nil, fmt.Errorf("cannot convert blob to gzip zinfo: %w", err)
	}
	return &GzipZinfo{idx: idx}, nil
}

// newGzipZinfoFromFile creates a new instance of `GzipZinfo` given a gzip
// file name and span size.
func newGzipZinfoFromFile(gzipFile string, spanSize int64) (*GzipZinfo, error) {
	f, err := os.Open(gzipFile)
	if err != nil {
		return nil, fmt.Errorf("could not generate gzip zinfo: %w", err)
	}
	defer f.Close()

	idx, err := gzipinfo.BuildIndex(f, spanSize)
	if err != nil {
		return nil, fmt.Errorf("could not generate gzip zinfo: %w", err)
	}
	return &GzipZinfo{idx: idx}, nil
}

// Close is a no-op: GzipZinfo holds no unmanaged resources now that it's
// backed by pure Go rather than C-allocated memory. Kept to satisfy the
// Zinfo interface.
func (i *GzipZinfo) Close() {}

// Bytes returns the byte slice containing the zinfo.
func (i *GzipZinfo) Bytes() ([]byte, error) {
	return i.idx.MarshalBinary()
}

// MaxSpanID returns the max span ID.
func (i *GzipZinfo) MaxSpanID() SpanID {
	return SpanID(i.idx.MaxSpanID())
}

// SpanSize returns the span size of the constructed ztoc.
func (i *GzipZinfo) SpanSize() Offset {
	return Offset(i.idx.SpanSize())
}

// UncompressedOffsetToSpanID returns the ID of the span containing the data pointed by uncompressed offset.
func (i *GzipZinfo) UncompressedOffsetToSpanID(offset Offset) SpanID {
	return SpanID(i.idx.UncompressedOffsetToSpanID(int64(offset)))
}

// ExtractDataFromBuffer extracts the uncompressed data from `compressedBuf` and returns
// it as a byte slice.
func (i *GzipZinfo) ExtractDataFromBuffer(compressedBuf []byte, uncompressedSize, uncompressedOffset Offset, spanID SpanID) ([]byte, error) {
	if len(compressedBuf) == 0 {
		return nil, fmt.Errorf("empty compressed buffer")
	}
	if uncompressedSize < 0 {
		return nil, fmt.Errorf("invalid uncompressed size: %d", uncompressedSize)
	}
	if uncompressedSize == 0 {
		return []byte{}, nil
	}
	data, err := i.idx.ExtractFromBuffer(compressedBuf, int(spanID), int64(uncompressedOffset), int64(uncompressedSize))
	if err != nil {
		return nil, fmt.Errorf("error extracting data: %w", err)
	}
	return data, nil
}

// ExtractDataFromFile returns the decompressed bytes given the name of the .tar.gz file,
// offset and the size in uncompressed stream.
func (i *GzipZinfo) ExtractDataFromFile(fileName string, uncompressedSize, uncompressedOffset Offset) ([]byte, error) {
	if uncompressedSize < 0 {
		return nil, fmt.Errorf("invalid uncompressed size: %d", uncompressedSize)
	}
	if uncompressedSize == 0 {
		return []byte{}, nil
	}
	f, err := os.Open(fileName)
	if err != nil {
		return nil, fmt.Errorf("unable to open file: %w", err)
	}
	defer f.Close()

	data, err := i.idx.ExtractFromFile(f, int64(uncompressedOffset), int64(uncompressedSize))
	if err != nil {
		return nil, fmt.Errorf("unable to extract data: %w", err)
	}
	return data, nil
}

// StartCompressedOffset returns the start offset of the span in the compressed stream.
func (i *GzipZinfo) StartCompressedOffset(spanID SpanID) Offset {
	return Offset(i.idx.StartCompressedOffset(int(spanID)))
}

// EndCompressedOffset returns the end offset of the span in the compressed stream. If
// it's the last span, returns the size of the compressed stream.
func (i *GzipZinfo) EndCompressedOffset(spanID SpanID, fileSize Offset) Offset {
	return Offset(i.idx.EndCompressedOffset(int(spanID), int64(fileSize)))
}

// StartUncompressedOffset returns the start offset of the span in the uncompressed stream.
func (i *GzipZinfo) StartUncompressedOffset(spanID SpanID) Offset {
	return Offset(i.idx.StartUncompressedOffset(int(spanID)))
}

// EndUncompressedOffset returns the end offset of the span in the uncompressed stream. If
// it's the last span, returns the size of the uncompressed stream.
func (i *GzipZinfo) EndUncompressedOffset(spanID SpanID, fileSize Offset) Offset {
	return Offset(i.idx.EndUncompressedOffset(int(spanID), int64(fileSize)))
}

// VerifyHeader checks if the given zinfo has a proper header
func (i *GzipZinfo) VerifyHeader(r io.Reader) error {
	return i.idx.VerifyHeader(r)
}
