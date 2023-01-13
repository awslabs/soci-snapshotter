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
	"strconv"
	"strings"
	"testing"

	"github.com/awslabs/soci-snapshotter/soci"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
)

func TestSociZtocList(t *testing.T) {
	t.Parallel()
	sh, done := newSnapshotterBaseShell(t)
	defer done()
	rebootContainerd(t, sh, "", "")

	testImages := prepareSociIndices(t, sh)

	t.Run("soci ztoc list should print all ztocs", func(t *testing.T) {
		output := strings.Trim(string(sh.O("soci", "ztoc", "list")), "\n")
		outputLines := strings.Split(output, "\n")
		// output should have at least a header line
		if len(outputLines) < 1 {
			t.Fatalf("output should at least have a header line, actual output: %s", output)
		}
		outputLines = outputLines[1:]

		for _, img := range testImages {
			sociIndex, err := sociIndexFromDigest(sh, img.sociIndexDigest)
			if err != nil {
				t.Fatal(err)
			}

			for _, blob := range sociIndex.Blobs {
				if blob.MediaType != soci.SociLayerMediaType {
					continue
				}

				ztocExistChecker(t, outputLines, img, blob)
			}
		}
	})

	t.Run("soci ztoc list --ztoc-digest ztocDigest should print a single ztoc", func(t *testing.T) {
		target := testImages[0]
		sociIndex, err := sociIndexFromDigest(sh, target.sociIndexDigest)
		if err != nil {
			t.Fatal(err)
		}

		for _, blob := range sociIndex.Blobs {
			if blob.MediaType != soci.SociLayerMediaType {
				continue
			}

			output := strings.Trim(string(sh.O("soci", "ztoc", "list", "--ztoc-digest", blob.Digest.String())), "\n")
			outputLines := strings.Split(output, "\n")
			// outputLines should have exact 2 lines: 1 header and 1 ztoc
			if len(outputLines) != 2 {
				t.Fatalf("output should have exactly a header line and a ztoc line: %s", output)
			}
			outputLines = outputLines[1:]

			ztocExistChecker(t, outputLines, target, blob)
		}
	})

	t.Run("soci ztoc list --image-ref imageRef", func(t *testing.T) {
		for _, img := range testImages {
			sociIndex, err := sociIndexFromDigest(sh, img.sociIndexDigest)
			if err != nil {
				t.Fatal(err)
			}
			output := strings.Trim(string(sh.O("soci", "ztoc", "list", "--image-ref", img.imgInfo.ref)), "\n")
			outputLines := strings.Split(output, "\n")
			ztocOutput := outputLines[1:]

			for _, blob := range sociIndex.Blobs {
				if blob.MediaType != soci.SociLayerMediaType {
					continue
				}
				ztocExistChecker(t, ztocOutput, img, blob)
			}
		}
	})

	t.Run("soci ztoc list --image-ref imageRef --ztoc-digest expectedZtoc", func(t *testing.T) {
		for _, img := range testImages {
			sociIndex, err := sociIndexFromDigest(sh, img.sociIndexDigest)
			if err != nil {
				t.Fatal(err)
			}
			var ztoc v1.Descriptor
			for _, blob := range sociIndex.Blobs {
				if blob.MediaType == soci.SociLayerMediaType {
					ztoc = blob
					break
				}
			}
			output := strings.Trim(string(sh.O("soci", "ztoc", "list", "--image-ref", img.imgInfo.ref,
				"--ztoc-digest", ztoc.Digest.String())), "\n")
			outputLines := strings.Split(output, "\n")
			ztocOutput := outputLines[1:]
			ztocExistChecker(t, ztocOutput, img, ztoc)
		}
	})
	t.Run("soci ztoc list --image-ref imageRef --ztoc-digest unexpectedZtoc", func(t *testing.T) {
		for _, img := range testImages {
			_, err := sh.OLog("soci", "ztoc", "list", "--image-ref", img.imgInfo.ref, "--ztoc-digest", "digest")
			if err == nil {
				t.Fatalf("failed to return err")
			}
		}
	})
}

// ztocExistChecker checks if a ztoc exists in `soci ztoc list` output
func ztocExistChecker(t *testing.T, listOutputLines []string, img testImageIndex, ztocBlob v1.Descriptor) {
	ztocDigest := ztocBlob.Digest.String()
	size := strconv.FormatInt(ztocBlob.Size, 10)
	layerDigest := ztocBlob.Annotations[soci.IndexAnnotationImageLayerDigest]
	for _, line := range listOutputLines {
		if strings.Contains(line, ztocDigest) && strings.Contains(line, size) && strings.Contains(line, layerDigest) {
			return
		}
	}

	t.Fatalf("invalid ztoc from index %s for image %s:\n expected ztoc: digest: %s, size: %s, layer digest: %s\n actual output lines: %s",
		img.sociIndexDigest, img.imgInfo.ref, ztocDigest, size, layerDigest, listOutputLines)
}
