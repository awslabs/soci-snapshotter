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

package ztoc

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"math/rand"
	"os"
	"reflect"
	"testing"

	"github.com/awslabs/soci-snapshotter/util/testutil"
	"github.com/awslabs/soci-snapshotter/ztoc/compression"
	"github.com/opencontainers/go-digest"
)

func init() {
	rand.Seed(100)
}

// buildTarGZ creates a temp tar gz file with the given `tarEntries`.
// It returns the created tar gz filename, entry->content map, and the name of
// each entry in the tar gz file.
func buildTarGZ(t *testing.T, tarName string, tarEntries []testutil.TarEntry) (string, map[string][]byte, []string) {
	tarReader := testutil.BuildTarGz(tarEntries, gzip.DefaultCompression)
	tarGzFilePath, _, err := testutil.WriteTarToTempFile(tarName+".tar.gz", tarReader)
	if err != nil {
		t.Fatalf("cannot prepare the .tar.gz file for testing")
	}
	m, fileNames, err := testutil.GetFilesAndContentsWithinTarGz(tarGzFilePath)
	if err != nil {
		os.Remove(tarGzFilePath)
		t.Fatalf("failed to get targz files and their contents: %v", err)
	}
	return tarGzFilePath, m, fileNames
}

func buildTar(t *testing.T, tarName string, tarEntries []testutil.TarEntry) (string, map[string][]byte, []string) {
	tarReader := testutil.BuildTar(tarEntries)
	tarFilePath, _, err := testutil.WriteTarToTempFile(tarName+".tar", tarReader)
	if err != nil {
		t.Fatalf("cannot prepare the .tar file for testing")
	}
	m, fileNames, err := testutil.GetFilesAndContentsWithinTar(tarFilePath)
	if err != nil {
		os.Remove(tarFilePath)
		t.Fatalf("failed to get tar files and their contents: %v", err)
	}
	return tarFilePath, m, fileNames
}

// tarGenerator represents a function that receives a tar filename pattern and a list of
// tar entries, creates a temp tar file (e.g., .tar, .tar.gz) and
// returns the created tar filename, a map that maps each filename within
// the tar to its content, and a list of filenames within the tarfile.
type tarGenerator func(*testing.T, string, []testutil.TarEntry) (string, map[string][]byte, []string)

var testZtocs = []struct {
	name            string
	compressionAlgo string
	tarGenerator    tarGenerator
}{
	{
		name:            "gzip",
		compressionAlgo: compression.Gzip,
		tarGenerator:    buildTarGZ,
	},
	{
		name:            "uncompressed",
		compressionAlgo: compression.Uncompressed,
		tarGenerator:    buildTar,
	},
}

func TestDecompress(t *testing.T) {
	for _, tc := range testZtocs {
		testDecompress(t, tc.compressionAlgo, tc.tarGenerator)
	}
}

func testDecompress(t *testing.T, compressionAlgo string, generator tarGenerator) {
	tarEntries := []testutil.TarEntry{
		testutil.File("smallfile", string(testutil.RandomByteDataRange(1, 100))),
		testutil.File("mediumfile", string(testutil.RandomByteDataRange(10000, 128000))),
		testutil.File("largefile", string(testutil.RandomByteDataRange(350000, 500000))),
		testutil.File("jumbofile", string(testutil.RandomByteDataRange(3000000, 5000000))),
	}

	tests := []struct {
		name     string
		spanSize int64
	}{
		{
			name:     "decompress span size 10kB",
			spanSize: 10000,
		},
		{
			name:     "decompress span size 64KiB",
			spanSize: 65535,
		},
		{
			name:     "decompress span size 128kB",
			spanSize: 128000,
		},
		{
			name:     "decompress span size 256kB",
			spanSize: 256000,
		},
		{
			name:     "decompress span size 512kB",
			spanSize: 512000,
		},
		{
			name:     "decompress span size 1MiB",
			spanSize: 1 << 20,
		},
	}

	ztocBuilder := NewBuilder("test")

	tarFilePath, m, fileNames := generator(t, "decompress", tarEntries)
	defer os.Remove(tarFilePath)

	for _, tc := range tests {
		tc := tc
		ztoc, err := ztocBuilder.BuildZtoc(tarFilePath, tc.spanSize, WithCompression(compressionAlgo))
		if err != nil {
			t.Fatalf("%s: can't build ztoc: %v", tc.name, err)
		}
		if ztoc == nil {
			t.Fatalf("%s: ztoc should not be nil", tc.name)
		}

		for _, f := range fileNames {
			file, err := os.Open(tarFilePath)
			if err != nil {
				t.Fatalf("%s: could not open open the .tar.gz file", tc.name)
			}
			defer file.Close()

			fi, err := file.Stat()
			if err != nil {
				t.Fatalf("%s: could not get the stat for the file %s", tc.name, tarFilePath)
			}

			extracted, err := ztoc.ExtractFile(io.NewSectionReader(file, 0, fi.Size()), f)
			if err != nil {
				t.Fatalf("%s: could not extract from tar gz", tc.name)
			}
			original := m[f]
			if !bytes.Equal(extracted, original) {
				diffIdx := getPositionOfFirstDiffInByteSlice(extracted, original)
				t.Fatalf("%s: span_size=%d: file %s extracted bytes != original bytes; byte %d is different",
					tc.name, tc.spanSize, f, diffIdx)
			}
		}
	}
}

