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

package compression

import (
	"bufio"
	"bytes"
	"cmp"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"sort"
	"sync"

	zinfo_flatbuffers "github.com/awslabs/soci-snapshotter/ztoc/compression/fbs/zinfo"
	flatbuffers "github.com/google/flatbuffers/go"
	"github.com/klauspost/compress/zstd"
)

// ZstdZinfo implements the `Zinfo` interface for zstd compressed streams.
//
// A zstd frame can only be decoded from the start, because every block may
// depend on the decoder state left by the blocks before it: the window of
// previously decoded data (up to the frame's window size),
// the three repeat offsets, and the entropy tables. To start decoding at a
// block boundary, a checkpoint records that state. Storing the whole window
// would make the zinfo larger than the layer, so a checkpoint keeps only the
// window bytes that the blocks of its span actually reference.
//
// To decode a span, the checkpoint is turned into a zstd dictionary (window
// bytes, entropy tables and repeat offsets) and the span's blocks are
// wrapped in a new frame that uses this dictionary. The window bytes that
// the span does not reference are left as zeros.
type ZstdZinfo struct {
	version     int32
	spanSize    int64
	checkpoints []zstdCheckpoint
}

type zstdCheckpoint struct {
	compressedOffset   int64
	uncompressedOffset int64
	frameOffset        int64
	windowSize         int64
	frameHasChecksum   bool
	repeatOffsets      [3]uint32
	huffmanTable       []byte
	llTable            []byte
	ofTable            []byte
	mlTable            []byte
	history            []byte // compressed, see setHistory
}

const (
	zstdZinfoVersion = 1

	// zstdDictID is the dictionary ID used in the frames created to decode a span.
	zstdDictID = 1

	// zstdHistoryMergeGap is the largest gap between two referenced window
	// ranges that is stored instead of starting a new range. Merging small
	// gaps keeps the range list short at the cost of a few extra bytes.
	zstdHistoryMergeGap = 8

	// zstdMaxHistoryRanges is the largest number of window ranges that a
	// checkpoint stores. Above it, the ranges are merged with larger gaps.
	zstdMaxHistoryRanges = 1 << 16

	// zstdMaxZinfoRatio limits the size of the checkpoint histories to the
	// compressed layer size divided by this value. Data that repeats within
	// the window, such as copies of a file, can make every span reference
	// most of the window, and the zinfo would be larger than the layer.
	zstdMaxZinfoRatio = 2

	// zstdMinHistoryBudget is the history size per checkpoint that is always
	// allowed, whatever the compressed layer size.
	zstdMinHistoryBudget = 64 << 10

	// zstdMaxConcurrentDecodes limits the number of spans decoded at the
	// same time. A decode that starts inside a frame needs memory for the
	// checkpoint dictionary and the decoder's window, up to about three
	// windows.
	zstdMaxConcurrentDecodes = 4
)

// A Huffman tree description with two symbols. It is used in a checkpoint
// when no Huffman table was set in the frame yet, because a dictionary must
// contain one. Blocks after such a checkpoint cannot reference it.
var zstdPlaceholderHuffmanTable = []byte{128, 0x10}

var zstdHistoryEncoder = sync.OnceValues(func() (*zstd.Encoder, error) {
	return zstd.NewWriter(nil, zstd.WithEncoderConcurrency(1), zstd.WithEncoderLevel(zstd.SpeedDefault))
})

// zstdDecoder decodes spans that start at the beginning of a frame.
var zstdDecoder = sync.OnceValues(func() (*zstd.Decoder, error) {
	return zstd.NewReader(nil,
		zstd.WithDecoderConcurrency(zstdMaxConcurrentDecodes),
		zstd.WithDecoderMaxWindow(zstdMaxWindowSize))
})

// zstdHistoryDecoder decodes checkpoint histories. A history holds at most a
// window of bytes plus the range list, so its decoded size is limited.
var zstdHistoryDecoder = sync.OnceValues(func() (*zstd.Decoder, error) {
	return zstd.NewReader(nil,
		zstd.WithDecoderConcurrency(zstdMaxConcurrentDecodes),
		zstd.WithDecoderMaxMemory(4*zstdMaxWindowSize))
})

