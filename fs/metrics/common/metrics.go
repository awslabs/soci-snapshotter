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

package commonmetrics

import (
	"context"
	"sync"
	"time"

	"github.com/containerd/containerd/log"
	digest "github.com/opencontainers/go-digest"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	// OperationLatencyKeyMilliseconds is the key for soci operation latency metrics in milliseconds.
	OperationLatencyKeyMilliseconds = "operation_duration_milliseconds"

	// OperationLatencyKeyMicroseconds is the key for soci operation latency metrics in microseconds.
	OperationLatencyKeyMicroseconds = "operation_duration_microseconds"

	// OperationCountKey is the key for soci operation count metrics.
	OperationCountKey = "operation_count"

	// BytesServedKey is the key for any metric related to counting bytes served as the part of specific operation.
	BytesServedKey = "bytes_served"

	// Keep namespace as soci and subsystem as fs.
	namespace = "soci"
	subsystem = "fs"
)

// Lists all metric labels.
const (
	/**
	Metrics emitted via prometheus endpoint. All latency metrics are measured in milliseconds,
	unless specified otherwise.
	*/
	// Mount measures the duration of Mount operation for resolved layer.
	Mount = "mount"

	// RemoteRegistryGet records the roundtrip latency for remote registry GET operation.
	RemoteRegistryGet = "remote_registry_get"

	// NodeReaddir measures how long readdir operation takes (in microseconds).
	NodeReaddir = "node_readdir"

	// InitMetadataStore measures how long it takes to initialize the file metadata store.
	InitMetadataStore = "init_metadata_store"

	// ReadOnDemand measures the latency of on-demand reads (in microseconds).
	ReadOnDemand = "read_on_demand"

	// OnDemandReadAccessCount records the count of on-demand reads.
	OnDemandReadAccessCount = "on_demand_read_access_count"

	// OnDemandRemoteRegistryFetchCount records the count of the on-demand fetch requests to remote registry.
	OnDemandRemoteRegistryFetchCount = "on_demand_remote_registry_fetch_count"

	// OnDemandBytesServed counts the number of bytes served on-demand.
	OnDemandBytesServed = "on_demand_bytes_served"

	// OnDemandBytesFetched counts the number of bytes fetched on-demand.
	OnDemandBytesFetched = "on_demand_bytes_fetched"

	/**
	Metrics logged in snapshotter's log.
	All latencies here are measured in milliseconds, unless stated otherwise.
	*/
	// MountLayerToLastOnDemandFetch measures the latency of last on demand fetch,
	// which must be invoked at the end of background fetch operation only.
	// Since this is expected to happen only once per container launch, it is recorded as a log line,
	// instead of being emitted as a metric.
	// The algorithm for recording is as follows:
	// 1. Record the mount start time.
	// 2. Constantly record the timestamps when the on-demand fetch happening for each layer.
	// 3. On background fetch completed, measure the difference between the last on demand fetch
	// and mount start time and record it.
	MountLayerToLastOnDemandFetch = "mount_layer_to_last_on_demand_fetch"

	// BackgroundFetchTotal measures how long it took for background fetch to complete for a specific layer.
	BackgroundFetchTotal = "background_fetch_total"

	// BackgroundFetchDownload measures the time to download all data for a specific layer during background fetch.
	BackgroundFetchDownload = "background_fetch_download"

	// BackgroundFetchDecompress used to measure the time to decompress all data for a specific layer
	// during background fetch. Since we plan to re-work the background fetch, it may not be needed anymore.
	// Keeping it here until background fetcher re-work is completed.
	BackgroundFetchDecompress = "background_fetch_decompress"
)

var (
	// Buckets for OperationLatency metrics.
	latencyBucketsMilliseconds = []float64{1, 2, 4, 8, 16, 32, 64, 128, 256, 512, 1024, 2048, 4096, 8192, 16384} // in milliseconds
	latencyBucketsMicroseconds = []float64{1, 2, 4, 8, 16, 32, 64, 128, 256, 512, 1024}                          // in microseconds

	// operationLatencyMilliseconds collects operation latency numbers in milliseconds grouped by
	// operation, type and layer digest.
	operationLatencyMilliseconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      OperationLatencyKeyMilliseconds,
			Help:      "Latency in milliseconds of soci snapshotter operations. Broken down by operation type and layer sha.",
			Buckets:   latencyBucketsMilliseconds,
		},
		[]string{"operation_type", "layer"},
	)

	// operationLatencyMicroseconds collects operation latency numbers in microseconds grouped by
	// operation, type and layer digest.
	operationLatencyMicroseconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      OperationLatencyKeyMicroseconds,
			Help:      "Latency in microseconds of soci snapshotter operations. Broken down by operation type and layer sha.",
			Buckets:   latencyBucketsMicroseconds,
		},
		[]string{"operation_type", "layer"},
	)

	// operationCount collects operation count numbers by operation
	// type and layer sha.
	operationCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      OperationCountKey,
			Help:      "The count of soci snapshotter operations. Broken down by operation type and layer sha.",
		},
		[]string{"operation_type", "layer"},
	)

	// bytesCount reflects the number of bytes served as the part of specitic operation type per layer sha.
	bytesCount = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      BytesServedKey,
			Help:      "The number of bytes served per soci snapshotter operations. Broken down by operation type and layer sha.",
		},
		[]string{"operation_type", "layer"},
	)
)

