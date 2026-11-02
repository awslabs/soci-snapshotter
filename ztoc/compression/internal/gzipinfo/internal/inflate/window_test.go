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

package inflate

import (
	"bytes"
	"compress/gzip"
	mathrand "math/rand"
	"testing"
)

// TestWindowAndTotalOutAgainstGroundTruth directly checks the two additions
// this fork makes on top of upstream compress/flate: Window() and
// TotalOut(). At every block boundary it decodes to, it recomputes what the
// last WindowSize bytes of output *should* be by slicing the known source
// data directly (bypassing the decompressor entirely) and compares.
//
// This exists because TotalOut() previously undercounted relative to what
// Window() reflects: Step()'s returned byte count only reflects bytes the
// internal dictionary buffer has flushed to the caller (full buffer or end
// of stream), which lags behind the true output at block boundaries that
// don't happen to coincide with a flush - exactly the boundaries
// checkpointing needs. That bug only reproduced with specific content
// shapes (it needed a transition from incompressible to highly compressible
// data to surface, since that's what produces a block boundary that lands
// strictly between two flush points), so this sweeps several shapes rather
// than relying on one fixed corpus - a higher-level round-trip test
// (BuildIndex/Extract in gzipinfo_test.go) could and did pass with data
// shapes that didn't happen to trigger it.
func TestWindowAndTotalOutAgainstGroundTruth(t *testing.T) {
	const size = 512 * 1024

	shapes := map[string]func() []byte{
		"half-random-half-pattern": func() []byte { return seededBytes(1, size) },
		"all-random": func() []byte {
			out := make([]byte, size)
			mathrand.New(mathrand.NewSource(2)).Read(out)
			return out
		},
		"all-pattern": func() []byte {
			out := make([]byte, size)
			pattern := []byte("the quick brown fox jumps over the lazy dog; ")
			for i := range out {
				out[i] = pattern[i%len(pattern)]
			}
			return out
		},
	}

	for name, build := range shapes {
		t.Run(name, func(t *testing.T) {
			src := build()

			var buf bytes.Buffer
			gw := gzip.NewWriter(&buf)
			if _, err := gw.Write(src); err != nil {
				t.Fatalf("gzip write: %v", err)
			}
			if err := gw.Close(); err != nil {
				t.Fatalf("gzip close: %v", err)
			}
			compressed := buf.Bytes()

			// Skip the 10-byte gzip header (no extra flags set by
			// gzip.Writer by default) - this package only decodes raw
			// deflate, matching how gzipinfo's BuildIndex uses it.
			d := NewReader(bytes.NewReader(compressed[10:]))

			var totOut int64
			checked := 0
			for {
				_, err := d.Step()
				totOut = d.TotalOut()
				if err != nil {
					break
				}
				if !d.AtBoundary() {
					continue
				}

				var want [WindowSize]byte
				if totOut >= WindowSize {
					copy(want[:], src[totOut-WindowSize:totOut])
				} else {
					copy(want[WindowSize-totOut:], src[:totOut])
				}

				if got := d.Window(); got != want {
					idx := -1
					for i := range want {
						if got[i] != want[i] {
							idx = i
							break
						}
					}
					t.Fatalf("window mismatch at totOut=%d: first diff at window index %d: got=%#x want=%#x",
						totOut, idx, got[idx], want[idx])
				}
				checked++
			}

			if totOut != int64(len(src)) {
				t.Fatalf("final TotalOut() = %d, want %d (full source length)", totOut, len(src))
			}
			if checked == 0 {
				t.Fatal("no checkpoints observed - test isn't exercising AtBoundary()/Window() at all")
			}
			t.Logf("checked %d checkpoints, final totOut=%d", checked, totOut)
		})
	}
}

// seededBytes returns n bytes, reproducibly across runs: half pseudo-random
// (incompressible, forces varied block types, including stored blocks) and
// half a repeating pattern (compressible, exercises LZ77 back-references
// across the window) - the same shape gzipinfo_test.go uses, since it's
// this exact transition that originally triggered the TotalOut() bug.
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
