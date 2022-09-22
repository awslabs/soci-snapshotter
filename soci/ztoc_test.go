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

package soci

import (
	"bytes"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/opencontainers/go-digest"
)

func init() {
	rand.Seed(100)
}

func TestDecompress(t *testing.T) {
	testTag := "soci_test_data"
	fileDirName, err := GenerateTempTestingDir(t)
	if err != nil {
		t.Fatalf("cannot prepare testing directory, %s", err)
	}

	tarGzip, err := tempTarGz(fileDirName, testTag+".tar.gz")
	if err != nil {
		t.Fatalf("cannot prepare the .tar.gz file for testing")
	}
	defer os.Remove(*tarGzip)

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
		cfg := &buildConfig{}
		ztoc, err := BuildZtoc(*tarGzip, spansize, cfg)
		if err != nil {
			t.Fatalf("%s: can't build ztoc: %v", tc.name, err)
		}
		if ztoc == nil {
			t.Fatalf("%s: ztoc should not be nil", tc.name)
		}

		// read the directory, enumerate the files and try to decompress files
		dir, err := os.Open(fileDirName)
		if err != nil {
			t.Fatalf("%s: can't open the %s dir", tc.name, fileDirName)
		}
		defer dir.Close()
		fis, err := dir.Readdir(0)
		if err != nil {
			t.Fatalf("%s: can't read the %s dir", tc.name, fileDirName)
		}
		sort.Slice(fis, func(i, j int) bool {
			return fis[i].Name() < fis[j].Name()
		})

		extractConfigs := func() map[string](*FileExtractConfig) {
			configs := make(map[string](*FileExtractConfig))
			for _, m := range ztoc.Metadata {
				extractConfig := &FileExtractConfig{
					UncompressedSize:   m.UncompressedSize,
					UncompressedOffset: m.UncompressedOffset,
					SpanStart:          m.SpanStart,
					SpanEnd:            m.SpanEnd,
					IndexByteData:      ztoc.IndexByteData,
					CompressedFileSize: ztoc.CompressedFileSize,
					MaxSpanID:          ztoc.MaxSpanID,
				}
				configs[m.Name] = extractConfig
			}
			return configs
		}()

		for _, fi := range fis {
			fileName := filepath.Join(fileDirName, fi.Name())

			file, err := os.Open(*tarGzip)
			if err != nil {
				t.Fatalf("%s: could not open open the .tar.gz file", tc.name)
			}
			defer file.Close()
			var extractConfig *FileExtractConfig
			if extractConfig = extractConfigs[fileName]; extractConfig == nil {
				t.Fatalf("%s: could not find the metadata entry for the file %s", tc.name, fileName)
			}
			fi, err := file.Stat()
			if err != nil {
				t.Fatalf("%s: could not get the stat for the file %s", tc.name, *tarGzip)
			}
			sr := io.NewSectionReader(file, 0, fi.Size())
			extracted, err := ExtractFile(sr, extractConfig)
			if err != nil {
				t.Fatalf("%s: could not extract from tar gz", tc.name)
			}
			original, err := os.ReadFile(fileName)
			if err != nil {
				t.Fatalf("%s: could not read file %s", tc.name, fileName)
			}
			if !bytes.Equal(extracted, original) {
				for i := 0; i < len(original); i++ {
					if extracted[i] != original[i] {
						t.Fatalf("%s: span_size=%d: file %s extracted bytes != original bytes; byte %d is different",
							tc.name, tc.spanSize, fileName, i)
					}
				}
			}
		}

	}
}

