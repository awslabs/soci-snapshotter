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
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/klauspost/compress/zstd"
)

// compressibleData returns data that compresses like text: words picked from
// a small vocabulary, with some random bytes and long repeats mixed in, so
// that matches reach far back into the window.
func compressibleData(seed int64, size int) []byte {
	rng := rand.New(rand.NewSource(seed))
	words := strings.Fields("the quick brown fox jumps over lazy dog container image layer span zstd gzip window " +
		"checkpoint frame block literal sequence offset match length huffman table entropy snapshot")
	var b bytes.Buffer
	for b.Len() < size {
		switch n := rng.Intn(100); {
		case n < 2:
			raw := make([]byte, rng.Intn(4096))
			rng.Read(raw)
			b.Write(raw)
		case n < 5 && b.Len() > 0:
			// Copy an earlier part of the data, possibly from far back.
			start := rng.Intn(b.Len())
			end := min(b.Len(), start+rng.Intn(64<<10))
			b.Write(bytes.Clone(b.Bytes()[start:end]))
		default:
			b.WriteString(words[rng.Intn(len(words))])
			b.WriteByte(" \n"[rng.Intn(2)])
		}
	}
	return b.Bytes()[:size]
}

func zstdCompress(t *testing.T, data []byte, opts ...zstd.EOption) []byte {
	t.Helper()
	var b bytes.Buffer
	w, err := zstd.NewWriter(&b, opts...)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func skippableFrame(payload []byte) []byte {
	b := binary.LittleEndian.AppendUint32(nil, zstdSkippableMagic+3)
	b = binary.LittleEndian.AppendUint32(b, uint32(len(payload)))
	return append(b, payload...)
}

func writeTemp(t *testing.T, b []byte) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "layer.zst")
	if err := os.WriteFile(p, b, 0600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestZstdZinfo(t *testing.T) {
	t.Parallel()
	data := compressibleData(1, 3<<20)
	random := make([]byte, 1<<20)
	rand.New(rand.NewSource(2)).Read(random)
	encodeAll := func(opts ...zstd.EOption) []byte {
		enc, err := zstd.NewWriter(nil, opts...)
		if err != nil {
			t.Fatal(err)
		}
		defer enc.Close()
		return enc.EncodeAll(data, nil)
	}

	testCases := []struct {
		name       string
		compressed []byte
		expected   []byte
	}{
		{
			name:       "fastest",
			compressed: zstdCompress(t, data, zstd.WithEncoderLevel(zstd.SpeedFastest)),
			expected:   data,
		},
		{
			name:       "default",
			compressed: zstdCompress(t, data, zstd.WithEncoderLevel(zstd.SpeedDefault)),
			expected:   data,
		},
		{
			name:       "better compression with checksum",
			compressed: zstdCompress(t, data, zstd.WithEncoderLevel(zstd.SpeedBetterCompression), zstd.WithEncoderCRC(true)),
			expected:   data,
		},
		{
			name:       "best compression without checksum",
			compressed: zstdCompress(t, data, zstd.WithEncoderLevel(zstd.SpeedBestCompression), zstd.WithEncoderCRC(false)),
			expected:   data,
		},
		{
			name:       "small window",
			compressed: zstdCompress(t, data, zstd.WithWindowSize(1<<16)),
			expected:   data,
		},
		{
			name:       "single segment frame",
			compressed: encodeAll(zstd.WithSingleSegment(true)),
			expected:   data,
		},
		{
			name:       "incompressible",
			compressed: zstdCompress(t, random),
			expected:   random,
		},
		{
			name: "multiple frames and skippable frames",
			compressed: bytes.Join([][]byte{
				skippableFrame([]byte("metadata")),
				zstdCompress(t, data[:1<<20]),
				skippableFrame(nil),
				zstdCompress(t, data[1<<20:1<<20+100]),
				zstdCompress(t, data[1<<20+100:]),
				skippableFrame([]byte("trailer")),
			}, nil),
			expected: data,
		},
	}

	for _, tc := range testCases {
		for _, spanSize := range []int64{32 << 10, 300 << 10, 4 << 20} {
			t.Run(fmt.Sprintf("%s span size %d", tc.name, spanSize), func(t *testing.T) {
				t.Parallel()
				testZstdZinfo(t, tc.compressed, tc.expected, spanSize)
			})
		}
	}
}

func testZstdZinfo(t *testing.T, compressed, expected []byte, spanSize int64) {
	path := writeTemp(t, compressed)
	built, err := NewZinfoFromFile(Zstd, path, spanSize)
	if err != nil {
		t.Fatalf("cannot build zinfo: %v", err)
	}
	b, err := built.Bytes()
	if err != nil {
		t.Fatalf("cannot serialize zinfo: %v", err)
	}
	zinfo, err := NewZinfo(Zstd, b)
	if err != nil {
		t.Fatalf("cannot deserialize zinfo: %v", err)
	}
	defer zinfo.Close()

	if zinfo.MaxSpanID() != built.MaxSpanID() || zinfo.SpanSize() != Offset(spanSize) {
		t.Fatalf("deserialized zinfo differs: max span %d vs %d, span size %d", zinfo.MaxSpanID(), built.MaxSpanID(), zinfo.SpanSize())
	}
	// Checkpoints are at block boundaries, and blocks hold up to 128 KiB.
	wantSpans := int64(len(expected)) / max(spanSize, 128<<10) / 2
	if int64(zinfo.MaxSpanID()) < wantSpans {
		t.Fatalf("only %d spans for %d bytes with span size %d", zinfo.MaxSpanID()+1, len(expected), spanSize)
	}

	header := compressed[:zinfo.StartCompressedOffset(0)]
	if err := zinfo.VerifyHeader(bytes.NewReader(header)); err != nil {
		t.Fatalf("header verification failed: %v", err)
	}

	fileSize := Offset(len(compressed))
	uncompressedSize := Offset(len(expected))
	for id := SpanID(0); id <= zinfo.MaxSpanID(); id++ {
		cs, ce := zinfo.StartCompressedOffset(id), zinfo.EndCompressedOffset(id, fileSize)
		us, ue := zinfo.StartUncompressedOffset(id), zinfo.EndUncompressedOffset(id, uncompressedSize)
		if zinfo.UncompressedOffsetToSpanID(us) != id || zinfo.UncompressedOffsetToSpanID(ue-1) != id {
			t.Fatalf("span %d: offsets [%d, %d) map to spans %d and %d", id, us, ue,
				zinfo.UncompressedOffsetToSpanID(us), zinfo.UncompressedOffsetToSpanID(ue-1))
		}
		// Checkpoints are moved past blocks that reuse RLE tables, but
		// spans should stay close to the span size.
		if int64(ue-us) > spanSize+zstdMaxBlockSize {
			t.Fatalf("span %d holds %d bytes, span size is %d", id, ue-us, spanSize)
		}
		got, err := zinfo.ExtractDataFromBuffer(compressed[cs:ce], ue-us, us, id)
		if err != nil {
			t.Fatalf("span %d: cannot extract data: %v", id, err)
		}
		if !bytes.Equal(got, expected[us:ue]) {
			t.Fatalf("span %d: extracted data differs", id)
		}
	}

	rng := rand.New(rand.NewSource(spanSize))
	for i := 0; i < 20; i++ {
		off := Offset(rng.Int63n(int64(uncompressedSize)))
		size := min(Offset(rng.Int63n(2*spanSize)), uncompressedSize-off)
		got, err := zinfo.ExtractDataFromFile(path, size, off)
		if err != nil {
			t.Fatalf("cannot extract %d bytes at %d from file: %v", size, off, err)
		}
		if !bytes.Equal(got, expected[off:off+size]) {
			t.Fatalf("data extracted from file at %d differs", off)
		}
	}
}

func TestZstdZinfoHistoryIsSparse(t *testing.T) {
	t.Parallel()
	data := compressibleData(3, 8<<20)
	// Add random bytes, so that the history is small compared with the
	// compressed data, like in real layers.
	rng := rand.New(rand.NewSource(3))
	for range 1 << 20 {
		data[rng.Intn(len(data))] = byte(rng.Intn(256))
	}
	compressed := zstdCompress(t, data, zstd.WithWindowSize(8<<20))
	zinfo, err := NewZinfoFromFile(Zstd, writeTemp(t, compressed), 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	b, err := zinfo.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	// The checkpoints should keep a small fraction of the window bytes that
	// storing whole windows would take.
	var fullWindows int64
	for id := SpanID(1); id <= zinfo.MaxSpanID(); id++ {
		fullWindows += min(int64(zinfo.StartUncompressedOffset(id)), 8<<20)
	}
	if int64(len(b)) > fullWindows/10 {
		t.Fatalf("zinfo is %d bytes, whole windows would be %d bytes", len(b), fullWindows)
	}
}

func TestZstdZinfoCorruptInput(t *testing.T) {
	t.Parallel()
	data := compressibleData(4, 1<<20)
	compressed := zstdCompress(t, data)
	testCases := []struct {
		name  string
		input []byte
	}{
		{"empty", nil},
		{"not zstd", []byte("this is not a zstd stream")},
		{"truncated", compressed[:len(compressed)/2]},
		{"only skippable frames", skippableFrame([]byte("x"))},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if _, err := NewZinfoFromFile(Zstd, writeTemp(t, tc.input), 1<<16); err == nil {
				t.Fatal("expected an error")
			}
		})
	}

	corrupted := bytes.Clone(compressed)
	for i := 100; i < len(corrupted); i += 997 {
		corrupted[i] ^= 0x55
	}
	if _, err := NewZinfoFromFile(Zstd, writeTemp(t, corrupted), 1<<16); err == nil {
		t.Fatal("expected an error for corrupted data")
	}
}

