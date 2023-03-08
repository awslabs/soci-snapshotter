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

package source

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/labels"
	ctdsnapshotters "github.com/containerd/containerd/pkg/snapshotters"
	"github.com/containerd/containerd/reference"
	"github.com/containerd/containerd/remotes/docker"
	digest "github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// GetSources is a function for converting snapshot labels into typed blob sources
// information. This package defines a default converter which provides source
// information based on some labels but implementations aren't required to use labels.
// Implementations are allowed to return several sources (registry config + image refs)
// about the blob.
type GetSources func(labels map[string]string) (source []Source, err error)

// RegistryHosts returns a list of registries that provides the specified image.
type RegistryHosts func(reference.Spec) ([]docker.RegistryHost, error)

// Source is a typed blob source information. This contains information about
// a blob stored in registries and some contexts of the blob.
type Source struct {

	// Hosts is a registry configuration where this blob is stored.
	Hosts RegistryHosts

	// Name is an image reference which contains this blob.
	Name reference.Spec

	// Target is a descriptor of this blob.
	Target ocispec.Descriptor

	// Manifest is an image manifest which contains the blob. This will
	// be used by the filesystem to pre-resolve some layers contained in
	// the manifest.
	// Currently, layer digest (Manifest.Layers.Digest) and size will be used.
	Manifest ocispec.Manifest
}

const (
	// TargetSizeLabel is a label which contains layer size.
	TargetSizeLabel = "containerd.io/snapshot/remote/soci.size"

	// targetImageLayersSizeLabel is a label which contains layer digests contained in
	// the target image.
	targetImageLayersSizeLabel = "containerd.io/snapshot/remote/image.layers.size"

	// TargetSociIndexDigestLabel is a label which contains the digest of the soci index.
	TargetSociIndexDigestLabel = "containerd.io/snapshot/remote/soci.index.digest"
)

// FromDefaultLabels returns a function for converting snapshot labels to
// source information based on labels.
func FromDefaultLabels(hosts RegistryHosts) GetSources {
	return func(labels map[string]string) ([]Source, error) {
		refStr, ok := labels[ctdsnapshotters.TargetRefLabel]
		if !ok {
			return nil, fmt.Errorf("reference hasn't been passed")
		}
		refspec, err := reference.Parse(refStr)
		if err != nil {
			return nil, err
		}

		digestStr, ok := labels[ctdsnapshotters.TargetLayerDigestLabel]
		if !ok {
			return nil, fmt.Errorf("digest hasn't been passed")
		}
		target, err := digest.Parse(digestStr)
		if err != nil {
			return nil, err
		}

		targetSizeStr, ok := labels[TargetSizeLabel]
		if !ok {
			return nil, fmt.Errorf("layer size hasn't been passed")
		}
		targetSize, err := strconv.ParseInt(targetSizeStr, 10, 64)
		if err != nil {
			return nil, err
		}

		var neighboringLayers []ocispec.Descriptor
		if l, ok := labels[ctdsnapshotters.TargetImageLayersLabel]; ok {
			layerDigestsStr := strings.Split(l, ",")
			layerSizes := strings.Split(labels[targetImageLayersSizeLabel], ",")
			if len(layerDigestsStr) != len(layerSizes) {
				return nil, fmt.Errorf("the lengths of layer digests and layer sizes don't match")
			}

			for i := 0; i < len(layerDigestsStr); i++ {
				l := layerDigestsStr[i]
				d, err := digest.Parse(l)
				if err != nil {
					return nil, err
				}
				if d.String() != target.String() {
					size, err := strconv.ParseInt(layerSizes[i], 10, 64)
					if err != nil {
						return nil, err
					}
					desc := ocispec.Descriptor{Digest: d, Size: size}
					neighboringLayers = append(neighboringLayers, desc)
				}
			}
		}
		targetDesc := ocispec.Descriptor{
			Digest:      target,
			Size:        targetSize,
			Annotations: labels,
		}

		return []Source{
			{
				Hosts:    hosts,
				Name:     refspec,
				Target:   targetDesc,
				Manifest: ocispec.Manifest{Layers: append([]ocispec.Descriptor{targetDesc}, neighboringLayers...)},
			},
		}, nil
	}
}

// AppendDefaultLabelsHandlerWrapper makes a handler which appends image's basic
// information to each layer descriptor as annotations during unpack. These
// annotations will be passed to this remote snapshotter as labels and used to
// construct source information.
func AppendDefaultLabelsHandlerWrapper(indexDigest string, wrapper func(images.Handler) images.Handler) func(f images.Handler) images.Handler {
	return func(f images.Handler) images.Handler {
		return images.HandlerFunc(func(ctx context.Context, desc ocispec.Descriptor) ([]ocispec.Descriptor, error) {
			children, err := wrapper(f).Handle(ctx, desc)
			if err != nil {
				return nil, err
			}
			switch desc.MediaType {
			case ocispec.MediaTypeImageManifest, images.MediaTypeDockerSchema2Manifest:
				for i := range children {
					c := &children[i]
					if images.IsLayerType(c.MediaType) {
						if c.Annotations == nil {
							c.Annotations = make(map[string]string)
						}

						c.Annotations[TargetSizeLabel] = fmt.Sprintf("%d", c.Size)
						c.Annotations[TargetSociIndexDigestLabel] = indexDigest

						var layerSizes string
						for _, l := range children[i:] {
							if images.IsLayerType(l.MediaType) {
								ls := fmt.Sprintf("%d,", l.Size)
								// This avoids the label hits the size limitation.
								// Skipping layers is allowed here and only affects performance.
								if err := labels.Validate(targetImageLayersSizeLabel, layerSizes+ls); err != nil {
									break
								}
								layerSizes += ls
							}
						}
						c.Annotations[targetImageLayersSizeLabel] = strings.TrimSuffix(layerSizes, ",")
					}
				}
			}
			return children, nil
		})
	}
}
