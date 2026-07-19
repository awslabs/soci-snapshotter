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

package purego

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/awslabs/soci-snapshotter/util/testutil"
	"github.com/awslabs/soci-snapshotter/ztoc/compression"
)

func TestSerializationRoundTrip(t *testing.T) {
	t.Parallel()
	// Build a small gzip file.
	tarGzReader := testutil.BuildTarGz(
		[]testutil.TarEntry{
			testutil.File("hello.txt", "Hello, World!"),
		},
		gzip.DefaultCompression,
	)

	tmpFile, _, err := testutil.WriteTarToTempFile("purego-test-*.tar.gz", tarGzReader)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile)

	z, err := NewGzipZinfoFromFile(tmpFile, 1024)
	if err != nil {
		t.Fatalf("NewGzipZinfoFromFile failed: %v", err)
	}

	blob, err := z.Bytes()
	if err != nil {
		t.Fatalf("Bytes() failed: %v", err)
	}

	z2, err := NewGzipZinfo(blob)
	if err != nil {
		t.Fatalf("NewGzipZinfo failed: %v", err)
	}

	if z.MaxSpanID() != z2.MaxSpanID() {
		t.Fatalf("MaxSpanID mismatch: %d vs %d", z.MaxSpanID(), z2.MaxSpanID())
	}
	if z.SpanSize() != z2.SpanSize() {
		t.Fatalf("SpanSize mismatch: %d vs %d", z.SpanSize(), z2.SpanSize())
	}

	// Verify checkpoint data matches.
	for i := range z.points {
		if z.points[i].In != z2.points[i].In {
			t.Fatalf("checkpoint %d In mismatch: %d vs %d", i, z.points[i].In, z2.points[i].In)
		}
		if z.points[i].Out != z2.points[i].Out {
			t.Fatalf("checkpoint %d Out mismatch: %d vs %d", i, z.points[i].Out, z2.points[i].Out)
		}
		if z.points[i].Bits != z2.points[i].Bits {
			t.Fatalf("checkpoint %d Bits mismatch: %d vs %d", i, z.points[i].Bits, z2.points[i].Bits)
		}
		if z.points[i].Window != z2.points[i].Window {
			t.Fatalf("checkpoint %d Window mismatch", i)
		}
	}
}

func TestBlobFormat(t *testing.T) {
	t.Parallel()
	// Test that serialization produces correct header.
	z := &GzipZinfo{
		version:  zinfoVersionCur,
		spanSize: 65536,
		points: []gzipCheckpoint{
			{In: 10, Out: 0, Bits: 0},
		},
	}

	blob, err := z.Bytes()
	if err != nil {
		t.Fatalf("Bytes() failed: %v", err)
	}

	// Check header.
	numCP := int32(binary.LittleEndian.Uint32(blob[0:4]))
	span := int64(binary.LittleEndian.Uint64(blob[4:12]))
	if numCP != 1 {
		t.Fatalf("expected 1 checkpoint in header, got %d", numCP)
	}
	if span != 65536 {
		t.Fatalf("expected span size 65536, got %d", span)
	}

	expectedSize := blobHeaderSize + packedCheckpointSize
	if len(blob) != expectedSize {
		t.Fatalf("expected blob size %d, got %d", expectedSize, len(blob))
	}
}

func TestIndexGeneration(t *testing.T) {
	t.Parallel()
	rng := testutil.NewTestRand(t)

	// Generate a file large enough for multiple spans.
	content := string(rng.RandomByteData(100000))
	tarGzReader := testutil.BuildTarGz(
		[]testutil.TarEntry{
			testutil.File("data.bin", content),
		},
		gzip.DefaultCompression,
	)

	tmpFile, _, err := testutil.WriteTarToTempFile("purego-gen-*.tar.gz", tarGzReader)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile)

	z, err := NewGzipZinfoFromFile(tmpFile, 32768)
	if err != nil {
		t.Fatalf("NewGzipZinfoFromFile failed: %v", err)
	}

	if z.MaxSpanID() < 0 {
		t.Fatal("expected at least one checkpoint")
	}

	// Verify first checkpoint.
	if z.points[0].Out != 0 {
		t.Fatalf("first checkpoint Out should be 0, got %d", z.points[0].Out)
	}

	// Verify checkpoints are ordered.
	for i := 1; i < len(z.points); i++ {
		if z.points[i].Out <= z.points[i-1].Out {
			t.Fatalf("checkpoints not in ascending order at index %d", i)
		}
	}
}

