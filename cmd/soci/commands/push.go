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

package commands

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/awslabs/soci-snapshotter/fs/config"
	"github.com/awslabs/soci-snapshotter/soci"
	"github.com/containerd/containerd/cmd/ctr/commands"
	"github.com/containerd/containerd/reference"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/urfave/cli"
	oraslib "oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/oci"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
)

// PushCommand is a command to push an image artifacts from local content store to the remote repository
var PushCommand = cli.Command{
	Name:      "push",
	Usage:     "push SOCI artifacts to a registry",
	ArgsUsage: "[flags] <ref>",
	Description: `Push SOCI artifacts to a registry by image reference.
If multiple soci indices exist for the given image, the most recent one will be pushed.

After pushing the soci artifacts, they should be available in the registry. Soci artifacts will be pushed only
if they are available in the snapshotter's local content store.
`,
	Flags: append(append(append(commands.RegistryFlags, commands.LabelFlag), commands.SnapshotterFlags...),
		cli.Uint64Flag{
			Name:  "max-concurrent-uploads",
			Usage: "Max concurrent uploads. Default is 10",
			Value: 10,
		}),
	Action: func(cliContext *cli.Context) error {
		ref := cliContext.Args().First()
		if ref == "" {
			return fmt.Errorf("please provide an image reference to push")
		}

		client, ctx, cancel, err := commands.NewClient(cliContext)
		if err != nil {
			return err
		}
		defer cancel()

		cs := client.ContentStore()
		is := client.ImageService()
		img, err := is.Get(ctx, ref)
		if err != nil {
			return err
		}

		indexDescriptors, err := soci.GetIndexDescriptorCollection(ctx, cs, img)
		if err != nil {
			return err
		}

		if len(indexDescriptors) == 0 {
			return fmt.Errorf("could not find any soci indices to push")
		}

		username := cliContext.String("user")
		var secret string
		if i := strings.IndexByte(username, ':'); i > 0 {
			secret = username[i+1:]
			username = username[0:i]
		}

		src, err := oci.New(config.SociContentStorePath)
		if err != nil {
			return fmt.Errorf("cannot create OCI local store: %w", err)
		}

		indexDesc := indexDescriptors[len(indexDescriptors)-1]
		refspec, err := reference.Parse(ref)
		if err != nil {
			return err
		}

		dst, err := remote.NewRepository(refspec.Locator)
		if err != nil {
			return err
		}
		authClient := auth.DefaultClient
		authClient.Credential = func(_ context.Context, host string) (auth.Credential, error) {
			return auth.Credential{
				Username: username,
				Password: secret,
			}, nil
		}

		dst.Client = authClient
		dst.PlainHTTP = cliContext.Bool("plain-http")

		debug := cliContext.GlobalBool("debug")
		if debug {
			dst.Client = &debugClient{client: authClient}
		} else {
			dst.Client = authClient
		}

		options := oraslib.DefaultCopyGraphOptions
		options.PreCopy = func(_ context.Context, desc ocispec.Descriptor) error {
			fmt.Printf("pushing artifact with digest: %v\n", desc.Digest)
			return nil
		}
		options.PostCopy = func(_ context.Context, desc ocispec.Descriptor) error {
			fmt.Printf("successfully pushed artifact with digest: %v\n", desc.Digest)
			return nil
		}
		options.OnCopySkipped = func(ctx context.Context, desc ocispec.Descriptor) error {
			fmt.Printf("skipped artifact with digest: %v\n", desc.Digest)
			return nil
		}

		err = oraslib.CopyGraph(context.Background(), src, dst, indexDesc, options)
		if err != nil {
			return fmt.Errorf("error pushing graph to remote: %w", err)
		}

		return nil
	},
}

type debugClient struct {
	client remote.Client
}

func (c *debugClient) Do(req *http.Request) (*http.Response, error) {
	fmt.Printf("http req %s %s\n", req.Method, req.URL)
	res, err := c.client.Do(req)
	if err != nil {
		fmt.Printf("http err %v\n", err)
	} else {
		fmt.Printf("http res %s\n", res.Status)
	}
	return res, err
}
