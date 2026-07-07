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

package source

import (
	"testing"

	"github.com/containerd/containerd/v2/core/remotes/docker"
	"github.com/containerd/containerd/v2/pkg/reference"
	ctdsnapshotters "github.com/containerd/containerd/v2/pkg/snapshotters"
)

const (
	testRef    = "example.com/repo:tag"
	testTarget = "sha256:1111111111111111111111111111111111111111111111111111111111111111"
	testNeighA = "sha256:2222222222222222222222222222222222222222222222222222222222222222"
	testNeighB = "sha256:3333333333333333333333333333333333333333333333333333333333333333"
)

func noopHosts(reference.Spec) ([]docker.RegistryHost, error) { return nil, nil }

// TestNeighboringLayersFromDigestsWithoutSizeLabel verifies that neighboring
// layers are pre-resolvable from the image.layers digest list alone, even when
// the (SOCI-wrapper-only) image.layers.size label is absent. Without this,
// pre-resolution silently did nothing on CRI pull paths.
func TestNeighboringLayersFromDigestsWithoutSizeLabel(t *testing.T) {
	labels := map[string]string{
		ctdsnapshotters.TargetRefLabel:         testRef,
		ctdsnapshotters.TargetLayerDigestLabel: testTarget,
		ctdsnapshotters.TargetImageLayersLabel: testTarget + "," + testNeighA + "," + testNeighB,
		// Note: no targetImageLayersSizeLabel.
	}

	sources, err := FromDefaultLabels(noopHosts)(labels)
	if err != nil {
		t.Fatalf("FromDefaultLabels: %v", err)
	}
	if len(sources) != 1 {
		t.Fatalf("expected 1 source, got %d", len(sources))
	}

	// Manifest layers = target + neighbors. The target is index 0.
	layers := sources[0].Manifest.Layers
	if len(layers) != 3 {
		t.Fatalf("expected 3 manifest layers (target + 2 neighbors), got %d", len(layers))
	}
	// Neighbors must exclude the target and carry the right digests.
	got := map[string]bool{}
	for _, l := range layers[1:] {
		got[l.Digest.String()] = true
		if l.Digest.String() == testTarget {
			t.Fatalf("target must not appear in neighboring layers")
		}
	}
	if !got[testNeighA] || !got[testNeighB] {
		t.Fatalf("expected neighbors %s and %s, got %v", testNeighA, testNeighB, got)
	}
}

// TestNeighboringLayersWithSizeLabel verifies sizes are still applied when the
// image.layers.size label is present.
func TestNeighboringLayersWithSizeLabel(t *testing.T) {
	labels := map[string]string{
		ctdsnapshotters.TargetRefLabel:         testRef,
		ctdsnapshotters.TargetLayerDigestLabel: testTarget,
		ctdsnapshotters.TargetImageLayersLabel: testTarget + "," + testNeighA,
		targetImageLayersSizeLabel:             "100,200",
	}

	sources, err := FromDefaultLabels(noopHosts)(labels)
	if err != nil {
		t.Fatalf("FromDefaultLabels: %v", err)
	}
	layers := sources[0].Manifest.Layers
	if len(layers) != 2 {
		t.Fatalf("expected 2 manifest layers, got %d", len(layers))
	}
	neigh := layers[1]
	if neigh.Digest.String() != testNeighA {
		t.Fatalf("expected neighbor %s, got %s", testNeighA, neigh.Digest)
	}
	if neigh.Size != 200 {
		t.Fatalf("expected neighbor size 200, got %d", neigh.Size)
	}
}

// TestNeighboringLayersSizeLengthMismatch verifies the length-mismatch error is
// preserved when the size label is present but inconsistent with the digests.
func TestNeighboringLayersSizeLengthMismatch(t *testing.T) {
	labels := map[string]string{
		ctdsnapshotters.TargetRefLabel:         testRef,
		ctdsnapshotters.TargetLayerDigestLabel: testTarget,
		ctdsnapshotters.TargetImageLayersLabel: testTarget + "," + testNeighA,
		targetImageLayersSizeLabel:             "100", // only one size for two digests
	}

	if _, err := FromDefaultLabels(noopHosts)(labels); err == nil {
		t.Fatal("expected error on digest/size length mismatch, got nil")
	}
}
