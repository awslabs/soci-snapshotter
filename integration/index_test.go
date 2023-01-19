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
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/awslabs/soci-snapshotter/soci"
	"github.com/awslabs/soci-snapshotter/util/dockershell"
	shell "github.com/awslabs/soci-snapshotter/util/dockershell"
	"github.com/containerd/containerd/platforms"
)

type testImageIndex struct {
	imgName         string
	platform        string
	imgInfo         imageInfo
	sociIndexDigest string
	ztocDigests     []string
}

func prepareSociIndices(t *testing.T, sh *dockershell.Shell) []testImageIndex {
	testImages := []testImageIndex{
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

	for i, img := range testImages {
		platform := platforms.DefaultSpec()
		if img.platform != "" {
			var err error
			platform, err = platforms.Parse(img.platform)
			if err != nil {
				t.Fatalf("could not parse platform: %v", err)
			}
		}
		img.imgInfo = dockerhub(img.imgName, withPlatform(platform))
		img.sociIndexDigest = buildIndex(sh, img.imgInfo)
		ztocDigests, err := getZtocDigestsForImage(sh, img.imgInfo)
		if err != nil {
			t.Fatalf("could not get ztoc digests: %v", err)
		}
		img.ztocDigests = ztocDigests
		testImages[i] = img
	}

	return testImages
}

func getZtocDigestsForImage(sh *dockershell.Shell, img imageInfo) ([]string, error) {
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

	for _, img := range testImages {
		tests := []struct {
			name      string
			digest    string
			expectErr bool
		}{
			{
				name:      img.imgName + " with index digest",
				digest:    img.sociIndexDigest,
				expectErr: false,
			},
			{
				name:      img.imgName + " with ztoc digest",
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

					validateSociIndex(t, sh, sociIndex, m, nil)
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
			command:      []string{"soci", "index", "list", "--ref", testImages[0].imgInfo.ref},
			filter:       func(img testImageIndex) bool { return img.imgInfo.ref == testImages[0].imgInfo.ref },
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
			command:      []string{"soci", "index", "list", "-q", "--ref", testImages[0].imgInfo.ref},
			filter:       func(img testImageIndex) bool { return img.imgInfo.ref == testImages[0].imgInfo.ref },
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
	rebootContainerd(t, sh, "", "")

	t.Run("soci index rm indexDigest removes an index", func(t *testing.T) {
		testImages := prepareSociIndices(t, sh)
		target := testImages[0]
		indicesRaw := sh.
			X("soci", "index", "rm", target.sociIndexDigest).
			O("soci", "index", "list", "-q")
		if strings.Contains(string(indicesRaw), target.sociIndexDigest) {
			t.Fatalf("\"soci index rm indexDigest\" doesn't remove the given index: %s", target.sociIndexDigest)
		}
	})

	t.Run("soci index rm --ref imgRef removes all indices for imgRef", func(t *testing.T) {
		testImages := prepareSociIndices(t, sh)
		target := testImages[0]
		indicesRaw := sh.
			X("soci", "index", "rm", "--ref", target.imgInfo.ref).
			O("soci", "index", "list", "-q", "--ref", target.imgInfo.ref)
		indices := strings.Trim(string(indicesRaw), "\n")
		if indices != "" {
			t.Fatalf("\"soci index rm --ref\" doesn't remove all soci indices for the given image %s, remaining indices: %s", target.imgInfo.ref, indices)
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

func sociIndexFromDigest(sh *shell.Shell, indexDigest string) (index soci.Index, err error) {
	rawSociIndexJSON, err := sh.OLog("soci", "index", "info", indexDigest)
	if err != nil {
		return
	}
	if err = json.Unmarshal(rawSociIndexJSON, &index); err != nil {
		err = fmt.Errorf("invalid soci index from digest %s: %s", indexDigest, err)
	}
	return
}