func TestZtocGenerationConsistency(t *testing.T) {
	testcases := []struct {
		name         string
		fileContents []fileContent
		spanSize     int64
		targzName    string
	}{
		{
			name: "success generate consistent ztocs, two small files, span_size=64",
			fileContents: []fileContent{
				{fileName: "file1", content: genRandomByteData(10)},
				{fileName: "file2", content: genRandomByteData(15)},
			},
			spanSize:  64,
			targzName: "testcase0.tar.gz",
		},
		{
			name: "success generate consistent ztocs, mixed files, span_size=64",
			fileContents: []fileContent{
				{fileName: "file1", content: genRandomByteData(1000000)},
				{fileName: "file2", content: genRandomByteData(2500000)},
				{fileName: "file3", content: genRandomByteData(25)},
				{fileName: "file4", content: genRandomByteData(88888)},
			},
			spanSize:  64,
			targzName: "testcase1.tar.gz",
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			tarGzip, fileNames, err := buildTempTarGz(tc.fileContents, tc.targzName)
			if err != nil {
				t.Fatalf("cannot build targzip: %v", err)
			}
			defer os.Remove(*tarGzip)
			spansize := tc.spanSize
			cfg := &buildConfig{}
			ztoc1, err := BuildZtoc(*tarGzip, spansize, cfg)
			if err != nil {
				t.Fatalf("can't build ztoc1: %v", err)
			}
			if ztoc1 == nil {
				t.Fatalf("ztoc1 should not be nil")
			}
			if len(ztoc1.Metadata) != len(fileNames) {
				t.Fatalf("ztoc1 metadata file count mismatch. expected: %d, actual: %d", len(fileNames), len(ztoc1.Metadata))
			}

			ztoc2, err := BuildZtoc(*tarGzip, spansize, cfg)
			if err != nil {
				t.Fatalf("can't build ztoc2: %v", err)
			}
			if ztoc2 == nil {
				t.Fatalf("ztoc2 should not be nil")
			}
			if len(ztoc2.Metadata) != len(fileNames) {
				t.Fatalf("ztoc2 should contain the metadata for %d files, but found %d", len(fileNames), len(ztoc2.Metadata))
			}

			// compare two ztocs
			if ztoc1.CompressedFileSize != ztoc2.CompressedFileSize {
				t.Fatalf("ztoc1.CompressedFileSize should be equal to ztoc2.CompressedFileSize")
			}
			if ztoc1.MaxSpanID != ztoc2.MaxSpanID {
				t.Fatalf("ztoc1.MaxSpanID should be equal to ztoc2.MaxSpanID")
			}
			if ztoc1.Version != ztoc2.Version {
				t.Fatalf("ztoc1.IndexByteData should be equal to ztoc2.IndexByteData")
			}
			for i := 0; i < len(ztoc1.Metadata); i++ {
				metadata1 := ztoc1.Metadata[i]
				metadata2 := ztoc2.Metadata[i]
				if !reflect.DeepEqual(metadata1, metadata2) {
					t.Fatalf("ztoc1.Metadata[%d] should be equal to ztoc2.Metadata[%d]", i, i)
				}
			}

			// Compare raw IndexByteData
			if !bytes.Equal(ztoc1.IndexByteData, ztoc2.IndexByteData) {

				// compare IndexByteData within Go
				index1, err := unmarshalGzipIndex(ztoc1.IndexByteData[0])
				if err != nil {
					t.Fatalf("index from ztoc1 should contain data")
				}
				index2, err := unmarshalGzipIndex(ztoc2.IndexByteData[0])
				if err != nil {
					t.Fatalf("index from ztoc2 should contain data")
				}

				if index1.have != index2.have {
					t.Fatalf("index1.have=%d must be equal to index2.have=%d", index1.have, index2.have)
				}

				if index1.size != index2.size {
					t.Fatalf("index1.size=%d must be equal to index2.size=%d", index1.size, index2.size)
				}

				if index1.span_size != index2.span_size {
					t.Fatalf("index1.span_size=%d must be equal to index2.span_size=%d", index1.span_size, index2.span_size)
				}

				if len(index1.list) != len(index2.list) {
					t.Fatalf("len(index1.list)=%d must be equal to len(index2.list)=%d", len(index1.list), len(index2.list))
				}

				for i := 0; i < len(index1.list); i++ {
					indexPoint1 := index1.list[i]
					indexPoint2 := index2.list[i]

					if indexPoint1.bits != indexPoint2.bits {
						t.Fatalf("index1.list[%d].bits=%d must be equal to index2.list[%d].bits=%d", i, index1.list[i].bits, i, index2.list[i].bits)
					}

					if indexPoint1.in != indexPoint2.in {
						t.Fatalf("index1.list[%d].in=%d must be equal to index2.list[%d].in=%d", i, index1.list[i].in, i, index2.list[i].in)
					}

					if indexPoint1.out != indexPoint2.out {
						t.Fatalf("index1.list[%d].out=%d must be equal to index2.list[%d].out=%d", i, index1.list[i].out, i, index2.list[i].out)
					}

					if !reflect.DeepEqual(indexPoint1.window, indexPoint2.window) {
						t.Fatalf("index1.list[%d].window must be identical to index2.list[%d].window", i, i)
					}
				}
			}

		})
	}

}