func TestDecompressWithGzipHeaders(t *testing.T) {
	const spanSize = 1024
	testcases := []struct {
		name string
		opts []testutil.BuildTarOption
	}{
		{
			name: "ztoc decompress works with gzip comments",
			opts: []testutil.BuildTarOption{testutil.WithGzipComment("test comment")},
		},
		{
			name: "ztoc decompress works with gzip filename",
			opts: []testutil.BuildTarOption{testutil.WithGzipFilename("filename.tar")},
		},
		{
			name: "ztoc decompress works with gzip extra data",
			opts: []testutil.BuildTarOption{testutil.WithGzipExtra(testutil.RandomByteData(100))},
		},
		{
			name: "ztoc decompress works with gzip comments, filename, and extra data",
			opts: []testutil.BuildTarOption{
				testutil.WithGzipComment("test comment"),
				testutil.WithGzipFilename("filename.tar"),
				testutil.WithGzipExtra(testutil.RandomByteData(100)),
			},
		},
		{
			name: "ztoc decompress works when extra data is bigger than the span size",
			opts: []testutil.BuildTarOption{testutil.WithGzipExtra(testutil.RandomByteData(2 * spanSize))},
		},
	}
	for _, tc := range testcases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			data := testutil.RandomByteData(100)
			ztoc, sr, err := BuildZtocReader(t,
				[]testutil.TarEntry{
					testutil.File("file", string(data)),
				},
				gzip.DefaultCompression,
				spanSize,
				tc.opts...,
			)
			if err != nil {
				t.Fatalf("failed to create ztoc: %v", err)
			}
			b, err := ztoc.ExtractFile(sr, ztoc.FileMetadata[0].Name)
			if err != nil {
				t.Fatalf("failed to extract from ztoc: %v", err)
			}
			diff := getPositionOfFirstDiffInByteSlice(data, b)
			if diff != -1 {
				t.Fatalf("data mismatched at %d. expected %v, got %v", diff, data, b)
			}
		})
	}
}

func TestZtocGenerationConsistency(t *testing.T) {
	for _, tc := range testZtocs {
		testZtocGenerationConsistency(t, tc.compressionAlgo, tc.tarGenerator)
	}
}

