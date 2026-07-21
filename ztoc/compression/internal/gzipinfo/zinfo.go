package gzipinfo

import (
	"compress/gzip"
	"io"
)

// UncompressedOffsetToSpanID returns the ID of the span containing the
// given uncompressed offset (i.e. the largest span ID whose checkpoint's
// Out is <= offset). Equivalent to gzip_zinfo.c's
// pt_index_from_ucmp_offset, computed by binary search since checkpoints
// are stored in ascending Out order.
func (idx *Index) UncompressedOffsetToSpanID(offset int64) int {
	lo, hi := 0, len(idx.checkpoints)-1
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if idx.checkpoints[mid].Out <= offset {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	return lo
}

// StartCompressedOffset returns the offset, in the compressed stream, of
// the first byte belonging to spanID. When the checkpoint has leftover
// bits from the preceding byte, that byte is included (offset is one
// lower) since it's needed to resume decoding - this is the convention
// ExtractFromBuffer expects its buffer to start at.
func (idx *Index) StartCompressedOffset(spanID int) int64 {
	cp := idx.checkpoints[spanID]
	if cp.Bits != 0 {
		return cp.In - 1
	}
	return cp.In
}

// EndCompressedOffset returns the offset, in the compressed stream, one
// past the last byte belonging to spanID. If spanID is the last span,
// fileSize is returned.
func (idx *Index) EndCompressedOffset(spanID int, fileSize int64) int64 {
	if spanID == idx.MaxSpanID() {
		return fileSize
	}
	return idx.StartCompressedOffset(spanID + 1)
}

// StartUncompressedOffset returns the offset, in the uncompressed
// stream, of the first byte belonging to spanID.
func (idx *Index) StartUncompressedOffset(spanID int) int64 {
	return idx.checkpoints[spanID].Out
}

// EndUncompressedOffset returns the offset, in the uncompressed stream,
// one past the last byte belonging to spanID. If spanID is the last
// span, fileSize is returned.
func (idx *Index) EndUncompressedOffset(spanID int, fileSize int64) int64 {
	if spanID == idx.MaxSpanID() {
		return fileSize
	}
	return idx.checkpoints[spanID+1].Out
}

// VerifyHeader checks that r begins with a valid gzip header.
func (idx *Index) VerifyHeader(r io.Reader) error {
	gz, err := gzip.NewReader(r)
	if gz != nil {
		gz.Close()
	}
	return err
}
