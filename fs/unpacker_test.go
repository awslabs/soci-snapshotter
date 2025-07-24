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
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

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

func TestAsyncTeeReader(t *testing.T) {
	testcases := []struct {
		name     string
		src      io.Reader
		expected string
		experror bool
	}{
		{
			name:     "basic test",
			src:      strings.NewReader("hello world"),
			expected: "hello world",
			experror: false,
		},
		{
			name:     "empty input",
			src:      strings.NewReader(""),
			expected: "",
			experror: false,
		},
		{
			name:     "error reader",
			src:      &errorReader{},
			expected: "",
			experror: true,
		},
		{
			name:     "large input",
			src:      strings.NewReader(strings.Repeat("x", 1024*1024)),
			expected: strings.Repeat("x", 1024*1024),
			experror: false,
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			bp := newbufferPool(32)
			w := newTestWriter(0)
			r := AsyncTeeReader(tc.src, w, bp)

			out, err := io.ReadAll(r)
			if !tc.experror && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.experror && err == nil {
				t.Fatal("expected error, got nil")
			}
			if string(out) != tc.expected {
				t.Errorf("unexpected output: %q", out)
			}
			w.Wait()
			if w.Written() != len(tc.expected) {
				t.Errorf("tee writer did not receive correct data: %q", w.Written())
			}
			if w.String() != tc.expected {
				t.Errorf("tee writer output mismatch: got %q, want %q", w.String(), tc.expected)
			}
		})
	}
}

// Test that AsyncTeeReader propagates errors from the source reader.
type errorReader struct{}

func (e *errorReader) Read(p []byte) (int, error) {
	return 0, errors.New("test error")
}

type testWriter struct {
	buf   bytes.Buffer
	delay time.Duration
	ch    chan struct{}
}

func newTestWriter(delay time.Duration) *testWriter {
	return &testWriter{
		buf:   *bytes.NewBuffer(make([]byte, 0, 1024)),
		delay: delay,
		ch:    make(chan struct{}),
	}
}

func (sw *testWriter) Write(p []byte) (n int, err error) {
	if sw.delay > 0 {
		time.Sleep(sw.delay)
	}
	return sw.buf.Write(p)
}

func (sw *testWriter) Close() error {
	close(sw.ch)
	return nil
}

func (sw *testWriter) Wait() {
	<-sw.ch
}

func (sw *testWriter) Written() int {
	return sw.buf.Len()
}

func (sw *testWriter) String() string {
	return sw.buf.String()
}

func BenchmarkAsyncTeeReader(b *testing.B) {
	const dataSize = 1 * 1024 * 1024
	bf := newbufferPool(256)
	data := bytes.Repeat([]byte("x"), dataSize)
	for i := 0; i < b.N; i++ {
		src := bytes.NewReader(data)
		w := newTestWriter(1 * time.Millisecond)
		r := AsyncTeeReader(src, w, bf)
		for {
			n, err := io.CopyN(io.Discard, r, 256)
			if n > 0 {
				time.Sleep(2 * time.Millisecond)
			}
			if err != nil {
				break
			}
		}
		w.Wait()
	}
}

func BenchmarkTeeReader(b *testing.B) {
	const dataSize = 1 * 1024 * 1024
	data := bytes.Repeat([]byte("x"), dataSize)
	for i := 0; i < b.N; i++ {
		src := bytes.NewReader(data)
		w := newTestWriter(1 * time.Millisecond)
		r := io.TeeReader(src, w)
		for {
			n, err := io.CopyN(io.Discard, r, 256)
			if n > 0 {
				time.Sleep(2 * time.Millisecond)
			}
			if err != nil {
				break
			}
		}
		w.Close()
		w.Wait()
	}
}