func testZtocGenerationConsistency(t *testing.T, compressionAlgo string, generator tarGenerator) {
	testcases := []struct {
		name       string
		tarEntries []testutil.TarEntry
		spanSize   int64
		tarName    string
	}{
		{
			name: "success generate consistent ztocs, two small files, span_size=64",
			tarEntries: []testutil.TarEntry{
				testutil.File("file1", string(testutil.RandomByteData(10))),
				testutil.File("file2", string(testutil.RandomByteData(15))),
			},
			spanSize: 64,
			tarName:  "testcase0",
		},
		{
			name: "success generate consistent ztocs, mixed files, span_size=64",
			tarEntries: []testutil.TarEntry{
				testutil.File("file1", string(testutil.RandomByteData(1000000))),
				testutil.File("file2", string(testutil.RandomByteData(2500000))),
				testutil.File("file3", string(testutil.RandomByteData(25))),
				testutil.File("file4", string(testutil.RandomByteData(88888))),
			},
			spanSize: 64,
			tarName:  "testcase1",
		},
	}

	ztocBuilder := NewBuilder("test")

	for _, tc := range testcases {
		tc := tc
		t.Run(fmt.Sprintf("%s-%s", compressionAlgo, tc.name), func(t *testing.T) {
			tarFilePath, _, fileNames := generator(t, tc.tarName, tc.tarEntries)
			defer os.Remove(tarFilePath)

			ztoc1, err := ztocBuilder.BuildZtoc(tarFilePath, tc.spanSize, WithCompression(compressionAlgo))
			if err != nil {
				t.Fatalf("can't build ztoc1: %v", err)
			}
			if ztoc1 == nil {
				t.Fatalf("ztoc1 should not be nil")
			}
			if len(ztoc1.FileMetadata) != len(fileNames) {
				t.Fatalf("ztoc1 metadata file count mismatch. expected: %d, actual: %d", len(fileNames), len(ztoc1.FileMetadata))
			}

			ztoc2, err := ztocBuilder.BuildZtoc(tarFilePath, tc.spanSize, WithCompression(compressionAlgo))
			if err != nil {
				t.Fatalf("can't build ztoc2: %v", err)
			}
			if ztoc2 == nil {
				t.Fatalf("ztoc2 should not be nil")
			}
			if len(ztoc2.FileMetadata) != len(fileNames) {
				t.Fatalf("ztoc2 should contain the metadata for %d files, but found %d", len(fileNames), len(ztoc2.FileMetadata))
			}

			// compare two ztocs
			if ztoc1.CompressedArchiveSize != ztoc2.CompressedArchiveSize {
				t.Fatalf("ztoc1.CompressedArchiveSize should be equal to ztoc2.CompressedArchiveSize")
			}
			if ztoc1.MaxSpanID != ztoc2.MaxSpanID {
				t.Fatalf("ztoc1.MaxSpanID should be equal to ztoc2.MaxSpanID")
			}
			if ztoc1.Version != ztoc2.Version {
				t.Fatalf("ztoc1.Checkpoints should be equal to ztoc2.Checkpoints")
			}
			for i := 0; i < len(ztoc1.FileMetadata); i++ {
				metadata1 := ztoc1.FileMetadata[i]
				metadata2 := ztoc2.FileMetadata[i]
				if !reflect.DeepEqual(metadata1, metadata2) {
					t.Fatalf("ztoc1.FileMetadata[%d] should be equal to ztoc2.FileMetadata[%d]", i, i)
				}
			}

			// Compare raw Checkpoints
			if !bytes.Equal(ztoc1.Checkpoints, ztoc2.Checkpoints) {
				diffIdx := getPositionOfFirstDiffInByteSlice(ztoc1.Checkpoints, ztoc2.Checkpoints)
				t.Fatalf("ztoc1.Checkpoints differ ztoc2.Checkpoints starting from position %d", diffIdx)
			}
		})
	}

}

func TestZtocGeneration(t *testing.T) {
	for _, tc := range testZtocs {
		testZtocGeneration(t, tc.compressionAlgo, tc.tarGenerator)
	}
}

