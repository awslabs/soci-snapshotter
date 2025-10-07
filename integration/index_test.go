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
	"bufio"
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/awslabs/soci-snapshotter/config"
	"github.com/awslabs/soci-snapshotter/soci/store"
	shell "github.com/awslabs/soci-snapshotter/util/dockershell"
	"github.com/containerd/platforms"
)

type testImageIndex struct {
	imgName         string
	platform        string
	imgInfo         imageInfo
	sociIndexDigest string
	ztocDigests     []string
}

func prepareSociIndices(t *testing.T, sh *shell.Shell, opt ...indexBuildOption) map[string]testImageIndex {
	imageIndexes := []testImageIndex{
		{
			imgName:  ubuntuImage,
			platform: "linux/arm64",
		},
		{
			imgName:  alpineImage,
			platform: "linux/amd64",
		},
		{
			imgName:  nginxImage,
			platform: "linux/arm64",
		},
		{
			imgName:  drupalImage,
			platform: "linux/amd64",
		},
	}
	return prepareCustomSociIndices(t, sh, imageIndexes, opt...)
}

func prepareCustomSociIndices(t *testing.T, sh *shell.Shell, images []testImageIndex, opt ...indexBuildOption) map[string]testImageIndex {
	indexBuildConfig := defaultIndexBuildConfig()
	for _, o := range opt {
		o(&indexBuildConfig)
	}
	testImages := make(map[string]testImageIndex)
	for _, tii := range images {
		testImages[tii.imgName] = tii
	}

	for imgName, img := range testImages {
		platform := platforms.DefaultSpec()
		if img.platform != "" {
			var err error
			platform, err = platforms.Parse(img.platform)
			if err != nil {
				t.Fatalf("could not parse platform: %v", err)
			}
		}
		img.imgInfo = dockerhub(imgName, withPlatform(platform))
		img.sociIndexDigest = buildIndex(sh, img.imgInfo, withIndexBuildConfig(indexBuildConfig), withMinLayerSize(0), withRunRebuildDbBeforeCreate())
		ztocDigests, err := getZtocDigestsForImage(sh, img.imgInfo)
		if err != nil {
			t.Fatalf("could not get ztoc digests: %v", err)
		}
		img.ztocDigests = ztocDigests
		testImages[imgName] = img
	}

	return testImages
}

func getZtocDigestsForImage(sh *shell.Shell, img imageInfo) ([]string, error) {
	ztocInfoBytes := sh.O("soci", "ztoc", "list", "--image-ref", img.ref)
	scanner := bufio.NewScanner(bytes.NewReader(ztocInfoBytes))
	scanner.Split(bufio.ScanLines)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	var ztocDigests []string
	for i := 1; i < len(lines); i++ {
		entries := strings.Fields(lines[i])
		ztocDigests = append(ztocDigests, entries[0])
	}
	return ztocDigests, nil
}

func TestSociIndexInfo(t *testing.T) {
	t.Parallel()
	sh, done := newSnapshotterBaseShell(t)
	defer done()
	rebootContainerd(t, sh, "", "")

	testImages := prepareSociIndices(t, sh)

	for imgName, img := range testImages {
		tests := []struct {
			name      string
			digest    string
			expectErr bool
		}{
			{
				name:      imgName + " with index digest",
				digest:    img.sociIndexDigest,
				expectErr: false,
			},
			{
				name:      imgName + " with ztoc digest",
				digest:    img.ztocDigests[0],
				expectErr: true,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				sociIndex, err := sociIndexFromDigest(sh, tt.digest)
				if !tt.expectErr {
					if err != nil {
						t.Fatal(err)
					}

					m, err := getManifestDigest(sh, img.imgInfo.ref, img.imgInfo.platform)
					if err != nil {
						t.Fatalf("failed to get manifest digest: %v", err)
					}

					if err := validateSociIndex(sh, config.DefaultContentStoreType, sociIndex, m, nil); err != nil {
						t.Fatalf("failed to validate soci index: %v", err)
					}
				} else if err == nil {
					t.Fatalf("failed to return err")
				}
			})
		}
	}
}

