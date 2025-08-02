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
	"sync"
	"testing"
	"time"

	"github.com/awslabs/soci-snapshotter/cache"
	spanmanager "github.com/awslabs/soci-snapshotter/fs/span-manager"
	"github.com/awslabs/soci-snapshotter/util/testutil"
	"github.com/awslabs/soci-snapshotter/ztoc"
	"github.com/opencontainers/go-digest"
)

func withPauser(p pauser) Option {
	return func(bf *BackgroundFetcher) error {
		bf.bfPauser = p
		return nil
	}
}

type countingPauser struct {
	mu    sync.Mutex
	count int
}

func (c *countingPauser) pause(time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.count++
}

func TestBackgroundFetcherPause(t *testing.T) {
	p := &countingPauser{}
	bf, err := NewBackgroundFetcher(WithSilencePeriod(0), withPauser(p), WithEmitMetricPeriod(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	go bf.Run(context.Background())
	defer bf.Close()
	bf.Pause()

	time.Sleep(10 * time.Millisecond)

	p.mu.Lock()
	defer p.mu.Unlock()
	if p.count != 1 {
		t.Fatalf("unexpected pause count; expected 1, got %v", p.count)
	}
}

func TestBackgroundFetcherRun(t *testing.T) {
	r := testutil.NewTestRand(t)
	testCases := []struct {
		name     string
		waitTime time.Duration
		entries  [][]testutil.TarEntry
	}{
		{
			name:     "background fetcher fetches all data for single span manager",
			waitTime: 1 * time.Second,
			entries: [][]testutil.TarEntry{
				{
					testutil.File("test", string(r.RandomByteData(10000000))),
				},
			},
		},
		{
			name:     "background fetcher fetches all data for multiple span managers",
			waitTime: 3 * time.Second,
			entries: [][]testutil.TarEntry{
				{
					testutil.File("test1", string(r.RandomByteData(10000000))),
				},
				{
					testutil.File("test2", string(r.RandomByteData(20000000))),
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			type testInfo struct {
				sm    *spanmanager.SpanManager
				cache *countingCache
				ztoc  *ztoc.Ztoc
			}

			var infos []testInfo
			for _, entries := range tc.entries {
				ztoc, sr, err := ztoc.BuildZtocReader(t, entries, gzip.DefaultCompression, 1000000)
				if err != nil {
					t.Fatalf("error building span manager and section reader: %v", err)
				}
				cache := &countingCache{}
				sm := spanmanager.New(ztoc, sr, cache, 0)
				infos = append(infos, testInfo{sm, cache, ztoc})
			}

			bf, err := NewBackgroundFetcher(WithFetchPeriod(0), WithEmitMetricPeriod(time.Second))
			if err != nil {
				t.Fatalf("unable to construct background fetcher: %v", err)
			}

			go bf.Run(context.Background())
			defer bf.Close()

			for _, info := range infos {
				bf.Add(NewSequentialResolver(digest.FromString("test"), info.sm))
			}

			time.Sleep(tc.waitTime)

			for _, info := range infos {
				info.cache.mu.Lock()
				defer info.cache.mu.Unlock()
				if info.cache.addCount != int(info.ztoc.MaxSpanID)+1 {
					t.Fatalf("unexpected number of adds to cache; expected %d, got %d", info.ztoc.MaxSpanID+1, info.cache.addCount)
				}

				// The first 10 bytes of a compressed gzip archive is the gzip header.
				// We don't fetch it when lazy-loading; therefore, subtracting 10 from the total compressed archive size.
				compressedSize := info.ztoc.CompressedArchiveSize - 10
				if info.cache.addBytes != int64(compressedSize) {
					t.Fatalf("unexpected number of bytes added to cache; expected %d, got %d", compressedSize, info.cache.addBytes)
				}
			}
		})
	}
}

// countingCache is an implementation of cache.BlobCache
// which counts the number of times `cache.Add` was invoked
// and the number of bytes added to the cache.
// All writes to the cache succeed.
type countingCache struct {
	addCount int
	addBytes int64
	mu       sync.Mutex
}

var _ cache.BlobCache = &countingCache{}

func (c *countingCache) Add(key string, opts ...cache.Option) (cache.Writer, error) {
	return &countingWriter{c}, nil
}

func (c *countingCache) Get(key string, opts ...cache.Option) (cache.Reader, error) {
	return nil, nil
}

func (c *countingCache) Close() error {
	return nil
}

type countingWriter struct {
	cache *countingCache
}

var _ cache.Writer = &countingWriter{}

func (c *countingWriter) Write(p []byte) (int, error) {
	c.cache.mu.Lock()
	defer c.cache.mu.Unlock()
	c.cache.addBytes += int64(len(p))
	c.cache.addCount++
	return len(p), nil
}

func (c *countingWriter) Close() error {
	return nil
}

func (c *countingWriter) Commit() error {
	return nil
}

func (c *countingWriter) Abort() error {
	return nil
}
