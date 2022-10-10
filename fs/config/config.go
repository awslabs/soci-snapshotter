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

/*
   Copyright The containerd Authors.

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

/*
   Copyright 2019 The Go Authors. All rights reserved.
   Use of this source code is governed by a BSD-style
   license that can be found in the NOTICE.md file.
*/

package config

const (
	// TargetImageRefLabel is a snapshot label key that contains the image ref
	TargetImageRefLabel = "com.amazon.soci/remote/image.reference"

	// TargetSociIndexDigestLabel is a snapshot label key that contains the soci index digest
	TargetSociIndexDigestLabel = "com.amazon.soci/remote/soci.index.digest"

	// Default path to OCI-compliant CAS
	SociContentStorePath = "/var/lib/soci-snapshotter-grpc/content/"

	// Default path to snapshotter root dir
	SociSnapshotterRootPath = "/var/lib/soci-snapshotter-grpc/"
)

type Config struct {
	HTTPCacheType                    string `toml:"http_cache_type"`
	FSCacheType                      string `toml:"filesystem_cache_type"`
	ResolveResultEntry               int    `toml:"resolve_result_entry"`
	NoBackgroundFetch                bool   `toml:"no_background_fetch"`
	Debug                            bool   `toml:"debug"`
	AllowNoVerification              bool   `toml:"allow_no_verification"`
	DisableVerification              bool   `toml:"disable_verification"`
	MaxConcurrency                   int64  `toml:"max_concurrency"`
	NoPrometheus                     bool   `toml:"no_prometheus"`
	PrioritizedTaskSilencePeriodMSec int    `toml:"prioritized_task_silence_period_msec"`

	// BlobConfig is config for layer blob management.
	BlobConfig `toml:"blob"`

	// DirectoryCacheConfig is config for directory-based cache.
	DirectoryCacheConfig `toml:"directory_cache"`

	FuseConfig `toml:"fuse"`
}

type BlobConfig struct {
	ValidInterval int64 `toml:"valid_interval"`
	CheckAlways   bool  `toml:"check_always"`
	// ChunkSize is the granularity at which background fetch and on-demand reads
	// are fetched from the remote registry.
	ChunkSize            int64 `toml:"chunk_size"`
	FetchTimeoutSec      int64 `toml:"fetching_timeout_sec"`
	ForceSingleRangeMode bool  `toml:"force_single_range_mode"`
	MaxRetries           int   `toml:"max_retries"`
	MinWaitMSec          int   `toml:"min_wait_msec"`
	MaxWaitMSec          int   `toml:"max_wait_msec"`
}

type DirectoryCacheConfig struct {
	MaxLRUCacheEntry int  `toml:"max_lru_cache_entry"`
	MaxCacheFds      int  `toml:"max_cache_fds"`
	SyncAdd          bool `toml:"sync_add"`
	Direct           bool `toml:"direct" default:"true"`
}

type FuseConfig struct {
	// AttrTimeout defines overall timeout attribute for a file system in seconds.
	AttrTimeout int64 `toml:"attr_timeout"`

	// EntryTimeout defines TTL for directory, name lookup in seconds.
	EntryTimeout int64 `toml:"entry_timeout"`
}
