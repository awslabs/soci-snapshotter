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

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/opencontainers/go-digest"
	"golang.org/x/sync/errgroup"
)

// FileSize will hold any file size and offset values
type FileSize int64

// SpanID will hold any span related values (SpanID, MaxSpanID, SpanStart, SpanEnd, etc)
type SpanID int32

type FileMetadata struct {
	Name               string
	Type               string
	UncompressedOffset FileSize
	UncompressedSize   FileSize
	SpanStart          SpanID
	SpanEnd            SpanID
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
	Version                 string
	BuildToolIdentifier     string
	CompressedArchiveSize   FileSize
	UncompressedArchiveSize FileSize
	TOC                     TOC
	CompressionInfo         CompressionInfo
}

type CompressionInfo struct {
	MaxSpanID   SpanID //The total number of spans in Ztoc - 1
	SpanDigests []digest.Digest
	Checkpoints []byte
}

type TOC struct {
	Metadata []FileMetadata
}

type FileExtractConfig struct {
	UncompressedSize      FileSize
	UncompressedOffset    FileSize
	SpanStart             SpanID
	SpanEnd               SpanID
	Checkpoints           []byte
	CompressedArchiveSize FileSize
	MaxSpanID             SpanID
}

type MetadataEntry struct {
	UncompressedSize   FileSize
	UncompressedOffset FileSize
	SpanStart          SpanID
	SpanEnd            SpanID
}

func ExtractFile(r *io.SectionReader, config *FileExtractConfig) ([]byte, error) {
	if config.UncompressedSize == 0 {
		return []byte{}, nil
	}

	numSpans := config.SpanEnd - config.SpanStart + 1

	gzipZinfo, err := NewGzipZinfo(config.Checkpoints)
	if err != nil {
		return nil, nil
	}
	defer gzipZinfo.Close()

	var bufSize FileSize
	starts := make([]FileSize, numSpans)
	ends := make([]FileSize, numSpans)

	var i SpanID
	for i = 0; i < numSpans; i++ {
		starts[i] = gzipZinfo.SpanIDToCompressedOffset(i + config.SpanStart)
		if i+config.SpanStart == config.MaxSpanID {
			ends[i] = config.CompressedArchiveSize - 1
		} else {
			ends[i] = gzipZinfo.SpanIDToCompressedOffset(i + 1 + config.SpanStart)
		}
		bufSize += (ends[i] - starts[i])
	}

	start := starts[0]

	// It the first span the file is present in has partially uncompressed data,
	// fetch the previous byte too.
	firstSpanHasBits := gzipZinfo.HasBits(config.SpanStart)
	if firstSpanHasBits {
		bufSize++
		start--
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
				rangeStart--
			}
			n, err := r.ReadAt(buf[rangeStart-start:rangeEnd-start], int64(rangeStart)) // need to convert rangeStart to int64 to use in ReadAt
			if err != nil {
				return err
			}

			bytesToFetch := rangeEnd - rangeStart
			if n != int(bytesToFetch) {
				return fmt.Errorf("unexpected data size. read = %d, expected = %d", n, bytesToFetch)
			}
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return nil, err
	}

	bytes, err := gzipZinfo.ExtractDataFromBuffer(buf, config.UncompressedSize, config.UncompressedOffset, config.SpanStart)
	if err != nil {
		return nil, err
	}

	return bytes, nil
}

func GetMetadataEntry(ztoc *Ztoc, text string) (*MetadataEntry, error) {
	for _, v := range ztoc.TOC.Metadata {
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

	gzipZinfo, err := NewGzipZinfo(ztoc.CompressionInfo.Checkpoints)
	if err != nil {
		return "", err
	}
	defer gzipZinfo.Close()

	bytes, err := gzipZinfo.ExtractData(gz, entry.UncompressedSize, entry.UncompressedOffset)
	if err != nil {
		return "", err
	}

	return string(bytes), nil
}
