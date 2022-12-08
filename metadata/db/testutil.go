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

package db

import (
	"io"
	"os"

	"github.com/awslabs/soci-snapshotter/metadata"
	"github.com/awslabs/soci-snapshotter/ztoc"
	bolt "go.etcd.io/bbolt"
)

func NewDbMetadataStore(sr *io.SectionReader, ztoc *ztoc.Ztoc, opts ...metadata.Option) (metadata.Reader, error) {
	f, err := os.CreateTemp("", "readertestdb")
	if err != nil {
		return nil, err
	}
	defer f.Close()
	db, err := bolt.Open(f.Name(), 0600, nil)
	if err != nil {
		return nil, err
	}
	r, err := NewReader(db, sr, ztoc, opts...)
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
	metadata.Reader
	closeFn func() error
}

func (r *readCloser) Close() error {
	r.closeFn()
	return r.Reader.Close()
}
