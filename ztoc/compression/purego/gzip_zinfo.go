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

// Package purego provides a pure Go implementation of GzipZinfo that produces
// byte-for-byte identical output to the C (zlib-based) implementation.
package purego

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"fmt"
	"io"
	"os"

	"github.com/awslabs/soci-snapshotter/ztoc/compression"
	"github.com/awslabs/soci-snapshotter/ztoc/compression/flate"
)

const (
	winSize              = flate.WindowSize // 32768
	packedCheckpointSize = 8 + 8 + 1 + winSize
	blobHeaderSize       = 4 + 8

	zinfoVersionOne = 1
	zinfoVersionTwo = 2
	zinfoVersionCur = zinfoVersionTwo
)

// gzipCheckpoint stores the state needed to resume decompression at a
// deflate block boundary.
type gzipCheckpoint struct {
	In     int64            // offset in compressed file of first full byte
	Out    int64            // corresponding offset in uncompressed data
	Bits   uint8            // number of bits (1-7) from byte at In-1, or 0
	Window [winSize]byte    // preceding 32K of uncompressed data
}

// GzipZinfo is a pure Go equivalent of the C struct gzip_zinfo.
// It implements the compression.Zinfo interface.
type GzipZinfo struct {
	version  int32
	spanSize int64
	points   []gzipCheckpoint
}

// NewGzipZinfoFromFile creates a new GzipZinfo by reading a gzip file and
// building a seek index with checkpoints every spanSize uncompressed bytes.
func NewGzipZinfoFromFile(gzipFile string, spanSize int64) (*GzipZinfo, error) {
	f, err := os.Open(gzipFile)
	if err != nil {
		return nil, fmt.Errorf("could not open file: %w", err)
	}
	defer f.Close()
	return newGzipZinfoFromReader(f, spanSize)
}

// newGzipZinfoFromReader builds a seek index from an io.Reader containing
// gzip data.
func newGzipZinfoFromReader(r io.Reader, spanSize int64) (*GzipZinfo, error) {
	// Buffer the input so we can parse the gzip header and feed raw deflate
	// to our forked decompressor.
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		return nil, fmt.Errorf("could not read input: %w", err)
	}
	data := buf.Bytes()

	// Skip gzip header to find where the raw deflate stream starts.
	headerLen, err := gzipHeaderLen(data)
	if err != nil {
		return nil, fmt.Errorf("invalid gzip header: %w", err)
	}

	deflateData := data[headerLen:]
	deflateReader := bytes.NewReader(deflateData)

	// The C code (using zlib Z_BLOCK) creates the first checkpoint right
	// after the gzip header, before any deflate blocks are processed.
	// We replicate this by manually adding the initial checkpoint.
	var points []gzipCheckpoint
	points = append(points, gzipCheckpoint{
		In:  int64(headerLen),
		Out: 0,
	})
	var last int64 // totout at last checkpoint

	dec := flate.NewReader(deflateReader)

	// Track block boundaries via callback.
	dec.OnBlockEnd = func(final bool) {
		// Skip the last block — C code checks !(data_type & 64).
		if final {
			return
		}

		// Use DecompressedTotal for accurate count at callback time
		// (Read-based totalOut may lag behind).
		totout := dec.DecompressedTotal()

		if totout-last > spanSize {
			bitsCount, _ := dec.BitsState()
			byteOff := dec.ByteOffset()

			// C's totin includes the partial byte; Go's ByteOffset excludes it.
			// Adjust to match C convention.
			absIn := int64(headerLen) + byteOff
			if bitsCount > 0 {
				absIn++
			}

			cp := gzipCheckpoint{
				In:   absIn,
				Out:  totout,
				Bits: bitsCount,
			}
			cp.Window = dec.Window()
			points = append(points, cp)
			last = totout
		}
	}

	// Read all decompressed data to drive the decompressor through all blocks.
	outBuf := make([]byte, 32768)
	for {
		_, err := dec.Read(outBuf)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("decompression error: %w", err)
		}
	}

	return &GzipZinfo{
		version:  zinfoVersionCur,
		spanSize: spanSize,
		points:   points,
	}, nil
}

