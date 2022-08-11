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
	"encoding/gob"
	"fmt"
	"io"
	"os"

	"github.com/klauspost/compress/zstd"
)

func GetZtocFromFile(filename string) (*Ztoc, error) {
	reader, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return GetZtoc(reader)
}

// GetZtoc reads and returns the Ztoc
func GetZtoc(reader io.Reader) (*Ztoc, error) {
	zs, err := zstd.NewReader(reader)
	if err != nil {
		return nil, fmt.Errorf("cannot create gzip reader: %v", err)
	}
	defer zs.Close()

	decoder := gob.NewDecoder(zs)
	ztoc := new(Ztoc)

	if err := decoder.Decode(ztoc); err != nil {
		return nil, fmt.Errorf("cannot decode ztoc: %w", err)
	}
	return ztoc, nil
}

// Get file mode from ztoc
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