func testZtocGeneration(t *testing.T, compressionAlgo string, generator tarGenerator) {
	testcases := []struct {
		name       string
		tarEntries []testutil.TarEntry
		spanSize   int64
		tarName    string
		buildTool  string
	}{
		{
			name: "success generate ztoc with multiple files, span_size=64KiB",
			tarEntries: []testutil.TarEntry{
				testutil.File("file1", string(testutil.RandomByteData(1080033))),
				testutil.File("file2", string(testutil.RandomByteData(6030502))),
				testutil.File("file3", string(testutil.RandomByteData(93000))),
				testutil.File("file4", string(testutil.RandomByteData(1070021))),
				testutil.File("file5", string(testutil.RandomByteData(55333))),
				testutil.File("file6", string(testutil.RandomByteData(1070))),
				testutil.File("file7", string(testutil.RandomByteData(999993))),
				testutil.File("file8", string(testutil.RandomByteData(1080033))),
				testutil.File("file9", string(testutil.RandomByteData(305))),
				testutil.File("filea", string(testutil.RandomByteData(3000))),
				testutil.File("fileb", string(testutil.RandomByteData(107))),
				testutil.File("filec", string(testutil.RandomByteData(559333))),
				testutil.File("filed", string(testutil.RandomByteData(100))),
				testutil.File("filee", string(testutil.RandomByteData(989993))),
			},
			spanSize:  65535,
			tarName:   "testcase0",
			buildTool: "AWS SOCI CLI",
		},
		{
			name: "success generate ztoc with two files, span_size=10kB",
			tarEntries: []testutil.TarEntry{
				testutil.File("file1", string(testutil.RandomByteData(10800))),
				testutil.File("file2", string(testutil.RandomByteData(10))),
			},
			spanSize:  10000,
			tarName:   "testcase1",
			buildTool: "foo",
		},
		{
			name: "success generate ztoc with two files, span_size=1MiB",
			tarEntries: []testutil.TarEntry{
				testutil.File("file1", string(testutil.RandomByteData(9911873))),
				testutil.File("file2", string(testutil.RandomByteData(800333))),
			},
			spanSize:  1 << 20,
			tarName:   "testcase2",
			buildTool: "bar",
		},
		{
			name: "success generate ztoc with one file, span_size=256kB",
			tarEntries: []testutil.TarEntry{
				testutil.File("file1", string(testutil.RandomByteData(5108033))),
			},
			spanSize: 256000,
			tarName:  "testcase3",
		},
	}

	for _, tc := range testcases {
		tc := tc
		t.Run(fmt.Sprintf("%s-%s", compressionAlgo, tc.name), func(t *testing.T) {
			tarFilePath, m, fileNames := generator(t, tc.tarName, tc.tarEntries)
			defer os.Remove(tarFilePath)

			ztoc, err := NewBuilder(tc.buildTool).BuildZtoc(tarFilePath, tc.spanSize, WithCompression(compressionAlgo))
			if err != nil {
				t.Fatalf("can't build ztoc: error=%v", err)
			}
			if ztoc == nil {
				t.Fatalf("ztoc should not be nil")
			}

			if ztoc.BuildToolIdentifier != tc.buildTool {
				t.Fatalf("ztoc build tool identifiers do not match: expected %s, got %s", tc.buildTool, ztoc.BuildToolIdentifier)
			}

			if len(ztoc.FileMetadata) != len(fileNames) {
				t.Fatalf("ztoc metadata count mismatch. expected: %d, actual: %d", len(fileNames), len(ztoc.FileMetadata))
			}

			for i := 0; i < len(ztoc.FileMetadata); i++ {
				compressedFileName := ztoc.FileMetadata[i].Name
				if compressedFileName != fileNames[i] {
					t.Fatalf("%d file name mismatch. expected: %s, actual: %s", i, fileNames[i], compressedFileName)
				}

				if int(ztoc.FileMetadata[i].UncompressedSize) != len(m[fileNames[i]]) {
					t.Fatalf("%d uncompressed content size mismatch. expected: %d, actual: %d",
						i, len(m[fileNames[i]]), int(ztoc.FileMetadata[i].UncompressedSize))
				}

				extractedBytes, err := ztoc.ExtractFromTarGz(tarFilePath, compressedFileName)
				if err != nil {
					t.Fatalf("could not extract file %s from %s using generated ztoc: %v", compressedFileName, tarFilePath, err)
				}

				if extractedBytes != string(m[fileNames[i]]) {
					t.Fatalf("the extracted content does not match. expected: %s, actual: %s",
						string(m[fileNames[i]]), extractedBytes)
				}
			}

		})
	}
}

func TestZtocSerialization(t *testing.T) {
	for _, tc := range testZtocs {
		testZtocSerialization(t, tc.compressionAlgo, tc.tarGenerator)
	}
}

