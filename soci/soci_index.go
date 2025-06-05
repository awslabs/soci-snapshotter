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
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/awslabs/soci-snapshotter/config"
	"github.com/awslabs/soci-snapshotter/soci/store"
	"github.com/awslabs/soci-snapshotter/ztoc"
	"github.com/awslabs/soci-snapshotter/ztoc/compression"
	"github.com/containerd/containerd/content"
	"github.com/containerd/errdefs"
	"oras.land/oras-go/v2/errdef"

	"github.com/containerd/containerd/images"
	"github.com/containerd/log"
	"github.com/containerd/platforms"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

const (
	// SociIndexArtifactType is the artifactType of index SOCI index
	SociIndexArtifactType = SociIndexArtifactTypeV1
	// SociIndexArtifactTypeV1 is the artifact type of a v1 SOCI index which
	// uses the subject field and the OCI referrers API
	SociIndexArtifactTypeV1 = "application/vnd.amazon.soci.index.v1+json"
	// SociIndexArtifactTypeV2 is the artifact type of a v2 SOCI index which
	// does not contain a subject and instead maintains a reference via an annotation on an image manifest
	SociIndexArtifactTypeV2 = "application/vnd.amazon.soci.index.v2+json"
	// SociLayerMediaType is the mediaType of ztoc
	SociLayerMediaType = "application/octet-stream"
	// IndexAnnotationImageLayerMediaType is the index annotation for image layer media type
	IndexAnnotationImageLayerMediaType = "com.amazon.soci.image-layer-mediaType"
	// IndexAnnotationImageLayerDigest is the index annotation for image layer digest
	IndexAnnotationImageLayerDigest = "com.amazon.soci.image-layer-digest"
	// IndexAnnotationBuildToolIdentifier is the index annotation for build tool identifier
	IndexAnnotationBuildToolIdentifier = "com.amazon.soci.build-tool-identifier"
	// IndexAnnotationDisableXAttrs is the index annotation if the layer has
	// extended attributes
	IndexAnnotationDisableXAttrs = "com.amazon.soci.disable-xattrs"
	// IndexAnnotationImageManifestDigest is the annotation to indicate the digest
	// of the associated image manifest. This is useful for v2 SOCI indexes which do not contain
	// a subject field. This annotation goes on a SOCI index descriptor in an OCI index,
	// not in the SOCI index itself.
	IndexAnnotationImageManifestDigest = "com.amazon.soci.image-manifest-digest"

	// ImageAnnotationSociIndexDigest is an annotation on image manifests to specify
	// a SOCI index digest for the image.
	ImageAnnotationSociIndexDigest = "com.amazon.soci.index-digest"

	defaultSpanSize            = int64(1 << 22) // 4MiB
	defaultMinLayerSize        = 10 << 20       // 10MiB
	defaultBuildToolIdentifier = "AWS SOCI CLI v0.2"
	// emptyJSONObjectDigest is the digest of the content "{}".
	emptyJSONObjectDigest = "sha256:44136fa355b3678a1146ad16f7e8649e94fb4fc21fe77e8310c060f61caaff8a"
	// whiteoutOpaqueDir is a special file that indicates that a directory is opaque and all files and subdirectories
	// should be hidden. See https://github.com/opencontainers/image-spec/blob/v1.1.0-rc5/layer.md#whiteouts
	// [soci-snapshotter] NOTE: this is a duplicate of fs/layer/layer.go so that the SOCI package and the snapshotter can
	// be independent (and SOCI could be split out into it's own module in the future).
	whiteoutOpaqueDir = ".wh..wh..opq"
	disableXAttrsTrue = "true"
)

var (
	errNotLayerType           = errors.New("not a layer mediaType")
	errUnsupportedLayerFormat = errors.New("unsupported layer format")
	ErrEmptyIndex             = errors.New("no ztocs created, all layers either skipped or produced errors")
	// defaultConfigContent is the content of the config object used when serializing
	// a SOCI index as an OCI 1.0 Manifest for fallback compatibility. OCI 1.0 Manifests
	// require a non-empty config object, so we use the empty JSON object. The content of
	// the config is never used by SOCI, but it is validated by registries.
	defaultConfigContent = []byte("{}")
	// defaultConfigDescriptor is the descriptor of the of the config object used when
	// serializing a SOCI index as an OCI 1.0 Manifest for fallback compatibility.
	defaultConfigDescriptor = defaultConfigDescriptorV1
	// defaultConfigDescriptorV1 is the descriptor of the of the config object used when
	// serializing a v1 SOCI index as an OCI 1.0 Manifest for fallback compatibility.
	defaultConfigDescriptorV1 = ocispec.Descriptor{
		// The Config's media type is set to `SociIndexArtifactType` so that the oras-go
		// library can use it to filter artifacts.
		MediaType: SociIndexArtifactTypeV1,
		Digest:    emptyJSONObjectDigest,
		Size:      2,
	}

	// defaultConfigDescriptorV1 is the descriptor of the of the config object used when
	// serializing a v2 SOCI index as an OCI 1.0 Manifest for fallback compatibility.
	defaultConfigDescriptorV2 = ocispec.Descriptor{
		// The Config's media type is set to `SociIndexArtifactType` so that the oras-go
		// library can use it to filter artifacts.
		MediaType: SociIndexArtifactTypeV2,
		Digest:    emptyJSONObjectDigest,
		Size:      2,
	}
)

