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
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/awslabs/soci-snapshotter/soci/store"
	"oras.land/oras-go/v2/content/memory"

	"github.com/containerd/containerd/v2/core/remotes/docker"
	"github.com/containerd/containerd/v2/pkg/reference"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// fakeRegistry serves the blobs it has and records requests.
type fakeRegistry struct {
	*httptest.Server
	blobs    map[digest.Digest][]byte
	failGet  bool // answer HEAD, but fail GET with 500
	requests atomic.Int32
	mu       sync.Mutex
	gets     int
	ns       []string // ns query parameter of each request
}

func newFakeRegistry(t *testing.T, blobs ...[]byte) *fakeRegistry {
	r := &fakeRegistry{blobs: map[digest.Digest][]byte{}}
	for _, b := range blobs {
		r.blobs[digest.FromBytes(b)] = b
	}
	r.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		r.requests.Add(1)
		r.mu.Lock()
		r.ns = append(r.ns, req.URL.Query().Get("ns"))
		if req.Method == http.MethodGet {
			r.gets++
		}
		r.mu.Unlock()
		i := strings.LastIndex(req.URL.Path, "/blobs/")
		if i < 0 {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		blob, ok := r.blobs[digest.Digest(req.URL.Path[i+len("/blobs/"):])]
		switch {
		case !ok:
			w.WriteHeader(http.StatusNotFound)
		case req.Method == http.MethodGet && r.failGet:
			w.WriteHeader(http.StatusInternalServerError)
		default:
			http.ServeContent(w, req, "", time.Time{}, bytes.NewReader(blob))
		}
	}))
	t.Cleanup(r.Close)
	return r
}

func (r *fakeRegistry) host(t *testing.T) string {
	u, err := url.Parse(r.URL)
	if err != nil {
		t.Fatalf("cannot parse test server URL: %v", err)
	}
	return u.Host
}

func TestSelectBlobSource(t *testing.T) {
	onMirror := []byte("on mirror")
	notOnMirror := []byte("not on mirror")

	mirror := newFakeRegistry(t, onMirror)
	origin := newFakeRegistry(t, onMirror, notOnMirror)

	refspec, err := reference.Parse(origin.host(t) + "/team/app:v1")
	if err != nil {
		t.Fatalf("cannot parse ref: %v", err)
	}
	hosts := []docker.RegistryHost{
		{Client: http.DefaultClient, Host: mirror.host(t), Scheme: "http", Path: "/v2"},
		{Client: http.DefaultClient, Host: origin.host(t), Scheme: "http", Path: "/v2"},
	}
	fs := &filesystem{}
	sources, err := fs.newBlobSources(context.Background(), refspec, hosts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sources) != 2 {
		t.Fatalf("expected 2 sources, got %d", len(sources))
	}
	if got, want := sources[0].refspec.Locator, mirror.host(t)+"/team/app"; got != want {
		t.Fatalf("unexpected mirror locator, expected %s, got %s", want, got)
	}

	testCases := []struct {
		name           string
		digest         digest.Digest
		expectedHosts  []string
		mirrorRequests int32
	}{
		{name: "layer on mirror is fetched from mirror, origin is the fallback", digest: digest.FromBytes(onMirror), expectedHosts: []string{mirror.host(t), origin.host(t)}, mirrorRequests: 1},
		{name: "layer missing on mirror is fetched from origin", digest: digest.FromBytes(notOnMirror), expectedHosts: []string{origin.host(t)}, mirrorRequests: 1},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mirror.requests.Store(0)
			var got []string
			for _, src := range selectBlobSource(context.Background(), sources, ocispec.Descriptor{Digest: tc.digest}) {
				got = append(got, src.refspec.Hostname())
			}
			if strings.Join(got, ",") != strings.Join(tc.expectedHosts, ",") {
				t.Fatalf("unexpected sources, expected %v, got %v", tc.expectedHosts, got)
			}
			if got := mirror.requests.Load(); got != tc.mirrorRequests {
				t.Fatalf("unexpected number of mirror requests, expected %d, got %d", tc.mirrorRequests, got)
			}
		})
	}
}

