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

package soci

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/containerd/containerd/images"
	"github.com/google/go-cmp/cmp"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/content/memory"
)

func TestSkipBuildingZtoc(t *testing.T) {
	testcases := []struct {
		name        string
		desc        ocispec.Descriptor
		buildConfig buildConfig
		skip        bool
	}{
		{
			name: "skip, size<minLayerSize",
			desc: ocispec.Descriptor{
				MediaType: SociLayerMediaType,
				Digest:    parseDigest("sha256:88a7002d88ed7b174259637a08a2ef9b7f4f2a314dfb51fa1a4a6a1d7e05dd01"),
				Size:      5223,
			},
			buildConfig: buildConfig{
				minLayerSize: 65535,
			},
			skip: true,
		},
		{
			name: "do not skip, size=minLayerSize",
			desc: ocispec.Descriptor{
				MediaType: SociLayerMediaType,
				Digest:    parseDigest("sha256:88a7002d88ed7b174259637a08a2ef9b7f4f2a314dfb51fa1a4a6a1d7e05dd01"),
				Size:      65535,
			},
			buildConfig: buildConfig{
				minLayerSize: 65535,
			},
			skip: false,
		},
		{
			name: "do not skip, size>minLayerSize",
			desc: ocispec.Descriptor{
				MediaType: SociLayerMediaType,
				Digest:    parseDigest("sha256:88a7002d88ed7b174259637a08a2ef9b7f4f2a314dfb51fa1a4a6a1d7e05dd01"),
				Size:      5000,
			},
			buildConfig: buildConfig{
				minLayerSize: 500,
			},
			skip: false,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			if skipBuildingZtoc(tc.desc, &tc.buildConfig) != tc.skip {
				t.Fatalf("%v: the value returned does not equal actual value %v", tc.name, tc.skip)
			}
		})
	}
}

func TestBuildSociIndexNotLayer(t *testing.T) {
	testcases := []struct {
		name      string
		mediaType string
		err       error
	}{
		{
			name:      "empty media type",
			mediaType: "",
			err:       errNotLayerType,
		},
		{
			name:      "soci index manifest",
			mediaType: ORASManifestMediaType,
			err:       errNotLayerType,
		},
		{
			name:      "soci layer",
			mediaType: SociLayerMediaType,
			err:       errNotLayerType,
		},
		{
			name:      "index manifest",
			mediaType: "application/vnd.oci.image.manifest.v1+json",
			err:       errNotLayerType,
		},
		{
			name:      "layer as tar",
			mediaType: "application/vnd.oci.image.layer.v1.tar",
			err:       errUnsupportedLayerFormat,
		},
		{
			name:      "docker",
			mediaType: images.MediaTypeDockerSchema2Layer,
			err:       errUnsupportedLayerFormat,
		},
		{
			name:      "layer as tar+gzip",
			mediaType: "application/vnd.oci.image.layer.v1.tar+gzip",
		},
		{
			name:      "layer as tar+zstd",
			mediaType: "application/vnd.oci.image.layer.v1.tar+zstd",
		},
		{
			name:      "layer prefix",
			mediaType: "application/vnd.oci.image.layer.",
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			cs := newFakeContentStore()
			desc := ocispec.Descriptor{
				MediaType: tc.mediaType,
				Digest:    "layerdigest",
			}
			cfg := &buildConfig{}
			spanSize := int64(65535)
			blobStore := memory.New()
			_, err := buildSociLayer(ctx, cs, desc, spanSize, blobStore, cfg)
			if tc.err != nil {
				if !errors.Is(err, tc.err) {
					t.Fatalf("%v: should error out as not a layer", tc.name)
				}
			} else {
				if err == errNotLayerType {
					t.Fatalf("%v: should not error out for any of the layer types", tc.name)
				}
			}
		})
	}
}

