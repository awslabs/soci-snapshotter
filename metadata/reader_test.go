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

package metadata

import (
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/awslabs/soci-snapshotter/ztoc"
	bolt "go.etcd.io/bbolt"
)

func TestMetadataReader(t *testing.T) {
	testReader(t, newTestableReader)
}

func newTestableReader(sr *io.SectionReader, toc ztoc.TOC, opts ...Option) (testableReader, error) {
	f, err := os.CreateTemp("", "readertestdb")
	if err != nil {
		return nil, err
	}
	defer os.Remove(f.Name())
	db, err := bolt.Open(f.Name(), 0600, nil)
	if err != nil {
		return nil, err
	}
	r, err := NewReader(db, sr, toc, opts...)
	if err != nil {
		return nil, err
	}
	return &testableReadCloser{
		testableReader: r.(*reader),
		closeFn: func() error {
			db.Close()
			return os.Remove(f.Name())
		},
	}, nil
}

type testableReadCloser struct {
	testableReader
	closeFn func() error
}

func (r *testableReadCloser) Close() error {
	r.closeFn()
	return r.testableReader.Close()
}

func TestPartition(t *testing.T) {
	testCases := []struct {
		name      string
		chunkSize int
		input     []int
		want      [][]int
	}{
		{
			name:      "equality when length of slice mod chunkSize is zero",
			chunkSize: 3,
			input:     []int{1, 2, 3, 4, 5, 6, 7, 8, 9},
			want:      [][]int{{1, 2, 3}, {4, 5, 6}, {7, 8, 9}},
		},
		{
			name:      "equality when length of slice mod chunkSize is non-zero",
			chunkSize: 4,
			input:     []int{1, 2, 3, 4, 5, 6, 7, 8, 9},
			want:      [][]int{{1, 2, 3, 4}, {5, 6, 7, 8}, {9}},
		},
		{
			name:      "zero slice returns zero slice",
			chunkSize: 1,
			input:     []int{},
			want:      [][]int{},
		},
		{
			name:      "chunkSize greater than length of slice returns original slice",
			chunkSize: 10,
			input:     []int{1, 2, 3, 4, 5, 6, 7, 8, 9},
			want:      [][]int{{1, 2, 3, 4, 5, 6, 7, 8, 9}},
		},
		{
			name:      "zero chunkSize returns zero slice",
			chunkSize: 0,
			input:     []int{1, 2, 3, 4, 5, 6, 7, 8, 9},
			want:      [][]int{},
		},
		{
			name:      "negative chunkSize returns zero slice",
			chunkSize: -1,
			input:     []int{1, 2, 3, 4, 5, 6, 7, 8, 9},
			want:      [][]int{},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			checkEquivalence := func(have, want [][]int) error {
				if len(have) != len(want) {
					return fmt.Errorf("mismatched chunk count; want: %v, have: %v", len(want), len(have))
				}
				for i := 0; i < len(have); i++ {
					if len(have[i]) != len(want[i]) {
						return fmt.Errorf("mismatched chunk length; want: %v, have: %v", len(want[i]), len(have[i]))
					}
					for j := 0; j < len(have[i]); j++ {
						if have[i][j] != want[i][j] {
							return fmt.Errorf("mismatched values; want: %v, have: %v", want[i][j], have[i][j])
						}
					}
				}
				return nil
			}
			if err := checkEquivalence(partition(tc.input, tc.chunkSize), tc.want); err != nil {
				t.Fatal(err)
			}
		})

	}
}
