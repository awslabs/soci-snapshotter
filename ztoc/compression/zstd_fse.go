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
	"encoding/binary"
	"fmt"
	"math/bits"
)

// This file implements the parts of RFC 8878 needed to walk the sequences
// of a zstd block: reading and writing FSE table descriptions, building FSE
// decoding tables and reading the backward bitstream. Literals are never
// decoded here; the zstd zinfo only needs match offsets and lengths.

const (
	zstdMaxLLLog = 9
	zstdMaxMLLog = 9
	zstdMaxOFLog = 8

	zstdMaxLLSymbol = 35
	zstdMaxMLSymbol = 52
	zstdMaxOFSymbol = 31
)

// Predefined distributions. RFC 8878 3.1.1.3.2.2.
var (
	zstdPredefLL = []int16{4, 3, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 1, 1, 1,
		2, 2, 2, 2, 2, 2, 2, 2, 2, 3, 2, 1, 1, 1, 1, 1,
		-1, -1, -1, -1}
	zstdPredefML = []int16{1, 4, 3, 2, 2, 2, 2, 2, 2, 1, 1, 1, 1, 1, 1, 1,
		1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1,
		1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, -1, -1,
		-1, -1, -1, -1, -1}
	zstdPredefOF = []int16{1, 1, 1, 1, 1, 1, 2, 2, 2, 1, 1, 1, 1, 1, 1, 1,
		1, 1, 1, 1, 1, 1, 1, 1, -1, -1, -1, -1, -1}
)

const (
	zstdPredefLLLog = 6
	zstdPredefMLLog = 6
	zstdPredefOFLog = 5
)

// Literal length and match length codes. RFC 8878 3.1.1.3.2.1.
var zstdLLBase = [36]uint32{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15,
	16, 18, 20, 22, 24, 28, 32, 40, 48, 64, 128, 256, 512, 1024, 2048, 4096,
	8192, 16384, 32768, 65536}
var zstdLLBits = [36]uint8{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	1, 1, 1, 1, 2, 2, 3, 3, 4, 6, 7, 8, 9, 10, 11, 12,
	13, 14, 15, 16}
var zstdMLBase = [53]uint32{3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18,
	19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32, 33, 34,
	35, 37, 39, 41, 43, 47, 51, 59, 67, 83, 99, 131, 259, 515, 1027, 2051,
	4099, 8195, 16387, 32771, 65539}
var zstdMLBits = [53]uint8{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	1, 1, 1, 1, 2, 2, 3, 3, 4, 4, 5, 7, 8, 9, 10, 11,
	12, 13, 14, 15, 16}

// fseDistribution is a normalized FSE distribution: a count per symbol where
// -1 means "less than 1".
type fseDistribution struct {
	accuracyLog uint8
	counts      []int16
}

type fseState struct {
	symbol   uint8
	nbBits   uint8
	baseline uint16
}

// readFSEDistribution reads an FSE table description from the start of b.
// It returns the distribution and the number of bytes it used.
// RFC 8878 4.1.1.
func readFSEDistribution(b []byte, maxSymbol int, maxLog uint8) (fseDistribution, int, error) {
	br := forwardBitReader{b: b}
	al, err := br.read(4)
	if err != nil {
		return fseDistribution{}, 0, err
	}
	accuracyLog := uint8(al) + 5
	if accuracyLog > maxLog {
		return fseDistribution{}, 0, fmt.Errorf("%w: FSE accuracy log %d too large", errZstdCorrupt, accuracyLog)
	}
	remaining := int32(1<<accuracyLog) + 1
	threshold := int32(1 << accuracyLog)
	nbBits := uint(accuracyLog) + 1
	counts := make([]int16, 0, maxSymbol+1)
	for remaining > 1 {
		if len(counts) > maxSymbol {
			return fseDistribution{}, 0, fmt.Errorf("%w: too many FSE symbols", errZstdCorrupt)
		}
		limit := (2*threshold - 1) - remaining
		v := br.peek(nbBits)
		var count int32
		if int32(v)&(threshold-1) < limit {
			count = int32(v) & (threshold - 1)
			br.skip(nbBits - 1)
		} else {
			count = int32(v) & (2*threshold - 1)
			if count >= threshold {
				count -= limit
			}
			br.skip(nbBits)
		}
		count--
		if count < 0 {
			remaining += count
		} else {
			remaining -= count
		}
		if remaining < 1 {
			return fseDistribution{}, 0, fmt.Errorf("%w: FSE distribution overflows", errZstdCorrupt)
		}
		counts = append(counts, int16(count))
		if count == 0 {
			for {
				r, err := br.read(2)
				if err != nil {
					return fseDistribution{}, 0, err
				}
				for i := uint64(0); i < r; i++ {
					counts = append(counts, 0)
				}
				if r != 3 {
					break
				}
			}
			if len(counts) > maxSymbol+1 {
				return fseDistribution{}, 0, fmt.Errorf("%w: too many FSE symbols", errZstdCorrupt)
			}
		}
		for remaining < threshold {
			nbBits--
			threshold >>= 1
		}
	}
	if remaining != 1 {
		return fseDistribution{}, 0, fmt.Errorf("%w: FSE distribution does not add up", errZstdCorrupt)
	}
	return fseDistribution{accuracyLog: accuracyLog, counts: counts}, br.bytesUsed(), nil
}

