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

package soci

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"github.com/awslabs/soci-snapshotter/ztoc/compression"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

const (
	PrefetchArtifactMediaType = "application/vnd.amazon.soci.prefetch.v1+json"
	PrefetchArtifactVersion   = "1.0"
)

type PrefetchArtifact struct {
	Version       string         `json:"version"`
	PrefetchSpans []PrefetchSpan `json:"prefetch_spans"`
}

type PrefetchSpan struct {
	StartSpan compression.SpanID `json:"start_span"`
	EndSpan   compression.SpanID `json:"end_span"`

	// Priority is an optional field for future use to support prioritized prefetching.
	// Lower values indicate higher priority. If omitted, all spans have equal priority.
	Priority int `json:"priority,omitempty"`
}

func NewPrefetchArtifact() *PrefetchArtifact {
	return &PrefetchArtifact{
		Version:       PrefetchArtifactVersion,
		PrefetchSpans: make([]PrefetchSpan, 0),
	}
}

func (p *PrefetchArtifact) AddPrefetchSpan(prefetchSpan PrefetchSpan) {
	p.PrefetchSpans = append(p.PrefetchSpans, prefetchSpan)
}

func (p *PrefetchArtifact) IsEmpty() bool {
	return len(p.PrefetchSpans) == 0
}

func MarshalPrefetchArtifact(artifact *PrefetchArtifact) (io.Reader, ocispec.Descriptor, error) {
	if artifact == nil || artifact.IsEmpty() {
		return nil, ocispec.Descriptor{}, fmt.Errorf("prefetch artifact is empty")
	}

	jsonData, err := json.Marshal(artifact)
	if err != nil {
		return nil, ocispec.Descriptor{}, fmt.Errorf("failed to marshal prefetch artifact: %w", err)
	}

	dgst := digest.FromBytes(jsonData)
	desc := ocispec.Descriptor{
		MediaType: PrefetchArtifactMediaType,
		Digest:    dgst,
		Size:      int64(len(jsonData)),
	}

	return bytes.NewReader(jsonData), desc, nil
}

func UnmarshalPrefetchArtifact(reader io.Reader) (*PrefetchArtifact, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read prefetch artifact: %w", err)
	}

	var artifact PrefetchArtifact
	if err := json.Unmarshal(data, &artifact); err != nil {
		return nil, fmt.Errorf("failed to unmarshal prefetch artifact: %w", err)
	}

	if artifact.Version != PrefetchArtifactVersion {
		return nil, fmt.Errorf("unsupported prefetch artifact version: %s (expected %s)",
			artifact.Version, PrefetchArtifactVersion)
	}

	return &artifact, nil
}
