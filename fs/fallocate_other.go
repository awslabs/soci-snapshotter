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

//go:build !linux

package fs

import (
	"errors"
	"os"
)

// preallocateFile is a no-op on non-Linux platforms.
// It returns an error so the caller falls back to Truncate.
func preallocateFile(file *os.File, size int64) error {
	return errors.New("fallocate not supported on this platform")
}
