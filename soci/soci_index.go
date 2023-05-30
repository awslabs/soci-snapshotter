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
	"os"
	"sync"
	"time"

	"github.com/awslabs/soci-snapshotter/fs/config"
	"github.com/awslabs/soci-snapshotter/ztoc"
	"github.com/awslabs/soci-snapshotter/ztoc/compression"
	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/platforms"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"

	orascontent "oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/content/oci"
	"oras.land/oras-go/v2/errdef"
)

const (
	// SociIndexArtifactType is the artifactType of index SOCI index
	SociIndexArtifactType = "application/vnd.amazon.soci.index.v1+json"
	// SociLayerMediaType is the mediaType of ztoc
	SociLayerMediaType = "application/octet-stream"
	// IndexAnnotationImageLayerMediaType is the index annotation for image layer media type
	IndexAnnotationImageLayerMediaType = "com.amazon.soci.image-layer-mediaType"
	// IndexAnnotationImageLayerDigest is the index annotation for image layer digest
	IndexAnnotationImageLayerDigest = "com.amazon.soci.image-layer-digest"
	// IndexAnnotationBuildToolIdentifier is the index annotation for build tool identifier
	IndexAnnotationBuildToolIdentifier = "com.amazon.soci.build-tool-identifier"

	defaultSpanSize            = int64(1 << 22) // 4MiB
	defaultMinLayerSize        = 10 << 20       // 10MiB
	defaultBuildToolIdentifier = "AWS SOCI CLI v0.1"
	// emptyJSONObjectDigest is the digest of the content "{}".
	emptyJSONObjectDigest = "sha256:44136fa355b3678a1146ad16f7e8649e94fb4fc21fe77e8310c060f61caaff8a"
)

var (
	errNotLayerType           = errors.New("not a layer mediaType")
	errUnsupportedLayerFormat = errors.New("unsupported layer format")
	// defaultConfigContent is the content of the config object used when serializing
	// a SOCI index as an OCI 1.0 Manifest for fallback compatibility. OCI 1.0 Manifests
	// require a non-empty config object, so we use the empty JSON object. The content of
	// the config is never used by SOCI, but it is validated by registries.
	defaultConfigContent = []byte("{}")
	// defaultConfigDescriptor is the descriptor of the of the config object used when
	// serializing a SOCI index as an OCI 1.0 Manifest for fallback compatibility.
	defaultConfigDescriptor = ocispec.Descriptor{
		// The Config's media type is set to `SociIndexArtifactType` so that the oras-go
		// library can use it to filter artifacts.
		MediaType: SociIndexArtifactType,
		Digest:    emptyJSONObjectDigest,
		Size:      2,
	}
)

// Index represents a SOCI index manifest.
type Index struct {
	// MediaType represents the type of document into which the SOCI index manifest will be serialized
	MediaType string `json:"mediaType"`

	// Artifact type is the media type of the SOCI index itself.
	ArtifactType string `json:"artifactType"`

	// Blobs are descriptors for the zTOCs in the index.
	Blobs []ocispec.Descriptor `json:"blobs,omitempty"`

	// Subject is the descriptor for the resource to which the index applies.
	Subject *ocispec.Descriptor `json:"subject,omitempty"`

	// Annotations are optional additional metadata for the index.
	Annotations map[string]string `json:"annotations,omitempty"`
}

// IndexWithMetadata has a soci `Index` and its metadata.
type IndexWithMetadata struct {
	Index       *Index
	Platform    *ocispec.Platform
	ImageDigest digest.Digest
	CreatedAt   time.Time
}

// IndexDescriptorInfo has a soci index descriptor and additional metadata.
type IndexDescriptorInfo struct {
	ocispec.Descriptor
	CreatedAt time.Time
}

// DecodeIndex deserializes a JSON blob in an io.Reader
// into a SOCI index. The blob is an OCI 1.0 Manifest
func DecodeIndex(r io.Reader, index *Index) error {
	b, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	return UnmarshalIndex(b, index)
}

