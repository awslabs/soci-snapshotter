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
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/awslabs/soci-snapshotter/config"
	"github.com/awslabs/soci-snapshotter/soci"
	"github.com/awslabs/soci-snapshotter/soci/store"

	"github.com/containerd/platforms"
)

func TestSociArtifactsPushAndPull(t *testing.T) {
	regConfig := newRegistryConfig()
	sh, done := newShellWithRegistry(t, regConfig)
	defer done()

	tests := []struct {
		Name     string
		Platform string
	}{
		{
			Name:     "amd64",
			Platform: "linux/amd64",
		},
		{
			Name:     "arm64",
			Platform: "linux/arm64",
		},
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			rebootContainerd(t, sh, getContainerdConfigToml(t, false), getSnapshotterConfigToml(t))

			platform, err := platforms.Parse(tt.Platform)
			if err != nil {
				t.Fatalf("could not parse platform %s: %v", tt.Platform, err)
			}

			imageName := ubuntuImage
			copyImage(sh, dockerhub(imageName, withPlatform(platform)), regConfig.mirror(imageName, withPlatform(platform)))
			indexDigest := buildIndex(sh, regConfig.mirror(imageName, withPlatform(platform)), withMinLayerSize(0))
			artifactsStoreContentDigest, err := getSociLocalStoreContentDigest(sh, config.DefaultContentStoreType)
			if err != nil {
				t.Fatalf("could not get digest of local content store: %v", err)
			}

			sh.X("soci", "push", "--user", regConfig.creds(), "--platform", tt.Platform, regConfig.mirror(imageName).ref)
			sh.X("rm", "-rf", filepath.Join(store.DefaultSociContentStorePath, "blobs", "sha256"))
			sh.X(append(imagePullCmd, "--soci-index-digest", indexDigest, "--platform", tt.Platform, regConfig.mirror(imageName).ref)...)

			artifactsStoreContentDigestAfterRPull, err := getSociLocalStoreContentDigest(sh, config.DefaultContentStoreType)
			if err != nil {
				t.Fatalf("could not get digest of local content store: %v", err)
			}

			if artifactsStoreContentDigest != artifactsStoreContentDigestAfterRPull {
				t.Fatalf("unexpected digests before and after rpull; before = %v, after = %v", artifactsStoreContentDigest, artifactsStoreContentDigestAfterRPull)
			}
		})
	}
}

func TestPushAlwaysMostRecentlyCreatedIndex(t *testing.T) {
	regConfig := newRegistryConfig()
	sh, done := newShellWithRegistry(t, regConfig)
	defer done()

	type buildOpts struct {
		spanSize     int64
		minLayerSize int64
	}

	testCases := []struct {
		name string
		ref  string
		opts []buildOpts
	}{
		{
			name: "rabbitmq",
			// Pinning a specific image, so that this test is guaranteed to fail in case of any regressions.
			ref: pinnedRabbitmqImage,
			opts: []buildOpts{
				{
					spanSize:     1 << 22,  // 4MiB
					minLayerSize: 10 << 20, // 10MiB
				},
				{
					spanSize:     128000,
					minLayerSize: 10 << 20,
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rebootContainerd(t, sh, getContainerdConfigToml(t, false), getSnapshotterConfigToml(t))

			imgInfo := regConfig.mirror(tc.ref)
			copyImage(sh, dockerhub(tc.ref), imgInfo)
			pushedPlatformDigest, _ := sh.OLog("nerdctl", "image", "convert", "--platform",
				platforms.Format(imgInfo.platform), imgInfo.ref, "test")
			imgInfo.ref = fmt.Sprintf("%s/%s@%s", regConfig.host, tc.name, strings.TrimSpace(string(pushedPlatformDigest)))
			for _, opt := range tc.opts {
				index := buildIndex(sh, imgInfo, withMinLayerSize(opt.minLayerSize), withSpanSize(opt.spanSize))
				index = strings.Split(index, "\n")[0]
				out := sh.O("soci", "push", "--existing-index", "allow", "--user", regConfig.creds(), imgInfo.ref, "-q")
				pushedIndex := strings.Trim(string(out), "\n")
				if index != pushedIndex {
					t.Fatalf("incorrect index pushed to remote registry; expected %s, got %s", index, pushedIndex)
				}
			}
		})
	}
}

func TestLegacyOCI(t *testing.T) {
	tests := []struct {
		name          string
		registryImage string
		expectError   bool
	}{
		{
			name:          "OCI 1.0 Artifacts succeed with OCI 1.0 registry",
			registryImage: oci10RegistryImage,
			expectError:   false,
		},
		{
			name:          "OCI 1.0 Artifacts succeed with OCI 1.1 registry",
			registryImage: oci11RegistryImage,
			expectError:   false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			regConfig := newRegistryConfig()
			sh, done := newShellWithRegistry(t, regConfig, withRegistryImageRef(tc.registryImage))
			defer done()

			rebootContainerd(t, sh, getContainerdConfigToml(t, false), getSnapshotterConfigToml(t))

			imageName := ubuntuImage
			copyImage(sh, dockerhub(imageName), regConfig.mirror(imageName))

			indexDigest := buildIndex(sh, regConfig.mirror(imageName))
			rawJSON := sh.O("soci", "index", "info", indexDigest)
			var sociIndex soci.Index
			if err := soci.UnmarshalIndex(rawJSON, &sociIndex); err != nil {
				t.Fatalf("invalid soci index from digest %s: %v", indexDigest, rawJSON)
			}
			_, err := sh.OLog("soci", "push", "--user", regConfig.creds(), regConfig.mirror(imageName).ref)
			hasError := err != nil
			if hasError != tc.expectError {
				t.Fatalf("unexpected error state: expected error? %v, got %v", tc.expectError, err)
			} else if hasError {
				// if we have an error and we expected an error, the test is done
				return
			}
			sh.X("rm", "-rf", filepath.Join(store.DefaultSociContentStorePath, "blobs", "sha256"))

			sh.X(append(imagePullCmd, "--soci-index-digest", indexDigest, regConfig.mirror(imageName).ref)...)
			if err := sh.Err(); err != nil {
				t.Fatalf("failed to rpull: %v", err)
			}
			checkFuseMounts(t, sh, len(sociIndex.Blobs))
		})
	}
}

