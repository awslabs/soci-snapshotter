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

package ociutil

import (
	"slices"
	"testing"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

func assertPlatformEqual(t *testing.T, expected ocispec.Platform, actual ocispec.Platform) {
	if expected.OS != actual.OS {
		t.Fatalf("OS did not match. Expected %v, Got %v", expected.OS, actual.OS)
	}
	if expected.OSVersion != actual.OSVersion {
		t.Fatalf("OSVersion did not match. Expected %v, Got %v", expected.OSVersion, actual.OSVersion)
	}
	if !slices.Equal(expected.OSFeatures, actual.OSFeatures) {
		t.Fatalf("OSFeatures did not match. Expected %v, Got %v", expected.OSFeatures, actual.OSFeatures)
	}
	if expected.Architecture != actual.Architecture {
		t.Fatalf("Architecture did not match. Expected %v, Got %v", expected.Architecture, actual.Architecture)
	}
	if expected.Variant != actual.Variant {
		t.Fatalf("Variant did not match. Expected %v, Got %v", expected.Variant, actual.Variant)
	}
}

func TestDedupePlatforms(t *testing.T) {
	LinuxX86 := ocispec.Platform{
		OS:           "linux",
		Architecture: "amd64",
	}
	Linuxi386 := ocispec.Platform{
		OS:           "linux",
		Architecture: "386",
	}
	LinuxX86Denormalized := ocispec.Platform{
		OS:           "linux",
		Architecture: "x86_64",
	}

	tests := []struct {
		name      string
		platforms []ocispec.Platform
		expected  []ocispec.Platform
	}{
		{
			name: "no duplicates results in no change",
			platforms: []ocispec.Platform{
				LinuxX86,
				Linuxi386,
			},
			expected: []ocispec.Platform{
				LinuxX86,
				Linuxi386,
			},
		},
		{
			name: "exact duplicates are removed",
			platforms: []ocispec.Platform{
				LinuxX86,
				LinuxX86,
			},
			expected: []ocispec.Platform{
				LinuxX86,
			},
		},
		{
			name: "denormalized duplciates are removed",
			platforms: []ocispec.Platform{
				LinuxX86,
				LinuxX86Denormalized,
			},
			expected: []ocispec.Platform{
				LinuxX86,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual := DedupePlatforms(test.platforms)
			if len(actual) != len(test.expected) {
				t.Fatalf("unexpected number of dedupe platforms, expected %d, actual %d. Result: %v", len(test.expected), len(actual), actual)
			}
			for i := range actual {
				assertPlatformEqual(t, test.expected[i], actual[i])
			}
		})
	}

}