// UnmarshalIndex deserializes a JSON blob in a byte array
// into a SOCI index. The blob is an OCI 1.0 Manifest
func UnmarshalIndex(b []byte, index *Index) error {
	if err := json.Unmarshal(b, index); err != nil {
		return err
	}

	var manifest ocispec.Manifest
	if err := json.Unmarshal(b, &manifest); err != nil {
		return err
	}

	fromManifest(manifest, index)
	return nil
}

// fromManifest converts an OCI 1.0 Manifest to a SOCI Index
func fromManifest(manifest ocispec.Manifest, index *Index) {
	index.MediaType = manifest.MediaType
	index.ArtifactType = SociIndexArtifactType
	index.Blobs = manifest.Layers
	index.Subject = manifest.Subject
	index.Annotations = manifest.Annotations
}

// MarshalIndex serializes a SOCI index into a JSON blob.
// The JSON blob is an OCI 1.0 Manifest
func MarshalIndex(i *Index) ([]byte, error) {
	var manifest ocispec.Manifest
	manifest.SchemaVersion = 2
	manifest.MediaType = ocispec.MediaTypeImageManifest
	manifest.Config = defaultConfigDescriptor
	manifest.Layers = i.Blobs
	manifest.Subject = i.Subject
	manifest.Annotations = i.Annotations
	return json.Marshal(manifest)
}

// GetIndexDescriptorCollection returns all `IndexDescriptorInfo` of the given image and platforms.
func GetIndexDescriptorCollection(ctx context.Context, cs content.Store, artifactsDb *ArtifactsDb, img images.Image, ps []ocispec.Platform) ([]IndexDescriptorInfo, *ocispec.Descriptor, error) {
	var (
		descriptors []IndexDescriptorInfo
		entries     []ArtifactEntry
		indexDesc   *ocispec.Descriptor
		err         error
	)
	for _, platform := range ps {
		indexDesc, err = GetImageManifestDescriptor(ctx, cs, img.Target, platforms.OnlyStrict(platform))
		if err != nil {
			return nil, nil, err
		}
		e, err := artifactsDb.getIndexArtifactEntries(indexDesc.Digest.String())
		if err != nil {
			return nil, nil, err
		}
		entries = append(entries, e...)
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
			CreatedAt:  entry.CreatedAt,
		})
	}

	return descriptors, indexDesc, nil
}

type buildConfig struct {
	spanSize            int64
	minLayerSize        int64
	buildToolIdentifier string
	artifactsDb         *ArtifactsDb
	platform            ocispec.Platform
}

// BuildOption specifies a config change to build soci indices.
type BuildOption func(c *buildConfig) error

// WithSpanSize specifies span size.
func WithSpanSize(spanSize int64) BuildOption {
	return func(c *buildConfig) error {
		c.spanSize = spanSize
		return nil
	}
}

// WithMinLayerSize specifies min layer size to build a ztoc for a layer.
func WithMinLayerSize(minLayerSize int64) BuildOption {
	return func(c *buildConfig) error {
		c.minLayerSize = minLayerSize
		return nil
	}
}

// WithBuildToolIdentifier specifies the build tool annotation value.
func WithBuildToolIdentifier(tool string) BuildOption {
	return func(c *buildConfig) error {
		c.buildToolIdentifier = tool
		return nil
	}
}

// WithPlatform specifies platform used to build soci indices.
func WithPlatform(platform ocispec.Platform) BuildOption {
	return func(c *buildConfig) error {
		c.platform = platform
		return nil
	}
}

// WithArtifactsDb speicifies the artifacts database
func WithArtifactsDb(db *ArtifactsDb) BuildOption {
	return func(c *buildConfig) error {
		c.artifactsDb = db
		return nil
	}
}

// IndexBuilder creates soci indices.
type IndexBuilder struct {
	contentStore content.Store
	blobStore    orascontent.Storage
	ArtifactsDb  *ArtifactsDb
	config       *buildConfig
	ztocBuilder  *ztoc.Builder
}

