// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Forked from Go stdlib compress/flate/dict_decoder.go.
// Added Window() method to export sliding window state for zinfo checkpoints.

package flate

const WindowSize = 32768

// dictDecoder implements the LZ77 sliding dictionary as used in decompression.
type dictDecoder struct {
	hist []byte // Sliding window history

	// Invariant: 0 <= rdPos <= wrPos <= len(hist)
	wrPos int  // Current output position in buffer
	rdPos int  // Have emitted hist[:rdPos] already
	full  bool // Has a full window length been written yet?

	totalWritten int64 // Total uncompressed bytes written (across all wraps)
}

func (dd *dictDecoder) init(size int, dict []byte) {
	*dd = dictDecoder{hist: dd.hist}

	if cap(dd.hist) < size {
		dd.hist = make([]byte, size)
	}
	dd.hist = dd.hist[:size]

	if len(dict) > len(dd.hist) {
		dict = dict[len(dict)-len(dd.hist):]
	}
	dd.wrPos = copy(dd.hist, dict)
	if dd.wrPos == len(dd.hist) {
		dd.wrPos = 0
		dd.full = true
	}
	dd.rdPos = dd.wrPos
}

func (dd *dictDecoder) histSize() int {
	if dd.full {
		return len(dd.hist)
	}
	return dd.wrPos
}

func (dd *dictDecoder) availRead() int {
	return dd.wrPos - dd.rdPos
}

func (dd *dictDecoder) availWrite() int {
	return len(dd.hist) - dd.wrPos
}

func (dd *dictDecoder) writeSlice() []byte {
	return dd.hist[dd.wrPos:]
}

func (dd *dictDecoder) writeMark(cnt int) {
	dd.wrPos += cnt
	dd.totalWritten += int64(cnt)
}

func (dd *dictDecoder) writeByte(c byte) {
	dd.hist[dd.wrPos] = c
	dd.wrPos++
	dd.totalWritten++
}

func (dd *dictDecoder) writeCopy(dist, length int) int {
	dstBase := dd.wrPos
	dstPos := dstBase
	srcPos := dstPos - dist
	endPos := dstPos + length
	if endPos > len(dd.hist) {
		endPos = len(dd.hist)
	}

	if srcPos < 0 {
		srcPos += len(dd.hist)
		dstPos += copy(dd.hist[dstPos:endPos], dd.hist[srcPos:])
		srcPos = 0
	}

	for dstPos < endPos {
		dstPos += copy(dd.hist[dstPos:endPos], dd.hist[srcPos:dstPos])
	}

	dd.wrPos = dstPos
	written := dstPos - dstBase
	dd.totalWritten += int64(written)
	return written
}

func (dd *dictDecoder) tryWriteCopy(dist, length int) int {
	dstPos := dd.wrPos
	endPos := dstPos + length
	if dstPos < dist || endPos > len(dd.hist) {
		return 0
	}
	dstBase := dstPos
	srcPos := dstPos - dist

	for dstPos < endPos {
		dstPos += copy(dd.hist[dstPos:endPos], dd.hist[srcPos:dstPos])
	}

	dd.wrPos = dstPos
	written := dstPos - dstBase
	dd.totalWritten += int64(written)
	return written
}

func (dd *dictDecoder) readFlush() []byte {
	toRead := dd.hist[dd.rdPos:dd.wrPos]
	dd.rdPos = dd.wrPos
	if dd.wrPos == len(dd.hist) {
		dd.wrPos, dd.rdPos = 0, 0
		dd.full = true
	}
	return toRead
}

// Window returns the current 32KB sliding window in decompression order,
// matching the layout produced by zlib's inflate (used for zinfo checkpoints).
// The C code captures the window as: window[WINSIZE-left:] + window[:WINSIZE-left]
// where left = strm.avail_out (remaining space in the output window buffer).
// Go's dictDecoder uses a circular buffer: hist with wrPos as the write cursor.
// When full, the oldest data starts at wrPos and wraps around.
func (dd *dictDecoder) Window() [WindowSize]byte {
	var win [WindowSize]byte
	if !dd.full {
		// Buffer hasn't wrapped yet. Data is hist[0:wrPos], preceded by zeros.
		// This matches C behavior: when less than WINSIZE has been output,
		// the window is zero-padded at the beginning.
		if dd.wrPos > 0 {
			copy(win[WindowSize-dd.wrPos:], dd.hist[:dd.wrPos])
		}
	} else {
		// Buffer has wrapped. wrPos is where next write goes = oldest data position.
		// Data in decompression order: hist[wrPos:] then hist[:wrPos]
		n := copy(win[:], dd.hist[dd.wrPos:])
		copy(win[n:], dd.hist[:dd.wrPos])
	}
	return win
}
