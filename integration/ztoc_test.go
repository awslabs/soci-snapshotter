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
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/awslabs/soci-snapshotter/config"
	"github.com/awslabs/soci-snapshotter/soci"
	"github.com/awslabs/soci-snapshotter/soci/store"
	"github.com/awslabs/soci-snapshotter/util/dockershell"
	"github.com/awslabs/soci-snapshotter/util/testutil"
	"github.com/awslabs/soci-snapshotter/ztoc"
	"github.com/awslabs/soci-snapshotter/ztoc/compression"
	"github.com/google/go-cmp/cmp"
	"github.com/opencontainers/go-digest"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
)

type Info struct {
	Version           string             `json:"version"`
	BuildTool         string             `json:"build_tool"`
	Size              int64              `json:"size"`
	SpanSize          compression.Offset `json:"span_size"`
	NumSpans          compression.SpanID `json:"num_spans"`
	NumFiles          int                `json:"num_files"`
	NumMultiSpanFiles int                `json:"num_multi_span_files"`
	Files             []FileInfo         `json:"files"`
}

type FileInfo struct {
	Filename  string             `json:"filename"`
	Offset    int64              `json:"offset"`
	Size      int64              `json:"size"`
	Type      string             `json:"type"`
	StartSpan compression.SpanID `json:"start_span"`
	EndSpan   compression.SpanID `json:"end_span"`
}

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
		target := testImages[ubuntuImage]
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
func TestSociZtocInfo(t *testing.T) {
	t.Parallel()
	sh, done := newSnapshotterBaseShell(t)
	defer done()
	rebootContainerd(t, sh, "", "")

	testImages := prepareSociIndices(t, sh)

	getFullZtoc := func(sh *dockershell.Shell, ztocPath string) (*ztoc.Ztoc, error) {
		output := sh.O("cat", ztocPath)
		reader := bytes.NewReader(output)
		z, err := ztoc.Unmarshal(reader)
		return z, err
	}

	for _, img := range testImages {
		tests := []struct {
			name       string
			ztocDigest string
			expectErr  bool
		}{
			{
				name:       "Empty ztoc digest",
				ztocDigest: "",
				expectErr:  true,
			},
			{
				name:       "Invalid ztoc digest format",
				ztocDigest: "hello",
				expectErr:  true,
			},
			{
				name:       "Invalid ztoc digest length",
				ztocDigest: "sha256:hello",
				expectErr:  true,
			},
			{
				name:       "Ztoc digest does not exist",
				ztocDigest: string(digest.NewDigestFromBytes("sha256", []byte("does not exist"))),
				expectErr:  true,
			},
			{
				name:       "Correct ztoc digest",
				ztocDigest: img.ztocDigests[0],
				expectErr:  false,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				var zinfo Info
				output, err := sh.OLog("soci", "ztoc", "info", tt.ztocDigest)
				if !tt.expectErr {
					err := json.Unmarshal(output, &zinfo)
					if err != nil {
						t.Fatalf("expected Info type got %s: %v", output, err)
					}
					blobPath, err := testutil.GetContentStoreBlobPath(config.DefaultContentStoreType)
					if err != nil {
						t.Fatalf("cannot get local content store blob path: %v", err)
					}
					dgst, err := digest.Parse(tt.ztocDigest)
					if err != nil {
						t.Fatalf("cannot parse digest: %v", err)
					}
					ztocPath := filepath.Join(blobPath, dgst.Encoded())
					ztoc, err := getFullZtoc(sh, ztocPath)
					if err != nil {
						t.Fatalf("failed getting original ztoc: %v", err)
					}
					err = verifyInfoOutput(zinfo, ztoc)
					if err != nil {
						t.Fatal(err)
					}
				} else if err == nil {
					t.Fatal("failed to return error")
				}
			})
		}
	}

}