// IndexVersion represents the version of an index created by the index builder
type IndexVersion struct {
	version          string
	artifactType     string
	configDescriptor ocispec.Descriptor
}

var (
	V1 = IndexVersion{
		version:          "v1",
		artifactType:     SociIndexArtifactTypeV1,
		configDescriptor: defaultConfigDescriptorV1,
	}
	V2 = IndexVersion{
		version:          "v2",
		artifactType:     SociIndexArtifactTypeV2,
		configDescriptor: defaultConfigDescriptorV2,
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

	Config ocispec.Descriptor `json:"-"`
}

// IndexWithMetadata has a soci `Index` and its metadata.
type IndexWithMetadata struct {
	// Index is the SOCI index itself
	Index *Index
	// Desc is the descriptor for the serialized SOCI index
	Desc ocispec.Descriptor
	// Platform is the platform for the SOCI index
	Platform *ocispec.Platform
	// ImageDesc is the descriptor of the original image used
	// to create the SOCI index. This could either be a reference to an image
	// manifest in the case of single-platorm images or an OCI Index/Docker Manifest List
	// in the case of multi-platform images. This descriptor is intended for mapping
	// the SOCI index to a particular image ref, but not necessarily a specific platform.
	ImageDesc ocispec.Descriptor
	// ManifestDesc is the descriptor of the original image manifest used
	// to create the SOCI index. This is the same as the ImageDesc for single-platform images,
	// but not for multiplatform images. This descriptor always maps to the platform-specific image
	// manifest that was used when creating the SOCI index. For SOCI v1 indexes, this is the same
	// as the Subject. For SOCI v2 indexes, this is used in place of the subject.
	ManifestDesc ocispec.Descriptor
	CreatedAt    time.Time
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

	return fromManifest(manifest, index)
}

// fromManifest converts an OCI 1.0 Manifest to a SOCI Index
func fromManifest(manifest ocispec.Manifest, index *Index) error {
	index.MediaType = manifest.MediaType
	switch manifest.Config.MediaType {
	case V1.artifactType:
		index.ArtifactType = V1.artifactType
	case V2.artifactType:
		index.ArtifactType = V2.artifactType
	default:
		return fmt.Errorf("unknown index version: %s", manifest.Config.MediaType)
	}
	index.Blobs = manifest.Layers
	index.Subject = manifest.Subject
	index.Annotations = manifest.Annotations
	index.Config = manifest.Config
	return nil
}

// MarshalIndex serializes a SOCI index into a JSON blob.
// The JSON blob is an OCI 1.0 Manifest
func MarshalIndex(i *Index) ([]byte, error) {
	var manifest ocispec.Manifest
	manifest.SchemaVersion = 2
	manifest.MediaType = ocispec.MediaTypeImageManifest
	manifest.Config = i.Config
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
			MediaType:    entry.MediaType,
			ArtifactType: entry.ArtifactType,
			Digest:       dgst,
			Size:         entry.Size,
		}
		descriptors = append(descriptors, IndexDescriptorInfo{
			Descriptor: desc,
			CreatedAt:  entry.CreatedAt,
		})
	}

	return descriptors, indexDesc, nil
}

type builderConfig struct {
	spanSize            int64
	minLayerSize        int64
	buildToolIdentifier string
	artifactsDb         *ArtifactsDb
	optimizations       []Optimization
}

func (b *builderConfig) hasOptimization(o Optimization) bool {
	for _, optimization := range b.optimizations {
		if o == optimization {
			return true
		}
	}
	return false
}

// Optimization represents an optional optimization to be applied when building the SOCI index
type Optimization string

const (
	// XAttrOptimization optimizes xattrs by disabling them for layers where there are no xattrs or opaque directories
	XAttrOptimization Optimization = "xattr"
	// Be sure to add any new optimizations to `Optimizations` below
)

