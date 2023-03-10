package ztoc

import (
	"fmt"
	"sort"
	"time"

	"github.com/awslabs/soci-snapshotter/compression"
	ztoc_flatbuffers "github.com/awslabs/soci-snapshotter/ztoc/fbs/ztoc"
	flatbuffers "github.com/google/flatbuffers/go"
	"github.com/opencontainers/go-digest"
)

func flatbufToZtoc(flatbuffer []byte) (z *Ztoc, err error) {
	defer func() {
		if r := recover(); r != nil {
			z = nil
			err = fmt.Errorf("cannot unmarshal ztoc: %v", r)
		}
	}()

	ztoc := new(Ztoc)
	ztocFlatbuf := ztoc_flatbuffers.GetRootAsZtoc(flatbuffer, 0)
	ztoc.Version = string(ztocFlatbuf.Version())
	ztoc.CompressedArchiveSize = compression.Offset(ztocFlatbuf.CompressedArchiveSize())
	ztoc.UncompressedArchiveSize = compression.Offset(ztocFlatbuf.UncompressedArchiveSize())

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
		me.UncompressedOffset = compression.Offset(metadataEntry.UncompressedOffset())
		me.UncompressedSize = compression.Offset(metadataEntry.UncompressedSize())
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
	ztoc.CompressionInfo.MaxSpanID = compression.SpanID(compressionInfo.MaxSpanId())
	ztoc.CompressionInfo.SpanDigests = make([]digest.Digest, compressionInfo.SpanDigestsLength())
	for i := 0; i < compressionInfo.SpanDigestsLength(); i++ {
		dgst, _ := digest.Parse(string(compressionInfo.SpanDigests(i)))
		ztoc.CompressionInfo.SpanDigests[i] = dgst
	}
	ztoc.CompressionInfo.Checkpoints = compressionInfo.CheckpointsBytes()
	return ztoc, nil
}

func ztocToFlatbuffer(ztoc *Ztoc) (fb []byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			fb = nil
			err = fmt.Errorf("cannot marshal Ztoc to flatbuffers")
		}
	}()

	builder := flatbuffers.NewBuilder(0)
	version := builder.CreateString(ztoc.Version)

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
	ztoc_flatbuffers.ZtocAddToc(builder, toc)
	ztoc_flatbuffers.ZtocAddCompressedArchiveSize(builder, int64(ztoc.CompressedArchiveSize))
	ztoc_flatbuffers.ZtocAddUncompressedArchiveSize(builder, int64(ztoc.UncompressedArchiveSize))
	ztoc_flatbuffers.ZtocAddCompressionInfo(builder, ztocInfo)
	ztocFlatbuf := ztoc_flatbuffers.ZtocEnd(builder)
	builder.Finish(ztocFlatbuf)
	return builder.FinishedBytes(), nil
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
