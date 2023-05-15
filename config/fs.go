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

import "time"

type FSConfig struct {
	HTTPCacheType                  string `toml:"http_cache_type"`
	FSCacheType                    string `toml:"filesystem_cache_type"`
	ResolveResultEntry             int    `toml:"resolve_result_entry"`
	Debug                          bool   `toml:"debug"`
	AllowNoVerification            bool   `toml:"allow_no_verification"`
	DisableVerification            bool   `toml:"disable_verification"`
	MaxConcurrency                 int64  `toml:"max_concurrency"`
	NoPrometheus                   bool   `toml:"no_prometheus"`
	MountTimeoutSec                int64  `toml:"mount_timeout_sec"`
	FuseMetricsEmitWaitDurationSec int64  `toml:"fuse_metrics_emit_wait_duration_sec"`

	BlobConfig `toml:"blob"`

	DirectoryCacheConfig `toml:"directory_cache"`

	FuseConfig `toml:"fuse"`

	BackgroundFetchConfig `toml:"background_fetch"`
}

// BlobConfig is config for layer blob management.
type BlobConfig struct {
	ValidInterval        int64 `toml:"valid_interval"`
	CheckAlways          bool  `toml:"check_always"`
	FetchTimeoutSec      int64 `toml:"fetching_timeout_sec"`
	ForceSingleRangeMode bool  `toml:"force_single_range_mode"`
	MaxRetries           int   `toml:"max_retries"`
	MinWaitMsec          int64 `toml:"min_wait_msec"`
	MaxWaitMsec          int64 `toml:"max_wait_msec"`

	// MaxSpanVerificationRetries defines the number of additional times fetch
	// will be invoked in case of span verification failure.
	MaxSpanVerificationRetries int `toml:"max_span_verification_retries"`
}

// DirectoryCacheConfig is config for directory-based cache.
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
	// for debugging purposes only. This option may emit sensitive information,
	// e.g. filenames and paths within an image
	LogFuseOperations bool `toml:"log_fuse_operations"`
}

type BackgroundFetchConfig struct {
	Disable bool `toml:"disable"`

	// SilencePeriodMsec defines the time (in ms) the background fetcher
	// will be paused for when a new image is mounted.
	SilencePeriodMsec int64 `toml:"silence_period_msec"`

	// FetchPeriodMsec specifies how often a background fetch will occur.
	// The background fetcher will fetch one span every FetchPeriodMsec.
	FetchPeriodMsec int64 `toml:"fetch_period_msec"`

	// MaxQueueSize specifies the maximum size of the work queue
	// i.e., the maximum number of span managers that can be queued
	// in the background fetcher.
	MaxQueueSize int `toml:"max_queue_size"`

	// EmitMetricPeriodSec is the amount of interval (in second) at which the background
	// fetcher emits metrics
	EmitMetricPeriodSec int64 `toml:"emit_metric_period_sec"`
}

// RetryConfig represents the settings for retries in a retryable http client.
type RetryConfig struct {
	// MaxRetries is the maximum number of retries before giving up on a retryable request.
	// This does not include the initial request so the total number of attempts will be MaxRetries + 1.
	MaxRetries int
	// MinWait is the minimum wait time between attempts. The actual wait time is governed by the BackoffStrategy,
	// but the wait time will never be shorter than this duration.
	MinWait time.Duration
	// MaxWait is the maximum wait time between attempts. The actual wait time is governed by the BackoffStrategy,
	// but the wait time will never be longer than this duration.
	MaxWait time.Duration
}

// TimeoutConfig represents the settings for timeout at various points in a request lifecycle in a retryable http client.
type TimeoutConfig struct {
	// DialTimeout is the maximum duration that connection can take before a request attempt is timed out.
	DialTimeout time.Duration
	// ResponseHeaderTimeout is the maximum duration waiting for response headers before a request attempt is timed out.
	// This starts after the entire request body is uploaded to the remote endpoint and stops when the request headers
	// are fully read. It does not include reading the body.
	ResponseHeaderTimeout time.Duration
	// RequestTimeout is the maximum duration before the entire request attempt is timed out. This starts when the
	// client starts the connection attempt and ends when the entire response body is read.
	RequestTimeout time.Duration
}

// RetryableClientConfig is the complete config for a retryable http client
type RetryableClientConfig struct {
	TimeoutConfig
	RetryConfig
}
