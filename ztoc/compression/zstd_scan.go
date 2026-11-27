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
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

const (
	zstdFrameMagic      = 0xFD2FB528
	zstdSkippableMask   = 0xFFFFFFF0
	zstdSkippableMagic  = 0x184D2A50
	zstdMaxBlockSize    = 128 << 10
	zstdBlockHeaderSize = 3
	zstdChecksumSize    = 4
	// zstdMaxWindowSize is the largest window that the zstd zinfo supports.
	// A checkpoint can need up to a window of history, and every span decode
	// allocates it, so larger windows (zstd --long, --ultra) are not lazily
	// loaded. 8 MiB is the window that RFC 8878 recommends that decoders
	// support, the default window of BuildKit (klauspost/compress), and the
	// largest window of zstd CLI levels 1 to 19.
	zstdMaxWindowSize = 8 << 20
)

// ErrUnsupportedZstdFeature is returned when a zstd stream is valid but uses
// a feature that the zstd zinfo does not support, such as a window larger
// than 8 MiB or a dictionary.
var ErrUnsupportedZstdFeature = errors.New("unsupported zstd feature")

var errZstdCorrupt = errors.New("corrupt zstd stream")

const (
	zstdBlockRaw        = 0
	zstdBlockRLE        = 1
	zstdBlockCompressed = 2
)

// zstdFrameHeader is the part of a frame header that the zstd zinfo needs.
type zstdFrameHeader struct {
	headerSize  int    // bytes, including the magic number
	windowSize  uint64 // bytes
	hasChecksum bool
	dictID      uint32
}

// parseZstdFrameHeader parses the frame header at the start of b. It returns
// io.ErrUnexpectedEOF if b is too short to contain the header.
// RFC 8878 3.1.1.1.
func parseZstdFrameHeader(b []byte) (zstdFrameHeader, error) {
	if len(b) < 5 {
		return zstdFrameHeader{}, io.ErrUnexpectedEOF
	}
	if binary.LittleEndian.Uint32(b) != zstdFrameMagic {
		return zstdFrameHeader{}, fmt.Errorf("%w: invalid frame magic", errZstdCorrupt)
	}
	desc := b[4]
	singleSegment := desc&(1<<5) != 0
	fcsSize := [4]int{0, 2, 4, 8}[desc>>6]
	if fcsSize == 0 && singleSegment {
		fcsSize = 1
	}
	if desc&(1<<3) != 0 {
		return zstdFrameHeader{}, fmt.Errorf("%w: reserved bit set in frame header", errZstdCorrupt)
	}
	dictIDSize := [4]int{0, 1, 2, 4}[desc&3]
	wdSize := 1
	if singleSegment {
		wdSize = 0
	}
	h := zstdFrameHeader{
		headerSize:  5 + wdSize + dictIDSize + fcsSize,
		hasChecksum: desc&(1<<2) != 0,
	}
	if len(b) < h.headerSize {
		return zstdFrameHeader{}, io.ErrUnexpectedEOF
	}
	p := 5
	if !singleSegment {
		wd := b[p]
		base := uint64(1) << (uint(wd>>3) + 10)
		h.windowSize = base + (base/8)*uint64(wd&7)
		p++
	}
	switch dictIDSize {
	case 1:
		h.dictID = uint32(b[p])
	case 2:
		h.dictID = uint32(binary.LittleEndian.Uint16(b[p:]))
	case 4:
		h.dictID = binary.LittleEndian.Uint32(b[p:])
	}
	p += dictIDSize
	if singleSegment {
		var fcs uint64
		switch fcsSize {
		case 1:
			fcs = uint64(b[p])
		case 2:
			fcs = uint64(binary.LittleEndian.Uint16(b[p:])) + 256
		case 4:
			fcs = uint64(binary.LittleEndian.Uint32(b[p:]))
		case 8:
			fcs = binary.LittleEndian.Uint64(b[p:])
		}
		h.windowSize = fcs
	}
	if h.windowSize > zstdMaxWindowSize {
		return zstdFrameHeader{}, fmt.Errorf("%w: window size %d is larger than %d", ErrUnsupportedZstdFeature, h.windowSize, zstdMaxWindowSize)
	}
	if h.dictID != 0 {
		return zstdFrameHeader{}, fmt.Errorf("%w: frame uses a dictionary", ErrUnsupportedZstdFeature)
	}
	return h, nil
}

