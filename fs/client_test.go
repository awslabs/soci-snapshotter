//go:build testing

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
	"context"
	"errors"
	"io"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type fakeInner struct {
	descs []ocispec.Descriptor
}

func newFakeInner(descs []ocispec.Descriptor) *fakeInner {
	return &fakeInner{
		descs: descs,
	}
}

var _ Inner = &fakeInner{}

func (f *fakeInner) Exists(ctx context.Context, desc ocispec.Descriptor) (bool, error) {
	return false, nil
}

func (f *fakeInner) Fetch(ctx context.Context, desc ocispec.Descriptor) (io.ReadCloser, error) {
	return nil, nil
}

func (f *fakeInner) Push(ctx context.Context, expected ocispec.Descriptor, content io.Reader) error {
	return nil
}

func (f *fakeInner) Referrers(ctx context.Context, desc ocispec.Descriptor, artifactType string, fn func(referrers []ocispec.Descriptor) error) error {
	return fn(f.descs)
}

func TestOCIArtifactClientSelectReferrer(t *testing.T) {
	testCases := []struct {
		name            string
		descs           []ocispec.Descriptor
		expectedErr     error
		expectedDesc    ocispec.Descriptor
		selectionPolicy IndexSelectionPolicy
	}{
		{
			name:        "empty referrers list returns ErrNoReferrers",
			descs:       make([]ocispec.Descriptor, 0),
			expectedErr: ErrNoReferrers,
		},
		{
			name: "SelectFirstPolicy returns the first descriptor",
			descs: []ocispec.Descriptor{
				{
					Digest: digest.FromBytes([]byte("foo")),
					Size:   3,
				},
				{
					Digest: digest.FromBytes([]byte("test")),
					Size:   4,
				},
			},
			expectedDesc: ocispec.Descriptor{
				Digest: digest.FromBytes([]byte("foo")),
				Size:   3,
			},
			selectionPolicy: SelectFirstPolicy,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			inner := newFakeInner(tc.descs)
			client := NewOCIArtifactClient(inner)

			desc, err := client.SelectReferrer(context.Background(), ocispec.Descriptor{}, tc.selectionPolicy)
			if err != nil && !errors.Is(err, tc.expectedErr) {
				t.Fatalf("unexpected error getting descriptor: %v", err)
			}

			if diff := cmp.Diff(desc, tc.expectedDesc); diff != "" {
				t.Fatalf("unexpected descriptor; diff = %v", diff)
			}
		})
	}
}
