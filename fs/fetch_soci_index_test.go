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
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/awslabs/soci-snapshotter/snapshot"
	"github.com/awslabs/soci-snapshotter/soci"
	"github.com/awslabs/soci-snapshotter/soci/store"
	"github.com/containerd/containerd/remotes/docker"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/content/memory"
)

type inMemoryTestStore struct {
	*memory.Store
}

func newInMemoryTestStore() *inMemoryTestStore {
	return &inMemoryTestStore{
		Store: memory.New(),
	}
}

func (s *inMemoryTestStore) Label(context.Context, ocispec.Descriptor, string, string) error {
	return nil
}

func (s *inMemoryTestStore) Delete(context.Context, digest.Digest) error {
	return nil
}

func (s *inMemoryTestStore) BatchOpen(ctx context.Context) (context.Context, store.CleanupFunc, error) {
	return ctx, store.NopCleanup, nil
}

func registryHostFromServer(ts *httptest.Server) docker.RegistryHost {
	u, _ := url.Parse(ts.URL)
	return docker.RegistryHost{
		Host:   u.Host,
		Scheme: u.Scheme,
		Path:   "/v2",
		Client: ts.Client(),
	}
}

func TestFetchSociIndexFallsBackToNextHost(t *testing.T) {
	indexBytes, err := soci.MarshalIndex(soci.NewIndex(soci.V2, nil, nil, nil))
	if err != nil {
		t.Fatalf("failed to marshal index: %v", err)
	}
	indexDigest := digest.FromBytes(indexBytes)
	manifestDigest := digest.FromString("manifest")
	indexBlobPath := "/v2/library/nginx/blobs/" + indexDigest.String()
	indexManifestPath := "/v2/library/nginx/manifests/" + indexDigest.String()

	var firstHostRequests int32
	firstHost := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&firstHostRequests, 1)
		http.Error(w, "temporary upstream failure", http.StatusBadGateway)
	}))
	defer firstHost.Close()

	var secondHostRequests int32
	secondHost := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&secondHostRequests, 1)
		if r.URL.Path != indexBlobPath && r.URL.Path != indexManifestPath {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Length", strconv.Itoa(len(indexBytes)))
		switch r.Method {
		case http.MethodHead:
			w.WriteHeader(http.StatusOK)
		case http.MethodGet:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(indexBytes)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer secondHost.Close()

	fs := &filesystem{
		contentStore: newInMemoryTestStore(),
	}
	index, err := fs.fetchSociIndex(
		context.Background(),
		"docker.io/library/nginx:latest",
		indexDigest.String(),
		manifestDigest.String(),
		[]docker.RegistryHost{
			registryHostFromServer(firstHost),
			registryHostFromServer(secondHost),
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if index == nil {
		t.Fatal("expected non-nil index")
	}
	if got := atomic.LoadInt32(&firstHostRequests); got == 0 {
		t.Fatal("expected first registry host to be attempted")
	}
	if got := atomic.LoadInt32(&secondHostRequests); got == 0 {
		t.Fatal("expected second registry host to be attempted")
	}
}

func TestFetchSociIndexReturnsNoIndexWhenAllHostsFail(t *testing.T) {
	indexBytes, err := soci.MarshalIndex(soci.NewIndex(soci.V2, nil, nil, nil))
	if err != nil {
		t.Fatalf("failed to marshal index: %v", err)
	}
	indexDigest := digest.FromBytes(indexBytes)
	manifestDigest := digest.FromString("manifest")

	failingHost := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "temporary upstream failure", http.StatusBadGateway)
	}))
	defer failingHost.Close()

	fs := &filesystem{
		contentStore: newInMemoryTestStore(),
	}
	_, err = fs.fetchSociIndex(
		context.Background(),
		"docker.io/library/nginx:latest",
		indexDigest.String(),
		manifestDigest.String(),
		[]docker.RegistryHost{
			registryHostFromServer(failingHost),
		},
	)
	if err == nil {
		t.Fatal("expected error but got nil")
	}
	if !errors.Is(err, snapshot.ErrNoIndex) {
		t.Fatalf("expected error to match snapshot.ErrNoIndex, got: %v", err)
	}
}
