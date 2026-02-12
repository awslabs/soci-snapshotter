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

package artifacts

type FilterFn func(*Entry) bool

// WithAnyFilters combines the given filters with OR logic
func WithAnyFilters(filters ...FilterFn) FilterFn {
	if len(filters) == 0 {
		return func(*Entry) bool {
			return true
		}
	}
	return func(e *Entry) bool {
		for _, f := range filters {
			if f(e) {
				return true
			}
		}
		return false
	}
}

// WithAllFilters combines the given filters with AND logic
func WithAllFilters(filters ...FilterFn) FilterFn {
	return func(e *Entry) bool {
		for _, f := range filters {
			if !f(e) {
				return false
			}
		}
		return true
	}
}

// WithDigest returns a filter that matches entries with the specified digest
func WithDigest(digest string) FilterFn {
	return func(e *Entry) bool {
		return e.Digest == digest
	}
}

// WithImageDigest returns a filter that matches entries with the specified image digest
func WithImageDigest(digest string) FilterFn {
	return func(e *Entry) bool {
		return e.ImageDigest == digest
	}
}

// WithOriginalDigest returns a filter that matches entries with the specified original digest
func WithOriginalDigest(digest string) FilterFn {
	return func(e *Entry) bool {
		return e.OriginalDigest == digest
	}
}

// WithEntryType returns a filter that matches entries with the specified entry type
func WithEntryType(t EntryType) FilterFn {
	return func(e *Entry) bool {
		return e.Type == t
	}
}

// WithMediaType returns a filter that matches entries with the specified media type
func WithMediaType(mediaType string) FilterFn {
	return func(e *Entry) bool {
		return e.MediaType == mediaType
	}
}

// With Platform returns a filter that matches entries with the specified platform
func WithPlatform(p string) FilterFn {
	return func(e *Entry) bool {
		return e.Platform == p
	}
}
