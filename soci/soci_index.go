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
	"fmt"
	"io"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/platforms"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	orascontent "oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/errdef"
)

const (
	// artifactType of index SOCI index
	SociIndexArtifactType = "application/vnd.amazon.soci.index.v1+json"
	// mediaType of ztoc
	SociLayerMediaType = "application/octet-stream"
	// index annotation for image layer media type
	IndexAnnotationImageLayerMediaType = "com.amazon.soci.image-layer-mediaType"
	// index annotation for image layer digest
	IndexAnnotationImageLayerDigest = "com.amazon.soci.image-layer-digest"
	// index annotation for build tool identifier
	IndexAnnotationBuildToolIdentifier = "com.amazon.soci.build-tool-identifier"
	// media type for OCI Artifact manifest
	OCIArtifactManifestMediaType = "application/vnd.oci.artifact.manifest.v1+json"
	// media type for ORAS manifest
	ORASManifestMediaType = "application/vnd.cncf.oras.artifact.manifest.v1+json"
)

type ManifestType int

const (
	ManifestOCIArtifact ManifestType = iota
	ManifestORAS
)

var (
	errNotLayerType           = errors.New("not a layer mediaType")
	errUnsupportedLayerFormat = errors.New("unsupported layer format")
)

// Index represents an ORAS/OCI Artifact Manifest
type Index struct {
	// The media type of the manifest
	MediaType string `json:"mediaType"`

	// Artifact type of the manifest
	ArtifactType string `json:"artifactType"`

	// Blobs referenced by the manifest
	Blobs []ocispec.Descriptor `json:"blobs,omitempty"`

	Subject *ocispec.Descriptor `json:"subject,omitempty"`

	// Optional annotations in the manifest
	Annotations map[string]string `json:"annotations,omitempty"`
}

type IndexWithMetadata struct {
	Index       *Index
	ImageDigest digest.Digest
	Platform    ocispec.Platform
}

type IndexDescriptorInfo struct {
	ocispec.Descriptor
}

func GetIndexDescriptorCollection(ctx context.Context, cs content.Store, img images.Image) ([]IndexDescriptorInfo, error) {
	descriptors := []IndexDescriptorInfo{}
	platform := platforms.Default()
	indexDesc, err := GetImageManifestDescriptor(ctx, cs, img.Target, platform)
	if err != nil {
		return descriptors, err
	}

	entries, err := getIndexArtifactEntries(indexDesc.Digest.String())
	if err != nil {
		return descriptors, err
	}

	for _, entry := range entries {
		dgst, err := digest.Parse(entry.Digest)
		if err != nil {
			continue
		}
		desc := ocispec.Descriptor{
			MediaType: entry.MediaType,
			Digest:    dgst,
			Size:      entry.Size,
		}
		descriptors = append(descriptors, IndexDescriptorInfo{
			Descriptor: desc,
		})
	}

	return descriptors, nil
}

type buildConfig struct {
	minLayerSize        int64
	buildToolIdentifier string
	manifestType        ManifestType
}

func newOCIArtifactManifest(blobs []ocispec.Descriptor, subject *ocispec.Descriptor, annotations map[string]string) *Index {
	return &Index{
		Blobs:        blobs,
		ArtifactType: SociIndexArtifactType,
		Annotations:  annotations,
		Subject:      subject,
		MediaType:    OCIArtifactManifestMediaType,
	}
}

// Returns a new index from a Reader.
func NewIndexFromReader(reader io.Reader) (*Index, error) {
	index := new(Index)
	if err := json.NewDecoder(reader).Decode(index); err != nil {
		return nil, fmt.Errorf("unable to decode reader into index: %v", err)
	}
	return index, nil
}

func skipBuildingZtoc(desc ocispec.Descriptor, cfg *buildConfig) bool {
	if cfg == nil {
		return false
	}
	// avoid the file access if the layer size is below threshold
	if desc.Size < cfg.minLayerSize {
		return true
	}
	return false
}

// getImageManifestDescriptor gets the descriptor of image manifest
func GetImageManifestDescriptor(ctx context.Context, cs content.Store, imageTarget ocispec.Descriptor, platform platforms.MatchComparer) (*ocispec.Descriptor, error) {
	if images.IsIndexType(imageTarget.MediaType) {
		manifests, err := images.Children(ctx, cs, imageTarget)
		if err != nil {
			return nil, err
		}
		for _, manifest := range manifests {
			if manifest.Platform == nil {
				return nil, errors.New("manifest should have proper platform")
			}
			if platform.Match(*manifest.Platform) {
				return &manifest, nil
			}
		}
		return nil, errors.New("image manifest not found")
	} else if images.IsManifestType(imageTarget.MediaType) {
		return &imageTarget, nil
	}

	return nil, nil
}

// WriteSociIndex writes the SociIndex manifest
func WriteSociIndex(ctx context.Context, indexWithMetadata IndexWithMetadata, store orascontent.Storage) error {
	manifest, err := json.Marshal(indexWithMetadata.Index)
	if err != nil {
		return err
	}

	dgst := digest.FromBytes(manifest)
	size := int64(len(manifest))

	err = store.Push(ctx, ocispec.Descriptor{
		Digest: dgst,
		Size:   size,
	}, bytes.NewReader(manifest))

	if err != nil && !errors.Is(err, errdef.ErrAlreadyExists) {
		return fmt.Errorf("cannot write SOCI index to local store: %w", err)
	}

	log.G(ctx).WithField("digest", dgst.String()).Debugf("soci index has been written")

	refers := indexWithMetadata.Index.Subject

	if refers == nil {
		return errors.New("cannot write soci index: the Refers field is nil")
	}

	// this entry is persisted to be used by cli push
	entry := &ArtifactEntry{
		Digest:         dgst.String(),
		OriginalDigest: refers.Digest.String(),
		ImageDigest:    indexWithMetadata.ImageDigest.String(),
		Platform:       platforms.Format(indexWithMetadata.Platform),
		Type:           ArtifactEntryTypeIndex,
		Location:       refers.Digest.String(),
		Size:           size,
		MediaType:      indexWithMetadata.Index.MediaType,
	}
	return writeArtifactEntry(entry)
}
