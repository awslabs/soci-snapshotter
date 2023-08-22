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

import "github.com/awslabs/soci-snapshotter/ztoc/compression"

// TarBlockSize is the size of a tar block
const TarBlockSize = 512

// AlignToTarBlock aligns an offset to the next larger multiple of TarBlockSize
func AlignToTarBlock(o compression.Offset) compression.Offset {
	offset := o % TarBlockSize
	if offset > 0 {
		o += TarBlockSize - offset
	}
	return o
}
