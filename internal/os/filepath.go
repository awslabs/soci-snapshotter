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
	"path/filepath"
	"strings"
)

var (
	errFilePathContainsInvalidCharacters = errors.New("path contains invalid characters")
	errFilePathIsADirectory              = errors.New("path is a directory, not an executable file")
	errFilePathIsNotExecutable           = errors.New("file is not executable")
)

// SanitizeExecutablePath cleans and validates a file path to prevent command injection
func SanitizeExecutablePath(path string) (string, error) {
	// Check if the path contains any suspicious characters
	if strings.ContainsAny(path, "#%{}\\|;&$<>") {
		return "", errFilePathContainsInvalidCharacters
	}

	// Clean the path to resolve any ".." or "." components
	cleanPath := filepath.Clean(path)
	resolvedPath, err := filepath.EvalSymlinks(cleanPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve symlinks: %w", err)
	}

	// Get absolute path to ensure we're working with a full path
	absPath, err := filepath.Abs(resolvedPath)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %w", err)
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return "", fmt.Errorf("path validation failed: %w", err)
	}

	if info.IsDir() {
		return "", errFilePathIsADirectory
	}

	// Check if the file is executable
	if info.Mode().Perm()&0111 == 0 {
		return "", errFilePathIsNotExecutable
	}

	return resolvedPath, nil
}
