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

	"github.com/awslabs/soci-snapshotter/ztoc"
	"github.com/containerd/containerd/images"
	"github.com/google/go-cmp/cmp"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

func TestSkipBuildingZtoc(t *testing.T) {
	testcases := []struct {
		name        string
		desc        ocispec.Descriptor
		buildConfig builderConfig
		skip        bool
	}{
		{
			name: "skip, size<minLayerSize",
			desc: ocispec.Descriptor{
				MediaType: SociLayerMediaType,
				Digest:    parseDigest("sha256:88a7002d88ed7b174259637a08a2ef9b7f4f2a314dfb51fa1a4a6a1d7e05dd01"),
				Size:      5223,
			},
			buildConfig: builderConfig{
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
			buildConfig: builderConfig{
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
			buildConfig: builderConfig{
				minLayerSize: 500,
			},
			skip: false,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			if skip, _ := skipBuildingZtoc(tc.desc, &tc.buildConfig); skip != tc.skip {
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
			mediaType: ocispec.MediaTypeImageManifest,
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
		},
		{
			name:      "docker",
			mediaType: images.MediaTypeDockerSchema2Layer,
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

	spanSize := int64(65535)
	ctx := context.Background()
	cs := newFakeContentStore()
	blobStore := NewOrasMemoryStore()

	artifactsDb, err := newTestableDb()
	if err != nil {
		t.Fatalf("can't create a test db")
	}
	builder, err := NewIndexBuilder(cs, blobStore, WithArtifactsDb(artifactsDb), WithSpanSize(spanSize), WithMinLayerSize(0))

	if err != nil {
		t.Fatalf("cannot create index builer: %v", err)
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			desc := ocispec.Descriptor{
				MediaType: tc.mediaType,
				Digest:    "layerdigest",
			}
			_, err := builder.buildSociLayer(ctx, desc)
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
			spanSize := int64(65535)
			blobStore := NewOrasMemoryStore()
			artifactsDb, err := newTestableDb()
			if err != nil {
				t.Fatalf("can't create a test db")
			}
			builder, _ := NewIndexBuilder(cs, blobStore, WithArtifactsDb(artifactsDb), WithSpanSize(spanSize), WithMinLayerSize(tc.minLayerSize))
			ztoc, err := builder.buildSociLayer(ctx, desc)
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

func TestDisableXattrs(t *testing.T) {
	testcases := []struct {
		name                string
		metadata            []ztoc.FileMetadata
		shouldDisableXattrs bool
	}{
		{
			name: "ztoc with xattrs should not have xattrs disabled",
			metadata: []ztoc.FileMetadata{
				{
					Name: "Xattrs",
					PAXHeaders: map[string]string{
						"SCHILY.xattr.user.any": "true",
					},
				},
			},
			shouldDisableXattrs: false,
		},
		{
			name: "ztoc with opaque dirs should not have xattrs disabled",
			metadata: []ztoc.FileMetadata{
				{
					Name: "dir/",
				},
				{
					Name: "dir/" + whiteoutOpaqueDir,
				},
			},
			shouldDisableXattrs: false,
		},
		{
			name: "ztoc with no xattrs or opaque dirs should have xattrs disabled",
			metadata: []ztoc.FileMetadata{
				{
					Name: "dir/",
				},
				{
					Name: "dir/file",
				},
			},
			shouldDisableXattrs: true,
		},
	}
	for _, tc := range testcases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			desc := ocispec.Descriptor{
				MediaType:   "application/vnd.oci.image.layer.",
				Size:        1,
				Annotations: make(map[string]string, 1),
			}
			ztoc := ztoc.Ztoc{
				TOC: ztoc.TOC{
					FileMetadata: tc.metadata,
				},
			}

			cs := newFakeContentStore()
			blobStore := NewOrasMemoryStore()
			artifactsDb, err := newTestableDb()
			if err != nil {
				t.Fatalf("can't create a test db")
			}
			builder, _ := NewIndexBuilder(cs, blobStore, WithArtifactsDb(artifactsDb), WithOptimizations([]Optimization{XAttrOptimization}))
			builder.maybeAddDisableXattrAnnotation(&desc, &ztoc)
			disableXAttrs := desc.Annotations[IndexAnnotationDisableXAttrs] == disableXAttrsTrue
			if disableXAttrs != tc.shouldDisableXattrs {
				t.Fatalf("expected xattrs to be disabled = %v, actual = %v", tc.shouldDisableXattrs, disableXAttrs)
			}
		})
	}
}

func TestNewIndex(t *testing.T) {
	testcases := []struct {
		name        string
		blobs       []ocispec.Descriptor
		subject     ocispec.Descriptor
		annotations map[string]string
	}{
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
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			index := NewIndex(V1, tc.blobs, &tc.subject, tc.annotations)

			if diff := cmp.Diff(index.Blobs, tc.blobs); diff != "" {
				t.Fatalf("unexpected blobs; diff = %v", diff)
			}

			if index.ArtifactType != SociIndexArtifactType {
				t.Fatalf("unexpected artifact type; expected = %s, got = %s", SociIndexArtifactType, index.ArtifactType)
			}

			if index.MediaType != ocispec.MediaTypeImageManifest {
				t.Fatalf("unexpected media type; expected = %v, got = %v", ocispec.MediaTypeImageManifest, index.MediaType)
			}

			if diff := cmp.Diff(index.Subject, &tc.subject); diff != "" {
				t.Fatalf("the subject field is not equal; diff = %v", diff)
			}
		})
	}
}

func TestDecodeIndex(t *testing.T) {
	testcases := []struct {
		name        string
		blobs       []ocispec.Descriptor
		subject     ocispec.Descriptor
		annotations map[string]string
	}{
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
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			index := NewIndex(V1, tc.blobs, &tc.subject, tc.annotations)
			jsonBytes, err := MarshalIndex(index)
			if err != nil {
				t.Fatalf("cannot convert index to json byte data: %v", err)
			}
			var index2 Index
			err = DecodeIndex(bytes.NewReader(jsonBytes), &index2)
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(index, &index2); diff != "" {
				t.Fatalf("unexpected index after deserialzing from byte data; diff = %v", diff)
			}
		})
	}
}

func TestMarshalIndex(t *testing.T) {
	blobs := []ocispec.Descriptor{
		{
			Size:   4,
			Digest: digest.FromBytes([]byte("test")),
		},
	}

	subject := ocispec.Descriptor{
		Size:   4,
		Digest: digest.FromBytes([]byte("test")),
	}

	annotations := map[string]string{
		"foo": "bar",
	}

	testcases := []struct {
		name  string
		index *Index
		ty    interface{}
	}{
		{
			name:  "successfully roundtrip a v1 SOCI index",
			index: NewIndex(V1, blobs, &subject, annotations),
			ty:    ocispec.Manifest{},
		},
		{
			name:  "successfully roundtrip a v2 SOCI index",
			index: NewIndex(V2, blobs, nil, annotations),
			ty:    ocispec.Manifest{},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			b, err := MarshalIndex(tc.index)
			if err != nil {
				t.Fatalf("could not marshal index: %v", err)
			}
			err = json.Unmarshal(b, &tc.ty)
			if err != nil {
				t.Fatalf("could not unmarshal index as underlying type: %v", err)
			}
			var unmarshalled Index
			err = UnmarshalIndex(b, &unmarshalled)
			if err != nil {
				t.Fatalf("could not unmarshal index as index: %v", err)
			}
			diff := cmp.Diff(tc.index, &unmarshalled)
			if diff != "" {
				t.Fatalf("deserialized index does not match original index: %s", diff)
			}
		})
	}
}
