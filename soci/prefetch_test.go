package soci

import (
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/awslabs/soci-snapshotter/ztoc/compression"
)

func TestNewPrefetchArtifact(t *testing.T) {
	artifact := NewPrefetchArtifact()

	if artifact == nil {
		t.Fatalf("NewPrefetchArtifact() returned nil")
	}
	if artifact.Version != PrefetchArtifactVersion {
		t.Fatalf("expected Version %q, got %q", PrefetchArtifactVersion, artifact.Version)
	}
	if !artifact.IsEmpty() {
		t.Fatalf("new artifact should be empty")
	}
}

func TestPrefetchArtifactAddPrefetchSpanAndIsEmpty(t *testing.T) {
	artifact := NewPrefetchArtifact()

	if !artifact.IsEmpty() {
		t.Fatalf("expected empty artifact before adding spans")
	}

	span := PrefetchSpan{StartSpan: compression.SpanID(1), EndSpan: compression.SpanID(3)}
	artifact.AddPrefetchSpan(span)

	if artifact.IsEmpty() {
		t.Fatalf("expected non-empty artifact after adding span")
	}
	if len(artifact.PrefetchSpans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(artifact.PrefetchSpans))
	}
	if artifact.PrefetchSpans[0] != span {
		t.Fatalf("stored span does not match added span: %#v != %#v", artifact.PrefetchSpans[0], span)
	}
}

func TestMarshalPrefetchArtifact_EmptyError(t *testing.T) {
	// nil artifact
	if _, _, err := MarshalPrefetchArtifact(nil); err == nil {
		t.Fatalf("expected error when marshaling nil artifact, got nil")
	}

	// empty artifact
	empty := &PrefetchArtifact{Version: PrefetchArtifactVersion}
	if _, _, err := MarshalPrefetchArtifact(empty); err == nil {
		t.Fatalf("expected error when marshaling empty artifact, got nil")
	}
}

func TestMarshalPrefetchArtifact_Success(t *testing.T) {
	artifact := &PrefetchArtifact{
		Version: PrefetchArtifactVersion,
		PrefetchSpans: []PrefetchSpan{
			{StartSpan: compression.SpanID(0), EndSpan: compression.SpanID(2)},
		},
	}

	reader, desc, err := MarshalPrefetchArtifact(artifact)
	if err != nil {
		t.Fatalf("MarshalPrefetchArtifact() error = %v", err)
	}
	if reader == nil {
		t.Fatalf("expected non-nil reader")
	}
	if desc.MediaType != PrefetchArtifactMediaType {
		t.Fatalf("expected MediaType %q, got %q", PrefetchArtifactMediaType, desc.MediaType)
	}
	if desc.Size <= 0 {
		t.Fatalf("expected positive descriptor size, got %d", desc.Size)
	}

	// Read back the data and ensure it matches the original artifact
	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("failed to read marshaled data: %v", err)
	}

	var decoded PrefetchArtifact
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal marshaled data: %v", err)
	}

	if decoded.Version != artifact.Version {
		t.Fatalf("expected Version %q, got %q", artifact.Version, decoded.Version)
	}
	if len(decoded.PrefetchSpans) != len(artifact.PrefetchSpans) {
		t.Fatalf("expected %d spans, got %d", len(artifact.PrefetchSpans), len(decoded.PrefetchSpans))
	}
	if decoded.PrefetchSpans[0] != artifact.PrefetchSpans[0] {
		t.Fatalf("decoded span does not match original: %#v != %#v", decoded.PrefetchSpans[0], artifact.PrefetchSpans[0])
	}
}

func TestUnmarshalPrefetchArtifact_Success(t *testing.T) {
	original := &PrefetchArtifact{
		Version: PrefetchArtifactVersion,
		PrefetchSpans: []PrefetchSpan{
			{StartSpan: compression.SpanID(5), EndSpan: compression.SpanID(10), Priority: 1},
		},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("failed to marshal original artifact: %v", err)
	}

	artifact, err := UnmarshalPrefetchArtifact(strings.NewReader(string(data)))
	if err != nil {
		t.Fatalf("UnmarshalPrefetchArtifact() error = %v", err)
	}

	if artifact.Version != original.Version {
		t.Fatalf("expected Version %q, got %q", original.Version, artifact.Version)
	}
	if len(artifact.PrefetchSpans) != len(original.PrefetchSpans) {
		t.Fatalf("expected %d spans, got %d", len(original.PrefetchSpans), len(artifact.PrefetchSpans))
	}
	if artifact.PrefetchSpans[0] != original.PrefetchSpans[0] {
		t.Fatalf("decoded span does not match original: %#v != %#v", artifact.PrefetchSpans[0], original.PrefetchSpans[0])
	}
}

func TestUnmarshalPrefetchArtifact_InvalidJSON(t *testing.T) {
	_, err := UnmarshalPrefetchArtifact(strings.NewReader("not-json"))
	if err == nil {
		t.Fatalf("expected error for invalid JSON, got nil")
	}
}

func TestUnmarshalPrefetchArtifact_UnsupportedVersion(t *testing.T) {
	artifact := &PrefetchArtifact{
		Version: "0.9", // unsupported
		PrefetchSpans: []PrefetchSpan{
			{StartSpan: compression.SpanID(1), EndSpan: compression.SpanID(1)},
		},
	}

	data, err := json.Marshal(artifact)
	if err != nil {
		t.Fatalf("failed to marshal artifact: %v", err)
	}

	_, err = UnmarshalPrefetchArtifact(strings.NewReader(string(data)))
	if err == nil {
		t.Fatalf("expected error for unsupported version, got nil")
	}
}