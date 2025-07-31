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

package backgroundfetcher

import (
	"compress/gzip"
	"context"
	"testing"

	"github.com/awslabs/soci-snapshotter/cache"
	spanmanager "github.com/awslabs/soci-snapshotter/fs/span-manager"
	"github.com/awslabs/soci-snapshotter/util/testutil"
	"github.com/awslabs/soci-snapshotter/ztoc"
	"github.com/opencontainers/go-digest"
)

func TestSequentialResolver(t *testing.T) {
	r := testutil.NewTestRand(t)
	testCases := []struct {
		name    string
		entries []testutil.TarEntry
	}{
		{
			name: "resolver fetches spans sequentially",
			entries: []testutil.TarEntry{
				testutil.File("test", string(r.RandomByteData(10000000))),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ztoc, sr, err := ztoc.BuildZtocReader(t, tc.entries, gzip.DefaultCompression, 1000000)
			if err != nil {
				t.Fatalf("error build ztoc and section reader: %v", err)
			}
			sm := spanmanager.New(ztoc, sr, cache.NewMemoryCache(), 0)
			sequentialResolver := NewSequentialResolver(digest.FromString("test"), sm)

			var resolvedSpans []int
			for {
				resolvedSpans = append(resolvedSpans, int(sequentialResolver.(*sequentialLayerResolver).nextSpanFetchID))
				more, err := sequentialResolver.Resolve(context.Background())
				if !more {
					break
				}
				if err != nil {
					t.Fatalf("error while resolving span: %v", err)
				}
			}

			lastSpanID := sequentialResolver.(*sequentialLayerResolver).nextSpanFetchID
			// assert that we've resolved all spans
			if lastSpanID != ztoc.MaxSpanID+1 {
				t.Fatalf("unexpected number of spans resolved; expected %d, got %d", ztoc.MaxSpanID+1, lastSpanID)
			}

			// assert that all spans are resolved sequentially
			for i := 0; i < len(resolvedSpans); i++ {
				if i != resolvedSpans[i] {
					t.Fatalf("unexpected span id; expected %d, got %d", i, resolvedSpans[i])
				}
			}
		})
	}
}
