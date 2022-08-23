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

	"github.com/awslabs/soci-snapshotter/cmd/soci/commands/internal"
	"github.com/awslabs/soci-snapshotter/fs/config"
	"github.com/awslabs/soci-snapshotter/soci"
	"github.com/containerd/containerd/cmd/ctr/commands"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
	"oras.land/oras-go/v2/content/oci"
)

const (
	buildToolIdentifier = "AWS SOCI CLI v0.1"

	spanSizeFlag           = "span-size"
	minLayerSizeFlag       = "min-layer-size"
	createORASManifestFlag = "oras"
)

// CreateCommand creates SOCI index for an image
// Output of this command is SOCI layers and SOCI index stored in a local directory
// SOCI layer is named as <image-layer-digest>.soci.layer
// SOCI index is named as <image-manifest-digest>.soci.index
var CreateCommand = cli.Command{
	Name:      "create",
	Usage:     "create SOCI index",
	ArgsUsage: "[flags] <image_ref>",
	Flags: append(
		internal.PlatformFlags,
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
		cli.BoolFlag{
			Name:  createORASManifestFlag,
			Usage: "If set, will create an ORAS manifest instead of an OCI Artifact manifest. Default is false.",
		},
	),
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

		ps, err := internal.GetPlatforms(ctx, cliContext, srcImg, cs)
		if err != nil {
			return err
		}

		manifestType := soci.ManifestOCIArtifact
		if cliContext.Bool(createORASManifestFlag) {
			manifestType = soci.ManifestORAS
		}

		for _, plat := range ps {
			sociIndexWithMetadata, err := soci.BuildSociIndex(ctx, cs, srcImg, spanSize, blobStore,
				soci.WithMinLayerSize(minLayerSize),
				soci.WithBuildToolIdentifier(buildToolIdentifier),
				soci.WithManifestType(manifestType),
				soci.WithPlatform(plat))

			if err != nil {
				return err
			}

			err = soci.WriteSociIndex(ctx, sociIndexWithMetadata, blobStore)
			if err != nil {
				return err
			}
		}

		return nil
	},
}
