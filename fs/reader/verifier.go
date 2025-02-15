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

package reader

import (
	"errors"
	"sync"

	"github.com/containerd/log"
)

var ErrNotReader = errors.New("reader is not a *reader.reader")

// Verifier is a rate-limited reader.reader verifier.
// It will verify all reader.reader's added via `Add`
// when their spanmanager indicates that the image is fully
// downloaded. Verifier will verify exactly 1 reader.reader
// at a time.
// Similar to the BackgroundFetcher, this is a concurrency limiter
// to make sure SOCI's background processes don't compete with
// the containerized workload.
type Verifier struct {
	queue chan *reader

	closedMu sync.Mutex
	closed   bool
	closedC  chan struct{}
}

// NewVerifier creates a verifier with the specified max queue size.
func NewVerifier(maxQueueSize int) *Verifier {
	return &Verifier{
		queue:    make(chan *reader, maxQueueSize),
		closedMu: sync.Mutex{},
		closed:   false,
		closedC:  make(chan struct{}),
	}
}

// Add adds a reader.reader to the verifier's queue
// once the reader.reader's span manager finishes downloading
// the image.
//
// `r` must be a `*reader.reader` obtained via `reader.NewReader`
func (v *Verifier) Add(r Reader) error {
	switch r := r.(type) {
	case *reader:
		go func() {
			select {
			case <-r.spanManager.DownloadedC:
				v.queue <- r
			case <-v.closedC:
			case <-r.closedC:
			}
		}()
		return nil
	default:
		return ErrNotReader
	}
}

// Run runs the verifier to verify readers on the queue.
// The process will run until the verifier is closed and
// will only verify one reader at a time.
func (v *Verifier) Run() {
	for {
		select {
		case r := <-v.queue:
			if !r.isClosed() {
				l := log.L.WithField("layer", r.layerSha)
				err := r.Verify()
				if err == nil {
					l.Debug("verified reader")
				} else {
					l.WithError(err).Error("failed to verify reader")
				}

			}
		case <-v.closedC:
		}
	}
}

// Close closes the verifier and prevents new readers from queuing.
func (v *Verifier) Close() {
	v.closedMu.Lock()
	if !v.closed {
		v.closed = true
		close(v.closedC)
	}
	v.closedMu.Unlock()
}
