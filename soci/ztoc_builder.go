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
	"fmt"
	"io"
	"os"
	"sort"

	ztoc_flatbuffers "github.com/awslabs/soci-snapshotter/soci/fbs/ztoc"
	flatbuffers "github.com/google/flatbuffers/go"

	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

func BuildZtoc(gzipFile string, span int64, buildToolIdentifier string) (*Ztoc, error) {
	if gzipFile == "" {
		return nil, fmt.Errorf("need to provide gzip file")
	}

	index, err := NewGzipZinfoFromFile(gzipFile, span)
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

func NewZtocReader(ztoc *Ztoc) (io.Reader, ocispec.Descriptor, error) {
	flatbuf := ztocToFlatbuffer(ztoc)
	buf := bytes.NewReader(flatbuf)
	dgst := digest.FromBytes(flatbuf)
	size := len(flatbuf)
	return buf, ocispec.Descriptor{
		Digest: dgst,
		Size:   int64(size),
	}, nil
}

func ztocToFlatbuffer(ztoc *Ztoc) []byte {
	builder := flatbuffers.NewBuilder(0)
	version := builder.CreateString(ztoc.Version)
	buildToolIdentifier := builder.CreateString(ztoc.BuildToolIdentifier)

	metadataOffsetList := make([]flatbuffers.UOffsetT, len(ztoc.TOC.Metadata))
	for i := len(ztoc.TOC.Metadata) - 1; i >= 0; i-- {
		me := ztoc.TOC.Metadata[i]
		// preparing the individual file medatada element
		metadataOffsetList[i] = prepareMetadataOffset(builder, me)
	}
	ztoc_flatbuffers.TOCStartMetadataVector(builder, len(ztoc.TOC.Metadata))
	for i := len(metadataOffsetList) - 1; i >= 0; i-- {
		builder.PrependUOffsetT(metadataOffsetList[i])
	}
	metadata := builder.EndVector(len(ztoc.TOC.Metadata))

	ztoc_flatbuffers.TOCStart(builder)
	ztoc_flatbuffers.TOCAddMetadata(builder, metadata)
	toc := ztoc_flatbuffers.TOCEnd(builder)

	// CompressionInfo
	checkpointsVector := builder.CreateByteVector(ztoc.CompressionInfo.Checkpoints)
	spanDigestsOffsets := make([]flatbuffers.UOffsetT, 0, len(ztoc.CompressionInfo.SpanDigests))
	for _, spanDigest := range ztoc.CompressionInfo.SpanDigests {
		off := builder.CreateString(spanDigest.String())
		spanDigestsOffsets = append(spanDigestsOffsets, off)
	}
	ztoc_flatbuffers.CompressionInfoStartSpanDigestsVector(builder, len(spanDigestsOffsets))
	for i := len(spanDigestsOffsets) - 1; i >= 0; i-- {
		builder.PrependUOffsetT(spanDigestsOffsets[i])
	}
	spanDigests := builder.EndVector(len(spanDigestsOffsets))
	ztoc_flatbuffers.CompressionInfoStart(builder)
	ztoc_flatbuffers.CompressionInfoAddMaxSpanId(builder, int32(ztoc.CompressionInfo.MaxSpanID))
	ztoc_flatbuffers.CompressionInfoAddSpanDigests(builder, spanDigests)
	ztoc_flatbuffers.CompressionInfoAddCheckpoints(builder, checkpointsVector)
	ztocInfo := ztoc_flatbuffers.CompressionInfoEnd(builder)

	ztoc_flatbuffers.ZtocStart(builder)
	ztoc_flatbuffers.ZtocAddVersion(builder, version)
	ztoc_flatbuffers.ZtocAddBuildToolIdentifier(builder, buildToolIdentifier)
	ztoc_flatbuffers.ZtocAddToc(builder, toc)
	ztoc_flatbuffers.ZtocAddCompressedArchiveSize(builder, int64(ztoc.CompressedArchiveSize))
	ztoc_flatbuffers.ZtocAddUncompressedArchiveSize(builder, int64(ztoc.UncompressedArchiveSize))
	ztoc_flatbuffers.ZtocAddCompressionInfo(builder, ztocInfo)
	ztocFlatbuf := ztoc_flatbuffers.ZtocEnd(builder)
	builder.Finish(ztocFlatbuf)
	return builder.FinishedBytes()
}

func prepareMetadataOffset(builder *flatbuffers.Builder, me FileMetadata) flatbuffers.UOffsetT {
	name := builder.CreateString(me.Name)
	t := builder.CreateString(me.Type)
	linkName := builder.CreateString(me.Linkname)
	uname := builder.CreateString(me.Uname)
	gname := builder.CreateString(me.Gname)
	modTimeBinary, _ := me.ModTime.MarshalText()
	modTime := builder.CreateString(string(modTimeBinary))

	xattrs := prepareXattrsOffset(me, builder)

	ztoc_flatbuffers.FileMetadataStart(builder)
	ztoc_flatbuffers.FileMetadataAddName(builder, name)
	ztoc_flatbuffers.FileMetadataAddType(builder, t)
	ztoc_flatbuffers.FileMetadataAddUncompressedOffset(builder, int64(me.UncompressedOffset))
	ztoc_flatbuffers.FileMetadataAddUncompressedSize(builder, int64(me.UncompressedSize))
	ztoc_flatbuffers.FileMetadataAddSpanStart(builder, int32(me.SpanStart))
	ztoc_flatbuffers.FileMetadataAddSpanEnd(builder, int32(me.SpanEnd))
	ztoc_flatbuffers.FileMetadataAddLinkname(builder, linkName)
	ztoc_flatbuffers.FileMetadataAddMode(builder, me.Mode)
	ztoc_flatbuffers.FileMetadataAddUid(builder, uint32(me.UID))
	ztoc_flatbuffers.FileMetadataAddGid(builder, uint32(me.GID))
	ztoc_flatbuffers.FileMetadataAddUname(builder, uname)
	ztoc_flatbuffers.FileMetadataAddGname(builder, gname)
	ztoc_flatbuffers.FileMetadataAddModTime(builder, modTime)
	ztoc_flatbuffers.FileMetadataAddDevmajor(builder, me.Devmajor)
	ztoc_flatbuffers.FileMetadataAddDevminor(builder, me.Devminor)

	ztoc_flatbuffers.FileMetadataAddXattrs(builder, xattrs)

	off := ztoc_flatbuffers.FileMetadataEnd(builder)
	return off
}

func prepareXattrsOffset(me FileMetadata, builder *flatbuffers.Builder) flatbuffers.UOffsetT {
	keys := make([]string, 0, len(me.Xattrs))
	for k := range me.Xattrs {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	xattrOffsetList := make([]flatbuffers.UOffsetT, 0, len(me.Xattrs))
	for _, key := range keys {
		keyOffset := builder.CreateString(key)
		valueOffset := builder.CreateString(me.Xattrs[key])
		ztoc_flatbuffers.XattrStart(builder)
		ztoc_flatbuffers.XattrAddKey(builder, keyOffset)
		ztoc_flatbuffers.XattrAddValue(builder, valueOffset)
		xattrOffset := ztoc_flatbuffers.XattrEnd(builder)
		xattrOffsetList = append(xattrOffsetList, xattrOffset)
	}
	ztoc_flatbuffers.FileMetadataStartXattrsVector(builder, len(xattrOffsetList))
	for j := len(xattrOffsetList) - 1; j >= 0; j-- {
		builder.PrependUOffsetT(xattrOffsetList[j])
	}
	xattrs := builder.EndVector(len(me.Xattrs))
	return xattrs
}

func getPerSpanDigests(gzipFile string, fileSize int64, index *GzipZinfo) ([]digest.Digest, error) {
	file, err := os.Open(gzipFile)
	if err != nil {
		return nil, fmt.Errorf("could not open file for reading: %w", err)
	}
	defer file.Close()

	var digests []digest.Digest
	var i SpanID
	maxSpanID := index.MaxSpanID()
	for i = 0; i <= maxSpanID; i++ {
		var (
			startOffset = index.SpanIDToCompressedOffset(i)
			endOffset   FileSize
		)

		if index.HasBits(i) {
			startOffset--
		}

		if i == maxSpanID {
			endOffset = FileSize(fileSize)
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

func getGzipFileMetadata(gzipFile string, index *GzipZinfo) ([]FileMetadata, FileSize, error) {
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

		start := pt.CurrentPos()
		end := pt.CurrentPos() + FileSize(hdr.Size)
		indexStart := index.UncompressedOffsetToSpanID(start)
		indexEnd := index.UncompressedOffsetToSpanID(end)

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
