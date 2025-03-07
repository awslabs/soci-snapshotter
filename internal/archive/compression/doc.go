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

// Package compression defines the mechanism for configuring decompress streams
// used to unpack image layer tarballs.
//
// Copied and modified from https://github.com/containerd/containerd/blob/bcc810d6b9066471b0b6fa75f557a15a1cbf31bb/archive/compression
package compression // import "github.com/awslabs/soci-snapshotter/internal/archive/compression"