func TestZstdZinfoUnsupportedFeatures(t *testing.T) {
	t.Parallel()
	data := compressibleData(6, 1<<20)
	rawBlock := func(b []byte) []byte {
		h := uint32(len(b))<<3 | 1 // raw, last block
		return append([]byte{byte(h), byte(h >> 8), byte(h >> 16)}, b...)
	}
	frameWithDict := binary.LittleEndian.AppendUint32(nil, zstdFrameMagic)
	frameWithDict = append(frameWithDict, 0x01, 0x00, 42) // 1-byte dictionary ID 42, 1 KiB window
	frameWithDict = append(frameWithDict, rawBlock([]byte("data"))...)
	// Every span references a whole copy of random data.
	random := make([]byte, 1<<20)
	rand.New(rand.NewSource(6)).Read(random)
	copies := bytes.Repeat(random, 8)

	testCases := []struct {
		name  string
		input []byte
	}{
		{"window larger than 8 MiB", zstdCompress(t, data, zstd.WithWindowSize(16<<20))},
		{"frame with a dictionary", frameWithDict},
		{"zinfo larger than the layer", zstdCompress(t, copies, zstd.WithWindowSize(8<<20))},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := NewZinfoFromFile(Zstd, writeTemp(t, tc.input), 1<<16)
			if !errors.Is(err, ErrUnsupportedZstdFeature) {
				t.Fatalf("expected ErrUnsupportedZstdFeature, got %v", err)
			}
		})
	}
}

