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
	"errors"
	"fmt"

	"github.com/awslabs/soci-snapshotter/cmd/soci/commands/internal"
	"github.com/awslabs/soci-snapshotter/soci"
	"github.com/awslabs/soci-snapshotter/soci/store"
	"github.com/containerd/platforms"
	"github.com/urfave/cli/v3"
)

const (
	// buildToolIdentifier is placed in annotations of the SOCI index
	// to help identify how a SOCI index was created
	buildToolIdentifier         = "AWS SOCI CLI v0.2"
	spanSizeFlag                = "span-size"
	minLayerSizeFlag            = "min-layer-size"
	optimizationFlag            = "optimizations"
	forceRecreateZtocsFlag      = "force"
	forceRecreateZtocsFlagShort = "f"
	sociIndexGCLabel            = "containerd.io/gc.ref.content.soci-index"
)

// CreateCommand creates SOCI index for an image
// Output of this command is SOCI layers and SOCI index stored in a local directory
// SOCI layer is named as <image-layer-digest>.soci.layer
// SOCI index is named as <image-manifest-digest>.soci.index
var CreateCommand = &cli.Command{
	Name:      "create",
	Usage:     "create SOCI index",
	ArgsUsage: "[flags] <image_ref>",
	Flags: append(
		internal.PlatformFlags,
		&cli.Int64Flag{
			Name:  spanSizeFlag,
			Usage: "Span size that soci index uses to segment layer data. Default is 4 MiB",
			Value: 1 << 22,
		},
		&cli.Int64Flag{
			Name:  minLayerSizeFlag,
			Usage: "Minimum layer size to build zTOC for. Smaller layers won't have zTOC and not lazy pulled. Default is 10 MiB.",
			Value: 10 << 20,
		},
		&cli.StringSliceFlag{
			Name:  optimizationFlag,
			Usage: fmt.Sprintf("(Experimental) Enable optional optimizations. Valid values are %v", soci.Optimizations),
		},
		&cli.BoolFlag{
			Name:    forceRecreateZtocsFlag,
			Usage:   "Force recreate zTOCs for layers even if they already exist. Defaults to false.",
			Value:   false,
			Aliases: []string{forceRecreateZtocsFlagShort},
		},
	),
	Action: func(ctx context.Context, cmd *cli.Command) error {
		srcRef := cmd.Args().Get(0)
		if srcRef == "" {
			return errors.New("source image needs to be specified")
		}

		var optimizations []soci.Optimization
		for _, o := range cmd.StringSlice(optimizationFlag) {
			optimization, err := soci.ParseOptimization(o)
			if err != nil {
				return err
			}
			optimizations = append(optimizations, optimization)
		}

		client, ctx, cancel, err := internal.NewClient(ctx, cmd)
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

		spanSize := cmd.Int64(spanSizeFlag)
		minLayerSize := cmd.Int64(minLayerSizeFlag)
		forceRecreateZtocs := cmd.Bool(forceRecreateZtocsFlag)

		blobStore, err := store.NewContentStore(internal.ContentStoreOptions(ctx, cmd)...)
		if err != nil {
			return err
		}

		ps, err := internal.GetPlatforms(ctx, cmd, srcImg, cs)
		if err != nil {
			return err
		}
		if len(ps) == 0 {
			ps = append(ps, platforms.DefaultSpec())
		}

		artifactsDb, err := soci.NewDB(soci.ArtifactsDbPath(cmd.String("root")))
		if err != nil {
			return err
		}

		builderOpts := []soci.BuilderOption{
			soci.WithMinLayerSize(minLayerSize),
			soci.WithSpanSize(spanSize),
			soci.WithBuildToolIdentifier(buildToolIdentifier),
			soci.WithOptimizations(optimizations),
			soci.WithArtifactsDb(artifactsDb),
			soci.WithForceRecreateZtocs(forceRecreateZtocs),
		}

		builder, err := soci.NewIndexBuilder(cs, blobStore, builderOpts...)
		if err != nil {
			return err
		}

		for _, plat := range ps {
			batchCtx, done, err := blobStore.BatchOpen(ctx)
			if err != nil {
				return err
			}
			defer done(ctx)

			indexWithMetadata, err := builder.Build(batchCtx, srcImg, soci.WithPlatform(plat), soci.WithNoGarbageCollectionLabel())
			if err != nil {
				return err
			}

			if srcImg.Labels == nil {
				srcImg.Labels = make(map[string]string)
			}
			srcImg.Labels[sociIndexGCLabel] = indexWithMetadata.Desc.Digest.String()
			is.Update(ctx, srcImg, "labels")
		}

		return nil
	},
}
