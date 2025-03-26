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
	"errors"
	"fmt"
	"os"

	"github.com/awslabs/soci-snapshotter/cmd/soci/commands/internal"
	"github.com/awslabs/soci-snapshotter/config"
	"github.com/awslabs/soci-snapshotter/soci"
	"github.com/awslabs/soci-snapshotter/soci/store"
	"github.com/urfave/cli"
)

const (
	buildToolIdentifier = "AWS SOCI CLI v0.1"
	spanSizeFlag        = "span-size"
	minLayerSizeFlag    = "min-layer-size"
	optimizationFlag    = "optimizations"
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
			Usage: "Span size that soci index uses to segment layer data. Default is 4 MiB",
			Value: 1 << 22,
		},
		cli.Int64Flag{
			Name:  minLayerSizeFlag,
			Usage: "Minimum layer size to build zTOC for. Smaller layers won't have zTOC and not lazy pulled. Default is 10 MiB.",
			Value: 10 << 20,
		},
		cli.StringSliceFlag{
			Name:  optimizationFlag,
			Usage: fmt.Sprintf("(Experimental) Enable optional optimizations. Valid values are %v", soci.Optimizations),
		},
	),
	Action: func(cliContext *cli.Context) error {
		srcRef := cliContext.Args().Get(0)
		if srcRef == "" {
			return errors.New("source image needs to be specified")
		}

		var optimizations []soci.Optimization
		for _, o := range cliContext.StringSlice(optimizationFlag) {
			optimization, err := soci.ParseOptimization(o)
			if err != nil {
				return err
			}
			optimizations = append(optimizations, optimization)
		}

		client, ctx, cancel, err := internal.NewClient(cliContext)
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

		ctx, blobStore, err := store.NewContentStore(ctx, internal.ContentStoreOptions(cliContext)...)
		if err != nil {
			return err
		}

		ps, err := internal.GetPlatforms(ctx, cliContext, srcImg, cs)
		if err != nil {
			return err
		}

		artifactsDb, err := soci.NewDB(soci.ArtifactsDbPath())
		if err != nil {
			return err
		}

		builderOpts := []soci.BuilderOption{
			soci.WithMinLayerSize(minLayerSize),
			soci.WithSpanSize(spanSize),
			soci.WithBuildToolIdentifier(buildToolIdentifier),
			soci.WithOptimizations(optimizations),
			soci.WithArtifactsDb(artifactsDb),
		}

		builder, err := soci.NewIndexBuilder(cs, blobStore, builderOpts...)

		if err != nil {
			return err
		}

		for _, plat := range ps {
			_, err = builder.Build(ctx, srcImg, soci.WithPlatform(plat))
			if err != nil {
				return err
			}
		}

		return nil
	},
}
