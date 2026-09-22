package gzipinfo

import (
	"bufio"
	"bytes"
	"fmt"
	"io"

	"github.com/awslabs/soci-snapshotter/ztoc/compression/internal/gzipinfo/internal/inflate"
)

// byteReader is what the forked decompressor requires of its input:
// io.Reader plus io.ByteReader, satisfied directly by both *bufio.Reader
// and *bytes.Reader (so neither needs an extra internal buffering layer
// wrapped around it - see inflate.Reader's doc comment on why that
// matters for byte-accounting).
type byteReader interface {
	io.Reader
	io.ByteReader
}

// ExtractFromFile decompresses uncompressedSize bytes starting at
// uncompressedOffset from the gzip stream in f, seeking directly to the
// nearest checkpoint at or before uncompressedOffset instead of
// decompressing from the start of the file.
func (idx *Index) ExtractFromFile(f io.ReadSeeker, uncompressedOffset, uncompressedSize int64) ([]byte, error) {
	spanID := idx.UncompressedOffsetToSpanID(uncompressedOffset)
	cp := idx.checkpoints[spanID]

	if _, err := f.Seek(idx.StartCompressedOffset(spanID), io.SeekStart); err != nil {
		return nil, fmt.Errorf("gzip-info: seeking to checkpoint: %w", err)
	}

	return idx.extract(bufio.NewReader(f), cp, uncompressedOffset, uncompressedSize)
}

// ExtractFromBuffer decompresses uncompressedSize bytes starting at
// uncompressedOffset, using compressedBuf as the compressed bytes for
// checkpoint spanID's span. compressedBuf must start at
// idx.StartCompressedOffset(spanID) and extend at least through
// idx.EndCompressedOffset(spanID, fileSize). This mirrors
// gzip_zinfo.c's extract_data_from_buffer, used when the caller has
// already fetched just one span's compressed bytes (e.g. over the
// network via an HTTP range request) rather than having the whole file
// available.
func (idx *Index) ExtractFromBuffer(compressedBuf []byte, spanID int, uncompressedOffset, uncompressedSize int64) ([]byte, error) {
	if spanID < 0 || spanID > idx.MaxSpanID() {
		return nil, fmt.Errorf("gzip-info: span %d out of range (have %d checkpoints)", spanID, len(idx.checkpoints))
	}
	cp := idx.checkpoints[spanID]
	return idx.extract(bytes.NewReader(compressedBuf), cp, uncompressedOffset, uncompressedSize)
}

// extract is the shared core of ExtractFromFile/ExtractFromBuffer: r must
// be positioned so its next byte is the one at
// idx.StartCompressedOffset(spanID) for cp's span.
func (idx *Index) extract(r byteReader, cp Checkpoint, uncompressedOffset, uncompressedSize int64) ([]byte, error) {
	if uncompressedOffset < cp.Out {
		return nil, fmt.Errorf("gzip-info: checkpoint out=%d is after requested offset=%d", cp.Out, uncompressedOffset)
	}

	var bitBuf uint32
	if cp.Bits != 0 {
		b, err := r.ReadByte()
		if err != nil {
			return nil, fmt.Errorf("gzip-info: reading checkpoint priming byte: %w", err)
		}
		bitBuf = uint32(b) >> (8 - cp.Bits)
	}

	var d errReader = inflate.NewCheckpointedReader(r, cp.In, bitBuf, uint(cp.Bits), cp.Window[:])

	if toSkip := uncompressedOffset - cp.Out; toSkip > 0 {
		var err error
		d, err = copyAcrossMembers(d, r, nil, toSkip)
		if err != nil {
			return nil, fmt.Errorf("gzip-info: skipping to offset %d: %w", uncompressedOffset, err)
		}
	}

	out := make([]byte, uncompressedSize)
	if _, err := copyAcrossMembers(d, r, out, uncompressedSize); err != nil {
		return nil, fmt.Errorf("gzip-info: reading %d bytes at offset %d: %w", uncompressedSize, uncompressedOffset, err)
	}
	return out, nil
}

// errReader is what copyAcrossMembers needs beyond io.Reader: a way to
// tell a genuinely clean end of THIS member's deflate stream (Err()
// returns exactly io.EOF, unwrapped) apart from decoding failing or the
// underlying reader running out of bytes mid-block (Err() returns some
// other non-nil error, e.g. io.ErrUnexpectedEOF). io.ReadFull's own
// wrapping can't be used for this: it converts a clean io.EOF into
// io.ErrUnexpectedEOF too whenever a partial read preceded it, which
// would make the two conditions indistinguishable.
type errReader interface {
	io.Reader
	Err() error
}

// copyAcrossMembers copies exactly n bytes of decompressed output from
// the current member decompressor d into dst (or discards them if dst is
// nil), transparently continuing into subsequent concatenated gzip
// members read from r if d cleanly finishes before satisfying the
// request. It returns the decompressor to use for any subsequent call,
// since a member transition replaces d with a fresh one.
func copyAcrossMembers(d errReader, r byteReader, dst []byte, n int64) (errReader, error) {
	var scratch []byte
	if dst == nil {
		scratch = make([]byte, 32*1024)
	}

	var written int64
	for written < n {
		var buf []byte
		if dst != nil {
			buf = dst[written:]
		} else {
			want := n - written
			if want > int64(len(scratch)) {
				want = int64(len(scratch))
			}
			buf = scratch[:want]
		}

		nr, err := io.ReadFull(d, buf)
		written += int64(nr)
		if err == nil {
			continue
		}
		if written >= n {
			return d, nil
		}
		if d.Err() != io.EOF {
			return d, fmt.Errorf("decompressing (%d of %d bytes): %w", written, n, d.Err())
		}

		// d's member ended cleanly (a truly final deflate block was
		// decoded) before satisfying the request: cross into the next
		// concatenated gzip member, same as gzip_zinfo.c's
		// Z_STREAM_END handling in extract_data_from_fp /
		// extract_data_from_buffer.
		if _, err := skipGzipTrailer(r); err != nil {
			return d, fmt.Errorf("reading gzip trailer: %w", err)
		}
		if _, err := gzipHeaderSize(r); err != nil {
			return d, fmt.Errorf("expected another gzip member, found none: %w", err)
		}
		d = inflate.NewReader(r)
	}
	return d, nil
}