// NewIndexBuilder returns an `IndexBuilder` that is used to create soci indices.
func NewIndexBuilder(contentStore content.Store, blobStore orascontent.Storage, artifactsDb *ArtifactsDb, opts ...BuildOption) (*IndexBuilder, error) {
	defaultPlatform := platforms.DefaultSpec()
	config := &buildConfig{
		spanSize:            defaultSpanSize,
		minLayerSize:        defaultMinLayerSize,
		buildToolIdentifier: defaultBuildToolIdentifier,
		platform:            defaultPlatform,
	}

	for _, opt := range opts {
		if err := opt(config); err != nil {
			return nil, err
		}
	}

	return &IndexBuilder{
		contentStore: contentStore,
		blobStore:    blobStore,
		ArtifactsDb:  artifactsDb,
		config:       config,
		ztocBuilder:  ztoc.NewBuilder(config.buildToolIdentifier),
	}, nil
}

// Build builds a soci index for `img` and return the index with metadata.
func (b *IndexBuilder) Build(ctx context.Context, img images.Image) (*IndexWithMetadata, error) {
	// we get manifest descriptor before calling images.Manifest, since after calling
	// images.Manifest, images.Children will error out when reading the manifest blob (this happens on containerd side)
	imgManifestDesc, err := GetImageManifestDescriptor(ctx, b.contentStore, img.Target, platforms.OnlyStrict(b.config.platform))
	if err != nil {
		return nil, err
	}
	manifest, err := images.Manifest(ctx, b.contentStore, img.Target, platforms.OnlyStrict(b.config.platform))

	if err != nil {
		return nil, err
	}

	// attempt to build a ztoc for each layer
	sociLayersDesc := make([]*ocispec.Descriptor, len(manifest.Layers))
	errChan := make(chan error)
	go func() {
		var wg sync.WaitGroup
		for i, l := range manifest.Layers {
			wg.Add(1)
			go func(i int, l ocispec.Descriptor) {
				defer wg.Done()
				desc, err := b.buildSociLayer(ctx, l)
				if err != nil {
					if err != errUnsupportedLayerFormat {
						errChan <- err
					}
					return
				}
				if desc != nil {
					// index layers must be in some deterministic order
					// actual layer order used for historic consistency
					sociLayersDesc[i] = desc
				}
			}(i, l)
		}
		wg.Wait()
		close(errChan)
	}()

	errs := make([]error, 0, len(manifest.Layers))

	for err := range errChan {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		errWrap := errors.New("errors encountered while building soci layers")
		for _, err := range errs {
			errWrap = fmt.Errorf("%w; %v", errWrap, err)
		}

		return nil, errWrap
	}

	ztocsDesc := make([]ocispec.Descriptor, 0, len(sociLayersDesc))
	for _, desc := range sociLayersDesc {
		if desc != nil {
			ztocsDesc = append(ztocsDesc, *desc)
		}
	}

	if len(ztocsDesc) == 0 {
		return nil, errors.New("no ztocs created, all layers either skipped or produced errors")
	}

	annotations := map[string]string{
		IndexAnnotationBuildToolIdentifier: b.config.buildToolIdentifier,
	}

	refers := &ocispec.Descriptor{
		MediaType: imgManifestDesc.MediaType,
		Digest:    imgManifestDesc.Digest,
		Size:      imgManifestDesc.Size,
	}

	index := NewIndex(ztocsDesc, refers, annotations)
	return &IndexWithMetadata{
		Index:       index,
		Platform:    &b.config.platform,
		ImageDigest: img.Target.Digest,
		CreatedAt:   time.Now(),
	}, nil
}