func TestSelectBlobSourceWithoutMirrors(t *testing.T) {
	origin := newFakeRegistry(t)
	refspec, err := reference.Parse(origin.host(t) + "/repo:tag")
	if err != nil {
		t.Fatalf("cannot parse ref: %v", err)
	}
	fs := &filesystem{}
	sources, err := fs.newBlobSources(context.Background(), refspec,
		[]docker.RegistryHost{{Client: http.DefaultClient, Host: origin.host(t), Scheme: "http", Path: "/v2"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	selected := selectBlobSource(context.Background(), sources, ocispec.Descriptor{Digest: digest.FromString("layer")})
	if len(selected) != 1 || selected[0].refspec.String() != refspec.String() {
		t.Fatalf("expected only the origin refspec %s, got %v", refspec, selected)
	}
	if got := origin.requests.Load(); got != 0 {
		t.Fatalf("expected no probe requests without mirrors, got %d", got)
	}
}

func TestNewBlobSourcesOriginScheme(t *testing.T) {
	refspec, err := reference.Parse("registry.example.com/repo:tag")
	if err != nil {
		t.Fatalf("cannot parse ref: %v", err)
	}
	origin := docker.RegistryHost{Client: http.DefaultClient, Host: "registry.example.com", Scheme: "https", Path: "/v2"}
	testCases := []struct {
		name      string
		mirror    docker.RegistryHost
		plainHTTP bool
	}{
		{
			name:      "plain HTTP mirror does not make the image registry plain HTTP",
			mirror:    docker.RegistryHost{Client: http.DefaultClient, Host: "10.0.0.1:30020", Scheme: "http", Path: "/v2"},
			plainHTTP: false,
		},
		{
			name:      "image registry configured as a plain HTTP mirror of itself",
			mirror:    docker.RegistryHost{Client: http.DefaultClient, Host: "registry.example.com", Scheme: "http", Path: "/v2"},
			plainHTTP: true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fs := &filesystem{}
			sources, err := fs.newBlobSources(context.Background(), refspec, []docker.RegistryHost{tc.mirror, origin})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got := sources[len(sources)-1].store.PlainHTTP; got != tc.plainHTTP {
				t.Fatalf("unexpected image registry PlainHTTP, expected %v, got %v", tc.plainHTTP, got)
			}
		})
	}
}

func TestParallelFetchFromMirrorWithFallback(t *testing.T) {
	layer := []byte("layer contents")
	desc := ocispec.Descriptor{Digest: digest.FromBytes(layer), Size: int64(len(layer))}

	testCases := []struct {
		name          string
		mirrorFailGet bool
		mirrorGets    int
		originGets    int
	}{
		{name: "mirror serves the layer", mirrorFailGet: false, mirrorGets: 1, originGets: 0},
		{name: "mirror download fails, image registry serves the layer", mirrorFailGet: true, mirrorGets: 1, originGets: 1},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancelCause(context.Background())
			defer cancel(nil)

			mirror := newFakeRegistry(t, layer)
			mirror.failGet = tc.mirrorFailGet
			origin := newFakeRegistry(t, layer)
			refspec, err := reference.Parse(origin.host(t) + "/team/app:v1")
			if err != nil {
				t.Fatalf("cannot parse ref: %v", err)
			}
			fs := &filesystem{}
			sources, err := fs.newBlobSources(ctx, refspec, []docker.RegistryHost{
				{Client: http.DefaultClient, Host: mirror.host(t), Scheme: "http", Path: "/v2"},
				{Client: http.DefaultClient, Host: origin.host(t), Scheme: "http", Path: "/v2"},
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			selected := selectBlobSource(ctx, sources, desc)
			var localStore store.BasicStore = &fakeLocalStore{Store: memory.New()}
			fetcher, err := newParallelArtifactFetcher(selected[0].refspec, localStore, selected[0].store,
				newTestLayerJob(ctx, t, cancel, desc), 0, newAsyncVerifier(desc.Digest.Verifier()))
			if err != nil {
				t.Fatalf("cannot create fetcher: %v", err)
			}
			fetcher.fallbacks = selected[1:]

			rc, _, err := fetcher.Fetch(ctx, desc)
			if err != nil {
				t.Fatalf("unexpected fetch error: %v", err)
			}
			defer rc.Close()
			got, err := io.ReadAll(rc)
			if err != nil {
				t.Fatalf("cannot read fetched layer: %v", err)
			}
			if !bytes.Equal(got, layer) {
				t.Fatalf("unexpected layer contents %q", got)
			}
			if mirror.gets != tc.mirrorGets || origin.gets != tc.originGets {
				t.Fatalf("unexpected GET requests, mirror expected %d got %d, image registry expected %d got %d",
					tc.mirrorGets, mirror.gets, tc.originGets, origin.gets)
			}
			for _, ns := range mirror.ns {
				if ns != origin.host(t) {
					t.Fatalf("expected ns=%s on every mirror request, got %q", origin.host(t), ns)
				}
			}
			for _, ns := range origin.ns {
				if ns != "" {
					t.Fatalf("expected no ns on image registry requests, got %q", ns)
				}
			}
		})
	}
}
