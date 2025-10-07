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
	"fmt"
	"math"
	"path/filepath"
	"strings"
	"testing"

	"github.com/awslabs/soci-snapshotter/config"
	"github.com/awslabs/soci-snapshotter/soci"
	"github.com/awslabs/soci-snapshotter/soci/store"
	"github.com/awslabs/soci-snapshotter/util/testutil"
	"github.com/containerd/platforms"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

func TestCreateWithForceRecreateZtocs(t *testing.T) {
	sh, done := newSnapshotterBaseShell(t)
	defer done()
	rebootContainerd(t, sh, "", "")

	// if we build an index for the image first and then create it again
	// without the --force flag, all ztoc's should be skipped
	image := dockerhub(nginxAlpineImage)
	indexDigest := buildIndex(sh, image)
	if indexDigest == "" {
		t.Fatal("failed to get soci index for test image")
	}
	contentStoreBlobPath, _ := testutil.GetContentStoreBlobPath(config.DefaultContentStoreType)
	parsedDigest, err := digest.Parse(indexDigest)
	if err != nil {
		t.Fatalf("cannot parse digest: %v", err)
	}
	checkpoints := fetchContentFromPath(sh, filepath.Join(contentStoreBlobPath, parsedDigest.Encoded()))
	var index soci.Index
	err = soci.DecodeIndex(bytes.NewReader(checkpoints), &index)
	if err != nil {
		t.Fatalf("cannot get index data: %v", err)
	}

	testCases := []struct {
		name                   string
		forceRecreateZtocsFlag bool
		numZtocSkipped         int
	}{
		{
			name:                   "test soci create without --force flag",
			forceRecreateZtocsFlag: false,
			numZtocSkipped:         len(index.Blobs),
		},
		{
			name:                   "test soci create with --force flag",
			forceRecreateZtocsFlag: true,
			numZtocSkipped:         0,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			args := []string{"soci", "create", "--min-layer-size=0", "--platform", platforms.Format(image.platform)}
			if tc.forceRecreateZtocsFlag {
				args = append(args, "--force")
			}
			args = append(args, image.ref)
			b, err := sh.CombinedOLog(args...)
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}

			numSkipped := strings.Count(string(b), "already exists")
			if numSkipped != tc.numZtocSkipped {
				t.Fatalf("expected %v ztoc to be skipped, got %v", tc.numZtocSkipped, numSkipped)
			}
		})
	}
}

func TestCreateConvertParameterValidation(t *testing.T) {
	tests := []struct {
		name          string
		minLayerSize  string
		spanSize      string
		expectedError string
	}{
		{
			name:          "minLayerSize < 0 fails validation",
			minLayerSize:  "-1",
			spanSize:      "0",
			expectedError: "min layer size must be >= 0",
		},
		{
			name:          "spanSize < 0 fails validation",
			minLayerSize:  "0",
			spanSize:      "-1",
			expectedError: "span size must be >= 0",
		},
		{
			name:          "minLayerSize > int64.MaxValue fails validation",
			minLayerSize:  fmt.Sprintf("%d", uint64(math.MaxInt64)+1),
			spanSize:      "0",
			expectedError: fmt.Sprintf(`invalid value "%d" for flag -min-layer-size`, uint64(math.MaxInt64)+1),
		},
		{
			name:          "spanSize > int64.MaxValue fails validation",
			minLayerSize:  "0",
			spanSize:      fmt.Sprintf("%d", uint64(math.MaxInt64)+1),
			expectedError: fmt.Sprintf(`invalid value "%d" for flag -span-size`, uint64(math.MaxInt64)+1),
		},
	}

	sh, done := newSnapshotterBaseShell(t)
	defer done()
	rebootContainerd(t, sh, "", "")
	image := dockerhub(alpineImage)
	sh.X("nerdctl", "pull", "-q", image.ref)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Run("create", func(t *testing.T) {
				b, err := sh.CombinedOLog("soci", "create", "--min-layer-size", test.minLayerSize, "--span-size", test.spanSize, image.ref)
				if !strings.Contains(string(b), test.expectedError) {
					t.Fatalf("expected error to contain %q, got %q", test.expectedError, string(b))
				}
				if err == nil {
					t.Fatal("expected error but got nil")
				}
			})

			t.Run("convert", func(t *testing.T) {
				b, err := sh.CombinedOLog("soci", "convert", "--min-layer-size", test.minLayerSize, "--span-size", test.spanSize, image.ref, image.ref)
				if !strings.Contains(string(b), test.expectedError) {
					t.Fatalf("convert: expected error to contain %q, got %q", test.expectedError, string(b))
				}
				if err == nil {
					t.Fatal("convert: expected error but got nil")
				}
			})
		})
	}
}