// zstdBlockHeader is a parsed block header. RFC 8878 3.1.1.2.
type zstdBlockHeader struct {
	last      bool
	blockType uint8
	size      int // block content size in the compressed stream
	rawSize   int // regenerated size for raw and RLE blocks
}

func parseZstdBlockHeader(b []byte) (zstdBlockHeader, error) {
	if len(b) < zstdBlockHeaderSize {
		return zstdBlockHeader{}, io.ErrUnexpectedEOF
	}
	v := uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16
	h := zstdBlockHeader{
		last:      v&1 != 0,
		blockType: uint8(v>>1) & 3,
	}
	n := int(v >> 3)
	if n > zstdMaxBlockSize {
		return zstdBlockHeader{}, fmt.Errorf("%w: block too large", errZstdCorrupt)
	}
	switch h.blockType {
	case zstdBlockRaw:
		h.size, h.rawSize = n, n
	case zstdBlockRLE:
		h.size, h.rawSize = 1, n
	case zstdBlockCompressed:
		h.size = n
	default:
		return zstdBlockHeader{}, fmt.Errorf("%w: reserved block type", errZstdCorrupt)
	}
	return h, nil
}

// Entropy table modes, kept per table at each block boundary.
const (
	zstdTableNone       = 0 // not set in this frame yet
	zstdTablePredefined = 1
	zstdTableRLE        = 2
	zstdTableFSE        = 3
)

// zstdSeqTable is the state of one sequence decoding table.
type zstdSeqTable struct {
	mode   uint8
	rle    uint8           // symbol, for zstdTableRLE
	dist   fseDistribution // for zstdTableFSE
	states []fseState      // decoding table, for zstdTablePredefined and zstdTableFSE
}

// zstdBlockState is the decoder state at a block boundary that a block may
// depend on, apart from the window: the repeat offsets and the entropy tables.
type zstdBlockState struct {
	repeat  [3]uint32
	huffman []byte          // Huffman tree description, nil if not set yet
	seq     [3]zstdSeqTable // literal lengths, offsets, match lengths
}

func newZstdBlockState() zstdBlockState {
	return zstdBlockState{repeat: [3]uint32{1, 4, 8}}
}

func (s *zstdBlockState) hasRLETable() bool {
	for _, t := range s.seq {
		if t.mode == zstdTableRLE {
			return true
		}
	}
	return false
}

const (
	zstdSeqLL = 0
	zstdSeqOF = 1
	zstdSeqML = 2
)

var (
	zstdSeqMaxSymbol = [3]int{zstdMaxLLSymbol, zstdMaxOFSymbol, zstdMaxMLSymbol}
	zstdSeqMaxLog    = [3]uint8{zstdMaxLLLog, zstdMaxOFLog, zstdMaxMLLog}
	zstdPredefDist   = [3]fseDistribution{
		{accuracyLog: zstdPredefLLLog, counts: zstdPredefLL},
		{accuracyLog: zstdPredefOFLog, counts: zstdPredefOF},
		{accuracyLog: zstdPredefMLLog, counts: zstdPredefML},
	}
	zstdPredefStates [3][]fseState
)

func init() {
	for i, d := range zstdPredefDist {
		t, err := buildFSETable(d)
		if err != nil {
			panic(err)
		}
		zstdPredefStates[i] = t
	}
}

