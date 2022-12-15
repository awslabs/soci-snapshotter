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
	"sync"
	"time"

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
	// prometheus metrics
	Mount             = "mount"
	RemoteRegistryGet = "remote_registry_get"
	NodeReaddir       = "node_readdir"
	InitMetadataStore = "init_metadata_store"
	ReadOnDemand      = "read_on_demand"

	OnDemandReadAccessCount          = "on_demand_read_access_count"
	OnDemandRemoteRegistryFetchCount = "on_demand_remote_registry_fetch_count"
	OnDemandBytesServed              = "on_demand_bytes_served"
	OnDemandBytesFetched             = "on_demand_bytes_fetched"
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
