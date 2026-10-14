// Package gzipinfo builds and uses a zran.c-style random-access index
// into a gzip stream, entirely in Go. It's a drop-in, cgo-free
// replacement for soci-snapshotter's earlier C implementation
// (ztoc/compression/gzip_zinfo.c, itself derived from zlib's zran.c
// example): checkpoints are taken at the same points (deflate block
// boundaries, every span uncompressed bytes) using the same fields, and
// MarshalBinary/UnmarshalBinary use the exact same tightly-packed
// little-endian byte layout, so a Go-built Index round-trips to a
// bit-for-bit identical blob and can read blobs the C code produced.
package gzipinfo

import (
	"encoding/binary"
	"fmt"

	"github.com/awslabs/soci-snapshotter/ztoc/compression/internal/gzipinfo/internal/inflate"
)

// WindowSize is the size of the LZ77 dictionary window saved with each
// checkpoint: 32768 bytes, the deflate format's maximum match offset.
const WindowSize = inflate.WindowSize

const (
	versionOne = 1
	versionTwo = 2

	// blobHeaderSize is 4 bytes (checkpoint count) + 8 bytes (span size).
	blobHeaderSize = 4 + 8
	// packedCheckpointSize is 8 (in) + 8 (out) + 1 (bits) + WindowSize.
	packedCheckpointSize = 8 + 8 + 1 + WindowSize
)

// Checkpoint is a single random-access point into a gzip stream: enough
// state to resume decompression at an arbitrary point without starting
// from the beginning of the file. Fields correspond field-for-field to
// gzip_zinfo.c's struct gzip_checkpoint.
type Checkpoint struct {
	// In is the absolute compressed byte offset (from the start of the
	// gzip stream, including container framing) at which this
	// checkpoint's deflate block boundary occurs.
	In int64
	// Out is the absolute uncompressed byte offset produced up to this
	// checkpoint.
	Out int64
	// Bits is the number of leftover bits (0-7) from the byte
	// immediately before In that have been read from the input but not
	// yet consumed by the decoder.
	Bits uint8
	// Window holds the last WindowSize bytes of uncompressed output
	// produced before this checkpoint (zero-padded at the front for
	// early checkpoints that haven't yet produced a full window's worth
	// of output), needed as the LZ77 dictionary to resume decoding here.
	Window [WindowSize]byte
}

// Index is a random-access index into a single gzip stream, chunking it
// into spans of approximately SpanSize() uncompressed bytes each (spans
// are rounded up to the next deflate block boundary, so they aren't
// exactly SpanSize - see BuildIndex), with a Checkpoint at the start of
// each span.
type Index struct {
	checkpoints      []Checkpoint
	spanSize         int64
	uncompressedSize int64
}

func (idx *Index) addCheckpoint(in, out int64, bits uint8, window [WindowSize]byte) {
	idx.checkpoints = append(idx.checkpoints, Checkpoint{In: in, Out: out, Bits: bits, Window: window})
}

// SpanSize returns the span size used to build this index.
func (idx *Index) SpanSize() int64 { return idx.spanSize }

// MaxSpanID returns the highest valid span ID.
func (idx *Index) MaxSpanID() int { return len(idx.checkpoints) - 1 }

// Checkpoint returns the checkpoint at the start of the given span.
func (idx *Index) Checkpoint(spanID int) Checkpoint { return idx.checkpoints[spanID] }

// MarshalBinary serializes idx into the same tightly-packed, little-endian
// byte layout as soci-snapshotter's earlier C implementation
// (gzip_zinfo.c's zinfo_to_blob, ZINFO_VERSION_TWO), so it can be stored
// in and read back from existing ztoc artifacts.
func (idx *Index) MarshalBinary() ([]byte, error) {
	n := len(idx.checkpoints)
	buf := make([]byte, blobHeaderSize+packedCheckpointSize*n)

	binary.LittleEndian.PutUint32(buf[0:4], uint32(n))
	binary.LittleEndian.PutUint64(buf[4:12], uint64(idx.spanSize))

	off := blobHeaderSize
	for _, cp := range idx.checkpoints {
		binary.LittleEndian.PutUint64(buf[off:off+8], uint64(cp.In))
		binary.LittleEndian.PutUint64(buf[off+8:off+16], uint64(cp.Out))
		buf[off+16] = cp.Bits
		copy(buf[off+17:off+17+WindowSize], cp.Window[:])
		off += packedCheckpointSize
	}
	return buf, nil
}

// UnmarshalBinary deserializes a checkpoints blob produced either by
// MarshalBinary or by soci-snapshotter's earlier C implementation
// (gzip_zinfo.c's zinfo_to_blob), including its ZINFO_VERSION_ONE
// backward-compatibility quirk where the first checkpoint was never
// stored and must be synthesized on read.
//
// UnmarshalBinary does not set the total uncompressed size (it isn't
// part of the blob); callers that need EndUncompressedOffset/
// EndCompressedOffset for the last span must call SetSize first.
func (idx *Index) UnmarshalBinary(data []byte) error {
	if len(data) < blobHeaderSize {
		return fmt.Errorf("gzip-info: blob too short: %d bytes", len(data))
	}
	have := int(binary.LittleEndian.Uint32(data[0:4]))
	spanSize := int64(binary.LittleEndian.Uint64(data[4:12]))
	body := data[blobHeaderSize:]

	var version int
	switch {
	case len(body) == packedCheckpointSize*have:
		// Also correctly matches have == 0 (an empty, checkpoint-less
		// index): body is empty too, 0 == 0.
		version = versionTwo
	case have > 0 && len(body) == packedCheckpointSize*(have-1):
		version = versionOne
	default:
		return fmt.Errorf("gzip-info: blob size %d doesn't match have=%d checkpoints", len(data), have)
	}

	checkpoints := make([]Checkpoint, have)
	firstStored := 0
	if version == versionOne {
		// v1 never stored checkpoint 0 (it assumed a fixed-size gzip
		// header); synthesize the same fixed values gzip_zinfo.c's
		// blob_to_zinfo does for backward compatibility.
		checkpoints[0] = Checkpoint{In: 10, Out: 0, Bits: 0}
		firstStored = 1
	}

	off := 0
	for i := firstStored; i < have; i++ {
		checkpoints[i].In = int64(binary.LittleEndian.Uint64(body[off : off+8]))
		checkpoints[i].Out = int64(binary.LittleEndian.Uint64(body[off+8 : off+16]))
		checkpoints[i].Bits = body[off+16]
		copy(checkpoints[i].Window[:], body[off+17:off+17+WindowSize])
		off += packedCheckpointSize
	}

	idx.spanSize = spanSize
	idx.checkpoints = checkpoints
	return nil
}

// SetSize records the total uncompressed and compressed sizes of the
// stream this index was built from, needed to answer
// EndUncompressedOffset/EndCompressedOffset for the last span. BuildIndex
// sets this automatically; callers using UnmarshalBinary must call it
// themselves if they need those two methods.
func (idx *Index) SetSize(uncompressedSize int64) {
	idx.uncompressedSize = uncompressedSize
}
