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

package testutil

import (
	"hash/fnv"
	"math/rand/v2"
	"testing"

	"github.com/opencontainers/go-digest"
)

// Seed rand source
const TestRandomSeed = 1658503010463818386

// TestRand is a struct that wraps rand/v2 Rand with helper functions.
// It is instantiated with NewTestRand, which seeds it with TestRandomSeed
// and the name of the test it is being called from.
// Note TestRand is NOT thread-safe, and trying to have thread-safety
// as well as determinism doesn't really make sense anyway, so ensure all
// calls to the returned variable are only used within a single thread.
type TestRand struct {
	*rand.Rand
}

// NewSetSeedRand allows us to have deterministic tests by seeding the random var.
// It uses the test name as part of the seed, which allows better randomness across
// different tests, but also allows for deterministic results between runs.
func NewTestRand(t testing.TB) *TestRand {
	h := fnv.New64a()
	h.Write([]byte(t.Name()))

	// PCG is a little faster than ChaCha8, but the latter has slightly better randomness.
	// For the sake of testing it's probably better to just use the faster one.
	return &TestRand{
		rand.New(rand.NewPCG(TestRandomSeed, h.Sum64())),
	}
}

func (r *TestRand) Read(b []byte) {
	for i := range b {
		b[i] = byte(r.Int64())
	}
}

// RandomByteData returns a byte slice with `size` populated with random generated data
func (r *TestRand) RandomByteData(size int64) []byte {
	b := make([]byte, size)
	r.Read(b)
	return b
}

// RandomByteDataRange returns a byte slice with `size` between minBytes and maxBytes exclusive populated with random data
func (r *TestRand) RandomByteDataRange(minBytes int, maxBytes int) []byte {
	const charset = "abcdefghijklmnopqrstuvwxyz" +
		"ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789" + " "

	randByteNum := r.IntN(maxBytes-minBytes) + minBytes
	randBytes := make([]byte, randByteNum)
	for i := range randBytes {
		randBytes[i] = charset[r.IntN(len(charset))]
	}
	return randBytes
}

// RandomDigest generates a random digest from a random sequence of bytes
func (r *TestRand) RandomDigest() string {
	d := digest.FromBytes(r.RandomByteData(10))
	return d.String()
}