// buildSociLayer builds a ztoc for an image layer (`desc`) and returns ztoc descriptor.
// It may skip building ztoc (e.g., if layer size < `minLayerSize`) and return nil.
func (b *IndexBuilder) buildSociLayer(ctx context.Context, desc ocispec.Descriptor) (*ocispec.Descriptor, error) {
	if !images.IsLayerType(desc.MediaType) {
		return nil, errNotLayerType
	}
	// check if we need to skip building the zTOC
	if skip, reason := skipBuildingZtoc(desc, b.config); skip {
		fmt.Printf("ztoc skipped - layer %s (%s) %s\n", desc.Digest, desc.MediaType, reason)
		return nil, nil
	}

	compressionAlgo, err := images.DiffCompression(ctx, desc.MediaType)
	if err != nil {
		return nil, fmt.Errorf("could not determine layer compression: %w", err)
	}

	if compressionAlgo == "" {
		switch desc.MediaType {
		case ocispec.MediaTypeImageLayer:
			// for OCI image layers, empty is returned for an uncompressed layer.
			compressionAlgo = compression.Uncompressed
		}
	}

	if !b.ztocBuilder.CheckCompressionAlgorithm(compressionAlgo) {
		fmt.Printf("ztoc skipped - layer %s (%s) is compressed in an unsupported format. expect: [tar, gzip, unknown] but got %q\n",
			desc.Digest, desc.MediaType, compressionAlgo)
		return nil, errUnsupportedLayerFormat
	}

	ra, err := b.contentStore.ReaderAt(ctx, desc)
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

	toc, err := b.ztocBuilder.BuildZtoc(tmpFile.Name(), b.config.spanSize, ztoc.WithCompression(compressionAlgo))
	if err != nil {
		return nil, err
	}

	ztocReader, ztocDesc, err := ztoc.Marshal(toc)
	if err != nil {
		return nil, err
	}

	err = b.blobStore.Push(ctx, ztocDesc, ztocReader)
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
		CreatedAt:      time.Now(),
	}
	err = b.ArtifactsDb.WriteArtifactEntry(entry)
	if err != nil {
		return nil, err
	}

	fmt.Printf("layer %s -> ztoc %s\n", desc.Digest, ztocDesc.Digest)

	ztocDesc.MediaType = SociLayerMediaType
	ztocDesc.Annotations = map[string]string{
		IndexAnnotationImageLayerMediaType: desc.MediaType,
		IndexAnnotationImageLayerDigest:    desc.Digest.String(),
	}
	return &ztocDesc, err
}

// NewIndex returns a new index.
func NewIndex(blobs []ocispec.Descriptor, subject *ocispec.Descriptor, annotations map[string]string) *Index {
	return &Index{
		Blobs:        blobs,
		ArtifactType: SociIndexArtifactType,
		Annotations:  annotations,
		Subject:      subject,
		MediaType:    ocispec.MediaTypeImageManifest,
	}
}

// NewIndexFromReader returns a new index from a Reader.
func NewIndexFromReader(reader io.Reader) (*Index, error) {
	index := new(Index)
	if err := json.NewDecoder(reader).Decode(index); err != nil {
		return nil, fmt.Errorf("unable to decode reader into index: %v", err)
	}
	return index, nil
}

func skipBuildingZtoc(desc ocispec.Descriptor, cfg *buildConfig) (bool, string) {
	if cfg == nil {
		return false, ""
	}
	// avoid the file access if the layer size is below threshold
	if desc.Size < cfg.minLayerSize {
		return true, fmt.Sprintf("size %d is less than min-layer-size %d", desc.Size, cfg.minLayerSize)
	}
	return false, ""
}

// GetImageManifestDescriptor gets the descriptor of image manifest
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