// zstdDictDecoders holds the decoders that decode spans with a checkpoint
// dictionary. A decoder is created when it is first needed. Taking one from
// the channel limits the number of these decodes that run at the same time.
var zstdDictDecoders = func() chan *zstd.Decoder {
	c := make(chan *zstd.Decoder, zstdMaxConcurrentDecodes)
	for range zstdMaxConcurrentDecodes {
		c <- nil
	}
	return c
}()

// newZstdZinfo creates a new instance of `ZstdZinfo` from serialized bytes.
func newZstdZinfo(zinfoBytes []byte) (zinfo *ZstdZinfo, err error) {
	defer func() {
		if r := recover(); r != nil {
			zinfo = nil
			err = fmt.Errorf("cannot unmarshal zstd zinfo: %v", r)
		}
	}()
	if len(zinfoBytes) == 0 {
		return nil, errors.New("empty checkpoints")
	}
	fb := zinfo_flatbuffers.GetRootAsZstdZinfo(zinfoBytes, 0)
	if v := fb.Version(); v != zstdZinfoVersion {
		return nil, fmt.Errorf("unsupported zstd zinfo version %d", v)
	}
	// Every checkpoint takes at least a 4-byte offset in the buffer.
	n := fb.CheckpointsLength()
	if n == 0 || n > len(zinfoBytes)/4 {
		return nil, fmt.Errorf("invalid number of checkpoints: %d", n)
	}
	zinfo = &ZstdZinfo{
		version:     fb.Version(),
		spanSize:    fb.SpanSize(),
		checkpoints: make([]zstdCheckpoint, n),
	}
	var c zinfo_flatbuffers.ZstdCheckpoint
	for i := range zinfo.checkpoints {
		fb.Checkpoints(&c, i)
		if c.RepeatOffsetsLength() != 3 {
			return nil, fmt.Errorf("checkpoint %d has %d repeat offsets", i, c.RepeatOffsetsLength())
		}
		zinfo.checkpoints[i] = zstdCheckpoint{
			compressedOffset:   c.CompressedOffset(),
			uncompressedOffset: c.UncompressedOffset(),
			frameOffset:        c.FrameOffset(),
			windowSize:         c.WindowSize(),
			frameHasChecksum:   c.FrameHasChecksum(),
			repeatOffsets:      [3]uint32{c.RepeatOffsets(0), c.RepeatOffsets(1), c.RepeatOffsets(2)},
			huffmanTable:       bytes.Clone(c.HuffmanTableBytes()),
			llTable:            bytes.Clone(c.LiteralLengthsTableBytes()),
			ofTable:            bytes.Clone(c.OffsetsTableBytes()),
			mlTable:            bytes.Clone(c.MatchLengthsTableBytes()),
			history:            bytes.Clone(c.HistoryBytes()),
		}
		if err := zinfo.checkpoints[i].validate(); err != nil {
			return nil, fmt.Errorf("checkpoint %d: %w", i, err)
		}
		if i > 0 {
			prev := &zinfo.checkpoints[i-1]
			cur := &zinfo.checkpoints[i]
			if cur.compressedOffset <= prev.compressedOffset || cur.uncompressedOffset <= prev.uncompressedOffset {
				return nil, fmt.Errorf("checkpoint %d: offsets are not increasing", i)
			}
		}
	}
	return zinfo, nil
}

// validate checks the fields of a checkpoint read from a zinfo.
func (c *zstdCheckpoint) validate() error {
	if c.compressedOffset < 0 || c.uncompressedOffset < 0 || c.frameOffset < 0 || c.frameOffset > c.uncompressedOffset {
		return errors.New("invalid offsets")
	}
	if c.windowSize < 0 || c.windowSize > zstdMaxWindowSize {
		return fmt.Errorf("invalid window size %d", c.windowSize)
	}
	if c.frameOffset != 0 {
		for _, o := range c.repeatOffsets {
			// Before the first sequence of a frame, the repeat offsets are
			// 1, 4 and 8, which can be larger than a tiny window.
			if o == 0 || int64(o) > max(c.windowSize, 8) {
				return fmt.Errorf("invalid repeat offset %d", o)
			}
		}
		if len(c.huffmanTable) == 0 || len(c.llTable) == 0 || len(c.ofTable) == 0 || len(c.mlTable) == 0 {
			return errors.New("missing entropy tables")
		}
	}
	return nil
}