func TestSociIndexList(t *testing.T) {
	t.Parallel()
	sh, done := newSnapshotterBaseShell(t)
	defer done()
	rebootContainerd(t, sh, "", "")

	testImages := prepareSociIndices(t, sh)

	existHandlerFull := func(output string, img testImageIndex) bool {
		// full output should have both img ref and soci index digest
		return strings.Contains(output, img.imgInfo.ref) && strings.Contains(output, img.sociIndexDigest)
	}

	existHandlerQuiet := func(output string, img testImageIndex) bool {
		// a given soci index should match exactly one line in the quiet output
		// for the first index, it should have prefix of digest+\n
		// for the rest, it should have `\n` before and after its digest
		return strings.HasPrefix(output, img.sociIndexDigest+"\n") || strings.Contains(output, "\n"+img.sociIndexDigest+"\n")
	}

	existHandlerExact := func(output string, img testImageIndex) bool {
		// when quiet output has only one index, it should be the exact soci_index_digest string
		return strings.Trim(output, "\n") == img.sociIndexDigest
	}

	// each test runs a soci command, filter to get expected images, and check
	// (only) expected images exist in command output
	tests := []struct {
		name         string
		command      []string
		filter       func(img testImageIndex) bool                // return true if `img` is expected in command output
		existHandler func(output string, img testImageIndex) bool // return true if `img` appears in `output`
	}{
		{
			name:         "`soci index ls` should list all soci indices",
			command:      []string{"soci", "index", "list"},
			filter:       func(img testImageIndex) bool { return true },
			existHandler: existHandlerFull,
		},
		{
			name:         "`soci index ls -q` should list digests of all soci indices",
			command:      []string{"soci", "index", "list", "-q"},
			filter:       func(img testImageIndex) bool { return true },
			existHandler: existHandlerQuiet,
		},
		{
			name:         "`soci index ls --ref imgRef` should only list soci indices for the image",
			command:      []string{"soci", "index", "list", "--ref", testImages[ubuntuImage].imgInfo.ref},
			filter:       func(img testImageIndex) bool { return img.imgInfo.ref == testImages[ubuntuImage].imgInfo.ref },
			existHandler: existHandlerFull,
		},
		{
			name:         "`soci index ls --platform linux/arm64` should only list soci indices for arm64 platform",
			command:      []string{"soci", "index", "list", "--platform", "linux/arm64"},
			filter:       func(img testImageIndex) bool { return img.platform == "linux/arm64" },
			existHandler: existHandlerFull,
		},
		{
			// make sure the image only generates one soci index (the test expects a single digest output)
			name:         "`soci index ls --ref imgRef -q` should print the exact soci index digest",
			command:      []string{"soci", "index", "list", "-q", "--ref", testImages[ubuntuImage].imgInfo.ref},
			filter:       func(img testImageIndex) bool { return img.imgInfo.ref == testImages[ubuntuImage].imgInfo.ref },
			existHandler: existHandlerExact,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := string(sh.O(tt.command...))
			for _, img := range testImages {
				expected := tt.filter(img)
				if expected && !tt.existHandler(output, img) {
					t.Fatalf("output doesn't have expected soci index: image: %s, soci index: %s", img.imgInfo.ref, img.sociIndexDigest)
				}
				if !expected && tt.existHandler(output, img) {
					t.Fatalf("output has unexpected soci index: image: %s, soci index: %s", img.imgInfo.ref, img.sociIndexDigest)
				}
			}
		})
	}
}

