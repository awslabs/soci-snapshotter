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

package metadata

import (
	"io"
	"os"

	"github.com/awslabs/soci-snapshotter/ztoc"
	"go.etcd.io/bbolt"
)

// NewTempDbStore returns a Reader by creating a temp bolt db, which will
// be removed when `Reader.Close()` is called.
func NewTempDbStore(sr *io.SectionReader, toc ztoc.TOC, opts ...Option) (Reader, error) {
	f, err := os.CreateTemp("", "readertestdb")
	if err != nil {
		return nil, err
	}
	defer f.Close()
	db, err := bbolt.Open(f.Name(), 0600, nil)
	if err != nil {
		return nil, err
	}
	r, err := NewReader(db, sr, toc, opts...)
	if err != nil {
		return nil, err
	}
	return &readCloser{
		Reader: r,
		closeFn: func() error {
			db.Close()
			return os.Remove(f.Name())
		},
	}, nil
}

type readCloser struct {
	Reader
	closeFn func() error
}

func (r *readCloser) Close() error {
	r.closeFn()
	return r.Reader.Close()
}
