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
	// index annotation for build tool version
	IndexAnnotationBuildToolVersion = "com.amazon.soci.build-tool-version"
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

	// Optional reference to manifest to refer to
	// ORAS and OCI Artifact have different names for the field
	// During deserialization, the appropriate field is filled.

	// For ORAS
	Subject *ocispec.Descriptor `json:"subject,omitempty"`

	// For OCI Artifact
	Refers *ocispec.Descriptor `json:"refers,omitempty"`

	// Optional annotations in the manifest
	Annotations map[string]string `json:"annotations,omitempty"`
}

func (i *Index) refers() *ocispec.Descriptor {
	switch i.MediaType {
	case ORASManifestMediaType:
		return i.Subject
	case OCIArtifactManifestMediaType:
		return i.Refers
	}
	return nil
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
	buildToolVersion    string
	manifestType        ManifestType
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

func WithManifestType(val ManifestType) BuildOption {
	return func(c *buildConfig) error {
		c.manifestType = val
		return nil
	}
}

func BuildSociIndex(ctx context.Context, cs content.Store, img images.Image, spanSize int64, store orascontent.Storage, opts ...BuildOption) (*Index, error) {
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
				return fmt.Errorf("could not build zTOC for %s: %w", l.Digest.String(), err)
			}
			sociLayersDesc[i] = desc
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return nil, err
	}

	ztocsDesc := make([]ocispec.Descriptor, 0, len(manifest.Layers))
	for _, desc := range sociLayersDesc {
		if desc != nil {
			ztocsDesc = append(ztocsDesc, *desc)
		}
	}

	annotations := map[string]string{
		IndexAnnotationBuildToolIdentifier: config.buildToolIdentifier,
		IndexAnnotationBuildToolVersion:    config.buildToolVersion,
	}

	refers := &ocispec.Descriptor{
		MediaType:   imgManifestDesc.MediaType,
		Digest:      imgManifestDesc.Digest,
		Size:        imgManifestDesc.Size,
		Annotations: imgManifestDesc.Annotations,
	}
	return NewIndex(ztocsDesc, refers, annotations, config.manifestType), nil
}

// Returns a new index.
func NewIndex(blobs []ocispec.Descriptor, subject *ocispec.Descriptor, annotations map[string]string, manifestType ManifestType) *Index {
	if manifestType == ManifestOCIArtifact {
		return newOCIArtifactManifest(blobs, subject, annotations)
	}
	return newORASManifest(blobs, subject, annotations)
}

func newOCIArtifactManifest(blobs []ocispec.Descriptor, subject *ocispec.Descriptor, annotations map[string]string) *Index {
	return &Index{
		Blobs:        blobs,
		ArtifactType: SociIndexArtifactType,
		Annotations:  annotations,
		Refers:       subject,
		MediaType:    OCIArtifactManifestMediaType,
	}
}

func newORASManifest(blobs []ocispec.Descriptor, subject *ocispec.Descriptor, annotations map[string]string) *Index {
	return &Index{
		Blobs:        blobs,
		ArtifactType: SociIndexArtifactType,
		Annotations:  annotations,
		Subject:      subject,
		MediaType:    ORASManifestMediaType,
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
	compression, err := images.DiffCompression(ctx, desc.MediaType)
	if err != nil {
		return nil, fmt.Errorf("could not determine layer compression: %w", err)
	}
	if compression != "gzip" {
		return nil, fmt.Errorf("layer %s (%s) must be compressed by gzip, but got %q: %w",
			desc.Digest, desc.MediaType, compression, errUnsupportedLayerFormat)
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
		MediaType:      SociLayerMediaType,
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

	refers := indexWithMetadata.Index.refers()

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
