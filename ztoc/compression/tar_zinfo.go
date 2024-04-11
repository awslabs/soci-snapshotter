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

	zinfo_flatbuffers "github.com/awslabs/soci-snapshotter/ztoc/compression/fbs/zinfo"
	flatbuffers "github.com/google/flatbuffers/go"
)

const (
	// `TarZinfo` version. consistent with `GzipZinfo` version
	zinfoVersion = 2
)

// TarZinfo implements the `Zinfo` interface for uncompressed tar files.
// It only needs a span size and tar file size, since a tar file is already
// uncompressed.
// For tar file, `compressed`-related concepts (e.g., `CompressedArchiveSize`)
// are only to santisfy the `Zinfo` interface and equal to their `uncompressed`-equivalent.
type TarZinfo struct {
	version  int32
	spanSize int64
	size     int64
}

// newTarZinfo creates a new instance of `TarZinfo` from serialized bytes.
func newTarZinfo(zinfoBytes []byte) (zinfo *TarZinfo, err error) {
	defer func() {
		if r := recover(); r != nil {
			zinfo = nil
			err = fmt.Errorf("cannot unmarshal tar zinfo: %w", err)
		}
	}()

	zinfo = new(TarZinfo)
	zinfoFlatbuf := zinfo_flatbuffers.GetRootAsTarZinfo(zinfoBytes, 0)
	zinfo.version = zinfoFlatbuf.Version()
	zinfo.spanSize = zinfoFlatbuf.SpanSize()
	zinfo.size = zinfoFlatbuf.Size()

	return zinfo, nil
}

// newTarZinfoFromFile creates a new instance of `TarZinfo` given tar file name and span size.
func newTarZinfoFromFile(tarFile string, spanSize int64) (*TarZinfo, error) {
	fstat, err := os.Stat(tarFile)
	if err != nil {
		return nil, fmt.Errorf("unable to get file stat: %w", err)
	}

	return &TarZinfo{
		version:  zinfoVersion,
		spanSize: spanSize,
		size:     fstat.Size(),
	}, nil
}

// Close doesn't do anything since there is nothing to close/release.
func (i *TarZinfo) Close() {}

// Bytes returns the byte slice containing the `TarZinfo`. Integers are serialized
// to `LittleEndian` binaries.
func (i *TarZinfo) Bytes() (fb []byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			fb = nil
			err = fmt.Errorf("failed to generate tar zinfo flatbuf bytes: %w", err)
		}
	}()

	builder := flatbuffers.NewBuilder(0)
	zinfo_flatbuffers.TarZinfoStart(builder)
	zinfo_flatbuffers.TarZinfoAddVersion(builder, i.version)
	zinfo_flatbuffers.TarZinfoAddSpanSize(builder, i.spanSize)
	zinfo_flatbuffers.TarZinfoAddSize(builder, i.size)
	tarZinfoFlatbuf := zinfo_flatbuffers.TarZinfoEnd(builder)
	builder.Finish(tarZinfoFlatbuf)
	return builder.FinishedBytes(), nil
}

// MaxSpanID returns the max span ID.
func (i *TarZinfo) MaxSpanID() SpanID {
	res := SpanID(i.size / i.spanSize)
	if i.size%i.spanSize == 0 {
		res--
	}
	return res
}

// SpanSize returns the span size of the constructed zinfo.
func (i *TarZinfo) SpanSize() Offset {
	return Offset(i.spanSize)
}

// UncompressedOffsetToSpanID returns the ID of the span containing the data pointed by uncompressed offset.
func (i *TarZinfo) UncompressedOffsetToSpanID(offset Offset) SpanID {
	return SpanID(int64(offset) / i.spanSize)
}

// ExtractDataFromBuffer does sanity checks and returns the bytes specified by
// offset and size from the buffer, since for tar file the buffer is already uncompressed.
func (i *TarZinfo) ExtractDataFromBuffer(compressedBuf []byte, uncompressedSize, uncompressedOffset Offset, spanID SpanID) ([]byte, error) {
	if len(compressedBuf) == 0 {
		return nil, fmt.Errorf("empty compressed buffer")
	}
	if uncompressedSize < 0 {
		return nil, fmt.Errorf("invalid uncompressed size: %d", uncompressedSize)
	}
	if uncompressedSize == 0 {
		return []byte{}, nil
	}

	// minus offset from spans before `spanID`.
	uncompressedOffset -= i.StartUncompressedOffset(spanID)
	return compressedBuf[uncompressedOffset : uncompressedOffset+uncompressedSize], nil
}

// ExtractDataFromFile does sanity checks and returns the bytes specified by
// offset and size by reading from the tar file, since for tar file the buffer is already uncompressed.
func (i *TarZinfo) ExtractDataFromFile(fileName string, uncompressedSize, uncompressedOffset Offset) ([]byte, error) {
	if uncompressedSize < 0 {
		return nil, fmt.Errorf("invalid uncompressed size: %d", uncompressedSize)
	}
	if uncompressedSize == 0 {
		return []byte{}, nil
	}

	f, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	bytes := make([]byte, uncompressedSize)
	if n, err := f.ReadAt(bytes, int64(uncompressedOffset)); err != nil || Offset(n) != uncompressedSize {
		return nil, fmt.Errorf("failed to extract data. expect length: %d, actual length: %d", uncompressedSize, n)
	}
	return bytes, nil
}

// Notice that for tar files, compressed and uncompressed means the same thing
// since tar file is already uncompressed.

// StartCompressedOffset returns the start offset of the span in the compressed stream.
func (i *TarZinfo) StartCompressedOffset(spanID SpanID) Offset {
	return i.spanIDToOffset(spanID)
}

// EndCompressedOffset returns the end offset of the span in the compressed stream. If
// it's the last span, returns the size of the compressed stream.
func (i *TarZinfo) EndCompressedOffset(spanID SpanID, fileSize Offset) Offset {
	if spanID == i.MaxSpanID() {
		return fileSize
	}
	return i.spanIDToOffset(spanID + 1)
}

// StartUncompressedOffset returns the start offset of the span in the uncompressed stream.
func (i *TarZinfo) StartUncompressedOffset(spanID SpanID) Offset {
	return i.spanIDToOffset(spanID)
}

// EndUncompressedOffset returns the end offset of the span in the uncompressed stream. If
// it's the last span, returns the size of the uncompressed stream.
func (i *TarZinfo) EndUncompressedOffset(spanID SpanID, fileSize Offset) Offset {
	if spanID == i.MaxSpanID() {
		return fileSize
	}
	return i.spanIDToOffset(spanID + 1)
}

// VerifyHeader checks if the given zinfo has a proper header
func (i *TarZinfo) VerifyHeader(r io.Reader) error {
	// As this is a catch-all for all compression algorithms,
	// there's not really a way to verify the header,
	// so blindly assume it's correct.
	return nil
}

func (i *TarZinfo) spanIDToOffset(spanID SpanID) Offset {
	return Offset(i.spanSize * int64(spanID))
}
