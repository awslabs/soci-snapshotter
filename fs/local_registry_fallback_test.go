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
	"archive/tar"
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/containerd/containerd/remotes/docker"
	digest "github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type countingTestStore struct {
	*inMemoryTestStore
	deleteCalls int32
}

func newCountingTestStore() *countingTestStore {
	return &countingTestStore{
		inMemoryTestStore: newInMemoryTestStore(),
	}
}

func (s *countingTestStore) Delete(context.Context, digest.Digest) error {
	atomic.AddInt32(&s.deleteCalls, 1)
	return nil
}

func TestUnpackLocalLayerFromRegistryHostsFallsBackWithoutRefetchingRemote(t *testing.T) {
	layerBytes := makeTestTarLayer(t, "hello.txt", "mirror fallback")
	layerDigest := digest.FromBytes(layerBytes)
	blobPath := "/v2/library/nginx/blobs/" + layerDigest.String()

	var firstHostRequests int32
	firstHost := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&firstHostRequests, 1)
		http.Error(w, "temporary upstream failure", http.StatusBadGateway)
	}))
	defer firstHost.Close()

	var secondHostGetRequests int32
	secondHost := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != blobPath {
			http.NotFound(w, r)
			return
		}
		switch r.Method {
		case http.MethodGet:
			atomic.AddInt32(&secondHostGetRequests, 1)
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Header().Set("Content-Length", strconv.Itoa(len(layerBytes)))
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(layerBytes)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer secondHost.Close()

	hostRefs, err := buildRegistryHostRefs("docker.io/library/nginx:latest", []docker.RegistryHost{
		registryHostFromServer(firstHost),
		registryHostFromServer(secondHost),
	})
	if err != nil {
		t.Fatalf("unexpected error building registry hosts: %v", err)
	}

	mountpoint := filepath.Join(t.TempDir(), "fs")
	if err := os.MkdirAll(mountpoint, 0755); err != nil {
		t.Fatalf("failed to create mountpoint: %v", err)
	}

	fs := &filesystem{
		contentStore: newInMemoryTestStore(),
	}
	desc := ocispec.Descriptor{
		MediaType: ocispec.MediaTypeImageLayer,
		Digest:    layerDigest,
		Size:      int64(len(layerBytes)),
	}
	if err := fs.unpackLocalLayerFromRegistryHosts(context.Background(), mountpoint, nil, desc, layerDigest, hostRefs); err != nil {
		t.Fatalf("unexpected error unpacking layer: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(mountpoint, "hello.txt"))
	if err != nil {
		t.Fatalf("failed to read unpacked file: %v", err)
	}
	if got, want := string(content), "mirror fallback"; got != want {
		t.Fatalf("unexpected unpacked content, got %q, want %q", got, want)
	}
	if got := atomic.LoadInt32(&firstHostRequests); got == 0 {
		t.Fatal("expected first registry host to be attempted")
	}
	if got, want := atomic.LoadInt32(&secondHostGetRequests), int32(1); got != want {
		t.Fatalf("unexpected number of GET requests to fallback host, got %d, want %d", got, want)
	}
}

func TestUnpackLocalLayerFromRegistryHostsDoesNotDeleteCachedBlobOnApplyFailure(t *testing.T) {
	layerBytes := []byte("not a tar archive")
	layerDigest := digest.FromBytes(layerBytes)
	blobPath := "/v2/library/nginx/blobs/" + layerDigest.String()

	registry := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != blobPath {
			http.NotFound(w, r)
			return
		}
		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Header().Set("Content-Length", strconv.Itoa(len(layerBytes)))
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(layerBytes)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer registry.Close()

	hostRefs, err := buildRegistryHostRefs("docker.io/library/nginx:latest", []docker.RegistryHost{
		registryHostFromServer(registry),
	})
	if err != nil {
		t.Fatalf("unexpected error building registry hosts: %v", err)
	}

	mountpoint := filepath.Join(t.TempDir(), "fs")
	if err := os.MkdirAll(mountpoint, 0755); err != nil {
		t.Fatalf("failed to create mountpoint: %v", err)
	}

	contentStore := newCountingTestStore()
	fs := &filesystem{
		contentStore: contentStore,
	}
	desc := ocispec.Descriptor{
		MediaType: ocispec.MediaTypeImageLayer,
		Digest:    layerDigest,
		Size:      int64(len(layerBytes)),
	}
	err = fs.unpackLocalLayerFromRegistryHosts(context.Background(), mountpoint, nil, desc, layerDigest, hostRefs)
	if err == nil {
		t.Fatal("expected unpack failure for invalid tar content")
	}
	if got := atomic.LoadInt32(&contentStore.deleteCalls); got != 0 {
		t.Fatalf("unexpected content store deletes, got %d, want 0", got)
	}
	exists, err := contentStore.Exists(context.Background(), desc)
	if err != nil {
		t.Fatalf("failed checking cached blob existence: %v", err)
	}
	if !exists {
		t.Fatal("expected cached blob to remain available after apply failure")
	}
}

func makeTestTarLayer(t *testing.T, name, contents string) []byte {
	t.Helper()

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	payload := []byte(contents)
	if err := tw.WriteHeader(&tar.Header{
		Name: name,
		Mode: 0644,
		Size: int64(len(payload)),
	}); err != nil {
		t.Fatalf("failed to write tar header: %v", err)
	}
	if _, err := tw.Write(payload); err != nil {
		t.Fatalf("failed to write tar payload: %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("failed to close tar writer: %v", err)
	}
	return buf.Bytes()
}