// newZstdZinfoFromFile creates a new instance of `ZstdZinfo` given zstd file name and span size.
func newZstdZinfoFromFile(zstdFile string, spanSize int64) (*ZstdZinfo, error) {
	if spanSize <= 0 {
		return nil, fmt.Errorf("invalid span size %d", spanSize)
	}
	f, err := os.Open(zstdFile)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// Pass 1: parse the stream, choose the checkpoints and find which window
	// bytes each span references.
	var (
		cps      []zstdCheckpoint
		ranges   [][][2]int64 // per checkpoint, absolute uncompressed [start, end)
		frameAbs int64        // uncompressed offset of the current frame
		maxWin   int64
		mergeGap int64 // merge gap of the current checkpoint's ranges
	)
	merge := func() {
		r := mergeZstdRanges(ranges[len(ranges)-1], mergeGap)
		for len(r) > zstdMaxHistoryRanges {
			mergeGap *= 2
			r = mergeZstdRanges(r, mergeGap)
		}
		ranges[len(ranges)-1] = r
	}
	onBlock := func(frame *zstdFrameHeader, compOff, uncompOff, framePos uint64, s *zstdBlockState, h zstdBlockHeader, block []byte) error {
		frameAbs = int64(uncompOff - framePos)
		if int64(frame.windowSize) > maxWin {
			maxWin = int64(frame.windowSize)
		}
		if len(cps) > 0 && int64(uncompOff)-cps[len(cps)-1].uncompressedOffset <= spanSize {
			return nil
		}
		if len(cps) > 0 && !s.replacesRLETables(h, block) {
			// A dictionary cannot hold an RLE table, so the checkpoint moves
			// to a later block that replaces the RLE tables.
			return nil
		}
		if len(ranges) > 0 {
			merge()
		}
		cps = append(cps, newZstdCheckpoint(frame, int64(compOff), int64(uncompOff), int64(framePos), s))
		ranges = append(ranges, nil)
		mergeGap = zstdHistoryMergeGap
		return nil
	}
	onMatch := func(m zstdMatch) {
		cur := len(cps) - 1
		cpStart := cps[cur].uncompressedOffset
		src := frameAbs + int64(m.pos-m.offset)
		if src >= cpStart {
			return
		}
		end := min(src+int64(m.length), cpStart)
		r := ranges[cur]
		if n := len(r); n > 0 && r[n-1][0] <= src && src <= r[n-1][1] {
			// Common case: the match continues the previous range.
			r[n-1][1] = max(r[n-1][1], end)
			return
		}
		ranges[cur] = append(r, [2]int64{src, end})
		if len(ranges[cur]) >= 2*zstdMaxHistoryRanges {
			merge()
		}
	}
	compressedSize, uncompressedSize, err := walkZstdStream(f, onBlock, onMatch)
	if err != nil {
		return nil, fmt.Errorf("could not parse zstd stream: %w", err)
	}
	// Streams often end with an empty block. Don't make an empty span for it.
	if n := len(cps); n > 1 && cps[n-1].uncompressedOffset == int64(uncompressedSize) {
		cps, ranges = cps[:n-1], ranges[:n-1]
	}
	merge()

	// Pass 2: decompress the stream and copy the referenced window bytes
	// into each checkpoint.
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	dec, err := zstd.NewReader(bufio.NewReaderSize(f, 1<<20), zstd.WithDecoderConcurrency(1), zstd.WithDecoderMaxWindow(zstdMaxWindowSize))
	if err != nil {
		return nil, err
	}
	defer dec.Close()
	hist := newZstdRing(maxWin)
	var historySize int64
	historyBudget := max(int64(compressedSize)/zstdMaxZinfoRatio, int64(len(cps))*zstdMinHistoryBudget)
	for i := range cps {
		if _, err := io.CopyN(hist, dec, cps[i].uncompressedOffset-hist.offset); err != nil {
			return nil, fmt.Errorf("could not decompress zstd stream: %w", err)
		}
		if err := cps[i].setHistory(hist, ranges[i]); err != nil {
			return nil, err
		}
		ranges[i] = nil
		historySize += int64(len(cps[i].history))
		if historySize > historyBudget {
			return nil, fmt.Errorf("%w: checkpoint histories exceed %d bytes", ErrUnsupportedZstdFeature, historyBudget)
		}
	}
	// Read the rest of the stream so that the decoder verifies the checksums.
	rest, err := io.Copy(io.Discard, dec)
	if err != nil {
		return nil, fmt.Errorf("could not decompress zstd stream: %w", err)
	}
	if hist.offset+rest != int64(uncompressedSize) {
		return nil, fmt.Errorf("zstd stream decompresses to %d bytes, parsed %d", hist.offset+rest, uncompressedSize)
	}

	return &ZstdZinfo{
		version:     zstdZinfoVersion,
		spanSize:    spanSize,
		checkpoints: cps,
	}, nil
}