// zstdMatch is a back reference found in a block: `length` bytes copied
// from `offset` bytes before uncompressed position `pos` (relative to the
// start of the frame).
type zstdMatch struct {
	pos    uint64
	offset uint64
	length uint64
}

// scanZstdBlock parses the compressed block in b, updates s and calls
// onMatch for every back reference. framePos is the uncompressed position of
// the start of the block relative to the frame start. It returns the
// regenerated size of the block.
// RFC 8878 3.1.1.3.
func scanZstdBlock(b []byte, s *zstdBlockState, framePos uint64, onMatch func(zstdMatch)) (int, error) {
	litSize, n, err := scanZstdLiterals(b, s)
	if err != nil {
		return 0, err
	}
	b = b[n:]

	// Sequences section header. RFC 8878 3.1.1.3.2.1.
	if len(b) < 1 {
		return 0, fmt.Errorf("%w: missing sequences section", errZstdCorrupt)
	}
	nbSeq, modes, n, err := scanZstdSequencesHeader(b)
	if err != nil {
		return 0, err
	}
	if nbSeq == 0 {
		return litSize, nil
	}
	b = b[n:]
	for i, shift := range zstdSequenceTableShift {
		t := &s.seq[i]
		switch (modes >> shift) & 3 {
		case 0:
			*t = zstdSeqTable{mode: zstdTablePredefined, states: zstdPredefStates[i]}
		case 1:
			if len(b) < 1 {
				return 0, io.ErrUnexpectedEOF
			}
			if int(b[0]) > zstdSeqMaxSymbol[i] {
				return 0, fmt.Errorf("%w: invalid RLE symbol", errZstdCorrupt)
			}
			*t = zstdSeqTable{mode: zstdTableRLE, rle: b[0]}
			b = b[1:]
		case 2:
			d, n, err := readFSEDistribution(b, zstdSeqMaxSymbol[i], zstdSeqMaxLog[i])
			if err != nil {
				return 0, err
			}
			if n > len(b) {
				return 0, io.ErrUnexpectedEOF
			}
			states, err := buildFSETable(d)
			if err != nil {
				return 0, err
			}
			*t = zstdSeqTable{mode: zstdTableFSE, dist: d, states: states}
			b = b[n:]
		case 3:
			if t.mode == zstdTableNone {
				return 0, fmt.Errorf("%w: repeat mode without a previous table", errZstdCorrupt)
			}
		}
	}

	br, err := newBackwardBitReader(b)
	if err != nil {
		return 0, err
	}
	var st [3]uint16
	var ds [3]fseState
	initState := func(i int) {
		t := &s.seq[i]
		if t.mode == zstdTableRLE {
			return
		}
		st[i] = uint16(br.read(uint8(bitsLen(len(t.states)) - 1)))
	}
	symbol := func(i int) uint8 {
		t := &s.seq[i]
		if t.mode == zstdTableRLE {
			return t.rle
		}
		ds[i] = t.states[st[i]]
		return ds[i].symbol
	}
	update := func(i int) {
		t := &s.seq[i]
		if t.mode == zstdTableRLE {
			return
		}
		st[i] = ds[i].baseline + uint16(br.read(ds[i].nbBits))
	}
	initState(zstdSeqLL)
	initState(zstdSeqOF)
	initState(zstdSeqML)

	pos := framePos
	var totalLit uint64
	for i := 0; i < nbSeq; i++ {
		llCode, ofCode, mlCode := symbol(zstdSeqLL), symbol(zstdSeqOF), symbol(zstdSeqML)
		if int(llCode) > zstdMaxLLSymbol || int(mlCode) > zstdMaxMLSymbol || ofCode > 31 {
			return 0, fmt.Errorf("%w: invalid sequence code", errZstdCorrupt)
		}
		ofValue := uint32(1)<<ofCode + uint32(br.read(ofCode))
		ml := uint64(zstdMLBase[mlCode]) + br.read(zstdMLBits[mlCode])
		ll := uint64(zstdLLBase[llCode]) + br.read(zstdLLBits[llCode])

		// Repeat offsets. RFC 8878 3.1.1.5.
		var offset uint32
		r := &s.repeat
		if ofValue > 3 {
			offset = ofValue - 3
			r[2], r[1], r[0] = r[1], r[0], offset
		} else {
			idx := ofValue - 1
			if ll == 0 {
				idx++
			}
			switch idx {
			case 0:
				offset = r[0]
			case 1:
				offset = r[1]
				r[1], r[0] = r[0], offset
			case 2:
				offset = r[2]
				r[2], r[1], r[0] = r[1], r[0], offset
			case 3:
				offset = r[0] - 1
				if offset == 0 {
					return 0, fmt.Errorf("%w: zero offset", errZstdCorrupt)
				}
				r[2], r[1], r[0] = r[1], r[0], offset
			}
		}

		totalLit += ll
		pos += ll
		if uint64(offset) > pos {
			return 0, fmt.Errorf("%w: offset %d beyond start of frame", errZstdCorrupt, offset)
		}
		onMatch(zstdMatch{pos: pos, offset: uint64(offset), length: ml})
		pos += ml

		if i != nbSeq-1 {
			update(zstdSeqLL)
			update(zstdSeqML)
			update(zstdSeqOF)
		}
		if br.overread() {
			return 0, fmt.Errorf("%w: sequences bitstream overread", errZstdCorrupt)
		}
	}
	if !br.done() {
		return 0, fmt.Errorf("%w: sequences bitstream not fully consumed", errZstdCorrupt)
	}
	if totalLit > uint64(litSize) {
		return 0, fmt.Errorf("%w: literal lengths exceed literals", errZstdCorrupt)
	}
	size := pos - framePos + uint64(litSize) - totalLit
	if size > zstdMaxBlockSize {
		return 0, fmt.Errorf("%w: block regenerates too much data", errZstdCorrupt)
	}
	return int(size), nil
}

