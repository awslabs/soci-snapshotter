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

	"github.com/awslabs/soci-snapshotter/compression"
	"github.com/opencontainers/go-digest"
)

func BuildZtoc(gzipFile string, span int64, buildToolIdentifier string) (*Ztoc, error) {
	if gzipFile == "" {
		return nil, fmt.Errorf("need to provide gzip file")
	}

	index, err := compression.NewGzipZinfoFromFile(gzipFile, span)
	if err != nil {
		return nil, err
	}
	defer index.Close()

	fm, uncompressedArchiveSize, err := getGzipFileMetadata(gzipFile, index)
	if err != nil {
		return nil, err
	}

	fs, err := getFileSize(gzipFile)
	if err != nil {
		return nil, err
	}

	digests, err := getPerSpanDigests(gzipFile, int64(fs), index)
	if err != nil {
		return nil, err
	}

	checkpoints, err := index.Bytes()
	if err != nil {
		return nil, err
	}

	toc := TOC{
		Metadata: fm,
	}

	compressionInfo := CompressionInfo{
		MaxSpanID:   index.MaxSpanID(),
		SpanDigests: digests,
		Checkpoints: checkpoints,
	}

	return &Ztoc{
		Version:                 "0.9",
		TOC:                     toc,
		CompressedArchiveSize:   fs,
		UncompressedArchiveSize: uncompressedArchiveSize,
		BuildToolIdentifier:     buildToolIdentifier,
		CompressionInfo:         compressionInfo,
	}, nil
}

func getPerSpanDigests(gzipFile string, fileSize int64, index *compression.GzipZinfo) ([]digest.Digest, error) {
	file, err := os.Open(gzipFile)
	if err != nil {
		return nil, fmt.Errorf("could not open file for reading: %w", err)
	}
	defer file.Close()

	var digests []digest.Digest
	var i compression.SpanID
	maxSpanID := index.MaxSpanID()
	for i = 0; i <= maxSpanID; i++ {
		var (
			startOffset = index.SpanIDToCompressedOffset(i)
			endOffset   compression.Offset
		)

		if index.HasBits(i) {
			startOffset--
		}

		if i == maxSpanID {
			endOffset = compression.Offset(fileSize)
		} else {
			endOffset = index.SpanIDToCompressedOffset(i + 1)
		}

		section := io.NewSectionReader(file, int64(startOffset), int64(endOffset-startOffset))
		dgst, err := digest.FromReader(section)
		if err != nil {
			return nil, fmt.Errorf("unable to compute digest for section; start=%d, end=%d, file=%s, size=%d", startOffset, endOffset, gzipFile, fileSize)
		}
		digests = append(digests, dgst)
	}
	return digests, nil
}

func getGzipFileMetadata(gzipFile string, index *compression.GzipZinfo) ([]FileMetadata, compression.Offset, error) {
	file, err := os.Open(gzipFile)
	if err != nil {
		return nil, 0, fmt.Errorf("could not open file for reading: %v", err)
	}
	defer file.Close()

	gzipRdr, err := gzip.NewReader(file)
	if err != nil {
		return nil, 0, fmt.Errorf("could not create gzip reader: %v", err)
	}

	f, sr, uncompressedArchiveSize, err := getTarReader(gzipRdr)

	if err != nil {
		return nil, 0, err
	}
	defer os.Remove(f.Name())

	pt := &positionTrackerReader{r: sr}
	tarRdr := tar.NewReader(pt)
	var md []FileMetadata

	for {
		hdr, err := tarRdr.Next()
		if err != nil {
			if err == io.EOF {
				break
			} else {
				return nil, 0, fmt.Errorf("error while reading tar header: %w", err)
			}
		}

		fileType, err := getType(hdr)
		if err != nil {
			return nil, 0, err
		}

		metadataEntry := FileMetadata{
			Name:               hdr.Name,
			Type:               fileType,
			UncompressedOffset: pt.CurrentPos(),
			UncompressedSize:   compression.Offset(hdr.Size),
			Linkname:           hdr.Linkname,
			Mode:               hdr.Mode,
			UID:                hdr.Uid,
			GID:                hdr.Gid,
			Uname:              hdr.Uname,
			Gname:              hdr.Gname,
			ModTime:            hdr.ModTime,
			Devmajor:           hdr.Devmajor,
			Devminor:           hdr.Devminor,
			Xattrs:             hdr.PAXRecords,
		}
		md = append(md, metadataEntry)
	}
	return md, uncompressedArchiveSize, nil
}

func getFileSize(file string) (compression.Offset, error) {
	f, err := os.Open(file)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return 0, err
	}
	return compression.Offset(st.Size()), nil
}

func getTarReader(gzipReader io.Reader) (*os.File, *io.SectionReader, compression.Offset, error) {
	file, err := os.CreateTemp("/tmp", "tempfile-ztoc-builder")
	if err != nil {
		return nil, nil, 0, err
	}
	_, err = io.Copy(file, gzipReader)
	if err != nil {
		os.Remove(file.Name())
		return nil, nil, 0, err
	}

	tarRdr, uncompressedArchiveSize, err := tarSectionReaderFromFile(file)
	if err != nil {
		return nil, nil, 0, err
	}

	return file, tarRdr, uncompressedArchiveSize, nil
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

func tarSectionReaderFromFile(f *os.File) (*io.SectionReader, compression.Offset, error) {
	st, err := f.Stat()
	if err != nil {
		return nil, 0, err
	}

	return io.NewSectionReader(f, 0, st.Size()), compression.Offset(st.Size()), nil
}

type positionTrackerReader struct {
	r   io.ReaderAt
	pos compression.Offset
}

func (p *positionTrackerReader) Read(b []byte) (int, error) {
	n, err := p.r.ReadAt(b, int64(p.pos))
	if err == nil {
		p.pos += compression.Offset(n)
	}
	return n, err
}

func (p *positionTrackerReader) CurrentPos() compression.Offset {
	return p.pos
}
