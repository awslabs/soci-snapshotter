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
	"os"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/platforms"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
	orascontent "oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/errdef"
)

const (
	// mediaType of SOCI index
	sociIndexMediaType = "application/vnd.cncf.oras.artifact.manifest.v1+json"
	// artifactType of index SOCI index
	SociIndexArtifactType = "application/vnd.amazon.soci.index.v1+json"
	// mediaType of ztoc
	SociLayerMediaType = "application/zstd"
	// index annotation for image layer media type
	IndexAnnotationImageLayerMediaType = "com.amazon.soci.image-layer-mediaType"
	// index annotation for image layer digest
	IndexAnnotationImageLayerDigest = "com.amazon.soci.image-layer-digest"
	// index annotation for build tool identifier
	IndexAnnotationBuildToolIdentifier = "com.amazon.soci.build-tool-identifier"
	// index annotation for build tool version
	IndexAnnotationBuildToolVersion = "com.amazon.soci.build-tool-version"
)

var (
	errNotLayerType = errors.New("not a layer mediaType")
)

// nolint:revive
type SociIndex struct {
	MediaType    string `json:"mediaType"`
	ArtifactType string `json:"artifactType"`
	// descriptors of ztocs
	Blobs []*ocispec.Descriptor `json:"blobs,omitempty"`
	// descriptor of image manifest
	Subject ocispec.Descriptor `json:"subject,omitempty"`

	Annotations map[string]string `json:"annotations,omitempty"`
}

type IndexWithMetadata struct {
	Index       *SociIndex
	ImageDigest digest.Digest
	Platform    ocispec.Platform
}

func GetIndexDescriptorCollection(ctx context.Context, cs content.Store, img images.Image) ([]ocispec.Descriptor, error) {
	descriptors := []ocispec.Descriptor{}
	platform := platforms.Default()
	indexDesc, err := GetImageManifestDescriptor(ctx, cs, img, platform)
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
			MediaType: sociIndexMediaType,
			Digest:    dgst,
			Size:      entry.Size,
		}
		descriptors = append(descriptors, desc)
	}

	return descriptors, nil
}

type buildConfig struct {
	minLayerSize        int64
	buildToolIdentifier string
	buildToolVersion    string
}

type BuildOption func(c *buildConfig) error

func WithMinLayerSize(minLayerSize int64) BuildOption {
	return func(c *buildConfig) error {
		c.minLayerSize = minLayerSize
		return nil
	}
}

func WithBuildToolIdentifier(tool string) BuildOption {
	return func(c *buildConfig) error {
		c.buildToolIdentifier = tool
		return nil
	}
}

func WithBuildToolVersion(version string) BuildOption {
	return func(c *buildConfig) error {
		c.buildToolVersion = version
		return nil
	}
}

