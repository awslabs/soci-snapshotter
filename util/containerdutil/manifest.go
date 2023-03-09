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

package containerdutil

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/platforms"
	"github.com/containerd/containerd/remotes"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// Forked from github.com/containerd/containerd/images/image.go
// commit: a776a27af54a803657d002e7574a4425b3949f56

// unknownDocument represents a manifest, manifest list, or index that has not
// yet been validated.
type unknownDocument struct {
	MediaType string          `json:"mediaType,omitempty"`
	Config    json.RawMessage `json:"config,omitempty"`
	Layers    json.RawMessage `json:"layers,omitempty"`
	Manifests json.RawMessage `json:"manifests,omitempty"`
	FSLayers  json.RawMessage `json:"fsLayers,omitempty"` // schema 1
}

// ValidateMediaType returns an error if the byte slice is invalid JSON or if
// the media type identifies the blob as one format but it contains elements of
// another format.
func ValidateMediaType(b []byte, mt string) error {
	var doc unknownDocument
	if err := json.Unmarshal(b, &doc); err != nil {
		return err
	}
	if len(doc.FSLayers) != 0 {
		return fmt.Errorf("media-type: schema 1 not supported")
	}
	switch mt {
	case images.MediaTypeDockerSchema2Manifest, ocispec.MediaTypeImageManifest:
		if len(doc.Manifests) != 0 ||
			doc.MediaType == images.MediaTypeDockerSchema2ManifestList ||
			doc.MediaType == ocispec.MediaTypeImageIndex {
			return fmt.Errorf("media-type: expected manifest but found index (%s)", mt)
		}
	case images.MediaTypeDockerSchema2ManifestList, ocispec.MediaTypeImageIndex:
		if len(doc.Config) != 0 || len(doc.Layers) != 0 ||
			doc.MediaType == images.MediaTypeDockerSchema2Manifest ||
			doc.MediaType == ocispec.MediaTypeImageManifest {
			return fmt.Errorf("media-type: expected index but found manifest (%s)", mt)
		}
	}
	return nil
}

// Fetch manifest of the specified platform
func FetchManifestPlatform(ctx context.Context, fetcher remotes.Fetcher, desc ocispec.Descriptor, platform ocispec.Platform) (ocispec.Manifest, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	r, err := fetcher.Fetch(ctx, desc)
	if err != nil {
		return ocispec.Manifest{}, err
	}
	defer r.Close()

	var manifest ocispec.Manifest
	switch desc.MediaType {
	case images.MediaTypeDockerSchema2Manifest, ocispec.MediaTypeImageManifest:
		p, err := io.ReadAll(r)
		if err != nil {
			return ocispec.Manifest{}, err
		}
		if err := ValidateMediaType(p, desc.MediaType); err != nil {
			return ocispec.Manifest{}, err
		}
		if err := json.Unmarshal(p, &manifest); err != nil {
			return ocispec.Manifest{}, err
		}
		return manifest, nil
	case images.MediaTypeDockerSchema2ManifestList, ocispec.MediaTypeImageIndex:
		var index ocispec.Index
		p, err := io.ReadAll(r)
		if err != nil {
			return ocispec.Manifest{}, err
		}
		if err := ValidateMediaType(p, desc.MediaType); err != nil {
			return ocispec.Manifest{}, err
		}
		if err = json.Unmarshal(p, &index); err != nil {
			return ocispec.Manifest{}, err
		}
		var target ocispec.Descriptor
		found := false
		for _, m := range index.Manifests {
			p := platforms.DefaultSpec()
			if m.Platform != nil {
				p = *m.Platform
			}
			if !platforms.NewMatcher(platform).Match(p) {
				continue
			}
			target = m
			found = true
			break
		}
		if !found {
			return ocispec.Manifest{}, fmt.Errorf("no manifest found for platform")
		}
		return FetchManifestPlatform(ctx, fetcher, target, platform)
	}
	return ocispec.Manifest{}, fmt.Errorf("unknown mediatype %q", desc.MediaType)
}