func newZstdCheckpoint(frame *zstdFrameHeader, compOff, uncompOff, framePos int64, s *zstdBlockState) zstdCheckpoint {
	c := zstdCheckpoint{
		compressedOffset:   compOff,
		uncompressedOffset: uncompOff,
		frameOffset:        framePos,
		windowSize:         int64(frame.windowSize),
		frameHasChecksum:   frame.hasChecksum,
		repeatOffsets:      s.repeat,
	}
	if framePos == 0 {
		// The start of a frame needs no state.
		return c
	}
	c.huffmanTable = bytes.Clone(s.huffman)
	if c.huffmanTable == nil {
		c.huffmanTable = bytes.Clone(zstdPlaceholderHuffmanTable)
	}
	tables := [3]*[]byte{&c.llTable, &c.ofTable, &c.mlTable}
	for i, t := range s.seq {
		var d fseDistribution
		switch t.mode {
		case zstdTableNone, zstdTablePredefined, zstdTableRLE:
			// The block at a checkpoint replaces RLE tables, so any valid
			// table can stand in for them.
			d = zstdPredefDist[i]
		case zstdTableFSE:
			d = t.dist
		}
		*tables[i] = appendFSEDistribution(nil, d)
	}
	return c
}

// mergeZstdRanges sorts and merges ranges that overlap or are at most gap
// bytes apart.
func mergeZstdRanges(r [][2]int64, gap int64) [][2]int64 {
	if len(r) == 0 {
		return r
	}
	slices.SortFunc(r, func(a, b [2]int64) int { return cmp.Compare(a[0], b[0]) })
	out := r[:1]
	for _, x := range r[1:] {
		last := &out[len(out)-1]
		if x[0] <= last[1]+gap {
			last[1] = max(last[1], x[1])
			continue
		}
		out = append(out, x)
	}
	return slices.Clip(out)
}

// setHistory stores the window bytes in `ranges`, which hist must hold.
//
// The history is encoded as varints: the size of the dictionary content,
// the number of ranges, then for each range its distance from the end of the
// previous one (or from the start of the content) and its length, followed
// by the bytes of all ranges. The whole encoding is zstd-compressed.
func (c *zstdCheckpoint) setHistory(hist *zstdRing, ranges [][2]int64) error {
	if c.frameOffset == 0 {
		return nil
	}
	// The dictionary content must reach the furthest referenced byte and,
	// for the decoder to accept the dictionary, the largest repeat offset.
	size := int64(8)
	for _, o := range c.repeatOffsets {
		size = max(size, int64(o))
	}
	if len(ranges) > 0 {
		size = max(size, c.uncompressedOffset-ranges[0][0])
	}
	contentStart := c.uncompressedOffset - size

	var b []byte
	b = binary.AppendUvarint(b, uint64(size))
	b = binary.AppendUvarint(b, uint64(len(ranges)))
	prev := contentStart
	for _, r := range ranges {
		b = binary.AppendUvarint(b, uint64(r[0]-prev))
		b = binary.AppendUvarint(b, uint64(r[1]-r[0]))
		prev = r[1]
	}
	for _, r := range ranges {
		var err error
		b, err = hist.appendRange(b, r[0], r[1])
		if err != nil {
			return err
		}
	}
	enc, err := zstdHistoryEncoder()
	if err != nil {
		return err
	}
	c.history = enc.EncodeAll(b, nil)
	return nil
}

