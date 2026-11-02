package gzipinfo

import (
	"bytes"
	"compress/gzip"
	mathrand "math/rand"
	"testing"
)

// buildTestGzip generates chunks*chunkSize bytes of test content and
// gzip-compresses it, flushing after every chunk so the stream contains
// many deflate block boundaries - giving BuildIndex plenty of
// opportunities to checkpoint even with a small span size.
func buildTestGzip(t *testing.T, chunks, chunkSize int) (compressed []byte, uncompressed []byte) {
	t.Helper()
	return gzipCompress(t, deterministicBytes(t, chunks*chunkSize), chunkSize)
}

// gzipCompress gzip-compresses src, flushing after every chunkSize bytes.
func gzipCompress(t *testing.T, src []byte, chunkSize int) (compressed []byte, uncompressed []byte) {
	t.Helper()

	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	for i := 0; i < len(src); i += chunkSize {
		end := i + chunkSize
		if end > len(src) {
			end = len(src)
		}
		if _, err := gw.Write(src[i:end]); err != nil {
			t.Fatalf("gzip write: %v", err)
		}
		if err := gw.Flush(); err != nil {
			t.Fatalf("gzip flush: %v", err)
		}
	}
	if err := gw.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}
	return buf.Bytes(), src
}

// deterministicBytes returns n bytes, reproducibly across runs: half
// pseudo-random (incompressible, forces varied block types, including
// stored blocks) and half a repeating pattern (compressible, exercises
// LZ77 back-references across the window).
func deterministicBytes(t *testing.T, n int) []byte {
	t.Helper()
	return seededBytes(1, n)
}

// seededBytes is deterministicBytes parameterized by seed, for tests that
// need several different content shapes rather than always the same one.
func seededBytes(seed int64, n int) []byte {
	out := make([]byte, n)
	half := n / 2
	mathrand.New(mathrand.NewSource(seed)).Read(out[:half])
	pattern := []byte("the quick brown fox jumps over the lazy dog; ")
	for i := half; i < n; i++ {
		out[i] = pattern[(i-half)%len(pattern)]
	}
	return out
}

func buildAndVerifyIndex(t *testing.T, compressed, uncompressed []byte, span int64) *Index {
	t.Helper()
	idx, err := BuildIndex(bytes.NewReader(compressed), span)
	if err != nil {
		t.Fatalf("BuildIndex: %v", err)
	}
	if idx.MaxSpanID() < 0 {
		t.Fatalf("index has no checkpoints")
	}
	if idx.checkpoints[0].Out != 0 || idx.checkpoints[0].Bits != 0 {
		t.Fatalf("first checkpoint should be {out:0, bits:0}, got %+v", idx.checkpoints[0])
	}
	if int64(len(uncompressed)) != idx.uncompressedSize {
		t.Fatalf("uncompressedSize = %d, want %d", idx.uncompressedSize, len(uncompressed))
	}
	return idx
}

func TestBuildIndexProducesMultipleCheckpointsForSmallSpan(t *testing.T) {
	compressed, uncompressed := buildTestGzip(t, 64, 512) // 32KiB uncompressed
	idx := buildAndVerifyIndex(t, compressed, uncompressed, 1024)

	if idx.MaxSpanID() == 0 {
		t.Fatalf("expected multiple spans for a small span size on 32KiB of input, got 1")
	}
}

func TestExtractFromFileRoundTrip(t *testing.T) {
	compressed, uncompressed := buildTestGzip(t, 64, 512)
	idx := buildAndVerifyIndex(t, compressed, uncompressed, 1024)

	cases := []struct{ offset, length int64 }{
		{0, 1},
		{0, int64(len(uncompressed))},
		{100, 50},
		{1023, 2}, // straddles a likely span boundary
		{1024, 1024},
		{int64(len(uncompressed)) - 10, 10},
	}

	for _, c := range cases {
		got, err := idx.ExtractFromFile(bytes.NewReader(compressed), c.offset, c.length)
		if err != nil {
			t.Fatalf("ExtractFromFile(offset=%d, length=%d): %v", c.offset, c.length, err)
		}
		want := uncompressed[c.offset : c.offset+c.length]
		if !bytes.Equal(got, want) {
			t.Fatalf("ExtractFromFile(offset=%d, length=%d) mismatch", c.offset, c.length)
		}
	}
}

