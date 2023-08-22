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
	"io"
)

// PositionTrackerReader is an `io.Reader` that tracks the current read position
// in an underlying `io.Reader`
type PositionTrackerReader struct {
	r   io.Reader
	pos int64
}

// NewPositionTrackerReader creates a new PositionTrackerReader with the initial position
// set to 0.
func NewPositionTrackerReader(r io.Reader) *PositionTrackerReader {
	return &PositionTrackerReader{r, 0}
}

// Read reads from the PositionTrackerReader into the provided byte slice.
// The number position of the PositionTrackerReader is updated based on the
// number of bytes read
func (p *PositionTrackerReader) Read(b []byte) (int, error) {
	n, err := p.r.Read(b)
	p.pos += int64(n)
	return n, err
}

// CurrentPos is the current position of the PositionTrackerReader
func (p *PositionTrackerReader) CurrentPos() int64 {
	return p.pos
}
