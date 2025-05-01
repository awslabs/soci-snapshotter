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
	"slices"
	"testing"

	"github.com/awslabs/soci-snapshotter/soci"
	shell "github.com/awslabs/soci-snapshotter/util/dockershell"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

var (
	convertImages = []string{nginxImage, rabbitmqImage, drupalImage, ubuntuImage}
)

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
		if manifest.ArtifactType == soci.SociIndexArtifactTypeV2 {
			sociIndexes = append(sociIndexes, manifest)
		} else if manifest.ArtifactType == "" {
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

				sh.X("nerdctl", "pull", "--all-platforms", imgRef)
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

				sh.X("nerdctl", "pull", "--all-platforms", imgRef)
				sh.X("soci", "convert", imgRef, convertedRef1)
				sh.X("soci", "convert", convertedRef1, convertedRef2)

				convertedDigest1 := getImageDigest(sh, convertedRef1)
				convertedDigest2 := getImageDigest(sh, convertedRef2)
				if convertedDigest1 != convertedDigest2 {
					t.Fatalf("converting an image that was already soci enabled was not idempotent: %s != %s", convertedDigest1, convertedDigest2)
				}
			})
		}
	})

	t.Run("single platform conversion", func(t *testing.T) {
		images := []string{cloudwatchAgentx86ImageRef}
		for _, imgRef := range images {
			t.Run(imgRef, func(t *testing.T) {
				rebootContainerd(t, sh, "", "")
				convertedRef := imgRef + "-soci"

				sh.X("nerdctl", "pull", "--platform", "linux/amd64", imgRef)
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

				sh.X("nerdctl", "pull", "--all-platforms", imgRef)
				sh.X("soci", "convert", imgRef, imgRef)

				imgDigest := getImageDigest(sh, imgRef)

				validateConversion(t, sh, originalDigest, imgDigest)
			})
		}
	})

}

func TestConvertAndPush(t *testing.T) {
	registryConfig := newRegistryConfig()
	sh, done := newShellWithRegistry(t, registryConfig)
	defer done()
	for _, imageName := range convertImages {
		t.Run(imageName, func(t *testing.T) {
			rebootContainerd(t, sh, "", "")
			img := dockerhub(imageName)
			convertedImg := registryConfig.mirror(imageName)

			sh.X("nerdctl", "pull", "--all-platforms", img.ref)
			sh.X("soci", "convert", img.ref, convertedImg.ref)
			sh.X("nerdctl", "login", "--username", registryConfig.user, "--password", registryConfig.pass, convertedImg.ref)
			sh.X("nerdctl", "push", "--all-platforms", convertedImg.ref)
		})
	}
}
