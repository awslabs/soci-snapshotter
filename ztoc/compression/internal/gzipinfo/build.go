package gzipinfo

import (
	"bufio"
	"fmt"
	"io"

	"github.com/awslabs/soci-snapshotter/ztoc/compression/internal/gzipinfo/internal/inflate"
)

// BuildIndex decompresses the gzip stream read from r exactly once,
// recording a checkpoint at the very start and then every time
// accumulated uncompressed output exceeds span bytes since the last
// checkpoint, so that Extract can later randomly access any offset
// without decompressing from the beginning.
//
// Checkpoints are only ever taken at deflate block boundaries (never
// mid-block, since resuming mid-block would also require saved Huffman
// table state that isn't captured here), using the same
// "totOut == 0 || totOut-last > span" trigger as zlib zran.c and
// soci-snapshotter's earlier C implementation of the same algorithm
// (gzip_zinfo.c) - so a Go-built Index is bit-for-bit interchangeable
// with one built by that C code. Concatenated gzip members (as produced
// by parallel gzip tools like pigz/mgzip) are handled transparently: the
// uncompressed/compressed byte counters run continuously across member
// boundaries.
func BuildIndex(r io.Reader, span int64) (*Index, error) {
	if span <= 0 {
		return nil, fmt.Errorf("gzip-info: span must be positive, got %d", span)
	}

	br := bufio.NewReader(r)
	idx := &Index{spanSize: span}

	var (
		containerOffset int64 // bytes consumed by gzip headers/trailers so far
		memberBaseOut   int64 // cumulative uncompressed bytes from all previous members
		haveCheckpoint  bool
		lastCheckpoint  int64
	)

	for {
		hdrSize, err := gzipHeaderSize(br)
		if err != nil {
			return nil, fmt.Errorf("gzip-info: reading gzip header at byte %d: %w", containerOffset, err)
		}
		containerOffset += hdrSize

		d := inflate.NewReader(br)

		// The very first checkpoint is always taken right after the
		// first member's header, before any deflate block has been
		// processed (totOut == 0): this matches zlib zran.c, which
		// always produces an initial access point at the start of the
		// stream, and guarantees the index is never empty.
		if !haveCheckpoint {
			idx.addCheckpoint(containerOffset, 0, 0, d.Window())
			haveCheckpoint = true
			lastCheckpoint = 0
		}

		var totOut int64
		for {
			_, stepErr := d.Step()
			// d.TotalOut() (not Step's returned byte count) is the
			// authoritative measure of how much this member has
			// produced: Step/Read only hand back bytes once the
			// internal dictionary buffer flushes (full, or end of
			// stream), which lags behind the true output at block
			// boundaries that don't happen to coincide with a flush -
			// exactly the boundaries checkpoints are taken at. Window()
			// is tracked independently of flush timing too, so it must
			// be measured against this same counter, not Step's return.
			totOut = memberBaseOut + d.TotalOut()

			if stepErr == io.EOF {
				containerOffset += d.InputOffset()
				break
			}
			if stepErr != nil {
				return nil, fmt.Errorf("gzip-info: decompressing at byte %d: %w", containerOffset+d.InputOffset(), stepErr)
			}

			if d.AtBoundary() && totOut-lastCheckpoint > span {
				idx.addCheckpoint(containerOffset+d.InputOffset(), totOut, uint8(d.NumBits()), d.Window())
				lastCheckpoint = totOut
			}
		}
		memberBaseOut = totOut

		if _, err := skipGzipTrailer(br); err != nil {
			return nil, fmt.Errorf("gzip-info: reading gzip trailer at byte %d: %w", containerOffset, err)
		}
		containerOffset += gzipTrailerSize

		if _, err := br.Peek(1); err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("gzip-info: checking for further gzip members: %w", err)
		}
		// More data follows: another concatenated gzip member (e.g.
		// produced by pigz/mgzip); loop around and parse its header.
	}

	idx.uncompressedSize = memberBaseOut
	return idx, nil
}
