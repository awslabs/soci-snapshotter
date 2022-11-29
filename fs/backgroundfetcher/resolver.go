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

package backgroundfetcher

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/awslabs/soci-snapshotter/compression"
	sm "github.com/awslabs/soci-snapshotter/fs/span-manager"
	"github.com/containerd/containerd/log"
	"github.com/opencontainers/go-digest"
	"github.com/sirupsen/logrus"
)

type Resolver interface {
	// Resolve fetches and caches the next span. Returns true if there is still more data to be fetched.
	// Returns false otherwise.
	Resolve(context.Context) (bool, error)

	// Closes the resolver.
	Close() error

	// Checks whether the resolver is closed or not.
	Closed() bool
}

type base struct {
	*sm.SpanManager
	layerDigest digest.Digest
	closed      bool
	closedMu    sync.Mutex
}

func (b *base) Close() error {
	b.closedMu.Lock()
	defer b.closedMu.Unlock()
	b.closed = true
	return nil
}

func (b *base) Closed() bool {
	b.closedMu.Lock()
	defer b.closedMu.Unlock()
	return b.closed
}

// A sequentialLayerResolver background fetches spans sequentially, starting from span 0.
type sequentialLayerResolver struct {
	*base
	nextSpanFetchID compression.SpanID
}

func NewSequentialResolver(layerDigest digest.Digest, spanManager *sm.SpanManager) Resolver {
	return &sequentialLayerResolver{
		base: &base{
			SpanManager: spanManager,
			layerDigest: layerDigest,
		},
	}
}

func (lr *sequentialLayerResolver) Resolve(ctx context.Context) (bool, error) {
	log.G(ctx).WithFields(logrus.Fields{
		"layer":  lr.layerDigest,
		"spanId": lr.nextSpanFetchID,
	}).Debug("fetching span")

	err := lr.FetchSingleSpan(lr.nextSpanFetchID)
	if err == nil {
		lr.nextSpanFetchID++
		return true, nil
	}
	if errors.Is(err, sm.ErrExceedMaxSpan) {
		return false, nil
	}

	return false, fmt.Errorf("error trying to fetch span with spanId = %d from layerDigest = %s: %w",
		lr.nextSpanFetchID, lr.layerDigest.String(), err)
}
