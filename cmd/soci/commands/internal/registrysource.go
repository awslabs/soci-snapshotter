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
	"fmt"

	"github.com/containerd/containerd/v2/core/content"
	"github.com/containerd/containerd/v2/core/images"
	"github.com/containerd/containerd/v2/pkg/reference"
	"github.com/urfave/cli/v3"
	oraslib "oras.land/oras-go/v2"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
)

const (
	SourceFlag       = "source"
	SourceContainerd = "containerd"
	SourceRegistry   = "registry"
)

// SupportedSourceOptions are the valid values for --source.
var SupportedSourceOptions = []string{SourceContainerd, SourceRegistry}

// SourceFlags are the flags controlling where create/push read the source
// image from.
var SourceFlags = []cli.Flag{
	&cli.StringFlag{
		Name: SourceFlag,
		Usage: `Where to read the source image from:
			containerd - read from containerd's image/content store (default; requires a running containerd with the image already pulled)
			registry   - pull the image directly from its registry; no containerd required`,
		Value: SourceContainerd,
	},
}

// PopulateFromRegistry pulls the image graph (manifest/index, config, and
// layers - recursively, so multi-platform indexes are handled automatically)
// for ref directly from its registry into cs, without requiring a live
// containerd daemon. It returns an images.Image describing the pulled
// image, suitable for passing directly to soci.NewIndexBuilder.Build or
// soci.GetIndexDescriptorCollection exactly as the containerd-backed path
// does - both only need a generic content.Store and this plain struct.
func PopulateFromRegistry(ctx context.Context, cmd *cli.Command, ref string, cs content.Store) (images.Image, error) {
	refspec, err := reference.Parse(ref)
	if err != nil {
		return images.Image{}, fmt.Errorf("could not parse reference %q: %w", ref, err)
	}

	src, err := remote.NewRepository(refspec.Locator)
	if err != nil {
		return images.Image{}, fmt.Errorf("could not create repository client for %q: %w", refspec.Locator, err)
	}

	username, password := ResolveCredentials(cmd, refspec.Hostname())
	authClient := auth.DefaultClient
	authClient.Credential = func(_ context.Context, _ string) (auth.Credential, error) {
		return auth.Credential{Username: username, Password: password}, nil
	}
	src.Client = authClient
	src.PlainHTTP = cmd.Bool(PlainHTTPFlag)

	object := refspec.Object
	if object == "" {
		object = "latest"
	}
	desc, err := src.Resolve(ctx, object)
	if err != nil {
		return images.Image{}, fmt.Errorf("could not resolve %q: %w", ref, err)
	}

	if err := oraslib.CopyGraph(ctx, src, newContentStoreAdapter(cs), desc, oraslib.DefaultCopyGraphOptions); err != nil {
		return images.Image{}, fmt.Errorf("could not pull %q: %w", ref, err)
	}

	return images.Image{Name: ref, Target: desc}, nil
}
