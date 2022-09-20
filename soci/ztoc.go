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

// #include "indexer.h"
// #include <stdlib.h>
// #include <stdio.h>
import "C"

import (
	"context"
	"fmt"
	"io"
	"time"
	"unsafe"

	"github.com/opencontainers/go-digest"
	"golang.org/x/sync/errgroup"
)

// FileSize will hold any file size and offset values
type FileSize int64

// SpanId will hold any span related values (SpanId, MaxSpanId, SpanStart, SpanEnd, etc)
type SpanId int32

type FileMetadata struct {
	Name               string
	Type               string
	UncompressedOffset FileSize
	UncompressedSize   FileSize
	SpanStart          SpanId
	SpanEnd            SpanId
	Linkname           string // Target name of link (valid for TypeLink or TypeSymlink)
	Mode               int64  // Permission and mode bits
	UID                int    // User ID of owner
	GID                int    // Group ID of owner
	Uname              string // User name of owner
	Gname              string // Group name of owner

	ModTime  time.Time // Modification time
	Devmajor int64     // Major device number (valid for TypeChar or TypeBlock)
	Devminor int64     // Minor device number (valid for TypeChar or TypeBlock)

	Xattrs map[string]string
}

type Ztoc struct {
	Version             string
	BuildToolIdentifier string

	Metadata []FileMetadata

	CompressedFileSize   FileSize
	UncompressedFileSize FileSize
	MaxSpanId            SpanId //The total number of spans in Ztoc - 1
	ZtocInfo             ztocInfo
	IndexByteData        []byte
}

type ztocInfo struct {
	SpanDigests []digest.Digest
}

type FileExtractConfig struct {
	UncompressedSize   FileSize
	UncompressedOffset FileSize
	SpanStart          SpanId
	SpanEnd            SpanId
	IndexByteData      []byte
	CompressedFileSize FileSize
	MaxSpanId          SpanId
}

type MetadataEntry struct {
	UncompressedSize   FileSize
	UncompressedOffset FileSize
	SpanStart          SpanId
	SpanEnd            SpanId
}

func ExtractFile(r *io.SectionReader, config *FileExtractConfig) ([]byte, error) {
	bytes := make([]byte, config.UncompressedSize)
	if config.UncompressedSize == 0 {
		return bytes, nil
	}

	numSpans := config.SpanEnd - config.SpanStart + 1

	index := C.blob_to_index(unsafe.Pointer(&config.IndexByteData[0]))

	if index == nil {
		return bytes, fmt.Errorf("cannot convert blob to gzip_index")
	}
	defer C.free_index(index)
	var bufSize FileSize
	starts := make([]FileSize, numSpans)
	ends := make([]FileSize, numSpans)

	var i SpanId
	for i = 0; i < numSpans; i++ {
		starts[i] = FileSize(C.get_comp_off(index, C.int(i+config.SpanStart)))
		if i+config.SpanStart == config.MaxSpanId {
			ends[i] = config.CompressedFileSize - 1
		} else {
			ends[i] = FileSize(C.get_comp_off(index, C.int(i+1+config.SpanStart)))
		}
		bufSize += (ends[i] - starts[i] + 1)
	}

	start := starts[0]

	// It the first span the file is present in has partially uncompressed data,
	// fetch the previous byte too.
	firstSpanHasBits := C.has_bits(index, C.int(config.SpanStart)) != 0
	if firstSpanHasBits {
		bufSize += 1
		start -= 1
	}

	buf := make([]byte, bufSize)
	eg, _ := errgroup.WithContext(context.Background())

	// Fetch all span data in parallel
	for i = 0; i < numSpans; i++ {
		j := i
		eg.Go(func() error {
			rangeStart := starts[j]
			rangeEnd := ends[j]
			if j == 0 && firstSpanHasBits {
				rangeStart -= 1
			}
			n, err := r.ReadAt(buf[rangeStart-start:rangeEnd-start+1], int64(rangeStart)) // need to convert rangeStart to int64 to use in ReadAt
			if err != nil {
				return err
			}

			bytesToFetch := rangeEnd - rangeStart + 1
			if n != int(bytesToFetch) {
				return fmt.Errorf("unexpected data size. read = %d, expected = %d", n, bytesToFetch)
			}
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return bytes, err
	}

	ret := C.extract_data_from_buffer(unsafe.Pointer(&buf[0]), C.off_t(len(buf)), index, C.off_t(config.UncompressedOffset), unsafe.Pointer(&bytes[0]), C.off_t(config.UncompressedSize), C.int(config.SpanStart))
	if ret <= 0 {
		return bytes, fmt.Errorf("error extracting data; return code: %v", ret)
	}

	return bytes, nil
}

func GetMetadataEntry(ztoc *Ztoc, text string) (*MetadataEntry, error) {
	for _, v := range ztoc.Metadata {
		if v.Name == text {
			if v.Linkname != "" {
				return GetMetadataEntry(ztoc, v.Linkname)
			}
			return &MetadataEntry{
				UncompressedSize:   v.UncompressedSize,
				UncompressedOffset: v.UncompressedOffset,
				SpanStart:          v.SpanStart,
				SpanEnd:            v.SpanEnd,
			}, nil
		}
	}
	return nil, fmt.Errorf("text %s does not exist in metadata", text)
}

func ExtractFromTarGz(gz string, ztoc *Ztoc, text string) (string, error) {
	entry, err := GetMetadataEntry(ztoc, text)

	if err != nil {
		return "", err
	}

	if entry.UncompressedSize == 0 {
		return "", nil
	}

	cstr := C.CString(gz)
	defer C.free(unsafe.Pointer(cstr))

	index := C.blob_to_index(unsafe.Pointer(&ztoc.IndexByteData[0]))

	if index == nil {
		return "", fmt.Errorf("cannot convert blob to gzip_index")
	}

	defer C.free_index(index)

	bytes := make([]byte, entry.UncompressedSize)
	ret := C.extract_data(cstr, index, C.off_t(entry.UncompressedOffset), unsafe.Pointer(&bytes[0]), C.int(entry.UncompressedSize))

	if ret <= 0 {
		return "", fmt.Errorf("unable to extract data; return code = %v", ret)
	}

	return string(bytes), nil
}
