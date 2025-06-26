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

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestConfigDefaults(t *testing.T) {
	cfg := NewConfig()

	tests := []struct {
		name     string
		expected any
		actual   any
	}{
		{
			name:     "soci v1 enabled",
			expected: DefaultSOCIV1Enable,
			actual:   cfg.PullModes.SOCIv1.Enable,
		},
		{
			name:     "soci v2 enabled",
			expected: DefaultSOCIV2Enable,
			actual:   cfg.PullModes.SOCIv2.Enable,
		},
		{
			name:     "parallel pull enabled",
			expected: DefaultParallelPullUnpackEnable,
			actual:   cfg.PullModes.Parallel.Enable,
		},
		{
			name:     "metrics network",
			expected: defaultMetricsNetwork,
			actual:   cfg.MetricsNetwork,
		},
		{
			name:     "metadata store",
			expected: defaultMetadataStore,
			actual:   cfg.MetadataStore,
		},
		{
			name:     "cri image service address",
			expected: DefaultImageServiceAddress,
			actual:   cfg.CRIKeychainConfig.ImageServicePath,
		},
		{
			name:     "mount timeout",
			expected: int64(defaultMountTimeoutSec),
			actual:   cfg.MountTimeoutSec,
		},
		{
			name:     "fuse metric emit wait duration",
			expected: int64(defaultFuseMetricsEmitWaitDurationSec),
			actual:   cfg.FuseMetricsEmitWaitDurationSec,
		},
		{
			name:     "max concurrency",
			expected: int64(defaultMaxConcurrency),
			actual:   cfg.MaxConcurrency,
		},
		{
			name:     "fuse attr timeout",
			expected: int64(defaultFuseTimeoutSec),
			actual:   cfg.FuseConfig.AttrTimeout,
		},
		{
			name:     "fuse entry timeout",
			expected: int64(defaultFuseTimeoutSec),
			actual:   cfg.FuseConfig.EntryTimeout,
		},
		{
			name:     "fuse negative timeout",
			expected: int64(defaultFuseTimeoutSec),
			actual:   cfg.FuseConfig.NegativeTimeout,
		},
		{
			name:     "fuse directory cache direct",
			expected: true,
			actual:   cfg.FSConfig.DirectoryCacheConfig.Direct,
		},
		{
			name:     "bg fetch period",
			expected: int64(defaultBgFetchPeriodMsec),
			actual:   cfg.BackgroundFetchConfig.FetchPeriodMsec,
		},
		{
			name:     "bg silence period",
			expected: int64(defaultBgSilencePeriodMsec),
			actual:   cfg.BackgroundFetchConfig.SilencePeriodMsec,
		},

		{
			name:     "bg max queue size",
			expected: int(defaultBgMaxQueueSize),
			actual:   cfg.BackgroundFetchConfig.MaxQueueSize,
		},
		{
			name:     "bg emit metrics period",
			expected: int64(defaultBgMetricEmitPeriodSec),
			actual:   cfg.BackgroundFetchConfig.EmitMetricPeriodSec,
		},
		{
			name:     "http dial timeout",
			expected: int64(defaultDialTimeoutMsec),
			actual:   cfg.RetryableHTTPClientConfig.TimeoutConfig.DialTimeoutMsec,
		},
		{
			name:     "http header timeout",
			expected: int64(defaultResponseHeaderTimeoutMsec),
			actual:   cfg.RetryableHTTPClientConfig.TimeoutConfig.ResponseHeaderTimeoutMsec,
		},
		{
			name:     "http request timeout",
			expected: int64(defaultRequestTimeoutMsec),
			actual:   cfg.RetryableHTTPClientConfig.TimeoutConfig.RequestTimeoutMsec,
		},
		{
			name:     "http max retries",
			expected: int(defaultMaxRetries),
			actual:   cfg.RetryableHTTPClientConfig.RetryConfig.MaxRetries,
		},
		{
			name:     "http retry min wait",
			expected: int64(defaultMinWaitMsec),
			actual:   cfg.RetryableHTTPClientConfig.RetryConfig.MinWaitMsec,
		},
		{
			name:     "http retry max wait",
			expected: int64(defaultMaxWaitMsec),
			actual:   cfg.RetryableHTTPClientConfig.RetryConfig.MaxWaitMsec,
		},
		{
			name:     "blob valid interval",
			expected: int64(defaultValidIntervalSec),
			actual:   cfg.BlobConfig.ValidInterval,
		},
		{
			name:     "blob fetch timeout",
			expected: int64(defaultFetchTimeoutSec),
			actual:   cfg.BlobConfig.FetchTimeoutSec,
		},
		{
			name:     "content store type",
			expected: SociContentStoreType,
			actual:   cfg.ContentStoreConfig.Type,
		},
		{
			name:     "max concurrent downloads",
			expected: int64(defaultMaxConcurrentDownloads),
			actual:   cfg.PullModes.Parallel.MaxConcurrentDownloads,
		},
		{
			name:     "max concurrent downloads per image",
			expected: int64(defaultMaxConcurrentDownloadsPerImage),
			actual:   cfg.PullModes.Parallel.MaxConcurrentDownloadsPerImage,
		},
		{
			name:     "concurrent download chunk size",
			expected: int64(defaultConcurrentDownloadChunkSize),
			actual:   cfg.PullModes.Parallel.ConcurrentDownloadChunkSize,
		},
		{
			name:     "max concurrent unpack",
			expected: int64(defaultMaxConcurrentUnpacks),
			actual:   cfg.PullModes.Parallel.MaxConcurrentUnpacks,
		},
		{
			name:     "max concurrent unpack per image",
			expected: int64(defaultMaxConcurrentUnpacksPerImage),
			actual:   cfg.PullModes.Parallel.MaxConcurrentUnpacksPerImage,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.expected != tc.actual {
				t.Fatalf("invalid default value. expected: %v. actual: %v", tc.expected, tc.actual)
			}
		})
	}
}

