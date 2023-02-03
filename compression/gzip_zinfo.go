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

// #cgo CFLAGS: -I${SRCDIR}/
// #cgo LDFLAGS: -L${SRCDIR}/../out -lz
// #include "zinfo.h"
// #include <stdlib.h>
// #include <stdint.h>
import "C"

import (
	"fmt"
	"unsafe"
)

type GzipZinfo struct {
	cZinfo *C.struct_gzip_zinfo
}

// NewGzipZinfo creates a new instance of `GzipZinfo` from cZinfo byte blob on zTOC.
func NewGzipZinfo(checkpoints []byte) (*GzipZinfo, error) {
	if len(checkpoints) == 0 {
		return nil, fmt.Errorf("empty checkpoints")
	}
	cZinfo := C.blob_to_zinfo(unsafe.Pointer(&checkpoints[0]), C.off_t(len(checkpoints)))
	if cZinfo == nil {
		return nil, fmt.Errorf("cannot convert blob to gzip_zinfo")
	}
	return &GzipZinfo{
		cZinfo: cZinfo,
	}, nil
}

// NewGzipZinfoFromFile creates a new instance of `GzipZinfo` given gzip file name and span size.
func NewGzipZinfoFromFile(gzipFile string, spanSize int64) (*GzipZinfo, error) {
	cstr := C.CString(gzipFile)
	defer C.free(unsafe.Pointer(cstr))

	var cZinfo *C.struct_gzip_zinfo
	ret := C.generate_index(cstr, C.off_t(spanSize), &cZinfo)
	if int(ret) < 0 {
		return nil, fmt.Errorf("could not generate gzip zinfo. gzip error: %v", ret)
	}

	return &GzipZinfo{
		cZinfo: cZinfo,
	}, nil
}

// Close calls `C.free` on the pointer to `C.struct_gzip_zinfo`.
func (i *GzipZinfo) Close() {
	if i.cZinfo != nil {
		C.free(unsafe.Pointer(i.cZinfo))
	}
}

// Bytes returns the byte slice containing the zinfo.
func (i *GzipZinfo) Bytes() ([]byte, error) {
	blobSize := C.get_blob_size(i.cZinfo)
	bytes := make([]byte, uint64(blobSize))
	if len(bytes) == 0 {
		return nil, fmt.Errorf("could not allocate byte array of size %d", blobSize)
	}

	ret := C.index_to_blob(i.cZinfo, unsafe.Pointer(&bytes[0]))
	if int(ret) <= 0 {
		return nil, fmt.Errorf("could not serialize gzip zinfo to byte array; gzip error: %v", ret)
	}
	return bytes, nil
}

// MaxSpanID returns the max span ID.
func (i *GzipZinfo) MaxSpanID() SpanID {
	return SpanID(C.get_max_span_id(i.cZinfo))
}

// SpanSize returns the span size of the constructed ztoc.
func (i *GzipZinfo) SpanSize() Offset {
	return Offset(i.cZinfo.span_size)
}

// UncompressedOffsetToSpanID returns the ID of the span containing the data pointed by uncompressed offset.
func (i *GzipZinfo) UncompressedOffsetToSpanID(offset Offset) SpanID {
	return SpanID(C.pt_index_from_ucmp_offset(i.cZinfo, C.long(offset)))
}

// HasBits wraps `C.has_bits` and returns true if any data is contained in the previous span.
func (i *GzipZinfo) HasBits(spanID SpanID) bool {
	return C.has_bits(i.cZinfo, C.int(spanID)) != 0
}

// SpanIDToCompressedOffset wraps `C.get_comp_off` and returns the offset for the span in the compressed stream.
func (i *GzipZinfo) SpanIDToCompressedOffset(spanID SpanID) Offset {
	return Offset(C.get_comp_off(i.cZinfo, C.int(spanID)))
}

// SpanIDToUncompressedOffset wraps `C.get_uncomp_off` and returns the offset for the span in the uncompressed stream.
func (i *GzipZinfo) SpanIDToUncompressedOffset(spanID SpanID) Offset {
	return Offset(C.get_ucomp_off(i.cZinfo, C.int(spanID)))
}

// ExtractDataFromBuffer wraps the call to `C.extract_data_from_buffer`, which takes in the compressed bytes
// and returns the decompressed bytes.
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
	bytes := make([]byte, uncompressedSize)
	ret := C.extract_data_from_buffer(
		unsafe.Pointer(&compressedBuf[0]),
		C.off_t(len(compressedBuf)),
		i.cZinfo,
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
func (i *GzipZinfo) ExtractData(fileName string, uncompressedSize, uncompressedOffset Offset) ([]byte, error) {
	cstr := C.CString(fileName)
	defer C.free(unsafe.Pointer(cstr))
	if uncompressedSize < 0 {
		return nil, fmt.Errorf("invalid uncompressed size: %d", uncompressedSize)
	}
	if uncompressedSize == 0 {
		return []byte{}, nil
	}
	bytes := make([]byte, uncompressedSize)
	ret := C.extract_data(cstr, i.cZinfo, C.off_t(uncompressedOffset), unsafe.Pointer(&bytes[0]), C.int(uncompressedSize))
	if ret <= 0 {
		return nil, fmt.Errorf("unable to extract data; return code = %v", ret)
	}

	return bytes, nil
}
