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
	"fmt"
	"io"
	"os"
	"time"

	ztoc_flatbuffers "github.com/awslabs/soci-snapshotter/soci/fbs/ztoc"
	"github.com/opencontainers/go-digest"
)

func GetZtocFromFile(filename string) (*Ztoc, error) {
	reader, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return GetZtoc(reader)
}

// GetZtoc reads and returns the Ztoc
func GetZtoc(reader io.Reader) (*Ztoc, error) {
	flatbuf, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("cannot read ztoc: %v", err)
	}
	ztoc := flatbufToZtoc(flatbuf)
	return ztoc, nil
}

func flatbufToZtoc(flatbuffer []byte) *Ztoc {
	ztoc := new(Ztoc)
	ztocFlatbuf := ztoc_flatbuffers.GetRootAsZtoc(flatbuffer, 0)
	ztoc.Version = string(ztocFlatbuf.Version())
	ztoc.BuildToolIdentifier = string(ztocFlatbuf.BuildToolIdentifier())
	ztoc.CompressedArchiveSize = FileSize(ztocFlatbuf.CompressedArchiveSize())
	ztoc.UncompressedArchiveSize = FileSize(ztocFlatbuf.UncompressedArchiveSize())

	toc := new(ztoc_flatbuffers.TOC)
	ztocFlatbuf.Toc(toc)

	metadata := make([]FileMetadata, toc.MetadataLength())
	ztoc.TOC = TOC{
		Metadata: metadata,
	}

	for i := 0; i < toc.MetadataLength(); i++ {
		metadataEntry := new(ztoc_flatbuffers.FileMetadata)
		toc.Metadata(metadataEntry, i)
		var me FileMetadata
		me.Name = string(metadataEntry.Name())
		me.Type = string(metadataEntry.Type())
		me.UncompressedOffset = FileSize(metadataEntry.UncompressedOffset())
		me.UncompressedSize = FileSize(metadataEntry.UncompressedSize())
		me.Linkname = string(metadataEntry.Linkname())
		me.Mode = metadataEntry.Mode()
		me.UID = int(metadataEntry.Uid())
		me.GID = int(metadataEntry.Gid())
		me.Uname = string(metadataEntry.Uname())
		me.Gname = string(metadataEntry.Gname())
		modTime := new(time.Time)
		modTime.UnmarshalText(metadataEntry.ModTime())
		me.ModTime = *modTime
		me.Devmajor = metadataEntry.Devmajor()
		me.Devminor = metadataEntry.Devminor()
		me.Xattrs = make(map[string]string)
		for j := 0; j < metadataEntry.XattrsLength(); j++ {
			xattrEntry := new(ztoc_flatbuffers.Xattr)
			metadataEntry.Xattrs(xattrEntry, j)
			key := string(xattrEntry.Key())
			value := string(xattrEntry.Value())
			me.Xattrs[key] = value
		}

		ztoc.TOC.Metadata[i] = me
	}

	compressionInfo := new(ztoc_flatbuffers.CompressionInfo)
	ztocFlatbuf.CompressionInfo(compressionInfo)
	ztoc.CompressionInfo.MaxSpanID = SpanID(compressionInfo.MaxSpanId())
	ztoc.CompressionInfo.SpanDigests = make([]digest.Digest, compressionInfo.SpanDigestsLength())
	for i := 0; i < compressionInfo.SpanDigestsLength(); i++ {
		dgst, _ := digest.Parse(string(compressionInfo.SpanDigests(i)))
		ztoc.CompressionInfo.SpanDigests[i] = dgst
	}
	ztoc.CompressionInfo.Checkpoints = compressionInfo.CheckpointsBytes()
	return ztoc
}

// Get file mode from ztoc
func GetFileMode(src *FileMetadata) (m os.FileMode) {
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
