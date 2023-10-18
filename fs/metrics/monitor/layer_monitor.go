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
	"time"

	"github.com/awslabs/soci-snapshotter/fs/layer/fuse"
	cm "github.com/awslabs/soci-snapshotter/fs/metrics/common"
	"github.com/opencontainers/go-digest"
)

var fuseOpFailureMetrics = map[string]string{
	fuse.OpGetattr:         cm.FuseNodeGetattrFailureCount,
	fuse.OpListxattr:       cm.FuseNodeListxattrFailureCount,
	fuse.OpLookup:          cm.FuseNodeLookupFailureCount,
	fuse.OpOpen:            cm.FuseNodeOpenFailureCount,
	fuse.OpReaddir:         cm.FuseNodeReaddirFailureCount,
	fuse.OpFileRead:        cm.FuseFileReadFailureCount,
	fuse.OpFileGetattr:     cm.FuseFileGetattrFailureCount,
	fuse.OpWhiteoutGetattr: cm.FuseWhiteoutGetattrFailureCount,
}

// NewLayerMonitor returns a new LayerMonitor. A LayerMonitor encapsulates the emission of FUSE
// telemetry data at a layer level.
func NewLayerMonitor() *LayerMonitor {
	return &LayerMonitor{}
}

type LayerMonitor struct {
}

func (lm *LayerMonitor) MeasureFuseLatencyInMilliseconds(operation string, layer digest.Digest, start time.Time) {
	cm.MeasureLatencyInMilliseconds(operation, layer, start)
}

func (lm *LayerMonitor) MeasureFuseLatencyInMicroseconds(operation string, layer digest.Digest, start time.Time) {
	cm.MeasureLatencyInMicroseconds(operation, layer, start)
}

func (lm *LayerMonitor) IncFuseOperationCount(operation string, layer digest.Digest) {
	cm.IncOperationCount(operation, layer)
}

func (lm *LayerMonitor) IncFuseFailureOperationCount(operation string, layer digest.Digest) {
	label, ok := fuseOpFailureMetrics[operation]
	if !ok {
		label = cm.FuseUnknownFailureCount
	}
	cm.IncOperationCount(label, layer)
}
