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

package soci

// #cgo CFLAGS: -I${SRCDIR}/../c/
// #cgo LDFLAGS: -L${SRCDIR}/../out -lindexer -lz
// #include "indexer.h"
// #include <stdlib.h>
// #include <stdint.h>
import "C"

import (
	"fmt"
	"unsafe"
)

type GzipIndexer struct {
	index *C.struct_gzip_index
}

// GenerateIndex wraps `C.generate_index` and should be used for index generation instead of relying on C code.
// Returns the byte slice containing the `gzip_index`.
func GenerateIndex(gzipFile string, spanSize int64) ([]byte, error) {
	cstr := C.CString(gzipFile)
	defer C.free(unsafe.Pointer(cstr))

	var index *C.struct_gzip_index
	ret := C.generate_index(cstr, C.off_t(spanSize), &index)
	if int(ret) < 0 {
		return nil, fmt.Errorf("could not generate gzip index. gzip error: %v", ret)
	}
	defer C.free(unsafe.Pointer(index))

	blobSize := C.get_blob_size(index)
	bytes := make([]byte, uint64(blobSize))
	if bytes == nil {
		return nil, fmt.Errorf("could not allocate byte array of size %d", blobSize)
	}

	ret = C.index_to_blob(index, unsafe.Pointer(&bytes[0]))
	if int(ret) <= 0 {
		return nil, fmt.Errorf("could not serialize gzip index to byte array; gzip error: %v", ret)
	}

	return bytes, nil
}

// Close calls `C.free` on the pointer to `C.struct_gzip_index`.
func (i *GzipIndexer) Close() {
	if i.index != nil {
		C.free(unsafe.Pointer(i.index))
	}
}

// GetMaxSpanID returns the max span ID.
func (i *GzipIndexer) GetMaxSpanID() SpanID {
	return SpanID(i.index.have - 1)
}

// GetSpanIDByUncompressedOffset returns the ID of the span containing the data pointed by uncompressed offset.
func (i *GzipIndexer) GetSpanIDByUncompressedOffset(offset FileSize) SpanID {
	return SpanID(C.pt_index_from_ucmp_offset(i.index, C.long(offset)))
}

// HasBits wraps `C.has_bits` and returns true if any data is contained in the previous span.
func (i *GzipIndexer) HasBits(spanID SpanID) bool {
	return C.has_bits(i.index, C.int(spanID)) != 0
}

// GetCompressedOffset wraps `C.get_comp_off` and returns the offset for the span in the compressed stream.
func (i *GzipIndexer) GetCompressedOffset(spanID SpanID) FileSize {
	return FileSize(C.get_comp_off(i.index, C.int(spanID)))
}

// GetUncompressedOffset wraps `C.get_uncomp_off` and returns the offset for the span in the uncompressed stream.
func (i *GzipIndexer) GetUncompressedOffset(spanID SpanID) FileSize {
	return FileSize(C.get_ucomp_off(i.index, C.int(spanID)))
}

// ExtractDataFromBuffer wraps the call to `C.extract_data_from_buffer`, which takes in the compressed bytes
// and returns the decompressed bytes.
func (i *GzipIndexer) ExtractDataFromBuffer(compressedBuf []byte, uncompressedSize, uncompressedOffset FileSize, spanID SpanID) ([]byte, error) {
	bytes := make([]byte, uncompressedSize)
	ret := C.extract_data_from_buffer(
		unsafe.Pointer(&compressedBuf[0]),
		C.off_t(len(compressedBuf)),
		i.index,
		C.off_t(uncompressedOffset),
		unsafe.Pointer(&bytes[0]),
		C.off_t(uncompressedSize),
		C.int(spanID),
	)
	if ret <= 0 {
		return bytes, fmt.Errorf("error extracting data; return code: %v", ret)
	}

	return bytes, nil
}

// ExtractData wraps `C.extract_data` and returns the decompressed bytes given the name of the .tar.gz file,
// offset and the size in uncompressed stream.
func (i *GzipIndexer) ExtractData(fileName string, uncompressedSize, uncompressedOffset FileSize) ([]byte, error) {
	cstr := C.CString(fileName)
	defer C.free(unsafe.Pointer(cstr))
	bytes := make([]byte, uncompressedSize)
	ret := C.extract_data(cstr, i.index, C.off_t(uncompressedOffset), unsafe.Pointer(&bytes[0]), C.int(uncompressedSize))
	if ret <= 0 {
		return nil, fmt.Errorf("unable to extract data; return code = %v", ret)
	}

	return bytes, nil
}

// GetSpanIndicesForFile wraps `C.span_indices_for_file` and returns IDs of starting and ending given start and end offsets.
func (i *GzipIndexer) GetSpanIndicesForFile(startOffset, endOffset FileSize) (SpanID, SpanID, error) {
	var indexStart SpanID
	var indexEnd SpanID
	ret := C.span_indices_for_file(i.index, C.off_t(startOffset), C.off_t(endOffset), unsafe.Pointer(&indexStart), unsafe.Pointer(&indexEnd))
	if int(ret) <= 0 {
		return 0, 0, fmt.Errorf("cannot get the span indices for file with start and end offset: %d, %d; gzip error: %v", startOffset, endOffset, ret)
	}
	return indexStart, indexEnd, nil
}

func NewGzipIndexer(indexData []byte) (*GzipIndexer, error) {
	index := C.blob_to_index(unsafe.Pointer(&indexData[0]))
	if index == nil {
		return nil, fmt.Errorf("cannot convert blob to gzip_index")
	}
	return &GzipIndexer{
		index: index,
	}, nil
}
