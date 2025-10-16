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
	"path/filepath"
	"strings"

	"github.com/awslabs/soci-snapshotter/config"
	"github.com/awslabs/soci-snapshotter/soci"
	"github.com/awslabs/soci-snapshotter/soci/store"
	shell "github.com/awslabs/soci-snapshotter/util/dockershell"
	"github.com/awslabs/soci-snapshotter/util/testutil"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/platforms"
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
	spanSize                 int64
	minLayerSize             int64
	allowErrors              bool
	contentStoreType         store.ContentStoreType
	namespace                string
	runRebuildDbBeforeCreate bool
	forceRecreateZtocs       bool
}

// indexBuildOption is a functional argument to update `indexBuildConfig`
type indexBuildOption func(*indexBuildConfig)

// withIndexBuildConfig copies a provided config
func withIndexBuildConfig(newIbc indexBuildConfig) indexBuildOption {
	return func(ibc *indexBuildConfig) {
		ibc.spanSize = newIbc.spanSize
		ibc.minLayerSize = newIbc.minLayerSize
		ibc.allowErrors = newIbc.allowErrors
		ibc.contentStoreType = newIbc.contentStoreType
		ibc.namespace = newIbc.namespace
		ibc.runRebuildDbBeforeCreate = newIbc.runRebuildDbBeforeCreate
		ibc.forceRecreateZtocs = newIbc.forceRecreateZtocs
	}
}

// withForceRecreateZtocs passes the --force flag to "soci create".
func withForceRecreateZtocs(forceRecreateZtocs bool) indexBuildOption {
	return func(ibc *indexBuildConfig) {
		ibc.forceRecreateZtocs = forceRecreateZtocs
	}
}

// withRebuildDbBeforeCreate syncs the artifact store with the content store before calling "soci create"
// We do this because the artifact store could be out of sync with the content store when 'soci create' is called.
// This is problematic in cases where we create soci indexes for some images, delete those indexes and immediately recreate
// them (like in TestSociIndexRemove) - as there could be ztoc entries in the artifact store which are not present in the
// content store, causing 'soci create/convert' without --force flag to throw an error.
//
// We can run this by default and probably remove this option in the future when the race condition with rebuild-db is solved.
func withRunRebuildDbBeforeCreate() indexBuildOption {
	return func(ibc *indexBuildConfig) {
		ibc.runRebuildDbBeforeCreate = true
	}
}

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

// withContentStoreType overrides the default content store
func withContentStoreType(contentStoreType store.ContentStoreType) indexBuildOption {
	return func(ibc *indexBuildConfig) {
		ibc.contentStoreType, _ = store.CanonicalizeContentStoreType(contentStoreType)
	}
}

// withNamespace overrides the default namespace
func withNamespace(namespace string) indexBuildOption {
	return func(ibc *indexBuildConfig) {
		ibc.namespace = namespace
	}
}

// withAllowErrors does not fatally fail the test on the a shell command non-zero exit code
func withAllowErrors(ibc *indexBuildConfig) {
	ibc.allowErrors = true
}

// defaultIndexBuildConfig is the default parameters when creating and index with `buildIndex`
func defaultIndexBuildConfig() indexBuildConfig {
	return indexBuildConfig{
		spanSize:         defaultSpanSize,
		minLayerSize:     defaultMinLayerSize,
		contentStoreType: config.DefaultContentStoreType,
		namespace:        namespaces.Default,
	}
}

// buildIndex builds an index for the source image with given options. By default, it will build with
// min-layer-size = 0 and span-size = CLI default
// returns the index digest, or an empty string for failure
func buildIndex(sh *shell.Shell, src imageInfo, opt ...indexBuildOption) string {
	indexBuildConfig := defaultIndexBuildConfig()
	for _, o := range opt {
		o(&indexBuildConfig)
	}
	opts := encodeImageInfoNerdctl(src)

	createCommand := []string{
		"soci",
		"--namespace", indexBuildConfig.namespace,
		"--content-store", string(indexBuildConfig.contentStoreType),
		"create",
		"--min-layer-size", fmt.Sprintf("%d", indexBuildConfig.minLayerSize),
		"--span-size", fmt.Sprintf("%d", indexBuildConfig.spanSize),
		"--platform", platforms.Format(src.platform),
		src.ref,
	}
	if indexBuildConfig.forceRecreateZtocs {
		createCommand = append(createCommand, "--force")
	}

	shx := sh.X
	if indexBuildConfig.allowErrors {
		shx = sh.XLog
	}

	shx(append([]string{"nerdctl", "--namespace", indexBuildConfig.namespace, "pull", "-q", "--platform", platforms.Format(src.platform)}, opts[0]...)...)

	if indexBuildConfig.runRebuildDbBeforeCreate {
		shx("soci", "--content-store", string(indexBuildConfig.contentStoreType), "rebuild-db")
	}

	shx(createCommand...)
	indexDigest, err := sh.OLog("soci",
		"--namespace", indexBuildConfig.namespace,
		"--content-store", string(indexBuildConfig.contentStoreType),
		"index", "list",
		"-q", "--ref", src.ref,
		"--platform", platforms.Format(src.platform), // this will make SOCI artifact available locally
	)
	if err != nil {
		return ""
	}

	return strings.Trim(string(indexDigest), "\n")
}

func validateSociIndex(sh *shell.Shell, contentStoreType store.ContentStoreType, sociIndex soci.Index, imgManifestDigest string, includedLayers map[string]struct{}) error {
	if sociIndex.MediaType != ocispec.MediaTypeImageManifest {
		return fmt.Errorf("unexpected index media type; expected types: [%v], got: %v", ocispec.MediaTypeImageManifest, sociIndex.MediaType)
	}
	if sociIndex.ArtifactType != soci.SociIndexArtifactType {
		return fmt.Errorf("unexpected index artifact type; expected = %v, got = %v", soci.SociIndexArtifactType, sociIndex.ArtifactType)
	}

	expectedAnnotations := map[string]string{
		soci.IndexAnnotationBuildToolIdentifier: "AWS SOCI CLI v0.2",
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
		blobPath, err := testutil.GetContentStoreBlobPath(contentStoreType)
		if err != nil {
			return err
		}
		blobContent := fetchContentFromPath(sh, filepath.Join(blobPath, blob.Digest.Encoded()))
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
// Files that are smaller than 10 bytes will not be included when generating the digest
func getSociLocalStoreContentDigest(sh *shell.Shell, contentStoreType store.ContentStoreType) (string, error) {
	content := new(bytes.Buffer)
	blobPath, err := testutil.GetContentStoreBlobPath(contentStoreType)
	if err != nil {
		return "", err
	}
	sh.Pipe(nil, []string{"find", blobPath, "-maxdepth", "1", "-type", "f", "-size", "+10c"}).Pipe(content, []string{"sort"})
	return digest.FromBytes(content.Bytes()).String(), nil
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
