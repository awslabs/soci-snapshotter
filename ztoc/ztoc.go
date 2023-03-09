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

package ztoc

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/opencontainers/go-digest"
	"golang.org/x/sync/errgroup"

	"github.com/awslabs/soci-snapshotter/ztoc/compression"
)

type Version string

const (
	Version09 Version = "0.9"
)

// Ztoc is a table of contents for compressed data which consists 2 parts:
//
// (1). toc (`TOC`): a table of contents containing file metadata and its
// offset in the decompressed TAR archive.
// (2). zinfo (`CompressionInfo`): a collection of "checkpoints" of the
// state of the compression engine at various points in the layer.
type Ztoc struct {
	Version                 Version
	BuildToolIdentifier     string
	CompressedArchiveSize   compression.Offset
	UncompressedArchiveSize compression.Offset
	TOC                     TOC
	CompressionInfo         CompressionInfo
}

// CompressionInfo is the "zinfo" part of ztoc including the `Checkpoints` data
// and other metadata such as all span digests.
type CompressionInfo struct {
	MaxSpanID   compression.SpanID //The total number of spans in Ztoc - 1
	SpanDigests []digest.Digest
	Checkpoints []byte
}

// TOC is the "ztoc" part of ztoc including metadata of all files in the compressed
// data (e.g., a gzip tar file).
type TOC struct {
	Metadata []FileMetadata
}

// FileMetadata contains metadata of a file in the compressed data.
type FileMetadata struct {
	Name               string
	Type               string
	UncompressedOffset compression.Offset
	UncompressedSize   compression.Offset
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

// FileExtractConfig contains information used to extract a file from compressed data.
type FileExtractConfig struct {
	UncompressedSize      compression.Offset
	UncompressedOffset    compression.Offset
	Checkpoints           []byte
	CompressedArchiveSize compression.Offset
	MaxSpanID             compression.SpanID
}

// MetadataEntry is used to locate a file based on its metadata.
type MetadataEntry struct {
	UncompressedSize   compression.Offset
	UncompressedOffset compression.Offset
}

// ExtractFile extracts a file from compressed data (as a reader) and returns the
// byte data.
func ExtractFile(r *io.SectionReader, config *FileExtractConfig) ([]byte, error) {
	if config.UncompressedSize == 0 {
		return []byte{}, nil
	}

	gzipZinfo, err := compression.NewZinfo(compression.Gzip, config.Checkpoints)
	if err != nil {
		return nil, nil
	}
	defer gzipZinfo.Close()

	spanStart := gzipZinfo.UncompressedOffsetToSpanID(config.UncompressedOffset)
	spanEnd := gzipZinfo.UncompressedOffsetToSpanID(config.UncompressedOffset + config.UncompressedSize)
	numSpans := spanEnd - spanStart + 1

	checkpoints := make([]compression.Offset, numSpans+1)
	checkpoints[0] = gzipZinfo.StartCompressedOffset(spanStart)

	var i compression.SpanID
	for i = 0; i < numSpans; i++ {
		checkpoints[i+1] = gzipZinfo.EndCompressedOffset(spanStart+i, config.CompressedArchiveSize)
	}

	bufSize := checkpoints[len(checkpoints)-1] - checkpoints[0]
	buf := make([]byte, bufSize)
	eg, _ := errgroup.WithContext(context.Background())

	// Fetch all span data in parallel
	for i = 0; i < numSpans; i++ {
		i := i
		eg.Go(func() error {
			rangeStart := checkpoints[i]
			rangeEnd := checkpoints[i+1]
			n, err := r.ReadAt(buf[rangeStart-checkpoints[0]:rangeEnd-checkpoints[0]], int64(rangeStart)) // need to convert rangeStart to int64 to use in ReadAt
			if err != nil && err != io.EOF {
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

	bytes, err := gzipZinfo.ExtractDataFromBuffer(buf, config.UncompressedSize, config.UncompressedOffset, spanStart)
	if err != nil {
		return nil, err
	}

	return bytes, nil
}

// NewGzipZinfo is the go implementation of getting "checkpoints" from compressed data.
func NewGzipZinfo(b []byte) {
	panic("unimplemented")
}

// GetMetadataEntry gets MetadataEntry from `ztoc` given a filename.
func GetMetadataEntry(ztoc *Ztoc, filename string) (*MetadataEntry, error) {
	for _, v := range ztoc.TOC.Metadata {
		if v.Name == filename {
			if v.Linkname != "" {
				return GetMetadataEntry(ztoc, v.Linkname)
			}
			return &MetadataEntry{
				UncompressedSize:   v.UncompressedSize,
				UncompressedOffset: v.UncompressedOffset,
			}, nil
		}
	}
	return nil, fmt.Errorf("file %s does not exist in metadata", filename)
}

// ExtractFromTarGz extracts data given a gzip tar file (`gz`) and its `ztoc`.
func ExtractFromTarGz(gz string, ztoc *Ztoc, text string) (string, error) {
	entry, err := GetMetadataEntry(ztoc, text)

	if err != nil {
		return "", err
	}

	if entry.UncompressedSize == 0 {
		return "", nil
	}

	gzipZinfo, err := compression.NewZinfo(compression.Gzip, ztoc.CompressionInfo.Checkpoints)
	if err != nil {
		return "", err
	}
	defer gzipZinfo.Close()

	bytes, err := gzipZinfo.ExtractDataFromFile(gz, entry.UncompressedSize, entry.UncompressedOffset)
	if err != nil {
		return "", err
	}

	return string(bytes), nil
}
