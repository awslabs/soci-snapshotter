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
	"sync/atomic"
	"testing"

	"github.com/awslabs/soci-snapshotter/config"
	"github.com/containerd/containerd/reference"
)

func TestRegistryHostsCachesAndReturnsHosts(t *testing.T) {
	cred := func(_ reference.Spec, _ string) (string, string, error) {
		return "user", "pass1", nil
	}
	rm := NewRegistryManager(config.RetryableHTTPClientConfig{}, config.ResolverConfig{}, []Credential{cred})
	hosts := rm.AsRegistryHosts()
	ref, _ := reference.Parse("docker.io/library/alpine:latest")

	h1, err := hosts(ref)
	if err != nil {
		t.Fatal(err)
	}
	h2, err := hosts(ref)
	if err != nil {
		t.Fatal(err)
	}
	// Cached — same slice returned.
	if &h1[0] != &h2[0] {
		t.Fatal("expected cached hosts")
	}
}

func TestInvalidateRegistryHostsForcesFreshHosts(t *testing.T) {
	var version atomic.Int32
	version.Store(1)
	cred := func(_ reference.Spec, _ string) (string, string, error) {
		return "user", string(rune('0' + version.Load())), nil
	}
	rm := NewRegistryManager(config.RetryableHTTPClientConfig{}, config.ResolverConfig{}, []Credential{cred})
	hosts := rm.AsRegistryHosts()
	ref, _ := reference.Parse("docker.io/library/alpine:latest")

	h1, err := hosts(ref)
	if err != nil {
		t.Fatal(err)
	}

	// Rotate credentials.
	version.Store(2)

	// Without invalidation — still returns cached (stale) hosts.
	h2, err := hosts(ref)
	if err != nil {
		t.Fatal(err)
	}
	if &h1[0] != &h2[0] {
		t.Fatal("expected stale cached hosts without invalidation")
	}

	// Invalidate.
	rm.InvalidateRegistryHosts(ref.String())

	// After invalidation — returns fresh hosts (new AuthClient).
	h3, err := hosts(ref)
	if err != nil {
		t.Fatal(err)
	}
	if &h1[0] == &h3[0] {
		t.Fatal("expected fresh hosts after invalidation, got cached")
	}
}

func TestInvalidateIsNoOpForUnknownRef(t *testing.T) {
	cred := func(_ reference.Spec, _ string) (string, string, error) {
		return "user", "pass", nil
	}
	rm := NewRegistryManager(config.RetryableHTTPClientConfig{}, config.ResolverConfig{}, []Credential{cred})

	// Invalidating a ref that was never cached should not panic.
	rm.InvalidateRegistryHosts("docker.io/library/nonexistent:latest")
}
