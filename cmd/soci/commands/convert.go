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
	"strings"

	"github.com/awslabs/soci-snapshotter/cmd/soci/commands/global"
	"github.com/awslabs/soci-snapshotter/cmd/soci/commands/internal"
	"github.com/awslabs/soci-snapshotter/soci"
	"github.com/awslabs/soci-snapshotter/soci/store"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/reference"
	"github.com/urfave/cli/v3"
)

const (
	standaloneFlag   = "standalone"
	outputFormatFlag = "format"

	outputFormatOCIArchive = "oci-archive"
	outputFormatOCIDir     = "oci-dir"
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
//
// In standalone mode, the command reads an OCI image layout (tar or directory)
// and writes a converted OCI image layout (tar or directory) without requiring containerd.
var ConvertCommand = &cli.Command{
	Name:      "convert",
	Usage:     "convert an OCI image to a SOCI enabled image",
	ArgsUsage: "[flags] <image_ref> <dest_ref>",
	Flags: slices.Concat(
		internal.PlatformFlags,
		createZtocFlags,
		internal.PrefetchFlags,
		[]cli.Flag{
			&cli.BoolFlag{
				Name:  standaloneFlag,
				Usage: "Run in standalone mode without containerd runtime. In this mode, the command reads an OCI image layout (tar or directory) and writes a converted OCI image layout without requiring a running containerd instance.",
			},
			&cli.StringFlag{
				Name:  outputFormatFlag,
				Usage: "Output format for standalone mode: oci-archive (tar) or oci-dir (directory).",
				Value: outputFormatOCIArchive,
				Validator: func(s string) error {
					if s != outputFormatOCIArchive && s != outputFormatOCIDir {
						return fmt.Errorf("unsupported output format %q: must be %q or %q", s, outputFormatOCIArchive, outputFormatOCIDir)
					}
					return nil
				},
			},
		}),
	Action: func(ctx context.Context, cmd *cli.Command) error {
		src := cmd.Args().Get(0)
		if src == "" {
			return errors.New("source image needs to be specified")
		}

		dst := cmd.Args().Get(1)
		if dst == "" {
			return errors.New("destination needs to be specified")
		}

		if cmd.Bool(standaloneFlag) {
			return runStandaloneConvert(ctx, cmd, src, dst)
		}

		err := verifyRef(dst)
		if err != nil {
			return fmt.Errorf("%w: %w", ErrInvalidDestRef, err)
		}

		client, ctx, cancel, err := internal.NewClient(ctx, cmd)
		if err != nil {
			return err
		}
		defer cancel()

		cs := client.ContentStore()
		is := client.ImageService()
		srcImg, err := is.Get(ctx, src)
		if err != nil {
			return err
		}

		blobStore, err := store.NewContentStore(internal.ContentStoreOptions(ctx, cmd)...)
		if err != nil {
			return err
		}

		artifactsDb, err := soci.NewDB(soci.ArtifactsDbPath(cmd.String(global.RootFlag)))
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
			Name:   dst,
			Target: *desc,
		}
		img, err := is.Get(ctx, dst)
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

// runStandaloneConvert runs the convert command in standalone mode (without containerd).
// It reads an OCI image layout (tar or directory), performs the SOCI conversion, and
// writes the result as an OCI image layout tar or directory based on the --format flag.
func runStandaloneConvert(ctx context.Context, cmd *cli.Command, inputPath string, outputPath string) error {
	format := cmd.String(outputFormatFlag)

	ociLayoutDir, err := os.MkdirTemp("", "soci-oci-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(ociLayoutDir)

	imageInfo, err := internal.LoadImage(ctx, inputPath, ociLayoutDir)
	if err != nil {
		return err
	}

	artifactsDir, err := os.MkdirTemp("", "soci-artifacts-*")
	if err != nil {
		return fmt.Errorf("failed to create artifacts temp directory: %w", err)
	}
	defer os.RemoveAll(artifactsDir)

	artifactsDb, err := soci.NewDB(soci.ArtifactsDbPath(artifactsDir))
	if err != nil {
		return fmt.Errorf("failed to create artifacts database: %w", err)
	}

	builderOpts, err := parseBuilderOptions(cmd)
	if err != nil {
		return err
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

	convertedDesc, err := builder.Convert(batchCtx, imageInfo.Image, soci.ConvertWithPlatforms(requestedPlatforms...))
	if err != nil {
		return err
	}

	if format == outputFormatOCIDir {
		return internal.SaveImageToDir(ociLayoutDir, *convertedDesc, outputPath)
	}
	return internal.SaveImageToTar(ctx, imageInfo.ContentStore, *convertedDesc, outputPath)
}

func parseBuilderOptions(cmd *cli.Command) ([]soci.BuilderOption, error) {
	var optimizations []soci.Optimization
	for _, o := range cmd.StringSlice(optimizationFlag) {
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