// scanZstdSequencesHeader parses the start of the sequences section: the
// number of sequences and, if there are any, the table modes byte. It
// returns the number of bytes used. RFC 8878 3.1.1.3.2.1.
func scanZstdSequencesHeader(b []byte) (nbSeq int, modes byte, n int, err error) {
	if len(b) < 1 {
		return 0, 0, 0, fmt.Errorf("%w: missing sequences section", errZstdCorrupt)
	}
	switch {
	case b[0] < 128:
		nbSeq, n = int(b[0]), 1
	case b[0] < 255:
		if len(b) < 2 {
			return 0, 0, 0, io.ErrUnexpectedEOF
		}
		nbSeq, n = (int(b[0]-128)<<8)+int(b[1]), 2
	default:
		if len(b) < 3 {
			return 0, 0, 0, io.ErrUnexpectedEOF
		}
		nbSeq, n = int(b[1])+(int(b[2])<<8)+0x7F00, 3
	}
	if nbSeq == 0 {
		// A zero count can also be written in the 2-byte form.
		return 0, 0, n, nil
	}
	if len(b) < n+1 {
		return 0, 0, 0, io.ErrUnexpectedEOF
	}
	modes = b[n]
	if modes&3 != 0 {
		return 0, 0, 0, fmt.Errorf("%w: reserved bits set in sequence modes", errZstdCorrupt)
	}
	return nbSeq, modes, n + 1, nil
}

// zstdSequenceTableShift is the position of each table's mode in the
// sequence modes byte, in the order literal lengths, offsets, match lengths.
var zstdSequenceTableShift = [3]uint{6, 4, 2}