// WriteSociIndex writes the SociIndex manifest to oras `store`.
func WriteSociIndex(ctx context.Context, indexWithMetadata *IndexWithMetadata, store orascontent.Storage, artifactsDb *ArtifactsDb) error {
	manifest, err := MarshalIndex(indexWithMetadata.Index)
	if err != nil {
		return err
	}

	// If we're serializing the SOCI index as an OCI 1.0 Manifest, create an
	// empty config objct in the store as well. We will need to push this to the
	// registry later.
	if indexWithMetadata.Index.MediaType == ocispec.MediaTypeImageManifest {
		err = store.Push(ctx, defaultConfigDescriptor, bytes.NewReader(defaultConfigContent))
		if err != nil && !errors.Is(err, errdef.ErrAlreadyExists) {
			return fmt.Errorf("error creating OCI 1.0 empty config: %w", err)
		}
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
		Platform:       platforms.Format(*indexWithMetadata.Platform),
		Type:           ArtifactEntryTypeIndex,
		Location:       refers.Digest.String(),
		Size:           size,
		MediaType:      indexWithMetadata.Index.MediaType,
		CreatedAt:      indexWithMetadata.CreatedAt,
	}
	return artifactsDb.WriteArtifactEntry(entry)
}

// FetchIndex takes a digest string and returns the soci.Index with that digest
func FetchIndex(ctx context.Context, storage *oci.Store, digestString string) (*Index, error) {
	dgst, err := digest.Parse(digestString)
	if err != nil {
		return nil, err
	}
	reader, err := storage.Fetch(ctx, ocispec.Descriptor{Digest: dgst})
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	var index Index
	if err := DecodeIndex(reader, &index); err != nil {
		return nil, err
	}

	return &index, nil
}

// RemoveIndex takes a digest string and removes that index manifest from the artifact db and content store
// It also optionally returns a list of zTOCs referenced by the removed index manifest
func RemoveIndex(ctx context.Context, storage *oci.Store, digestString string, returnReferencedZtocs bool) (*map[digest.Digest]bool, error) {
	index, err := FetchIndex(ctx, storage, digestString)
	if err != nil {
		return nil, err
	}

	var referencedZtocDigests map[digest.Digest]bool
	if returnReferencedZtocs {
		referencedZtocDigests = make(map[digest.Digest]bool)
		for _, blob := range index.Blobs {
			referencedZtocDigests[blob.Digest] = true
		}
	}

	// TODO remove the index manifest file

	// remove the index manifest artifact
	err = db.RemoveArtifactEntryByIndexDigest(digestString)
	if err != nil {
		return nil, err
	}

	return &referencedZtocDigests, nil
}

func RemoveIndexes(ctx context.Context, digestStrings []string, removeOrphanedZtocs bool) error {
	storage, err := oci.New(config.SociContentStorePath)
	if err != nil {
		return err
	}

	maybeOrphanZtocDigests := make(map[digest.Digest]bool)
	for _, desc := range digestStrings {
		referencedZtocDigests, err := RemoveIndex(ctx, storage, desc, removeOrphanedZtocs)
		if err != nil {
			return err
		}
		for k := range *referencedZtocDigests {
			maybeOrphanZtocDigests[k] = true
		}
	}

	if removeOrphanedZtocs {
		err = db.Walk(func(ae *ArtifactEntry) error {
			if ae.Type == ArtifactEntryTypeIndex {
				index, err := FetchIndex(ctx, storage, ae.Digest)
				if err != nil {
					return err
				}

				// remove still-referenced ztocs from the list of potentially orphaned ztoc digests
				for _, blob := range index.Blobs {
					// FIXME why does go-staticcheck think this guard is unnecessary for just {delete()}?
					if _, ok := maybeOrphanZtocDigests[blob.Digest]; ok {
						fmt.Printf("keeping ztoc digest %s referenced by index manifest %s\n", blob.Digest.String(), ae.Digest)
						delete(maybeOrphanZtocDigests, blob.Digest)
						// bail out early if there are no more potential orphans
						if len(maybeOrphanZtocDigests) == 0 {
							return ErrWalkBailout
						}
					}
				}
			}
			return nil
		})
		if err != nil && !errors.Is(err, ErrWalkBailout) {
			return err
		}

		// all remaining potential orphans are actually orphaned
		for dgst := range maybeOrphanZtocDigests {
			fmt.Printf("removing orphaned ztoc digest %s\n", dgst.String())
			err = db.RemoveArtifactEntryByZtocDigest(dgst.String())
			if err != nil {
				return err
			}
		}
	}

	return nil
}
