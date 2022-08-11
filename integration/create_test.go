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

package integration

import (
	"encoding/json"
	"testing"

	"github.com/awslabs/soci-snapshotter/soci"
	shell "github.com/awslabs/soci-snapshotter/util/dockershell"
	"github.com/google/go-cmp/cmp"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	artifactsspec "github.com/oras-project/artifacts-spec/specs-go/v1"
)

const blobStorePath = "/var/lib/soci-snapshotter-grpc/content/blobs/sha256"

func TestSociCreateSparseIndex(t *testing.T) {
	sh, done := newSnapshotterBaseShell(t)
	defer done()

	rebootContainerd(t, sh, "", "")

	dockerhub := func(name string) imageInfo {
		return imageInfo{dockerLibrary + name, "", false}
	}
	tests := []struct {
		name         string
		minLayerSize int64
	}{
		{
			name:         "test create for rethinkdb:latest with min-layer-size 0 bytes",
			minLayerSize: 0,
		},
		{
			name:         "test create for rethinkdb:latest with min-layer-size 1000000 bytes",
			minLayerSize: 1000000,
		},
		{
			name:         "test create for rethinkdb:latest with min-layer-size 6000000 bytes",
			minLayerSize: 10000000,
		},
		{
			name:         "test create for rethinkdb:latest with min-layer-size 10000000 bytes",
			minLayerSize: 100000000,
		},
	}

	const containerImage = "rethinkdb@sha256:4452aadba3e99771ff3559735dab16279c5a352359d79f38737c6fdca941c6e5"
	const manifestDigest = "sha256:4452aadba3e99771ff3559735dab16279c5a352359d79f38737c6fdca941c6e5"

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rebootContainerd(t, sh, "", "")
			imgInfo := dockerhub(containerImage)
			indexDigest := buildSparseIndex(sh, imgInfo, tt.minLayerSize)
			indexByteData := fetchContentFromPath(sh, blobStorePath+"/"+trimSha256Prefix(indexDigest))
			sociIndex := new(soci.SociIndex)
			if err := json.Unmarshal(indexByteData, sociIndex); err != nil {
				t.Fatalf("cannot unmarshal soci index byte data: %v", err)
			}

			if sociIndex.MediaType != artifactsspec.MediaTypeArtifactManifest {
				t.Fatalf("unexpected index media type; expected = %v, got = %v", artifactsspec.MediaTypeArtifactManifest, sociIndex.MediaType)
			}

			if sociIndex.ArtifactType != soci.SociIndexArtifactType {
				t.Fatalf("unexpected index artifact type; expected = %v, got = %v", soci.SociIndexArtifactType, sociIndex.ArtifactType)
			}

			expectedAnnotations := map[string]string{
				soci.IndexAnnotationBuildToolIdentifier: "AWS SOCI CLI",
				soci.IndexAnnotationBuildToolVersion:    "0.1",
			}

			if diff := cmp.Diff(sociIndex.Annotations, expectedAnnotations); diff != "" {
				t.Fatalf("unexpected index annotations; diff = %v", diff)
			}

			imageManifestJSON := fetchContentByDigest(sh, manifestDigest)
			imageManifest := new(ocispec.Manifest)
			if err := json.Unmarshal(imageManifestJSON, imageManifest); err != nil {
				t.Fatalf("cannot unmarshal index manifest: %v", err)
			}

			includedLayers := make(map[int]ocispec.Descriptor)
			for i, layerBlob := range imageManifest.Layers {
				if layerBlob.Size >= tt.minLayerSize {
					includedLayers[i] = layerBlob
				}
			}

			blobs := sociIndex.Blobs
			notNilBlobCount := 0
			for _, b := range blobs {
				if b != nil {
					notNilBlobCount++
				}
			}
			if notNilBlobCount != len(includedLayers) {
				t.Fatalf("unexpected blob count; expected=%v, got=%v", len(includedLayers), notNilBlobCount)
			}

			for i, blob := range blobs {
				if blob == nil {
					continue
				}
				blob := *blob

				blobContent := fetchContentFromPath(sh, blobStorePath+"/"+trimSha256Prefix(blob.Digest.String()))
				blobSize := int64(len(blobContent))
				blobDigest := digest.FromBytes(blobContent)
				layerDigest := blob.Annotations[soci.IndexAnnotationImageLayerDigest]
				if layerDigest != includedLayers[i].Digest.String() {
					t.Fatalf("unexpected layer digest; expected=%s, got=%s", includedLayers[i].Digest.String(), layerDigest)
				}

				if blobSize != blob.Size {
					t.Fatalf("unexpected blob size; expected = %v, got = %v", blob.Size, blobSize)
				}

				if blobDigest != blob.Digest {
					t.Fatalf("unexpected blob digest; expected = %v, got = %v", blob.Digest, blobDigest)
				}
			}
		})
	}
}

