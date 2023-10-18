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

	"github.com/opencontainers/go-digest"
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

	// ImageOperationCountKey is the key for any metric related to operation count metric at the image level (as opposed to layer).
	ImageOperationCountKey = "image_operation_count_key"

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
	SynchronousRead   = "synchronous_read"
	BackgroundFetch   = "background_fetch"

	SynchronousReadCount              = "synchronous_read_count"
	SynchronousReadRegistryFetchCount = "synchronous_read_remote_registry_fetch_count" // TODO revisit (wrong place)
	SynchronousBytesServed            = "synchronous_bytes_served"

	// fuse operation failure metrics
	FuseNodeGetattrFailureCount     = "fuse_node_getattr_failure_count"
	FuseNodeListxattrFailureCount   = "fuse_node_listxattr_failure_count"
	FuseNodeLookupFailureCount      = "fuse_node_lookup_failure_count"
	FuseNodeOpenFailureCount        = "fuse_node_open_failure_count"
	FuseNodeReaddirFailureCount     = "fuse_node_readdir_failure_count"
	FuseFileReadFailureCount        = "fuse_file_read_failure_count"
	FuseFileGetattrFailureCount     = "fuse_file_getattr_failure_count"
	FuseWhiteoutGetattrFailureCount = "fuse_whiteout_getattr_failure_count"
	FuseUnknownFailureCount         = "fuse_unknown_operation_failure_count"

	// TODO this metric is not available now. This needs to go down to BlobReader where the actuall http call is issued
	SynchronousBytesFetched = "synchronous_bytes_fetched"

	// Number of times the snapshotter falls back to use a normal overlay mount instead of mounting the layer as a FUSE mount.
	// Note that a layer not having a ztoc is NOT classified as an error, even though `fs.Mount` returns an error in that case.
	FuseMountFailureCount = "fuse_mount_failure_count"

	// Number of errors of span fetch by background fetcher
	BackgroundSpanFetchFailureCount = "background_span_fetch_failure_count"

	// Number of spans fetched by background fetcher
	BackgroundSpanFetchCount = "background_span_fetch_count"

	// Number of items in the work queue of background fetcher
	BackgroundFetchWorkQueueSize = "background_fetch_work_queue_size"
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

	// bytesCount reflects the number of bytes served as the part of specific operation type per layer sha.
	bytesCount = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      BytesServedKey,
			Help:      "The number of bytes served per soci snapshotter operations. Broken down by operation type and layer sha.",
		},
		[]string{"operation_type", "layer"},
	)

	imageOperationCount = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      ImageOperationCountKey,
			Help:      "The count of soci snapshotter operations. Broken down by operation type and image digest.",
		},
		[]string{"operation_type", "image"})
)

var register sync.Once

// sinceInMilliseconds gets the time since the specified start in milliseconds.
// The division is made to have the milliseconds value as floating point number, since the native method
// .Milliseconds() returns an integer value and you can lose precision for sub-millisecond values.
func sinceInMilliseconds(start time.Time) float64 {
	return float64(time.Since(start).Nanoseconds()) / float64(time.Millisecond/time.Nanosecond)
}

// sinceInMicroseconds gets the time since the specified start in microseconds.
// The division is made to have the microseconds value as floating point number, since the native method
// .Microseconds() returns an integer value and you can lose precision for sub-microsecond values.
func sinceInMicroseconds(start time.Time) float64 {
	return float64(time.Since(start).Nanoseconds()) / float64(time.Microsecond/time.Nanosecond)
}

// Register registers metrics. This is always called only once.
func Register() {
	register.Do(func() {
		prometheus.MustRegister(operationLatencyMilliseconds)
		prometheus.MustRegister(operationLatencyMicroseconds)
		prometheus.MustRegister(operationCount)
		prometheus.MustRegister(bytesCount)
		prometheus.MustRegister(imageOperationCount)
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

// AddImageOperationCount wraps the labels attachment as well as calling Add into a single method.
func AddImageOperationCount(operation string, image digest.Digest, count int32) {
	imageOperationCount.WithLabelValues(operation, image.String()).Add(float64(count))
}
