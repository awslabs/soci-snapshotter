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
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/awslabs/soci-snapshotter/config"
	"github.com/awslabs/soci-snapshotter/soci"
	shell "github.com/awslabs/soci-snapshotter/util/dockershell"
	"github.com/awslabs/soci-snapshotter/util/testutil"
	"github.com/containerd/platforms"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

var (
	convertImages = []string{nginxImage, rabbitmqImage, drupalImage, ubuntuImage}
)

func TestConvertWithForceRecreateZtocs(t *testing.T) {
	sh, done := newSnapshotterBaseShell(t)
	defer done()
	rebootContainerd(t, sh, "", "")

	// if we build an index for the image first and then convert it
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
			name:                   "test soci convert without --force flag",
			forceRecreateZtocsFlag: false,
			numZtocSkipped:         len(index.Blobs),
		},
		{
			name:                   "test soci convert with --force flag",
			forceRecreateZtocsFlag: true,
			numZtocSkipped:         0,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			args := []string{"soci", "convert", "--min-layer-size=0", "--platform", platforms.Format(image.platform)}
			if tc.forceRecreateZtocsFlag {
				args = append(args, "--force")
			}
			args = append(args, image.ref, image.ref+"-soci")
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

func validateConversion(t *testing.T, sh *shell.Shell, originalDigest, convertedDigest string) {
	t.Helper()

	if originalDigest == convertedDigest {
		t.Fatalf("conversion did not change the digest: %s", originalDigest)
	}

	var index ocispec.Index
	content := sh.O("ctr", "content", "get", convertedDigest)
	err := json.Unmarshal(content, &index)
	if err != nil {
		t.Fatalf("failed to decode index: %v", err)
	}
	var manifests []ocispec.Descriptor
	var sociIndexes []ocispec.Descriptor
	for _, manifest := range index.Manifests {
		switch manifest.ArtifactType {
		case soci.SociIndexArtifactTypeV2:
			sociIndexes = append(sociIndexes, manifest)
		case "":
			manifests = append(manifests, manifest)
		}
		// ignore unknown artifacts
	}

	// We can't verify that manifests and soci indexes are 1-1 because there might be other,
	// non-soci artifacts in the image (e.g. Docker attestation manifests)
	if len(manifests) < len(sociIndexes) {
		t.Errorf("the converted image contains more SOCI indexes than manifests. %d manifests, %d soci indexes", len(manifests), len(sociIndexes))
	}
	for _, sociIndexDesc := range sociIndexes {
		if sociIndexDesc.Annotations == nil {
			t.Errorf("SOCI index %v has no annotations", sociIndexDesc)
			continue
		}
		if sociIndexDesc.Annotations[soci.IndexAnnotationImageManifestDigest] == "" {
			t.Errorf("SOCI index %v does not contain image digest", sociIndexDesc)
		}
		if sociIndexDesc.Platform == nil {
			t.Errorf("SOCI index %v does not contain platform", sociIndexDesc)
			continue
		}

		manifestIdx := slices.IndexFunc(manifests, func(desc ocispec.Descriptor) bool {
			return desc.Digest.String() == sociIndexDesc.Annotations[soci.IndexAnnotationImageManifestDigest]
		})
		if manifestIdx == -1 {
			t.Errorf("SOCI index %v does not point to a manifest", sociIndexDesc)
			continue
		}
		manifestDesc := manifests[manifestIdx]
		if manifestDesc.MediaType != ocispec.MediaTypeImageManifest {
			t.Errorf("manifest desc %v is not an image manifest", manifestDesc)
			continue
		}
		if manifestDesc.ArtifactType != "" {
			t.Errorf("manifest desc %v has an artifact type", manifestDesc)
			continue
		}
		if manifestDesc.Annotations == nil {
			t.Errorf("manifest desc %v has no annotations", manifestDesc)
			continue
		}
		if dg, ok := manifestDesc.Annotations[soci.ImageAnnotationSociIndexDigest]; !ok || dg != sociIndexDesc.Digest.String() {
			t.Errorf("manifest desc %v does not contain expected soci index digest %v", manifestDesc, sociIndexDesc.Digest)
		}
		if manifestDesc.Platform == nil {
			t.Errorf("manifest desc %v does not contain platform", manifestDesc)
			continue
		}
		if manifestDesc.Platform.OS != sociIndexDesc.Platform.OS || manifestDesc.Platform.Architecture != sociIndexDesc.Platform.Architecture {
			t.Errorf("manifest desc %v does not match SOCI platform %v", manifestDesc, sociIndexDesc.Platform)
		}

		var manifest ocispec.Manifest
		manifestBytes := sh.O("ctr", "content", "get", manifestDesc.Digest.String())
		if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
			t.Errorf("failed to decode manifest: %v", err)
			continue
		}
		if manifest.MediaType != ocispec.MediaTypeImageManifest {
			t.Errorf("manifest %v is not an image manifest", manifest)
			continue
		}
		if manifest.ArtifactType != "" {
			t.Errorf("manifest %v has an artifact type", manifest)
			continue
		}
		if manifest.Config.MediaType != ocispec.MediaTypeImageConfig {
			t.Errorf("manifest %v has a non-OCI config", manifest)
			continue
		}
		if manifest.Annotations == nil {
			t.Errorf("manifest %v has no annotations", manifest)
			continue
		}
		if dg, ok := manifest.Annotations[soci.ImageAnnotationSociIndexDigest]; !ok || dg != sociIndexDesc.Digest.String() {
			t.Errorf("manifest %v does not contain expected soci index digest %v", manifestDesc, sociIndexDesc.Digest)
		}
	}
}

