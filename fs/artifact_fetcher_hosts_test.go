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
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/containerd/containerd/v2/core/remotes/docker"
	"github.com/containerd/containerd/v2/pkg/reference"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// blobServer serves a single blob and records the requests it receives.
type blobServer struct {
	*httptest.Server
	mu       sync.Mutex
	requests []*http.Request
}

func newBlobServer(t *testing.T, repository string, blob []byte, available bool) *blobServer {
	t.Helper()
	bs := &blobServer{}
	blobPath := "/v2/" + repository + "/blobs/" + digest.FromBytes(blob).String()
	bs.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bs.mu.Lock()
		bs.requests = append(bs.requests, r.Clone(context.Background()))
		bs.mu.Unlock()
		if !available || r.URL.Path != blobPath {
			http.NotFound(w, r)
			return
		}
		// ServeContent handles Range requests and sets Accept-Ranges: bytes.
		http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(blob))
	}))
	t.Cleanup(bs.Close)
	return bs
}

func (bs *blobServer) requestCount() int {
	bs.mu.Lock()
	defer bs.mu.Unlock()
	return len(bs.requests)
}

func (bs *blobServer) nsValues() []string {
	bs.mu.Lock()
	defer bs.mu.Unlock()
	var values []string
	for _, r := range bs.requests {
		values = append(values, r.URL.Query().Get("ns"))
	}
	return values
}

func (bs *blobServer) host() string {
	return strings.TrimPrefix(bs.URL, "http://")
}

func testRegistryHost(bs *blobServer) docker.RegistryHost {
	return docker.RegistryHost{
		Client:       bs.Client(),
		Host:         bs.host(),
		Scheme:       "http",
		Path:         "/v2",
		Capabilities: docker.HostCapabilityPull | docker.HostCapabilityResolve,
	}
}

func TestNewBlobHosts(t *testing.T) {
	refspec, err := reference.Parse("docker.io/library/busybox:latest")
	if err != nil {
		t.Fatal(err)
	}
	hosts := []docker.RegistryHost{
		{Host: "10.0.0.1:30020", Scheme: "http", Path: "/v2", Capabilities: docker.HostCapabilityPull},
		{Host: "push-only.example.com", Scheme: "https", Path: "/v2", Capabilities: docker.HostCapabilityPush},
		{Host: "registry-1.docker.io", Scheme: "https", Capabilities: docker.HostCapabilityPull | docker.HostCapabilityResolve},
	}
	blobHosts, err := newBlobHosts(refspec, hosts)
	if err != nil {
		t.Fatal(err)
	}
	if len(blobHosts) != 2 {
		t.Fatalf("expected 2 blob hosts, got %d", len(blobHosts))
	}
	mirrorURL := blobHosts[0].blobURL("library/busybox", "sha256:abc")
	if expected := "http://10.0.0.1:30020/v2/library/busybox/blobs/sha256:abc?ns=docker.io"; mirrorURL != expected {
		t.Fatalf("unexpected mirror blob URL, expected %s, got %s", expected, mirrorURL)
	}
	registryURL := blobHosts[1].blobURL("library/busybox", "sha256:abc")
	if expected := "https://registry-1.docker.io/v2/library/busybox/blobs/sha256:abc"; registryURL != expected {
		t.Fatalf("unexpected registry blob URL, expected %s, got %s", expected, registryURL)
	}
}

func TestOrasBlobStoreFromHostsFallback(t *testing.T) {
	const repository = "repo"
	blob := bytes.Repeat([]byte("soci"), 1024)
	desc := ocispec.Descriptor{
		MediaType: ocispec.MediaTypeImageLayerGzip,
		Digest:    digest.FromBytes(blob),
		Size:      int64(len(blob)),
	}

	testCases := []struct {
		name           string
		mirrorHasBlob  bool
		mirrorDown     bool
		expectRegistry bool
	}{
		{
			name:           "mirror serves the blob",
			mirrorHasBlob:  true,
			expectRegistry: false,
		},
		{
			name:           "mirror does not have the blob",
			mirrorHasBlob:  false,
			expectRegistry: true,
		},
		{
			name:           "mirror is unreachable",
			mirrorDown:     true,
			expectRegistry: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			reg := newBlobServer(t, repository, blob, true)
			mirror := newBlobServer(t, repository, blob, tc.mirrorHasBlob)
			if tc.mirrorDown {
				mirror.Close()
			}

			refspec, err := reference.Parse(reg.host() + "/" + repository + ":tag")
			if err != nil {
				t.Fatal(err)
			}
			store, err := newRemoteBlobStoreFromHosts(refspec, []docker.RegistryHost{testRegistryHost(mirror), testRegistryHost(reg)})
			if err != nil {
				t.Fatal(err)
			}
			ref := constructRef(refspec, desc)

			rangeSupported, err := store.doInitialFetch(ctx, ref)
			if err != nil {
				t.Fatalf("doInitialFetch failed: %v", err)
			}
			if !rangeSupported {
				t.Fatal("expected ranged GET support")
			}

			var got []byte
			chunk := int64(1000)
			for lower := int64(0); lower < desc.Size; lower += chunk {
				upper := min(lower+chunk, desc.Size) - 1
				rc, err := store.FetchRange(ctx, ref, lower, upper)
				if err != nil {
					t.Fatalf("FetchRange [%d, %d] failed: %v", lower, upper, err)
				}
				b, err := io.ReadAll(rc)
				rc.Close()
				if err != nil {
					t.Fatal(err)
				}
				got = append(got, b...)
			}
			if !bytes.Equal(got, blob) {
				t.Fatal("ranged fetch returned unexpected content")
			}

			rc, err := store.Fetch(ctx, desc)
			if err != nil {
				t.Fatalf("Fetch failed: %v", err)
			}
			b, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(b, blob) {
				t.Fatal("fetch returned unexpected content")
			}

			if tc.expectRegistry && reg.requestCount() == 0 {
				t.Fatal("expected requests to the registry")
			}
			if !tc.expectRegistry && reg.requestCount() != 0 {
				t.Fatalf("expected no requests to the registry, got %d", reg.requestCount())
			}
			for _, ns := range mirror.nsValues() {
				if ns != reg.host() {
					t.Fatalf("expected mirror requests to carry ns=%s, got %q", reg.host(), ns)
				}
			}
			for _, ns := range reg.nsValues() {
				if ns != "" {
					t.Fatalf("expected registry requests without ns, got %q", ns)
				}
			}
		})
	}
}
