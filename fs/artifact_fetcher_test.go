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

	"github.com/containerd/containerd/reference"
	"github.com/google/go-cmp/cmp"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/content/memory"
)

const imageRef = "dummy.host/repo:tag"

func TestConstructRef(t *testing.T) {

	testCases := []struct {
		name           string
		artifactDigest string
	}{
		{
			name:           "constructRef returns correct ref",
			artifactDigest: "sha256:7b236f6c6ca259a4497e98c204bc1dcf3e653438e74af17bfe39da5329789f4a",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fetcher, err := newFakeArtifactFetcher(imageRef, nil)
			if err != nil {
				t.Fatalf("could not create artifact fetcher: %v", err)
			}
			expectedRef := fmt.Sprintf("dummy.host/repo@%s", tc.artifactDigest)
			dgst, err := digest.Parse(tc.artifactDigest)
			if err != nil {
				t.Fatal(err)
			}
			constructedRef := fetcher.constructRef(ocispec.Descriptor{Digest: dgst})
			if expectedRef != constructedRef {
				t.Fatalf("unexpected ref from constructRef, got = %s, expected = %s", constructedRef, expectedRef)
			}
		})
	}
}

func TestArtifactFetcherFetch(t *testing.T) {

	testCases := []struct {
		name     string
		contents []byte
		size     int64
	}{
		{
			name:     "correct data fetched",
			contents: []byte("test"),
			size:     4,
		},
		{
			name:     "correct data fetched when desc.Size = 0",
			contents: []byte("test"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fetcher, err := newFakeArtifactFetcher(imageRef, tc.contents)
			if err != nil {
				t.Fatalf("could not create artifact fetcher: %v", err)
			}
			dgst := digest.FromBytes(tc.contents)
			desc := ocispec.Descriptor{
				Digest: dgst,
				Size:   tc.size,
			}

			reader, _, err := fetcher.Fetch(context.Background(), desc)
			if err != nil {
				t.Fatal(err)
			}
			defer reader.Close()

			readBytes, err := io.ReadAll(reader)
			if err != nil {
				t.Fatal(err)
			}

			if diff := cmp.Diff(tc.contents, readBytes); diff != "" {
				t.Fatalf("unexpected content, diff = %v", diff)
			}
		})
	}
}

func TestArtifactFetcherResolve(t *testing.T) {
	testCases := []struct {
		name     string
		contents []byte
	}{
		{
			name:     "correct size fetched",
			contents: []byte("test"),
		},
		{
			name:     "correct size fetched 2",
			contents: []byte("foobarbaz"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fetcher, err := newFakeArtifactFetcher(imageRef, tc.contents)
			if err != nil {
				t.Fatalf("could not create artifact fetcher: %v", err)
			}
			dgst := digest.FromBytes(tc.contents)
			size := int64(len(tc.contents))
			desc := ocispec.Descriptor{
				Digest: dgst,
			}
			ctx := context.Background()

			desc2, err := fetcher.resolve(ctx, desc)
			if err != nil {
				t.Fatalf("cannot resolve: %v", err)
			}

			if desc2.Size != size {
				t.Fatalf("unexpected size; expected = %d, got = %d", size, desc2.Size)
			}
		})
	}
}

// Tests to make sure that data stored in local store is not fetched again from remote
func TestArtifactFetcherFetchOnlyOnce(t *testing.T) {
	testCases := []struct {
		name     string
		contents []byte
	}{
		{
			name:     "correct data fetched",
			contents: []byte("test"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fetcher, err := newFakeArtifactFetcher(imageRef, tc.contents)
			if err != nil {
				t.Fatalf("could not create artifact fetcher: %v", err)
			}
			dgst := digest.FromBytes(tc.contents)
			size := len(tc.contents)
			desc := ocispec.Descriptor{
				Digest: dgst,
				Size:   int64(size),
			}
			ctx := context.Background()

			reader, local, err := fetcher.Fetch(ctx, desc)
			if err != nil {
				t.Fatal(err)
			}
			if local {
				t.Fatalf("unexpected value of local; expected = false, got = true")
			}
			defer reader.Close()

			err = fetcher.Store(ctx, desc, reader)
			if err != nil {
				t.Fatal(err)
			}

			reader, local, err = fetcher.Fetch(ctx, desc)
			if err != nil {
				t.Fatal(err)
			}
			if !local {
				t.Fatalf("unexpected value of local; expected = true, got = false")
			}
			defer reader.Close()

			readBytes, err := io.ReadAll(reader)
			if err != nil {
				t.Fatal(err)
			}

			if diff := cmp.Diff(tc.contents, readBytes); diff != "" {
				t.Fatalf("unexpected content, diff = %v", diff)
			}
		})
	}
}

func newFakeArtifactFetcher(ref string, contents []byte) (*artifactFetcher, error) {
	refspec, err := reference.Parse(ref)
	if err != nil {
		return nil, err
	}
	return newArtifactFetcher(refspec, memory.New(), newFakeRemoteStore(contents))
}

func newFakeRemoteStore(contents []byte) resolverStorage {
	return &fakeRemoteStore{
		contents: contents,
	}
}

type fakeRemoteStore struct {
	contents []byte
}

var _ content.Storage = &fakeRemoteStore{}

func (f *fakeRemoteStore) Fetch(_ context.Context, desc ocispec.Descriptor) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(f.contents)), nil
}

func (f *fakeRemoteStore) Push(_ context.Context, desc ocispec.Descriptor, ra io.Reader) error {
	return nil
}

func (f *fakeRemoteStore) Exists(_ context.Context, desc ocispec.Descriptor) (bool, error) {
	return true, nil
}

func (f *fakeRemoteStore) Resolve(_ context.Context, ref string) (ocispec.Descriptor, error) {
	return ocispec.Descriptor{
		Size: int64(len(f.contents)),
	}, nil
}