func TestConvert(t *testing.T) {
	sh, done := newSnapshotterBaseShell(t)
	defer done()

	t.Run("basic conversion", func(t *testing.T) {
		for _, imageName := range convertImages {
			t.Run(imageName, func(t *testing.T) {
				rebootContainerd(t, sh, "", "")
				imgRef := dockerhub(imageName).ref
				convertedRef := imgRef + "-soci"

				sh.X("nerdctl", "pull", "-q", "--all-platforms", imgRef)
				sh.X("soci", "convert", "--all-platforms", "--min-layer-size", "0", imgRef, convertedRef)

				imgDigest := getImageDigest(sh, imgRef)
				convertedDigest := getImageDigest(sh, convertedRef)

				validateConversion(t, sh, imgDigest, convertedDigest)
			})
		}
	})

	t.Run("single platform conversion", func(t *testing.T) {
		for _, imageName := range convertImages {
			t.Run(imageName, func(t *testing.T) {
				rebootContainerd(t, sh, "", "")
				imgRef := dockerhub(imageName).ref
				convertedRef := imgRef + "-soci"

				sh.X("nerdctl", "pull", "-q", imgRef)
				sh.X("soci", "convert", "--min-layer-size", "0", imgRef, convertedRef)

				imgDigest := getImageDigest(sh, imgRef)
				convertedDigest := getImageDigest(sh, convertedRef)

				validateConversion(t, sh, imgDigest, convertedDigest)
			})
		}
	})

	t.Run("conversion idempotency", func(t *testing.T) {
		for _, imageName := range convertImages {
			t.Run(imageName, func(t *testing.T) {
				rebootContainerd(t, sh, "", "")
				imgRef := dockerhub(imageName).ref
				convertedRef1 := imgRef + "-soci"
				convertedRef2 := imgRef + "-soci-2"

				sh.X("nerdctl", "pull", "-q", "--all-platforms", imgRef)
				sh.X("soci", "convert", "--all-platforms", imgRef, convertedRef1)
				sh.X("soci", "convert", "--all-platforms", convertedRef1, convertedRef2)

				convertedDigest1 := getImageDigest(sh, convertedRef1)
				convertedDigest2 := getImageDigest(sh, convertedRef2)
				if convertedDigest1 != convertedDigest2 {
					t.Fatalf("converting an image that was already soci enabled was not idempotent: %s != %s", convertedDigest1, convertedDigest2)
				}
			})
		}
	})

	t.Run("single image manifest conversion", func(t *testing.T) {
		images := []string{cloudwatchAgentx86ImageRef}
		for _, imgRef := range images {
			t.Run(imgRef, func(t *testing.T) {
				rebootContainerd(t, sh, "", "")
				convertedRef := imgRef + "-soci"

				sh.X("nerdctl", "pull", "-q", "--platform", "linux/amd64", imgRef)
				sh.X("soci", "convert", imgRef, convertedRef)

				imgDigest := getImageDigest(sh, imgRef)
				convertedDigest := getImageDigest(sh, convertedRef)

				validateConversion(t, sh, imgDigest, convertedDigest)
			})
		}
	})

	t.Run("convert and replace", func(t *testing.T) {
		for _, imageName := range convertImages {
			t.Run(imageName, func(t *testing.T) {
				rebootContainerd(t, sh, "", "")
				imgRef := dockerhub(imageName).ref

				originalDigest := getImageDigest(sh, imgRef)

				sh.X("nerdctl", "pull", "-q", "--all-platforms", imgRef)
				sh.X("soci", "convert", imgRef, imgRef)

				imgDigest := getImageDigest(sh, imgRef)

				validateConversion(t, sh, originalDigest, imgDigest)
			})
		}
	})

}

type convertAndPushTestConfig struct {
	PullPlatforms    string
	ConvertPlatforms string
	PushPlatforms    string
}

func (c convertAndPushTestConfig) String() string {
	return strings.Join([]string{c.PullPlatforms, c.ConvertPlatforms, c.PushPlatforms}, ",")
}

