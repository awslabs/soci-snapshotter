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

import (
	"strings"

	"github.com/containerd/containerd/defaults"
)

type FSConfig struct {
	HTTPCacheType                  string `toml:"http_cache_type"`
	FSCacheType                    string `toml:"filesystem_cache_type"`
	ResolveResultEntry             int    `toml:"resolve_result_entry"`
	Debug                          bool   `toml:"debug"`
	DisableVerification            bool   `toml:"disable_verification"`
	MaxConcurrency                 int64  `toml:"max_concurrency"`
	NoPrometheus                   bool   `toml:"no_prometheus"`
	MountTimeoutSec                int64  `toml:"mount_timeout_sec"`
	FuseMetricsEmitWaitDurationSec int64  `toml:"fuse_metrics_emit_wait_duration_sec"`

	RetryableHTTPClientConfig `toml:"http"`
	BlobConfig                `toml:"blob"`

	DirectoryCacheConfig `toml:"directory_cache"`

	FuseConfig `toml:"fuse"`

	BackgroundFetchConfig `toml:"background_fetch"`

	ContentStoreConfig `toml:"content_store"`
}

// BlobConfig is config for layer blob management.
type BlobConfig struct {
	ValidInterval        int64 `toml:"valid_interval"`
	FetchTimeoutSec      int64 `toml:"fetching_timeout_sec"`
	MaxRetries           int   `toml:"max_retries"`
	MinWaitMsec          int64 `toml:"min_wait_msec"`
	MaxWaitMsec          int64 `toml:"max_wait_msec"`
	CheckAlways          bool  `toml:"check_always"`
	ForceSingleRangeMode bool  `toml:"force_single_range_mode"`

	// MaxSpanVerificationRetries defines the number of additional times fetch
	// will be invoked in case of span verification failure.
	MaxSpanVerificationRetries int `toml:"max_span_verification_retries"`
}

// DirectoryCacheConfig is config for directory-based cache.
type DirectoryCacheConfig struct {
	MaxLRUCacheEntry int  `toml:"max_lru_cache_entry"`
	MaxCacheFds      int  `toml:"max_cache_fds"`
	SyncAdd          bool `toml:"sync_add"`
	Direct           bool `toml:"direct"`
}

func defaultDirectoryCacheConfig(cfg *Config) error {
	cfg.FSConfig.DirectoryCacheConfig.Direct = true
	return nil
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
	MinWaitMsec int64
	// MaxWait is the maximum wait time between attempts. The actual wait time is governed by the BackoffStrategy,
	// but the wait time will never be longer than this duration.
	MaxWaitMsec int64
}

// TimeoutConfig represents the settings for timeout at various points in a request lifecycle in a retryable http client.
type TimeoutConfig struct {
	// DialTimeout is the maximum duration that connection can take before a request attempt is timed out.
	DialTimeoutMsec int64
	// ResponseHeaderTimeout is the maximum duration waiting for response headers before a request attempt is timed out.
	// This starts after the entire request body is uploaded to the remote endpoint and stops when the request headers
	// are fully read. It does not include reading the body.
	ResponseHeaderTimeoutMsec int64
	// RequestTimeout is the maximum duration before the entire request attempt is timed out. This starts when the
	// client starts the connection attempt and ends when the entire response body is read.
	RequestTimeoutMsec int64
}

// RetryableHTTPClientConfig is the complete config for a retryable http client
type RetryableHTTPClientConfig struct {
	TimeoutConfig
	RetryConfig
}

type ContentStoreType string

const (
	ContainerdContentStoreType ContentStoreType = "containerd"
	SociContentStoreType       ContentStoreType = "soci"
)

func TrimSocketAddress(address string) string {
	return strings.TrimPrefix(address, "unix://")
}

// ContentStoreConfig chooses and configures the content store
type ContentStoreConfig struct {
	Type ContentStoreType `toml:"type"`

	// ContainerdAddress is the containerd socket address.
	// Applicable if and only if using containerd content store.
	ContainerdAddress string `toml:"containerd_address"`
}

func parseFSConfig(cfg *Config) error {
	// Parse top level fs config
	if cfg.MountTimeoutSec == 0 {
		cfg.MountTimeoutSec = defaultMountTimeoutSec
	}
	if cfg.FuseMetricsEmitWaitDurationSec == 0 {
		cfg.FuseMetricsEmitWaitDurationSec = defaultFuseMetricsEmitWaitDurationSec
	}
	if cfg.MaxConcurrency == 0 {
		cfg.MaxConcurrency = defaultMaxConcurrency
	}
	// If MaxConcurrency is negative, disable concurrency limits entirely.
	if cfg.MaxConcurrency < 0 {
		cfg.MaxConcurrency = 0
	}
	// Parse nested fs configs
	parsers := []configParser{parseFuseConfig, parseBackgroundFetchConfig, parseRetryableHTTPClientConfig, parseBlobConfig, parseContentStoreConfig}
	for _, p := range parsers {
		if err := p(cfg); err != nil {
			return err
		}
	}
	return nil
}

