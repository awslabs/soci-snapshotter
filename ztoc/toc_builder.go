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
	"compress/gzip"
	"fmt"
	"io"
	"os"

	"github.com/awslabs/soci-snapshotter/util/ioutils"
	"github.com/awslabs/soci-snapshotter/ztoc/compression"
	"github.com/klauspost/compress/zstd"
)

// TarProvider creates a tar reader from a compressed file reader (e.g., a gzip file reader),
// which can be used by `TocBuilder` to create `TOC` from it.
type TarProvider func(file *os.File) (io.Reader, error)

// TarProviderGzip creates a tar reader from gzip reader.
func TarProviderGzip(compressedReader *os.File) (io.Reader, error) {
	return gzip.NewReader(compressedReader)
}

// TarProviderZstd creates a tar reader from zstd reader.
func TarProviderZstd(compressedReader *os.File) (io.Reader, error) {
	return zstd.NewReader(compressedReader)
}

// TarProviderTar return the tar file directly as the input to
// `tar.NewReader`.
func TarProviderTar(compressedReader *os.File) (io.Reader, error) {
	return compressedReader, nil
}

// TocBuilder builds the `TOC` part of a ztoc and works with different
// compression algorithms (e.g., gzip, zstd) with a registered `TarProvider`.
type TocBuilder struct {
	tarProviders map[string]TarProvider
}

// NewTocBuilder return a `TocBuilder` struct. Users need to call `RegisterTarProvider`
// to support a specific compression algorithm.
func NewTocBuilder() TocBuilder {
	return TocBuilder{
		tarProviders: make(map[string]TarProvider),
	}
}

// RegisterTarProvider adds a TarProvider for a compression algorithm.
func (tb TocBuilder) RegisterTarProvider(algorithm string, provider TarProvider) {
	if tb.tarProviders == nil {
		tb.tarProviders = make(map[string]TarProvider)
	}
	tb.tarProviders[algorithm] = provider
}

// CheckCompressionAlgorithm checks if a compression algorithm is supported.
func (tb TocBuilder) CheckCompressionAlgorithm(algorithm string) bool {
	_, ok := tb.tarProviders[algorithm]
	return ok
}

// TocFromFile creates a `TOC` given a layer blob filename and the compression
// algorithm used by the layer.
func (tb TocBuilder) TocFromFile(algorithm, filename string) (TOC, compression.Offset, error) {
	if !tb.CheckCompressionAlgorithm(algorithm) {
		return TOC{}, 0, fmt.Errorf("unsupported compression algorithm: %s", algorithm)
	}

	fm, uncompressedArchiveSize, err := tb.getFileMetadata(algorithm, filename)
	if err != nil {
		return TOC{}, 0, err
	}

	return TOC{FileMetadata: fm}, uncompressedArchiveSize, nil
}

// getFileMetadata creates `FileMetadata` for each file within the compressed file
// and calculate the uncompressed size of the passed file.
func (tb TocBuilder) getFileMetadata(algorithm, filename string) ([]FileMetadata, compression.Offset, error) {
	// read compress file and create compress tar reader.
	compressFile, err := os.Open(filename)
	if err != nil {
		return nil, 0, fmt.Errorf("could not open file for reading: %v", err)
	}
	defer compressFile.Close()

	compressTarReader, err := tb.tarProviders[algorithm](compressFile)
	if err != nil {
		return nil, 0, err
	}

	md, uncompressFileSize, err := metadataFromTarReader(compressTarReader)
	if err != nil {
		return nil, 0, err
	}
	return md, uncompressFileSize, nil
}

// metadataFromTarReader reads every file from tar reader `sr` and creates
// `FileMetadata` for each file.
func metadataFromTarReader(r io.Reader) ([]FileMetadata, compression.Offset, error) {
	pt := ioutils.NewPositionTrackerReader(r)
	tarRdr := tar.NewReader(pt)
	var md []FileMetadata
	// the first tar header occurs at offset 0
	var tarHeaderOffset compression.Offset
	for {
		hdr, err := tarRdr.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, 0, fmt.Errorf("error while reading tar header: %w", err)
		}

		fileType, err := getType(hdr)
		if err != nil {
			return nil, 0, err
		}

		metadataEntry := FileMetadata{
			Name:               hdr.Name,
			Type:               fileType,
			UncompressedOffset: compression.Offset(pt.CurrentPos()),
			UncompressedSize:   compression.Offset(hdr.Size),
			TarHeaderOffset:    tarHeaderOffset,
			Linkname:           hdr.Linkname,
			Mode:               hdr.Mode,
			UID:                hdr.Uid,
			GID:                hdr.Gid,
			Uname:              hdr.Uname,
			Gname:              hdr.Gname,
			ModTime:            hdr.ModTime,
			Devmajor:           hdr.Devmajor,
			Devminor:           hdr.Devminor,
			PAXHeaders:         hdr.PAXRecords,
		}
		md = append(md, metadataEntry)
		// The next file's tar header can be found immediately after the current file + padding
		tarHeaderOffset = AlignToTarBlock(metadataEntry.UncompressedOffset + metadataEntry.UncompressedSize)
	}
	return md, compression.Offset(pt.CurrentPos()), nil
}

func getType(header *tar.Header) (fileType string, e error) {
	switch header.Typeflag {
	case tar.TypeLink:
		fileType = "hardlink"
	case tar.TypeSymlink:
		fileType = "symlink"
	case tar.TypeDir:
		fileType = "dir"
	case tar.TypeReg:
		fileType = "reg"
	case tar.TypeChar:
		fileType = "char"
	case tar.TypeBlock:
		fileType = "block"
	case tar.TypeFifo:
		fileType = "fifo"
	default:
		return "", fmt.Errorf("unsupported input tar entry %q", header.Typeflag)
	}
	return
}
