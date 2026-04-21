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
	"path/filepath"
	"strings"
	"testing"

	"github.com/awslabs/soci-snapshotter/soci"
	"github.com/awslabs/soci-snapshotter/util/dockershell"
	"github.com/awslabs/soci-snapshotter/util/testutil"
	"github.com/containerd/platforms"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
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

	baseDir, err := testutil.TempDir(sh)
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer sh.X("rm", "-rf", baseDir)

	inputDir := filepath.Join(baseDir, "input")
	inputTar := filepath.Join(baseDir, "input.tar")

	copyImage(sh, dockerhub(imageRef), mirrorImg)

	exportToOCIDir(sh, srcRef, inputDir)
	sh.X("tar", "-cf", inputTar, "-C", inputDir, ".")

	testCases := []struct {
		name   string
		input  string
		output string
		format string
	}{
		{"dir-to-tar", inputDir, filepath.Join(baseDir, "dt.tar"), "oci-archive"},
		{"tar-to-tar", inputTar, filepath.Join(baseDir, "tt.tar"), "oci-archive"},
		{"dir-to-dir", inputDir, filepath.Join(baseDir, "dd"), "oci-dir"},
		{"tar-to-dir", inputTar, filepath.Join(baseDir, "td"), "oci-dir"},
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

	baseDir, err := testutil.TempDir(sh)
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer sh.X("rm", "-rf", baseDir)

	inputDir := filepath.Join(baseDir, "input")
	outputTar := filepath.Join(baseDir, "output.tar")

	copyImage(sh, dockerhub(imageRef), mirrorImg)

	exportToOCIDir(sh, srcRef, inputDir)

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

	srcRef := mirrorImg.ref

	baseDir, err := testutil.TempDir(sh)
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer sh.X("rm", "-rf", baseDir)

	inputDir := filepath.Join(baseDir, "input")
	outputTar1 := filepath.Join(baseDir, "output1.tar")
	outputTar2 := filepath.Join(baseDir, "output2.tar")

	copyImage(sh, dockerhub(imageRef), mirrorImg)

	exportToOCIDir(sh, srcRef, inputDir)

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

func exportToOCIDir(sh *dockershell.Shell, imageRef, outputDir string, saveArgs ...string) {
	exportTar := outputDir + ".export.tar"
	sh.X(append(append([]string{"nerdctl", "save"}, saveArgs...), "-o", exportTar, imageRef)...)
	sh.X("mkdir", "-p", outputDir)
	sh.X("tar", "-xf", exportTar, "-C", outputDir)
	sh.X("rm", "-f", exportTar)
}

func TestStandaloneConvertAllPlatforms(t *testing.T) {
	regConfig := newRegistryConfig()
	sh, done := newShellWithRegistry(t, regConfig)
	defer done()

	rebootContainerd(t, sh, getContainerdConfigToml(t, false), getSnapshotterConfigToml(t))

	// Pull and save the multi-arch image directly from its public registry.
	srcRef := dockerhub(nginxImage).ref

	baseDir, err := testutil.TempDir(sh)
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer sh.X("rm", "-rf", baseDir)

	inputDir := filepath.Join(baseDir, "input")
	outputTar := filepath.Join(baseDir, "output.tar")

	sh.X("nerdctl", "pull", "-q", "--all-platforms", srcRef)
	exportToOCIDir(sh, srcRef, inputDir, "--all-platforms")

	srcDigest := getImageDigest(sh, srcRef)
	var srcPlatforms []string
	for _, m := range readIndex(t, sh, srcDigest).Manifests {
		if m.Platform != nil {
			srcPlatforms = append(srcPlatforms, platforms.Format(*m.Platform))
		}
	}
	if len(srcPlatforms) < 2 {
		t.Skipf("expected multi-arch input, got %d platforms", len(srcPlatforms))
	}

	stopContainerd(t, sh)

	sh.X("soci", "convert",
		"--standalone",
		"--all-platforms",
		"--min-layer-size=0",
		inputDir,
		outputTar,
	)

	rebootContainerd(t, sh, getContainerdConfigToml(t, false), getSnapshotterConfigToml(t))

	dstRef := srcRef + "-standalone-allplatforms"
	sh.X("ctr", "images", "import", "--no-unpack", "--index-name", dstRef, outputTar)

	dstDigest := getImageDigest(sh, dstRef)
	validateConversion(t, sh, srcDigest, dstDigest)

	imgs, sociIdx := map[string]bool{}, map[string]bool{}
	for _, m := range readIndex(t, sh, dstDigest).Manifests {
		if m.Platform == nil {
			continue
		}
		key := platforms.Format(*m.Platform)
		switch m.ArtifactType {
		case soci.SociIndexArtifactTypeV2:
			sociIdx[key] = true
		case "":
			imgs[key] = true
		}
	}
	for _, p := range srcPlatforms {
		if !imgs[p] {
			t.Errorf("converted output missing image manifest for %s", p)
		}
		if !sociIdx[p] {
			t.Errorf("converted output missing SOCI index for %s", p)
		}
	}
}

// readIndex fetches an OCI image index from the content store, unwrapping a
// single-manifest wrapper (as produced by `ctr images import --index-name`).
func readIndex(t *testing.T, sh *dockershell.Shell, indexDigest string) ocispec.Index {
	t.Helper()
	var idx ocispec.Index
	if err := json.Unmarshal(sh.O("ctr", "content", "get", indexDigest), &idx); err != nil {
		t.Fatalf("parse index %s: %v", indexDigest, err)
	}
	if len(idx.Manifests) == 1 && idx.Manifests[0].MediaType == ocispec.MediaTypeImageIndex {
		return readIndex(t, sh, idx.Manifests[0].Digest.String())
	}
	return idx
}