func TestMergeZstdRanges(t *testing.T) {
	t.Parallel()
	ranges := [][2]int64{{30, 40}, {0, 10}, {12, 20}, {5, 8}, {100, 101}}
	got := mergeZstdRanges(slices.Clone(ranges), 2)
	want := [][2]int64{{0, 20}, {30, 40}, {100, 101}}
	if !slices.Equal(got, want) {
		t.Fatalf("gap 2: got %v, want %v", got, want)
	}
	got = mergeZstdRanges(slices.Clone(ranges), 60)
	want = [][2]int64{{0, 101}}
	if !slices.Equal(got, want) {
		t.Fatalf("gap 60: got %v, want %v", got, want)
	}
}

func TestZstdZinfoTinyWindow(t *testing.T) {
	t.Parallel()
	// A single-segment frame with a 7-byte window and raw blocks of 3 and 4
	// bytes. The checkpoint at the second block has the initial repeat
	// offsets 1, 4 and 8, larger than the window.
	stream := binary.LittleEndian.AppendUint32(nil, zstdFrameMagic)
	stream = append(stream, 0x20, 7) // single segment, 1-byte content size 7
	for _, b := range []string{"abc", "defg"} {
		h := uint32(len(b)) << 3
		if b == "defg" {
			h |= 1
		}
		stream = append(stream, byte(h), byte(h>>8), byte(h>>16))
		stream = append(stream, b...)
	}
	testZstdZinfo(t, stream, []byte("abcdefg"), 1)
}

func TestZstdZinfoTwoByteZeroSequences(t *testing.T) {
	t.Parallel()
	// A compressed block with raw literals "hello" and a sequence count of
	// zero written in the 2-byte form.
	block := []byte{5 << 3, 'h', 'e', 'l', 'l', 'o', 0x80, 0x00}
	h := uint32(len(block))<<3 | zstdBlockCompressed<<1 | 1
	stream := binary.LittleEndian.AppendUint32(nil, zstdFrameMagic)
	stream = append(stream, 0x00, 0x00) // no checksum, 1 KiB window
	stream = append(stream, byte(h), byte(h>>8), byte(h>>16))
	stream = append(stream, block...)
	testZstdZinfo(t, stream, []byte("hello"), 1<<16)
}

func TestNewZstdZinfo(t *testing.T) {
	t.Parallel()
	for _, b := range [][]byte{nil, {}, {0}, []byte("garbage that is not a flatbuffer")} {
		if _, err := NewZinfo(Zstd, b); err == nil {
			t.Fatalf("expected an error for %q", b)
		}
	}
}

