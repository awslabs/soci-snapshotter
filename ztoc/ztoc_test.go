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
	"io"
	"math/rand"
	"os"
	"reflect"
	"testing"

	"github.com/awslabs/soci-snapshotter/compression"
	"github.com/awslabs/soci-snapshotter/util/testutil"
	"github.com/opencontainers/go-digest"
)

func init() {
	rand.Seed(100)
}

func TestDecompress(t *testing.T) {
	testTag := "soci_test_data"
	tarEntries := []testutil.TarEntry{
		testutil.File("smallfile", string(testutil.RandomByteDataRange(1, 100))),
		testutil.File("mediumfile", string(testutil.RandomByteDataRange(10000, 128000))),
		testutil.File("largefile", string(testutil.RandomByteDataRange(350000, 500000))),
		testutil.File("jumbofile", string(testutil.RandomByteDataRange(3000000, 5000000))),
	}

	tarReader := testutil.BuildTarGz(tarEntries, gzip.DefaultCompression)
	tarGzFilePath, err := testutil.WriteTarToTempFile(testTag+".tar.gz", tarReader)
	if err != nil {
		t.Fatalf("cannot prepare the .tar.gz file for testing")
	}
	m, fileNames, err := testutil.GetFilesAndContentsWithinTarGz(tarGzFilePath)
	if err != nil {
		t.Fatalf("failed to get targz files and their contents: %v", err)
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

	for _, tc := range tests {
		spansize := tc.spanSize
		ztoc, err := BuildZtoc(tarGzFilePath, spansize, "test")
		if err != nil {
			t.Fatalf("%s: can't build ztoc: %v", tc.name, err)
		}
		if ztoc == nil {
			t.Fatalf("%s: ztoc should not be nil", tc.name)
		}

		extractConfigs := func() map[string](*FileExtractConfig) {
			configs := make(map[string](*FileExtractConfig))
			for _, m := range ztoc.TOC.Metadata {
				extractConfig := &FileExtractConfig{
					UncompressedSize:      m.UncompressedSize,
					UncompressedOffset:    m.UncompressedOffset,
					Checkpoints:           ztoc.CompressionInfo.Checkpoints,
					CompressedArchiveSize: ztoc.CompressedArchiveSize,
					MaxSpanID:             ztoc.CompressionInfo.MaxSpanID,
				}
				configs[m.Name] = extractConfig
			}
			return configs
		}()

		for _, f := range fileNames {
			file, err := os.Open(tarGzFilePath)
			if err != nil {
				t.Fatalf("%s: could not open open the .tar.gz file", tc.name)
			}
			defer file.Close()
			var extractConfig *FileExtractConfig
			if extractConfig = extractConfigs[f]; extractConfig == nil {
				t.Fatalf("%s: could not find the metadata entry for the file %s", tc.name, f)
			}
			fi, err := file.Stat()
			if err != nil {
				t.Fatalf("%s: could not get the stat for the file %s", tc.name, tarGzFilePath)
			}
			sr := io.NewSectionReader(file, 0, fi.Size())
			extracted, err := ExtractFile(sr, extractConfig)
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
			ztoc, sr, err := BuildZtocReader(
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
			metadata := ztoc.TOC.Metadata[0]
			config := FileExtractConfig{
				UncompressedSize:      metadata.UncompressedSize,
				UncompressedOffset:    metadata.UncompressedOffset,
				Checkpoints:           ztoc.CompressionInfo.Checkpoints,
				CompressedArchiveSize: ztoc.CompressedArchiveSize,
				MaxSpanID:             ztoc.CompressionInfo.MaxSpanID,
			}
			b, err := ExtractFile(sr, &config)
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
	testcases := []struct {
		name       string
		tarEntries []testutil.TarEntry
		spanSize   int64
		targzName  string
	}{
		{
			name: "success generate consistent ztocs, two small files, span_size=64",
			tarEntries: []testutil.TarEntry{
				testutil.File("file1", string(testutil.RandomByteData(10))),
				testutil.File("file2", string(testutil.RandomByteData(15))),
			},
			spanSize:  64,
			targzName: "testcase0.tar.gz",
		},
		{
			name: "success generate consistent ztocs, mixed files, span_size=64",
			tarEntries: []testutil.TarEntry{
				testutil.File("file1", string(testutil.RandomByteData(1000000))),
				testutil.File("file2", string(testutil.RandomByteData(2500000))),
				testutil.File("file3", string(testutil.RandomByteData(25))),
				testutil.File("file4", string(testutil.RandomByteData(88888))),
			},
			spanSize:  64,
			targzName: "testcase1.tar.gz",
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			tarReader := testutil.BuildTarGz(tc.tarEntries, gzip.DefaultCompression)
			tarGzFilePath, err := testutil.WriteTarToTempFile(tc.targzName, tarReader)
			if err != nil {
				t.Fatalf("cannot prepare the .tar.gz file for testing")
			}
			defer os.Remove(tarGzFilePath)
			_, fileNames, err := testutil.GetFilesAndContentsWithinTarGz(tarGzFilePath)
			if err != nil {
				t.Fatalf("failed to get targz files and their contents: %v", err)
			}
			spansize := tc.spanSize
			ztoc1, err := BuildZtoc(tarGzFilePath, spansize, "test")
			if err != nil {
				t.Fatalf("can't build ztoc1: %v", err)
			}
			if ztoc1 == nil {
				t.Fatalf("ztoc1 should not be nil")
			}
			if len(ztoc1.TOC.Metadata) != len(fileNames) {
				t.Fatalf("ztoc1 metadata file count mismatch. expected: %d, actual: %d", len(fileNames), len(ztoc1.TOC.Metadata))
			}

			ztoc2, err := BuildZtoc(tarGzFilePath, spansize, "test")
			if err != nil {
				t.Fatalf("can't build ztoc2: %v", err)
			}
			if ztoc2 == nil {
				t.Fatalf("ztoc2 should not be nil")
			}
			if len(ztoc2.TOC.Metadata) != len(fileNames) {
				t.Fatalf("ztoc2 should contain the metadata for %d files, but found %d", len(fileNames), len(ztoc2.TOC.Metadata))
			}

			// compare two ztocs
			if ztoc1.CompressedArchiveSize != ztoc2.CompressedArchiveSize {
				t.Fatalf("ztoc1.CompressedArchiveSize should be equal to ztoc2.CompressedArchiveSize")
			}
			if ztoc1.CompressionInfo.MaxSpanID != ztoc2.CompressionInfo.MaxSpanID {
				t.Fatalf("ztoc1.MaxSpanID should be equal to ztoc2.MaxSpanID")
			}
			if ztoc1.Version != ztoc2.Version {
				t.Fatalf("ztoc1.Checkpoints should be equal to ztoc2.Checkpoints")
			}
			for i := 0; i < len(ztoc1.TOC.Metadata); i++ {
				metadata1 := ztoc1.TOC.Metadata[i]
				metadata2 := ztoc2.TOC.Metadata[i]
				if !reflect.DeepEqual(metadata1, metadata2) {
					t.Fatalf("ztoc1.Metadata[%d] should be equal to ztoc2.Metadata[%d]", i, i)
				}
			}

			// Compare raw Checkpoints
			if !bytes.Equal(ztoc1.CompressionInfo.Checkpoints, ztoc2.CompressionInfo.Checkpoints) {
				diffIdx := getPositionOfFirstDiffInByteSlice(ztoc1.CompressionInfo.Checkpoints, ztoc2.CompressionInfo.Checkpoints)
				t.Fatalf("ztoc1.CompressionInfo.Checkpoints differ ztoc2.CompressionInfo.Checkpoints starting from position %d", diffIdx)
			}

		})
	}

}

func TestZtocGeneration(t *testing.T) {
	testcases := []struct {
		name       string
		tarEntries []testutil.TarEntry
		spanSize   int64
		targzName  string
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
			targzName: "testcase0.tar.gz",
			buildTool: "AWS SOCI CLI",
		},
		{
			name: "success generate ztoc with two files, span_size=10kB",
			tarEntries: []testutil.TarEntry{
				testutil.File("file1", string(testutil.RandomByteData(10800))),
				testutil.File("file2", string(testutil.RandomByteData(10))),
			},
			spanSize:  10000,
			targzName: "testcase1.tar.gz",
			buildTool: "foo",
		},
		{
			name: "success generate ztoc with two files, span_size=1MiB",
			tarEntries: []testutil.TarEntry{
				testutil.File("file1", string(testutil.RandomByteData(9911873))),
				testutil.File("file2", string(testutil.RandomByteData(800333))),
			},
			spanSize:  1 << 20,
			targzName: "testcase2.tar.gz",
			buildTool: "bar",
		},
		{
			name: "success generate ztoc with one file, span_size=256kB",
			tarEntries: []testutil.TarEntry{
				testutil.File("file1", string(testutil.RandomByteData(5108033))),
			},
			spanSize:  256000,
			targzName: "testcase3.tar.gz",
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			tarReader := testutil.BuildTarGz(tc.tarEntries, gzip.DefaultCompression)
			tarGzFilePath, err := testutil.WriteTarToTempFile(tc.targzName, tarReader)
			if err != nil {
				t.Fatalf("cannot prepare the .tar.gz file for testing")
			}
			defer os.Remove(tarGzFilePath)
			m, fileNames, err := testutil.GetFilesAndContentsWithinTarGz(tarGzFilePath)
			if err != nil {
				t.Fatalf("failed to get targz files and their contents: %v", err)
			}
			spansize := tc.spanSize

			ztoc, err := BuildZtoc(tarGzFilePath, spansize, tc.buildTool)
			if err != nil {
				t.Fatalf("can't build ztoc: error=%v", err)
			}
			if ztoc == nil {
				t.Fatalf("ztoc should not be nil")
			}

			if ztoc.BuildToolIdentifier != tc.buildTool {
				t.Fatalf("ztoc build tool identifiers do not match: expected %s, got %s", tc.buildTool, ztoc.BuildToolIdentifier)
			}

			if len(ztoc.TOC.Metadata) != len(fileNames) {
				t.Fatalf("ztoc metadata count mismatch. expected: %d, actual: %d", len(fileNames), len(ztoc.TOC.Metadata))
			}

			for i := 0; i < len(ztoc.TOC.Metadata); i++ {
				compressedFileName := ztoc.TOC.Metadata[i].Name
				if compressedFileName != fileNames[i] {
					t.Fatalf("%d file name mismatch. expected: %s, actual: %s", i, fileNames[i], compressedFileName)
				}

				if int(ztoc.TOC.Metadata[i].UncompressedSize) != len(m[fileNames[i]]) {
					t.Fatalf("%d uncompressed content size mismatch. expected: %d, actual: %d",
						i, len(m[fileNames[i]]), int(ztoc.TOC.Metadata[i].UncompressedSize))
				}

				extractedBytes, err := ExtractFromTarGz(tarGzFilePath, ztoc, compressedFileName)
				if err != nil {
					t.Fatalf("could not extract file %s from %s using generated ztoc: %v", compressedFileName, tarGzFilePath, err)
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
	testcases := []struct {
		name       string
		tarEntries []testutil.TarEntry
		spanSize   int64
		targzName  string
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
			targzName: "testcase0.tar.gz",
			buildTool: "AWS SOCI CLI",
			xattrs:    map[string]string{"testKey": "testValue"},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			tarReader := testutil.BuildTarGz(tc.tarEntries, gzip.DefaultCompression)
			tarGzFilePath, err := testutil.WriteTarToTempFile(tc.targzName, tarReader)
			if err != nil {
				t.Fatalf("cannot prepare the .tar.gz file for testing")
			}
			defer os.Remove(tarGzFilePath)

			m, fileNames, err := testutil.GetFilesAndContentsWithinTarGz(tarGzFilePath)
			if err != nil {
				t.Fatalf("failed to get targz files and their contents: %v", err)
			}
			spansize := tc.spanSize
			createdZtoc, err := BuildZtoc(tarGzFilePath, spansize, tc.buildTool)
			if err != nil {
				t.Fatalf("can't build ztoc: error=%v", err)
			}
			if createdZtoc == nil {
				t.Fatalf("ztoc should not be nil")
			}

			// append xattrs
			for i := 0; i < len(createdZtoc.TOC.Metadata); i++ {
				for key := range tc.xattrs {
					createdZtoc.TOC.Metadata[i].Xattrs = make(map[string]string)
					createdZtoc.TOC.Metadata[i].Xattrs[key] = tc.xattrs[key]
				}
			}

			// verify the correctness of created ztoc
			if createdZtoc.BuildToolIdentifier != tc.buildTool {
				t.Fatalf("ztoc build tool identifiers do not match: expected %s, got %s", tc.buildTool, createdZtoc.BuildToolIdentifier)
			}

			if len(createdZtoc.TOC.Metadata) != len(fileNames) {
				t.Fatalf("ztoc metadata count mismatch. expected: %d, actual: %d", len(fileNames), len(createdZtoc.TOC.Metadata))
			}

			for i := 0; i < len(createdZtoc.TOC.Metadata); i++ {
				compressedFileName := createdZtoc.TOC.Metadata[i].Name
				if compressedFileName != fileNames[i] {
					t.Fatalf("%d file name mismatch. expected: %s, actual: %s", i, fileNames[i], compressedFileName)
				}

				if int(createdZtoc.TOC.Metadata[i].UncompressedSize) != len(m[fileNames[i]]) {
					t.Fatalf("%d uncompressed content size mismatch. expected: %d, actual: %d",
						i, len(m[fileNames[i]]), int(createdZtoc.TOC.Metadata[i].UncompressedSize))
				}

				extractedBytes, err := ExtractFromTarGz(tarGzFilePath, createdZtoc, compressedFileName)
				if err != nil {
					t.Fatalf("could not extract file %s from %s using generated ztoc: %v", compressedFileName, tarGzFilePath, err)
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
			if readZtoc.CompressionInfo.MaxSpanID != createdZtoc.CompressionInfo.MaxSpanID {
				t.Fatalf("readZtoc.MaxSpanID should be equal to createdZtoc.MaxSpanID")
			}

			if len(readZtoc.TOC.Metadata) != len(createdZtoc.TOC.Metadata) {
				t.Fatalf("ztoc metadata count mismatch. expected: %d, actual: %d", len(createdZtoc.TOC.Metadata), len(readZtoc.TOC.Metadata))
			}

			for i := 0; i < len(readZtoc.TOC.Metadata); i++ {
				readZtocMetadata := readZtoc.TOC.Metadata[i]
				createdZtocMetadata := createdZtoc.TOC.Metadata[i]
				compressedFileName := readZtocMetadata.Name

				if !reflect.DeepEqual(readZtocMetadata, createdZtocMetadata) {
					if readZtocMetadata.Name != createdZtocMetadata.Name {
						t.Fatalf("createdZtoc.Metadata[%d].Name should be equal to readZtoc.Metadata[%d].Name", i, i)
					}
					if readZtocMetadata.Type != createdZtocMetadata.Type {
						t.Fatalf("createdZtoc.Metadata[%d].Type should be equal to readZtoc.Metadata[%d].Type", i, i)
					}
					if !readZtocMetadata.ModTime.Equal(createdZtocMetadata.ModTime) {
						t.Fatalf("createdZtoc.Metadata[%d].ModTime=%v should be equal to readZtoc.Metadata[%d].ModTime=%v", i, createdZtocMetadata.ModTime, i, readZtocMetadata.ModTime)
					}
					if readZtocMetadata.UncompressedOffset != createdZtocMetadata.UncompressedOffset {
						t.Fatalf("createdZtoc.Metadata[%d].UncompressedOffset should be equal to readZtoc.Metadata[%d].UncompressedOffset", i, i)
					}
					if readZtocMetadata.UncompressedSize != createdZtocMetadata.UncompressedSize {
						t.Fatalf("createdZtoc.Metadata[%d].UncompressedSize should be equal to readZtoc.Metadata[%d].UncompressedSize", i, i)
					}
					if readZtocMetadata.Linkname != createdZtocMetadata.Linkname {
						t.Fatalf("createdZtoc.Metadata[%d].Linkname should be equal to readZtoc.Metadata[%d].Linkname", i, i)
					}
					if readZtocMetadata.Mode != createdZtocMetadata.Mode {
						t.Fatalf("createdZtoc.Metadata[%d].Mode should be equal to readZtoc.Metadata[%d].Mode", i, i)
					}
					if readZtocMetadata.UID != createdZtocMetadata.UID {
						t.Fatalf("createdZtoc.Metadata[%d].UID should be equal to readZtoc.Metadata[%d].UID", i, i)
					}
					if readZtocMetadata.GID != createdZtocMetadata.GID {
						t.Fatalf("createdZtoc.Metadata[%d].GID should be equal to readZtoc.Metadata[%d].GID", i, i)
					}
					if readZtocMetadata.Uname != createdZtocMetadata.Uname {
						t.Fatalf("createdZtoc.Metadata[%d].Uname should be equal to readZtoc.Metadata[%d].Uname", i, i)
					}
					if readZtocMetadata.Gname != createdZtocMetadata.Gname {
						t.Fatalf("createdZtoc.Metadata[%d].Gname should be equal to readZtoc.Metadata[%d].Gname", i, i)
					}
					if readZtocMetadata.Devmajor != createdZtocMetadata.Devmajor {
						t.Fatalf("createdZtoc.Metadata[%d].Devmajor should be equal to readZtoc.Metadata[%d].Devmajor", i, i)
					}
					if readZtocMetadata.Devminor != createdZtocMetadata.Devminor {
						t.Fatalf("createdZtoc.Metadata[%d].Devminor should be equal to readZtoc.Metadata[%d].Devminor", i, i)
					}
				}

				extractedBytes, err := ExtractFromTarGz(tarGzFilePath, readZtoc, compressedFileName)
				if err != nil {
					t.Fatalf("could not extract file %s from %s using generated ztoc: %v", compressedFileName, tarGzFilePath, err)
				}

				if extractedBytes != string(m[fileNames[i]]) {
					t.Fatalf("the extracted content does not match. expected: %s, actual: %s",
						string(m[fileNames[i]]), extractedBytes)
				}
			}

			// Compare raw Checkpoints
			if !bytes.Equal(createdZtoc.CompressionInfo.Checkpoints, readZtoc.CompressionInfo.Checkpoints) {
				t.Fatalf("createdZtoc.Checkpoints must be identical to readZtoc.Checkpoints")
			}
		})
	}

}

func TestWriteZtoc(t *testing.T) {
	testCases := []struct {
		name                    string
		version                 string
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
			name:                    "success write succeeds - same digest and size",
			version:                 "0.1",
			checkpoints:             make([]byte, 1<<16),
			metadata:                make([]FileMetadata, 2),
			compressedArchiveSize:   2000000,
			uncompressedArchiveSize: 2500000,
			maxSpanID:               3,
			buildTool:               "AWS SOCI CLI",
			expDigest:               "sha256:9c8effca78ecc82fad49a4d591dd81615555b05eaaae605a621c927ecf6fc1e7",
			expSize:                 65928,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			toc := TOC{
				Metadata: tc.metadata,
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
