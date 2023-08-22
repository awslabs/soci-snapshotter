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

package ioutils

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

var bs []byte = []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}

func copy(b []byte, n int) error {
	if len(b) < n {
		return errors.New("buffer is too short")
	}
	for i := 0; i < n; i++ {
		b[i] = bs[i]
	}
	return nil
}

type testReader struct {
	n   int
	err error
}

func (s *testReader) Read(b []byte) (int, error) {
	copy(b, s.n)
	return s.n, s.err
}

func TestPositionTrackingReader(t *testing.T) {
	tests := []struct {
		name        string
		r           io.Reader
		expectedPos int64
		expectedErr error
	}{
		{
			name:        "full read tracks position correctly",
			r:           bytes.NewReader(bs),
			expectedPos: 10,
			expectedErr: nil,
		},
		{
			name:        "short read tracks position correctly",
			r:           &testReader{5, nil},
			expectedPos: 5,
			expectedErr: nil,
		},
		{
			name:        "err read tracks position correctly",
			r:           &testReader{5, io.ErrUnexpectedEOF},
			expectedPos: 5,
			expectedErr: io.ErrUnexpectedEOF,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			pt := NewPositionTrackerReader(tc.r)
			b := make([]byte, 10)
			_, err := pt.Read(b)
			if pt.CurrentPos() != tc.expectedPos {
				t.Fatalf("incorrect position. Expected %d, Actual %d", tc.expectedPos, pt.CurrentPos())
			}
			if tc.expectedErr != nil && !errors.Is(err, tc.expectedErr) {
				t.Fatalf("incorrect error. Expected %v, Actual %v", tc.expectedErr, err)
			}
		})
	}

}