func TestExtractFromBufferRoundTrip(t *testing.T) {
	compressed, uncompressed := buildTestGzip(t, 64, 512)
	idx := buildAndVerifyIndex(t, compressed, uncompressed, 1024)

	fileSize := int64(len(compressed))
	// ExtractFromBuffer's contract is that compressedBuf holds exactly
	// one span's compressed bytes, so requests must stay within that
	// span's uncompressed bounds (its actual size varies - checkpoints
	// land on the next block boundary at or after SpanSize, not exactly
	// at it - see BuildIndex). Requesting more than that is the caller's
	// job to split across multiple ExtractFromBuffer calls, one per
	// span, which is exercised by looping over every span below.
	offsets := []int64{0, 100, int64(len(uncompressed)) - 10}

	for _, offset := range offsets {
		spanID := idx.UncompressedOffsetToSpanID(offset)
		spanEnd := idx.EndUncompressedOffset(spanID, int64(len(uncompressed)))
		length := spanEnd - offset
		if length > 50 {
			length = 50
		}

		start := idx.StartCompressedOffset(spanID)
		end := idx.EndCompressedOffset(spanID, fileSize)
		buf := compressed[start:end]

		got, err := idx.ExtractFromBuffer(buf, spanID, offset, length)
		if err != nil {
			t.Fatalf("ExtractFromBuffer(offset=%d, length=%d): %v", offset, length, err)
		}
		want := uncompressed[offset : offset+length]
		if !bytes.Equal(got, want) {
			t.Fatalf("ExtractFromBuffer(offset=%d, length=%d) mismatch", offset, length)
		}
	}

	// Every span, read in full via ExtractFromBuffer, concatenated,
	// must reconstruct the whole file.
	var rebuilt []byte
	for spanID := 0; spanID <= idx.MaxSpanID(); spanID++ {
		start := idx.StartCompressedOffset(spanID)
		end := idx.EndCompressedOffset(spanID, fileSize)
		spanStart := idx.StartUncompressedOffset(spanID)
		spanEnd := idx.EndUncompressedOffset(spanID, int64(len(uncompressed)))

		got, err := idx.ExtractFromBuffer(compressed[start:end], spanID, spanStart, spanEnd-spanStart)
		if err != nil {
			t.Fatalf("ExtractFromBuffer(full span %d): %v", spanID, err)
		}
		rebuilt = append(rebuilt, got...)
	}
	if !bytes.Equal(rebuilt, uncompressed) {
		t.Fatalf("reconstructing the whole file span-by-span via ExtractFromBuffer mismatched")
	}
}

func TestMarshalUnmarshalRoundTrip(t *testing.T) {
	compressed, uncompressed := buildTestGzip(t, 64, 512)
	idx := buildAndVerifyIndex(t, compressed, uncompressed, 1024)

	blob, err := idx.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary: %v", err)
	}

	var idx2 Index
	if err := idx2.UnmarshalBinary(blob); err != nil {
		t.Fatalf("UnmarshalBinary: %v", err)
	}
	idx2.SetSize(idx.uncompressedSize)

	blob2, err := idx2.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary (round 2): %v", err)
	}
	if !bytes.Equal(blob, blob2) {
		t.Fatalf("marshal->unmarshal->marshal did not round-trip byte-for-byte")
	}

	// Extraction must work identically after a marshal round-trip.
	got, err := idx2.ExtractFromFile(bytes.NewReader(compressed), 100, 50)
	if err != nil {
		t.Fatalf("ExtractFromFile after round-trip: %v", err)
	}
	if !bytes.Equal(got, uncompressed[100:150]) {
		t.Fatalf("ExtractFromFile after round-trip mismatch")
	}
}

func TestMultiMemberGzip(t *testing.T) {
	part1 := deterministicBytes(t, 16*512) // 8KiB
	part2 := deterministicBytes(t, 16*512)
	c1, _ := gzipCompress(t, part1, 512)
	c2, _ := gzipCompress(t, part2, 512)

	compressed := append(append([]byte{}, c1...), c2...)
	uncompressed := append(append([]byte{}, part1...), part2...)

	idx := buildAndVerifyIndex(t, compressed, uncompressed, 1024)

	// Extract a range that straddles the member boundary (8KiB in).
	offset := int64(len(part1)) - 100
	length := int64(200)
	got, err := idx.ExtractFromFile(bytes.NewReader(compressed), offset, length)
	if err != nil {
		t.Fatalf("ExtractFromFile across member boundary: %v", err)
	}
	want := uncompressed[offset : offset+length]
	if !bytes.Equal(got, want) {
		t.Fatalf("extraction across gzip member boundary mismatch")
	}
}