func TestSociCreateEmptyIndex(t *testing.T) {
	sh, done := newSnapshotterBaseShell(t)
	defer done()

	rebootContainerd(t, sh, "", "")
	imgInfo := dockerhub(alpineImage)
	indexDigest := buildIndex(sh, imgInfo, withMinLayerSize(1000000000), withAllowErrors)
	if indexDigest != "" {
		t.Fatal("index was created despite all layers being smaller than min layer size")
	}
}

func TestSociCreateSparseIndex(t *testing.T) {
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
			name:         "test create for rethinkdb:latest with min-layer-size 10000000 bytes",
			minLayerSize: 10000000,
		},
	}

	const manifestDigest = "sha256:4452aadba3e99771ff3559735dab16279c5a352359d79f38737c6fdca941c6e5"
	const containerImage = "rethinkdb@" + manifestDigest

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sh, done := newSnapshotterBaseShell(t)
			defer done()

			rebootContainerd(t, sh, "", "")
			imgInfo := dockerhub(containerImage)
			indexDigest := buildIndex(sh, imgInfo, withMinLayerSize(tt.minLayerSize))
			var index soci.Index
			contentStoreBlobPath, _ := testutil.GetContentStoreBlobPath(config.DefaultContentStoreType)
			if indexDigest != "" {
				dgst, err := digest.Parse(indexDigest)
				if err != nil {
					t.Fatalf("cannot parse digest: %v", err)
				}
				checkpoints := fetchContentFromPath(sh, filepath.Join(contentStoreBlobPath, dgst.Encoded()))

				err = soci.DecodeIndex(bytes.NewReader(checkpoints), &index)
				if err != nil {
					t.Fatalf("cannot get index data: %v", err)
				}
			}

			imageManifestJSON, err := FetchContentByDigest(sh, store.ContainerdContentStoreType, manifestDigest)
			if err != nil {
				t.Fatalf("cannot fetch content %s: %v", manifestDigest, err)
			}

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

			if indexDigest == "" {
				if len(includedLayers) > 0 {
					t.Fatalf("failed to validate soci index: unexpected layer count; expected=%v, got=0", len(includedLayers))
				}
			} else {
				if err := validateSociIndex(sh, config.DefaultContentStoreType, index, manifestDigest, includedLayers); err != nil {
					t.Fatalf("failed to validate soci index: %v", err)
				}
			}
		})
	}
}

