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

package internal

import (
	"bytes"
	"context"
	"io"
	"testing"

	local "github.com/containerd/containerd/v2/plugins/content/local"
	digest "github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

func TestContentStoreAdapter(t *testing.T) {
	ctx := context.Background()

	cs, err := local.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("local.NewStore: %v", err)
	}
	adapter := newContentStoreAdapter(cs)

	data := []byte("hello from the content store adapter")
	desc := ocispec.Descriptor{
		MediaType: "application/octet-stream",
		Digest:    digest.FromBytes(data),
		Size:      int64(len(data)),
	}

	if exists, err := adapter.Exists(ctx, desc); err != nil {
		t.Fatalf("Exists (before push): %v", err)
	} else if exists {
		t.Fatal("Exists (before push) = true, want false")
	}

	if err := adapter.Push(ctx, desc, bytes.NewReader(data)); err != nil {
		t.Fatalf("Push: %v", err)
	}

	if exists, err := adapter.Exists(ctx, desc); err != nil {
		t.Fatalf("Exists (after push): %v", err)
	} else if !exists {
		t.Fatal("Exists (after push) = false, want true")
	}

	rc, err := adapter.Fetch(ctx, desc)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	defer rc.Close()

	got, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("reading fetched content: %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Fatalf("Fetch returned %q, want %q", got, data)
	}

	// Pushing the same descriptor again should be a harmless no-op, not
	// an error (mirrors containerd's normal "already exists" semantics).
	if err := adapter.Push(ctx, desc, bytes.NewReader(data)); err != nil {
		t.Fatalf("Push (duplicate): %v", err)
	}
}
