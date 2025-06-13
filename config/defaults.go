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

// Config (root) defaults
const (
	defaultMetricsNetwork = "tcp"
	defaultMetadataStore  = "db"
)

// ServiceConfig defaults
const (
	DefaultImageServiceAddress = "/run/containerd/containerd.sock"
)

// ParallelPullUnpack defaults
const (
	Unbounded = -1
)

// FSConfig defaults
const (
	defaultFuseTimeoutSec = 1

	// defaultBgSilencePeriodMsec specifies the amount of time the background fetcher will wait once a new layer comes in
	// before (re)starting fetches.
	defaultBgSilencePeriodMsec = 30_000

	// defaultBgFetchPeriodMsec specifies how often the fetch will occur.
	// The background fetcher will fetch a single span every `defaultFetchPeriod`.
	defaultBgFetchPeriodMsec = 500

	// defaultBgMaxQueueSize specifies the maximum size of the bg-fetcher work queue i.e., the maximum number
	// of span managers that can be queued. In case of overflow, the `Add` call
	// will block until a span manager is removed from the workqueue.
	defaultBgMaxQueueSize = 100

	// defaultBgMetricEmitPeriodSec is the default amount of interval at which the background fetcher emits metrics
	defaultBgMetricEmitPeriodSec = 10

	// defaultMountTimeoutSec is the amount of time Mount will time out if a layer can't be resolved.
	defaultMountTimeoutSec = 30

	// defaultFuseMetricsEmitWaitDurationSec is the amount of time the snapshotter will wait before emitting the metrics for FUSE operation.
	defaultFuseMetricsEmitWaitDurationSec = 60

	// defaultMaxConcurrency is the maximum number of layers allowed to be pulled at once
	defaultMaxConcurrency = 100

	defaultValidIntervalSec = 60

	defaultFetchTimeoutSec = 300

	// defaultDialTimeoutMsec is the default number of milliseconds before timeout while connecting to a remote endpoint. See `TimeoutConfig.DialTimeout`.
	defaultDialTimeoutMsec = 3_000
	// defaultResponseHeaderTimeoutMsec is the default number of milliseconds before timeout while waiting for response header from a remote endpoint. See `TimeoutConfig.ResponseHeaderTimeout`.
	defaultResponseHeaderTimeoutMsec = 3_000
	// defaultRequestTimeoutMsec is the default number of milliseconds that the entire request can take before timeout. See `TimeoutConfig.RequestTimeout`.
	defaultRequestTimeoutMsec = 300_000

	// defaults based on a target total retry time of at least 5s. 30*((2^8)-1)>5000

	// defaultMaxRetries is the default number of retries that a retryable request will make. See `RetryConfig.MaxRetries`.
	defaultMaxRetries = 8
	// defaultMinWaitMsec is the default minimum number of milliseconds between attempts. See `RetryConfig.MinWait`.
	defaultMinWaitMsec = 30
	// defaultMaxWaitMsec is the default maximum number of milliseconds between attempts. See `RetryConfig.MaxWait`.
	defaultMaxWaitMsec = 300_000

	// DefaultContentStore chooses the soci or containerd content store as the default
	DefaultContentStoreType = "containerd"

	// DefaultSOCIV1Enable is the default value for whether SOCI v1 is enabled
	DefaultSOCIV1Enable = false

	// DefaultSOCIV2Enable is the default value for whether SOCI v2 is enabled
	DefaultSOCIV2Enable = true

	// DefaultParallelPullEnable is the default value for whether parallel pull and unpack is enabled
	DefaultParallelPullUnpackEnable = false

	// Defaults for ParallelPullUnpack.
	// The default values should mirror default containerd values.

	// defaultMaxConcurrentDownloads sets the default value for how many reads can be done over the network at a time.
	defaultMaxConcurrentDownloads = Unbounded
	// defaultMaxConcurrentDownloadsPerImage sets the default value for how many reads a single image can do over the network at a time.
	// This must be less than or equal to MaxConcurrentDownloads.
	defaultMaxConcurrentDownloadsPerImage = 3

	// defaultConcurrentDownloadChunkSize sets the default value for the max size, in bytes, of downloaded content per network read.
	// When unbounded, full layer is downloaded in one request
	defaultConcurrentDownloadChunkSize = Unbounded

	// defaultMaxConcurrentUnpacks sets the default value for the max number of layers that will be unpacked at once.
	defaultMaxConcurrentUnpacks = Unbounded
	// defaultMaxConcurrentUnpacks sets the default value for the max number of layers per image that will be unpacked at once.
	// This must be less than or equal to MaxConcurrentUnpacks.
	defaultMaxConcurrentUnpacksPerImage = 1
)