func BuildSociIndex(ctx context.Context, cs content.Store, img images.Image, spanSize int64, store orascontent.Storage, opts ...BuildOption) (*SociIndex, error) {
	var config buildConfig
	for _, o := range opts {
		if err := o(&config); err != nil {
			return nil, err
		}
	}

	platform := platforms.Default()
	// we get manifest descriptor before calling images.Manifest, since after calling
	// images.Manifest, images.Children will error out when reading the manifest blob (this happens on containerd side)
	imgManifestDesc, err := GetImageManifestDescriptor(ctx, cs, img, platform)
	if err != nil {
		return nil, err
	}
	manifest, err := images.Manifest(ctx, cs, img.Target, platform)
	if err != nil {
		return nil, err
	}

	sociLayersDesc := make([]*ocispec.Descriptor, len(manifest.Layers))
	eg, ctx := errgroup.WithContext(ctx)
	for i, l := range manifest.Layers {
		i, l := i, l
		eg.Go(func() error {
			desc, err := buildSociLayer(ctx, cs, l, spanSize, store, &config)
			if err != nil {
				return err
			}
			sociLayersDesc[i] = desc
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return nil, err
	}

	annotations := map[string]string{
		IndexAnnotationBuildToolIdentifier: config.buildToolIdentifier,
		IndexAnnotationBuildToolVersion:    config.buildToolVersion,
	}

	sociIndex := SociIndex{
		MediaType:    sociIndexMediaType,
		ArtifactType: SociIndexArtifactType,
		Blobs:        sociLayersDesc,
		Subject: ocispec.Descriptor{
			MediaType:   imgManifestDesc.MediaType,
			Digest:      imgManifestDesc.Digest,
			Size:        imgManifestDesc.Size,
			Annotations: imgManifestDesc.Annotations,
		},
		Annotations: annotations,
	}
	return &sociIndex, nil
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

// buildSociLayer builds the ztoc for an image layer and returns a Descriptor for the new ztoc.
func buildSociLayer(ctx context.Context, cs content.Store, desc ocispec.Descriptor, spanSize int64, store orascontent.Storage, cfg *buildConfig) (*ocispec.Descriptor, error) {
	if !images.IsLayerType(desc.MediaType) {
		return nil, errNotLayerType
	}
	// check if we need to skip building the zTOC
	if skipBuildingZtoc(desc, cfg) {
		fmt.Printf("layer %s -> ztoc skipped\n", desc.Digest)
		return nil, nil
	}
	ra, err := cs.ReaderAt(ctx, desc)
	if err != nil {
		return nil, err
	}
	defer ra.Close()
	sr := io.NewSectionReader(ra, 0, desc.Size)

	tmpFile, err := os.CreateTemp("", "tmp.*")
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmpFile.Name())
	n, err := io.Copy(tmpFile, sr)
	if err != nil {
		return nil, err
	}
	if n != desc.Size {
		return nil, errors.New("the size of the temp file doesn't match that of the layer")
	}

	ztoc, err := BuildZtoc(tmpFile.Name(), spanSize, cfg)
	if err != nil {
		return nil, err
	}

	ztocReader, ztocDesc, err := NewZtocReader(ztoc)
	if err != nil {
		return nil, err
	}

	err = store.Push(ctx, ztocDesc, ztocReader)
	if err != nil && !errors.Is(err, errdef.ErrAlreadyExists) {
		return nil, fmt.Errorf("cannot push ztoc to local store: %w", err)
	}

	// write the artifact entry for soci layer
	// this part is needed for local store only
	entry := &ArtifactEntry{
		Size:           ztocDesc.Size,
		Digest:         ztocDesc.Digest.String(),
		OriginalDigest: desc.Digest.String(),
		Type:           ArtifactEntryTypeLayer,
		Location:       desc.Digest.String(),
	}
	err = writeArtifactEntry(entry)
	if err != nil {
		return nil, err
	}

	fmt.Printf("layer %s -> ztoc %s\n", desc.Digest, ztocDesc.Digest)

	ztocDesc.MediaType = SociLayerMediaType
	ztocDesc.Annotations = map[string]string{
		IndexAnnotationImageLayerMediaType: ocispec.MediaTypeImageLayerGzip,
		IndexAnnotationImageLayerDigest:    desc.Digest.String(),
	}
	return &ztocDesc, err
}

// getImageManifestDescriptor gets the descriptor of image manifest
func GetImageManifestDescriptor(ctx context.Context, cs content.Store, img images.Image, platform platforms.MatchComparer) (*ocispec.Descriptor, error) {
	target := img.Target
	if images.IsIndexType(target.MediaType) {
		manifests, err := images.Children(ctx, cs, target)
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
	} else if images.IsManifestType(target.MediaType) {
		return &target, nil
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

	// this entry is persisted to be used by cli push
	entry := &ArtifactEntry{
		Digest:         dgst.String(),
		OriginalDigest: indexWithMetadata.Index.Subject.Digest.String(),
		ImageDigest:    indexWithMetadata.ImageDigest.String(),
		Platform:       platforms.Format(indexWithMetadata.Platform),
		Type:           ArtifactEntryTypeIndex,
		Location:       indexWithMetadata.Index.Subject.Digest.String(),
		Size:           size,
	}
	return writeArtifactEntry(entry)
}

func ReadSociIndex(ctx context.Context, sociDigest digest.Digest, store orascontent.Storage) (*SociIndex, error) {
	reader, err := store.Fetch(ctx, ocispec.Descriptor{Digest: sociDigest})
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	parser := json.NewDecoder(reader)
	sociIndex := SociIndex{}

	if err = parser.Decode(&sociIndex); err != nil {
		return nil, fmt.Errorf("cannot decode the file: %w", err)
	}

	return &sociIndex, nil
}