// dictionary returns the zstd dictionary that holds the decoder state of the
// checkpoint. RFC 8878 5.
func (c *zstdCheckpoint) dictionary() ([]byte, error) {
	hdec, err := zstdHistoryDecoder()
	if err != nil {
		return nil, err
	}
	b, err := hdec.DecodeAll(c.history, nil)
	if err != nil {
		return nil, fmt.Errorf("could not decompress checkpoint history: %w", err)
	}
	r := bytes.NewReader(b)
	size, err := binary.ReadUvarint(r)
	if err != nil {
		return nil, err
	}
	n, err := binary.ReadUvarint(r)
	if err != nil {
		return nil, err
	}
	// Each range takes at least two bytes.
	if size > uint64(c.windowSize)+8 || n > uint64(r.Len())/2 {
		return nil, errors.New("invalid checkpoint history")
	}
	ranges := make([][2]uint64, 0, n)
	var pos uint64
	for i := uint64(0); i < n; i++ {
		gap, err := binary.ReadUvarint(r)
		if err != nil {
			return nil, err
		}
		length, err := binary.ReadUvarint(r)
		if err != nil {
			return nil, err
		}
		if gap > size-pos || length > size-pos-gap {
			return nil, errors.New("invalid checkpoint history")
		}
		pos += gap
		ranges = append(ranges, [2]uint64{pos, pos + length})
		pos += length
	}

	var dict []byte
	dict = binary.LittleEndian.AppendUint32(dict, 0xEC30A437)
	dict = binary.LittleEndian.AppendUint32(dict, zstdDictID)
	dict = append(dict, c.huffmanTable...)
	dict = append(dict, c.ofTable...)
	dict = append(dict, c.mlTable...)
	dict = append(dict, c.llTable...)
	for _, o := range c.repeatOffsets {
		dict = binary.LittleEndian.AppendUint32(dict, o)
	}
	content := len(dict)
	dict = append(dict, make([]byte, size)...)
	for _, rg := range ranges {
		if _, err := io.ReadFull(r, dict[content+int(rg[0]):content+int(rg[1])]); err != nil {
			return nil, errors.New("invalid checkpoint history")
		}
	}
	return dict, nil
}

// Close doesn't do anything since there is nothing to close/release.
func (i *ZstdZinfo) Close() {}

// Bytes returns the byte slice containing the `ZstdZinfo`.
func (i *ZstdZinfo) Bytes() (fb []byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			fb = nil
			err = fmt.Errorf("failed to generate zstd zinfo flatbuf bytes: %v", r)
		}
	}()

	builder := flatbuffers.NewBuilder(0)
	bytesVector := func(b []byte) flatbuffers.UOffsetT {
		if b == nil {
			return 0
		}
		return builder.CreateByteVector(b)
	}
	offsets := make([]flatbuffers.UOffsetT, len(i.checkpoints))
	for n, c := range i.checkpoints {
		huffman := bytesVector(c.huffmanTable)
		ll := bytesVector(c.llTable)
		of := bytesVector(c.ofTable)
		ml := bytesVector(c.mlTable)
		history := bytesVector(c.history)
		zinfo_flatbuffers.ZstdCheckpointStartRepeatOffsetsVector(builder, 3)
		for j := 2; j >= 0; j-- {
			builder.PrependUint32(c.repeatOffsets[j])
		}
		reps := builder.EndVector(3)

		zinfo_flatbuffers.ZstdCheckpointStart(builder)
		zinfo_flatbuffers.ZstdCheckpointAddCompressedOffset(builder, c.compressedOffset)
		zinfo_flatbuffers.ZstdCheckpointAddUncompressedOffset(builder, c.uncompressedOffset)
		zinfo_flatbuffers.ZstdCheckpointAddFrameOffset(builder, c.frameOffset)
		zinfo_flatbuffers.ZstdCheckpointAddWindowSize(builder, c.windowSize)
		zinfo_flatbuffers.ZstdCheckpointAddFrameHasChecksum(builder, c.frameHasChecksum)
		zinfo_flatbuffers.ZstdCheckpointAddRepeatOffsets(builder, reps)
		if huffman != 0 {
			zinfo_flatbuffers.ZstdCheckpointAddHuffmanTable(builder, huffman)
			zinfo_flatbuffers.ZstdCheckpointAddLiteralLengthsTable(builder, ll)
			zinfo_flatbuffers.ZstdCheckpointAddOffsetsTable(builder, of)
			zinfo_flatbuffers.ZstdCheckpointAddMatchLengthsTable(builder, ml)
		}
		if history != 0 {
			zinfo_flatbuffers.ZstdCheckpointAddHistory(builder, history)
		}
		offsets[n] = zinfo_flatbuffers.ZstdCheckpointEnd(builder)
	}
	zinfo_flatbuffers.ZstdZinfoStartCheckpointsVector(builder, len(offsets))
	for n := len(offsets) - 1; n >= 0; n-- {
		builder.PrependUOffsetT(offsets[n])
	}
	checkpoints := builder.EndVector(len(offsets))

	zinfo_flatbuffers.ZstdZinfoStart(builder)
	zinfo_flatbuffers.ZstdZinfoAddVersion(builder, i.version)
	zinfo_flatbuffers.ZstdZinfoAddSpanSize(builder, i.spanSize)
	zinfo_flatbuffers.ZstdZinfoAddCheckpoints(builder, checkpoints)
	builder.Finish(zinfo_flatbuffers.ZstdZinfoEnd(builder))
	return builder.FinishedBytes(), nil
}