// Optimizations contains the list of all known optimizations
var Optimizations = []Optimization{XAttrOptimization}

// ParseOptimization parses a string into a known optimization.
// If the string does not match a known optimization, an error is returned.
func ParseOptimization(s string) (Optimization, error) {
	for _, optimization := range Optimizations {
		if s == string(optimization) {
			return optimization, nil
		}
	}
	return "", fmt.Errorf("optimization %s is not a valid optimization %v", s, Optimizations)
}

// BuilderOption is a functional argument that affects a SOCI index builder
// and all indexes built with that builder.
type BuilderOption func(c *builderConfig) error

// WithSpanSize specifies span size.
func WithSpanSize(spanSize int64) BuilderOption {
	return func(c *builderConfig) error {
		if spanSize < 0 {
			return fmt.Errorf("span size must be >= 0")
		}
		c.spanSize = spanSize
		return nil
	}
}

// WithMinLayerSize specifies min layer size to build a ztoc for a layer.
func WithMinLayerSize(minLayerSize int64) BuilderOption {
	return func(c *builderConfig) error {
		if minLayerSize < 0 {
			return fmt.Errorf("min layer size must be >= 0")
		}
		c.minLayerSize = minLayerSize
		return nil
	}
}

// WithOptimizations enables optional optimizations when building the SOCI Index (experimental)
func WithOptimizations(optimizations []Optimization) BuilderOption {
	return func(c *builderConfig) error {
		c.optimizations = optimizations
		return nil
	}
}

// WithBuildToolIdentifier specifies the build tool annotation value.
func WithBuildToolIdentifier(tool string) BuilderOption {
	return func(c *builderConfig) error {
		c.buildToolIdentifier = tool
		return nil
	}
}

// WithArtifactsDb specifies the artifacts database
func WithArtifactsDb(db *ArtifactsDb) BuilderOption {
	return func(c *builderConfig) error {
		c.artifactsDb = db
		return nil
	}
}

// BuildOption is a functional argument that affects a single SOCI Index build.
type BuildOption func(*buildConfig) error

// buildConfig represents the config for a single index build operation.
type buildConfig struct {
	platform     ocispec.Platform
	gcRoot       bool
	indexVersion IndexVersion
}

// WithPlatform sets the platform for a single build operation.
func WithPlatform(platform ocispec.Platform) BuildOption {
	return func(bc *buildConfig) error {
		bc.platform = platform
		return nil
	}
}

// WithNoGarbageCollectionLabel prevents the index builder from putting
// a root GC label on the soci index. The builder will set content GC labels
// to prevent the ztocs from being garbage collected.
//
// The caller is responsible for putting appropriate GC labels to prevent the
// index from being garbage collected. The caller is also responsible for
// ensuring that the SOCI index does not get garbage collected after the build finishes,
// but before the GC label is applied. This can be done by calling `contentStore.BatchOpen`
// before calling `Build`.
func WithNoGarbageCollectionLabel() BuildOption {
	return func(bc *buildConfig) error {
		bc.gcRoot = false
		return nil
	}
}

// withIndexVersion controls which version of a SOCI index to create in the build.
// This is set internally by the `Build` and `Convert` APIs and should not
// be available to external callers
func withIndexVersion(v IndexVersion) BuildOption {
	return func(bc *buildConfig) error {
		bc.indexVersion = v
		return nil
	}
}

// IndexBuilder creates soci indices.
type IndexBuilder struct {
	contentStore content.Store
	blobStore    store.Store
	config       *builderConfig
	ztocBuilder  *ztoc.Builder
}

// NewIndexBuilder returns an `IndexBuilder` that is used to create soci indices.
func NewIndexBuilder(contentStore content.Store, blobStore store.Store, opts ...BuilderOption) (*IndexBuilder, error) {
	cfg := &builderConfig{
		spanSize:            defaultSpanSize,
		minLayerSize:        defaultMinLayerSize,
		buildToolIdentifier: defaultBuildToolIdentifier,
	}

	for _, opt := range opts {
		if err := opt(cfg); err != nil {
			return nil, err
		}
	}
	if cfg.artifactsDb == nil {
		var err error
		cfg.artifactsDb, err = NewDB(ArtifactsDbPath(config.DefaultSociSnapshotterRootPath))
		if err != nil {
			return nil, err
		}
	}

	return &IndexBuilder{
		contentStore: contentStore,
		blobStore:    blobStore,
		config:       cfg,
		ztocBuilder:  ztoc.NewBuilder(cfg.buildToolIdentifier),
	}, nil
}