func TestExtraction(t *testing.T) {
	t.Parallel()
	// Create a tar.gz with known content and verify extraction.
	content := strings.Repeat("ABCDEFGH", 1000) // 8000 bytes
	tarGzReader := testutil.BuildTarGz(
		[]testutil.TarEntry{
			testutil.File("test.txt", content),
		},
		gzip.DefaultCompression,
	)

	tmpFile, gzData, err := testutil.WriteTarToTempFile("purego-extract-*.tar.gz", tarGzReader)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile)

	z, err := NewGzipZinfoFromFile(tmpFile, 4096)
	if err != nil {
		t.Fatalf("NewGzipZinfoFromFile failed: %v", err)
	}

	// Get the full uncompressed content for verification.
	f, err := os.Open(tmpFile)
	if err != nil {
		t.Fatal(err)
	}
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	fullUncompressed, err := io.ReadAll(gz)
	gz.Close()
	f.Close()
	if err != nil {
		t.Fatal(err)
	}

	// Test extraction from file for a range in the middle.
	if len(fullUncompressed) > 512 {
		offset := compression.Offset(512)
		size := compression.Offset(256)
		extracted, err := z.ExtractDataFromFile(tmpFile, size, offset)
		if err != nil {
			t.Fatalf("ExtractDataFromFile failed: %v", err)
		}
		expected := fullUncompressed[offset : offset+size]
		if !bytes.Equal(extracted, expected) {
			t.Fatalf("ExtractDataFromFile returned wrong data")
		}
	}

	// Test extraction from buffer.
	spanID := compression.SpanID(0)
	startComp := z.StartCompressedOffset(spanID)
	endComp := z.EndCompressedOffset(spanID, compression.Offset(len(gzData)))
	compBuf := gzData[startComp:endComp]

	startUncomp := z.StartUncompressedOffset(spanID)
	endUncomp := z.EndUncompressedOffset(spanID, compression.Offset(len(fullUncompressed)))
	size := endUncomp - startUncomp

	extracted, err := z.ExtractDataFromBuffer(compBuf, size, startUncomp, spanID)
	if err != nil {
		t.Fatalf("ExtractDataFromBuffer failed: %v", err)
	}
	expected := fullUncompressed[startUncomp:endUncomp]
	if !bytes.Equal(extracted[:len(expected)], expected) {
		t.Fatalf("ExtractDataFromBuffer returned wrong data for span 0")
	}
}

func TestVerifyHeader(t *testing.T) {
	t.Parallel()
	z := &GzipZinfo{}

	// Valid gzip.
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	gw.Write([]byte("test"))
	gw.Close()

	if err := z.VerifyHeader(bytes.NewReader(buf.Bytes())); err != nil {
		t.Fatalf("VerifyHeader should succeed on valid gzip: %v", err)
	}

	// Invalid data.
	if err := z.VerifyHeader(bytes.NewReader([]byte("not gzip"))); err == nil {
		t.Fatal("VerifyHeader should fail on invalid data")
	}
}

func TestGzipHeaderLen(t *testing.T) {
	t.Parallel()
	// Basic gzip header (no optional fields).
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	gw.Write([]byte("x"))
	gw.Close()

	hdrLen, err := gzipHeaderLen(buf.Bytes())
	if err != nil {
		t.Fatalf("gzipHeaderLen failed: %v", err)
	}
	if hdrLen != 10 {
		t.Fatalf("expected header length 10, got %d", hdrLen)
	}

	// Gzip with extra fields.
	buf.Reset()
	gw, _ = gzip.NewWriterLevel(&buf, gzip.DefaultCompression)
	gw.Extra = []byte("extra data")
	gw.Name = "test.txt"
	gw.Comment = "a comment"
	gw.Write([]byte("x"))
	gw.Close()

	hdrLen, err = gzipHeaderLen(buf.Bytes())
	if err != nil {
		t.Fatalf("gzipHeaderLen with extras failed: %v", err)
	}
	if hdrLen <= 10 {
		t.Fatalf("expected header length > 10 with extras, got %d", hdrLen)
	}
}
