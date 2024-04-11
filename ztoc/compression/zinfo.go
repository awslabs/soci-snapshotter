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
)

// Zinfo is the interface for dealing with compressed data efficiently. It chunks
// a compressed stream (e.g. a gzip file) into spans and records the chunk offset,
// so that you can interact with the compressed stream per span individually (or in parallel).
// For example, you can extract uncompressed data/file from the relevant compressed
// spans only (i.e., without uncompressing the whole compress file).
//
// The interface contains methods that are used to:
//  1. build a zinfo (e.g., `SpanSize`);
//  2. extract a chunk of uncompressed data (e.g., from a compressed buffer or file);
//  3. conversion between span and its start and end offset in the (un)compressed
//     stream so you can work on the individual span data only.
type Zinfo interface {
	// ExtractDataFromBuffer extracts the uncompressed data from `compressedBuf` and returns
	// as a byte slice.
	ExtractDataFromBuffer(compressedBuf []byte, uncompressedSize, uncompressedOffset Offset, spanID SpanID) ([]byte, error)
	// ExtractDataFromFile extracts the uncompressed data directly from a compressed file
	// (e.g. a gzip file) and returns as a byte slice.
	ExtractDataFromFile(fileName string, uncompressedSize, uncompressedOffset Offset) ([]byte, error)
	// Close releases any resources held by the interface implementation.
	Close()

	// Bytes serilizes the underlying zinfo data (depending on implementation) into bytes for storage.
	Bytes() ([]byte, error)
	// MaxSpanID returns the maximum span ID after chunking the compress stream into spans.
	MaxSpanID() SpanID
	// SpanSize returns the span size used to chunk compress stream into spans.
	SpanSize() Offset

	// Below funcs need to be part of the interface because of how we currently
	// extract data from a compressed data stream. Specifically, if we need to
	// extract a chunk of uncompressed data (e.g. `[start:end]`), we need to:
	// 1. get the span id of both `start` and `end` (thus `UncompressedOffsetToSpanID`);
	// 2. know where the uncompressed data `[start:end]` is located in the compressed stream
	//      (thus `StartCompressedOffset` and `EndCompressedOffset`);
	// 3. to speed up the uncompressed data extraction, we paralellize and extract
	//		the uncompressed data per *span*;
	//      (thus `StartUncompressedOffset` and `EndUncompressedOffset`).
	//
	// This may change in the future if we figure out better abstraction (e.g. when
	// implementing the interface for a new compression like zstd).

	// UncompressedOffsetToSpanID returns the ID of the span containing given `offset`.
	UncompressedOffsetToSpanID(offset Offset) SpanID
	// StartCompressedOffset returns the offset (in compressed stream)
	// of the 1st byte belonging to `spanID`.
	StartCompressedOffset(spanID SpanID) Offset
	// EndCompressedOffset returns the offset (in compressed stream)
	// of the last byte belonging to `spanID`. If it's the last span, `fileSize` is returned.
	EndCompressedOffset(spanID SpanID, fileSize Offset) Offset
	// StartUncompressedOffset returns the offset (in uncompressed stream)
	// of the 1st byte belonging to `spanID`.
	StartUncompressedOffset(spanID SpanID) Offset
	// EndUncompressedOffset returns the offset (in uncompressed stream)
	// of the last byte belonging to `spanID`. If it's the last span, `fileSize` is returned.
	EndUncompressedOffset(spanID SpanID, fileSize Offset) Offset
	// VerifyHeader checks if the given zinfo has a proper header
	VerifyHeader(r io.Reader) error
}

// NewZinfo deseralizes given zinfo bytes into a zinfo struct.
// This is often used when you have a serialized zinfo bytes and want to get the zinfo struct.
func NewZinfo(compressionAlgo string, zinfoBytes []byte) (Zinfo, error) {
	switch compressionAlgo {
	case Gzip:
		return newGzipZinfo(zinfoBytes)
	case Zstd:
		return nil, fmt.Errorf("not implemented: %s", Zstd)
	case Uncompressed, Unknown:
		return newTarZinfo(zinfoBytes)
	default:
		return nil, fmt.Errorf("unexpected compression algorithm: %s", compressionAlgo)
	}
}

// NewZinfoFromFile creates a zinfo struct given a compressed file and a span size.
// This is often used when you have a compressed file (e.g. gzip) and want to create
// a new zinfo for it.
func NewZinfoFromFile(compressionAlgo string, filename string, spanSize int64) (Zinfo, error) {
	switch compressionAlgo {
	case Gzip:
		return newGzipZinfoFromFile(filename, spanSize)
	case Zstd:
		return nil, fmt.Errorf("not implemented: %s", Zstd)
	case Uncompressed:
		return newTarZinfoFromFile(filename, spanSize)
	default:
		return nil, fmt.Errorf("unexpected compression algorithm: %s", compressionAlgo)
	}
}