// NewGzipZinfo deserializes a GzipZinfo from its binary blob representation.
func NewGzipZinfo(zinfoBytes []byte) (*GzipZinfo, error) {
	if len(zinfoBytes) == 0 {
		return nil, fmt.Errorf("empty checkpoints")
	}
	if len(zinfoBytes) < blobHeaderSize {
		return nil, fmt.Errorf("cannot convert blob to gzip_zinfo")
	}

	numCheckpoints := int32(binary.LittleEndian.Uint32(zinfoBytes[0:4]))
	spanSize := int64(binary.LittleEndian.Uint64(zinfoBytes[4:12]))

	claimedSize := int64(packedCheckpointSize)*int64(numCheckpoints) + int64(blobHeaderSize)
	actualLen := int64(len(zinfoBytes))

	var version int32
	var firstCheckpointIndex int32

	if claimedSize == actualLen {
		version = zinfoVersionCur
	} else if claimedSize-packedCheckpointSize == actualLen {
		version = zinfoVersionOne
	} else {
		return nil, fmt.Errorf("cannot convert blob to gzip_zinfo")
	}

	points := make([]gzipCheckpoint, numCheckpoints)

	if version == zinfoVersionOne {
		firstCheckpointIndex = 1
		points[0] = gzipCheckpoint{
			In:   10,
			Out:  0,
			Bits: 0,
		}
	}

	cur := zinfoBytes[blobHeaderSize:]
	for i := firstCheckpointIndex; i < numCheckpoints; i++ {
		if len(cur) < packedCheckpointSize {
			return nil, fmt.Errorf("cannot convert blob to gzip_zinfo")
		}
		points[i].In = int64(binary.LittleEndian.Uint64(cur[0:8]))
		points[i].Out = int64(binary.LittleEndian.Uint64(cur[8:16]))
		points[i].Bits = cur[16]
		copy(points[i].Window[:], cur[17:17+winSize])
		cur = cur[packedCheckpointSize:]
	}

	return &GzipZinfo{
		version:  version,
		spanSize: spanSize,
		points:   points,
	}, nil
}

// Close is a no-op (no C memory to free).
func (z *GzipZinfo) Close() {}

// Bytes serializes the GzipZinfo to the same binary format as the C implementation.
func (z *GzipZinfo) Bytes() ([]byte, error) {
	firstIdx := int32(0)
	numSerialized := int32(len(z.points))
	if z.version == zinfoVersionOne {
		firstIdx = 1
		numSerialized = int32(len(z.points)) - 1
	}

	size := int64(blobHeaderSize) + int64(numSerialized)*int64(packedCheckpointSize)
	buf := make([]byte, size)
	if size == 0 {
		return nil, fmt.Errorf("could not allocate byte array of size %d", size)
	}

	binary.LittleEndian.PutUint32(buf[0:4], uint32(len(z.points)))
	binary.LittleEndian.PutUint64(buf[4:12], uint64(z.spanSize))

	cur := buf[blobHeaderSize:]
	for i := firstIdx; i < int32(len(z.points)); i++ {
		pt := &z.points[i]
		binary.LittleEndian.PutUint64(cur[0:8], uint64(pt.In))
		binary.LittleEndian.PutUint64(cur[8:16], uint64(pt.Out))
		cur[16] = pt.Bits
		copy(cur[17:17+winSize], pt.Window[:])
		cur = cur[packedCheckpointSize:]
	}

	return buf, nil
}

// MaxSpanID returns the maximum span ID (number of checkpoints - 1).
func (z *GzipZinfo) MaxSpanID() compression.SpanID {
	return compression.SpanID(len(z.points) - 1)
}

// SpanSize returns the span size used to build this index.
func (z *GzipZinfo) SpanSize() compression.Offset {
	return compression.Offset(z.spanSize)
}