func TestSociCreate(t *testing.T) {
	sh, done := newSnapshotterBaseShell(t)
	defer done()

	rebootContainerd(t, sh, "", "")

	dockerhub := func(name string) imageInfo {
		return imageInfo{dockerLibrary + name, "", false}
	}

	tests := []struct {
		name           string
		containerImage string
	}{
		{
			name:           "test create for ubuntu:latest",
			containerImage: "ubuntu:latest",
		},
		{
			name:           "test create for nginx:latest",
			containerImage: "nginx:latest",
		},
		{
			name:           "test create for alpine:latest",
			containerImage: "alpine:latest",
		},
		{
			name:           "test create for drupal:latest",
			containerImage: "drupal:latest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rebootContainerd(t, sh, "", "")
			imgInfo := dockerhub(tt.containerImage)
			indexDigest := optimizeImage(sh, imgInfo)
			indexByteData := fetchContentFromPath(sh, blobStorePath+"/"+trimSha256Prefix(indexDigest))
			sociIndex := new(soci.SociIndex)
			if err := json.Unmarshal(indexByteData, sociIndex); err != nil {
				t.Fatalf("cannot unmarshal soci index byte data: %v", err)
			}

			if sociIndex.MediaType != artifactsspec.MediaTypeArtifactManifest {
				t.Fatalf("unexpected index media type; expected = %v, got = %v", artifactsspec.MediaTypeArtifactManifest, sociIndex.MediaType)
			}

			if sociIndex.ArtifactType != soci.SociIndexArtifactType {
				t.Fatalf("unexpected index artifact type; expected = %v, got = %v", soci.SociIndexArtifactType, sociIndex.ArtifactType)
			}

			expectedAnnotations := map[string]string{
				soci.IndexAnnotationBuildToolIdentifier: "AWS SOCI CLI",
				soci.IndexAnnotationBuildToolVersion:    "0.1",
			}

			if diff := cmp.Diff(sociIndex.Annotations, expectedAnnotations); diff != "" {
				t.Fatalf("unexpected index annotations; diff = %v", diff)
			}

			blobs := sociIndex.Blobs

			for _, blob := range blobs {
				if blob == nil {
					continue
				}
				blob := *blob

				blobContent := fetchContentFromPath(sh, blobStorePath+"/"+trimSha256Prefix(blob.Digest.String()))
				blobSize := int64(len(blobContent))
				blobDigest := digest.FromBytes(blobContent)

				if blobSize != blob.Size {
					t.Fatalf("unexpected blob size; expected = %v, got = %v", blob.Size, blobSize)
				}

				if blobDigest != blob.Digest {
					t.Fatalf("unexpected blob digest; expected = %v, got = %v", blob.Digest, blobDigest)
				}
			}
		})
	}
}

func fetchContentFromPath(sh *shell.Shell, path string) []byte {
	return sh.O("cat", path)
}

func fetchContentByDigest(sh *shell.Shell, digest string) []byte {
	return sh.O("ctr", "content", "get", digest)
}