// Build builds a soci index for `img` and pushes it with its corresponding zTOCs to the blob store.
// Returns the SOCI index and its metadata.
func (b *IndexBuilder) Build(ctx context.Context, img images.Image, opts ...BuildOption) (*IndexWithMetadata, error) {
	buildCfg := buildConfig{
		platform:     platforms.DefaultSpec(),
		gcRoot:       true,
		indexVersion: V1,
	}
	for _, opt := range opts {
		err := opt(&buildCfg)
		if err != nil {
			return nil, err
		}
	}

	// batch will prevent content from being garbage collected in the middle of the following operations
	ctx, done, err := b.blobStore.BatchOpen(ctx)
	if err != nil {
		return nil, err
	}
	defer done(ctx)

	// Create and push zTOCs to blob store
	index, err := b.build(ctx, img, buildCfg)
	if err != nil {
		return nil, err
	}

	// Label zTOCs and push SOCI index
	index.Desc, err = b.writeSociIndex(ctx, index, buildCfg.gcRoot)
	if err != nil && !errors.Is(err, errdef.ErrAlreadyExists) {
		return nil, err
	}

	return index, nil
}

// build attempts to create a zTOC in each layer and pushes the zTOC to the blob store.
// It then creates the SOCI index and returns it with some metadata.
// This should be done within a Batch and followed by writeSociIndex() to prevent garbage collection.
func (b *IndexBuilder) build(ctx context.Context, img images.Image, buildCfg buildConfig) (*IndexWithMetadata, error) {
	platformMatcher := platforms.OnlyStrict(buildCfg.platform)
	// we get manifest descriptor before calling images.Manifest, since after calling
	// images.Manifest, images.Children will error out when reading the manifest blob (this happens on containerd side)
	imgManifestDesc, err := GetImageManifestDescriptor(ctx, b.contentStore, img.Target, platformMatcher)
	if err != nil {
		if errors.Is(err, errdefs.ErrNotFound) {
			return nil, fmt.Errorf("image manifest for %s: %w", platforms.Format(buildCfg.platform), err)
		}
		return nil, err
	}
	manifest, err := images.Manifest(ctx, b.contentStore, img.Target, platformMatcher)

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
		return nil, ErrEmptyIndex
	}

	annotations := map[string]string{
		IndexAnnotationBuildToolIdentifier: b.config.buildToolIdentifier,
	}

	refers := &ocispec.Descriptor{
		MediaType: imgManifestDesc.MediaType,
		Digest:    imgManifestDesc.Digest,
		Size:      imgManifestDesc.Size,
	}
	if buildCfg.indexVersion.version == V2.version {
		refers = nil
	}

	index := NewIndex(buildCfg.indexVersion, ztocsDesc, refers, annotations)

	return &IndexWithMetadata{
		Index:        index,
		Platform:     &buildCfg.platform,
		ImageDesc:    img.Target,
		ManifestDesc: *imgManifestDesc,
		CreatedAt:    time.Now(),
	}, nil
}

// buildSociLayer builds a ztoc for an image layer (`desc`) and returns ztoc descriptor.
// It may skip building ztoc (e.g., if layer size < `minLayerSize`) and return nil.
// This should be done within a Batch and followed by Label calls to prevent garbage collection.
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
	err = b.config.artifactsDb.WriteArtifactEntry(entry)
	if err != nil {
		return nil, err
	}

	fmt.Printf("layer %s -> ztoc %s\n", desc.Digest, ztocDesc.Digest)

	ztocDesc.MediaType = SociLayerMediaType
	ztocDesc.Annotations = map[string]string{
		IndexAnnotationImageLayerMediaType: desc.MediaType,
		IndexAnnotationImageLayerDigest:    desc.Digest.String(),
	}
	b.maybeAddDisableXattrAnnotation(&ztocDesc, toc)
	return &ztocDesc, err
}