// replacesRLETables reports whether the block replaces every sequence table
// of s that is in RLE mode. A checkpoint before such a block does not need
// the RLE tables, which a dictionary cannot hold.
func (s *zstdBlockState) replacesRLETables(h zstdBlockHeader, block []byte) bool {
	if !s.hasRLETable() {
		return true
	}
	if h.blockType != zstdBlockCompressed {
		return false
	}
	tmp := zstdBlockState{huffman: []byte{}}
	_, n, err := scanZstdLiterals(block, &tmp)
	if err != nil {
		return false
	}
	nbSeq, modes, _, err := scanZstdSequencesHeader(block[n:])
	if err != nil || nbSeq == 0 {
		return false
	}
	for i, t := range s.seq {
		if t.mode == zstdTableRLE && (modes>>zstdSequenceTableShift[i])&3 == 3 {
			return false
		}
	}
	return true
}

// scanZstdLiterals parses the literals section header at the start of b,
// records the Huffman tree description in s if there is one, and returns
// the regenerated literals size and the size of the whole literals section.
// RFC 8878 3.1.1.3.1.
func scanZstdLiterals(b []byte, s *zstdBlockState) (int, int, error) {
	if len(b) < 1 {
		return 0, 0, io.ErrUnexpectedEOF
	}
	regen, n, err := scanZstdLiteralsHeader(b, s)
	if err != nil {
		return 0, 0, err
	}
	if regen > zstdMaxBlockSize {
		return 0, 0, fmt.Errorf("%w: literals too large", errZstdCorrupt)
	}
	return regen, n, nil
}

func scanZstdLiteralsHeader(b []byte, s *zstdBlockState) (int, int, error) {
	litType := b[0] & 3
	sizeFormat := (b[0] >> 2) & 3
	switch litType {
	case 0, 1: // raw, RLE
		var regen, hsize int
		switch sizeFormat {
		case 0, 2:
			regen, hsize = int(b[0]>>3), 1
		case 1:
			if len(b) < 2 {
				return 0, 0, io.ErrUnexpectedEOF
			}
			regen, hsize = int(b[0]>>4)+int(b[1])<<4, 2
		case 3:
			if len(b) < 3 {
				return 0, 0, io.ErrUnexpectedEOF
			}
			regen, hsize = int(b[0]>>4)+int(b[1])<<4+int(b[2])<<12, 3
		}
		content := regen
		if litType == 1 {
			content = 1
		}
		if hsize+content > len(b) {
			return 0, 0, io.ErrUnexpectedEOF
		}
		return regen, hsize + content, nil
	default: // compressed, treeless
		var regen, comp, hsize int
		switch sizeFormat {
		case 0, 1:
			if len(b) < 3 {
				return 0, 0, io.ErrUnexpectedEOF
			}
			v := uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16
			regen, comp, hsize = int(v>>4)&0x3FF, int(v>>14)&0x3FF, 3
		case 2:
			if len(b) < 4 {
				return 0, 0, io.ErrUnexpectedEOF
			}
			v := binary.LittleEndian.Uint32(b)
			regen, comp, hsize = int(v>>4)&0x3FFF, int(v>>18)&0x3FFF, 4
		case 3:
			if len(b) < 5 {
				return 0, 0, io.ErrUnexpectedEOF
			}
			v := binary.LittleEndian.Uint32(b)
			regen, comp, hsize = int(v>>4)&0x3FFFF, int(v>>22)|int(b[4])<<10, 5
		}
		if hsize+comp > len(b) {
			return 0, 0, io.ErrUnexpectedEOF
		}
		if litType == 2 {
			tree := b[hsize : hsize+comp]
			n, err := zstdHuffmanDescriptionSize(tree)
			if err != nil {
				return 0, 0, err
			}
			s.huffman = append(s.huffman[:0], tree[:n]...)
		} else if s.huffman == nil {
			return 0, 0, fmt.Errorf("%w: treeless literals without a previous Huffman table", errZstdCorrupt)
		}
		return regen, hsize + comp, nil
	}
}

