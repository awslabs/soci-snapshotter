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

package layer

import (
	"testing"

	"github.com/awslabs/soci-snapshotter/metadata"
	"github.com/hanwen/go-fuse/v2/fuse"
)

func TestEntryToAttr(t *testing.T) {
	testcases := []struct {
		name     string
		attr     metadata.Attr
		expected fuse.Attr
	}{
		{
			name: "fuse.Attr.Blocks is reported as # of 512-byte blocks",
			attr: metadata.Attr{
				Size: 1774757,
			},
			expected: fuse.Attr{
				Size: 1774757,
				// Blocks should be the number of 512-byte blocks aligned to blockSize.
				// Specifically we want to validate that it's not ceiling(Size/blockSize)
				Blocks:  3472,
				Blksize: blockSize,
				Mode:    fileModeToSystemMode(0),
				Nlink:   1,
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			var actual fuse.Attr
			var n node
			n.entryToAttr(0, tc.attr, &actual)
			tc.expected.Mtime = actual.Mtime
			if actual != tc.expected {
				t.Fatalf("unexpected fuse attr. actual %v expected %v", actual, tc.expected)
			}
		})
	}
}