var register sync.Once

// sinceInMilliseconds gets the time since the specified start in milliseconds.
// The division by 1e6 is made to have the milliseconds value as floating point number, since the native method
// .Milliseconds() returns an integer value and you can lost a precision for sub-millisecond values.
func sinceInMilliseconds(start time.Time) float64 {
	return float64(time.Since(start).Nanoseconds()) / 1e6
}

// sinceInMicroseconds gets the time since the specified start in microseconds.
// The division by 1e3 is made to have the microseconds value as floating point number, since the native method
// .Microseconds() returns an integer value and you can lost a precision for sub-microsecond values.
func sinceInMicroseconds(start time.Time) float64 {
	return float64(time.Since(start).Nanoseconds()) / 1e3
}

// Register registers metrics. This is always called only once.
func Register() {
	register.Do(func() {
		prometheus.MustRegister(operationLatencyMilliseconds)
		prometheus.MustRegister(operationLatencyMicroseconds)
		prometheus.MustRegister(operationCount)
		prometheus.MustRegister(bytesCount)
	})
}

// MeasureLatencyInMilliseconds wraps the labels attachment as well as calling Observe into a single method.
// Right now we attach the operation and layer digest, so it's possible to see the breakdown for latency
// by operation and individual layers.
// If you want this to be layer agnostic, just pass the digest from empty string, e.g.
// layerDigest := digest.FromString("")
func MeasureLatencyInMilliseconds(operation string, layer digest.Digest, start time.Time) {
	operationLatencyMilliseconds.WithLabelValues(operation, layer.String()).Observe(sinceInMilliseconds(start))
}

// MeasureLatencyInMicroseconds wraps the labels attachment as well as calling Observe into a single method.
// Right now we attach the operation and layer digest, so it's possible to see the breakdown for latency
// by operation and individual layers.
// If you want this to be layer agnostic, just pass the digest from empty string, e.g.
// layerDigest := digest.FromString("")
func MeasureLatencyInMicroseconds(operation string, layer digest.Digest, start time.Time) {
	operationLatencyMicroseconds.WithLabelValues(operation, layer.String()).Observe(sinceInMicroseconds(start))
}

// IncOperationCount wraps the labels attachment as well as calling Inc into a single method.
func IncOperationCount(operation string, layer digest.Digest) {
	operationCount.WithLabelValues(operation, layer.String()).Inc()
}

// AddBytesCount wraps the labels attachment as well as calling Add into a single method.
func AddBytesCount(operation string, layer digest.Digest, bytes int64) {
	bytesCount.WithLabelValues(operation, layer.String()).Add(float64(bytes))
}

// WriteLatencyLogValue wraps writing the log info record for latency in milliseconds. The log record breaks down by operation and layer digest.
func WriteLatencyLogValue(ctx context.Context, layer digest.Digest, operation string, start time.Time) {
	ctx = log.WithLogger(ctx, log.G(ctx).WithField("metrics", "latency").WithField("operation", operation).WithField("layer_sha", layer.String()))
	log.G(ctx).Infof("value=%v milliseconds", sinceInMilliseconds(start))
}

// WriteLatencyWithBytesLogValue wraps writing the log info record for latency in milliseconds with adding the size in bytes.
// The log record breaks down by operation, layer digest and byte value.
func WriteLatencyWithBytesLogValue(ctx context.Context, layer digest.Digest, latencyOperation string, start time.Time, bytesMetricName string, bytesMetricValue int64) {
	ctx = log.WithLogger(ctx, log.G(ctx).WithField("metrics", "latency").WithField("operation", latencyOperation).WithField("layer_sha", layer.String()))
	log.G(ctx).Infof("value=%v milliseconds; %v=%v bytes", sinceInMilliseconds(start), bytesMetricName, bytesMetricValue)
}

// LogLatencyForLastOnDemandFetch implements a special case for measuring the latency of last on demand fetch, which must be invoked at the end of
// background fetch operation only. Since this is expected to happen only once per container launch, it writes a log line,
// instead of directly emitting a metric.
// We do that in the following way:
// 1. We record the mount start time
// 2. We constantly record the timestamps when we do on demand fetch for each layer sha
// 3. On background fetch completed we measure the difference between the last on demand fetch and mount start time
// and record it as a metric
func LogLatencyForLastOnDemandFetch(ctx context.Context, layer digest.Digest, start time.Time, end time.Time) {
	diffInMilliseconds := float64(end.Sub(start).Milliseconds())
	// value can be negative if we pass the default value for time.Time as `end`
	// this can happen if there were no on-demand fetch for the particular layer
	if diffInMilliseconds > 0 {
		ctx = log.WithLogger(ctx, log.G(ctx).WithField("metrics", "latency").WithField("operation", MountLayerToLastOnDemandFetch).WithField("layer_sha", layer.String()))
		log.G(ctx).Infof("value=%v milliseconds", diffInMilliseconds)
	}
}
