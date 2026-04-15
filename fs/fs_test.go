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

/*
   Copyright The containerd Authors.

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

/*
   Copyright 2019 The Go Authors. All rights reserved.
   Use of this source code is governed by a BSD-style
   license that can be found in the NOTICE.md file.
*/

package fs

import (
	"context"
	"fmt"
	"testing"

	"github.com/awslabs/soci-snapshotter/fs/layer"
	"github.com/awslabs/soci-snapshotter/fs/remote"
	"github.com/awslabs/soci-snapshotter/fs/source"
	ctdsnapshotters "github.com/containerd/containerd/pkg/snapshotters"
	"github.com/awslabs/soci-snapshotter/idtools"
	"github.com/containerd/containerd/reference"
	"github.com/containerd/containerd/remotes/docker"
	fusefs "github.com/hanwen/go-fuse/v2/fs"
	digest "github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

func TestCheck(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	bl := &breakableLayer{}
	fs := &filesystem{
		layer: map[string]layer.Layer{
			"test": bl,
		},
		getSources: source.FromDefaultLabels(func(imgRefSpec reference.Spec) (hosts []docker.RegistryHost, _ error) {
			return docker.ConfigureDefaultRegistries(docker.WithPlainHTTP(docker.MatchLocalhost))(imgRefSpec.Hostname())
		}),
	}
	bl.success = true
	if err := fs.Check(ctx, "test", nil); err != nil {
		t.Errorf("connection failed; wanted to succeed: %v", err)
	}

	bl.success = false
	if err := fs.Check(ctx, "test", nil); err == nil {
		t.Errorf("connection succeeded; wanted to fail")
	}
}

func TestCheckSucceedsWithLayerRef(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	refreshCount := 0

	bl := &breakableLayer{}
	bl.success = false

	oldRef := reference.Spec{Locator: "registry.example.com/repo/image", Object: "old-tag"}
	newRef := reference.Spec{Locator: "registry.example.com/repo/image", Object: "new-tag"}

	fs := &filesystem{
		layer: map[string]layer.Layer{
			"test": bl,
		},
		getSources: func(labels map[string]string) ([]source.Source, error) {
			refreshCount++
			refStr := labels[ctdsnapshotters.TargetRefLabel]
			ref, _ := reference.Parse(refStr)
			if ref.String() == newRef.String() {
				bl.success = true
			}
			return []source.Source{
				{Name: ref},
			}, nil
		},
	}

	// Register the new ref on this layer (simulates Mount for shared layers).
	fs.addLayerRef("test", newRef.String())

	labels := map[string]string{
		ctdsnapshotters.TargetRefLabel: oldRef.String(),
	}

	if err := fs.Check(ctx, "test", labels); err != nil {
		t.Errorf("connection failed with layer ref; wanted to succeed: %v", err)
	}
	// refreshLayer: 1. old ref (fails), 2. new ref (layer-scoped, succeeds)
	if refreshCount != 2 {
		t.Errorf("expected 2 refreshLayer calls, got %d", refreshCount)
	}
}

func TestCheckSucceedsWithHostRefsPool(t *testing.T) {
	// Simulates the Mounts path with no per-layer refs,
	// but getHostRefs returns alternative refs from the CRI credential pool.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	refreshCount := 0

	bl := &breakableLayer{}
	bl.success = false

	oldRef := reference.Spec{Locator: "registry.example.com/repo/image", Object: "old-tag"}
	freshRef := reference.Spec{Locator: "registry.example.com/repo/image", Object: "fresh-tag"}

	fs := &filesystem{
		layer: map[string]layer.Layer{
			"test": bl,
		},
		getSources: func(labels map[string]string) ([]source.Source, error) {
			refreshCount++
			refStr := labels[ctdsnapshotters.TargetRefLabel]
			ref, _ := reference.Parse(refStr)
			if ref.String() == freshRef.String() {
				bl.success = true
			}
			return []source.Source{
				{Name: ref},
			}, nil
		},
		getHostRefs: func(host string) []string {
			return []string{freshRef.String()}
		},
	}

	// No per-layer refs. Only host-level pool.
	labels := map[string]string{
		ctdsnapshotters.TargetRefLabel: oldRef.String(),
	}

	if err := fs.Check(ctx, "test", labels); err != nil {
		t.Errorf("connection failed with host refs pool; wanted to succeed: %v", err)
	}
	// refreshLayer: 1. old ref (fails), 2. fresh ref from pool (succeeds)
	if refreshCount < 2 {
		t.Errorf("expected at least 2 refreshLayer calls, got %d", refreshCount)
	}
}

func TestCheckRemovesInvalidCredOnAuthError(t *testing.T) {
	// Verifies that when a layer-scoped ref gets a 401, RemoveLatestAuth is called.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var removedHost, removedRef string

	oldRef := reference.Spec{Locator: "registry.example.com/repo/image", Object: "old-tag"}
	newRef := reference.Spec{Locator: "registry.example.com/repo/image", Object: "new-tag"}

	bl := &controlledLayer{
		checkResult: fmt.Errorf("stale"),
		refreshFunc: func(ref string) error {
			// All refs fail with auth error
			return fmt.Errorf("%w on redirect 401", remote.ErrUnexpectedStatusCode)
		},
	}

	fs := &filesystem{
		layer: map[string]layer.Layer{"test": bl},
		getSources: func(labels map[string]string) ([]source.Source, error) {
			refStr := labels[ctdsnapshotters.TargetRefLabel]
			ref, _ := reference.Parse(refStr)
			return []source.Source{{Name: ref}}, nil
		},
		removeLatestAuth: func(host, ref string) {
			removedHost = host
			removedRef = ref
		},
	}

	fs.addLayerRef("test", newRef.String())

	labels := map[string]string{
		ctdsnapshotters.TargetRefLabel: oldRef.String(),
	}

	_ = fs.Check(ctx, "test", labels)

	if removedRef != newRef.String() {
		t.Errorf("RemoveLatestAuth called with ref %q, want %q", removedRef, newRef.String())
	}
	if removedHost != "registry.example.com" {
		t.Errorf("RemoveLatestAuth called with host %q, want registry.example.com", removedHost)
	}
}

func TestCheckTier2DoesNotRemoveAuth(t *testing.T) {
	// Verifies that Tier 2 (host-level refs) does NOT call RemoveLatestAuth,
	// because 401 from a host-level ref may be wrong-scope, not invalid.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	removeAuthCalled := false

	oldRef := reference.Spec{Locator: "registry.example.com/repo-A/image", Object: "old-tag"}
	hostRef := reference.Spec{Locator: "registry.example.com/repo-B/image", Object: "other-tag"}

	bl := &controlledLayer{
		checkResult: fmt.Errorf("stale"),
		refreshFunc: func(ref string) error {
			return fmt.Errorf("%w on redirect 401", remote.ErrUnexpectedStatusCode)
		},
	}

	fs := &filesystem{
		layer: map[string]layer.Layer{
			"test": bl,
		},
		getSources: func(labels map[string]string) ([]source.Source, error) {
			refStr := labels[ctdsnapshotters.TargetRefLabel]
			ref, _ := reference.Parse(refStr)
			return []source.Source{{Name: ref}}, nil
		},
		getHostRefs: func(host string) []string {
			return []string{hostRef.String()}
		},
		removeLatestAuth: func(host, ref string) {
			removeAuthCalled = true
		},
	}

	labels := map[string]string{
		ctdsnapshotters.TargetRefLabel: oldRef.String(),
	}

	_ = fs.Check(ctx, "test", labels)

	if removeAuthCalled {
		t.Error("RemoveLatestAuth should NOT be called for Tier 2 (host-level) refs")
	}
}

func TestCheckTier2RegistersRefOnSuccess(t *testing.T) {
	// When a host-level ref succeeds, it should be registered on the layer
	// so future checks use it in Tier 1.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	oldRef := reference.Spec{Locator: "registry.example.com/repo/image", Object: "old-tag"}
	freshRef := reference.Spec{Locator: "registry.example.com/repo/image", Object: "fresh-tag"}

	bl := &controlledLayer{
		checkResult: fmt.Errorf("stale"),
		refreshFunc: func(ref string) error {
			if ref == freshRef.String() {
				return nil // success
			}
			return fmt.Errorf("%w on redirect 401", remote.ErrUnexpectedStatusCode)
		},
	}

	fs := &filesystem{
		layer: map[string]layer.Layer{
			"test": bl,
		},
		getSources: func(labels map[string]string) ([]source.Source, error) {
			refStr := labels[ctdsnapshotters.TargetRefLabel]
			ref, _ := reference.Parse(refStr)
			return []source.Source{{Name: ref}}, nil
		},
		getHostRefs: func(host string) []string {
			return []string{freshRef.String()}
		},
	}

	labels := map[string]string{
		ctdsnapshotters.TargetRefLabel: oldRef.String(),
	}

	err := fs.Check(ctx, "test", labels)
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}

	// Verify fresh-tag was registered on the layer
	layerRefs := fs.getLayerRefs("test")
	found := false
	for _, ref := range layerRefs {
		if ref == freshRef.String() {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected fresh-tag to be registered on layer, got refs: %v", layerRefs)
	}
}

// controlledLayer allows per-ref control over Check/Refresh behavior.
type controlledLayer struct {
	checkResult error
	refreshFunc func(ref string) error // ref is the image ref used for refresh
}

func (l *controlledLayer) Info() layer.Info        { return layer.Info{Size: 1} }
func (l *controlledLayer) DisableXAttrs() bool     { return false }
func (l *controlledLayer) RootNode(uint32, idtools.IDMap) (fusefs.InodeEmbedder, error) {
	return nil, nil
}
func (l *controlledLayer) Verify(tocDigest digest.Digest) error { return nil }
func (l *controlledLayer) SkipVerify()                          {}
func (l *controlledLayer) ReadAt([]byte, int64, ...remote.Option) (int, error) {
	return 0, fmt.Errorf("fail")
}
func (l *controlledLayer) GetCacheRefKey() string { return "" }
func (l *controlledLayer) BackgroundFetch() error { return fmt.Errorf("fail") }
func (l *controlledLayer) Check() error           { return l.checkResult }
func (l *controlledLayer) Refresh(ctx context.Context, hosts []docker.RegistryHost, refspec reference.Spec, desc ocispec.Descriptor) error {
	if l.refreshFunc != nil {
		return l.refreshFunc(refspec.String())
	}
	return nil
}
func (l *controlledLayer) Done() {}

func TestIsAuthError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"nil error", nil, false},
		{"random error", fmt.Errorf("network timeout"), false},
		{"401 redirect", fmt.Errorf("%w on redirect 401", remote.ErrUnexpectedStatusCode), true},
		{"403 redirect", fmt.Errorf("%w on redirect 403", remote.ErrUnexpectedStatusCode), true},
		{"401 check", fmt.Errorf("%w on check: 401", remote.ErrUnexpectedStatusCode), true},
		{"403 check", fmt.Errorf("%w on check: 403", remote.ErrUnexpectedStatusCode), true},
		{"400 redirect (not auth)", fmt.Errorf("%w on redirect 400", remote.ErrUnexpectedStatusCode), false},
		{"500 redirect (not auth)", fmt.Errorf("%w on redirect 500", remote.ErrUnexpectedStatusCode), false},
		{"wrapped 401", fmt.Errorf("failed(layer:\"sha256:abc\", ref:\"test\"): %w", fmt.Errorf("%w on redirect 401", remote.ErrUnexpectedStatusCode)), true},
		{"string contains 401 but wrong error type", fmt.Errorf("on redirect 401"), false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := isAuthError(tc.err); got != tc.expected {
				t.Errorf("isAuthError(%v) = %v, want %v", tc.err, got, tc.expected)
			}
		})
	}
}

type breakableLayer struct {
	success bool
}

func (l *breakableLayer) Info() layer.Info {
	return layer.Info{
		Size: 1,
	}
}
func (l *breakableLayer) DisableXAttrs() bool { return false }
func (l *breakableLayer) RootNode(uint32, idtools.IDMap) (fusefs.InodeEmbedder, error) {
	return nil, nil
}
func (l *breakableLayer) Verify(tocDigest digest.Digest) error { return nil }
func (l *breakableLayer) SkipVerify()                          {}
func (l *breakableLayer) ReadAt([]byte, int64, ...remote.Option) (int, error) {
	return 0, fmt.Errorf("fail")
}
func (l *breakableLayer) GetCacheRefKey() string { return "" }
func (l *breakableLayer) BackgroundFetch() error { return fmt.Errorf("fail") }
func (l *breakableLayer) Check() error {
	if !l.success {
		return fmt.Errorf("failed")
	}
	return nil
}
func (l *breakableLayer) Refresh(ctx context.Context, hosts []docker.RegistryHost, refspec reference.Spec, desc ocispec.Descriptor) error {
	if !l.success {
		return fmt.Errorf("failed")
	}
	return nil
}
func (l *breakableLayer) Done() {}
