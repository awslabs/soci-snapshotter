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

package ociutil

import (
	"encoding/json"
	"fmt"

	"github.com/containerd/containerd/images"
	"github.com/containerd/platforms"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// UnknownDocument represents a manifest, manifest list, or index that has not
// yet been validated.
// Copied from https://github.com/containerd/containerd/blob/d5534c6014534e02a0893dd6894d88130f45ae02/core/images/image.go#L381
type UnknownDocument struct {
	MediaType string          `json:"mediaType,omitempty"`
	Config    json.RawMessage `json:"config,omitempty"`
	Layers    json.RawMessage `json:"layers,omitempty"`
	Manifests json.RawMessage `json:"manifests,omitempty"`
	FSLayers  json.RawMessage `json:"fsLayers,omitempty"` // schema 1
}

// ValidateMediaType returns an error if the byte slice is invalid JSON,
// if the format of the blob is not supported, or if the media type
// identifies the blob as one format, but it identifies itself as, or
// contains elements of another format.
// Copied from https://github.com/containerd/containerd/blob/d5534c6014534e02a0893dd6894d88130f45ae02/core/images/image.go#L393
func ValidateMediaType(b []byte, mt string) error {
	var doc UnknownDocument
	if err := json.Unmarshal(b, &doc); err != nil {
		return err
	}
	if len(doc.FSLayers) != 0 {
		return fmt.Errorf("media-type: schema 1 not supported")
	}
	if images.IsManifestType(mt) && (len(doc.Manifests) != 0 || images.IsIndexType(doc.MediaType)) {
		return fmt.Errorf("media-type: expected manifest but found index (%s)", mt)
	} else if images.IsIndexType(mt) && (len(doc.Config) != 0 || len(doc.Layers) != 0 || images.IsManifestType(doc.MediaType)) {
		return fmt.Errorf("media-type: expected index but found manifest (%s)", mt)
	}
	return nil
}

func DedupePlatforms(ps []ocispec.Platform) []ocispec.Platform {
	var matchers []platforms.Matcher
	var res []ocispec.Platform

NextPlatform:
	for _, p := range ps {
		for _, m := range matchers {
			if m.Match(platforms.Normalize(p)) {
				continue NextPlatform
			}
		}
		res = append(res, p)
		matchers = append(matchers, platforms.OnlyStrict(platforms.Normalize(p)))
	}
	return res
}