func TestSociZtocGetFile(t *testing.T) {
	t.Parallel()
	sh, done := newSnapshotterBaseShell(t)
	defer done()
	rebootContainerd(t, sh, "", "")

	testImages := prepareSociIndices(t, sh)

	var (
		tempOutputStream  = "test.txt"
		nonexistentFile   = "nonexistent file"
		nonexistentDigest = string(digest.NewDigestFromBytes("sha256", []byte("nonexistent digest")))
	)

	getRandomFilePathsWithinZtoc := func(t *testing.T, ztocDigest string, numFilesPerSpan int) []string {
		var (
			zinfo     Info
			randPaths []string
		)
		r := testutil.NewTestRand(t)
		regPathsBySpan := make(map[compression.SpanID][]string)
		output := sh.O("soci", "ztoc", "info", ztocDigest)
		json.Unmarshal(output, &zinfo)
		for _, file := range zinfo.Files {
			if file.Type == "reg" {
				regPathsBySpan[file.StartSpan] = append(regPathsBySpan[file.StartSpan], file.Filename)
			}
		}
		for _, regPaths := range regPathsBySpan {
			for i := 0; i < numFilesPerSpan; i++ {
				randPaths = append(randPaths, regPaths[r.IntN(len(regPaths))])
			}
		}
		return randPaths
	}

	verifyOutputStream := func(contents, output []byte) error {
		d := cmp.Diff(contents, output)
		if d == "" {
			return nil
		}
		return fmt.Errorf("unexpected output; diff = %v", d)
	}

	for _, img := range testImages {
		ztocDigest := img.ztocDigests[0]
		var layerDigest string
		sociIndex, err := sociIndexFromDigest(sh, img.sociIndexDigest)
		if err != nil {
			t.Fatalf("Failed getting soci index: %v", err)
		}
		for _, blob := range sociIndex.Blobs {
			if blob.Digest.String() == ztocDigest {
				layerDigest = blob.Annotations[soci.IndexAnnotationImageLayerDigest]
				break
			}
		}
		containerdStoreBlobPath, _ := testutil.GetContentStoreBlobPath(store.ContainerdContentStoreType)
		dgst, err := digest.Parse(layerDigest)
		if err != nil {
			t.Fatalf("cannot parse digest: %v", err)
		}
		layerContents := sh.O("cat", filepath.Join(containerdStoreBlobPath, dgst.Encoded()))
		files := getRandomFilePathsWithinZtoc(t, ztocDigest, 1)

		testCases := []struct {
			name        string
			cmd         []string
			toStdout    bool
			expectedErr bool
		}{
			{
				name:        "Ztoc that does not exist",
				cmd:         []string{"soci", "ztoc", "get-file", nonexistentDigest, nonexistentFile},
				toStdout:    true,
				expectedErr: true,
			},
			{
				name:        "Ztoc exists but file does not exist",
				cmd:         []string{"soci", "ztoc", "get-file", ztocDigest, nonexistentFile},
				toStdout:    true,
				expectedErr: true,
			},
			{
				name:        "Ztoc and each file exists, file contents redirected to stdout",
				cmd:         []string{"soci", "ztoc", "get-file", ztocDigest},
				toStdout:    true,
				expectedErr: false,
			},
			{
				name:        "Ztoc and each file exists, file contents redirected to output file",
				cmd:         []string{"soci", "ztoc", "get-file", "-o", tempOutputStream, ztocDigest},
				toStdout:    false,
				expectedErr: false,
			},
		}
		for _, tt := range testCases {
			t.Run(tt.name, func(t *testing.T) {
				if !tt.expectedErr {
					for _, f := range files {
						cmd := append(tt.cmd, f)
						output, err := sh.OLog(cmd...)
						if err != nil {
							t.Fatalf("failed to return file contents: %v", err)
						}
						gzipReader, err := gzip.NewReader(bytes.NewReader(layerContents))

						if err != nil {
							t.Fatalf("error returning gzip reader: %v", err)
						}

						tarReader := tar.NewReader(gzipReader)

						var contents []byte
						for {
							h, err := tarReader.Next()
							if err == io.EOF {
								break
							}
							if h.Name == f {
								contents, err = io.ReadAll(tarReader)
								if err != nil {
									t.Fatalf("failed getting original file content: %v", err)
								}
								break
							}
						}
						if tt.toStdout {
							output = output[:len(output)-1]
						} else {
							output = sh.O("cat", tempOutputStream)

						}
						err = verifyOutputStream(contents, output)
						if err != nil {
							t.Fatal(err)
						}
					}

				} else if _, err := sh.OLog(tt.cmd...); err == nil {
					t.Fatal("failed to return error")
				}
			})

		}
	}
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

func verifyInfoOutput(zinfo Info, ztoc *ztoc.Ztoc) error {
	if zinfo.Version != string(ztoc.Version) {
		return fmt.Errorf("different versions: expected %s got %s", ztoc.Version, zinfo.Version)
	}
	if zinfo.BuildTool != ztoc.BuildToolIdentifier {
		return fmt.Errorf("different buildtool: expected %s got %s", ztoc.BuildToolIdentifier, zinfo.BuildTool)
	}
	if zinfo.NumFiles != len(ztoc.FileMetadata) {
		return fmt.Errorf("different file counts: expected %v got %v", len(ztoc.FileMetadata), zinfo.NumFiles)
	}
	if zinfo.NumSpans != ztoc.MaxSpanID+1 {
		return fmt.Errorf("different number of spans: expected %v got %v", ztoc.MaxSpanID+1, zinfo.NumSpans)
	}
	for i := 0; i < len(zinfo.Files); i++ {
		zinfoFile := zinfo.Files[i]
		ztocFile := ztoc.FileMetadata[i]

		if zinfoFile.Filename != ztocFile.Name {
			return fmt.Errorf("different filename: expected %s got %s", ztocFile.Name, zinfoFile.Filename)

		}
		if zinfoFile.Offset != int64(ztocFile.UncompressedOffset) {
			return fmt.Errorf("different file offset: expected %v got %v", int64(ztocFile.UncompressedOffset), zinfoFile.Offset)

		}
		if zinfoFile.Size != int64(ztocFile.UncompressedSize) {
			return fmt.Errorf("different file size: expected %v got %v", int64(ztocFile.UncompressedSize), zinfoFile.Size)

		}
		if zinfoFile.Type != ztocFile.Type {
			return fmt.Errorf("different file type: expected %s got %s", ztocFile.Type, zinfoFile.Type)

		}
	}
	return nil
}