// zstdHuffmanDescriptionSize returns the size of the Huffman tree description
// at the start of b. RFC 8878 4.2.1.
func zstdHuffmanDescriptionSize(b []byte) (int, error) {
	if len(b) < 1 {
		return 0, io.ErrUnexpectedEOF
	}
	var n int
	if b[0] < 128 {
		n = 1 + int(b[0])
	} else {
		n = 1 + (int(b[0])-127+1)/2
	}
	if n > len(b) {
		return 0, io.ErrUnexpectedEOF
	}
	return n, nil
}

func bitsLen(n int) int {
	l := 0
	for n > 0 {
		l++
		n >>= 1
	}
	return l
}

// zstdBlockVisitor is called by walkZstdStream for every block, before the
// block is parsed. state is the state before the block, h and block are the
// block header and content. compOff is the offset of the block header in the
// compressed stream and uncompOff the offset of the block's data in the
// uncompressed stream.
type zstdBlockVisitor func(frame *zstdFrameHeader, compOff, uncompOff, framePos uint64, state *zstdBlockState, h zstdBlockHeader, block []byte) error

// walkZstdStream parses every frame and block of a zstd stream.
func walkZstdStream(r io.Reader, onBlock zstdBlockVisitor, onMatch func(zstdMatch)) (compressedSize, uncompressedSize uint64, err error) {
	br := bufio.NewReaderSize(r, 1<<20)
	var compOff, uncompOff uint64
	block := make([]byte, zstdMaxBlockSize)
	sawFrame := false
	for {
		peek, err := br.Peek(4)
		if errors.Is(err, io.EOF) && len(peek) == 0 {
			if !sawFrame {
				return 0, 0, errors.New("zstd stream has no data frames")
			}
			return compOff, uncompOff, nil
		}
		if err != nil {
			return 0, 0, fmt.Errorf("reading frame header: %w", err)
		}
		magic := binary.LittleEndian.Uint32(peek)
		if magic&zstdSkippableMask == zstdSkippableMagic {
			hdr := make([]byte, 8)
			if _, err := io.ReadFull(br, hdr); err != nil {
				return 0, 0, fmt.Errorf("reading skippable frame: %w", err)
			}
			n := int64(binary.LittleEndian.Uint32(hdr[4:]))
			if _, err := io.CopyN(io.Discard, br, n); err != nil {
				return 0, 0, fmt.Errorf("reading skippable frame: %w", err)
			}
			compOff += 8 + uint64(n)
			continue
		}
		peek, _ = br.Peek(18)
		frame, err := parseZstdFrameHeader(peek)
		if err != nil {
			return 0, 0, err
		}
		sawFrame = true
		br.Discard(frame.headerSize)
		compOff += uint64(frame.headerSize)
		state := newZstdBlockState()
		var framePos uint64
		for {
			var bh [zstdBlockHeaderSize]byte
			if _, err := io.ReadFull(br, bh[:]); err != nil {
				return 0, 0, fmt.Errorf("reading block header: %w", err)
			}
			h, err := parseZstdBlockHeader(bh[:])
			if err != nil {
				return 0, 0, err
			}
			if _, err := io.ReadFull(br, block[:h.size]); err != nil {
				return 0, 0, fmt.Errorf("reading block: %w", err)
			}
			if err := onBlock(&frame, compOff, uncompOff, framePos, &state, h, block[:h.size]); err != nil {
				return 0, 0, err
			}
			n := h.rawSize
			if h.blockType == zstdBlockCompressed {
				n, err = scanZstdBlock(block[:h.size], &state, framePos, onMatch)
				if err != nil {
					return 0, 0, fmt.Errorf("block at offset %d: %w", compOff, err)
				}
			}
			compOff += zstdBlockHeaderSize + uint64(h.size)
			uncompOff += uint64(n)
			framePos += uint64(n)
			if h.last {
				break
			}
		}
		if frame.hasChecksum {
			if _, err := br.Discard(zstdChecksumSize); err != nil {
				return 0, 0, fmt.Errorf("reading frame checksum: %w", err)
			}
			compOff += zstdChecksumSize
		}
	}
}