// UncompressedOffsetToSpanID returns the ID of the span containing the given
// uncompressed offset.
func (z *GzipZinfo) UncompressedOffsetToSpanID(offset compression.Offset) compression.SpanID {
	res := 0
	for i := 1; i < len(z.points); i++ {
		if z.points[i].Out <= int64(offset) {
			res = i
		} else {
			break
		}
	}
	return compression.SpanID(res)
}

// ExtractDataFromBuffer decompresses data from a compressed buffer starting at
// the given checkpoint.
func (z *GzipZinfo) ExtractDataFromBuffer(compressedBuf []byte, uncompressedSize, uncompressedOffset compression.Offset, spanID compression.SpanID) ([]byte, error) {
	if len(compressedBuf) == 0 {
		return nil, fmt.Errorf("empty compressed buffer")
	}
	if uncompressedSize < 0 {
		return nil, fmt.Errorf("invalid uncompressed size: %d", uncompressedSize)
	}
	if uncompressedSize == 0 {
		return []byte{}, nil
	}

	cp := &z.points[spanID]
	data := compressedBuf

	// If checkpoint has bits, first byte contains partial bits.
	var bitsCount uint8
	var bitsVal byte
	if cp.Bits > 0 {
		fullByte := data[0]
		data = data[1:]
		// Reconstruct the bits value: the high bits of the byte are the ones
		// that were already consumed; the low (8-bits) bits are what we
		// need to feed to inflatePrime.
		// C code: inflatePrime(strm, bits, ret >> (8 - bits))
		bitsCount = cp.Bits
		bitsVal = fullByte >> (8 - cp.Bits)
	}

	reader := bytes.NewReader(data)
	dec := flate.NewReaderDict(reader, cp.Window[:])
	if bitsCount > 0 {
		dec.InjectBits(bitsCount, bitsVal)
	}

	// Skip bytes until we reach the desired offset within this span.
	skip := int64(uncompressedOffset) - cp.Out
	discard := make([]byte, 32768)
	for skip > 0 {
		toRead := min(int64(len(discard)), skip)
		n, err := dec.Read(discard[:toRead])
		skip -= int64(n)
		if err != nil {
			break
		}
	}

	// Read the requested data. The buffer may end at a deflate block boundary,
	// causing unexpected EOF when the decompressor tries to read the next
	// block header. This is normal — C's inflate stops when avail_out is
	// filled. We stop on any error once we have data.
	result := make([]byte, uncompressedSize)
	total := 0
	for total < int(uncompressedSize) {
		n, err := dec.Read(result[total:])
		total += n
		if err != nil {
			break
		}
	}
	if total == 0 {
		return result, fmt.Errorf("error extracting data; return code: 0")
	}

	return result, nil
}

// ExtractDataFromFile decompresses data from a gzip file.
func (z *GzipZinfo) ExtractDataFromFile(fileName string, uncompressedSize, uncompressedOffset compression.Offset) ([]byte, error) {
	if uncompressedSize < 0 {
		return nil, fmt.Errorf("invalid uncompressed size: %d", uncompressedSize)
	}
	if uncompressedSize == 0 {
		return []byte{}, nil
	}

	// Find the right checkpoint.
	spanID := z.UncompressedOffsetToSpanID(uncompressedOffset)
	cp := &z.points[spanID]

	f, err := os.Open(fileName)
	if err != nil {
		return nil, fmt.Errorf("could not open file: %w", err)
	}
	defer f.Close()

	// Seek to the compressed position.
	seekPos := cp.In
	if cp.Bits > 0 {
		seekPos--
	}
	if _, err := f.Seek(seekPos, io.SeekStart); err != nil {
		return nil, fmt.Errorf("seek failed: %w", err)
	}

	var bitsCount uint8
	var bitsVal byte
	if cp.Bits > 0 {
		var b [1]byte
		if _, err := io.ReadFull(f, b[:]); err != nil {
			return nil, fmt.Errorf("could not read bits byte: %w", err)
		}
		bitsCount = cp.Bits
		bitsVal = b[0] >> (8 - cp.Bits)
	}

	dec := flate.NewReaderDict(f, cp.Window[:])
	if bitsCount > 0 {
		dec.InjectBits(bitsCount, bitsVal)
	}

	// Skip to offset.
	skip := int64(uncompressedOffset) - cp.Out
	discard := make([]byte, 32768)
	for skip > 0 {
		toRead := min(int64(len(discard)), skip)
		n, err := dec.Read(discard[:toRead])
		skip -= int64(n)
		if err != nil {
			break
		}
	}

	// Read requested data.
	result := make([]byte, uncompressedSize)
	total := 0
	for total < int(uncompressedSize) {
		n, err := dec.Read(result[total:])
		total += n
		if err != nil {
			break
		}
	}
	if total == 0 {
		return nil, fmt.Errorf("unable to extract data; return code = 0")
	}

	return result, nil
}

