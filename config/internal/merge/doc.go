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

// Package merge implements a simple mechanism for merging configuration values
// from a map into a struct. It uses reflection to match struct fields with map
// keys based on struct tags, allowing for flexible configuration handling.
//
// The primary function, Merge, takes a destination struct and a source map,
// then populates the struct fields with corresponding values from the map
// according to the struct's field tags.
package merge // import "github.com/awslabs/soci-snapshotter/config/internal/merge"