func TestSociCreate(t *testing.T) {
	sh, done := newSnapshotterBaseShell(t)
	defer done()

	tests := []struct {
		name             string
		containerImage   string
		platform         string
		contentStoreType store.ContentStoreType
	}{
		{
			name:           "test create for nginx",
			containerImage: nginxImage,
		},
		{
			name:           "test create for alpine",
			containerImage: alpineImage,
		},
		// The following two tests guarantee that we have tested both content
		// stores
		{
			name:             "test create for drupal on soci content store",
			containerImage:   drupalImage,
			contentStoreType: store.SociContentStoreType,
		},
		{
			name:             "test create for drupal on containerd content store",
			containerImage:   drupalImage,
			contentStoreType: store.ContainerdContentStoreType,
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
			rebootContainerd(t, sh, "", getSnapshotterConfigToml(t, withContentStoreConfig(store.WithType(tt.contentStoreType))))
			platform := platforms.DefaultSpec()
			if tt.platform != "" {
				var err error
				platform, err = platforms.Parse(tt.platform)
				if err != nil {
					t.Fatalf("could not parse platform: %v", err)
				}
			}
			imgInfo := dockerhub(tt.containerImage, withPlatform(platform))
			indexDigest := buildIndex(sh, imgInfo, withMinLayerSize(0), withContentStoreType(tt.contentStoreType))
			contentStoreBlobPath, err := testutil.GetContentStoreBlobPath(tt.contentStoreType)
			if err != nil {
				t.Fatalf("cannot get content store path: %v", err)
			}
			dgst, err := digest.Parse(indexDigest)
			if err != nil {
				t.Fatalf("cannot parse digest: %v", err)
			}
			checkpoints := fetchContentFromPath(sh, filepath.Join(contentStoreBlobPath, dgst.Encoded()))
			var sociIndex soci.Index
			err = soci.DecodeIndex(bytes.NewReader(checkpoints), &sociIndex)
			if err != nil {
				t.Fatalf("cannot get soci index: %v", err)
			}

			m, err := getManifestDigest(sh, imgInfo.ref, platform)
			if err != nil {
				t.Fatalf("failed to get manifest digest: %v", err)
			}

			if err := validateSociIndex(sh, tt.contentStoreType, sociIndex, m, nil); err != nil {
				t.Fatalf("failed to validate soci index: %v", err)
			}
		})
	}
}

// TestSociCreateGarbageCollection ensures that SOCI index artifacts are not garbage collected after creation.
func TestSociCreateGarbageCollection(t *testing.T) {
	image := rabbitmqImage
	smallImage := alpineImage
	sh, done := newSnapshotterBaseShell(t)
	defer done()

	extraContainerdConfig := `
[plugins."io.containerd.gc.v1.scheduler"]
	deletion_threshold = 1`

	rebootContainerd(t, sh, getContainerdConfigToml(t, false, extraContainerdConfig), getSnapshotterConfigToml(t, withTCPMetrics, withContentStoreConfig(store.WithType(config.ContainerdContentStoreType))))

	imgInfo := dockerhub(image)
	sh.X("nerdctl", "pull", "-q", imgInfo.ref)
	indexDigest := buildIndex(sh, imgInfo, withMinLayerSize(0), withContentStoreType(config.ContainerdContentStoreType))

	// Pull and remove a small image to automatically trigger GC.
	smallImageInfo := dockerhub(smallImage)
	sh.X("nerdctl", "pull", "-q", smallImageInfo.ref)
	sh.X("nerdctl", "rmi", smallImageInfo.ref)

	sh.X(append(imagePullCmd, "--soci-index-digest", indexDigest, imgInfo.ref)...)
	curlOutput := string(sh.O("curl", tcpMetricsAddress+metricsPath))
	if err := checkOverlayFallbackCount(curlOutput, 0); err != nil {
		t.Fatal(fmt.Errorf("resources unexpectedly garbage collected: %v", err))
	}
}

func TestSociImageGCLabel(t *testing.T) {
	image := rabbitmqImage
	sh, done := newSnapshotterBaseShell(t)
	defer done()

	extraContainerdConfig := `
[plugins."io.containerd.gc.v1.scheduler"]
	deletion_threshold = 1`

	rebootContainerd(t, sh, getContainerdConfigToml(t, false, extraContainerdConfig), getSnapshotterConfigToml(t, withTCPMetrics, withContentStoreConfig(store.WithType(config.ContainerdContentStoreType))))

	imgInfo := dockerhub(image)
	sh.X("nerdctl", "pull", "-q", imgInfo.ref)
	indexDigest := buildIndex(sh, imgInfo, withMinLayerSize(0), withContentStoreType(config.ContainerdContentStoreType))

	// This should succeed because the index should still exist
	sh.X("ctr", "content", "get", indexDigest)

	sh.X("nerdctl", "rmi", imgInfo.ref)

	// This should fail because the index should be GC'd
	o, err := sh.CombinedOLog("ctr", "content", "get", indexDigest)
	if !strings.Contains(string(o), "not found") {
		t.Fatal("getting the SOCI index succeeded unexpectedly after GC")
	}
	if err == nil {
		t.Fatal("getting the SOCI index after GC did not return an error")
	}

}
