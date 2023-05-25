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

package store

import (
	"context"
	"testing"

	"github.com/awslabs/soci-snapshotter/config"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/content/memory"
)

func TestStoreCanonicalizeContentStoreType(t *testing.T) {
	tests := []struct {
		input  string
		output ContentStoreType
		fail   bool
	}{
		{
			input:  "",
			output: config.DefaultContentStoreType,
		},
		{
			input:  "soci",
			output: SociContentStoreType,
		},
		{
			input:  "containerd",
			output: ContainerdContentStoreType,
		},
		{
			input: "bad",
			fail:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			output, err := CanonicalizeContentStoreType(ContentStoreType(tt.input))
			if err != nil {
				if !tt.fail {
					t.Fatalf("content store type \"%s\" canonicalized to \"%s\" and produced unexpected error %v", tt.input, output, err)
				}
			} else {
				if tt.output != output {
					t.Fatalf("content store type \"%s\" canonicalized to \"%s\", expected %s", tt.input, output, tt.output)
				}
			}
		})
	}

}

func TestStoreGetContentStorePath(t *testing.T) {
	var defaultContentStorePath string
	switch ContentStoreType(config.DefaultContentStoreType) {
	case SociContentStoreType:
		defaultContentStorePath = DefaultSociContentStorePath
	case ContainerdContentStoreType:
		defaultContentStorePath = DefaultContainerdContentStorePath
	default:
		t.Fatalf("test invalidated by unrecognized default content store type: %s", config.DefaultContentStoreType)
	}

	tests := []struct {
		input  string
		output string
		fail   bool
	}{
		{
			input:  "",
			output: defaultContentStorePath,
		},
		{
			input:  "soci",
			output: DefaultSociContentStorePath,
		},
		{
			input:  "containerd",
			output: DefaultContainerdContentStorePath,
		},
		{
			input: "bad",
			fail:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			output, err := GetContentStorePath(ContentStoreType(tt.input))
			if err != nil {
				if !tt.fail {
					t.Fatalf("content store type \"%s\" produced path %s with unexpected error %v", tt.input, output, err)
				}
			} else {
				if tt.output != output {
					t.Fatalf("content store type \"%s\" produced path %s, expected %s", tt.input, output, tt.output)
				}
			}
		})
	}

}

type fakeStore struct {
	*memory.Store
	Labels  [][]string
	Deleted []string
}

// assert that FakeStore implements Store
var _ Store = (*fakeStore)(nil)

func newFakeStore() *fakeStore {
	fakeStore := fakeStore{}
	fakeStore.Store = memory.New()
	return &fakeStore
}

// TODO read and record namespace from context
// Label fakes labeling resources by maintaining an array of labels that have been added
func (s *fakeStore) Label(_ context.Context, desc ocispec.Descriptor, name string, value string) error {
	s.Labels = append(s.Labels, []string{desc.Digest.String(), name, value})
	return nil
}

// Delete fakes deleting resources by maintaining an array of resources that have been "deleted"
func (s *fakeStore) Delete(_ context.Context, dgst digest.Digest) error {
	s.Deleted = append(s.Deleted, dgst.String())
	return nil
}

// BatchOpen is a TODO to mock and test
func (s *fakeStore) BatchOpen(ctx context.Context) (context.Context, CleanupFunc, error) {
	return ctx, func(context.Context) error { return nil }, nil
}

func TestStoreLabelGCRoot(t *testing.T) {
	store := newFakeStore()
	testTarget, _ := digest.Parse("sha256:7b236f6c6ca259a4497e98c204bc1dcf3e653438e74af17bfe39da5329789f4a")
	LabelGCRoot(context.Background(), store, ocispec.Descriptor{Digest: testTarget})
	if len(store.Labels) != 1 {
		t.Fatalf("wrong number of labels applied, expected 1, got %d", len(store.Labels))
	}
	if store.Labels[0][0] != testTarget.String() {
		t.Fatalf("label applied to wrong digest, expected \"%s\", got \"%s\"", testTarget.String(), store.Labels[0][0])
	}
	if store.Labels[0][1] != "containerd.io/gc.root" {
		t.Fatalf("label applied with wrong name, expected \"containerd.io/gc.root\", got \"%s\"", store.Labels[0][1])
	}
}

func TestStoreLabelGCRefContent(t *testing.T) {
	store := newFakeStore()
	testTarget, _ := digest.Parse("sha256:7b236f6c6ca259a4497e98c204bc1dcf3e653438e74af17bfe39da5329789f4a")
	testRef := "testRef"
	testDigest, _ := digest.Parse("sha256:4452aadba3e99771ff3559735dab16279c5a352359d79f38737c6fdca941c6e5")
	LabelGCRefContent(context.Background(), store, ocispec.Descriptor{Digest: testTarget}, testRef, testDigest.String())
	if len(store.Labels) != 1 {
		t.Fatalf("wrong number of labels applied, expected 1, got %d", len(store.Labels))
	}
	if store.Labels[0][0] != testTarget.String() {
		t.Fatalf("label applied to wrong digest, expected \"%s\", got \"%s\"", testTarget.String(), store.Labels[0][0])
	}
	if store.Labels[0][1] != "containerd.io/gc.ref.content."+testRef {
		t.Fatalf("label applied with wrong name, expected \"containerd.io/gc.ref.content."+testRef+"\", got \"%s\"", store.Labels[0][1])
	}
	if store.Labels[0][2] != testDigest.String() {
		t.Fatalf("label references wrong digest, expected \"%s\", got \"%s\"", testDigest.String(), store.Labels[0][2])
	}
}