// MaxSpanID returns the max span ID.
func (i *ZstdZinfo) MaxSpanID() SpanID {
	return SpanID(len(i.checkpoints) - 1)
}

// SpanSize returns the span size of the constructed zinfo.
func (i *ZstdZinfo) SpanSize() Offset {
	return Offset(i.spanSize)
}

// UncompressedOffsetToSpanID returns the ID of the span containing the data pointed by uncompressed offset.
func (i *ZstdZinfo) UncompressedOffsetToSpanID(offset Offset) SpanID {
	n := sort.Search(len(i.checkpoints), func(j int) bool {
		return i.checkpoints[j].uncompressedOffset > int64(offset)
	})
	return SpanID(max(n-1, 0))
}

// ExtractDataFromBuffer decompresses `compressedBuf`, which holds the
// compressed data from the start of span `spanID` up to the end of a span,
// and returns the `uncompressedSize` bytes at `uncompressedOffset`.
//
// Each span is decoded from its own checkpoint, because a checkpoint only
// holds the window bytes that its own span references.
func (i *ZstdZinfo) ExtractDataFromBuffer(compressedBuf []byte, uncompressedSize, uncompressedOffset Offset, spanID SpanID) ([]byte, error) {
	if len(compressedBuf) == 0 {
		return nil, errors.New("empty compressed buffer")
	}
	if uncompressedSize < 0 {
		return nil, fmt.Errorf("invalid uncompressed size: %d", uncompressedSize)
	}
	if uncompressedSize == 0 {
		return []byte{}, nil
	}
	if spanID < 0 || spanID > i.MaxSpanID() {
		return nil, fmt.Errorf("invalid span ID %d", spanID)
	}
	start := int64(uncompressedOffset) - i.checkpoints[spanID].uncompressedOffset
	if start < 0 {
		return nil, fmt.Errorf("offset %d is before span %d", uncompressedOffset, spanID)
	}
	want := start + int64(uncompressedSize)
	base := i.checkpoints[spanID].compressedOffset
	out := make([]byte, 0, want)
	for id := spanID; int64(len(out)) < want; id++ {
		if id > i.MaxSpanID() {
			return nil, fmt.Errorf("requested data ends after the end of the stream")
		}
		c := &i.checkpoints[id]
		cs, ce := c.compressedOffset-base, int64(len(compressedBuf))
		if id < i.MaxSpanID() {
			ce = i.checkpoints[id+1].compressedOffset - base
		}
		if ce > int64(len(compressedBuf)) || cs >= ce {
			return nil, fmt.Errorf("compressed buffer does not hold span %d", id)
		}
		n := len(out)
		var err error
		out, err = c.decode(compressedBuf[cs:ce], out)
		if err != nil {
			return nil, err
		}
		if id < i.MaxSpanID() && int64(len(out)-n) != i.checkpoints[id+1].uncompressedOffset-c.uncompressedOffset {
			return nil, fmt.Errorf("span %d decompressed to %d bytes, expected %d", id, len(out)-n,
				i.checkpoints[id+1].uncompressedOffset-c.uncompressedOffset)
		}
	}
	return out[start:want], nil
}