func TestZtocGeneration(t *testing.T) {
	testcases := []struct {
		name         string
		fileContents []fileContent
		spanSize     int64
		targzName    string
		buildTool    string
	}{
		{
			name: "success generate ztoc with multiple files, span_size=64KiB",
			fileContents: []fileContent{
				{fileName: "file1", content: genRandomByteData(10800333)},
				{fileName: "file2", content: genRandomByteData(60305021)},
				{fileName: "file3", content: genRandomByteData(93000)},
				{fileName: "file4", content: genRandomByteData(10700210)},
				{fileName: "file5", content: genRandomByteData(55333)},
				{fileName: "file6", content: genRandomByteData(1070)},
				{fileName: "file7", content: genRandomByteData(9999937)},
				{fileName: "file8", content: genRandomByteData(10800333)},
				{fileName: "file9", content: genRandomByteData(305)},
				{fileName: "filea", content: genRandomByteData(3000)},
				{fileName: "fileb", content: genRandomByteData(107)},
				{fileName: "filec", content: genRandomByteData(559333)},
				{fileName: "filed", content: genRandomByteData(100)},
				{fileName: "filee", content: genRandomByteData(9899937)},
			},
			spanSize:  65535,
			targzName: "testcase0.tar.gz",
			buildTool: "AWS SOCI CLI",
		},
		{
			name: "success generate ztoc with two files, span_size=10kB",
			fileContents: []fileContent{
				{fileName: "file1", content: genRandomByteData(10800)},
				{fileName: "file2", content: genRandomByteData(10)},
			},
			spanSize:  10000,
			targzName: "testcase1.tar.gz",
			buildTool: "foo",
		},
		{
			name: "success generate ztoc with two files, span_size=1MiB",
			fileContents: []fileContent{
				{fileName: "file1", content: genRandomByteData(9911873)},
				{fileName: "file2", content: genRandomByteData(800333)},
			},
			spanSize:  1 << 20,
			targzName: "testcase2.tar.gz",
			buildTool: "bar",
		},
		{
			name: "success generate ztoc with one file, span_size=256kB",
			fileContents: []fileContent{
				{fileName: "file1", content: genRandomByteData(51080333)},
			},
			spanSize:  256000,
			targzName: "testcase3.tar.gz",
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			tarGzip, fileNames, err := buildTempTarGz(tc.fileContents, tc.targzName)
			if err != nil {
				t.Fatalf("cannot build targzip: error=%v", err)
			}
			defer os.Remove(*tarGzip)
			spansize := tc.spanSize
			cfg := &buildConfig{
				buildToolIdentifier: tc.buildTool,
			}
			ztoc, err := BuildZtoc(*tarGzip, spansize, cfg)
			if err != nil {
				t.Fatalf("can't build ztoc: error=%v", err)
			}
			if ztoc == nil {
				t.Fatalf("ztoc should not be nil")
			}

			if ztoc.BuildToolIdentifier != tc.buildTool {
				t.Fatalf("ztoc build tool identifiers do not match: expected %s, got %s", tc.buildTool, ztoc.BuildToolIdentifier)
			}

			if len(ztoc.Metadata) != len(fileNames) {
				t.Fatalf("ztoc metadata count mismatch. expected: %d, actual: %d", len(fileNames), len(ztoc.Metadata))
			}

			for i := 0; i < len(ztoc.Metadata); i++ {
				compressedFileName := ztoc.Metadata[i].Name
				if compressedFileName != fileNames[i] {
					t.Fatalf("%d file name mismatch. expected: %s, actual: %s", i, fileNames[i], compressedFileName)
				}

				if int(ztoc.Metadata[i].UncompressedSize) != len(tc.fileContents[i].content) {
					t.Fatalf("%d uncompressed content size mismatch. expected: %d, actual: %d",
						i, len(tc.fileContents[i].content), int(ztoc.Metadata[i].UncompressedSize))
				}

				extractedBytes, err := ExtractFromTarGz(*tarGzip, ztoc, compressedFileName)
				if err != nil {
					t.Fatalf("could not extract file %s from %s using generated ztoc: %v", compressedFileName, *tarGzip, err)
				}

				if extractedBytes != string(tc.fileContents[i].content) {
					t.Fatalf("the extracted content does not match. expected: %s, actual: %s",
						string(tc.fileContents[i].content), extractedBytes)
				}
			}

		})
	}
}

