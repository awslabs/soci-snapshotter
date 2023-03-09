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

package compression

import (
	"testing"
)

func TestNewGzipZinfo(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name        string
		zinfoBytes  []byte
		expectError bool
	}{
		{
			name:        "nil zinfoBytes should return error",
			zinfoBytes:  nil,
			expectError: true,
		},
		{
			name:        "empty zinfoBytes should return error",
			zinfoBytes:  []byte{},
			expectError: true,
		},
		{
			name:        "zinfoBytes with less than 'header size' bytes header should return error",
			zinfoBytes:  []byte{00},
			expectError: true,
		},
		{
			name: "zinfoBytes with too few checkpoints should return error",
			zinfoBytes: []byte{
				0xFF, 00, 00, 00, // 255 checkpoints
				00, 00, 00, 00, 00, 00, 00, 00, // span size 0
				// No checkpoint data. We should not try to read 255 checkpoints from this buffer.
			},
			expectError: true,
		},
		{
			name: "zinfoBytes with zero checkpoints should succeed",
			zinfoBytes: []byte{
				00, 00, 00, 00, // 0 checkpoints
				00, 00, 00, 00, 00, 00, 00, 00, // span size 0
			},
			expectError: false,
		},
		{
			name: "zinfoBytes v1 with zero checkpoints should succeed",
			zinfoBytes: []byte{
				01, 00, 00, 00, // 1 checkpoint
				00, 00, 00, 00, 00, 00, 00, 00, // span size 0
			},
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := newGzipZinfo(tc.zinfoBytes)
			if tc.expectError != (err != nil) {
				t.Fatalf("expect error: %t, actual error: %v", tc.expectError, err)
			}
		})
	}
}

func TestExtractDataFromBuffer(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name               string
		gzipZinfo          GzipZinfo
		compressedBuf      []byte
		uncompressedSize   Offset
		uncompressedOffset Offset
		spanID             SpanID
		expectError        bool
	}{
		{
			name:          "nil buffer should return error",
			gzipZinfo:     GzipZinfo{},
			compressedBuf: nil,
			expectError:   true,
		},
		{
			name:          "empty buffer should return error",
			gzipZinfo:     GzipZinfo{},
			compressedBuf: []byte{},
			expectError:   true,
		},
		{
			name:             "negative uncompressedSize should return error",
			gzipZinfo:        GzipZinfo{},
			compressedBuf:    []byte("foobar"),
			uncompressedSize: -1,
			expectError:      true,
		},
		{
			name:             "zero uncompressedSize should return empty byte slice",
			gzipZinfo:        GzipZinfo{},
			compressedBuf:    []byte("foobar"),
			uncompressedSize: 0,
			expectError:      false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			data, err := tc.gzipZinfo.ExtractDataFromBuffer(tc.compressedBuf, tc.uncompressedSize, tc.uncompressedOffset, tc.spanID)
			if tc.expectError != (err != nil) {
				t.Fatalf("expect error: %t, actual error: %v", tc.expectError, err)
			}
			if err == nil && len(data) != int(tc.uncompressedSize) {
				t.Fatalf("wrong uncompressed size. expect: %d, actual: %d ", len(data), tc.uncompressedSize)
			}
		})
	}
}

func TestExtractDataFromFile(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name               string
		gzipZinfo          GzipZinfo
		filename           string
		uncompressedSize   Offset
		uncompressedOffset Offset
		expectError        bool
	}{
		{
			name:             "negative uncompressedSize should return error",
			gzipZinfo:        GzipZinfo{},
			filename:         "",
			uncompressedSize: -1,
			expectError:      true,
		},
		{
			name:             "zero uncompressedSize should return empty byte slice",
			gzipZinfo:        GzipZinfo{},
			filename:         "",
			uncompressedSize: 0,
			expectError:      false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			data, err := tc.gzipZinfo.ExtractDataFromFile(tc.filename, tc.uncompressedSize, tc.uncompressedOffset)
			if tc.expectError != (err != nil) {
				t.Fatalf("expect error: %t, actual error: %v", tc.expectError, err)
			}
			if err == nil && len(data) != int(tc.uncompressedSize) {
				t.Fatalf("wrong uncompressed size. expect: %d, actual: %d ", len(data), tc.uncompressedSize)
			}
		})
	}
}