// appendFSEDistribution appends the table description of d to b. It is the
// inverse of readFSEDistribution.
func appendFSEDistribution(b []byte, d fseDistribution) []byte {
	bw := forwardBitWriter{b: b}
	bw.write(uint64(d.accuracyLog-5), 4)
	remaining := int32(1<<d.accuracyLog) + 1
	threshold := int32(1 << d.accuracyLog)
	nbBits := uint(d.accuracyLog) + 1
	for i := 0; i < len(d.counts) && remaining > 1; i++ {
		count := int32(d.counts[i]) + 1
		limit := (2*threshold - 1) - remaining
		if count < limit {
			bw.write(uint64(count), nbBits-1)
		} else if count < threshold {
			bw.write(uint64(count), nbBits)
		} else {
			bw.write(uint64(count+limit), nbBits)
		}
		if d.counts[i] < 0 {
			remaining += int32(d.counts[i])
		} else {
			remaining -= int32(d.counts[i])
		}
		if d.counts[i] == 0 {
			zeros := 0
			for i+1+zeros < len(d.counts) && d.counts[i+1+zeros] == 0 {
				zeros++
			}
			i += zeros
			for zeros >= 3 {
				bw.write(3, 2)
				zeros -= 3
			}
			bw.write(uint64(zeros), 2)
		}
		for remaining < threshold {
			nbBits--
			threshold >>= 1
		}
	}
	return bw.flush()
}

// buildFSETable builds the decoding table of d. RFC 8878 4.1.1.
func buildFSETable(d fseDistribution) ([]fseState, error) {
	tableSize := 1 << d.accuracyLog
	table := make([]fseState, tableSize)
	next := make([]uint16, len(d.counts))
	high := tableSize - 1
	for s, c := range d.counts {
		if c == -1 {
			table[high].symbol = uint8(s)
			high--
			next[s] = 1
		} else {
			next[s] = uint16(c)
		}
	}
	step := (tableSize >> 1) + (tableSize >> 3) + 3
	mask := tableSize - 1
	pos := 0
	for s, c := range d.counts {
		for i := 0; i < int(c); i++ {
			table[pos].symbol = uint8(s)
			pos = (pos + step) & mask
			for pos > high {
				pos = (pos + step) & mask
			}
		}
	}
	if pos != 0 {
		return nil, fmt.Errorf("%w: invalid FSE distribution", errZstdCorrupt)
	}
	for i := range table {
		s := table[i].symbol
		n := next[s]
		next[s]++
		if n == 0 {
			return nil, fmt.Errorf("%w: invalid FSE distribution", errZstdCorrupt)
		}
		nb := d.accuracyLog - uint8(bits.Len16(n)-1)
		table[i].nbBits = nb
		table[i].baseline = uint16((uint32(n) << nb) - uint32(tableSize))
	}
	return table, nil
}

// forwardBitReader reads bits from the start of a byte slice, least
// significant bit first, as used by FSE table descriptions.
type forwardBitReader struct {
	b   []byte
	pos uint // in bits
}

func (r *forwardBitReader) peek(n uint) uint64 {
	var v uint64
	for i := uint(0); i < n; i++ {
		p := r.pos + i
		if p/8 >= uint(len(r.b)) {
			// Trailing bits past the end read as zero; callers check
			// bytesUsed against the input length.
			continue
		}
		v |= uint64((r.b[p/8]>>(p%8))&1) << i
	}
	return v
}

func (r *forwardBitReader) skip(n uint) { r.pos += n }

func (r *forwardBitReader) read(n uint) (uint64, error) {
	v := r.peek(n)
	r.skip(n)
	if r.pos > uint(len(r.b))*8 {
		return 0, fmt.Errorf("%w: truncated FSE table description", errZstdCorrupt)
	}
	return v, nil
}

func (r *forwardBitReader) bytesUsed() int { return int((r.pos + 7) / 8) }

type forwardBitWriter struct {
	b     []byte
	acc   uint64
	nbits uint
}

func (w *forwardBitWriter) write(v uint64, n uint) {
	w.acc |= (v & (1<<n - 1)) << w.nbits
	w.nbits += n
	for w.nbits >= 8 {
		w.b = append(w.b, byte(w.acc))
		w.acc >>= 8
		w.nbits -= 8
	}
}

func (w *forwardBitWriter) flush() []byte {
	if w.nbits > 0 {
		w.b = append(w.b, byte(w.acc))
	}
	return w.b
}

// backwardBitReader reads the bitstream of a sequences section, which is
// read from the end. RFC 8878 4.1.
type backwardBitReader struct {
	b   []byte
	pos int // bits left to read
}

func newBackwardBitReader(b []byte) (backwardBitReader, error) {
	if len(b) == 0 || b[len(b)-1] == 0 {
		return backwardBitReader{}, fmt.Errorf("%w: invalid bitstream padding", errZstdCorrupt)
	}
	return backwardBitReader{b: b, pos: len(b)*8 - (9 - bits.Len8(b[len(b)-1]))}, nil
}

// read returns the next n bits (n <= 56). Reading backward, the first bit
// read is the most significant, so the result is the little-endian value of
// the n bits below the current position.
func (r *backwardBitReader) read(n uint8) uint64 {
	if n == 0 {
		return 0
	}
	r.pos -= int(n)
	start := r.pos
	var v uint64
	if start >= 0 && start/8+8 <= len(r.b) {
		v = binary.LittleEndian.Uint64(r.b[start/8:]) >> (start % 8)
	} else {
		for i := 0; i < int(n); i++ {
			p := start + i
			if p >= 0 && p/8 < len(r.b) {
				v |= uint64(r.b[p/8]>>(p%8)&1) << i
			}
		}
	}
	return v & (1<<n - 1)
}

// overread reports whether more bits were read than the stream holds.
func (r *backwardBitReader) overread() bool { return r.pos < 0 }

func (r *backwardBitReader) done() bool { return r.pos == 0 }
