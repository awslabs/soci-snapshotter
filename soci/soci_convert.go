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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"

	"github.com/awslabs/soci-snapshotter/soci/store"
	"github.com/awslabs/soci-snapshotter/util/ociutil"
	"github.com/containerd/containerd/images"
	"github.com/containerd/platforms"
	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/specs-go"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/errdef"
)

type ConvertOption func(*convertConfig) error

type convertConfig struct {
	platforms []ocispec.Platform
	gcRoot    bool
}

// ConvertWithPlatforms sets the platforms that will be indexed during conversion
func ConvertWithPlatforms(platforms ...ocispec.Platform) ConvertOption {
	return func(cc *convertConfig) error {
		cc.platforms = platforms
		return nil
	}
}

// ConvertWithNoGarbageCollectionLabels disables adding a containerd root gc label
// to the converted image and SOCI indexes. The caller is responsible for ensuring the OCI Index
// doesn't get garbage collected.
func ConvertWithNoGarbageCollectionLabels() ConvertOption {
	return func(cc *convertConfig) error {
		cc.gcRoot = false
		return nil
	}
}

// Convert converts an image into a SOCI enabled image.
//
// At a high level, this process:
// 1. Creates a SOCI index for each platform (unless overridden by ConvertWithPlatforms)
// 2. Adds an annotation to each image with the SOCI index digest
// 3. Appends the SOCI indexes to the list of manifests in the OCI index
//
// Notes:
// Adding an annotation to an image changes the image digest. This is equivalent to creating a new image.
// This function will serialize and push the new image manifest to the content store and replaces the original
// image in the OCI index. The layers will be shared, not duplicated.
//
// If the image is a single platform image, this function will create an OCI index so that it can bundle the
// image and SOCI index into a single artifact.
func (b *IndexBuilder) Convert(ctx context.Context, img images.Image, opts ...ConvertOption) (*ocispec.Descriptor, error) {
	allPlatforms, err := images.Platforms(ctx, b.contentStore, img.Target)
	if err != nil {
		return nil, err
	}
	if len(allPlatforms) == 0 {
		return nil, errors.New("image does not support any platforms")
	}
	if images.IsManifestType(img.Target.MediaType) && img.Target.Platform == nil {
		// If the image's target descriptor is a single manifest, then it will not
		// contain a platform because that information is stored in the image config instead.
		// If we directly use this descriptor in the converted image,
		// runtimes will not be able to pull the correct manifest.
		// We know the actual platform by inspecting the image config in `images.Platforms`,
		// so we add that to the target descriptor.
		img.Target.Platform = &allPlatforms[0]
	}
	convertCfg := convertConfig{
		platforms: allPlatforms,
		gcRoot:    true,
	}
	for _, opt := range opts {
		err := opt(&convertCfg)
		if err != nil {
			return nil, err
		}
	}
	convertCfg.platforms = ociutil.DedupePlatforms(convertCfg.platforms)

	// Initialize the OCI Index
	ociIndex, err := b.newOciIndex(ctx, img)
	if err != nil {
		return nil, err
	}

	// Create the SOCI Indexes
	indexes, err := b.buildSociIndexesv2ForPlatforms(ctx, img, convertCfg.platforms)
	if err != nil {
		return nil, err
	}

	// Add Annotations to the image manifests
	err = b.annotateImages(ctx, &ociIndex, indexes)
	if err != nil {
		return nil, err
	}

	// Add SOCI indexes to OCI index
	err = b.addSociIndexes(ctx, &ociIndex, indexes)
	if err != nil {
		return nil, err
	}

	// Serialize the OCI Index
	ociIndexDesc, err := b.pushOCIObject(ctx, &ociIndex)
	ociIndexDesc.MediaType = ocispec.MediaTypeImageIndex
	if err != nil {
		return nil, err
	}

	// Apply GC Labels
	for i, desc := range ociIndex.Manifests {
		err := store.LabelGCRefContent(ctx, b.blobStore, ociIndexDesc, fmt.Sprintf("m.%d", i), desc.Digest.String())
		if err != nil {
			return nil, err
		}
	}
	if convertCfg.gcRoot {
		err := store.LabelGCRoot(ctx, b.blobStore, ociIndexDesc)
		if err != nil {
			return nil, err
		}
	}

	return &ociIndexDesc, nil
}

// buildSociIndexesv2ForPlatforms builds a SOCI index for each specified platform
func (b *IndexBuilder) buildSociIndexesv2ForPlatforms(ctx context.Context, img images.Image, platforms []ocispec.Platform) ([]*IndexWithMetadata, error) {
	var indexes []*IndexWithMetadata
	for _, platform := range platforms {
		index, err := b.Build(ctx, img,
			WithPlatform(platform),
			// Don't set a GC label on the SOCI indexes because their GC will be attached to
			// the converted image's OCI index. If we label them, they will not get
			// gcd with the rest of the converted image
			WithNoGarbageCollectionLabel(),
			withIndexVersion(V2),
		)
		if err != nil {
			// If a platform produces an empty index, try other platforms.
			// This is unlikely, but could happen if one platform doesn't have
			// any layers larger than min layer size. We should still try to index
			// other platforms.
			if errors.Is(err, ErrEmptyIndex) {
				continue
			}
			return nil, err
		}
		indexes = append(indexes, index)
	}

	// If we didn't produce any indexes, return that as an error
	if len(indexes) == 0 {
		return nil, ErrEmptyIndex
	}
	return indexes, nil
}

