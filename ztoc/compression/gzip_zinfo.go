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
// #cgo LDFLAGS: -L${SRCDIR}/../out -l:libz.a
// #include "gzip_zinfo.h"
// #include <stdlib.h>
// #include <stdint.h>
import "C"

import (
	"compress/gzip"
	"fmt"
	"io"
	"unsafe"
)

// GzipZinfo is a go struct wrapper of the gzip zinfo's C implementation.
type GzipZinfo struct {
	cZinfo *C.struct_gzip_zinfo
}

// newGzipZinfo creates a new instance of `GzipZinfo` from cZinfo byte blob on zTOC.
func newGzipZinfo(zinfoBytes []byte) (*GzipZinfo, error) {
	if len(zinfoBytes) == 0 {
		return nil, fmt.Errorf("empty checkpoints")
	}
	cZinfo := C.blob_to_zinfo(unsafe.Pointer(&zinfoBytes[0]), C.off_t(len(zinfoBytes)))
	if cZinfo == nil {
		return nil, fmt.Errorf("cannot convert blob to gzip_zinfo")
	}
	return &GzipZinfo{
		cZinfo: cZinfo,
	}, nil
}

// newGzipZinfoFromFile creates a new instance of `GzipZinfo` given gzip file name and span size.
func newGzipZinfoFromFile(gzipFile string, spanSize int64) (*GzipZinfo, error) {
	cstr := C.CString(gzipFile)
	defer C.free(unsafe.Pointer(cstr))

	var cZinfo *C.struct_gzip_zinfo
	ret := C.generate_zinfo_from_file(cstr, C.off_t(spanSize), &cZinfo)
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

	ret := C.zinfo_to_blob(i.cZinfo, unsafe.Pointer(&bytes[0]))
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

// ExtractDataFromFile wraps `C.extract_data_from_file` and returns the decompressed bytes given the name of the .tar.gz file,
// offset and the size in uncompressed stream.
func (i *GzipZinfo) ExtractDataFromFile(fileName string, uncompressedSize, uncompressedOffset Offset) ([]byte, error) {
	cstr := C.CString(fileName)
	defer C.free(unsafe.Pointer(cstr))
	if uncompressedSize < 0 {
		return nil, fmt.Errorf("invalid uncompressed size: %d", uncompressedSize)
	}
	if uncompressedSize == 0 {
		return []byte{}, nil
	}
	bytes := make([]byte, uncompressedSize)
	ret := C.extract_data_from_file(cstr, i.cZinfo, C.off_t(uncompressedOffset), unsafe.Pointer(&bytes[0]), C.int(uncompressedSize))
	if ret <= 0 {
		return nil, fmt.Errorf("unable to extract data; return code = %v", ret)
	}

	return bytes, nil
}

// StartCompressedOffset returns the start offset of the span in the compressed stream.
func (i *GzipZinfo) StartCompressedOffset(spanID SpanID) Offset {
	start := i.getCompressedOffset(spanID)
	if i.hasBits(spanID) {
		start--
	}
	return start
}

// EndCompressedOffset returns the end offset of the span in the compressed stream. If
// it's the last span, returns the size of the compressed stream.
func (i *GzipZinfo) EndCompressedOffset(spanID SpanID, fileSize Offset) Offset {
	if spanID == i.MaxSpanID() {
		return fileSize
	}
	return i.getCompressedOffset(spanID + 1)
}

// StartUncompressedOffset returns the start offset of the span in the uncompressed stream.
func (i *GzipZinfo) StartUncompressedOffset(spanID SpanID) Offset {
	return i.getUncompressedOffset(spanID)
}

// EndUncompressedOffset returns the end offset of the span in the uncompressed stream. If
// it's the last span, returns the size of the uncompressed stream.
func (i *GzipZinfo) EndUncompressedOffset(spanID SpanID, fileSize Offset) Offset {
	if spanID == i.MaxSpanID() {
		return fileSize
	}
	return i.getUncompressedOffset(spanID + 1)
}

// VerifyHeader checks if the given zinfo has a proper header
func (i *GzipZinfo) VerifyHeader(r io.Reader) error {
	gz, err := gzip.NewReader(r)
	if gz != nil {
		gz.Close()
	}
	return err
}

// getCompressedOffset wraps `C.get_comp_off` and returns the offset for the span in the compressed stream.
func (i *GzipZinfo) getCompressedOffset(spanID SpanID) Offset {
	return Offset(C.get_comp_off(i.cZinfo, C.int(spanID)))
}

// hasBits wraps `C.has_bits` and returns true if any data is contained in the previous span.
func (i *GzipZinfo) hasBits(spanID SpanID) bool {
	return C.has_bits(i.cZinfo, C.int(spanID)) != 0
}

// getUncompressedOffset wraps `C.get_uncomp_off` and returns the offset for the span in the uncompressed stream.
func (i *GzipZinfo) getUncompressedOffset(spanID SpanID) Offset {
	return Offset(C.get_ucomp_off(i.cZinfo, C.int(spanID)))
}
