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
	"context"
	"testing"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

var (
	linuxX86 = &ocispec.Platform{
		Architecture: "amd64",
		OS:           "linux",
	}

	imageDesc = ocispec.Descriptor{
		MediaType: ocispec.MediaTypeImageManifest,
		Digest:    "sha256:1234",
		Size:      1234,
		Platform:  linuxX86,
	}

	sociIndexDesc = ocispec.Descriptor{
		MediaType:    ocispec.MediaTypeImageManifest,
		ArtifactType: SociIndexArtifactTypeV2,
		Digest:       "sha256:5678",
		Size:         5678,
	}

	sociIndexDesc2 = ocispec.Descriptor{
		MediaType:    ocispec.MediaTypeImageManifest,
		ArtifactType: SociIndexArtifactTypeV2,
		Digest:       "sha256:9999",
		Size:         9999,
	}

	sociIndexDescWithPlatform  = sociIndexDesc
	sociIndexDesc2WithPlatform = sociIndexDesc2
)

func init() {
	sociIndexDescWithPlatform.Platform = linuxX86
	sociIndexDesc2WithPlatform.Platform = linuxX86
}

func TestAddSociIndexes(t *testing.T) {
	tests := []struct {
		name        string
		ociIndex    ocispec.Index
		sociIndexes []IndexWithMetadata
		expected    ocispec.Index
	}{
		{
			name: "add soci index",
			ociIndex: ocispec.Index{
				MediaType: ocispec.MediaTypeImageIndex,
				Manifests: []ocispec.Descriptor{imageDesc},
			},
			sociIndexes: []IndexWithMetadata{
				{
					Desc:     sociIndexDesc,
					Platform: linuxX86,
				},
			},
			expected: ocispec.Index{
				MediaType: ocispec.MediaTypeImageIndex,
				Manifests: []ocispec.Descriptor{
					imageDesc,
					sociIndexDescWithPlatform,
				},
			},
		},
		{
			name: "add existing soci index to an oci index",
			ociIndex: ocispec.Index{
				MediaType: ocispec.MediaTypeImageIndex,
				Manifests: []ocispec.Descriptor{
					imageDesc,
					sociIndexDescWithPlatform,
				},
			},
			sociIndexes: []IndexWithMetadata{
				{
					Desc:     sociIndexDesc,
					Platform: linuxX86,
				},
			},
			expected: ocispec.Index{
				MediaType: ocispec.MediaTypeImageIndex,
				Manifests: []ocispec.Descriptor{
					imageDesc,
					sociIndexDescWithPlatform,
				},
			},
		},
		{
			name: "replace existing soci index in an oci index",
			ociIndex: ocispec.Index{
				MediaType: ocispec.MediaTypeImageIndex,
				Manifests: []ocispec.Descriptor{
					imageDesc,
					sociIndexDescWithPlatform,
				},
			},
			sociIndexes: []IndexWithMetadata{
				{
					Desc:     sociIndexDesc2,
					Platform: linuxX86,
				},
			},
			expected: ocispec.Index{
				MediaType: ocispec.MediaTypeImageIndex,
				Manifests: []ocispec.Descriptor{
					imageDesc,
					sociIndexDesc2WithPlatform,
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			builder := IndexBuilder{}
			actual := test.ociIndex
			var indexes []*IndexWithMetadata
			for _, index := range test.sociIndexes {
				indexes = append(indexes, &index)
			}
			err := builder.addSociIndexes(context.Background(), &actual, indexes)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if len(actual.Manifests) != len(test.expected.Manifests) {
				t.Errorf("expected %d manifests, got %d", len(test.expected.Manifests), len(actual.Manifests))
			}
			for i, manifest := range actual.Manifests {
				if manifest.Digest != test.expected.Manifests[i].Digest {
					t.Errorf("expected digest %s, got %s", test.expected.Manifests[i].Digest, manifest.Digest)
				}
				if manifest.MediaType != test.expected.Manifests[i].MediaType {
					t.Errorf("expected media type %s, got %s", test.expected.Manifests[i].MediaType, manifest.MediaType)
				}
				if manifest.ArtifactType != test.expected.Manifests[i].ArtifactType {
					t.Errorf("expected artifact type %s, got %s", test.expected.Manifests[i].ArtifactType, manifest.ArtifactType)
				}
				if manifest.Platform.Architecture != test.expected.Manifests[i].Platform.Architecture {
					t.Errorf("expected architecture %s, got %s", test.expected.Manifests[i].Platform.Architecture, manifest.Platform.Architecture)
				}
				if manifest.Platform.OS != test.expected.Manifests[i].Platform.OS {
					t.Errorf("expected OS %s, got %s", test.expected.Manifests[i].Platform.OS, manifest.Platform.OS)
				}
			}
		})
	}

}
