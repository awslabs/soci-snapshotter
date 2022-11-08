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
	"os"

	"github.com/awslabs/soci-snapshotter/fs/config"
	"github.com/awslabs/soci-snapshotter/soci"
	"github.com/containerd/containerd/cmd/ctr/commands"
	"github.com/containerd/containerd/platforms"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
	"oras.land/oras-go/v2/content/oci"
)

const (
	buildToolIdentifier = "AWS SOCI CLI v0.1"

	spanSizeFlag     = "span-size"
	minLayerSizeFlag = "min-layer-size"
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
			Name:  spanSizeFlag,
			Usage: "Span size of index. Default is 4 MiB",
			Value: 1 << 22,
		},
		cli.Int64Flag{
			Name:  minLayerSizeFlag,
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
		spanSize := cliContext.Int64(spanSizeFlag)
		minLayerSize := cliContext.Int64(minLayerSizeFlag)
		// Creating the snapshotter's root path first if it does not exist, since this ensures, that
		// it has the limited permission set as drwx--x--x.
		// The subsequent oci.New creates a root path dir with too broad permission set.
		if _, err := os.Stat(config.SociSnapshotterRootPath); os.IsNotExist(err) {
			if err = os.Mkdir(config.SociSnapshotterRootPath, 0711); err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
		blobStore, err := oci.New(config.SociContentStorePath)
		if err != nil {
			return err
		}

		ib, err := soci.NewIndexBuilder(cs,
			soci.WithSpanSizeOption(spanSize),
			soci.WithMinLayerSizeOption(minLayerSize),
			soci.WithBuildToolIdentifierOption(buildToolIdentifier))
		if err != nil {
			return err
		}

		sociIndex, _, err := ib.BuildIndex(ctx, srcImg.Target)
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