func TestZstdZinfoRejectsInvalidCheckpoints(t *testing.T) {
	t.Parallel()
	data := compressibleData(7, 1<<20)
	built, err := NewZinfoFromFile(Zstd, writeTemp(t, zstdCompress(t, data)), 64<<10)
	if err != nil {
		t.Fatal(err)
	}
	valid := built.(*ZstdZinfo)
	if valid.MaxSpanID() < 2 {
		t.Fatalf("expected several spans, got %d", valid.MaxSpanID()+1)
	}

	testCases := []struct {
		name   string
		modify func(z *ZstdZinfo)
	}{
		{"huge window", func(z *ZstdZinfo) { z.checkpoints[1].windowSize = 1 << 62 }},
		{"negative offset", func(z *ZstdZinfo) { z.checkpoints[1].compressedOffset = -1 }},
		{"frame offset after checkpoint", func(z *ZstdZinfo) { z.checkpoints[1].frameOffset = z.checkpoints[1].uncompressedOffset + 1 }},
		{"offsets not increasing", func(z *ZstdZinfo) { z.checkpoints[2].uncompressedOffset = z.checkpoints[1].uncompressedOffset }},
		{"zero repeat offset", func(z *ZstdZinfo) { z.checkpoints[1].repeatOffsets[0] = 0 }},
		{"missing tables", func(z *ZstdZinfo) { z.checkpoints[1].llTable = nil }},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			z := *valid
			z.checkpoints = slices.Clone(valid.checkpoints)
			tc.modify(&z)
			b, err := z.Bytes()
			if err != nil {
				t.Fatal(err)
			}
			if _, err := NewZinfo(Zstd, b); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}

func TestZstdCheckpointInvalidHistory(t *testing.T) {
	t.Parallel()
	history := func(values ...uint64) []byte {
		var b []byte
		for _, v := range values {
			b = binary.AppendUvarint(b, v)
		}
		enc, err := zstdHistoryEncoder()
		if err != nil {
			t.Fatal(err)
		}
		return enc.EncodeAll(b, nil)
	}
	testCases := []struct {
		name    string
		history []byte
	}{
		{"range length overflows", history(100, 1, 10, 1<<64-5)},
		{"gap overflows", history(100, 1, 1<<64-1, 1)},
		{"range past the end", history(100, 1, 90, 20)},
		{"too many ranges", history(1<<20, 1<<19)},
		{"larger than the window", history(1<<30, 0)},
		{"not zstd", []byte("garbage")},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := zstdCheckpoint{windowSize: 1 << 20, history: tc.history}
			if _, err := c.dictionary(); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}

func TestFSEDistributionRoundTrip(t *testing.T) {
	t.Parallel()
	sparse := fseDistribution{accuracyLog: 6, counts: make([]int16, 20)}
	sparse.counts[3], sparse.counts[4], sparse.counts[19] = 30, -1, 33
	for i, d := range []fseDistribution{zstdPredefDist[0], zstdPredefDist[1], zstdPredefDist[2], sparse} {
		b := appendFSEDistribution(nil, d)
		got, n, err := readFSEDistribution(b, zstdMaxMLSymbol, zstdMaxLLLog)
		if err != nil {
			t.Fatalf("distribution %d: %v", i, err)
		}
		if n != len(b) {
			t.Fatalf("distribution %d: read %d of %d bytes", i, n, len(b))
		}
		if got.accuracyLog != d.accuracyLog || fmt.Sprint(got.counts) != fmt.Sprint(d.counts) {
			t.Fatalf("distribution %d: got %v, want %v", i, got, d)
		}
		if _, err := buildFSETable(got); err != nil {
			t.Fatalf("distribution %d: %v", i, err)
		}
	}
}

func TestZstdCheckpointDictionary(t *testing.T) {
	t.Parallel()
	s := newZstdBlockState()
	s.seq[zstdSeqLL] = zstdSeqTable{mode: zstdTableFSE, dist: zstdPredefDist[zstdSeqLL]}
	s.seq[zstdSeqOF] = zstdSeqTable{mode: zstdTablePredefined}
	s.seq[zstdSeqML] = zstdSeqTable{mode: zstdTableNone}
	c := newZstdCheckpoint(&zstdFrameHeader{windowSize: 1 << 20}, 100, 5000, 5000, &s)
	ring := newZstdRing(1 << 20)
	ring.Write(compressibleData(5, 5000))
	if err := c.setHistory(ring, [][2]int64{{10, 20}, {4000, 4990}}); err != nil {
		t.Fatal(err)
	}
	dict, err := c.dictionary()
	if err != nil {
		t.Fatal(err)
	}
	d, err := zstd.InspectDictionary(dict)
	if err != nil {
		t.Fatalf("decoder rejects the checkpoint dictionary: %v", err)
	}
	if d.ContentSize() != 4990 || d.Offsets() != [3]int{1, 4, 8} {
		t.Fatalf("unexpected dictionary: content size %d, offsets %v", d.ContentSize(), d.Offsets())
	}
	if !bytes.Equal(d.Content()[4000-10:4990-10], compressibleData(5, 5000)[4000:4990]) {
		t.Fatal("dictionary content differs")
	}
}