// ExtractDataFromFile reads the spans that hold the requested data from a
// zstd file and decompresses them.
func (i *ZstdZinfo) ExtractDataFromFile(fileName string, uncompressedSize, uncompressedOffset Offset) ([]byte, error) {
	if uncompressedSize < 0 {
		return nil, fmt.Errorf("invalid uncompressed size: %d", uncompressedSize)
	}
	if uncompressedSize == 0 {
		return []byte{}, nil
	}
	f, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	first := i.UncompressedOffsetToSpanID(uncompressedOffset)
	last := i.UncompressedOffsetToSpanID(uncompressedOffset + uncompressedSize - 1)
	start := i.StartCompressedOffset(first)
	end := i.EndCompressedOffset(last, Offset(st.Size()))
	buf := make([]byte, end-start)
	if _, err := f.ReadAt(buf, int64(start)); err != nil {
		return nil, fmt.Errorf("failed to read compressed data: %w", err)
	}
	return i.ExtractDataFromBuffer(buf, uncompressedSize, uncompressedOffset, first)
}

// StartCompressedOffset returns the start offset of the span in the compressed stream.
func (i *ZstdZinfo) StartCompressedOffset(spanID SpanID) Offset {
	return Offset(i.checkpoints[spanID].compressedOffset)
}

// EndCompressedOffset returns the end offset of the span in the compressed stream. If
// it's the last span, returns the size of the compressed stream.
func (i *ZstdZinfo) EndCompressedOffset(spanID SpanID, fileSize Offset) Offset {
	if spanID == i.MaxSpanID() {
		return fileSize
	}
	return Offset(i.checkpoints[spanID+1].compressedOffset)
}

// StartUncompressedOffset returns the start offset of the span in the uncompressed stream.
func (i *ZstdZinfo) StartUncompressedOffset(spanID SpanID) Offset {
	return Offset(i.checkpoints[spanID].uncompressedOffset)
}

// EndUncompressedOffset returns the end offset of the span in the uncompressed stream. If
// it's the last span, returns the size of the uncompressed stream.
func (i *ZstdZinfo) EndUncompressedOffset(spanID SpanID, fileSize Offset) Offset {
	if spanID == i.MaxSpanID() {
		return fileSize
	}
	return Offset(i.checkpoints[spanID+1].uncompressedOffset)
}

// VerifyHeader checks that r holds the zstd data before the first span:
// any skippable frames followed by a frame header.
func (i *ZstdZinfo) VerifyHeader(r io.Reader) error {
	b, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	for len(b) >= 8 && binary.LittleEndian.Uint32(b)&zstdSkippableMask == zstdSkippableMagic {
		n := uint64(binary.LittleEndian.Uint32(b[4:]))
		if n > uint64(len(b)-8) {
			return fmt.Errorf("%w: truncated skippable frame", errZstdCorrupt)
		}
		b = b[8+n:]
	}
	h, err := parseZstdFrameHeader(b)
	if err != nil {
		return err
	}
	if h.headerSize != len(b) {
		return fmt.Errorf("%w: unexpected data after frame header", errZstdCorrupt)
	}
	c := &i.checkpoints[0]
	if int64(h.windowSize) != c.windowSize || h.hasChecksum != c.frameHasChecksum {
		return errors.New("zstd frame header does not match the zinfo")
	}
	return nil
}

// decode decompresses buf, which holds the compressed data of the
// checkpoint's span, and appends the result to out.
//
// buf is split into segments, one per frame. The first segment continues the
// checkpoint's frame and is decoded with the checkpoint's dictionary unless
// the checkpoint is at the start of the frame. If buf ends inside a frame,
// the last block in buf is marked as the last block of its segment.
func (c *zstdCheckpoint) decode(buf []byte, out []byte) ([]byte, error) {
	pos := 0
	frame := zstdFrameHeader{windowSize: uint64(c.windowSize), hasChecksum: c.frameHasChecksum}
	useDict := c.frameOffset != 0
	for pos < len(buf) {
		// Collect the blocks of this frame that are in buf.
		segStart := pos
		last := false
		var lastHeader int
		for pos < len(buf) && !last {
			h, err := parseZstdBlockHeader(buf[pos:])
			if err != nil {
				return nil, fmt.Errorf("block at offset %d: %w", c.compressedOffset+int64(pos), err)
			}
			if pos+zstdBlockHeaderSize+h.size > len(buf) {
				return nil, fmt.Errorf("block at offset %d: %w", c.compressedOffset+int64(pos), io.ErrUnexpectedEOF)
			}
			lastHeader = pos
			last = h.last
			pos += zstdBlockHeaderSize + h.size
		}
		seg := appendZstdFrameHeader(make([]byte, 0, 18+pos-segStart), frame.windowSize, useDict)
		hdr := len(seg)
		seg = append(seg, buf[segStart:pos]...)
		seg[hdr+lastHeader-segStart] |= 1

		var err error
		if useDict {
			out, err = c.decodeWithDictionary(seg, out)
		} else {
			var dec *zstd.Decoder
			if dec, err = zstdDecoder(); err == nil {
				out, err = dec.DecodeAll(seg, out)
			}
		}
		if err != nil {
			return nil, fmt.Errorf("could not decompress span at offset %d: %w", c.compressedOffset, err)
		}

		if !last {
			break
		}
		if frame.hasChecksum {
			pos += zstdChecksumSize
		}
		// Find the next frame header, if buf has one with blocks after it.
		for pos+8 <= len(buf) && binary.LittleEndian.Uint32(buf[pos:])&zstdSkippableMask == zstdSkippableMagic {
			pos += 8 + int(binary.LittleEndian.Uint32(buf[pos+4:]))
		}
		if pos >= len(buf) {
			break
		}
		frame, err = parseZstdFrameHeader(buf[pos:])
		if err != nil {
			return nil, err
		}
		pos += frame.headerSize
		useDict = false
	}
	return out, nil
}