func testZtocSerialization(t *testing.T, compressionAlgo string, generator tarGenerator) {
	testcases := []struct {
		name       string
		tarEntries []testutil.TarEntry
		spanSize   int64
		tarName    string
		buildTool  string
		version    string
		xattrs     map[string]string
	}{
		{
			name: "success serialize ztoc with multiple files, span_size=64KiB",
			tarEntries: []testutil.TarEntry{
				testutil.File("file1", string(testutil.RandomByteData(1080033))),
				testutil.File("file2", string(testutil.RandomByteData(6030502))),
				testutil.File("file3", string(testutil.RandomByteData(93000))),
				testutil.File("file4", string(testutil.RandomByteData(1070021))),
				testutil.File("file5", string(testutil.RandomByteData(55333))),
				testutil.File("file6", string(testutil.RandomByteData(1070))),
				testutil.File("file7", string(testutil.RandomByteData(999993))),
				testutil.File("file8", string(testutil.RandomByteData(1080033))),
				testutil.File("file9", string(testutil.RandomByteData(305))),
				testutil.File("filea", string(testutil.RandomByteData(3000))),
				testutil.File("fileb", string(testutil.RandomByteData(107))),
				testutil.File("filec", string(testutil.RandomByteData(559333))),
				testutil.File("filed", string(testutil.RandomByteData(100))),
				testutil.File("filee", string(testutil.RandomByteData(989993))),
			},
			spanSize:  65535,
			tarName:   "testcase0",
			buildTool: "AWS SOCI CLI",
			xattrs:    map[string]string{"testKey": "testValue"},
		},
	}

	for _, tc := range testcases {
		tc := tc
		t.Run(fmt.Sprintf("%s-%s", compressionAlgo, tc.name), func(t *testing.T) {
			tarFilePath, m, fileNames := generator(t, tc.tarName, tc.tarEntries)
			defer os.Remove(tarFilePath)

			createdZtoc, err := NewBuilder(tc.buildTool).BuildZtoc(tarFilePath, tc.spanSize, WithCompression(compressionAlgo))
			if err != nil {
				t.Fatalf("can't build ztoc: error=%v", err)
			}
			if createdZtoc == nil {
				t.Fatalf("ztoc should not be nil")
			}

			// append xattrs
			for i := 0; i < len(createdZtoc.FileMetadata); i++ {
				for key := range tc.xattrs {
					createdZtoc.FileMetadata[i].Xattrs = make(map[string]string)
					createdZtoc.FileMetadata[i].Xattrs[key] = tc.xattrs[key]
				}
			}

			// verify the correctness of created ztoc
			if createdZtoc.BuildToolIdentifier != tc.buildTool {
				t.Fatalf("ztoc build tool identifiers do not match: expected %s, got %s", tc.buildTool, createdZtoc.BuildToolIdentifier)
			}

			if len(createdZtoc.FileMetadata) != len(fileNames) {
				t.Fatalf("ztoc metadata count mismatch. expected: %d, actual: %d", len(fileNames), len(createdZtoc.FileMetadata))
			}

			if createdZtoc.CompressionAlgorithm != compressionAlgo {
				t.Fatalf("ztoc compression algorithm mismatch. expected: %s, actual: %s", compressionAlgo, createdZtoc.CompressionAlgorithm)
			}

			for i := 0; i < len(createdZtoc.FileMetadata); i++ {
				compressedFileName := createdZtoc.FileMetadata[i].Name
				if compressedFileName != fileNames[i] {
					t.Fatalf("%d file name mismatch. expected: %s, actual: %s", i, fileNames[i], compressedFileName)
				}

				if int(createdZtoc.FileMetadata[i].UncompressedSize) != len(m[fileNames[i]]) {
					t.Fatalf("%d uncompressed content size mismatch. expected: %d, actual: %d",
						i, len(m[fileNames[i]]), int(createdZtoc.FileMetadata[i].UncompressedSize))
				}

				extractedBytes, err := createdZtoc.ExtractFromTarGz(tarFilePath, compressedFileName)
				if err != nil {
					t.Fatalf("could not extract file %s from %s using generated ztoc: %v", compressedFileName, tarFilePath, err)
				}

				if extractedBytes != string(m[fileNames[i]]) {
					t.Fatalf("the extracted content does not match. expected: %s, actual: %s",
						string(m[fileNames[i]]), extractedBytes)
				}
			}
			// serialize
			r, _, err := Marshal(createdZtoc)
			if err != nil {
				t.Fatalf("error occurred when getting ztoc reader: %v", err)
			}

			// replacing the original ztoc with the read version of it
			readZtoc, err := Unmarshal(r)
			if err != nil {
				t.Fatalf("error occurred when getting ztoc: %v", err)
			}
			if readZtoc == nil {
				t.Fatalf("ztoc should not be nil")
			}

			if readZtoc.BuildToolIdentifier != createdZtoc.BuildToolIdentifier {
				t.Fatalf("serialized ztoc build tool identifiers do not match: expected %s, got %s", createdZtoc.BuildToolIdentifier, readZtoc.BuildToolIdentifier)
			}

			if readZtoc.Version != createdZtoc.Version {
				t.Fatalf("serialized ztoc version identifiers do not match: expected %s, got %s", createdZtoc.Version, readZtoc.Version)
			}

			if readZtoc.CompressedArchiveSize != createdZtoc.CompressedArchiveSize {
				t.Fatalf("readZtoc.CompressedArchiveSize should be equal to createdZtoc.CompressedArchiveSize")
			}
			if readZtoc.MaxSpanID != createdZtoc.MaxSpanID {
				t.Fatalf("readZtoc.MaxSpanID should be equal to createdZtoc.MaxSpanID")
			}

			if len(readZtoc.FileMetadata) != len(createdZtoc.FileMetadata) {
				t.Fatalf("ztoc metadata count mismatch. expected: %d, actual: %d", len(createdZtoc.FileMetadata), len(readZtoc.FileMetadata))
			}

			if readZtoc.CompressionAlgorithm != createdZtoc.CompressionAlgorithm {
				t.Fatalf("ztoc compression algorithm mismatch. expected: %s, actual: %s", createdZtoc.CompressionAlgorithm, readZtoc.CompressionAlgorithm)
			}

			for i := 0; i < len(readZtoc.FileMetadata); i++ {
				readZtocMetadata := readZtoc.FileMetadata[i]
				createdZtocMetadata := createdZtoc.FileMetadata[i]
				compressedFileName := readZtocMetadata.Name

				if !reflect.DeepEqual(readZtocMetadata, createdZtocMetadata) {
					if readZtocMetadata.Name != createdZtocMetadata.Name {
						t.Fatalf("createdZtoc.FileMetadata[%d].Name should be equal to readZtoc.FileMetadata[%d].Name", i, i)
					}
					if readZtocMetadata.Type != createdZtocMetadata.Type {
						t.Fatalf("createdZtoc.FileMetadata[%d].Type should be equal to readZtoc.FileMetadata[%d].Type", i, i)
					}
					if !readZtocMetadata.ModTime.Equal(createdZtocMetadata.ModTime) {
						t.Fatalf("createdZtoc.FileMetadata[%d].ModTime=%v should be equal to readZtoc.FileMetadata[%d].ModTime=%v", i, createdZtocMetadata.ModTime, i, readZtocMetadata.ModTime)
					}
					if readZtocMetadata.UncompressedOffset != createdZtocMetadata.UncompressedOffset {
						t.Fatalf("createdZtoc.FileMetadata[%d].UncompressedOffset should be equal to readZtoc.FileMetadata[%d].UncompressedOffset", i, i)
					}
					if readZtocMetadata.UncompressedSize != createdZtocMetadata.UncompressedSize {
						t.Fatalf("createdZtoc.FileMetadata[%d].UncompressedSize should be equal to readZtoc.FileMetadata[%d].UncompressedSize", i, i)
					}
					if readZtocMetadata.Linkname != createdZtocMetadata.Linkname {
						t.Fatalf("createdZtoc.FileMetadata[%d].Linkname should be equal to readZtoc.FileMetadata[%d].Linkname", i, i)
					}
					if readZtocMetadata.Mode != createdZtocMetadata.Mode {
						t.Fatalf("createdZtoc.FileMetadata[%d].Mode should be equal to readZtoc.FileMetadata[%d].Mode", i, i)
					}
					if readZtocMetadata.UID != createdZtocMetadata.UID {
						t.Fatalf("createdZtoc.FileMetadata[%d].UID should be equal to readZtoc.FileMetadata[%d].UID", i, i)
					}
					if readZtocMetadata.GID != createdZtocMetadata.GID {
						t.Fatalf("createdZtoc.FileMetadata[%d].GID should be equal to readZtoc.FileMetadata[%d].GID", i, i)
					}
					if readZtocMetadata.Uname != createdZtocMetadata.Uname {
						t.Fatalf("createdZtoc.FileMetadata[%d].Uname should be equal to readZtoc.FileMetadata[%d].Uname", i, i)
					}
					if readZtocMetadata.Gname != createdZtocMetadata.Gname {
						t.Fatalf("createdZtoc.FileMetadata[%d].Gname should be equal to readZtoc.FileMetadata[%d].Gname", i, i)
					}
					if readZtocMetadata.Devmajor != createdZtocMetadata.Devmajor {
						t.Fatalf("createdZtoc.FileMetadata[%d].Devmajor should be equal to readZtoc.FileMetadata[%d].Devmajor", i, i)
					}
					if readZtocMetadata.Devminor != createdZtocMetadata.Devminor {
						t.Fatalf("createdZtoc.FileMetadata[%d].Devminor should be equal to readZtoc.FileMetadata[%d].Devminor", i, i)
					}
				}

				extractedBytes, err := readZtoc.ExtractFromTarGz(tarFilePath, compressedFileName)
				if err != nil {
					t.Fatalf("could not extract file %s from %s using generated ztoc: %v", compressedFileName, tarFilePath, err)
				}

				if extractedBytes != string(m[fileNames[i]]) {
					t.Fatalf("the extracted content does not match. expected: %s, actual: %s",
						string(m[fileNames[i]]), extractedBytes)
				}
			}

			// Compare raw Checkpoints
			if !bytes.Equal(createdZtoc.Checkpoints, readZtoc.Checkpoints) {
				t.Fatalf("createdZtoc.Checkpoints must be identical to readZtoc.Checkpoints")
			}
		})
	}

}

