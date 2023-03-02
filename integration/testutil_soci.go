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

package integration

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/awslabs/soci-snapshotter/soci"
	shell "github.com/awslabs/soci-snapshotter/util/dockershell"
	"github.com/containerd/containerd/platforms"
	"github.com/google/go-cmp/cmp"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

const (
	// copied from `soci/soci_index.go` for convenience so we don't always need to
	// import the `soci` pkg only to use the default values.
	defaultSpanSize     = int64(1 << 22) // 4MiB
	defaultMinLayerSize = 10 << 20       // 10MiB
)

// indexBuildConfig represents the values of the CLI flags that should be used
// when creating an index with `buildIndex`
type indexBuildConfig struct {
	spanSize                int64
	minLayerSize            int64
	supportArtifactRegistry bool
}

// indexBuildOption is a functional argument to update `indexBuildConfig`
type indexBuildOption func(*indexBuildConfig)

// withSpanSize overrides the default span size to use when creating an index with `buildIndex`
func withSpanSize(spanSize int64) indexBuildOption {
	return func(ibc *indexBuildConfig) {
		ibc.spanSize = spanSize
	}
}

// withMinLayerSize overrides the minimum layer size for which to create a ztoc
// when creating an index with `buildIndex`
func withMinLayerSize(minLayerSize int64) indexBuildOption {
	return func(ibc *indexBuildConfig) {
		ibc.minLayerSize = minLayerSize
	}
}

// withOCIArtifactRegistrySupport sets the SOCI index to built as an artifact manifest
func withOCIArtifactRegistrySupport(ibc *indexBuildConfig) {
	ibc.supportArtifactRegistry = true
}

// defaultIndexBuildConfig is the default parameters when creating and index with `buildIndex`
func defaultIndexBuildConfig() indexBuildConfig {
	return indexBuildConfig{
		spanSize:     defaultSpanSize,
		minLayerSize: defaultMinLayerSize,
	}
}

// buildIndex builds an index for the source image with given options. By default, it will build with
// min-layer-size = 0 and span-size = CLI default
func buildIndex(sh *shell.Shell, src imageInfo, opt ...indexBuildOption) string {
	indexBuildConfig := defaultIndexBuildConfig()
	for _, o := range opt {
		o(&indexBuildConfig)
	}
	opts := encodeImageInfoNerdctl(src)

	createCommand := []string{"soci", "create", src.ref}
	createArgs := []string{
		"--min-layer-size", fmt.Sprintf("%d", indexBuildConfig.minLayerSize),
		"--span-size", fmt.Sprintf("%d", indexBuildConfig.spanSize),
		"--platform", platforms.Format(src.platform),
	}
	if indexBuildConfig.supportArtifactRegistry {
		createArgs = append(createArgs, "--manifest-type", "artifact")
	}

	indexDigest := sh.
		X(append([]string{"nerdctl", "pull", "-q", "--platform", platforms.Format(src.platform)}, opts[0]...)...).
		X(append(createCommand, createArgs...)...).
		O("soci", "index", "list",
			"-q", "--ref", src.ref,
			"--platform", platforms.Format(src.platform)) // this will make SOCI artifact available locally
	return strings.Trim(string(indexDigest), "\n")
}

func validateSociIndex(sh *shell.Shell, sociIndex soci.Index, imgManifestDigest string, includedLayers map[string]struct{}) error {
	if sociIndex.MediaType != ocispec.MediaTypeArtifactManifest && sociIndex.MediaType != ocispec.MediaTypeImageManifest {
		return fmt.Errorf("unexpected index media type; expected types: [%v, %v], got: %v", ocispec.MediaTypeArtifactManifest, ocispec.MediaTypeImageManifest, sociIndex.MediaType)
	}
	if sociIndex.ArtifactType != soci.SociIndexArtifactType {
		return fmt.Errorf("unexpected index artifact type; expected = %v, got = %v", soci.SociIndexArtifactType, sociIndex.ArtifactType)
	}

	expectedAnnotations := map[string]string{
		soci.IndexAnnotationBuildToolIdentifier: "AWS SOCI CLI v0.1",
	}

	if diff := cmp.Diff(sociIndex.Annotations, expectedAnnotations); diff != "" {
		return fmt.Errorf("unexpected index annotations; diff = %v", diff)
	}

	if imgManifestDigest != sociIndex.Subject.Digest.String() {
		return fmt.Errorf("unexpected subject digest; expected = %v, got = %v", imgManifestDigest, sociIndex.Subject.Digest.String())
	}

	blobs := sociIndex.Blobs
	if includedLayers != nil && len(blobs) != len(includedLayers) {
		return fmt.Errorf("unexpected blob count; expected=%v, got=%v", len(includedLayers), len(blobs))
	}

	for _, blob := range blobs {
		blobContent := fetchContentFromPath(sh, blobStorePath+"/"+trimSha256Prefix(blob.Digest.String()))
		blobSize := int64(len(blobContent))
		blobDigest := digest.FromBytes(blobContent)

		if includedLayers != nil {
			layerDigest := blob.Annotations[soci.IndexAnnotationImageLayerDigest]

			if _, ok := includedLayers[layerDigest]; !ok {
				return fmt.Errorf("found ztoc for layer %v in index but should not have built ztoc for it", layerDigest)
			}
		}

		if blobSize != blob.Size {
			return fmt.Errorf("unexpected blob size; expected = %v, got = %v", blob.Size, blobSize)
		}

		if blobDigest != blob.Digest {
			return fmt.Errorf("unexpected blob digest; expected = %v, got = %v", blob.Digest, blobDigest)
		}
	}

	return nil
}

// getSociLocalStoreContentDigest will generate a digest based on the contents of the soci content store
// Files that are smaller than 10 bytes wil not be included when generating the digest
func getSociLocalStoreContentDigest(sh *shell.Shell) digest.Digest {
	content := new(bytes.Buffer)
	sh.Pipe(nil, []string{"find", blobStorePath, "-maxdepth", "1", "-type", "f", "-size", "+10c"}).Pipe(content, []string{"sort"})
	return digest.FromBytes(content.Bytes())
}

func sociIndexFromDigest(sh *shell.Shell, indexDigest string) (index soci.Index, err error) {
	rawSociIndexJSON, err := sh.OLog("soci", "index", "info", indexDigest)
	if err != nil {
		return
	}
	if err = soci.UnmarshalIndex(rawSociIndexJSON, &index); err != nil {
		err = fmt.Errorf("invalid soci index from digest %s: %s", indexDigest, err)
	}
	return
}
