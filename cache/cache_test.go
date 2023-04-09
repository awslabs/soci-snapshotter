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

/*
   Copyright The containerd Authors.

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

/*
   Copyright 2019 The Go Authors. All rights reserved.
   Use of this source code is governed by a BSD-style
   license that can be found in the NOTICE.md file.
*/

package cache

import (
	"crypto/sha256"
	"fmt"
	"io"
	"testing"
)

const (
	sampleData = "0123456789"
)

func TestDirectoryCache(t *testing.T) {

	// with enough memory cache
	newCache := func(t *testing.T) BlobCache {
		tmp := t.TempDir()
		c, err := NewDirectoryCache(tmp, DirectoryCacheConfig{
			MaxLRUCacheEntry: 10,
			SyncAdd:          true,
		})
		if err != nil {
			t.Fatalf("failed to make cache: %v", err)
		}
		return c
	}
	testCache(t, "dir-with-enough-mem", newCache)

	// with smaller memory cache
	newCache = func(t *testing.T) BlobCache {
		tmp := t.TempDir()
		c, err := NewDirectoryCache(tmp, DirectoryCacheConfig{
			MaxLRUCacheEntry: 1,
			SyncAdd:          true,
		})
		if err != nil {
			t.Fatalf("failed to make cache: %v", err)
		}
		return c
	}
	testCache(t, "dir-with-small-mem", newCache)
}

func TestMemoryCache(t *testing.T) {
	testCache(t, "memory", func(*testing.T) BlobCache { return NewMemoryCache() })
}

func testCache(t *testing.T, name string, newCache func(t *testing.T) BlobCache) {
	tests := []struct {
		name   string
		blobs  []string
		checks []check
	}{
		{
			name: "empty_data",
			blobs: []string{
				"",
			},
			checks: []check{
				hit(""),
				miss(sampleData),
			},
		},
		{
			name: "data",
			blobs: []string{
				sampleData,
			},
			checks: []check{
				hit(sampleData),
				miss("dummy"),
			},
		},
		{
			name: "manydata",
			blobs: []string{
				sampleData,
				"test",
			},
			checks: []check{
				hit(sampleData),
				miss("dummy"),
			},
		},
		{
			name: "dup_data",
			blobs: []string{
				sampleData,
				sampleData,
			},
			checks: []check{
				hit(sampleData),
			},
		},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s-%s", name, tt.name), func(t *testing.T) {
			c := newCache(t)
			for _, blob := range tt.blobs {
				d := digestFor(blob)
				w, err := c.Add(d)
				if err != nil {
					t.Fatalf("failed to add %v: %v", d, err)
				}
				if n, err := w.Write([]byte(blob)); err != nil || n != len(blob) {
					w.Close()
					t.Fatalf("failed to write %v (len:%d): %v", d, len(blob), err)
				}
				if err := w.Commit(); err != nil {
					w.Close()
					t.Fatalf("failed to commit %v (len:%d): %v", d, len(blob), err)
				}
				w.Close()
			}
			for _, check := range tt.checks {
				check(t, c)
			}
		})
	}
}

type check func(*testing.T, BlobCache)

func digestFor(content string) string {
	sum := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", sum)
}

func hit(sample string) check {
	return func(t *testing.T, c BlobCache) {
		// test whole blob
		key := digestFor(sample)
		testBlob(t, c, key, 0, sample)

		// test a chunk
		chunk := len(sample) / 3
		testBlob(t, c, key, int64(chunk), sample[chunk:2*chunk])
	}
}

func testBlob(t *testing.T, c BlobCache, key string, offset int64, sample string) {
	p := make([]byte, len(sample))
	r, err := c.Get(key)
	if err != nil {
		t.Errorf("missed %v", key)
		return
	}
	if n, err := r.ReadAt(p, offset); err != nil && err != io.EOF {
		t.Errorf("failed to fetch blob %q: %v", key, err)
		return
	} else if n != len(sample) {
		t.Errorf("fetched size %d; want %d", len(p), len(sample))
		return
	}
	if digestFor(sample) != digestFor(string(p)) {
		t.Errorf("fetched %q; want %q", string(p), sample)
	}
}

func miss(sample string) check {
	return func(t *testing.T, c BlobCache) {
		d := digestFor(sample)
		if _, err := c.Get(d); err == nil {
			t.Errorf("hit blob %q but must be missed: %v", d, err)
			return
		}
	}
}