// StartCompressedOffset returns the start offset of the span in the compressed stream.
func (z *GzipZinfo) StartCompressedOffset(spanID compression.SpanID) compression.Offset {
	start := z.points[spanID].In
	if z.points[spanID].Bits > 0 {
		start--
	}
	return compression.Offset(start)
}

// EndCompressedOffset returns the end offset of the span in the compressed stream.
func (z *GzipZinfo) EndCompressedOffset(spanID compression.SpanID, fileSize compression.Offset) compression.Offset {
	if spanID == z.MaxSpanID() {
		return fileSize
	}
	return compression.Offset(z.points[spanID+1].In)
}

// StartUncompressedOffset returns the start offset of the span in the uncompressed stream.
func (z *GzipZinfo) StartUncompressedOffset(spanID compression.SpanID) compression.Offset {
	return compression.Offset(z.points[spanID].Out)
}

// EndUncompressedOffset returns the end offset of the span in the uncompressed stream.
func (z *GzipZinfo) EndUncompressedOffset(spanID compression.SpanID, fileSize compression.Offset) compression.Offset {
	if spanID == z.MaxSpanID() {
		return fileSize
	}
	return compression.Offset(z.points[spanID+1].Out)
}

// VerifyHeader checks if the given reader contains a valid gzip header.
func (z *GzipZinfo) VerifyHeader(r io.Reader) error {
	gz, err := gzip.NewReader(r)
	if gz != nil {
		gz.Close()
	}
	return err
}

// gzipHeaderLen parses a gzip header and returns its length in bytes.
func gzipHeaderLen(data []byte) (int, error) {
	if len(data) < 10 {
		return 0, fmt.Errorf("data too short for gzip header")
	}
	if data[0] != 0x1f || data[1] != 0x8b {
		return 0, fmt.Errorf("invalid gzip magic number")
	}
	if data[2] != 8 {
		return 0, fmt.Errorf("unsupported compression method: %d", data[2])
	}

	flg := data[3]
	pos := 10 // Skip past the fixed 10-byte header

	// FEXTRA
	if flg&0x04 != 0 {
		if pos+2 > len(data) {
			return 0, fmt.Errorf("truncated gzip header (FEXTRA length)")
		}
		xlen := int(data[pos]) | int(data[pos+1])<<8
		pos += 2 + xlen
	}

	// FNAME
	if flg&0x08 != 0 {
		for pos < len(data) && data[pos] != 0 {
			pos++
		}
		pos++ // skip null terminator
	}

	// FCOMMENT
	if flg&0x10 != 0 {
		for pos < len(data) && data[pos] != 0 {
			pos++
		}
		pos++ // skip null terminator
	}

	// FHCRC
	if flg&0x02 != 0 {
		pos += 2
	}

	if pos > len(data) {
		return 0, fmt.Errorf("truncated gzip header")
	}

	return pos, nil
}

// Compile-time check that GzipZinfo implements compression.Zinfo.
var _ compression.Zinfo = (*GzipZinfo)(nil)
