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

	ctdarchive "github.com/containerd/containerd/archive"
	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/content/local"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/images/archive"
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
		if _, err := ctdarchive.Apply(ctx, tmpDir, tarFile); err != nil {
			return nil, fmt.Errorf("failed to extract OCI tar %s: %w", inputPath, err)
		}
	}

	indexData, err := os.ReadFile(filepath.Join(tmpDir, "index.json"))
	if err != nil {
		return nil, fmt.Errorf("failed to read index.json from %s: %w", inputPath, err)
	}
	rootDesc, err := parseRootDescriptor(indexData)
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

// parseRootDescriptor unmarshals OCI index JSON and returns the manifest descriptor.
func parseRootDescriptor(indexData []byte) (ocispec.Descriptor, error) {
	var index ocispec.Index
	if err := json.Unmarshal(indexData, &index); err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("failed to unmarshal index.json: %w", err)
	}
	if len(index.Manifests) == 0 {
		return ocispec.Descriptor{}, errors.New("index.json contains no manifests")
	}
	return index.Manifests[0], nil
}
