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
	"slices"
	"strings"

	"github.com/awslabs/soci-snapshotter/cmd/soci/commands/internal"
	"github.com/awslabs/soci-snapshotter/soci"
	"github.com/awslabs/soci-snapshotter/soci/store"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/reference"
	"github.com/urfave/cli/v3"
)

var ErrInvalidDestRef = errors.New(`the destination image must be a tagged ref of the form "registry/repository:tag"`)

func verifyRef(r string) error {
	if r == "" {
		return errors.New("reference cannot be empty")
	}

	ref, err := reference.Parse(r)
	if err != nil {
		return fmt.Errorf("could not parse reference: %w", err)
	}

	object := ref.Object
	if object == "" {
		return errors.New("reference must be tagged")
	}
	if strings.Contains(object, "@") {
		return errors.New("reference must not contain a digest")
	}

	return nil
}

// ConvertCommand converts an image into a SOCI enabled image.
// The new image is added to the containerd content store and can
// be pushed and deployed like a normal image.
var ConvertCommand = &cli.Command{
	Name:      "convert",
	Usage:     "convert an OCI image to a SOCI enabled image",
	ArgsUsage: "[flags] <image_ref> <dest_ref>",
	Flags: slices.Concat(
		internal.PlatformFlags,
		[]cli.Flag{
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
			&cli.BoolFlag{
				Name:  "standalone",
				Usage: "Run in standalone mode without containerd runtime. In this mode, the command will download the source image, perform the conversion, and push the converted image back to the registry without requiring a running containerd instance.",
			},
			&cli.BoolFlag{
				Name:    "verbose",
				Aliases: []string{"v"},
				Usage:   "Show detailed progress output",
			},
		},
		internal.RegistryFlags,
		internal.PrefetchFlags(),
	),
	Action: func(ctx context.Context, cmd *cli.Command) error {
		srcRef := cmd.Args().Get(0)
		if srcRef == "" {
			return errors.New("source image needs to be specified")
		}

		dstRef := cmd.Args().Get(1)
		if dstRef == "" {
			return errors.New("destination image needs to be specified")
		}

		err := verifyRef(dstRef)
		if err != nil {
			return fmt.Errorf("%w: %w", ErrInvalidDestRef, err)
		}

		if cmd.Bool("standalone") {
			return runStandaloneConvert(ctx, cmd, srcRef, dstRef)
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

		blobStore, err := store.NewContentStore(internal.ContentStoreOptions(ctx, cmd)...)
		if err != nil {
			return err
		}

		artifactsDb, err := soci.NewDB(soci.ArtifactsDbPath(cmd.String("root")))
		if err != nil {
			return err
		}

		builderOpts, err := parseBuilderOptions(cmd)
		if err != nil {
			return err
		}
		builderOpts = append(builderOpts, soci.WithArtifactsDb(artifactsDb))

		builder, err := soci.NewIndexBuilder(cs, blobStore, builderOpts...)
		if err != nil {
			return err
		}

		batchCtx, done, err := blobStore.BatchOpen(ctx)
		if err != nil {
			return err
		}
		defer done(ctx)

		platforms, err := internal.GetPlatforms(ctx, cmd, srcImg, cs)
		if err != nil {
			return err
		}

		desc, err := builder.Convert(batchCtx, srcImg,
			soci.ConvertWithPlatforms(platforms...),
			// Don't set a GC label on the converted OCI Index. We will create an image
			// in containerd that will act as the GC root. This way, the OCI index, SOCI indexes, and
			// images will be removed when the image is deleted in containerd
			soci.ConvertWithNoGarbageCollectionLabels(),
		)
		if err != nil {
			return err
		}

		im := images.Image{
			Name:   dstRef,
			Target: *desc,
		}
		img, err := is.Get(ctx, dstRef)
		if err != nil {
			if !errors.Is(err, errdefs.ErrNotFound) {
				return err
			}
			_, err = is.Create(ctx, im)
			return err
		}
		img.Target = *desc
		_, err = is.Update(ctx, img)

		return err
	},
}

// runStandaloneConvert runs the convert command in standalone mode (without containerd)
func runStandaloneConvert(ctx context.Context, cmd *cli.Command, srcRef string, dstRef string) error {
	verbose := cmd.Bool("verbose")
	if verbose {
		fmt.Printf("Standalone mode: downloading image %s\n", srcRef)
	}

	imageInfo, err := internal.DownloadImageFromRegistry(ctx, cmd, srcRef)
	if err != nil {
		return err
	}
	defer imageInfo.Cleanup()

	builderOpts, err := parseBuilderOptions(cmd)
	if err != nil {
		return err
	}

	artifactsDb, err := soci.NewDB(soci.ArtifactsDbPath(imageInfo.TempDir))
	if err != nil {
		return fmt.Errorf("failed to create artifacts database: %w", err)
	}
	builderOpts = append(builderOpts, soci.WithArtifactsDb(artifactsDb))

	sociStore := &store.SociStore{Store: imageInfo.OrasStore}

	builder, err := soci.NewIndexBuilder(imageInfo.ContentStore, sociStore, builderOpts...)
	if err != nil {
		return err
	}

	requestedPlatforms, err := internal.GetPlatforms(ctx, cmd, imageInfo.Image, imageInfo.ContentStore)
	if err != nil {
		return err
	}

	batchCtx, done, err := sociStore.BatchOpen(ctx)
	if err != nil {
		return err
	}
	defer done(ctx)

	if verbose {
		fmt.Printf("Converting image to SOCI-enabled format with %d platform(s)\n", len(requestedPlatforms))
	}

	convertedDesc, err := builder.Convert(batchCtx, imageInfo.Image, soci.ConvertWithPlatforms(requestedPlatforms...))
	if err != nil {
		return err
	}

	if verbose {
		fmt.Printf("Successfully created SOCI-enabled image: %s\n", convertedDesc.Digest)
	}

	if err := internal.PushConvertedImage(ctx, cmd, imageInfo, *convertedDesc, dstRef); err != nil {
		return err
	}

	if verbose {
		fmt.Println("Standalone mode: SOCI conversion completed successfully")
	}
	return nil
}

func parseBuilderOptions(cmd *cli.Command) ([]soci.BuilderOption, error) {
	var optimizations []soci.Optimization
	for _, o := range cmd.StringSlice("optimizations") {
		optimization, err := soci.ParseOptimization(o)
		if err != nil {
			return nil, err
		}
		optimizations = append(optimizations, optimization)
	}

	builderOpts := []soci.BuilderOption{
		soci.WithMinLayerSize(cmd.Int64(minLayerSizeFlag)),
		soci.WithSpanSize(cmd.Int64(spanSizeFlag)),
		soci.WithBuildToolIdentifier(buildToolIdentifier),
		soci.WithOptimizations(optimizations),
		soci.WithForceRecreateZtocs(cmd.Bool(forceRecreateZtocsFlag)),
	}

	allPrefetchFiles, err := internal.ParsePrefetchFiles(cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to parse prefetch files: %w", err)
	}
	if len(allPrefetchFiles) > 0 {
		builderOpts = append(builderOpts, soci.WithPrefetchPaths(allPrefetchFiles))
	}

	return builderOpts, nil
}
