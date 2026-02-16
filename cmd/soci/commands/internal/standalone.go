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
	"cmp"
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/awslabs/soci-snapshotter/soci"
	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/content/local"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/reference"
	"github.com/google/uuid"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/urfave/cli/v3"
	oraslib "oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/oci"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
)

type StandaloneImageInfo struct {
	ContentStore content.Store
	Image        images.Image
	TempDir      string
	OrasStore    *oci.Store
}

func (info *StandaloneImageInfo) Cleanup() {
	if info.TempDir != "" {
		cleanupTempDir(info.TempDir)
	}
}

// DownloadImageFromRegistry downloads an image from a registry using ORAS
// and returns a content store and image metadata that can be used by IndexBuilder.
func DownloadImageFromRegistry(ctx context.Context, cmd *cli.Command, imageRef string) (*StandaloneImageInfo, error) {
	refspec, err := reference.Parse(imageRef)
	if err != nil {
		return nil, fmt.Errorf("failed to parse image reference: %w", err)
	}

	tmpDir := filepath.Join(os.TempDir(), fmt.Sprintf("soci-%s", uuid.New().String()))
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}

	orasStore, err := oci.New(tmpDir)
	if err != nil {
		cleanupTempDir(tmpDir)
		return nil, fmt.Errorf("failed to create ORAS store: %w", err)
	}

	repo, err := newRemoteRepo(cmd, refspec)
	if err != nil {
		cleanupTempDir(tmpDir)
		return nil, fmt.Errorf("failed to create remote repository: %w", err)
	}

	verbose := cmd.Bool("verbose")
	if verbose {
		fmt.Printf("Downloading image %s from registry...\n", imageRef)
	}

	manifestDesc, err := repo.Resolve(ctx, refspec.Object)
	if err != nil {
		cleanupTempDir(tmpDir)
		return nil, fmt.Errorf("failed to resolve image %s: %w", imageRef, err)
	}

	copyOpts := oraslib.DefaultCopyGraphOptions
	if verbose {
		copyOpts.PreCopy = func(_ context.Context, desc ocispec.Descriptor) error {
			mediaType := cmp.Or(desc.MediaType, soci.SociLayerMediaType)
			fmt.Printf("  Downloading %s (%s, %d bytes)\n", desc.Digest.Encoded()[:12], mediaType, desc.Size)
			return nil
		}
	}

	if err := oraslib.CopyGraph(ctx, repo, orasStore, manifestDesc, copyOpts); err != nil {
		cleanupTempDir(tmpDir)
		return nil, fmt.Errorf("failed to download image %s: %w", imageRef, err)
	}

	if verbose {
		fmt.Printf("Successfully downloaded image\n")
	}

	contentStore, err := local.NewStore(tmpDir)
	if err != nil {
		cleanupTempDir(tmpDir)
		return nil, fmt.Errorf("failed to create content store: %w", err)
	}

	return &StandaloneImageInfo{
		ContentStore: contentStore,
		Image: images.Image{
			Name:   imageRef,
			Target: manifestDesc,
		},
		TempDir:   tmpDir,
		OrasStore: orasStore,
	}, nil
}

// PushConvertedImage pushes a converted OCI Index (containing image + SOCI indexes) to the registry.
// This is similar to `nerdctl push` but works without containerd by using the ORAS OCI store directly.
func PushConvertedImage(ctx context.Context, cmd *cli.Command, info *StandaloneImageInfo, convertedDesc ocispec.Descriptor, dstRef string) error {
	verbose := cmd.Bool("verbose")

	refspec, err := reference.Parse(dstRef)
	if err != nil {
		return fmt.Errorf("failed to parse reference %s: %w", dstRef, err)
	}

	repo, err := newRemoteRepo(cmd, refspec)
	if err != nil {
		return fmt.Errorf("failed to create remote repository: %w", err)
	}

	if verbose {
		fmt.Printf("Pushing SOCI-enabled image to %s...\n", dstRef)
	}

	copyOpts := oraslib.DefaultCopyGraphOptions
	if verbose {
		copyOpts.PreCopy = func(_ context.Context, d ocispec.Descriptor) error {
			mediaType := cmp.Or(d.MediaType, soci.SociLayerMediaType)
			fmt.Printf("  Pushing %s (%s, %d bytes)\n", d.Digest.Encoded()[:12], mediaType, d.Size)
			return nil
		}
	}

	if err := oraslib.CopyGraph(ctx, info.OrasStore, repo, convertedDesc, copyOpts); err != nil {
		return fmt.Errorf("failed to push to %s: %w", dstRef, err)
	}

	if err := repo.Tag(ctx, convertedDesc, refspec.Object); err != nil {
		return fmt.Errorf("failed to tag %s: %w", dstRef, err)
	}

	if verbose {
		fmt.Printf("Successfully pushed to %s (digest: %s)\n", dstRef, convertedDesc.Digest)
	}

	return nil
}

func newRemoteRepo(cmd *cli.Command, refspec reference.Spec) (*remote.Repository, error) {
	repo, err := remote.NewRepository(refspec.Locator)
	if err != nil {
		return nil, err
	}
	username, password := ResolveCredentials(cmd, refspec.Hostname())
	repo.Client = setupAuthClient(cmd, username, password)
	repo.PlainHTTP = cmd.Bool("plain-http")
	return repo, nil
}

func cleanupTempDir(tmpDir string) {
	if err := os.RemoveAll(tmpDir); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to clean up temp directory %s: %v\n", tmpDir, err)
	}
}

func setupAuthClient(cmd *cli.Command, username, password string) *auth.Client {
	authClient := &auth.Client{
		Credential: func(_ context.Context, host string) (auth.Credential, error) {
			return auth.Credential{
				Username: username,
				Password: password,
			}, nil
		},
	}

	if cmd.Bool("skip-verify") {
		authClient.Client = &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
			},
		}
	}

	return authClient
}
