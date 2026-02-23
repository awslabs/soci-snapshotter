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
	"strings"
	"testing"

	"github.com/awslabs/soci-snapshotter/config"
	"github.com/containerd/platforms"
)

func TestStandaloneConvertBasic(t *testing.T) {
	t.Parallel()
	regConfig := newRegistryConfig()
	sh, done := newShellWithRegistry(t, regConfig)
	defer done()

	rebootContainerd(t, sh, getContainerdConfigToml(t, false), getSnapshotterConfigToml(t))

	imageRef := nginxImage
	copyImage(sh, dockerhub(imageRef), regConfig.mirror(imageRef))

	srcRef := regConfig.mirror(imageRef).ref
	dstRef := srcRef + "-standalone-soci"

	stopContainerd(t, sh)

	sh.X("soci", "convert",
		"--standalone",
		"--all-platforms",
		"--min-layer-size=0",
		srcRef,
		dstRef,
	)

	rebootContainerd(t, sh, getContainerdConfigToml(t, false), getSnapshotterConfigToml(t))

	sh.X("nerdctl", "pull", "-q", dstRef)

	srcDigest := getImageDigest(sh, srcRef)
	dstDigest := getImageDigest(sh, dstRef)

	validateConversion(t, sh, srcDigest, dstDigest)

	sociV2Enabled := withPullModes(config.PullModes{
		SOCIv1: config.V1{Enable: false},
		SOCIv2: config.V2{Enable: true},
	})
	rebootContainerd(t, sh, getContainerdConfigToml(t, false), getSnapshotterConfigToml(t, sociV2Enabled))
	sh.X("nerdctl", "rmi", "-f", dstRef)
	sh.X("nerdctl", "pull", "-q", "--snapshotter=soci", "--all-platforms", dstRef)

	sh.X("nerdctl", "run", "--rm", "--net", "none", "--snapshotter=soci", dstRef, "echo", "success")
}

func TestStandaloneConvertSpecificPlatform(t *testing.T) {
	t.Parallel()
	regConfig := newRegistryConfig()
	sh, done := newShellWithRegistry(t, regConfig)
	defer done()

	rebootContainerd(t, sh, getContainerdConfigToml(t, false), getSnapshotterConfigToml(t))

	imageRef := nginxImage
	copyImage(sh, dockerhub(imageRef), regConfig.mirror(imageRef))

	mirrorImg := regConfig.mirror(imageRef)
	platformStr := platforms.Format(mirrorImg.platform)

	srcRef := mirrorImg.ref
	dstRef := srcRef + "-standalone-specific-platform"

	stopContainerd(t, sh)

	sh.X("soci", "convert",
		"--standalone",
		"--platform", platformStr,
		"--min-layer-size=0",
		srcRef,
		dstRef,
	)

	rebootContainerd(t, sh, getContainerdConfigToml(t, false), getSnapshotterConfigToml(t))

	sh.X("nerdctl", "pull", "-q", "--platform", platformStr, dstRef)

	srcDigest := getImageDigest(sh, srcRef)
	dstDigest := getImageDigest(sh, dstRef)

	validateConversion(t, sh, srcDigest, dstDigest)
}

func TestStandaloneConvertWithUserAuth(t *testing.T) {
	t.Parallel()
	regConfig := newRegistryConfig()
	sh, done := newShellWithRegistry(t, regConfig)
	defer done()

	rebootContainerd(t, sh, getContainerdConfigToml(t, false), getSnapshotterConfigToml(t))

	imageRef := nginxImage
	copyImage(sh, dockerhub(imageRef), regConfig.mirror(imageRef))

	srcRef := regConfig.mirror(imageRef).ref
	dstRef := srcRef + "-standalone-user-auth"

	stopContainerd(t, sh)

	sh.X("soci", "convert",
		"--standalone",
		"--min-layer-size=0",
		"--user", regConfig.creds(),
		srcRef,
		dstRef,
	)

	rebootContainerd(t, sh, getContainerdConfigToml(t, false), getSnapshotterConfigToml(t))

	sh.X("nerdctl", "pull", "-q", dstRef)
}

func TestStandaloneInvalidConversion(t *testing.T) {
	t.Parallel()
	regConfig := newRegistryConfig()
	sh, done := newShellWithRegistry(t, regConfig)
	defer done()

	t.Run("nonexistent image", func(t *testing.T) {
		output, err := sh.CombinedOLog("soci", "convert",
			"--standalone",
			regConfig.mirror("nonexistent:image").ref,
			regConfig.mirror("nonexistent:image").ref+"-standalone-soci",
		)
		if err == nil {
			t.Fatal("expected error for nonexistent image")
		}

		outputStr := string(output)
		if !strings.Contains(outputStr, "failed to resolve image") {
			t.Fatalf("expected error about failed download, got: %s", outputStr)
		}
	})

	t.Run("missing destination ref", func(t *testing.T) {
		output, err := sh.CombinedOLog("soci", "convert",
			"--standalone",
			regConfig.mirror(nginxImage).ref,
		)
		if err == nil {
			t.Fatal("expected error for missing destination ref")
		}

		outputStr := string(output)
		if !strings.Contains(outputStr, "destination image needs to be specified") {
			t.Errorf("expected error about missing destination, got: %s", outputStr)
		}
	})
}

func TestStandaloneConvertIdempotent(t *testing.T) {
	t.Parallel()
	regConfig := newRegistryConfig()
	sh, done := newShellWithRegistry(t, regConfig)
	defer done()

	rebootContainerd(t, sh, getContainerdConfigToml(t, false), getSnapshotterConfigToml(t))

	imageRef := nginxImage
	copyImage(sh, dockerhub(imageRef), regConfig.mirror(imageRef))

	srcRef := regConfig.mirror(imageRef).ref
	dstRef1 := srcRef + "-standalone-soci1"
	dstRef2 := srcRef + "-standalone-soci2"

	stopContainerd(t, sh)

	// First conversion
	sh.X("soci", "convert",
		"--standalone",
		"--min-layer-size=0",
		srcRef,
		dstRef1,
	)

	// Second conversion of the already-converted image
	sh.X("soci", "convert",
		"--standalone",
		"--min-layer-size=0",
		dstRef1,
		dstRef2,
	)

	rebootContainerd(t, sh, getContainerdConfigToml(t, false), getSnapshotterConfigToml(t))

	// Pull both to containerd
	sh.X("nerdctl", "pull", "-q", dstRef1)
	sh.X("nerdctl", "pull", "-q", dstRef2)

	digest1 := getImageDigest(sh, dstRef1)
	digest2 := getImageDigest(sh, dstRef2)

	if digest1 != digest2 {
		t.Errorf("converting a SOCI-enabled image should be idempotent, but digests differ: %s vs %s", digest1, digest2)
	}
}
