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
	regConfig := newRegistryConfig()
	sh, done := newShellWithRegistry(t, regConfig)
	defer done()

	rebootContainerd(t, sh, getContainerdConfigToml(t, false), getSnapshotterConfigToml(t))

	imageRef := nginxImage
	mirrorImg := regConfig.mirror(imageRef)
	srcRef := mirrorImg.ref
	srcDigest := getImageDigest(sh, srcRef)
	inputDir := "/tmp/standalone-basic-input"
	inputTar := "/tmp/standalone-basic-input.tar"

	copyImage(sh, dockerhub(imageRef), mirrorImg)

	sh.X("crane", "pull", "--format", "oci", "--insecure", srcRef, inputDir)
	sh.X("tar", "-cf", inputTar, "-C", inputDir, ".")

	testCases := []struct {
		name   string
		input  string
		output string
		format string
	}{
		{"dir-to-tar", inputDir, "/tmp/standalone-basic-dt.tar", "oci-archive"},
		{"tar-to-tar", inputTar, "/tmp/standalone-basic-tt.tar", "oci-archive"},
		{"dir-to-dir", inputDir, "/tmp/standalone-basic-dd", "oci-dir"},
		{"tar-to-dir", inputTar, "/tmp/standalone-basic-td", "oci-dir"},
	}

	stopContainerd(t, sh)

	for _, tc := range testCases {
		sh.X("soci", "convert",
			"--standalone",
			"--format", tc.format,
			"--min-layer-size=0",
			tc.input,
			tc.output,
		)
	}

	rebootContainerd(t, sh, getContainerdConfigToml(t, false), getSnapshotterConfigToml(t))

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			dstRef := srcRef + "-standalone-" + tc.name

			importPath := tc.output
			if tc.format == "oci-dir" {
				// ctr images import requires a tar, so tar the directory output first
				importPath = tc.output + ".tar"
				sh.X("tar", "-cf", importPath, "-C", tc.output, ".")
			}
			sh.X("ctr", "images", "import", "--no-unpack", "--index-name", dstRef, importPath)

			dstDigest := getImageDigest(sh, dstRef)
			validateConversion(t, sh, srcDigest, dstDigest)

			// Verify the converted image works with SOCIv2 lazy pull from registry
			sh.X("nerdctl", "push", "-q", dstRef)

			sociV2Enabled := withPullModes(config.PullModes{
				SOCIv1: config.V1{Enable: false},
				SOCIv2: config.V2{Enable: true},
			})
			rebootContainerd(t, sh, getContainerdConfigToml(t, false), getSnapshotterConfigToml(t, sociV2Enabled))
			sh.X("nerdctl", "rmi", "-f", dstRef)
			sh.X("nerdctl", "pull", "-q", "--snapshotter=soci", dstRef)

			sh.X("nerdctl", "run", "--rm", "--net", "none", "--snapshotter=soci", dstRef, "echo", "success")
		})
	}
}

func TestStandaloneConvertSpecificPlatform(t *testing.T) {
	regConfig := newRegistryConfig()
	sh, done := newShellWithRegistry(t, regConfig)
	defer done()

	rebootContainerd(t, sh, getContainerdConfigToml(t, false), getSnapshotterConfigToml(t))

	imageRef := nginxImage
	mirrorImg := regConfig.mirror(imageRef)
	platformStr := platforms.Format(mirrorImg.platform)

	srcRef := mirrorImg.ref
	srcDigest := getImageDigest(sh, srcRef)
	inputDir := "/tmp/standalone-platform-input"
	outputTar := "/tmp/standalone-platform-output.tar"

	copyImage(sh, dockerhub(imageRef), mirrorImg)

	sh.X("crane", "pull", "--format", "oci", "--platform", platformStr, "--insecure", srcRef, inputDir)

	stopContainerd(t, sh)

	sh.X("soci", "convert",
		"--standalone",
		"--platform", platformStr,
		"--min-layer-size=0",
		inputDir,
		outputTar,
	)

	rebootContainerd(t, sh, getContainerdConfigToml(t, false), getSnapshotterConfigToml(t))

	dstRef := srcRef + "-standalone-platform"
	sh.X("ctr", "images", "import", "--no-unpack", "--index-name", dstRef, outputTar)

	dstDigest := getImageDigest(sh, dstRef)
	validateConversion(t, sh, srcDigest, dstDigest)
}