// TestExtractFromFileRandomRanges exercises a larger, naturally-written
// gzip stream (no forced Flush - so deflate block sizes are whatever the
// compressor picked, not artificially small) with many random
// offset/length pairs, including ranges that span several checkpoints.
// This is the realistic shape of a compressed OCI layer.
func TestExtractFromFileRandomRanges(t *testing.T) {
	// Sweep several distinct content shapes (all-random/incompressible,
	// all-repetitive, and the half/half mix), each with its own seed, so
	// a bug that only manifests for certain block-type mixes (as one
	// previously did: totOut bookkeeping that lagged behind the true
	// dictionary window state, only visible once output crossed from an
	// incompressible to a highly compressible region) doesn't hide
	// behind a single fixed corpus.
	contentBuilders := map[string]func(n int) []byte{
		"half-random-half-pattern": func(n int) []byte { return seededBytes(1, n) },
		"all-random": func(n int) []byte {
			out := make([]byte, n)
			mathrand.New(mathrand.NewSource(2)).Read(out)
			return out
		},
		"all-pattern": func(n int) []byte {
			out := make([]byte, n)
			pattern := []byte("the quick brown fox jumps over the lazy dog; ")
			for i := range out {
				out[i] = pattern[i%len(pattern)]
			}
			return out
		},
	}

	for name, build := range contentBuilders {
		t.Run(name, func(t *testing.T) {
			src := build(512 * 1024) // 512KiB

			var buf bytes.Buffer
			gw := gzip.NewWriter(&buf)
			if _, err := gw.Write(src); err != nil {
				t.Fatalf("gzip write: %v", err)
			}
			if err := gw.Close(); err != nil {
				t.Fatalf("gzip close: %v", err)
			}
			compressed := buf.Bytes()

			idx := buildAndVerifyIndex(t, compressed, src, 8192)
			// Span count is incidental, not a correctness requirement:
			// highly repetitive content can legitimately compress into
			// very few, very large deflate blocks (checkpoints only
			// land on block boundaries), so don't assert a minimum
			// here - just log it, and let the extraction checks below
			// do the real work.
			t.Logf("%s: %d spans", name, idx.MaxSpanID()+1)

			rng := mathrand.New(mathrand.NewSource(42))
			for i := 0; i < 200; i++ {
				offset := rng.Int63n(int64(len(src)))
				maxLen := int64(len(src)) - offset
				length := rng.Int63n(maxLen) + 1

				got, err := idx.ExtractFromFile(bytes.NewReader(compressed), offset, length)
				if err != nil {
					t.Fatalf("ExtractFromFile(offset=%d, length=%d): %v", offset, length, err)
				}
				want := src[offset : offset+length]
				if !bytes.Equal(got, want) {
					t.Fatalf("ExtractFromFile(offset=%d, length=%d) mismatch", offset, length)
				}
			}
		})
	}
}

func TestUnmarshalBinaryEdgeCases(t *testing.T) {
	cases := []struct {
		name        string
		data        []byte
		expectError bool
	}{
		{"nil", nil, true},
		{"empty", []byte{}, true},
		{"shorter than header", []byte{0x00}, true},
		{
			name: "claims too many checkpoints with no data",
			data: []byte{
				0xFF, 0x00, 0x00, 0x00, // 255 checkpoints
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // span size 0
			},
			expectError: true,
		},
		{
			name: "zero checkpoints is valid",
			data: []byte{
				0x00, 0x00, 0x00, 0x00, // 0 checkpoints
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // span size 0
			},
			expectError: false,
		},
		{
			name: "v1 blob with synthesized first checkpoint",
			data: []byte{
				0x01, 0x00, 0x00, 0x00, // 1 checkpoint (v1: not stored)
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // span size 0
			},
			expectError: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var idx Index
			err := idx.UnmarshalBinary(tc.data)
			if tc.expectError != (err != nil) {
				t.Fatalf("expect error: %t, actual error: %v", tc.expectError, err)
			}
		})
	}
}
