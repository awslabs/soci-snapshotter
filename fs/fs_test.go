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
	"github.com/awslabs/soci-snapshotter/idtools"
	"github.com/containerd/containerd/v2/core/remotes/docker"
	"github.com/containerd/containerd/v2/pkg/reference"
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

// TestCheckRefreshesOnReferenceChange verifies that when the snapshot labels
// name a reference other than the one the connection was resolved from, the
// connection is refreshed from the labels before the old one is probed, and
// that nothing changes while the labels still name the resolved reference.
func TestCheckRefreshesOnReferenceChange(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	const (
		oldRef = "registry.example/image-a:v1"
		newRef = "registry.example/image-b:v1"
		layerD = "sha256:deadbeaf000000000000000000000000000000000000000000000000deadbeaf"
	)
	labelsFor := func(ref string) map[string]string {
		return map[string]string{
			"containerd.io/snapshot/cri.image-ref":    ref,
			"containerd.io/snapshot/cri.layer-digest": layerD,
		}
	}

	bl := &breakableLayer{success: true, refspec: oldRef}
	fs := &filesystem{
		layer: map[string]layer.Layer{
			"test": bl,
		},
		getSources: source.FromDefaultLabels(func(imgRefSpec reference.Spec) (hosts []docker.RegistryHost, _ error) {
			return docker.ConfigureDefaultRegistries(docker.WithPlainHTTP(docker.MatchLocalhost))(imgRefSpec.Hostname())
		}),
	}

	// Same reference: the connection is probed, nothing is refreshed.
	if err := fs.Check(ctx, "test", labelsFor(oldRef)); err != nil {
		t.Fatalf("check with unchanged reference failed: %v", err)
	}
	if bl.checks != 1 || len(bl.refreshed) != 0 {
		t.Fatalf("unchanged reference: checks=%d refreshed=%v; wanted 1 check and no refresh", bl.checks, bl.refreshed)
	}

	// New reference: refreshed from the labels without probing the old connection.
	if err := fs.Check(ctx, "test", labelsFor(newRef)); err != nil {
		t.Fatalf("check with changed reference failed: %v", err)
	}
	if bl.checks != 1 || len(bl.refreshed) != 1 || bl.refreshed[0] != newRef {
		t.Fatalf("changed reference: checks=%d refreshed=%v; wanted no new check and one refresh to %q", bl.checks, bl.refreshed, newRef)
	}
	if bl.Refspec().String() != newRef {
		t.Fatalf("connection reference = %q; wanted %q", bl.Refspec().String(), newRef)
	}

	// Refreshing to the new reference fails: falls back to the regular check,
	// which then fails too, and the error is reported.
	bl.success = false
	if err := fs.Check(ctx, "test", labelsFor(oldRef)); err == nil {
		t.Fatalf("check succeeded; wanted to fail when neither refresh nor check works")
	}
	if bl.checks != 2 {
		t.Fatalf("failed refresh: checks=%d; wanted the regular check to run", bl.checks)
	}
}

type breakableLayer struct {
	success bool
	// refspec is the reference the connection was resolved from; refreshed
	// contains the references passed to Refresh, and checks the calls to Check.
	refspec   string
	refreshed []string
	checks    int
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
	l.checks++
	if !l.success {
		return fmt.Errorf("failed")
	}
	return nil
}
func (l *breakableLayer) Refresh(ctx context.Context, hosts []docker.RegistryHost, refspec reference.Spec, desc ocispec.Descriptor) error {
	l.refreshed = append(l.refreshed, refspec.String())
	if !l.success {
		return fmt.Errorf("failed")
	}
	l.refspec = refspec.String()
	return nil
}
func (l *breakableLayer) Refspec() reference.Spec {
	spec, _ := reference.Parse(l.refspec)
	return spec
}
func (l *breakableLayer) Done() {}
