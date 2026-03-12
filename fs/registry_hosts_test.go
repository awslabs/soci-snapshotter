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

package fs

import (
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/containerd/containerd/remotes/docker"
)

func TestBuildRegistryHostRefsRewritesLocatorsInOrder(t *testing.T) {
	hosts := []docker.RegistryHost{
		{
			Host:   "first.example",
			Path:   "/v2",
			Client: &http.Client{},
		},
		{
			Host:   "second.example",
			Path:   "/v2/team-a",
			Client: &http.Client{},
		},
	}

	hostRefs, err := buildRegistryHostRefs("docker.io/library/nginx:latest", hosts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	gotLocators := []string{
		hostRefs[0].refspec.Locator,
		hostRefs[1].refspec.Locator,
	}
	wantLocators := []string{
		"first.example/library/nginx",
		"second.example/team-a/library/nginx",
	}
	if !slices.Equal(gotLocators, wantLocators) {
		t.Fatalf("unexpected locators, got %v, want %v", gotLocators, wantLocators)
	}
}

func TestBuildRegistryHostRefsReturnsUsableHostsAndSkippedHostErrors(t *testing.T) {
	hosts := []docker.RegistryHost{
		{
			Host: "missing-client.example",
			Path: "/v2",
		},
		{
			Host:   "bad-path.example",
			Path:   "/custom/path",
			Client: &http.Client{},
		},
		{
			Host:   "failing.example",
			Path:   "/v2",
			Client: &http.Client{},
		},
	}

	hostRefs, err := buildRegistryHostRefs("docker.io/library/nginx:latest", hosts)
	if err == nil {
		t.Fatal("expected skipped-host error but got nil")
	}
	if len(hostRefs) != 1 {
		t.Fatalf("expected one usable host, got %d", len(hostRefs))
	}
	if got, want := hostRefs[0].host.Host, "failing.example"; got != want {
		t.Fatalf("unexpected host, got %q, want %q", got, want)
	}

	for _, want := range []string{
		`registry host "missing-client.example" has no http client`,
		`registry host "bad-path.example": unsupported registry host path`,
	} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("expected error to contain %q, got %v", want, err)
		}
	}
}

func TestBuildRegistryHostRefsReturnsErrorWhenNoHostsAreUsable(t *testing.T) {
	hosts := []docker.RegistryHost{
		{
			Host: "missing-client.example",
			Path: "/v2",
		},
		{
			Host:   "bad-path.example",
			Path:   "/custom/path",
			Client: &http.Client{},
		},
	}

	hostRefs, err := buildRegistryHostRefs("docker.io/library/nginx:latest", hosts)
	if err == nil {
		t.Fatal("expected error but got nil")
	}
	if len(hostRefs) != 0 {
		t.Fatalf("expected no usable hosts, got %d", len(hostRefs))
	}
	for _, want := range []string{
		`registry host "missing-client.example" has no http client`,
		`registry host "bad-path.example": unsupported registry host path`,
	} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("expected error to contain %q, got %v", want, err)
		}
	}
}

func TestResetMountpointRecreatesEmptyDirectory(t *testing.T) {
	mountpoint := filepath.Join(t.TempDir(), "snapshots", "target")
	if err := os.MkdirAll(mountpoint, 0700); err != nil {
		t.Fatalf("failed to create mountpoint: %v", err)
	}
	if err := os.Chmod(mountpoint, 0751); err != nil {
		t.Fatalf("failed to chmod mountpoint: %v", err)
	}
	if err := os.Mkdir(filepath.Join(mountpoint, "stale-dir"), 0700); err != nil {
		t.Fatalf("failed to create stale directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(mountpoint, "stale-dir", "nested"), []byte("data"), 0600); err != nil {
		t.Fatalf("failed to seed nested mountpoint content: %v", err)
	}
	if err := os.WriteFile(filepath.Join(mountpoint, "stale"), []byte("data"), 0600); err != nil {
		t.Fatalf("failed to seed mountpoint: %v", err)
	}

	if err := resetMountpoint(mountpoint); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	info, err := os.Stat(mountpoint)
	if err != nil {
		t.Fatalf("expected mountpoint to exist: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("expected mountpoint to be a directory")
	}
	if got, want := info.Mode().Perm(), os.FileMode(0751); got != want {
		t.Fatalf("expected mountpoint mode %v, got %v", want, got)
	}
	entries, err := os.ReadDir(mountpoint)
	if err != nil {
		t.Fatalf("failed to read mountpoint: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected empty mountpoint, found %d entries", len(entries))
	}
}

func TestResetMountpointPreservesDirectory(t *testing.T) {
	mountpoint := filepath.Join(t.TempDir(), "snapshots", "target")
	if err := os.MkdirAll(mountpoint, 0755); err != nil {
		t.Fatalf("failed to create mountpoint: %v", err)
	}

	before, err := os.Stat(mountpoint)
	if err != nil {
		t.Fatalf("failed to stat mountpoint: %v", err)
	}

	if err := resetMountpoint(mountpoint); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	after, err := os.Stat(mountpoint)
	if err != nil {
		t.Fatalf("expected mountpoint to exist: %v", err)
	}

	if !os.SameFile(before, after) {
		t.Fatal("expected mountpoint directory itself to be preserved")
	}
}

func TestResetMountpointRemovesReadOnlyDirectories(t *testing.T) {
	mountpoint := filepath.Join(t.TempDir(), "snapshots", "target")
	if err := os.MkdirAll(mountpoint, 0755); err != nil {
		t.Fatalf("failed to create mountpoint: %v", err)
	}

	readonlyDir := filepath.Join(mountpoint, "readonly")
	if err := os.Mkdir(readonlyDir, 0755); err != nil {
		t.Fatalf("failed to create readonly dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(readonlyDir, "nested"), []byte("data"), 0600); err != nil {
		t.Fatalf("failed to seed readonly dir: %v", err)
	}
	if err := os.Chmod(readonlyDir, 0555); err != nil {
		t.Fatalf("failed to chmod readonly dir: %v", err)
	}

	if err := resetMountpoint(mountpoint); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	entries, err := os.ReadDir(mountpoint)
	if err != nil {
		t.Fatalf("failed to read mountpoint: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected empty mountpoint, found %d entries", len(entries))
	}
}
