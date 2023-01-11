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
	HTTPCacheType       string `toml:"http_cache_type"`
	FSCacheType         string `toml:"filesystem_cache_type"`
	ResolveResultEntry  int    `toml:"resolve_result_entry"`
	Debug               bool   `toml:"debug"`
	AllowNoVerification bool   `toml:"allow_no_verification"`
	DisableVerification bool   `toml:"disable_verification"`
	MaxConcurrency      int64  `toml:"max_concurrency"`
	NoPrometheus        bool   `toml:"no_prometheus"`
	MountTimeoutSec     int    `toml:"mount_timeout_sec"`

	// BlobConfig is config for layer blob management.
	BlobConfig `toml:"blob"`

	// DirectoryCacheConfig is config for directory-based cache.
	DirectoryCacheConfig `toml:"directory_cache"`

	FuseConfig `toml:"fuse"`

	BackgroundFetchConfig `toml:"background_fetch"`
}

type BlobConfig struct {
	ValidInterval        int64 `toml:"valid_interval"`
	CheckAlways          bool  `toml:"check_always"`
	FetchTimeoutSec      int64 `toml:"fetching_timeout_sec"`
	ForceSingleRangeMode bool  `toml:"force_single_range_mode"`
	MaxRetries           int   `toml:"max_retries"`
	MinWaitMSec          int   `toml:"min_wait_msec"`
	MaxWaitMSec          int   `toml:"max_wait_msec"`

	// MaxSpanVerificationRetries defines the number of times fetch will be invoked in case of
	// span verification failure.
	MaxSpanVerificationRetries int `toml:"max_span_verification_retries"`
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

	// NegativeTimeout defines the overall entry timeout for failed lookups.
	NegativeTimeout int64 `toml:"negative_timeout"`

	// LogFuseOperations enables logging of operations on FUSE FS. This is to be used
	// for debugging purposes only.
	LogFuseOperations bool `toml:"log_fuse_operations"`
}

type BackgroundFetchConfig struct {
	Disable bool `toml:"disable"`

	// SilencePeriodMSec defines the time (in ms) the background fetcher
	// will be paused for when a new image is mounted.
	SilencePeriodMSec int64 `toml:"silence_period_msec"`

	// FetchPeriodMSec specifies how often a background fetch will occur.
	// The background fetcher will fetch one span every FetchPeriodMSec.
	FetchPeriodMSec int64 `toml:"fetch_period_msec"`

	// MaxQueueSize specifies the maximum size of the work queue
	// i.e., the maximum number of span managers that can be queued
	// in the background fetcher.
	MaxQueueSize int `toml:"max_queue_size"`
}