func TestWriteZtoc(t *testing.T) {
	testCases := []struct {
		name                    string
		version                 Version
		checkpoints             []byte
		metadata                []FileMetadata
		compressedArchiveSize   compression.Offset
		uncompressedArchiveSize compression.Offset
		maxSpanID               compression.SpanID
		buildTool               string
		expDigest               string
		expSize                 int64
	}{
		{
			name:                    "success write succeeds - same digest and size " + string(Version09),
			version:                 Version09,
			checkpoints:             make([]byte, 1<<16),
			metadata:                make([]FileMetadata, 2),
			compressedArchiveSize:   2000000,
			uncompressedArchiveSize: 2500000,
			maxSpanID:               3,
			buildTool:               "AWS SOCI CLI",
			expDigest:               "sha256:eba28fdf50b1b57543f57dd051b2468c1d4f57b64d2006c75aa4de1d03e6c7ec",
			expSize:                 65928,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			toc := TOC{
				FileMetadata: tc.metadata,
			}
			compressionInfo := CompressionInfo{
				Checkpoints: tc.checkpoints,
				MaxSpanID:   tc.maxSpanID,
			}
			ztoc := &Ztoc{
				Version:                 tc.version,
				CompressedArchiveSize:   tc.compressedArchiveSize,
				UncompressedArchiveSize: tc.uncompressedArchiveSize,
				TOC:                     toc,
				CompressionInfo:         compressionInfo,
				BuildToolIdentifier:     tc.buildTool,
			}

			_, desc, err := Marshal(ztoc)
			if err != nil {
				t.Fatalf("error occurred when getting ztoc reader: %v", err)
			}

			if desc.Digest != digest.Digest(tc.expDigest) {
				t.Fatalf("unexpected digest; expected %v, got %v", tc.expDigest, desc.Digest)
			}

			if desc.Size != tc.expSize {
				t.Fatalf("unexpected size; expected %d, got %d", tc.expSize, desc.Size)
			}
		})
	}
}

func TestReadZtocInWrongFormat(t *testing.T) {
	testCases := []struct {
		name           string
		serializedZtoc []byte
	}{
		{
			name:           "ztoc unmarshal returns error and does not panic",
			serializedZtoc: testutil.RandomByteData(50000),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r := bytes.NewReader(tc.serializedZtoc)
			if _, err := Unmarshal(r); err == nil {
				t.Fatalf("expected error, but got nil")
			}
		})
	}
}

func getPositionOfFirstDiffInByteSlice(a, b []byte) int {
	sz := len(a)
	if len(b) < len(a) {
		sz = len(b)
	}
	for i := 0; i < sz; i++ {
		if a[i] != b[i] {
			return i
		}
	}

	return -1
}
