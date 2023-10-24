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
	"time"

	cm "github.com/awslabs/soci-snapshotter/fs/metrics/common"
	"github.com/opencontainers/go-digest"
)

// NewGlobalMonitor returns a new GlobalMonitor. A GlobalMonitor encapsulates the emission of FUSE
// telemetry data at a global level.
func NewGlobalMonitor() *GlobalMonitor {
	return &GlobalMonitor{
		fuseFailureSignal: make(chan struct{}),
	}
}

type GlobalMonitor struct {
	fuseFailureSignal chan struct{}
}

// RunFuseFailureListener infinitely listens for any FUSE failure. If one
// occurs, it increments a metric and sleeps for a time duration.
func (gm *GlobalMonitor) RunFuseFailureListener(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-gm.fuseFailureSignal:
			cm.IncOperationCount(cm.FuseFailureState, digest.Digest(""))
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Minute):
			}
		}
	}
}

// FuseFailureNotify notifies the listener that a FUSE failure occurred.
// We wrap the send in a select block with a default case to ensure
// the thread does not block if no receiver is available.
func (gm *GlobalMonitor) FuseFailureNotify() {
	select {
	case gm.fuseFailureSignal <- struct{}{}:
	default:
	}
}