// NewIndex returns a new index.
func NewIndex(version IndexVersion, blobs []ocispec.Descriptor, subject *ocispec.Descriptor, annotations map[string]string) *Index {
	return &Index{
		Blobs:        blobs,
		ArtifactType: version.artifactType,
		Annotations:  annotations,
		Subject:      subject,
		MediaType:    ocispec.MediaTypeImageManifest,
		Config:       version.configDescriptor,
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

func skipBuildingZtoc(desc ocispec.Descriptor, cfg *builderConfig) (bool, string) {
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
		return nil, errdefs.ErrNotFound
	} else if images.IsManifestType(imageTarget.MediaType) {
		if imageTarget.Platform != nil && !platform.Match(*imageTarget.Platform) {
			return nil, errdefs.ErrNotFound
		}
		return &imageTarget, nil
	}

	return nil, nil
}

// writeSociIndex writes the SociIndex manifest to the blob store.
// This should be done within a Batch to prevent garbage collection.
func (b *IndexBuilder) writeSociIndex(ctx context.Context, indexWithMetadata *IndexWithMetadata, gcRoot bool) (ocispec.Descriptor, error) {
	manifest, err := MarshalIndex(indexWithMetadata.Index)
	if err != nil {
		return ocispec.Descriptor{}, err
	}

	// If we're serializing the SOCI index as an OCI 1.0 Manifest, create an
	// empty config objct in the store as well. We will need to push this to the
	// registry later.
	if indexWithMetadata.Index.MediaType == ocispec.MediaTypeImageManifest {
		err = b.blobStore.Push(ctx, indexWithMetadata.Index.Config, bytes.NewReader(defaultConfigContent))
		if err != nil && !errors.Is(err, errdef.ErrAlreadyExists) {
			return ocispec.Descriptor{}, fmt.Errorf("error creating OCI 1.0 empty config: %w", err)
		}
	}

	dgst := digest.FromBytes(manifest)
	size := int64(len(manifest))
	desc := ocispec.Descriptor{
		MediaType:    ocispec.MediaTypeImageManifest,
		ArtifactType: indexWithMetadata.Index.ArtifactType,
		Digest:       dgst,
		Size:         size,
	}

	err = b.blobStore.Push(ctx, desc, bytes.NewReader(manifest))
	if err != nil && !errors.Is(err, errdef.ErrAlreadyExists) {
		return ocispec.Descriptor{}, fmt.Errorf("cannot write SOCI index to local store: %w", err)
	}

	log.G(ctx).WithField("digest", dgst.String()).Debugf("soci index has been written")

	if gcRoot {
		err = store.LabelGCRoot(ctx, b.blobStore, desc)
		if err != nil {
			return ocispec.Descriptor{}, fmt.Errorf("cannot apply garbage collection label to index %s: %w", desc.Digest.String(), err)
		}
	}
	err = store.LabelGCRefContent(ctx, b.blobStore, desc, "config", defaultConfigDescriptor.Digest.String())
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("cannot apply garbage collection label to index %s referencing default config: %w", desc.Digest.String(), err)
	}

	var allErr error
	for i, blob := range indexWithMetadata.Index.Blobs {
		err = store.LabelGCRefContent(ctx, b.blobStore, desc, "ztoc."+strconv.Itoa(i), blob.Digest.String())
		if err != nil {
			errors.Join(allErr, err)
		}
	}
	if allErr != nil {
		return ocispec.Descriptor{}, fmt.Errorf("cannot apply one or more garbage collection labels to index %s: %w", desc.Digest.String(), allErr)
	}

	refers := indexWithMetadata.Index.Subject

	if refers == nil && indexWithMetadata.Index.ArtifactType == SociIndexArtifactTypeV1 {
		return ocispec.Descriptor{}, errors.New("cannot write soci index: the Refers field is nil")
	}

	// this entry is persisted to be used by cli push
	entry := &ArtifactEntry{
		Digest:         dgst.String(),
		OriginalDigest: indexWithMetadata.ManifestDesc.Digest.String(),
		ImageDigest:    indexWithMetadata.ImageDesc.Digest.String(),
		Platform:       platforms.Format(*indexWithMetadata.Platform),
		Type:           ArtifactEntryTypeIndex,
		Location:       indexWithMetadata.ManifestDesc.Digest.String(),
		Size:           size,
		MediaType:      indexWithMetadata.Index.MediaType,
		ArtifactType:   indexWithMetadata.Index.Config.MediaType,
		CreatedAt:      indexWithMetadata.CreatedAt,
	}
	return desc, b.config.artifactsDb.WriteArtifactEntry(entry)
}

func (b *IndexBuilder) maybeAddDisableXattrAnnotation(ztocDesc *ocispec.Descriptor, ztoc *ztoc.Ztoc) {
	if b.config.hasOptimization(XAttrOptimization) && shouldDisableXattrs(ztoc) {
		if ztocDesc.Annotations == nil {
			ztocDesc.Annotations = make(map[string]string, 1)
		}
		ztocDesc.Annotations[IndexAnnotationDisableXAttrs] = disableXAttrsTrue
	}
}

func shouldDisableXattrs(ztoc *ztoc.Ztoc) bool {
	for _, md := range ztoc.TOC.FileMetadata {
		if len(md.Xattrs()) > 0 || strings.HasSuffix(md.Name, whiteoutOpaqueDir) {
			return false
		}
	}

	return true
}
