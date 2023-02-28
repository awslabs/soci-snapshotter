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
)

// Extractor specifies the interface for extracting a chunk of uncompressed data
// from a compressed buffer.
// Different compression algorithms need to implement this interface. So there
// will be GzipExtractor, ZstdExtractor, etc.
type Extractor interface {
	// Extract extracts the uncompressed data from `compressedBuf` and returns
	// as a byte slice.
	ExtractDataFromBuffer(compressedBuf []byte, uncompressedSize, uncompressedOffset Offset, spanID SpanID) ([]byte, error)
	// Extract extracts the uncompressed data directly from a compressed file
	// (e.g. a gzip file) and returns as a byte slice.
	ExtractDataFromFile(fileName string, uncompressedSize, uncompressedOffset Offset) ([]byte, error)
	// Close releases any resources held by the interface implementation.
	Close()

	// Below funcs need to be part of the interface because of how we currently
	// extract data from a compressed data stream. Specifically, if we need to
	// extract a chunk of uncompressed data (e.g. `[start:end]`), we need to:
	// 1. get the span id of both `start` and `end` (thus `UncompressedOffsetToSpanID`);
	// 2. know where the uncompressed data `[start:end]` is located in the compressed stream
	//      (thus `SpanIDToStartCompressedOffset` and `SpanIDToEndCompressedOffset`);
	// 3. to speed up the uncompressed data extraction, we paralellize and extract
	//		the uncompressed data per *span*;
	//      (thus `SpanIDToStartUncompressedOffset` and `SpanIDToEndUncompressedOffset`).
	//
	// This may change in the future if we figure out better abstraction (e.g. when
	// implementing the interface for a new compression like zstd).

	// UncompressedOffsetToSpanID returns the ID of the span containing given `offset`.
	UncompressedOffsetToSpanID(offset Offset) SpanID
	// SpanIDToStartCompressedOffset returns the offset (in compressed stream)
	// of the 1st byte belonging to `spanID`.
	SpanIDToStartCompressedOffset(spanID SpanID) Offset
	// SpanIDToEndCompressedOffset returns the offset (in compressed stream)
	// of the last byte belonging to `spanID`. If it's the last span, `fileSize` is returned.
	SpanIDToEndCompressedOffset(spanID SpanID, fileSize Offset) Offset
	// SpanIDToStartUncompressedOffset returns the offset (in uncompressed stream)
	// of the 1st byte belonging to `spanID`.
	SpanIDToStartUncompressedOffset(spanID SpanID) Offset
	// SpanIDToEndUncompressedOffset returns the offset (in uncompressed stream)
	// of the last byte belonging to `spanID`. If it's the last span, `fileSize` is returned.
	SpanIDToEndUncompressedOffset(spanID SpanID, fileSize Offset) Offset
}

// NewExtractor returns an Extracotr implmenetaion for a specific compression algorithm.
func NewExtractor(compressionAlgo string, zinfoBytes []byte) (Extractor, error) {
	switch compressionAlgo {
	case CompressionGzip:
		return NewGzipZinfo(zinfoBytes)
	case CompressionZstd:
		return nil, fmt.Errorf("not implemented: %s", CompressionZstd)
	default:
		return nil, fmt.Errorf("unexpected compression algorithm: %s", compressionAlgo)
	}
}