func TestConvertAndPush(t *testing.T) {
	registryConfig := newRegistryConfig()
	sh, done := newShellWithRegistry(t, registryConfig)
	defer done()

	imageName := nginxImage

	platformOptions := map[string][]string{
		"one": {"--platform", "linux/x86_64"},
		"all": {"--all-platforms"},
	}

	convertFailureMessages := map[convertAndPushTestConfig]string{
		// Any config not in this list should succeed on convert (no error message)
		{PullPlatforms: "one", ConvertPlatforms: "all", PushPlatforms: "one"}: "not found",
		{PullPlatforms: "one", ConvertPlatforms: "all", PushPlatforms: "all"}: "not found",
	}

	pushFailureMessages := map[convertAndPushTestConfig]string{
		// Any config not in this list should succeed on push (no error message)
		{PullPlatforms: "one", ConvertPlatforms: "one", PushPlatforms: "all"}: "not found",
	}

	for pullPlatform := range platformOptions {
		for convertPlatform := range platformOptions {
			for pushPlatform := range platformOptions {
				test := convertAndPushTestConfig{
					PullPlatforms:    pullPlatform,
					ConvertPlatforms: convertPlatform,
					PushPlatforms:    pushPlatform,
				}

				t.Run(test.String(), func(t *testing.T) {
					rebootContainerd(t, sh, "", "")
					img := dockerhub(imageName)
					convertedImg := registryConfig.mirror(imageName)

					pull := []string{"nerdctl", "pull", "-q"}
					convert := []string{"soci", "convert"}
					push := []string{"nerdctl", "push"}

					pull = append(pull, platformOptions[pullPlatform]...)
					convert = append(convert, platformOptions[convertPlatform]...)
					push = append(push, platformOptions[pushPlatform]...)

					convertFailureMessage, expectedConvertFailure := convertFailureMessages[test]
					pushFailureMessage := pushFailureMessages[test]

					sh.X(append(pull, img.ref)...)
					o, err := sh.CombinedOLog(append(convert, img.ref, convertedImg.ref)...)
					validateErrorOutput(t, "convert", string(o), err, convertFailureMessage)
					if expectedConvertFailure {
						// If we expected a convert error and we got the correct one, then the test is done.
						// We should push because we know it will fail.
						return
					}
					sh.X("nerdctl", "login", "--username", registryConfig.user, "--password", registryConfig.pass, convertedImg.ref)
					o, err = sh.CombinedOLog(append(push, convertedImg.ref)...)
					validateErrorOutput(t, "push", string(o), err, pushFailureMessage)
				})

			}
		}
	}
}

func validateErrorOutput(t *testing.T, name string, o string, err error, expectedError string) {
	if expectedError != "" {
		if !strings.Contains(o, expectedError) {
			t.Fatalf("%s: expected error %q, got %q", name, expectedError, o)
		}
	} else if err != nil {
		t.Fatalf("%s: unexpected error: %v", name, err)
	}
}

func TestInvalidConversion(t *testing.T) {
	registryConfig := newRegistryConfig()
	sh, done := newShellWithRegistry(t, registryConfig)
	defer done()

	tests := []struct {
		name     string
		repo     string
		modifier func(t *testing.T, sh *shell.Shell, img imageInfo)
	}{
		{
			name: "deleting manifest invalidates image",
			repo: "manifest",
			modifier: func(t *testing.T, sh *shell.Shell, img imageInfo) {
				digest, err := getManifestDigest(sh, img.ref, platforms.DefaultSpec())
				if err != nil {
					t.Fatalf("failed to get manifest digest: %v", err)
				}
				sh.X("ctr", "content", "delete", digest)
			},
		},
		{
			name: "deleting soci index invalidates image",
			repo: "sociindex",
			modifier: func(t *testing.T, sh *shell.Shell, img imageInfo) {
				index, err := getImageIndex(sh, img.ref)
				if err != nil {
					t.Fatalf("failed to get image index: %v", err)
				}
				idx := slices.IndexFunc(index.Manifests, func(desc ocispec.Descriptor) bool {
					return desc.ArtifactType == soci.SociIndexArtifactTypeV2
				})
				if idx == -1 {
					t.Fatalf("no soci index found")
				}
				sh.X("ctr", "content", "delete", index.Manifests[idx].Digest.String())
			},
		},
	}

	for _, imageName := range convertImages {
		t.Run(imageName, func(t *testing.T) {
			rebootContainerd(t, sh, "", "")
			img := dockerhub(imageName)

			sh.X("nerdctl", "pull", "-q", "--all-platforms", img.ref)

			for _, test := range tests {
				t.Run(test.name, func(t *testing.T) {
					imageName = fmt.Sprintf("%s/%s", test.repo, imageName)
					convertedImg := registryConfig.mirror(imageName)
					sh.X("soci", "convert", "--min-layer-size", "0", img.ref, convertedImg.ref)

					test.modifier(t, sh, convertedImg)

					sh.X("nerdctl", "login", "--username", registryConfig.user, "--password", registryConfig.pass, convertedImg.ref)
					out, err := sh.CombinedOLog("nerdctl", "push", "--all-platforms", convertedImg.ref)
					if err == nil {
						t.Fatalf("expected push to fail")
					}
					if !strings.Contains(string(out), "not found") {
						t.Fatalf("expected push to fail with 'not found' error, got %s", string(out))
					}
				})
			}
		})
	}
}
