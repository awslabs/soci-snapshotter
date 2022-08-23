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

package image

import (
	"context"
	"fmt"

	"github.com/awslabs/soci-snapshotter/cmd/soci/commands/internal"
	"github.com/awslabs/soci-snapshotter/fs/source"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cmd/ctr/commands"
	"github.com/containerd/containerd/cmd/ctr/commands/content"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/log"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/urfave/cli"
)

const (
	remoteSnapshotterName = "soci"
	skipContentVerifyOpt  = "skip-content-verify"
)

// rpullCommand is a subcommand to pull an image from a registry levaraging soci snapshotter
var rpullCommand = cli.Command{
	Name:      "rpull",
	Usage:     "pull an image from a registry levaraging soci snapshotter",
	ArgsUsage: "[flags] <ref>",
	Description: `Fetch and prepare an image for use in containerd levaraging soci snapshotter.

After pulling an image, it should be ready to use the same reference in a run
command. 
`,
	Flags: append(append(append(
		commands.RegistryFlags,
		commands.LabelFlag),
		commands.SnapshotterFlags...),
		cli.BoolFlag{
			Name:  skipContentVerifyOpt,
			Usage: "Skip content verification for layers contained in this image.",
		},
		// This is a standin for the snapshotter receiving the index digest from container runtimes.
		cli.StringFlag{
			Name:  "soci-index-digest",
			Usage: "The SOCI index digest.",
		},
		cli.StringFlag{
			Name:  internal.PlatformFlagKey,
			Usage: "The platform to pull.",
		},
	),
	Action: func(context *cli.Context) error {
		var (
			ref    = context.Args().First()
			config = &rPullConfig{}
		)
		if ref == "" {
			return fmt.Errorf("please provide an image reference")
		}

		config.indexDigest = context.String("soci-index-digest")

		client, ctx, cancel, err := commands.NewClient(context)
		if err != nil {
			return err
		}
		defer cancel()

		ctx, done, err := client.WithLease(ctx)
		if err != nil {
			return err
		}
		defer done(ctx)

		fc, err := content.NewFetchConfig(ctx, context)
		if err != nil {
			return err
		}
		config.FetchConfig = fc

		if context.Bool(skipContentVerifyOpt) {
			config.skipVerify = true
		}

		config.snapshotter = remoteSnapshotterName
		if sn := context.String("snapshotter"); sn != "" {
			config.snapshotter = sn
		}

		config.platform = context.String(internal.PlatformFlagKey)

		return pull(ctx, client, ref, config)
	},
}

type rPullConfig struct {
	*content.FetchConfig
	skipVerify  bool
	snapshotter string
	indexDigest string
	platform    string
}

func pull(ctx context.Context, client *containerd.Client, ref string, config *rPullConfig) error {
	pCtx := ctx
	h := images.HandlerFunc(func(ctx context.Context, desc ocispec.Descriptor) ([]ocispec.Descriptor, error) {
		if desc.MediaType != images.MediaTypeDockerSchema1Manifest {
			fmt.Printf("fetching %v... %v\n", desc.Digest.String()[:15], desc.MediaType)
		}
		return nil, nil
	})

	log.G(pCtx).WithField("image", ref).Debug("fetching")
	labels := commands.LabelArgs(config.Labels)
	if _, err := client.Pull(pCtx, ref, []containerd.RemoteOpt{
		containerd.WithPullLabels(labels),
		containerd.WithResolver(config.Resolver),
		containerd.WithImageHandler(h),
		containerd.WithSchema1Conversion,
		containerd.WithPullUnpack,
		containerd.WithPlatform(config.platform),
		containerd.WithPullSnapshotter(config.snapshotter),
		containerd.WithImageHandlerWrapper(source.AppendDefaultLabelsHandlerWrapper(ref, config.indexDigest)),
	}...); err != nil {
		return err
	}

	return nil
}
