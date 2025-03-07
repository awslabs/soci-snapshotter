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

package os

import (
	"errors"
	"fmt"
	"os"
	"testing"
)

func TestSanitizeExecutablePath(t *testing.T) {
	testCases := []struct {
		name         string
		path         func(string) string
		expectedPath func(string) string
		expectedErr  error
	}{
		{
			name: "Valid Path",
			path: func(root string) string {
				filepath := fmt.Sprintf("%s/gzip", root)
				f, err := os.OpenFile(filepath, os.O_CREATE|os.O_EXCL, 0o755)
				if err != nil {
					t.Fatal(err)
				}
				f.Close()
				return filepath
			},
			expectedPath: func(root string) string {
				return fmt.Sprintf("%s/gzip", root)
			},
		},
		{
			name: "SymlinkIsResolved",
			path: func(root string) string {
				targetPath := fmt.Sprintf("%s/gzip", root)
				f, err := os.OpenFile(targetPath, os.O_CREATE|os.O_EXCL, 0o755)
				if err != nil {
					t.Fatal(err)
				}
				f.Close()

				symlinkPath := fmt.Sprintf("%s/symlink-to-executable", root)
				if err := os.Symlink(targetPath, symlinkPath); err != nil {
					t.Fatal(err)
				}

				return symlinkPath
			},
			expectedPath: func(root string) string {
				return fmt.Sprintf("%s/gzip", root)
			},
		},
		{
			name: "PathContainsInvalidCharacters",
			path: func(root string) string {
				return fmt.Sprintf("%s/\\gzip", root)
			},
			expectedPath: func(_ string) string {
				return ""
			},
			expectedErr: errFilePathContainsInvalidCharacters,
		},
		{
			name: "PathDoesNotExist",
			path: func(root string) string {
				return fmt.Sprintf("%s/does-not-exist", root)
			},
			expectedPath: func(_ string) string {
				return ""
			},
			expectedErr: os.ErrNotExist,
		},
		{
			name: "SymlinkToPathDoesNotExist",
			path: func(root string) string {
				targetPath := fmt.Sprintf("%s/gzip", root)

				symlinkPath := fmt.Sprintf("%s/symlink-to-executable", root)
				if err := os.Symlink(targetPath, symlinkPath); err != nil {
					t.Fatal(err)
				}

				return symlinkPath
			},
			expectedPath: func(_ string) string {
				return ""
			},
			expectedErr: os.ErrNotExist,
		},
		{
			name: "PathIsADirectory",
			path: func(root string) string {
				return root
			},
			expectedPath: func(_ string) string {
				return ""
			},
			expectedErr: errFilePathIsADirectory,
		},
		{
			name: "PathIsNotExecutable",
			path: func(root string) string {
				filepath := fmt.Sprintf("%s/gzip", root)
				f, err := os.OpenFile(filepath, os.O_CREATE, 0o644)
				if err != nil {
					t.Fatal(err)
				}
				defer f.Close()
				return filepath
			},
			expectedPath: func(_ string) string {
				return ""
			},
			expectedErr: errFilePathIsNotExecutable,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			actualPath, actualErr := SanitizeExecutablePath(testCase.path(tmpDir))
			if !errors.Is(actualErr, testCase.expectedErr) {
				t.Errorf("Expected error %v, got %v", testCase.expectedErr, actualErr)
			}

			expectedPath := testCase.expectedPath(tmpDir)
			if actualPath != expectedPath {
				t.Errorf("Expected path %q, got %q", expectedPath, actualPath)
			}
		})
	}
}
