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

package cri

import (
	"fmt"
	"testing"
	"time"

	"github.com/containerd/containerd/reference"
	runtime "k8s.io/cri-api/pkg/apis/runtime/v1"
)

func newTestService() *instrumentedService {
	return &instrumentedService{
		hostCreds: make(map[string]map[string][]*pullRecord),
	}
}

func addCred(svc *instrumentedService, host, ref, username, password string) {
	host = normalizeHost(host)
	if svc.hostCreds[host] == nil {
		svc.hostCreds[host] = make(map[string][]*pullRecord)
	}
	svc.hostCreds[host][ref] = append(svc.hostCreds[host][ref], &pullRecord{
		auth: &runtime.AuthConfig{Username: username, Password: password},
		time: time.Now(),
	})
}

func TestCredentialsExactRefMatch(t *testing.T) {
	svc := newTestService()
	ref := "registry.example.com/repo/image:tag-A"
	addCred(svc, "registry.example.com", ref, "user-A", "pass-A")

	refspec, _ := reference.Parse(ref)
	username, password, err := svc.credentials(refspec, "registry.example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if username != "user-A" || password != "pass-A" {
		t.Errorf("got (%q, %q), want (user-A, pass-A)", username, password)
	}
}

func TestCredentialsReturnsLatest(t *testing.T) {
	svc := newTestService()
	ref := "registry.example.com/repo/image:tag-A"
	addCred(svc, "registry.example.com", ref, "user-old", "pass-old")
	time.Sleep(time.Millisecond) // ensure different timestamps
	addCred(svc, "registry.example.com", ref, "user-new", "pass-new")

	refspec, _ := reference.Parse(ref)
	username, _, err := svc.credentials(refspec, "registry.example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if username != "user-new" {
		t.Errorf("got %q, want user-new (latest)", username)
	}
}

func TestCredentialsMissReturnsEmpty(t *testing.T) {
	svc := newTestService()
	addCred(svc, "registry.example.com", "registry.example.com/repo:tag-A", "user", "pass")

	refspec, _ := reference.Parse("registry.example.com/repo:tag-UNKNOWN")
	username, password, err := svc.credentials(refspec, "registry.example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if username != "" || password != "" {
		t.Errorf("expected empty creds for unknown ref, got (%q, %q)", username, password)
	}
}

func TestHostRefsOrderedByLatest(t *testing.T) {
	svc := newTestService()
	addCred(svc, "registry.example.com", "registry.example.com/repo:tag-A", "u1", "p1")
	time.Sleep(time.Millisecond)
	addCred(svc, "registry.example.com", "registry.example.com/repo:tag-B", "u2", "p2")
	time.Sleep(time.Millisecond)
	addCred(svc, "registry.example.com", "registry.example.com/repo:tag-C", "u3", "p3")

	refs := svc.HostRefs("registry.example.com")
	if len(refs) != 3 {
		t.Fatalf("expected 3 refs, got %d", len(refs))
	}
	// Should be latest first
	if refs[0] != "registry.example.com/repo:tag-C" {
		t.Errorf("expected tag-C first (latest), got %q", refs[0])
	}
	if refs[2] != "registry.example.com/repo:tag-A" {
		t.Errorf("expected tag-A last (oldest), got %q", refs[2])
	}
}

func TestHostRefsEmptyForUnknownHost(t *testing.T) {
	svc := newTestService()
	refs := svc.HostRefs("unknown.host.com")
	if refs != nil {
		t.Errorf("expected nil for unknown host, got %v", refs)
	}
}

func TestRemoveLatestAuthRemovesNewest(t *testing.T) {
	svc := newTestService()
	ref := "registry.example.com/repo:tag-A"
	addCred(svc, "registry.example.com", ref, "user-old", "pass-old")
	time.Sleep(time.Millisecond)
	addCred(svc, "registry.example.com", ref, "user-new", "pass-new")

	// Remove latest — should remove user-new
	svc.RemoveLatestAuth("registry.example.com", ref)

	refspec, _ := reference.Parse(ref)
	username, _, _ := svc.credentials(refspec, "registry.example.com")
	if username != "user-old" {
		t.Errorf("after RemoveLatestAuth, expected user-old, got %q", username)
	}
}

func TestRemoveLatestAuthDeletesRefWhenEmpty(t *testing.T) {
	svc := newTestService()
	ref := "registry.example.com/repo:tag-A"
	addCred(svc, "registry.example.com", ref, "user", "pass")

	svc.RemoveLatestAuth("registry.example.com", ref)

	refspec, _ := reference.Parse(ref)
	username, _, _ := svc.credentials(refspec, "registry.example.com")
	if username != "" {
		t.Errorf("expected empty creds after removing only entry, got %q", username)
	}

	refs := svc.HostRefs("registry.example.com")
	if len(refs) != 0 {
		t.Errorf("expected empty HostRefs after removing only ref, got %v", refs)
	}
}

