package gzipinfo

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// RFC 1952 (gzip file format) constants.
const (
	gzipMagic1    = 0x1f
	gzipMagic2    = 0x8b
	gzipDeflateCM = 8

	gzipFlagText    = 1 << 0
	gzipFlagHCRC    = 1 << 1
	gzipFlagExtra   = 1 << 2
	gzipFlagName    = 1 << 3
	gzipFlagComment = 1 << 4

	// gzipTrailerSize is the 4-byte CRC32 + 4-byte ISIZE that follows
	// every member's deflate stream.
	gzipTrailerSize = 8
)

// ErrNotGzip is returned when a gzip member header's magic bytes don't
// match.
var ErrNotGzip = errors.New("gzip-info: not a gzip stream (bad magic bytes)")

// gzipHeaderSize reads and validates one gzip member header (RFC 1952
// section 2.3) from r, returning its size in bytes. Only the fields
// needed to find the end of the header are inspected; FNAME/FCOMMENT
// contents, FEXTRA subfields and FHCRC are skipped without validation,
// matching zlib's own permissive header parsing.
func gzipHeaderSize(r io.Reader) (int64, error) {
	var hdr [10]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return 0, err
	}
	if hdr[0] != gzipMagic1 || hdr[1] != gzipMagic2 {
		return 0, ErrNotGzip
	}
	if hdr[2] != gzipDeflateCM {
		return 0, fmt.Errorf("gzip-info: unsupported gzip compression method %d", hdr[2])
	}
	n := int64(len(hdr))
	flg := hdr[3]

	if flg&gzipFlagExtra != 0 {
		var xlenBuf [2]byte
		if _, err := io.ReadFull(r, xlenBuf[:]); err != nil {
			return 0, err
		}
		n += 2
		xlen := int64(binary.LittleEndian.Uint16(xlenBuf[:]))
		skipped, err := io.CopyN(io.Discard, r, xlen)
		n += skipped
		if err != nil {
			return 0, err
		}
	}
	for _, flag := range [...]byte{gzipFlagName, gzipFlagComment} {
		if flg&flag != 0 {
			consumed, err := skipCString(r)
			n += consumed
			if err != nil {
				return 0, err
			}
		}
	}
	if flg&gzipFlagHCRC != 0 {
		var crc [2]byte
		if _, err := io.ReadFull(r, crc[:]); err != nil {
			return 0, err
		}
		n += 2
	}
	return n, nil
}

// skipCString reads and discards a NUL-terminated string, returning the
// number of bytes consumed (including the terminating NUL).
func skipCString(r io.Reader) (int64, error) {
	var n int64
	var b [1]byte
	for {
		if _, err := io.ReadFull(r, b[:]); err != nil {
			return n, err
		}
		n++
		if b[0] == 0 {
			return n, nil
		}
	}
}

// skipGzipTrailer discards the 8-byte gzip trailer (CRC32 + ISIZE) that
// follows a member's deflate stream.
func skipGzipTrailer(r io.Reader) (int64, error) {
	return io.CopyN(io.Discard, r, gzipTrailerSize)
}
