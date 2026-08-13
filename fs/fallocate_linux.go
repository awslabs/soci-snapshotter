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

//go:build linux

package fs

import (
	"os"

	"golang.org/x/sys/unix"
)

// preallocateFile pre-allocates disk space for the file using fallocate.
// This avoids the overhead of block allocation during random WriteAt calls
// that occur with sparse files created by Truncate.
func preallocateFile(file *os.File, size int64) error {
	return unix.Fallocate(int(file.Fd()), 0, 0, size)
}
