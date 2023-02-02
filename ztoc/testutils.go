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
	"fmt"
	"io"
	"os"

	"github.com/awslabs/soci-snapshotter/util/testutil"
)

// buildZtocReader creates the tar gz file for tar entries. It returns ztoc and io.SectionReader of the file.
func BuildZtocReader(ents []testutil.TarEntry, compressionLevel int, spanSize int64, opts ...testutil.BuildTarOption) (*Ztoc, *io.SectionReader, error) {
	tarReader := testutil.BuildTarGz(ents, compressionLevel, opts...)

	tarFile, err := os.CreateTemp("", "tmp.*")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create temp file: %v", err)
	}
	defer os.Remove(tarFile.Name())
	tarBuf := new(bytes.Buffer)
	w := io.MultiWriter(tarFile, tarBuf)
	_, err = io.Copy(w, tarReader)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to write tar file: %v", err)
	}
	tarData := tarBuf.Bytes()
	sr := io.NewSectionReader(bytes.NewReader(tarData), 0, int64(len(tarData)))
	ztoc, err := BuildZtoc(tarFile.Name(), spanSize, "test")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to build sample ztoc: %v", err)
	}
	return ztoc, sr, nil
}
