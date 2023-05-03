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

package compression

// Offset will hold any file size and offset values
type Offset int64

// SpanID will hold any span related values (SpanID, MaxSpanID, etc)
type SpanID int32

// Compression algorithms used by an image layer. They should be kept consistent
// with the return of `DiffCompression` from containerd.
// https://github.com/containerd/containerd/blob/v1.7.0-beta.3/images/mediatypes.go#L66
const (
	Gzip         = "gzip"
	Zstd         = "zstd"
	Uncompressed = "uncompressed"
	Unknown      = "unknown"
)