// TestNewConfigFromToml asserts snapshotter configuration is parsed correctly.
func TestNewConfigFromToml(t *testing.T) {
	tests := []struct {
		name   string
		config []byte
		assert func(t *testing.T, actual *Config, err error)
	}{
		{
			name:   "DefaultConfig",
			config: []byte(``),
			assert: func(t *testing.T, actual *Config, err error) {
				if err != nil {
					t.Errorf("Expected no error, got %v", err)
				}
				cfg := NewConfig()
				if !reflect.DeepEqual(cfg, actual) {
					t.Errorf("Expected config %+v, got %+v", cfg, actual)
				}
			},
		},
		{
			name: "DecompressionConfig",
			config: []byte(`
[pull_modes.parallel_pull_unpack.decompress_streams."gzip"]
path = "/usr/bin/gzip"
args = ["-d", "-c"]

[pull_modes.parallel_pull_unpack.decompress_streams."zstd"]
path = "/usr/bin/zstd"
args = ["-d", "-c"]
`),
			assert: func(t *testing.T, actual *Config, err error) {
				if err != nil {
					t.Errorf("Expected no error, got %v", err)
				}
				if len(actual.PullModes.Parallel.DecompressStreams) != 2 {
					t.Errorf("Expected 2 decompression streams, got %d", len(actual.PullModes.Parallel.DecompressStreams))
				}
				if actual.PullModes.Parallel.DecompressStreams["gzip"].Path != "/usr/bin/gzip" {
					t.Errorf("Expected gzip path to be /usr/bin/gzip, got %s", actual.PullModes.Parallel.DecompressStreams["gzip"].Path)
				}
				if len(actual.PullModes.Parallel.DecompressStreams["gzip"].Args) != 2 {
					t.Errorf("Expected two args, got %d", len(actual.PullModes.Parallel.DecompressStreams["gzip"].Args))
				}
				if actual.PullModes.Parallel.DecompressStreams["zstd"].Path != "/usr/bin/zstd" {
					t.Errorf("Expected zstd path to be /usr/bin/zstd, got %s", actual.PullModes.Parallel.DecompressStreams["zstd"].Path)
				}
				if len(actual.PullModes.Parallel.DecompressStreams["zstd"].Args) != 2 {
					t.Errorf("Expected two args, got %d", len(actual.PullModes.Parallel.DecompressStreams["zstd"].Args))
				}
			},
		},
		{
			name: "ConcurrentChunkSizes",
			config: []byte(`
[pull_modes.parallel_pull_unpack]
concurrent_download_chunk_size = "1MB"
`),
			assert: func(t *testing.T, actual *Config, err error) {
				if err != nil {
					t.Errorf("Expected no error, got %v", err)
				}
				if actual.PullModes.Parallel.ConcurrentDownloadChunkSize != 1*1024*1024 {
					t.Errorf("Expected concurrent_download_chunk_size to be %d, got %d", 1*1024*1024, actual.PullModes.Parallel.ConcurrentDownloadChunkSize)
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.toml")
			err := os.WriteFile(path, test.config, 0644)
			if err != nil {
				t.Fatalf("Failed to write config file: %v", err)
			}

			actual, err := NewConfigFromToml(path)
			test.assert(t, actual, err)
		})
	}
}

func TestSizeParser(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int64
		experror bool
	}{
		{"Empty", "", -1, false},
		{"Bytes", "100B", 100, false},
		{"Kilobytes", "1KB", 1024, false},
		{"Megabytes", "1MB", 1024 * 1024, false},
		{"Gigabytes", "1GB", 1024 * 1024 * 1024, false},
		{"WithDecimal", "1.5MB", int64(1.5 * 1024 * 1024), false},
		{"MixedCase", "1Kb", 1024, false},
		{"WithSpaces", "  1 MB  ", 1024 * 1024, false},
		{"Zero", "0", -1, false},
		{"Negative", "-100B", -100, true},
		{"Invalid", "invalid", -1, true},
		{"WithUnit", "1000m", -1, true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual, err := parseSize(test.input)
			if err != nil {
				if !test.experror {
					t.Errorf("Unexpected error: %v", err)
				}
			} else if test.experror {
				t.Errorf("Expected error for input %q, but got none", test.input)
			} else if actual != test.expected {
				t.Errorf("Expected %d, got %d for input %q", test.expected, actual, test.input)
			}
		})
	}
}