func TestPushWithExistingIndices(t *testing.T) {
	t.Parallel()
	regConfig := newRegistryConfig()
	sh, done := newShellWithRegistry(t, regConfig)
	defer done()

	rebootContainerd(t, sh, getContainerdConfigToml(t, false), getSnapshotterConfigToml(t))

	const (
		singleFoundMessage   = "soci index found in remote repository with digest:"
		multipleFoundMessage = "multiple soci indices found in remote repository:"
		skipMessageTail      = "skipping pushing artifacts for image manifest:"
		warnMessageHead      = "[WARN]"
		warnMessageTail      = "pushing index anyway"
	)

	images := []string{nginxImage, rabbitmqImage, drupalImage, ubuntuImage}
	imageToIndexDigest := make(map[string]string)
	imageToManifestDigest := make(map[string]string)

	for _, img := range images {
		mirrorImg := regConfig.mirror(img)
		copyImage(sh, dockerhub(img), mirrorImg)
		indexDigest := buildIndex(sh, mirrorImg)
		manifestDigest, err := getManifestDigest(sh, mirrorImg.ref, mirrorImg.platform)
		if err != nil {
			t.Fatal(err)
		}
		imageToIndexDigest[img] = indexDigest
		imageToManifestDigest[mirrorImg.ref] = manifestDigest

		sh.X("soci", "push", "--user", regConfig.creds(), mirrorImg.ref)
		if img == ubuntuImage {
			buildIndex(sh, mirrorImg, withSpanSize(1280))
			sh.X("soci", "push", "--user", regConfig.creds(), mirrorImg.ref)
		}

	}

	tests := []struct {
		name               string
		imgInfo            imageInfo
		imgName            string
		cmd                []string
		hasOutput          bool
		outputContains     string
		expectedIndexCount int
	}{
		{
			name:               "Warn with existing index",
			imgInfo:            regConfig.mirror(nginxImage),
			imgName:            "nginx",
			cmd:                []string{"soci", "push", "--user", regConfig.creds(), "--existing-index", "warn"},
			hasOutput:          true,
			outputContains:     fmt.Sprintf("%s %s %s: %s", warnMessageHead, singleFoundMessage, imageToIndexDigest[nginxImage], warnMessageTail),
			expectedIndexCount: 2,
		},
		{
			name:               "Skip with existing index",
			imgInfo:            regConfig.mirror(rabbitmqImage),
			imgName:            "rabbitmq",
			cmd:                []string{"soci", "push", "--user", regConfig.creds(), "--existing-index", "skip"},
			hasOutput:          true,
			outputContains:     fmt.Sprintf("%s %s: %s %s", singleFoundMessage, imageToIndexDigest[rabbitmqImage], skipMessageTail, imageToManifestDigest[regConfig.mirror(rabbitmqImage).ref]),
			expectedIndexCount: 1,
		},

		{
			name:               "Allow with existing index",
			imgInfo:            regConfig.mirror(drupalImage),
			imgName:            "drupal",
			cmd:                []string{"soci", "push", "--user", regConfig.creds(), "--existing-index", "allow"},
			expectedIndexCount: 2,
		},
		{
			name:               "Warn with multiple existing indices",
			imgInfo:            regConfig.mirror(ubuntuImage),
			imgName:            "ubuntu",
			cmd:                []string{"soci", "push", "--user", regConfig.creds(), "--existing-index", "warn"},
			hasOutput:          true,
			outputContains:     fmt.Sprintf("%s %s %s", warnMessageHead, multipleFoundMessage, warnMessageTail),
			expectedIndexCount: 3,
		},
	}
	verifyOutput := func(given, expected string) error {
		if !strings.Contains(given, expected) {
			return fmt.Errorf("output: %s does not contain substring %s", given, expected)
		}
		return nil
	}
	verifyIndexCount := func(imgName, digest string, expected int) error {
		index, err := getReferrers(sh, regConfig, imgName, digest)
		if err != nil {
			return err
		}
		if len(index.Manifests) != expected {
			return fmt.Errorf("unexpected index count in remote: expected: %v; got: %v", expected, len(index.Manifests))
		}
		return nil
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc := tc
			digest := imageToManifestDigest[tc.imgInfo.ref]
			buildIndex(sh, tc.imgInfo, withSpanSize(1<<19))
			tc.cmd = append(tc.cmd, tc.imgInfo.ref)
			output, err := sh.OLog(tc.cmd...)
			if err != nil {
				t.Fatalf("unexpected error for test case: %v", err)
			}
			if tc.hasOutput {
				if err = verifyOutput(string(output), tc.outputContains); err != nil {
					t.Fatal(err)
				}
			}
			if err = verifyIndexCount(tc.imgName, strings.TrimSpace(digest), tc.expectedIndexCount); err != nil {
				t.Fatal(err)
			}
		})
	}
}
