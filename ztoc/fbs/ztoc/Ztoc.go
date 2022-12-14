// Code generated by the FlatBuffers compiler. DO NOT EDIT.

package ztoc

import (
	flatbuffers "github.com/google/flatbuffers/go"
)

type Ztoc struct {
	_tab flatbuffers.Table
}

func GetRootAsZtoc(buf []byte, offset flatbuffers.UOffsetT) *Ztoc {
	n := flatbuffers.GetUOffsetT(buf[offset:])
	x := &Ztoc{}
	x.Init(buf, n+offset)
	return x
}

func GetSizePrefixedRootAsZtoc(buf []byte, offset flatbuffers.UOffsetT) *Ztoc {
	n := flatbuffers.GetUOffsetT(buf[offset+flatbuffers.SizeUint32:])
	x := &Ztoc{}
	x.Init(buf, n+offset+flatbuffers.SizeUint32)
	return x
}

func (rcv *Ztoc) Init(buf []byte, i flatbuffers.UOffsetT) {
	rcv._tab.Bytes = buf
	rcv._tab.Pos = i
}

func (rcv *Ztoc) Table() flatbuffers.Table {
	return rcv._tab
}

func (rcv *Ztoc) Version() []byte {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(4))
	if o != 0 {
		return rcv._tab.ByteVector(o + rcv._tab.Pos)
	}
	return nil
}

func (rcv *Ztoc) BuildToolIdentifier() []byte {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(6))
	if o != 0 {
		return rcv._tab.ByteVector(o + rcv._tab.Pos)
	}
	return nil
}

func (rcv *Ztoc) CompressedArchiveSize() int64 {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(8))
	if o != 0 {
		return rcv._tab.GetInt64(o + rcv._tab.Pos)
	}
	return 0
}

func (rcv *Ztoc) MutateCompressedArchiveSize(n int64) bool {
	return rcv._tab.MutateInt64Slot(8, n)
}

func (rcv *Ztoc) UncompressedArchiveSize() int64 {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(10))
	if o != 0 {
		return rcv._tab.GetInt64(o + rcv._tab.Pos)
	}
	return 0
}

func (rcv *Ztoc) MutateUncompressedArchiveSize(n int64) bool {
	return rcv._tab.MutateInt64Slot(10, n)
}

func (rcv *Ztoc) Toc(obj *TOC) *TOC {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(12))
	if o != 0 {
		x := rcv._tab.Indirect(o + rcv._tab.Pos)
		if obj == nil {
			obj = new(TOC)
		}
		obj.Init(rcv._tab.Bytes, x)
		return obj
	}
	return nil
}

func (rcv *Ztoc) CompressionInfo(obj *CompressionInfo) *CompressionInfo {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(14))
	if o != 0 {
		x := rcv._tab.Indirect(o + rcv._tab.Pos)
		if obj == nil {
			obj = new(CompressionInfo)
		}
		obj.Init(rcv._tab.Bytes, x)
		return obj
	}
	return nil
}

func ZtocStart(builder *flatbuffers.Builder) {
	builder.StartObject(6)
}
func ZtocAddVersion(builder *flatbuffers.Builder, version flatbuffers.UOffsetT) {
	builder.PrependUOffsetTSlot(0, flatbuffers.UOffsetT(version), 0)
}
func ZtocAddBuildToolIdentifier(builder *flatbuffers.Builder, buildToolIdentifier flatbuffers.UOffsetT) {
	builder.PrependUOffsetTSlot(1, flatbuffers.UOffsetT(buildToolIdentifier), 0)
}
func ZtocAddCompressedArchiveSize(builder *flatbuffers.Builder, compressedArchiveSize int64) {
	builder.PrependInt64Slot(2, compressedArchiveSize, 0)
}
func ZtocAddUncompressedArchiveSize(builder *flatbuffers.Builder, uncompressedArchiveSize int64) {
	builder.PrependInt64Slot(3, uncompressedArchiveSize, 0)
}
func ZtocAddToc(builder *flatbuffers.Builder, toc flatbuffers.UOffsetT) {
	builder.PrependUOffsetTSlot(4, flatbuffers.UOffsetT(toc), 0)
}
func ZtocAddCompressionInfo(builder *flatbuffers.Builder, compressionInfo flatbuffers.UOffsetT) {
	builder.PrependUOffsetTSlot(5, flatbuffers.UOffsetT(compressionInfo), 0)
}
func ZtocEnd(builder *flatbuffers.Builder) flatbuffers.UOffsetT {
	return builder.EndObject()
}
