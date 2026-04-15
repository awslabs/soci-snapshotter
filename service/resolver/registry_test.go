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
	"testing"

	"github.com/awslabs/soci-snapshotter/config"
	"github.com/containerd/containerd/reference"
	"github.com/containerd/containerd/remotes/docker"
)

func TestAsRegistryHosts(t *testing.T) {
	rm := NewRegistryManager(config.RetryableHTTPClientConfig{}, config.ResolverConfig{}, nil)
	hosts := rm.AsRegistryHosts()

	ref := reference.Spec{Locator: "docker.io/library/alpine", Object: "latest"}

	result, err := hosts(ref)
	if err != nil {
		t.Fatalf("failed to get registry hosts: %v", err)
	}
	if len(result) == 0 {
		t.Fatal("expected at least one registry host")
	}
	if result[0].Client == nil {
		t.Fatal("expected non-nil Client")
	}
	if result[0].Host == "" {
		t.Fatal("expected non-empty Host")
	}
	if result[0].Scheme != "https" {
		t.Fatalf("expected https scheme, got %q", result[0].Scheme)
	}
	if result[0].Capabilities != docker.HostCapabilityPull|docker.HostCapabilityResolve {
		t.Fatalf("unexpected capabilities: %v", result[0].Capabilities)
	}

	// Repeated call returns cached result.
	cached, err := hosts(ref)
	if err != nil {
		t.Fatalf("cached call failed: %v", err)
	}
	if result[0].Client != cached[0].Client {
		t.Fatal("expected same Client from cache")
	}
}

