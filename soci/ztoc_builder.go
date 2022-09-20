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
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/gob"
	"fmt"
	"io"
	"os"

	"github.com/klauspost/compress/zstd"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

func BuildZtoc(gzipFile string, span int64, cfg *buildConfig) (*Ztoc, error) {
	if gzipFile == "" {
		return nil, fmt.Errorf("need to provide gzip file")
	}

	indexData, err := GenerateIndex(gzipFile, span)
	if err != nil {
		return nil, err
	}

	indexer, err := NewGzipIndexer(indexData)
	if err != nil {
		return nil, err
	}
	defer indexer.Close()

	fm, uncompressedFileSize, err := getGzipFileMetadata(gzipFile, indexer)
	if err != nil {
		return nil, err
	}

	fs, err := getFileSize(gzipFile)
	if err != nil {
		return nil, err
	}

	digests, err := getPerSpanDigests(gzipFile, int64(fs), indexer)
	if err != nil {
		return nil, err
	}

	ztocInfo := ztocInfo{
		SpanDigests: digests,
	}

	return &Ztoc{
		Version:              "0.1",
		IndexByteData:        indexData,
		Metadata:             fm,
		CompressedFileSize:   fs,
		UncompressedFileSize: uncompressedFileSize,
		MaxSpanID:            indexer.GetMaxSpanID(),
		BuildToolIdentifier:  cfg.buildToolIdentifier,
		ZtocInfo:             ztocInfo,
	}, nil
}

func NewZtocReader(ztoc *Ztoc) (io.Reader, ocispec.Descriptor, error) {
	serializedBuf := new(bytes.Buffer)
	enc := gob.NewEncoder(serializedBuf)
	err := enc.Encode(*ztoc)
	if err != nil {
		return nil, ocispec.Descriptor{}, fmt.Errorf("cannot serialize ztoc: %w", err)
	}

	compressedBuf := new(bytes.Buffer)
	zs, err := zstd.NewWriter(compressedBuf)
	if err != nil {
		return nil, ocispec.Descriptor{}, fmt.Errorf("cannot create zstd writer: %w", err)
	}

	if _, err := zs.Write(serializedBuf.Bytes()); err != nil {
		return nil, ocispec.Descriptor{}, fmt.Errorf("cannot compress ztoc: %w", err)
	}

	if err := zs.Close(); err != nil {
		return nil, ocispec.Descriptor{}, err
	}

	compressedBytes := compressedBuf.Bytes()
	dgst := digest.FromBytes(compressedBytes)
	size := len(compressedBytes)
	return compressedBuf, ocispec.Descriptor{
		Digest: dgst,
		Size:   int64(size),
	}, nil
}

func getPerSpanDigests(gzipFile string, fileSize int64, indexer *GzipIndexer) ([]digest.Digest, error) {
	file, err := os.Open(gzipFile)
	if err != nil {
		return nil, fmt.Errorf("could not open file for reading: %w", err)
	}
	defer file.Close()

	var digests []digest.Digest
	var i SpanID
	maxSpanID := indexer.GetMaxSpanID()
	for i = 0; i <= maxSpanID; i++ {
		var (
			startOffset = indexer.GetCompressedOffset(i)
			endOffset   FileSize
		)

		if indexer.HasBits(i) {
			startOffset--
		}

		if i == maxSpanID {
			endOffset = FileSize(fileSize)
		} else {
			endOffset = indexer.GetCompressedOffset(i + 1)
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

func getGzipFileMetadata(gzipFile string, indexer *GzipIndexer) ([]FileMetadata, FileSize, error) {
	file, err := os.Open(gzipFile)
	if err != nil {
		return nil, 0, fmt.Errorf("could not open file for reading: %v", err)
	}
	defer file.Close()

	gzipRdr, err := gzip.NewReader(file)
	if err != nil {
		return nil, 0, fmt.Errorf("could not create gzip reader: %v", err)
	}

	f, sr, uncompressedFileSize, err := getTarReader(gzipRdr)

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

		start := pt.CurrentPos()
		end := pt.CurrentPos() + FileSize(hdr.Size)

		indexStart, indexEnd, err := indexer.GetSpanIndicesForFile(start, end)
		if err != nil {
			return nil, 0, err
		}

		hasBits := indexer.HasBits(indexStart)

		fileType, err := getType(hdr)
		if err != nil {
			return nil, 0, err
		}

		metadataEntry := FileMetadata{
			Name:               hdr.Name,
			Type:               fileType,
			UncompressedOffset: pt.CurrentPos(),
			UncompressedSize:   FileSize(hdr.Size),
			SpanStart:          indexStart,
			SpanEnd:            indexEnd,
			FirstSpanHasBits:   hasBits,
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
	return md, uncompressedFileSize, nil
}

func getFileSize(file string) (FileSize, error) {
	f, err := os.Open(file)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return 0, err
	}
	return FileSize(st.Size()), nil
}

func getTarReader(gzipReader io.Reader) (*os.File, *io.SectionReader, FileSize, error) {
	file, err := os.CreateTemp("/tmp", "tempfile-ztoc-builder")
	if err != nil {
		return nil, nil, 0, err
	}
	_, err = io.Copy(file, gzipReader)
	if err != nil {
		os.Remove(file.Name())
		return nil, nil, 0, err
	}

	tarRdr, uncompressedFileSize, err := tarSectionReaderFromFile(file)
	if err != nil {
		return nil, nil, 0, err
	}

	return file, tarRdr, uncompressedFileSize, nil
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

func tarSectionReaderFromFile(f *os.File) (*io.SectionReader, FileSize, error) {
	st, err := f.Stat()
	if err != nil {
		return nil, 0, err
	}

	return io.NewSectionReader(f, 0, st.Size()), FileSize(st.Size()), nil
}

type positionTrackerReader struct {
	r   io.ReaderAt
	pos FileSize
}

func (p *positionTrackerReader) Read(b []byte) (int, error) {
	n, err := p.r.ReadAt(b, int64(p.pos))
	if err == nil {
		p.pos += FileSize(n)
	}
	return n, err
}

func (p *positionTrackerReader) CurrentPos() FileSize {
	return p.pos
}
