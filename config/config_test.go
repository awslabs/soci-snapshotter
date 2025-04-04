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
