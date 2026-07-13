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

package internal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/containerd/containerd/v2/core/content"
	"github.com/containerd/containerd/v2/core/images"
	"github.com/containerd/containerd/v2/core/images/archive"
	ctdarchive "github.com/containerd/containerd/v2/pkg/archive"
	"github.com/containerd/containerd/v2/plugins/content/local"
	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/specs-go"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/content/oci"
)

type StandaloneImageInfo struct {
	ContentStore content.Store
	Image        images.Image
	OrasStore    *oci.Store
}

// LoadImage loads an OCI image layout (tar or directory) into a writable OCI store
// and returns image metadata that can be used by IndexBuilder.
// If inputPath is a directory, it is copied into tmpDir.
// If inputPath is a tar file, it is extracted as an OCI image layout into tmpDir.
func LoadImage(ctx context.Context, inputPath string, tmpDir string) (*StandaloneImageInfo, error) {
	fi, err := os.Stat(inputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to access input %s: %w", inputPath, err)
	}

	if fi.IsDir() {
		if err := os.CopyFS(tmpDir, os.DirFS(inputPath)); err != nil {
			return nil, fmt.Errorf("failed to copy OCI image layout from %s: %w", inputPath, err)
		}
	} else {
		tarFile, err := os.Open(inputPath)
		if err != nil {
			return nil, fmt.Errorf("failed to open tar %s: %w", inputPath, err)
		}
		defer tarFile.Close()
		// Extract as the current user instead of reproducing the tar's UID/GID.
		// OCI layout tars record root ownership (0/0), which is irrelevant for a
		// content-addressed layout and makes Lchown fail for non-root users.
		if _, err := ctdarchive.Apply(ctx, tmpDir, tarFile, ctdarchive.WithNoSameOwner()); err != nil {
			return nil, fmt.Errorf("failed to extract OCI tar %s: %w", inputPath, err)
		}
	}

	indexData, err := os.ReadFile(filepath.Join(tmpDir, "index.json"))
	if err != nil {
		return nil, fmt.Errorf("failed to read index.json from %s: %w", inputPath, err)
	}
	rootDesc, err := resolveLayoutRoot(tmpDir, indexData)
	if err != nil {
		return nil, err
	}

	orasStore, err := oci.New(tmpDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create writable OCI store: %w", err)
	}

	contentStore, err := local.NewStore(tmpDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create content store: %w", err)
	}

	return &StandaloneImageInfo{
		ContentStore: contentStore,
		Image:        images.Image{Name: inputPath, Target: rootDesc},
		OrasStore:    orasStore,
	}, nil
}

func SaveImageToTar(ctx context.Context, cs content.Store, desc ocispec.Descriptor, outputTarPath string) error {
	outFile, err := os.Create(outputTarPath)
	if err != nil {
		return fmt.Errorf("failed to create output tar %s: %w", outputTarPath, err)
	}
	defer outFile.Close()

	return archive.Export(ctx, cs, outFile,
		archive.WithManifest(desc),
		archive.WithSkipDockerManifest(),
	)
}

// SaveImageToDir copies the OCI image layout from srcDir to outputPath and writes
// a clean index.json containing only the given descriptor.
func SaveImageToDir(srcDir string, desc ocispec.Descriptor, outputPath string) error {
	if err := os.RemoveAll(outputPath); err != nil {
		return fmt.Errorf("failed to clean output directory %s: %w", outputPath, err)
	}
	if err := os.MkdirAll(outputPath, 0755); err != nil {
		return fmt.Errorf("failed to create output directory %s: %w", outputPath, err)
	}
	if err := os.CopyFS(outputPath, os.DirFS(srcDir)); err != nil {
		return fmt.Errorf("failed to copy OCI layout to %s: %w", outputPath, err)
	}
	indexData, err := json.Marshal(ocispec.Index{
		Versioned: specs.Versioned{SchemaVersion: 2},
		MediaType: ocispec.MediaTypeImageIndex,
		Manifests: []ocispec.Descriptor{desc},
	})
	if err != nil {
		return fmt.Errorf("failed to marshal index.json: %w", err)
	}
	return os.WriteFile(filepath.Join(outputPath, "index.json"), indexData, 0644)
}

// resolveLayoutRoot returns a root descriptor for the OCI image layout. The result
// is either a single image manifest descriptor or a manifest list descriptor.
//
// It accepts index.json shapes produced by common tools: a single image manifest,
// a single descriptor pointing at a nested manifest list (e.g. nerdctl save), or
// a flat list of per-platform manifests (e.g. go-containerregistry layout.Write).
// If some children are missing their blobs, they are filtered out and a new
// manifest list blob is written into layoutDir.
func resolveLayoutRoot(layoutDir string, indexData []byte) (ocispec.Descriptor, error) {
	var top ocispec.Index
	if err := json.Unmarshal(indexData, &top); err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("unmarshal index.json: %w", err)
	}
	if len(top.Manifests) == 0 {
		return ocispec.Descriptor{}, errors.New("index.json contains no manifests")
	}
	// Single non-index entry: a plain single-platform image.
	if len(top.Manifests) == 1 && !images.IsIndexType(top.Manifests[0].MediaType) {
		return top.Manifests[0], nil
	}

	// Locate the manifest list to walk. Either index.json points at a nested list
	// blob, or index.json is itself the list.
	var (
		listBytes = indexData
		mediaType = top.MediaType
	)
	if len(top.Manifests) == 1 {
		mediaType = top.Manifests[0].MediaType
		b, err := os.ReadFile(blobPath(layoutDir, top.Manifests[0].Digest))
		if err != nil {
			return ocispec.Descriptor{}, fmt.Errorf("read manifest list: %w", err)
		}
		listBytes = b
	}
	if mediaType == "" {
		mediaType = ocispec.MediaTypeImageIndex
	}

	var list ocispec.Index
	if err := json.Unmarshal(listBytes, &list); err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("unmarshal manifest list: %w", err)
	}

	available := make([]ocispec.Descriptor, 0, len(list.Manifests))
	for _, d := range list.Manifests {
		if _, err := os.Stat(blobPath(layoutDir, d.Digest)); err == nil {
			available = append(available, d)
		}
	}
	switch {
	case len(available) == 0:
		return ocispec.Descriptor{}, errors.New("manifest list contains no entries with available blobs")
	case len(available) == 1 && images.IsManifestType(available[0].MediaType):
		return available[0], nil
	case len(top.Manifests) == 1 && len(available) == len(list.Manifests):
		return top.Manifests[0], nil
	}

	list.MediaType = mediaType
	list.Manifests = available
	data, err := json.Marshal(list)
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("marshal manifest list: %w", err)
	}
	dgst := digest.FromBytes(data)
	if err := os.MkdirAll(filepath.Dir(blobPath(layoutDir, dgst)), 0755); err != nil {
		return ocispec.Descriptor{}, err
	}
	if err := os.WriteFile(blobPath(layoutDir, dgst), data, 0644); err != nil {
		return ocispec.Descriptor{}, err
	}
	return ocispec.Descriptor{MediaType: mediaType, Digest: dgst, Size: int64(len(data))}, nil
}

func blobPath(layoutDir string, dgst digest.Digest) string {
	return filepath.Join(layoutDir, "blobs", dgst.Algorithm().String(), dgst.Encoded())
}
