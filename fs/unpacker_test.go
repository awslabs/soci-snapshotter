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

package fs

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"testing"

	"github.com/containerd/containerd/archive"
	"github.com/containerd/containerd/mount"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

func TestFailureModes(t *testing.T) {
	testCases := []struct {
		name         string
		mountpoint   string
		unpackedSize int64
		desc         ocispec.Descriptor
		applyFails   bool
		fetchFails   bool
		storeFails   bool
	}{
		{
			name:         "first fetch fails",
			mountpoint:   "/some/path/filename",
			unpackedSize: 65535,
			applyFails:   false,
			fetchFails:   true,
			storeFails:   false,
		},
		{
			name:         "store fails",
			mountpoint:   "/some/path/filename",
			unpackedSize: 65535,
			applyFails:   false,
			fetchFails:   false,
			storeFails:   true,
		},
		{
			name:         "apply fails",
			mountpoint:   "/some/path/filename",
			unpackedSize: 65535,
			applyFails:   true,
			fetchFails:   false,
			storeFails:   false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fetcher := newFakeFetcher(false, tc.storeFails, tc.fetchFails)
			archive := newFakeArchive(tc.unpackedSize, tc.applyFails)
			unpacker := NewLayerUnpacker(fetcher, archive)
			mounts := getFakeMounts()
			err := unpacker.Unpack(context.Background(), tc.desc, tc.mountpoint, mounts)
			if err == nil {
				t.Fatalf("%v: there should've been an error due to the following cases: fetch=%v, store=%v, apply=%v",
					tc.name, tc.fetchFails, tc.storeFails, tc.applyFails)
			}

			if tc.fetchFails && fetcher.fetchCount != 1 {
				t.Fatalf("%v: fetch must have been called once, but was called %d times", tc.name, fetcher.fetchCount)
			}
			if tc.storeFails && fetcher.storeCount != 1 {
				t.Fatalf("%v: store must have been called once, but was called %d times", tc.name, fetcher.storeCount)
			}
			if tc.applyFails && archive.applyCount != 1 {
				t.Fatalf("%v: apply must have been called once, but was called %d times", tc.name, archive.applyCount)
			}
		})
	}
}

func TestUnpackHappyPath(t *testing.T) {
	testCases := []struct {
		name         string
		mountpoint   string
		unpackedSize int64
		hasLocal     bool
		desc         ocispec.Descriptor
	}{
		{
			name:         "happy path layer exists locally",
			mountpoint:   "/some/path/filename",
			unpackedSize: 65535,
			hasLocal:     true,
		},
		{
			name:         "happy path layer does not exist locally",
			mountpoint:   "/some/path/filename",
			unpackedSize: 10000,
			hasLocal:     false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fetcher := newFakeFetcher(tc.hasLocal, false, false)
			archive := newFakeArchive(tc.unpackedSize, false)
			unpacker := NewLayerUnpacker(fetcher, archive)
			mounts := getFakeMounts()
			err := unpacker.Unpack(context.Background(), tc.desc, tc.mountpoint, mounts)
			if err != nil {
				t.Fatalf("%v: failed to unpack layer", tc.name)
			}
			if tc.hasLocal {
				if fetcher.storeCount != 0 {
					t.Fatalf("%v: Store was called on fetcher", tc.name)
				}
				if fetcher.fetchCount != 1 {
					t.Fatalf("%v: Fetch must be called only once if the layer exists locally, but was called %d times", tc.name, fetcher.fetchCount)
				}
			} else {
				if fetcher.storeCount != 1 {
					t.Fatalf("%v: Store must be called only once, but was called %d times", tc.name, fetcher.storeCount)
				}
				if fetcher.fetchCount != 2 {
					t.Fatalf("%v: Fetch must be called twice, but was called %d times", tc.name, fetcher.fetchCount)
				}
			}
			if archive.applyCount != 1 {
				t.Fatalf("%v: Apply() must be called only once, but was called %d times", tc.name, archive.applyCount)
			}
		})
	}
}

type fakeArtifactFetcher struct {
	storeFails bool
	fetchFails bool
	storeCount int64
	fetchCount int64
	hasLocal   bool
}

func newFakeFetcher(hasLocal, storeFails, fetchFails bool) *fakeArtifactFetcher {
	return &fakeArtifactFetcher{
		storeFails: storeFails,
		fetchFails: fetchFails,
		hasLocal:   hasLocal,
	}
}

func (f *fakeArtifactFetcher) Fetch(ctx context.Context, desc ocispec.Descriptor) (io.ReadCloser, bool, error) {
	f.fetchCount++
	if f.fetchFails {
		return nil, false, fmt.Errorf("dummy error on Fetch()")
	}
	return io.NopCloser(bytes.NewBuffer([]byte("test"))), f.hasLocal, nil
}

func (f *fakeArtifactFetcher) Store(ctx context.Context, desc ocispec.Descriptor, reader io.Reader) error {
	f.storeCount++
	if f.storeFails {
		return fmt.Errorf("dummy error on Store()")
	}
	f.hasLocal = true
	return nil
}

type fakeArchive struct {
	applyFails   bool
	unpackedSize int64
	applyCount   int64
}

func newFakeArchive(unpackedSize int64, applyFails bool) *fakeArchive {
	return &fakeArchive{
		applyFails:   applyFails,
		unpackedSize: unpackedSize,
	}
}

func (a *fakeArchive) Apply(ctx context.Context, root string, r io.Reader, opts ...archive.ApplyOpt) (int64, error) {
	a.applyCount++
	if a.applyFails {
		return 0, fmt.Errorf("dummy error on Apply()")
	}
	return a.unpackedSize, nil
}

func getFakeMounts() []mount.Mount {
	return []mount.Mount{
		{
			Type:   "overlay",
			Source: "overlay",
			Options: []string{
				"workdir=somedir1",
				"upperdir=somedir2",
				"lowerdir=somedir3:somedir4",
			},
		},
	}
}