func TestSociIndexRemove(t *testing.T) {
	sh, done := newSnapshotterBaseShell(t)
	defer done()
	rebootContainerd(t, sh, getContainerdConfigToml(t, false, `
[plugins."io.containerd.gc.v1.scheduler"]
	deletion_threshold = 1
	startup_delay = "10ms"
`), "")

	t.Run("soci index rm indexDigest removes an index", func(t *testing.T) {
		testImages := prepareSociIndices(t, sh)
		target := testImages[ubuntuImage]
		indicesRaw := sh.
			X("soci", "index", "rm", target.sociIndexDigest).
			O("soci", "index", "list", "-q")
		if strings.Contains(string(indicesRaw), target.sociIndexDigest) {
			t.Fatalf("\"soci index rm indexDigest\" doesn't remove the given index: %s", target.sociIndexDigest)
		}
	})

	t.Run("soci index rm --ref imgRef removes all indices for imgRef", func(t *testing.T) {
		testImages := prepareSociIndices(t, sh)
		target := testImages[ubuntuImage]
		indicesRaw := sh.
			X("soci", "index", "rm", "--ref", target.imgInfo.ref).
			O("soci", "index", "list", "-q", "--ref", target.imgInfo.ref)
		indices := strings.Trim(string(indicesRaw), "\n")
		if indices != "" {
			t.Fatalf("\"soci index rm --ref\" doesn't remove all soci indices for the given image %s, remaining indices: %s", target.imgInfo.ref, indices)
		}
	})

	t.Run("soci index rm on containerd content store removes orphaned zTOCs and not unorphaned zTOCs", func(t *testing.T) {
		testImages := prepareCustomSociIndices(t, sh,
			[]testImageIndex{{imgName: nginxAlpineImage}, {imgName: nginxAlpineImage2}}, withContentStoreType(store.ContainerdContentStoreType))

		remove := testImages[nginxAlpineImage]
		keep := testImages[nginxAlpineImage2]

		commonZtocs := make(map[string]struct{})
		removeZtocs := make(map[string]struct{})
		for _, dgst := range remove.ztocDigests {
			removeZtocs[dgst] = struct{}{}
		}
		for _, dgst := range keep.ztocDigests {
			if _, ok := removeZtocs[dgst]; ok {
				commonZtocs[dgst] = struct{}{}
			}
		}
		if len(commonZtocs) == 0 {
			t.Fatalf("test invalidated due to no common zTOCs between %s and %s", remove.sociIndexDigest, keep.sociIndexDigest)
		}
		if len(removeZtocs)-len(commonZtocs) < 1 {
			t.Fatalf("test invalidated due to no unique zTOCs between %s and %s", remove.sociIndexDigest, keep.sociIndexDigest)
		}

		sh.X("soci", "--content-store", string(store.ContainerdContentStoreType), "index", "rm", remove.sociIndexDigest)
		time.Sleep(1 * time.Second)
		// clean up zTOCs from the artifact db if they were removed from the content store due to garbage collection
		sh.X("soci", "--content-store", string(store.ContainerdContentStoreType), "rebuild-db")

		ztocsRaw := string(sh.O("soci", "ztoc", "list", "-q"))
		for dgst := range removeZtocs {
			if _, ok := commonZtocs[dgst]; ok {
				if !strings.Contains(ztocsRaw, dgst) {
					t.Fatalf("index removal removed non-oprhaned ztoc: %s", dgst)
				}
			} else {
				if strings.Contains(ztocsRaw, dgst) {
					t.Fatalf("index removal didn't remove oprhaned ztoc: %s", dgst)
				}
			}
		}
	})

	t.Run("soci index rm $(soci index ls -q) removes all existing indices", func(t *testing.T) {
		_ = prepareSociIndices(t, sh)
		// a walkaround due to that go exec doesn't support command substitution.
		allIndices := strings.Trim(string(sh.O("soci", "index", "list", "-q")), "\n")
		rmCommand := append([]string{"soci", "index", "rm"}, strings.Split(allIndices, "\n")...)
		indicesRaw := sh.
			X(rmCommand...).
			O("soci", "index", "list", "-q")
		indices := strings.Trim(string(indicesRaw), "\n")
		if indices != "" {
			t.Fatalf("\"soci index rm $(soci index ls -q)\" doesn't remove all soci indices, remaining indices: %s", indices)
		}
	})

	t.Run("soci index rm with an invalid index digest", func(t *testing.T) {
		invalidDgst := "digest"
		_, err := sh.OLog("soci", "index", "rm", invalidDgst)
		if err == nil {
			t.Fatalf("failed to return err")
		}
	})
}

func TestSociIndexRemoveAndRebuildWithSharedLayers(t *testing.T) {
	sh, done := newSnapshotterBaseShell(t)
	defer done()
	rebootContainerd(t, sh, "", "")

	t.Run("soci index rm with shared layers should not affect the other image", func(t *testing.T) {
		// the following 2 images share layers
		img1 := dockerhub(nginxAlpineImage)
		img2 := dockerhub(nginxAlpineImage2)
		imgDigest := buildIndex(sh, img1, withMinLayerSize(0), withRunRebuildDbBeforeCreate())
		if imgDigest == "" {
			t.Fatal("failed to get soci index for nginx alpine test image")
		}
		imgDigest2 := buildIndex(sh, img2, withMinLayerSize(0))
		if imgDigest2 == "" {
			t.Fatal("failed to get soci index for nginx alpine 2 test image")
		}

		sh.X("soci", "index", "rm", imgDigest)

		ztocsBeforeRebuild, err := getZtocDigestsForImage(sh, img2)
		if err != nil {
			t.Fatalf("could not get ztoc digests before rebuild-db: %v", err)
		}
		sh.X("soci", "rebuild-db")
		ztocsAfterRebuild, err := getZtocDigestsForImage(sh, img2)
		if err != nil {
			t.Fatalf("could not get ztoc digests after rebuild-db: %v", err)
		}

		if len(ztocsBeforeRebuild) != len(ztocsAfterRebuild) {
			t.Fatalf(
				"expected number of ztoc's to remain unchanged after rebuild-db: expected: %d, got: %d",
				len(ztocsBeforeRebuild),
				len(ztocsAfterRebuild),
			)
		}
		for i := range ztocsBeforeRebuild {
			if ztocsBeforeRebuild[i] != ztocsAfterRebuild[i] {
				t.Fatalf("expected ztoc digests to remain unchanged after rebuild-db: expected: %s, got: %s",
					ztocsBeforeRebuild[i],
					ztocsAfterRebuild[i],
				)
			}
		}
	})
}