func TestZtocSerialization(t *testing.T) {
	testcases := []struct {
		name         string
		fileContents []fileContent
		spanSize     int64
		targzName    string
		buildTool    string
		version      string
		xattrs       map[string]string
	}{
		{
			name: "success serialize ztoc with multiple files, span_size=64KiB",
			fileContents: []fileContent{
				{fileName: "file1", content: genRandomByteData(10800333)},
				{fileName: "file2", content: genRandomByteData(60305021)},
				{fileName: "file3", content: genRandomByteData(93000)},
				{fileName: "file4", content: genRandomByteData(10700210)},
				{fileName: "file5", content: genRandomByteData(55333)},
				{fileName: "file6", content: genRandomByteData(1070)},
				{fileName: "file7", content: genRandomByteData(9999937)},
				{fileName: "file8", content: genRandomByteData(10800333)},
				{fileName: "file9", content: genRandomByteData(305)},
				{fileName: "filea", content: genRandomByteData(3000)},
				{fileName: "fileb", content: genRandomByteData(107)},
				{fileName: "filec", content: genRandomByteData(559333)},
				{fileName: "filed", content: genRandomByteData(100)},
				{fileName: "filee", content: genRandomByteData(9899937)},
			},
			spanSize:  65535,
			targzName: "testcase0.tar.gz",
			buildTool: "AWS SOCI CLI",
			xattrs:    map[string]string{"testKey": "testValue"},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			tarGzip, fileNames, err := buildTempTarGz(tc.fileContents, tc.targzName)
			if err != nil {
				t.Fatalf("cannot build targzip: error=%v", err)
			}
			defer os.Remove(*tarGzip)
			spansize := tc.spanSize
			cfg := &buildConfig{
				buildToolIdentifier: tc.buildTool,
				buildToolVersion:    tc.version,
			}
			createdZtoc, err := BuildZtoc(*tarGzip, spansize, cfg)
			if err != nil {
				t.Fatalf("can't build ztoc: error=%v", err)
			}
			if createdZtoc == nil {
				t.Fatalf("ztoc should not be nil")
			}

			// append xattrs
			for i := 0; i < len(createdZtoc.Metadata); i++ {
				for key := range tc.xattrs {
					createdZtoc.Metadata[i].Xattrs = make(map[string]string)
					createdZtoc.Metadata[i].Xattrs[key] = tc.xattrs[key]
				}
			}

			// verify the correctness of created ztoc
			if createdZtoc.BuildToolIdentifier != tc.buildTool {
				t.Fatalf("ztoc build tool identifiers do not match: expected %s, got %s", tc.buildTool, createdZtoc.BuildToolIdentifier)
			}

			if len(createdZtoc.Metadata) != len(fileNames) {
				t.Fatalf("ztoc metadata count mismatch. expected: %d, actual: %d", len(fileNames), len(createdZtoc.Metadata))
			}

			for i := 0; i < len(createdZtoc.Metadata); i++ {
				compressedFileName := createdZtoc.Metadata[i].Name
				if compressedFileName != fileNames[i] {
					t.Fatalf("%d file name mismatch. expected: %s, actual: %s", i, fileNames[i], compressedFileName)
				}

				if int(createdZtoc.Metadata[i].UncompressedSize) != len(tc.fileContents[i].content) {
					t.Fatalf("%d uncompressed content size mismatch. expected: %d, actual: %d",
						i, len(tc.fileContents[i].content), int(createdZtoc.Metadata[i].UncompressedSize))
				}

				extractedBytes, err := ExtractFromTarGz(*tarGzip, createdZtoc, compressedFileName)
				if err != nil {
					t.Fatalf("could not extract file %s from %s using generated ztoc: %v", compressedFileName, *tarGzip, err)
				}

				if extractedBytes != string(tc.fileContents[i].content) {
					t.Fatalf("the extracted content does not match. expected: %s, actual: %s",
						string(tc.fileContents[i].content), extractedBytes)
				}
			}
			// serialize
			r, _, err := NewZtocReader(createdZtoc)
			if err != nil {
				t.Fatalf("error occurred when getting ztoc reader: %v", err)
			}

			// replacing the original ztoc with the read version of it
			readZtoc, err := GetZtoc(r)
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

			if readZtoc.CompressedFileSize != createdZtoc.CompressedFileSize {
				t.Fatalf("readZtoc.CompressedFileSize should be equal to createdZtoc.CompressedFileSize")
			}
			if readZtoc.MaxSpanID != createdZtoc.MaxSpanID {
				t.Fatalf("readZtoc.MaxSpanID should be equal to createdZtoc.MaxSpanID")
			}

			if len(readZtoc.Metadata) != len(createdZtoc.Metadata) {
				t.Fatalf("ztoc metadata count mismatch. expected: %d, actual: %d", len(createdZtoc.Metadata), len(readZtoc.Metadata))
			}

			for i := 0; i < len(readZtoc.Metadata); i++ {
				readZtocMetadata := readZtoc.Metadata[i]
				createdZtocMetadata := createdZtoc.Metadata[i]
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
					if readZtocMetadata.SpanStart != createdZtocMetadata.SpanStart {
						t.Fatalf("createdZtoc.Metadata[%d].SpanStart should be equal to readZtoc.Metadata[%d].SpanStart", i, i)
					}
					if readZtocMetadata.SpanEnd != createdZtocMetadata.SpanEnd {
						t.Fatalf("createdZtoc.Metadata[%d].SpanEnd should be equal to readZtoc.Metadata[%d].SpanEnd", i, i)
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

				extractedBytes, err := ExtractFromTarGz(*tarGzip, readZtoc, compressedFileName)
				if err != nil {
					t.Fatalf("could not extract file %s from %s using generated ztoc: %v", compressedFileName, *tarGzip, err)
				}

				if extractedBytes != string(tc.fileContents[i].content) {
					t.Fatalf("the extracted content does not match. expected: %s, actual: %s",
						string(tc.fileContents[i].content), extractedBytes)
				}
			}

			// Compare raw IndexByteData
			if !bytes.Equal(createdZtoc.IndexByteData, readZtoc.IndexByteData) {
				t.Fatalf("createdZtoc.IndexByteData must be identical to readZtoc.IndexByteData")
			}
		})
	}

}

