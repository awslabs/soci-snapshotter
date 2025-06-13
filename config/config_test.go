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
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.expected != tc.actual {
				t.Fatalf("invalid default value. expected: %v. actual: %v", tc.expected, tc.actual)
			}
		})
	}
}

func TestNewConfigFromToml(t *testing.T) {
	tests := []struct {
		name   string
		setup  func(testing.TB, string)
		assert func(testing.TB, *Config, error)
	}{
		{
			name: "basic config(import)",
			setup: func(tb testing.TB, dir string) {
				rootCfg := `
					imports = ["http.toml", "blob.toml", "fuse.toml"]
				`
				if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte(rootCfg), 0o600); err != nil {
					tb.Fatalf("unexpected error: %v", err)
				}

				httpCfg := `
					[http]
					MaxRetries = 10
					MinWaitMsec = 1000
					MaxWaitMsec = 2000
					DialTimeoutMsec = 1000
					ResponseHeaderTimeoutMsec = 2000
					RequestTimeoutMsec = 3000
				`
				if err := os.WriteFile(filepath.Join(dir, "http.toml"), []byte(httpCfg), 0o600); err != nil {
					tb.Fatalf("unexpected error: %v", err)
				}

				blobCfg := `
					[blob]
					valid_interval = 1000
					fetching_timeout_sec = 2000
				`
				if err := os.WriteFile(filepath.Join(dir, "blob.toml"), []byte(blobCfg), 0o600); err != nil {
					tb.Fatalf("unexpected error: %v", err)
				}

				fuseCfg := `
					[fuse]
					attr_timeout = 1000
				`
				if err := os.WriteFile(filepath.Join(dir, "fuse.toml"), []byte(fuseCfg), 0o600); err != nil {
					tb.Fatalf("unexpected error: %v", err)
				}
			},
			assert: func(tb testing.TB, cfg *Config, err error) {
				if err != nil {
					tb.Fatalf("unexpected error: %v", err)
				}

				// http.toml
				if cfg.RetryConfig.MaxRetries != 10 {
					tb.Fatalf("unexpected value. expected: %v. actual: %v", 10, cfg.RetryableHTTPClientConfig.RetryConfig.MaxRetries)
				}
				if cfg.RetryConfig.MinWaitMsec != 1000 {
					tb.Fatalf("unexpected value. expected: %v. actual: %v", 1000, cfg.RetryableHTTPClientConfig.RetryConfig.MinWaitMsec)
				}
				if cfg.RetryConfig.MaxWaitMsec != 2000 {
					tb.Fatalf("unexpected value. expected: %v. actual: %v", 2000, cfg.RetryableHTTPClientConfig.TimeoutConfig.ResponseHeaderTimeoutMsec)
				}
				if cfg.TimeoutConfig.DialTimeoutMsec != 1000 {
					tb.Fatalf("unexpected value. expected: %v. actual: %v", 1000, cfg.RetryableHTTPClientConfig.TimeoutConfig.DialTimeoutMsec)
				}
				if cfg.TimeoutConfig.ResponseHeaderTimeoutMsec != 2000 {
					tb.Fatalf("unexpected value. expected: %v. actual: %v", 2000, cfg.RetryableHTTPClientConfig.TimeoutConfig.ResponseHeaderTimeoutMsec)
				}
				if cfg.TimeoutConfig.RequestTimeoutMsec != 3000 {
					tb.Fatalf("unexpected value. expected: %v. actual: %v", 3000, cfg.RetryableHTTPClientConfig.TimeoutConfig.RequestTimeoutMsec)
				}

				// blob.toml
				if cfg.BlobConfig.ValidInterval != 1000 {
					tb.Fatalf("unexpected value. expected: %v. actual: %v", 1000, cfg.BlobConfig.ValidInterval)
				}

				// fuse.toml
				if cfg.FuseConfig.AttrTimeout != 1000 {
					tb.Fatalf("unexpected value. expected: %v. actual: %v", 1000, cfg.FuseConfig.AttrTimeout)
				}
			},
		},
		{
			name: "basic config(override)",
			setup: func(tb testing.TB, dir string) {
				rootCfg := `
					imports = ["http.toml"]

					[http]
					MaxRetries = 10
					MinWaitMsec = 1000
					MaxWaitMsec = 2000
				`
				if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte(rootCfg), 0o600); err != nil {
					tb.Fatalf("unexpected error: %v", err)
				}

				httpCfg := `
					[http]
					MaxRetries = 20
				`
				if err := os.WriteFile(filepath.Join(dir, "http.toml"), []byte(httpCfg), 0o600); err != nil {
					tb.Fatalf("unexpected error: %v", err)
				}
			},
			assert: func(tb testing.TB, cfg *Config, err error) {
				if err != nil {
					tb.Fatalf("unexpected error: %v", err)
				}

				// http.toml
				if cfg.RetryConfig.MaxRetries != 20 {
					tb.Fatalf("unexpected value. expected: %v. actual: %v", 10, cfg.RetryableHTTPClientConfig.RetryConfig.MaxRetries)
				}
				if cfg.RetryConfig.MinWaitMsec != 1000 {
					tb.Fatalf("unexpected value. expected: %v. actual: %v", 1000, cfg.RetryableHTTPClientConfig.RetryConfig.MinWaitMsec)
				}
				if cfg.RetryConfig.MaxWaitMsec != 2000 {
					tb.Fatalf("unexpected value. expected: %v. actual: %v", 2000, cfg.RetryableHTTPClientConfig.TimeoutConfig.ResponseHeaderTimeoutMsec)
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tempDir := t.TempDir()
			test.setup(t, tempDir)

			cfg, err := NewConfigFromToml(filepath.Join(tempDir, "config.toml"))
			test.assert(t, cfg, err)
		})
	}
}
