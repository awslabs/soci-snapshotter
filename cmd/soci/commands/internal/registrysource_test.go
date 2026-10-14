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
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/containerd/containerd/v2/core/images"
	local "github.com/containerd/containerd/v2/plugins/content/local"
	digest "github.com/opencontainers/go-digest"
	specs "github.com/opencontainers/image-spec/specs-go"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/urfave/cli/v3"
)

// fakeRegistry is a minimal in-process OCI registry - just the manifest
// HEAD/GET and blob GET endpoints oraslib.CopyGraph actually calls to pull
// a single-platform image - so PopulateFromRegistry can be tested without
// a real registry or a new dependency (go-containerregistry's registry
// package would make this trivial, but it isn't otherwise a dependency of
// this module).
type fakeRegistry struct {
	repoName     string
	manifestDesc ocispec.Descriptor
	manifest     []byte
	blobs        map[digest.Digest][]byte // config + layers, by digest
}

func newFakeRegistry(t *testing.T, repoName string) *fakeRegistry {
	t.Helper()

	configBytes := []byte(`{}`)
	configDesc := ocispec.Descriptor{
		MediaType: "application/vnd.oci.image.config.v1+json",
		Digest:    digest.FromBytes(configBytes),
		Size:      int64(len(configBytes)),
	}

	layerBytes := []byte("hello from the fake registry layer")
	layerDesc := ocispec.Descriptor{
		MediaType: "application/vnd.oci.image.layer.v1.tar",
		Digest:    digest.FromBytes(layerBytes),
		Size:      int64(len(layerBytes)),
	}

	manifest := ocispec.Manifest{
		Versioned: specs.Versioned{SchemaVersion: 2},
		MediaType: ocispec.MediaTypeImageManifest,
		Config:    configDesc,
		Layers:    []ocispec.Descriptor{layerDesc},
	}
	manifestBytes, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshaling manifest: %v", err)
	}
	manifestDesc := ocispec.Descriptor{
		MediaType: ocispec.MediaTypeImageManifest,
		Digest:    digest.FromBytes(manifestBytes),
		Size:      int64(len(manifestBytes)),
	}

	return &fakeRegistry{
		repoName:     repoName,
		manifestDesc: manifestDesc,
		manifest:     manifestBytes,
		blobs: map[digest.Digest][]byte{
			configDesc.Digest: configBytes,
			layerDesc.Digest:  layerBytes,
		},
	}
}

func (r *fakeRegistry) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	prefix := "/v2/" + r.repoName + "/"
	if !strings.HasPrefix(req.URL.Path, prefix) {
		http.NotFound(w, req)
		return
	}
	rest := strings.TrimPrefix(req.URL.Path, prefix)

	writeBlob := func(mediaType, dgst string, body []byte) {
		w.Header().Set("Content-Type", mediaType)
		w.Header().Set("Docker-Content-Digest", dgst)
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		w.WriteHeader(http.StatusOK)
		if req.Method != http.MethodHead {
			w.Write(body)
		}
	}

	switch {
	case strings.HasPrefix(rest, "manifests/"):
		ref := strings.TrimPrefix(rest, "manifests/")
		if ref != "latest" && ref != r.manifestDesc.Digest.String() {
			http.NotFound(w, req)
			return
		}
		writeBlob(r.manifestDesc.MediaType, r.manifestDesc.Digest.String(), r.manifest)

	case strings.HasPrefix(rest, "blobs/"):
		dgst, err := digest.Parse(strings.TrimPrefix(rest, "blobs/"))
		if err != nil {
			http.Error(w, "bad digest", http.StatusBadRequest)
			return
		}
		blob, ok := r.blobs[dgst]
		if !ok {
			http.NotFound(w, req)
			return
		}
		writeBlob("application/octet-stream", dgst.String(), blob)

	default:
		http.NotFound(w, req)
	}
}

func TestPopulateFromRegistry(t *testing.T) {
	const repoName = "test-image"
	reg := newFakeRegistry(t, repoName)

	srv := httptest.NewServer(reg)
	defer srv.Close()

	ref := strings.TrimPrefix(srv.URL, "http://") + "/" + repoName + ":latest"

	cs, err := local.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("local.NewStore: %v", err)
	}

	var (
		gotImg images.Image
		gotErr error
	)
	testCmd := &cli.Command{
		Name:  "test",
		Flags: RegistryFlags,
		Action: func(ctx context.Context, cmd *cli.Command) error {
			gotImg, gotErr = PopulateFromRegistry(ctx, cmd, ref, cs)
			return nil
		},
	}
	if err := testCmd.Run(context.Background(), []string{"test", "--plain-http"}); err != nil {
		t.Fatalf("running test command: %v", err)
	}
	if gotErr != nil {
		t.Fatalf("PopulateFromRegistry: %v", gotErr)
	}

	if gotImg.Name != ref {
		t.Errorf("Image.Name = %q, want %q", gotImg.Name, ref)
	}
	if gotImg.Target.Digest != reg.manifestDesc.Digest {
		t.Errorf("Image.Target.Digest = %s, want %s", gotImg.Target.Digest, reg.manifestDesc.Digest)
	}

	// Every blob the fake registry served (manifest, config, layer) must
	// have actually been ingested into cs with matching content.
	check := func(desc ocispec.Descriptor, want []byte) {
		t.Helper()
		ra, err := cs.ReaderAt(context.Background(), desc)
		if err != nil {
			t.Fatalf("ReaderAt(%s): %v", desc.Digest, err)
		}
		defer ra.Close()
		got, err := io.ReadAll(io.NewSectionReader(ra, 0, desc.Size))
		if err != nil {
			t.Fatalf("reading %s: %v", desc.Digest, err)
		}
		if string(got) != string(want) {
			t.Errorf("content for %s = %q, want %q", desc.Digest, got, want)
		}
	}
	check(reg.manifestDesc, reg.manifest)
	for dgst, blob := range reg.blobs {
		check(ocispec.Descriptor{Digest: dgst, Size: int64(len(blob))}, blob)
	}
}
