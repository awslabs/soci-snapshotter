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

/*
   Copyright The containerd Authors.

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
	"testing"

	"github.com/awslabs/soci-snapshotter/metadata"
	"github.com/awslabs/soci-snapshotter/soci"
	bolt "go.etcd.io/bbolt"
)

func TestReader(t *testing.T) {
	metadata.TestReader(t, newTestableReader)
}

func newTestableReader(sr *io.SectionReader, ztoc *soci.Ztoc, opts ...metadata.Option) (metadata.TestableReader, error) {
	f, err := os.CreateTemp("", "readertestdb")
	if err != nil {
		return nil, err
	}
	defer os.Remove(f.Name())
	db, err := bolt.Open(f.Name(), 0600, nil)
	if err != nil {
		return nil, err
	}
	r, err := NewReader(db, sr, ztoc, opts...)
	if err != nil {
		return nil, err
	}
	return &testableReadCloser{
		TestableReader: r.(*reader),
		closeFn: func() error {
			db.Close()
			return os.Remove(f.Name())
		},
	}, nil
}

type testableReadCloser struct {
	metadata.TestableReader
	closeFn func() error
}

func (r *testableReadCloser) Close() error {
	r.closeFn()
	return r.TestableReader.Close()
}