func TestBuildSociIndexWithLimits(t *testing.T) {
	testcases := []struct {
		name          string
		layerSize     int64
		minLayerSize  int64
		ztocGenerated bool
	}{
		{
			name:          "skip building ztoc: layer size 500 bytes, minimal layer size 32kB",
			layerSize:     500,
			minLayerSize:  32000,
			ztocGenerated: false,
		},
		{
			name:          "skip building ztoc: layer size 20kB, minimal layer size 32kB",
			layerSize:     20000,
			minLayerSize:  32000,
			ztocGenerated: false,
		},
		{
			name:          "build ztoc: layer size 500 bytes, minimal layer size 500 bytes",
			layerSize:     500,
			minLayerSize:  500,
			ztocGenerated: true,
		},
		{
			name:          "build ztoc: layer size 20kB, minimal layer size 500 bytes",
			layerSize:     20000,
			minLayerSize:  500,
			ztocGenerated: true,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			cs := newFakeContentStore()
			desc := ocispec.Descriptor{
				MediaType: "application/vnd.oci.image.layer.",
				Size:      tc.layerSize,
			}
			cfg := &buildConfig{
				minLayerSize: tc.minLayerSize,
			}
			spanSize := int64(65535)
			blobStore := memory.New()
			ztoc, err := buildSociLayer(ctx, cs, desc, spanSize, blobStore, cfg)
			if tc.ztocGenerated {
				// we check only for build skip, which is indicated as nil value for ztoc and nil value for error
				if ztoc == nil && err == nil {
					t.Fatalf("%v: ztoc should've been generated; error=%v", tc.name, err)
				}
			} else {
				if ztoc != nil {
					t.Fatalf("%v: ztoc should've skipped", tc.name)
				}
			}
		})
	}
}

func TestNewIndex(t *testing.T) {
	testcases := []struct {
		name         string
		blobs        []ocispec.Descriptor
		subject      ocispec.Descriptor
		annotations  map[string]string
		manifestType ManifestType
	}{
		{
			name: "successfully build oras manifest",
			blobs: []ocispec.Descriptor{
				{
					Size:   4,
					Digest: digest.FromBytes([]byte("test")),
				},
			},
			subject: ocispec.Descriptor{
				Size:   4,
				Digest: digest.FromBytes([]byte("test")),
			},
			annotations: map[string]string{
				"foo": "bar",
			},
			manifestType: ManifestORAS,
		},
		{
			name: "successfully build OCI ref type manifest",
			blobs: []ocispec.Descriptor{
				{
					Size:   4,
					Digest: digest.FromBytes([]byte("test")),
				},
			},
			subject: ocispec.Descriptor{
				Size:   4,
				Digest: digest.FromBytes([]byte("test")),
			},
			annotations: map[string]string{
				"foo": "bar",
			},
			manifestType: ManifestOCIArtifact,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			index := NewIndex(tc.blobs, &tc.subject, tc.annotations, tc.manifestType)

			if diff := cmp.Diff(index.Blobs, tc.blobs); diff != "" {
				t.Fatalf("unexpected blobs; diff = %v", diff)
			}

			if index.ArtifactType != SociIndexArtifactType {
				t.Fatalf("unexpected artifact type; expected = %s, got = %s", SociIndexArtifactType, index.ArtifactType)
			}

			mt := index.MediaType
			if tc.manifestType == ManifestORAS {
				if mt != ORASManifestMediaType {
					t.Fatalf("unexpected media type; expected = %v, got = %v", ORASManifestMediaType, mt)
				}
			} else if mt != OCIArtifactManifestMediaType {
				t.Fatalf("unexpected media type; expected = %v, got = %v", OCIArtifactManifestMediaType, mt)
			}

			if diff := cmp.Diff(index.Subject, &tc.subject); diff != "" {
				t.Fatalf("the subject field is not equal; diff = %v", diff)
			}
		})
	}
}

func TestNewIndexFromReader(t *testing.T) {
	testcases := []struct {
		name         string
		blobs        []ocispec.Descriptor
		subject      ocispec.Descriptor
		annotations  map[string]string
		manifestType ManifestType
	}{
		{
			name: "successfully build oras manifest",
			blobs: []ocispec.Descriptor{
				{
					Size:   4,
					Digest: digest.FromBytes([]byte("test")),
				},
			},
			subject: ocispec.Descriptor{
				Size:   4,
				Digest: digest.FromBytes([]byte("test")),
			},
			annotations: map[string]string{
				"foo": "bar",
			},
			manifestType: ManifestORAS,
		},
		{
			name: "successfully build OCI ref type manifest",
			blobs: []ocispec.Descriptor{
				{
					Size:   4,
					Digest: digest.FromBytes([]byte("test")),
				},
			},
			subject: ocispec.Descriptor{
				Size:   4,
				Digest: digest.FromBytes([]byte("test")),
			},
			annotations: map[string]string{
				"foo": "bar",
			},
			manifestType: ManifestOCIArtifact,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			index := NewIndex(tc.blobs, &tc.subject, tc.annotations, tc.manifestType)
			jsonBytes, err := json.Marshal(index)
			if err != nil {
				t.Fatalf("cannot convert index to json byte data: %v", err)
			}
			index2, err := NewIndexFromReader(bytes.NewReader(jsonBytes))
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(index, index2); diff != "" {
				t.Fatalf("unexpected index after deserialzing from byte data; diff = %v", diff)
			}
		})
	}
}