// newOciIndex creates an OCI index object for the converted image.
// If `img.Target` is already an OCI index (or docker manifest list), newOciIndex loads that index,
// otherwise, it creates a new OCI index containing just the single image target.
func (b *IndexBuilder) newOciIndex(ctx context.Context, img images.Image) (ocispec.Index, error) {
	if images.IsIndexType(img.Target.MediaType) {
		// Load the original target
		ra, err := b.contentStore.ReaderAt(ctx, img.Target)
		if err != nil {
			return ocispec.Index{}, err
		}
		b, err := io.ReadAll(io.NewSectionReader(ra, 0, ra.Size()))
		if err != nil {
			return ocispec.Index{}, err
		}
		err = ociutil.ValidateMediaType(b, img.Target.MediaType)
		if err != nil {
			return ocispec.Index{}, err
		}

		var ociIndex ocispec.Index
		err = json.Unmarshal(b, &ociIndex)
		// Some Registries don't like it when you push a Docker v2 manifest
		// To increase compatibility, change the media type to OCI image manifest
		ociIndex.MediaType = ocispec.MediaTypeImageIndex
		return ociIndex, err
	}
	return ocispec.Index{
		Versioned: specs.Versioned{
			SchemaVersion: 2,
		},
		MediaType: ocispec.MediaTypeImageIndex,
		Manifests: []ocispec.Descriptor{
			img.Target,
		},
	}, nil
}

// annotateImages adds the SOCI index digest to the corresponding image manifest descriptor, modifying `ociIndex` in the process.
// Adding annotations modifies the image manifest, so annotateImages also pushes the modified image manifests to the blobStore,
// computes the new image digests, and modified the image descriptors in `ociIndex`
func (b *IndexBuilder) annotateImages(ctx context.Context, ociIndex *ocispec.Index, sociIndexes []*IndexWithMetadata) error {
	for i := 0; i < len(ociIndex.Manifests); i++ {
		manifestDesc := &ociIndex.Manifests[i]
		// images.Manifest validates the mediatype, no need to do it ourselves like
		// we did when loading the OCI index
		manifest, err := images.Manifest(ctx, b.contentStore, *manifestDesc, nil)
		if err != nil {
			return err
		}
		// Some Registries don't like mixing Docker V2 manifests with OCI image manifests.
		// Since we use ArtifactTypes for SOCI indexes, we will use OCI image manifests everywhere to increase compatibility.
		// Registries don't seem to be as picky about layer and config types
		manifest.MediaType = ocispec.MediaTypeImageManifest

		idx := slices.IndexFunc(sociIndexes, func(i *IndexWithMetadata) bool { return i.ManifestDesc.Digest == manifestDesc.Digest })
		if idx >= 0 {
			indexWithMetadata := sociIndexes[idx]

			if manifest.Annotations == nil {
				manifest.Annotations = make(map[string]string)
			}
			manifest.Annotations[ImageAnnotationSociIndexDigest] = indexWithMetadata.Desc.Digest.String()
		}

		newManifestDesc, err := b.pushOCIObject(ctx, manifest)
		if err != nil {
			return err
		}

		// Modify the original
		manifestDesc.Digest = newManifestDesc.Digest
		manifestDesc.Size = newManifestDesc.Size
		manifestDesc.Annotations = manifest.Annotations
		manifestDesc.MediaType = ocispec.MediaTypeImageManifest

		if idx >= 0 {
			indexWithMetadata := sociIndexes[idx]
			if indexWithMetadata.Desc.Annotations == nil {
				indexWithMetadata.Desc.Annotations = make(map[string]string)
			}
			indexWithMetadata.Desc.Annotations[IndexAnnotationImageManifestDigest] = manifestDesc.Digest.String()
		}
	}
	return nil
}

// addSociIndexes modifies the list of manifests in the OCI index to include the SOCI indexes.
// If the OCI index already contains a SOCI index for the same platform, the old SOCI index is replaced,
// otherwise, the SOCI index is appended to list of manifests.
func (b *IndexBuilder) addSociIndexes(_ context.Context, ociIndex *ocispec.Index, sociIndexes []*IndexWithMetadata) error {
	for _, indexWithMetadata := range sociIndexes {
		desc := indexWithMetadata.Desc
		desc.Platform = indexWithMetadata.Platform

		if indexWithMetadata.Platform == nil {
			return fmt.Errorf("index does not have a valid platform")
		}
		sociIndexPlatform := platforms.Normalize(*indexWithMetadata.Platform)
		matcher := platforms.OnlyStrict(sociIndexPlatform)
		i := slices.IndexFunc(ociIndex.Manifests, func(desc ocispec.Descriptor) bool {
			return desc.ArtifactType == SociIndexArtifactTypeV2 && desc.Platform != nil && matcher.Match(*desc.Platform)
		})

		if i >= 0 {
			ociIndex.Manifests[i] = desc
		} else {
			ociIndex.Manifests = append(ociIndex.Manifests, desc)
		}
	}

	return nil
}

// pushOCIObject serializes and pushes an OCI object (manifest or OCI index) to the blobStore.
// It returns the descriptor of the object that was pushed which will only contain the digest and size.
// It is up to the caller to set other information (MediaType, ArtifactType, Platform, etc) as needed.
func (b *IndexBuilder) pushOCIObject(ctx context.Context, obj any) (ocispec.Descriptor, error) {
	bs, err := json.Marshal(obj)
	if err != nil {
		return ocispec.Descriptor{}, err
	}
	dg := digest.FromBytes(bs)
	desc := ocispec.Descriptor{
		Digest: dg,
		Size:   int64(len(bs)),
	}
	err = b.blobStore.Push(ctx, desc, bytes.NewReader(bs))
	if err != nil && !errors.Is(err, errdef.ErrAlreadyExists) {
		return ocispec.Descriptor{}, err
	}
	return desc, nil
}