func TestStandaloneInvalidConversion(t *testing.T) {
	regConfig := newRegistryConfig()
	sh, done := newShellWithRegistry(t, regConfig)
	defer done()

	t.Run("nonexistent input", func(t *testing.T) {
		output, err := sh.CombinedOLog("soci", "convert",
			"--standalone",
			"/tmp/nonexistent-input",
			"/tmp/output.tar",
		)
		if err == nil {
			t.Fatal("expected error for nonexistent input")
		}

		outputStr := string(output)
		if !strings.Contains(outputStr, "failed to access input") {
			t.Fatalf("expected error about failed input access, got: %s", outputStr)
		}
	})

	t.Run("invalid format", func(t *testing.T) {
		output, err := sh.CombinedOLog("soci", "convert",
			"--standalone",
			"--format", "invalid",
			"/tmp/some-input.tar",
			"/tmp/output.tar",
		)
		if err == nil {
			t.Fatal("expected error for invalid format")
		}

		outputStr := string(output)
		if !strings.Contains(outputStr, "unsupported output format") {
			t.Fatalf("expected error about unsupported format, got: %s", outputStr)
		}
	})

	t.Run("missing destination", func(t *testing.T) {
		output, err := sh.CombinedOLog("soci", "convert",
			"--standalone",
			"/tmp/some-input.tar",
		)
		if err == nil {
			t.Fatal("expected error for missing destination")
		}

		outputStr := string(output)
		if !strings.Contains(outputStr, "destination needs to be specified") {
			t.Errorf("expected error about missing destination, got: %s", outputStr)
		}
	})
}

func TestStandaloneConvertIdempotent(t *testing.T) {
	regConfig := newRegistryConfig()
	sh, done := newShellWithRegistry(t, regConfig)
	defer done()

	rebootContainerd(t, sh, getContainerdConfigToml(t, false), getSnapshotterConfigToml(t))

	imageRef := nginxImage
	mirrorImg := regConfig.mirror(imageRef)
	platformStr := platforms.Format(mirrorImg.platform)

	srcRef := mirrorImg.ref
	inputDir := "/tmp/standalone-idempotent-input"
	outputTar1 := "/tmp/standalone-idempotent-output1.tar"
	outputTar2 := "/tmp/standalone-idempotent-output2.tar"

	copyImage(sh, dockerhub(imageRef), mirrorImg)

	sh.X("crane", "pull", "--format", "oci", "--platform", platformStr, "--insecure", srcRef, inputDir)

	stopContainerd(t, sh)

	// First conversion
	sh.X("soci", "convert",
		"--standalone",
		"--min-layer-size=0",
		inputDir,
		outputTar1,
	)

	// Second conversion of the already-converted image
	sh.X("soci", "convert",
		"--standalone",
		"--min-layer-size=0",
		outputTar1,
		outputTar2,
	)

	rebootContainerd(t, sh, getContainerdConfigToml(t, false), getSnapshotterConfigToml(t))

	dstRef1 := srcRef + "-standalone-soci1"
	dstRef2 := srcRef + "-standalone-soci2"
	sh.X("ctr", "images", "import", "--no-unpack", "--index-name", dstRef1, outputTar1)
	sh.X("ctr", "images", "import", "--no-unpack", "--index-name", dstRef2, outputTar2)

	digest1 := getImageDigest(sh, dstRef1)
	digest2 := getImageDigest(sh, dstRef2)

	if digest1 != digest2 {
		t.Errorf("converting a SOCI-enabled image should be idempotent, but digests differ: %s vs %s", digest1, digest2)
	}
}