func TestRemoveLatestAuthNoopForUnknown(t *testing.T) {
	svc := newTestService()
	// Should not panic
	svc.RemoveLatestAuth("unknown.host.com", "unknown/ref:tag")
	svc.RemoveLatestAuth("registry.example.com", "unknown/ref:tag")
}

func TestMaxPullRecordsPerRefCap(t *testing.T) {
	svc := newTestService()
	host := "registry.example.com"
	ref := "registry.example.com/repo:tag-A"
	for i := 0; i < maxPullRecordsPerRef+3; i++ {
		addCred(svc, host, ref, "user", "pass")
		// Simulate the cap logic from PullImage
		h := normalizeHost(host)
		if len(svc.hostCreds[h][ref]) > maxPullRecordsPerRef {
			svc.hostCreds[h][ref] = svc.hostCreds[h][ref][len(svc.hostCreds[h][ref])-maxPullRecordsPerRef:]
		}
	}

	h := normalizeHost(host)
	if len(svc.hostCreds[h][ref]) != maxPullRecordsPerRef {
		t.Errorf("expected %d records, got %d", maxPullRecordsPerRef, len(svc.hostCreds[h][ref]))
	}
}

func TestMaxRefsPerHostEviction(t *testing.T) {
	svc := newTestService()
	host := "registry.example.com"
	h := normalizeHost(host)

	// Add maxRefsPerHost + 1 refs
	for i := 0; i <= maxRefsPerHost; i++ {
		ref := fmt.Sprintf("registry.example.com/repo:tag-%d", i)
		addCred(svc, host, ref, "user", "pass")
		time.Sleep(time.Millisecond)
	}

	// Simulate the eviction logic from PullImage
	if len(svc.hostCreds[h]) > maxRefsPerHost {
		var oldestRef string
		var oldestTime time.Time
		for r, records := range svc.hostCreds[h] {
			if len(records) > 0 {
				t := records[len(records)-1].time
				if oldestRef == "" || t.Before(oldestTime) {
					oldestRef = r
					oldestTime = t
				}
			}
		}
		if oldestRef != "" {
			delete(svc.hostCreds[h], oldestRef)
		}
	}

	if len(svc.hostCreds[h]) != maxRefsPerHost {
		t.Errorf("expected %d refs after eviction, got %d", maxRefsPerHost, len(svc.hostCreds[h]))
	}

	// The oldest ref (tag-0) should have been evicted
	if _, ok := svc.hostCreds[h]["registry.example.com/repo:tag-0"]; ok {
		t.Error("expected tag-0 (oldest) to be evicted")
	}
	// The newest ref should still exist
	lastRef := fmt.Sprintf("registry.example.com/repo:tag-%d", maxRefsPerHost)
	if _, ok := svc.hostCreds[h][lastRef]; !ok {
		t.Errorf("expected %s (newest) to be present", lastRef)
	}
}

func TestDockerHubNormalization(t *testing.T) {
	svc := newTestService()
	ref := "docker.io/library/alpine:latest"
	addCred(svc, "docker.io", ref, "user", "pass")

	// credentials normalizes host — should find creds stored under docker.io
	refspec, _ := reference.Parse(ref)

	// Query with different Docker Hub aliases
	for _, host := range []string{"docker.io", "registry-1.docker.io", "index.docker.io"} {
		username, _, err := svc.credentials(refspec, host)
		if err != nil {
			t.Fatalf("unexpected error for host %q: %v", host, err)
		}
		if username != "user" {
			t.Errorf("host %q: expected user, got %q", host, username)
		}
	}

	// HostRefs should work with all aliases
	for _, host := range []string{"docker.io", "registry-1.docker.io", "index.docker.io"} {
		refs := svc.HostRefs(host)
		if len(refs) != 1 {
			t.Errorf("host %q: expected 1 ref, got %d", host, len(refs))
		}
	}
}

func TestRemoveLatestAuthWithDockerHub(t *testing.T) {
	svc := newTestService()
	ref := "docker.io/library/alpine:latest"
	addCred(svc, "docker.io", ref, "user", "pass")

	// Remove using a different alias
	svc.RemoveLatestAuth("registry-1.docker.io", ref)

	refspec, _ := reference.Parse(ref)
	username, _, _ := svc.credentials(refspec, "docker.io")
	if username != "" {
		t.Errorf("expected empty after RemoveLatestAuth with alias, got %q", username)
	}
}
