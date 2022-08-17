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
	"github.com/awslabs/soci-snapshotter/fs/config"
	"github.com/awslabs/soci-snapshotter/soci"
	"github.com/containerd/containerd/cmd/ctr/commands"
	"github.com/containerd/containerd/platforms"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
	"oras.land/oras-go/v2/content/oci"
)

const (
	buildToolIdentifier = "AWS SOCI CLI"
	buildToolVersion    = "0.1"
)

// CreateCommand creates SOCI index for an image
// Output of this command is SOCI layers and SOCI index stored in a local directory
// SOCI layer is named as <image-layer-digest>.soci.layer
// SOCI index is named as <image-manifest-digest>.soci.index
var CreateCommand = cli.Command{
	Name:      "create",
	Usage:     "create SOCI index",
	ArgsUsage: "[flags] <image_ref>",
	Flags: []cli.Flag{
		cli.Int64Flag{
			Name:  "span-size",
			Usage: "span size of index. Default is 1 MiB",
			Value: 1 << 20,
		},
		cli.Int64Flag{
			Name:  "min-layer-size",
			Usage: "The minimum layer size in bytes to build zTOC for. Default is 0.",
			Value: 0,
		},
	},
	Action: func(cliContext *cli.Context) error {
		srcRef := cliContext.Args().Get(0)
		if srcRef == "" {
			return errors.New("source image needs to be specified")
		}

		client, ctx, cancel, err := commands.NewClient(cliContext)
		if err != nil {
			return err
		}
		defer cancel()

		cs := client.ContentStore()
		is := client.ImageService()
		srcImg, err := is.Get(ctx, srcRef)
		if err != nil {
			return err
		}
		spanSize := cliContext.Int64("span-size")
		minLayerSize := cliContext.Int64("min-layer-size")
		blobStore, err := oci.New(config.SociContentStorePath)
		if err != nil {
			return err
		}

		sociIndex, err := soci.BuildSociIndex(ctx, cs, srcImg, spanSize, blobStore,
			soci.WithMinLayerSize(minLayerSize),
			soci.WithBuildToolIdentifier(buildToolIdentifier),
			soci.WithBuildToolVersion(buildToolVersion))

		if err != nil {
			return err
		}

		sociIndexWithMetadata := soci.IndexWithMetadata{
			Index:       sociIndex,
			ImageDigest: srcImg.Target.Digest,
			// TODO: This is not strictly correct because the default platform matcher used in BuildSociIndex
			// might match a compatible version (i.e. linux/amd64 will match linux/i386). Building indices for
			// multiple/non-default platforms will be needed at some point and this should be fixed with that change as well.
			Platform: platforms.DefaultSpec(),
		}

		err = soci.WriteSociIndex(ctx, sociIndexWithMetadata, blobStore)
		if err != nil {
			return err
		}

		return nil
	},
}
