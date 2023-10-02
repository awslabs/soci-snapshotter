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
	"strings"

	"github.com/awslabs/soci-snapshotter/ztoc/compression"
)

const (
	// TarBlockSize is the size of a tar block
	TarBlockSize = 512

	schilyXattrPrefix string = "SCHILY.xattr."
)

// AlignToTarBlock aligns an offset to the next larger multiple of TarBlockSize
func AlignToTarBlock(o compression.Offset) compression.Offset {
	offset := o % TarBlockSize
	if offset > 0 {
		o += TarBlockSize - offset
	}
	return o
}

// Xattrs converts a set of tar PAXRecords to a set of Xattrs.
// Specifically, it looks for PAXRecords where the key is prefixed
// by `SCHILY.xattr.` - the prefix for Xattrs used by go and GNU tar.
// Those keys are kept with the prefix stripped. Other keys are dropped.
func Xattrs(paxHeaders map[string]string) map[string]string {
	if len(paxHeaders) == 0 {
		return nil
	}
	m := make(map[string]string)
	for k, v := range paxHeaders {
		if strings.HasPrefix(k, schilyXattrPrefix) {
			m[k[len(schilyXattrPrefix):]] = v
		}
	}
	return m
}
