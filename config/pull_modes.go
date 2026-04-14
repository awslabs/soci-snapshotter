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

package config

// PullModes contain config related to the ways in
// in which the SOCI snapshotter can pull images
type PullModes struct {
	SOCIv1   V1       `toml:"soci_v1"`
	SOCIv2   V2       `toml:"soci_v2"`
	Parallel Parallel `toml:"parallel_pull_unpack"`
}

// V1 contains config for SOCI v1 which uses the
// OCI referrers API to automatically discover SOCI
// indexes that reference an image
type V1 struct {
	Enable bool `toml:"enable"`
}

// V2 contains config for SOCI v2 which uses annotations
// on the container's image manifest to discover SOCI indexes
// without an out-of-band referrers API call
type V2 struct {
	Enable bool `toml:"enable"`
}

// Parallel contains config for parallel pull and unpacks
// Parallel mode does not implment lazy loading strategy but
// aims to speed up the process via concurrent operations.
type Parallel struct {
	ParallelConfig
	Enable bool `toml:"enable"`

	// ParallelPullAsFallback enables parallel-pull as an automatic fallback
	// when lazy-load is the primary mode but no SOCI index is found for an image.
	// When true (and Enable is false), the snapshotter will first attempt lazy-load;
	// if no SOCI index exists, it falls back to parallel-pull instead of deferring
	// to the container runtime's slower sequential pull.
	// If Enable is true, this option is a no-op (parallel-pull is already the primary mode).
	//
	// EXPERIMENTAL: This requires the containerd content store for both lazy-load
	// and parallel-pull (unless discard_unpacked_layers = true).
	// Lazy-load with the containerd content store may have
	// garbage collection edge cases. See https://github.com/awslabs/soci-snapshotter/issues/1843
	ExperimentalParallelPullAsFallback bool `toml:"experimental_parallel_pull_as_fallback"`
}

func defaultPullModes(cfg *Config) error {
	cfg.PullModes = DefaultPullModes()
	return nil
}

// DefaultPullModes returns a PullModes struct
// with the SOCI defaults set.
func DefaultPullModes() PullModes {
	return PullModes{
		SOCIv1: V1{
			Enable: DefaultSOCIV1Enable,
		},
		SOCIv2: V2{
			Enable: DefaultSOCIV2Enable,
		},
		Parallel: Parallel{
			Enable:         DefaultParallelPullUnpackEnable,
			ParallelConfig: defaultParallelConfig(),
		},
	}
}
