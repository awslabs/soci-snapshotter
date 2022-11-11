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
	"archive/tar"
	"os"
)

func GetZtocFromFile(filename string) (*Ztoc, error) {
	reader, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	zm := ZtocMarshaler{}
	return zm.Unmarshal(reader)
}

// Get file mode for file metadata
func GetFileMode(src *FileMetadata) (m os.FileMode) {
	// FileMetadata.Mode is tar.Header.Mode so we can understand the these bits using `tar` pkg.
	m = (&tar.Header{Mode: src.Mode}).FileInfo().Mode() &
		(os.ModePerm | os.ModeSetuid | os.ModeSetgid | os.ModeSticky)
	switch src.Type {
	case "dir":
		m |= os.ModeDir
	case "symlink":
		m |= os.ModeSymlink
	case "char":
		m |= os.ModeDevice | os.ModeCharDevice
	case "block":
		m |= os.ModeDevice
	case "fifo":
		m |= os.ModeNamedPipe
	}
	return m
}
