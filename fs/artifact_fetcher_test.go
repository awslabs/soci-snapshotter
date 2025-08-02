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
	"net/http"
	"testing"

	"github.com/awslabs/soci-snapshotter/soci"
	"github.com/awslabs/soci-snapshotter/soci/store"
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

func TestArtifactFetcherStore(t *testing.T) {
	testCases := []struct {
		name          string
		contents      []byte
		digest        digest.Digest
		expectedError error
	}{
		{
			name:          "correct digest succeeds on store",
			contents:      []byte("test"),
			digest:        digest.FromBytes([]byte("test")),
			expectedError: nil,
		},
		{
			name:          "incorrect digest fails on store",
			contents:      []byte("test"),
			digest:        digest.FromBytes([]byte("different data")),
			expectedError: content.ErrMismatchedDigest,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fetcher, err := newFakeArtifactFetcher(imageRef, tc.contents)
			if err != nil {
				t.Fatalf("could not create artifact fetcher: %v", err)
			}
			size := len(tc.contents)
			desc := ocispec.Descriptor{
				Digest: tc.digest,
				Size:   int64(size),
			}
			ctx := context.Background()

			err = fetcher.Store(ctx, desc, bytes.NewReader(tc.contents))
			if !errors.Is(err, tc.expectedError) {
				t.Fatalf("unexpected error, expected = %v, got = %v", tc.expectedError, err)
			}
		})
	}

}

func TestNewRemoteStore(t *testing.T) {
	client := http.Client{}
	testCases := []struct {
		name              string
		ref               string
		shouldBePlainHTTP bool
		expectedError     error
	}{
		{
			name:              "ECR public is not plain http",
			ref:               "public.ecr.aws/ref:tag",
			shouldBePlainHTTP: false,
		},
		{
			name:              "localhost is plain http",
			ref:               "localhost:5000/ref:tag",
			shouldBePlainHTTP: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			refspec, err := reference.Parse(tc.ref)
			if err != nil {
				t.Fatalf("unexpected failure parsing reference: %v", err)
			}
			r, err := newRemoteStore(refspec, &client)
			if err != nil {
				t.Fatalf("unexpected error, got %v", err)
			}
			if r.Client != &client {
				t.Fatalf("unexpected http client, expected %v, got %v", &client, r.Client)
			}
			if r.PlainHTTP != tc.shouldBePlainHTTP {
				t.Fatalf("unepected plain http, expected: %v, got %v", tc.shouldBePlainHTTP, r.PlainHTTP)
			}
		})
	}
}

func TestFetchSociArtifacts(t *testing.T) {
	fakeZtoc := []byte("test data")
	fakeZtocDesc := ocispec.Descriptor{
		Size:   int64(len(fakeZtoc)),
		Digest: digest.FromBytes(fakeZtoc),
	}

	blobs := []ocispec.Descriptor{
		{
			MediaType: soci.SociLayerMediaType,
			Digest:    fakeZtocDesc.Digest,
			Size:      fakeZtocDesc.Size,
		},
	}
	sociIndex := soci.NewIndex(soci.V2, blobs, nil, nil)
	sociBytes, err := soci.MarshalIndex(sociIndex)
	if err != nil {
		t.Fatalf("failed to serialize soci index: %v", err)
	}
	sociIndexDesc := ocispec.Descriptor{
		MediaType: ocispec.MediaTypeImageManifest,
		Size:      int64(len(sociBytes)),
		Digest:    digest.FromBytes(sociBytes),
	}

	modifiedSociIndex := soci.NewIndex(soci.V2, blobs, nil, map[string]string{"a": "b"})
	modifiedSociBytes, err := soci.MarshalIndex(modifiedSociIndex)
	if err != nil {
		t.Fatalf("failed to serialize modified soci index: %v", err)
	}
	modifiedZtocBytes := []byte("modified test data")

	tests := []struct {
		name           string
		remoteContents map[digest.Digest][]byte
		expectedError  error
	}{
		{
			name: "correct data succeeds",
			remoteContents: map[digest.Digest][]byte{
				sociIndexDesc.Digest: sociBytes,
				fakeZtocDesc.Digest:  fakeZtoc,
			},
			expectedError: nil,
		},
		{
			name: "modified index data fails",
			remoteContents: map[digest.Digest][]byte{
				sociIndexDesc.Digest: modifiedSociBytes,
				fakeZtocDesc.Digest:  fakeZtoc,
			},
			expectedError: content.ErrMismatchedDigest,
		},
		{
			name: "modified ztoc data fails",
			remoteContents: map[digest.Digest][]byte{
				sociIndexDesc.Digest: sociBytes,
				fakeZtocDesc.Digest:  modifiedZtocBytes,
			},
			expectedError: content.ErrTrailingData,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			_, err = FetchSociArtifacts(ctx, reference.Spec{}, sociIndexDesc, newFakeLocalStore(), newFakeRemoteStoreWithContents(test.remoteContents))
			if !errors.Is(err, test.expectedError) {
				t.Fatalf("unexpected error, got: %v. expected: %v", err, test.expectedError)
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

func newFakeLocalStore() store.Store {
	return &fakeLocalStore{
		Store: memory.New(),
	}
}

type fakeLocalStore struct {
	*memory.Store
}

func (s *fakeLocalStore) BatchOpen(ctx context.Context) (context.Context, store.CleanupFunc, error) {
	return ctx, func(ctx context.Context) error { return nil }, nil
}

func (s *fakeLocalStore) Delete(_ context.Context, _ digest.Digest) error {
	return nil
}

func (s *fakeLocalStore) Label(_ context.Context, _ ocispec.Descriptor, _, _ string) error {
	return nil
}

func newFakeRemoteStore(contents []byte) resolverStorage {
	return &fakeRemoteStore{
		defaultContents: contents,
		contents:        make(map[digest.Digest][]byte),
	}
}

func newFakeRemoteStoreWithContents(contents map[digest.Digest][]byte) resolverStorage {
	return &fakeRemoteStore{
		defaultContents: []byte{},
		contents:        contents,
	}
}

type fakeRemoteStore struct {
	defaultContents []byte
	contents        map[digest.Digest][]byte
}

var _ content.Storage = &fakeRemoteStore{}

func (f *fakeRemoteStore) Fetch(_ context.Context, desc ocispec.Descriptor) (io.ReadCloser, error) {
	if data, ok := f.contents[desc.Digest]; ok {
		return io.NopCloser(bytes.NewReader(data)), nil
	}

	return io.NopCloser(bytes.NewReader(f.defaultContents)), nil
}

func (f *fakeRemoteStore) Push(_ context.Context, desc ocispec.Descriptor, ra io.Reader) error {
	return nil
}

func (f *fakeRemoteStore) Exists(_ context.Context, desc ocispec.Descriptor) (bool, error) {
	return true, nil
}

func (f *fakeRemoteStore) Resolve(_ context.Context, ref string) (ocispec.Descriptor, error) {
	return ocispec.Descriptor{
		Size: int64(len(f.defaultContents)),
	}, nil
}