func parseFuseConfig(cfg *Config) error {
	if cfg.FuseConfig.AttrTimeout == 0 {
		cfg.FuseConfig.AttrTimeout = defaultFuseTimeoutSec
	}

	if cfg.FuseConfig.EntryTimeout == 0 {
		cfg.FuseConfig.EntryTimeout = defaultFuseTimeoutSec
	}

	if cfg.FuseConfig.NegativeTimeout == 0 {
		cfg.FuseConfig.NegativeTimeout = defaultFuseTimeoutSec
	}
	return nil
}

func parseBackgroundFetchConfig(cfg *Config) error {
	if cfg.BackgroundFetchConfig.FetchPeriodMsec == 0 {
		cfg.BackgroundFetchConfig.FetchPeriodMsec = defaultBgFetchPeriodMsec
	}
	if cfg.BackgroundFetchConfig.SilencePeriodMsec == 0 {
		cfg.BackgroundFetchConfig.SilencePeriodMsec = defaultBgSilencePeriodMsec
	}

	if cfg.BackgroundFetchConfig.MaxQueueSize == 0 {
		cfg.BackgroundFetchConfig.MaxQueueSize = defaultBgMaxQueueSize
	}

	if cfg.BackgroundFetchConfig.EmitMetricPeriodSec == 0 {
		cfg.BackgroundFetchConfig.EmitMetricPeriodSec = defaultBgMetricEmitPeriodSec
	}
	return nil
}

func parseRetryableHTTPClientConfig(cfg *Config) error {
	if cfg.RetryableHTTPClientConfig.TimeoutConfig.DialTimeoutMsec == 0 {
		cfg.RetryableHTTPClientConfig.TimeoutConfig.DialTimeoutMsec = defaultDialTimeoutMsec
	}

	if cfg.RetryableHTTPClientConfig.TimeoutConfig.ResponseHeaderTimeoutMsec == 0 {
		cfg.RetryableHTTPClientConfig.TimeoutConfig.ResponseHeaderTimeoutMsec = defaultResponseHeaderTimeoutMsec

	}
	if cfg.RetryableHTTPClientConfig.TimeoutConfig.RequestTimeoutMsec == 0 {
		cfg.RetryableHTTPClientConfig.TimeoutConfig.RequestTimeoutMsec = defaultRequestTimeoutMsec

	}
	if cfg.RetryableHTTPClientConfig.RetryConfig.MaxRetries == 0 {
		cfg.RetryableHTTPClientConfig.RetryConfig.MaxRetries = defaultMaxRetries

	}
	if cfg.RetryableHTTPClientConfig.RetryConfig.MinWaitMsec == 0 {
		cfg.RetryableHTTPClientConfig.RetryConfig.MinWaitMsec = defaultMinWaitMsec

	}
	if cfg.RetryableHTTPClientConfig.RetryConfig.MaxWaitMsec == 0 {
		cfg.RetryableHTTPClientConfig.RetryConfig.MaxWaitMsec = defaultMaxWaitMsec
	}
	return nil
}

func parseBlobConfig(cfg *Config) error {
	if cfg.BlobConfig.ValidInterval == 0 {
		cfg.BlobConfig.ValidInterval = defaultValidIntervalSec
	}
	if cfg.BlobConfig.CheckAlways {
		cfg.BlobConfig.ValidInterval = 0
	}
	if cfg.BlobConfig.FetchTimeoutSec == 0 {
		cfg.BlobConfig.FetchTimeoutSec = defaultFetchTimeoutSec
	}
	if cfg.BlobConfig.MaxRetries == 0 {
		cfg.BlobConfig.MaxRetries = cfg.RetryableHTTPClientConfig.RetryConfig.MaxRetries
	}
	if cfg.BlobConfig.MinWaitMsec == 0 {
		cfg.BlobConfig.MinWaitMsec = cfg.RetryableHTTPClientConfig.RetryConfig.MinWaitMsec
	}
	if cfg.BlobConfig.MaxWaitMsec == 0 {
		cfg.BlobConfig.MaxWaitMsec = cfg.RetryableHTTPClientConfig.RetryConfig.MaxWaitMsec
	}
	return nil
}

func parseContentStoreConfig(cfg *Config) error {
	if cfg.ContentStoreConfig.Type == "" {
		// We are intentionally not using containerd as the default content store until we do more testing.
		// Until we are confident, use the SOCI store instead.
		cfg.ContentStoreConfig.Type = SociContentStoreType
	}
	if cfg.ContentStoreConfig.ContainerdAddress == "" {
		cfg.ContentStoreConfig.ContainerdAddress = defaults.DefaultAddress
	} else {
		cfg.ContentStoreConfig.ContainerdAddress = TrimSocketAddress(cfg.ContentStoreConfig.ContainerdAddress)
	}
	return nil
}