func (c *zstdCheckpoint) decodeWithDictionary(seg, out []byte) ([]byte, error) {
	dec := <-zstdDictDecoders
	defer func() { zstdDictDecoders <- dec }()
	dict, err := c.dictionary()
	if err != nil {
		return nil, err
	}
	if dec == nil {
		dec, err = zstd.NewReader(nil,
			zstd.WithDecoderConcurrency(1),
			zstd.WithDecoderMaxWindow(zstdMaxWindowSize))
		if err != nil {
			return nil, err
		}
	}
	if err := dec.ResetWithOptions(nil, zstd.WithDecoderDicts(dict)); err != nil {
		return nil, fmt.Errorf("could not load checkpoint dictionary: %w", err)
	}
	out, err = dec.DecodeAll(seg, out)
	// Don't keep the dictionary while the decoder is idle.
	if derr := dec.ResetWithOptions(nil, zstd.WithDecoderDictDelete()); derr != nil && err == nil {
		err = derr
	}
	return out, err
}

// appendZstdFrameHeader appends a frame header with no content size and no
// checksum, and a window size of at least windowSize.
func appendZstdFrameHeader(b []byte, windowSize uint64, useDict bool) []byte {
	b = binary.LittleEndian.AppendUint32(b, zstdFrameMagic)
	desc := byte(0)
	if useDict {
		desc |= 3 // 4-byte dictionary ID
	}
	b = append(b, desc)
	// Window_Descriptor: the smallest window that holds windowSize. RFC 8878 3.1.1.1.2.
	wd := byte(0)
	for e := uint64(0); e < 32; e++ {
		base := uint64(1) << (10 + e)
		found := false
		for m := uint64(0); m < 8; m++ {
			if base+base/8*m >= windowSize {
				wd = byte(e<<3 | m)
				found = true
				break
			}
		}
		if found {
			break
		}
	}
	b = append(b, wd)
	if useDict {
		b = binary.LittleEndian.AppendUint32(b, zstdDictID)
	}
	return b
}

// zstdRing holds the last bytes of a stream written to it.
type zstdRing struct {
	buf    []byte
	offset int64 // stream offset of the next byte written
}

func newZstdRing(size int64) *zstdRing {
	return &zstdRing{buf: make([]byte, max(size, 1))}
}

func (r *zstdRing) Write(p []byte) (int, error) {
	n := len(p)
	size := int64(len(r.buf))
	if int64(len(p)) > size {
		r.offset += int64(len(p)) - size
		p = p[int64(len(p))-size:]
	}
	for len(p) > 0 {
		k := copy(r.buf[r.offset%size:], p)
		p = p[k:]
		r.offset += int64(k)
	}
	return n, nil
}

// appendRange appends the stream bytes [start, end) to b.
func (r *zstdRing) appendRange(b []byte, start, end int64) ([]byte, error) {
	size := int64(len(r.buf))
	if start < r.offset-size || end > r.offset || start > end {
		return nil, fmt.Errorf("history range [%d, %d) is outside the window", start, end)
	}
	for start < end {
		i := start % size
		n := min(end-start, size-i)
		b = append(b, r.buf[i:i+n]...)
		start += n
	}
	return b, nil
}
