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

package monitor

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	cm "github.com/awslabs/soci-snapshotter/fs/metrics/common"

	"github.com/awslabs/soci-snapshotter/fs/layer/fuse"
	"github.com/containerd/containerd/log"
	"github.com/opencontainers/go-digest"
)

// NewImageMonitor returns a new ImageMonitor. An ImageMonitor encapsulates the emission of FUSE
// telemetry data at an image level.
func NewImageMonitor(waitP time.Duration) *ImageMonitor {
	return &ImageMonitor{
		OpCounter:  sync.Map{},
		waitPeriod: waitP,
	}
}

type ImageMonitor struct {
	// OpCounter maintains a fuseOperationCounter per image.
	OpCounter  sync.Map
	waitPeriod time.Duration
}

func (im *ImageMonitor) AddImageFuseOperationCount(operation string, image digest.Digest, count int32) {
	cm.AddImageOperationCount(operation, image, count)
}

// InitOpCounter constructs a FuseOperationCounter for an image with digest imgDigest.
// It should be started in different goroutine so that it doesn't block the current goroutine.
func (im *ImageMonitor) InitOpCounter(ctx context.Context, imgDigest digest.Digest) {
	f := &FuseOperationCounter{
		opCounts: make(map[string]*int32),
	}
	for _, m := range fuse.OpsList {
		f.opCounts[m] = new(int32)
	}
	im.OpCounter.Store(imgDigest, f)
	im.runOpCounter(ctx, imgDigest, f)

}

// runOpCounter waits for a waitPeriod to pass before emitting a log and metric for each
// operation in FuseOpsList.
func (im *ImageMonitor) runOpCounter(ctx context.Context, imgDigest digest.Digest, f *FuseOperationCounter) {
	select {
	case <-ctx.Done():
		return
	case <-time.After(im.waitPeriod):
		for op, opCount := range f.opCounts {
			// We want both an aggregated metric (e.g. p90) and an image specific metric so that we can compare
			// how a specific image is behaving to a larger dataset. When the image cardinality is small,
			// we can just include the image digest as a label on the metric itself, however, when the cardinality
			// is large, this can be very expensive. Here we give consumers options by emitting both logs and
			// metrics. A low cardinality use case can rely on metrics. A high cardinality use case can
			// aggregate the metrics across all images, but still get the per-image info via logs.
			count := atomic.LoadInt32(opCount)
			im.AddImageFuseOperationCount(op, imgDigest, count)
			log.G(ctx).Infof("fuse operation count for image %s: %s = %d", imgDigest, op, count)
		}
	}
}

// FuseOperationCounter collects number of invocations of the various FUSE
// implementations and emits them as metrics.
type FuseOperationCounter struct {
	opCounts map[string]*int32
}

// Inc atomically increase the count of FUSE operation op.
// Noop if op is not in FuseOpsList.
func (f *FuseOperationCounter) Inc(op string) {
	opCount, ok := f.opCounts[op]
	if !ok {
		return
	}
	atomic.AddInt32(opCount, 1)
}
