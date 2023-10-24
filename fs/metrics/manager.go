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

package metrics

import (
	"time"

	"github.com/awslabs/soci-snapshotter/fs/metrics/monitor"
)

type FuseObservabilityManager struct {
	LayerMonitor      *monitor.LayerMonitor
	ImageMonitor      *monitor.ImageMonitor
	GlobalMonitor     *monitor.GlobalMonitor
	LogFuseOperations bool
}

// NewFuseObservabilityManager returns a new FuseObservabilityManager.
// A FuseObservabilityManager manages the emission of Prometheus metrics
// related to FUSE operations. It contains monitors at an image level,
// layer level and global level. Each monitor provides an abstraction over
// metric operations.
func NewFuseObservabilityManager(logFuseOperations bool, waitP time.Duration) *FuseObservabilityManager {
	return &FuseObservabilityManager{
		LayerMonitor:      monitor.NewLayerMonitor(),
		ImageMonitor:      monitor.NewImageMonitor(waitP),
		GlobalMonitor:     monitor.NewGlobalMonitor(),
		LogFuseOperations: logFuseOperations,
	}
}
