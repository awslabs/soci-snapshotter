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
	"bytes"
	"encoding/json"
	"testing"

	"github.com/awslabs/soci-snapshotter/soci"
	"github.com/containerd/containerd/platforms"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

func TestSociCreateSparseIndex(t *testing.T) {
	sh, done := newSnapshotterBaseShell(t)
	defer done()

	rebootContainerd(t, sh, "", "")
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
			indexDigest := buildIndex(sh, imgInfo, withMinLayerSize(tt.minLayerSize))
			checkpoints := fetchContentFromPath(sh, blobStorePath+"/"+trimSha256Prefix(indexDigest))

			var index soci.Index
			err := soci.DecodeIndex(bytes.NewReader(checkpoints), &index)
			if err != nil {
				t.Fatalf("cannot get index data: %v", err)
			}

			imageManifestJSON := fetchContentByDigest(sh, manifestDigest)
			imageManifest := new(ocispec.Manifest)
			if err := json.Unmarshal(imageManifestJSON, imageManifest); err != nil {
				t.Fatalf("cannot unmarshal index manifest: %v", err)
			}

			includedLayers := make(map[string]struct{})
			for _, layer := range imageManifest.Layers {
				if layer.Size >= tt.minLayerSize {
					includedLayers[layer.Digest.String()] = struct{}{}
				}
			}

			if err := validateSociIndex(sh, index, manifestDigest, includedLayers); err != nil {
				t.Fatalf("failed to validate soci index: %v", err)
			}
		})
	}
}

func TestSociCreate(t *testing.T) {
	sh, done := newSnapshotterBaseShell(t)
	defer done()

	rebootContainerd(t, sh, "", "")

	tests := []struct {
		name           string
		containerImage string
		platform       string
	}{
		{
			name:           "test create for ubuntu",
			containerImage: ubuntuImage,
		},
		{
			name:           "test create for nginx",
			containerImage: nginxImage,
		},
		{
			name:           "test create for alpine",
			containerImage: alpineImage,
		},
		{
			name:           "test create for drupal",
			containerImage: drupalImage,
		},
		// The following two tests guarantee that we have tested at least
		// 2 different platforms. Depending on what host they run on, one
		// might be a duplicate of the earlier test using the default platform
		{
			name:           "test create for ubuntu amd64",
			containerImage: ubuntuImage,
			platform:       "linux/amd64",
		},
		{
			name:           "test create for ubuntu arm64",
			containerImage: ubuntuImage,
			platform:       "linux/arm64",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rebootContainerd(t, sh, "", "")
			platform := platforms.DefaultSpec()
			if tt.platform != "" {
				var err error
				platform, err = platforms.Parse(tt.platform)
				if err != nil {
					t.Fatalf("could not parse platform: %v", err)
				}
			}
			imgInfo := dockerhub(tt.containerImage, withPlatform(platform))
			indexDigest := buildIndex(sh, imgInfo, withMinLayerSize(0))
			checkpoints := fetchContentFromPath(sh, blobStorePath+"/"+trimSha256Prefix(indexDigest))
			var sociIndex soci.Index
			err := soci.DecodeIndex(bytes.NewReader(checkpoints), &sociIndex)
			if err != nil {
				t.Fatalf("cannot get soci index: %v", err)
			}

			m, err := getManifestDigest(sh, imgInfo.ref, platform)
			if err != nil {
				t.Fatalf("failed to get manifest digest: %v", err)
			}

			if err := validateSociIndex(sh, sociIndex, m, nil); err != nil {
				t.Fatalf("failed to validate soci index: %v", err)
			}
		})
	}
}
