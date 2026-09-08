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
	"context"
	"testing"

	"github.com/containerd/containerd/v2/core/remotes/docker"
	"github.com/containerd/containerd/v2/pkg/reference"
	ctdsnapshotters "github.com/containerd/containerd/v2/pkg/snapshotters"
	"github.com/containerd/log"
	"github.com/sirupsen/logrus"
	logrustest "github.com/sirupsen/logrus/hooks/test"
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

// TestHeadersFromLabels verifies the generic prefix convention: any label keyed
// soci.header.<name> becomes header <name>, the snapshotter forwards it verbatim,
// and non-matching, empty, or reserved labels are ignored.
func TestHeadersFromLabels(t *testing.T) {
	labels := map[string]string{
		CustomHeaderLabelPrefix + "x-request-id":   "caller-123",
		CustomHeaderLabelPrefix + "x-custom-trace": "trace-abc",
		CustomHeaderLabelPrefix + "x-empty":        "",          // dropped: empty value
		CustomHeaderLabelPrefix:                    "no-name",   // dropped: no header name
		CustomHeaderLabelPrefix + "Authorization":  "Basic abc", // dropped: reserved
		CustomHeaderLabelPrefix + "range":          "bytes=0-1", // dropped: reserved (any case)
		"containerd.io/snapshot/remote/soci.size":  "4096",      // dropped: not a header label
		"unrelated": "nope",
	}

	h := HeadersFromLabels(context.Background(), labels)

	if got := h.Get("x-request-id"); got != "caller-123" {
		t.Fatalf("x-request-id = %q, want %q", got, "caller-123")
	}
	if got := h.Get("x-custom-trace"); got != "trace-abc" {
		t.Fatalf("x-custom-trace = %q, want %q", got, "trace-abc")
	}
	if _, ok := h["X-Empty"]; ok {
		t.Fatalf("empty-valued header label should be dropped, got %v", h["X-Empty"])
	}
	if got := h.Get("Authorization"); got != "" {
		t.Fatalf("reserved header Authorization should be dropped, got %q", got)
	}
	if got := h.Get("Range"); got != "" {
		t.Fatalf("reserved header Range should be dropped, got %q", got)
	}
	// Only the two valid headers should be present.
	if len(h) != 2 {
		t.Fatalf("expected exactly 2 headers, got %d: %v", len(h), h)
	}
}

func TestHeadersFromLabels_None(t *testing.T) {
	if h := HeadersFromLabels(context.Background(), map[string]string{"foo": "bar"}); len(h) != 0 {
		t.Fatalf("expected no headers, got %v", h)
	}
}

// TestHeadersFromLabels_InvalidDropped verifies a label with an invalid header
// name or a CR/LF in the value is dropped, so a label cannot inject a header.
func TestHeadersFromLabels_InvalidDropped(t *testing.T) {
	ctx, hook := ctxWithLogHook()

	invalid := []string{"x with space", "x@bad", "x-inject", "x-null"}
	labels := map[string]string{
		CustomHeaderLabelPrefix + "x with space": "v",              // invalid name: space
		CustomHeaderLabelPrefix + "x@bad":        "v",              // invalid name: non-token
		CustomHeaderLabelPrefix + "x-inject":     "v\r\nX-Evil: 1", // invalid value: CRLF
		CustomHeaderLabelPrefix + "x-null":       "a\x00b",         // invalid value: NUL
		CustomHeaderLabelPrefix + "x-request-id": "caller-123",     // valid: kept
	}

	h := HeadersFromLabels(ctx, labels)

	if got := h.Get("x-request-id"); got != "caller-123" {
		t.Fatalf("valid header dropped: got %q", got)
	}
	if len(h) != 1 {
		t.Fatalf("expected only the valid header, got %d: %v", len(h), h)
	}
	if got := h.Get("X-Evil"); got != "" {
		t.Fatalf("CRLF injection produced a second header: %q", got)
	}

	warned := warnedHeaders(hook)
	for _, name := range invalid {
		if !warned[name] {
			t.Fatalf("expected a WARN for dropped invalid header %q", name)
		}
	}
}

// TestHeadersFromLabels_ReservedDropped verifies every reserved header name is
// dropped (case-insensitively) and a WARN is logged for each.
func TestHeadersFromLabels_ReservedDropped(t *testing.T) {
	// The canonical reserved names, plus mixed-case spellings to prove the match
	// is case-insensitive.
	names := []string{
		"Range", "Accept-Encoding", "Accept", "Content-Type", "Content-Length",
		"Authorization", "User-Agent", "Referer",
		"AuThOrIzAtIoN", "USER-AGENT", "range",
	}

	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			ctx, hook := ctxWithLogHook()
			labels := map[string]string{
				CustomHeaderLabelPrefix + name:           "should-be-dropped",
				CustomHeaderLabelPrefix + "x-request-id": "kept",
			}

			h := HeadersFromLabels(ctx, labels)

			if got := h.Get(name); got != "" {
				t.Fatalf("reserved header %q reached the map: %q", name, got)
			}
			if got := h.Get("x-request-id"); got != "kept" {
				t.Fatalf("non-reserved header dropped: %q", got)
			}
			if !warnedHeaders(hook)[name] {
				t.Fatalf("expected a WARN for dropped reserved header %q", name)
			}
		})
	}
}

// ctxWithLogHook returns a context carrying a private WARN-level logger and a
// hook that captures its entries. It does not touch the process-global logger,
// so tests stay isolated.
func ctxWithLogHook() (context.Context, *logrustest.Hook) {
	logger, hook := logrustest.NewNullLogger()
	logger.SetLevel(logrus.WarnLevel)
	return log.WithLogger(context.Background(), logrus.NewEntry(logger)), hook
}

// warnedHeaders returns the set of header names that were logged at WARN level.
func warnedHeaders(hook *logrustest.Hook) map[string]bool {
	warned := map[string]bool{}
	for _, e := range hook.AllEntries() {
		if e.Level != logrus.WarnLevel {
			continue
		}
		if h, ok := e.Data["header"].(string); ok {
			warned[h] = true
		}
	}
	return warned
}
