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
	"os"
	"slices"

	"github.com/awslabs/soci-snapshotter/cmd/soci/commands/global"
	"github.com/awslabs/soci-snapshotter/cmd/soci/commands/internal"
	"github.com/awslabs/soci-snapshotter/soci"
	"github.com/awslabs/soci-snapshotter/soci/store"
	"github.com/containerd/containerd/v2/core/content"
	"github.com/containerd/containerd/v2/core/images"
	local "github.com/containerd/containerd/v2/plugins/content/local"
	"github.com/containerd/platforms"
	"github.com/urfave/cli/v3"
)

const (
	// buildToolIdentifier is placed in annotations of the SOCI index
	// to help identify how a SOCI index was created
	buildToolIdentifier = "AWS SOCI CLI v0.2"
	sociIndexGCLabel    = "containerd.io/gc.ref.content.soci-index"

	spanSizeFlag           = "span-size"
	minLayerSizeFlag       = "min-layer-size"
	optimizationFlag       = "optimizations"
	forceRecreateZtocsFlag = "force"
)

// SourceFlag and SourceRegistry re-export internal.SourceFlag/
// internal.SourceRegistry so cmd/soci/main.go - which lives outside the
// internal package's visibility boundary - can check whether create/push
// were invoked with --source=registry, to skip requiring the snapshotter
// root path in that mode (see main.go's Before hook).
const (
	SourceFlag     = internal.SourceFlag
	SourceRegistry = internal.SourceRegistry
)

var createZtocFlags = []cli.Flag{
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
		Aliases: []string{"f"},
	},
}

// CreateCommand creates SOCI index for an image
// Output of this command is SOCI layers and SOCI index stored in a local directory
// SOCI layer is named as <image-layer-digest>.soci.layer
// SOCI index is named as <image-manifest-digest>.soci.index
var CreateCommand = &cli.Command{
	Name:      "create",
	Usage:     "create SOCI index",
	ArgsUsage: "[flags] <image_ref>",
	Flags:     slices.Concat(internal.PlatformFlags, internal.SourceFlags, internal.RegistryFlags, createZtocFlags, internal.PrefetchFlags),
	Action: func(ctx context.Context, cmd *cli.Command) error {
		srcRef := cmd.Args().Get(0)
		if srcRef == "" {
			return errors.New("source image needs to be specified")
		}

		source := cmd.String(internal.SourceFlag)
		if !internal.SupportedArg(source, internal.SupportedSourceOptions) {
			return fmt.Errorf("unexpected value for flag %s: %s, expected types %v",
				internal.SourceFlag, source, internal.SupportedSourceOptions)
		}
		if source == internal.SourceRegistry && store.ContentStoreType(cmd.String(global.ContentStoreFlag)) == store.ContainerdContentStoreType {
			return fmt.Errorf("--%s=%s cannot be combined with --%s=%s: there is no containerd content store to write into",
				internal.SourceFlag, internal.SourceRegistry, global.ContentStoreFlag, store.ContainerdContentStoreType)
		}

		var optimizations []soci.Optimization
		for _, o := range cmd.StringSlice(optimizationFlag) {
			optimization, err := soci.ParseOptimization(o)
			if err != nil {
				return err
			}
			optimizations = append(optimizations, optimization)
		}

		var (
			cs     content.Store
			srcImg images.Image
			is     images.Store       // only set (and only used) in containerd mode
			cancel context.CancelFunc = func() {}
		)
		defer func() { cancel() }()

		if source == internal.SourceRegistry {
			// The pulled image content (manifest/config/layers) is only
			// needed transiently, to compute the ztoc during this
			// invocation - nothing needs it afterward, unlike the SOCI
			// output below, which push must be able to find later. A temp
			// dir means this never needs --root to point somewhere
			// writable, and it's cleaned up when the command exits.
			contentDir, err := os.MkdirTemp("", "soci-create-registry-content-*")
			if err != nil {
				return fmt.Errorf("could not create temp content directory: %w", err)
			}
			defer os.RemoveAll(contentDir)

			localStore, err := local.NewStore(contentDir)
			if err != nil {
				return fmt.Errorf("could not create local content store at %s: %w", contentDir, err)
			}
			cs = localStore

			srcImg, err = internal.PopulateFromRegistry(ctx, cmd, srcRef, cs)
			if err != nil {
				return err
			}
		} else {
			client, actxCtx, actxCancel, err := internal.NewClient(ctx, cmd)
			if err != nil {
				return err
			}
			ctx = actxCtx
			cancel = actxCancel

			cs = client.ContentStore()
			is = client.ImageService()
			srcImg, err = is.Get(ctx, srcRef)
			if err != nil {
				return err
			}
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

		artifactsDb, err := soci.NewDB(soci.ArtifactsDbPath(cmd.String(global.RootFlag)))
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

		allPrefetchFiles, err := internal.ParsePrefetchFiles(cmd)
		if err != nil {
			return fmt.Errorf("failed to parse prefetch files: %w", err)
		}
		if len(allPrefetchFiles) > 0 {
			builderOpts = append(builderOpts, soci.WithPrefetchPaths(allPrefetchFiles))
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

			if is != nil {
				// Label the image in containerd so its GC won't collect the
				// SOCI index we just wrote. There's no containerd image
				// record to label in registry-source mode.
				if srcImg.Labels == nil {
					srcImg.Labels = make(map[string]string)
				}
				srcImg.Labels[sociIndexGCLabel] = indexWithMetadata.Desc.Digest.String()
				is.Update(ctx, srcImg, "labels")
			}
		}

		return nil
	},
}
