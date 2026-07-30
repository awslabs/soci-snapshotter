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

package resolver

import (
	"context"
	"testing"
	"time"

	"github.com/awslabs/soci-snapshotter/config"
	"go.uber.org/goleak"
)

// TestNoGoroutinesAreLeakedWhenRegistryManagerContextIsCancelled asserts that the
// periodic cache sweeper is tied to the lifetime of the snapshotter service: once
// the context passed to NewRegistryManager is cancelled, the sweeper goroutine
// exits instead of ticking for the life of the process.
func TestNoGoroutinesAreLeakedWhenRegistryManagerContextIsCancelled(t *testing.T) {
	// Ignore goroutines that already existed, so this only reports goroutines
	// started by this test rather than pre-existing ones from other tests.
	ignoreExisting := goleak.IgnoreCurrent()
	defer goleak.VerifyNone(t, ignoreExisting)

	testCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// A positive TTL is what starts the sweeper. Keep it long enough that the
	// goroutine cannot exit simply because the ticker fired.
	rm := NewRegistryManager(testCtx, config.RetryableHTTPClientConfig{},
		config.ResolverConfig{AuthClientTTLSec: 3600}, nil)
	if rm.authClientTTL <= 0 {
		t.Fatalf("expected a positive authClientTTL so the sweeper starts, got %v", rm.authClientTTL)
	}

	// Cancelling the service context must stop the sweeper. goleak (deferred
	// above) retries for a short while, so it tolerates the scheduling delay
	// between cancel() and the goroutine actually returning.
	cancel()
}

// TestRegistryManagerSweeperIsNotStartedWithoutTTL asserts that no sweeper
// goroutine is started when expiry is disabled (TTL <= 0), in which case cache
// entries never expire and there is nothing to sweep.
func TestRegistryManagerSweeperIsNotStartedWithoutTTL(t *testing.T) {
	ignoreExisting := goleak.IgnoreCurrent()
	defer goleak.VerifyNone(t, ignoreExisting)

	// Deliberately not cancelled: with expiry disabled there should be no
	// goroutine depending on the context to shut down.
	testCtx := context.Background()

	for _, ttlSec := range []int64{0, -1} {
		rm := NewRegistryManager(testCtx, config.RetryableHTTPClientConfig{},
			config.ResolverConfig{AuthClientTTLSec: ttlSec}, nil)
		if rm.authClientTTL > 0 {
			t.Fatalf("auth_client_ttl_sec=%d: expected non-positive authClientTTL, got %v", ttlSec, rm.authClientTTL)
		}
		if rm.expired(&expiringEntry{createdAt: time.Now().Add(-24 * time.Hour)}) {
			t.Fatalf("auth_client_ttl_sec=%d: entries must never expire when expiry is disabled", ttlSec)
		}
	}
}
