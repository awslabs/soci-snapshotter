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
	"archive/tar"
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/opencontainers/go-digest"
	"golang.org/x/sync/errgroup"

	"github.com/awslabs/soci-snapshotter/ztoc/compression"
)

// Version defines the version of a Ztoc.
type Version string

// Ztoc versions available.
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
	TOC
	CompressionInfo

	Version                 Version
	BuildToolIdentifier     string
	CompressedArchiveSize   compression.Offset
	UncompressedArchiveSize compression.Offset
}

// CompressionInfo is the "zinfo" part of ztoc including the `Checkpoints` data
// and other metadata such as all span digests.
type CompressionInfo struct {
	MaxSpanID            compression.SpanID //The total number of spans in Ztoc - 1
	SpanDigests          []digest.Digest
	Checkpoints          []byte
	CompressionAlgorithm string
}

// TOC is the "ztoc" part of ztoc including metadata of all files in the compressed
// data (e.g., a gzip tar file).
type TOC struct {
	FileMetadata []FileMetadata
}

// FileMetadata contains metadata of a file in the compressed data.
type FileMetadata struct {
	Name               string
	Type               string
	UncompressedOffset compression.Offset
	UncompressedSize   compression.Offset
	TarHeaderOffset    compression.Offset
	Linkname           string // Target name of link (valid for TypeLink or TypeSymlink)
	Mode               int64  // Permission and mode bits
	UID                int    // User ID of owner
	GID                int    // Group ID of owner
	Uname              string // User name of owner
	Gname              string // Group name of owner

	ModTime  time.Time // Modification time
	Devmajor int64     // Major device number (valid for TypeChar or TypeBlock)
	Devminor int64     // Minor device number (valid for TypeChar or TypeBlock)

	PAXHeaders map[string]string
}

// FileMode gets file mode for the file metadata
func (src FileMetadata) FileMode() (m os.FileMode) {
	// FileMetadata.Mode is tar.Header.Mode so we can understand the these bits using `tar` pkg.
	m = (&tar.Header{Mode: src.Mode}).FileInfo().Mode() &
		(os.ModePerm | os.ModeSetuid | os.ModeSetgid | os.ModeSticky)
	switch src.Type {
	case "dir":
		m |= os.ModeDir
	case "symlink":
		m |= os.ModeSymlink
	case "char":
		m |= os.ModeDevice | os.ModeCharDevice
	case "block":
		m |= os.ModeDevice
	case "fifo":
		m |= os.ModeNamedPipe
	}
	return m
}

func (src FileMetadata) Equal(o FileMetadata) bool {
	if src.Name != o.Name ||
		src.Type != o.Type ||
		src.UncompressedOffset != o.UncompressedOffset ||
		src.UncompressedSize != o.UncompressedSize ||
		src.TarHeaderOffset != o.TarHeaderOffset ||
		src.Linkname != o.Linkname ||
		src.Mode != o.Mode ||
		src.UID != o.UID ||
		src.GID != o.GID ||
		src.Uname != o.Uname ||
		src.Gname != o.Gname ||
		src.ModTime != o.ModTime ||
		src.Devmajor != o.Devmajor ||
		src.Devminor != o.Devminor {
		return false
	}
	if len(src.PAXHeaders) != len(o.PAXHeaders) {
		return false
	}
	for k, v := range src.PAXHeaders {
		if o.PAXHeaders[k] != v {
			return false
		}
	}
	return true
}

func (src FileMetadata) Xattrs() map[string]string {
	return Xattrs(src.PAXHeaders)
}

// MetadataEntry is used to locate a file based on its metadata.
type MetadataEntry struct {
	UncompressedSize   compression.Offset
	UncompressedOffset compression.Offset
}

// GetMetadataEntry gets MetadataEntry given a filename.
func (toc TOC) GetMetadataEntry(filename string) (MetadataEntry, error) {
	for _, v := range toc.FileMetadata {
		if v.Name == filename {
			if v.Linkname != "" {
				return toc.GetMetadataEntry(v.Linkname)
			}
			return MetadataEntry{
				UncompressedSize:   v.UncompressedSize,
				UncompressedOffset: v.UncompressedOffset,
			}, nil
		}
	}
	return MetadataEntry{}, fmt.Errorf("file %s does not exist in metadata", filename)
}

// ExtractFile extracts a file from compressed data (as a reader) and returns the
// byte data.
func (zt Ztoc) ExtractFile(r *io.SectionReader, filename string) ([]byte, error) {
	entry, err := zt.GetMetadataEntry(filename)
	if err != nil {
		return nil, err
	}
	if entry.UncompressedSize == 0 {
		return []byte{}, nil
	}

	zinfo, err := zt.Zinfo()
	if err != nil {
		return nil, nil
	}
	defer zinfo.Close()
	zt.Checkpoints = nil

	spanStart := zinfo.UncompressedOffsetToSpanID(entry.UncompressedOffset)
	spanEnd := zinfo.UncompressedOffsetToSpanID(entry.UncompressedOffset + entry.UncompressedSize)
	numSpans := spanEnd - spanStart + 1

	checkpoints := make([]compression.Offset, numSpans+1)
	checkpoints[0] = zinfo.StartCompressedOffset(spanStart)

	var i compression.SpanID
	for i = 0; i < numSpans; i++ {
		checkpoints[i+1] = zinfo.EndCompressedOffset(spanStart+i, zt.CompressedArchiveSize)
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

	bytes, err := zinfo.ExtractDataFromBuffer(buf, entry.UncompressedSize, entry.UncompressedOffset, spanStart)
	if err != nil {
		return nil, err
	}

	return bytes, nil
}

// ExtractFromTarGz extracts data given a gzip tar file (`gz`) and its `ztoc`.
func (zt Ztoc) ExtractFromTarGz(gz string, filename string) (string, error) {
	entry, err := zt.GetMetadataEntry(filename)
	if err != nil {
		return "", err
	}

	if entry.UncompressedSize == 0 {
		return "", nil
	}

	zinfo, err := zt.Zinfo()
	if err != nil {
		return "", err
	}
	defer zinfo.Close()
	zt.Checkpoints = nil

	bytes, err := zinfo.ExtractDataFromFile(gz, entry.UncompressedSize, entry.UncompressedOffset)
	if err != nil {
		return "", err
	}

	return string(bytes), nil
}

// Zinfo deserilizes and returns a Zinfo based on the zinfo bytes and compression
// algorithm in the ztoc.
func (zt Ztoc) Zinfo() (compression.Zinfo, error) {
	return compression.NewZinfo(zt.CompressionAlgorithm, zt.Checkpoints)
}
