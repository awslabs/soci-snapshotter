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
	"errors"
	"testing"

	ztoc_flatbuffers "github.com/awslabs/soci-snapshotter/ztoc/fbs/ztoc"
	flatbuffers "github.com/google/flatbuffers/go"
)

// roundtrip serializes and then immediately deserializes
// a TOC. This is useful for verifying the deserialization logic.
func roundtrip(toc *TOC) (TOC, error) {
	builder := flatbuffers.NewBuilder(0)
	tocFB := tocToFlatbuffer(toc, builder)
	builder.Finish(tocFB)
	bytes := builder.FinishedBytes()

	fbtoc := ztoc_flatbuffers.GetRootAsTOC(bytes, 0)
	return flatbufferToTOC(fbtoc)
}

func TestPositiveTOCRoundtrip(t *testing.T) {
	// The two files in this test are aligned like a well-formatted tar file with one
	// header+data following the other. The offsets/sizes are mostly arbitrary except
	// that offsets are aligned to 512 as expected for tar.
	//
	//  1 tar hdr            1 data     2 tar hdr  2 tar data
	// |--------------------|----------|----------|--------------------|
	// 0                  1024       1536       2048                 3072

	testFile1 := FileMetadata{
		Name:               "file1",
		UncompressedOffset: 1024,
		UncompressedSize:   500,
		TarHeaderOffset:    0,
	}
	testFile2 := FileMetadata{
		Name:               "file2",
		UncompressedOffset: 2048,
		UncompressedSize:   1000,
		TarHeaderOffset:    testFile1.UncompressedOffset + 512,
	}
	tests := []struct {
		name        string
		toc         TOC
		expectedTOC TOC
	}{
		{
			name: "serialize -> deserialize produces same toc",
			toc: TOC{
				[]FileMetadata{
					testFile1,
					testFile2,
				},
			},
			expectedTOC: TOC{
				[]FileMetadata{
					testFile1,
					testFile2,
				},
			},
		},
		{
			name: "files are reordered by uncompressed offset",
			toc: TOC{
				[]FileMetadata{
					testFile2,
					testFile1,
				},
			},
			expectedTOC: TOC{
				[]FileMetadata{
					testFile1,
					testFile2,
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actualTOC, err := roundtrip(&tc.toc)
			if err != nil {
				t.Fatalf("toc deserialization produced unexpected error: %v", err)
			}
			if len(actualTOC.FileMetadata) != len(tc.expectedTOC.FileMetadata) {
				t.Fatalf("toc did not deserialize as expected. Actual %v, expected %v", actualTOC, tc.expectedTOC)
			}
			for i := range actualTOC.FileMetadata {
				actualEntry := actualTOC.FileMetadata[i]
				expectedEntry := tc.expectedTOC.FileMetadata[i]
				if !actualEntry.Equal(expectedEntry) {
					t.Fatalf("toc entry did not deserialize as expected. Actual %v, expected %v", actualEntry, expectedEntry)
				}
			}
		})
	}
}

func TestNegativeTOCRoundtrip(t *testing.T) {
	var anyError error
	// The two files in this test are aligned as if overlapped.
	//
	//  1 tar hdr            1 data
	// |--------------------|----------|
	//                      |----------|--------------------|
	//                       2 tar hdr  2 tar data
	//   0                1024       1536                 2560
	testFile1 := FileMetadata{
		UncompressedOffset: 1024,
		UncompressedSize:   500,
	}
	overlapTestFile1 := FileMetadata{
		UncompressedOffset: testFile1.UncompressedOffset,
		UncompressedSize:   500,
	}
	tests := []struct {
		name          string
		toc           TOC
		expectedError error
	}{
		{
			name: "overlapping files are invalid",
			toc: TOC{
				[]FileMetadata{
					testFile1,
					overlapTestFile1,
				},
			},
			expectedError: ErrInvalidTOCEntry,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := roundtrip(&tc.toc)
			if err == nil {
				t.Fatal("toc deserialization did not produce expected error")
			}
			if tc.expectedError != anyError && !errors.Is(err, tc.expectedError) {
				t.Fatalf("toc deserializtion did not produce the correct error. Actual %v, expected %v", err, tc.expectedError)
			}
		})
	}
}