func TestWriteZtoc(t *testing.T) {
	testCases := []struct {
		name                 string
		version              string
		indexByteData        []byte
		metadata             []FileMetadata
		compressedFileSize   FileSize
		uncompressedFileSize FileSize
		maxSpanID            SpanID
		buildTool            string
		expDigest            string
		expSize              int64
	}{
		{
			name:                 "success write succeeds - same digest and size",
			version:              "0.1",
			indexByteData:        make([]byte, 1<<16),
			metadata:             make([]FileMetadata, 2),
			compressedFileSize:   2000000,
			uncompressedFileSize: 2500000,
			maxSpanID:            3,
			buildTool:            "AWS SOCI CLI",
			expDigest:            "sha256:ee2fd7cf479ccbbe769fa67120a04b420133a7425ea0fa03791ed9ffe9b8340b",
			expSize:              65936,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ztoc := &Ztoc{
				Version:              tc.version,
				IndexByteData:        tc.indexByteData,
				Metadata:             tc.metadata,
				CompressedFileSize:   tc.compressedFileSize,
				UncompressedFileSize: tc.uncompressedFileSize,
				MaxSpanID:            tc.maxSpanID,
				BuildToolIdentifier:  tc.buildTool,
			}

			_, desc, err := NewZtocReader(ztoc)
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

func genRandomByteData(size int) []byte {
	b := make([]byte, size)
	rand.Read(b)
	return b
}
