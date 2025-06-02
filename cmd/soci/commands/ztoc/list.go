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

package ztoc

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/awslabs/soci-snapshotter/cmd/soci/commands/internal"
	"github.com/awslabs/soci-snapshotter/soci"
	"github.com/containerd/containerd/images"
	"github.com/containerd/platforms"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/urfave/cli/v2"
)

var listCommand = &cli.Command{
	Name:        "list",
	Description: "list ztocs",
	Aliases:     []string{"ls"},
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "ztoc-digest",
			Usage: "filter ztocs by digest",
		},
		&cli.StringFlag{
			Name:  "image-ref",
			Usage: "filter ztocs to those that are associated with a specific image",
		},
		&cli.BoolFlag{
			Name:    "verbose",
			Aliases: []string{"v"},
			Usage:   "display extra debugging messages",
		},
		&cli.BoolFlag{
			Name:    "quiet",
			Aliases: []string{"q"},
			Usage:   "only display the index digests",
		},
	},
	Action: func(cliContext *cli.Context) error {
		db, err := soci.NewDB(soci.ArtifactsDbPath(cliContext.String("root")))
		if err != nil {
			return err
		}
		ztocDgst := cliContext.String("ztoc-digest")
		imgRef := cliContext.String("image-ref")
		verbose := cliContext.Bool("verbose")
		quiet := cliContext.Bool("quiet")

		var artifacts []*soci.ArtifactEntry
		if imgRef == "" {
			_, cancel := internal.AppContext(cliContext)
			defer cancel()
			db.Walk(func(ae *soci.ArtifactEntry) error {
				if ae.Type == soci.ArtifactEntryTypeLayer && (ztocDgst == "" || ae.Digest == ztocDgst) {
					artifacts = append(artifacts, ae)
				}
				return nil
			})
		} else {
			client, ctx, cancel, err := internal.NewClient(cliContext)
			if err != nil {
				return err
			}
			defer cancel()

			is := client.ImageService()
			img, err := is.Get(ctx, imgRef)
			if err != nil {
				return err
			}
			platform, err := images.Platforms(ctx, client.ContentStore(), img.Target)
			if err != nil {
				return err
			}
			var layers []ocispec.Descriptor
			for _, p := range platform {
				manifest, err := images.Manifest(ctx, client.ContentStore(), img.Target, platforms.OnlyStrict(p))
				if err != nil && verbose {
					// print a warning message if a manifest can't be resolved
					// continue looking for manifests of other platforms
					fmt.Printf("no image manifest for platform %s/%s. err: %v\n", p.Architecture, p.OS, err)
				} else {
					layers = append(layers, manifest.Layers...)
				}
			}
			if len(layers) == 0 {
				return fmt.Errorf("no image layers. could not filter ztoc")
			}

			db.Walk(func(ae *soci.ArtifactEntry) error {
				if ae.Type == soci.ArtifactEntryTypeLayer {
					if ztocDgst == "" {
						// add all ztocs associated with the image
						for _, l := range layers {
							if ae.OriginalDigest == l.Digest.String() {
								artifacts = append(artifacts, ae)
							}
						}
					} else {
						// only add the specific ztoc if the ztoc is with an image layer
						for _, l := range layers {
							if ae.Digest == ztocDgst && ae.OriginalDigest == l.Digest.String() {
								artifacts = append(artifacts, ae)
							}
						}
					}
				}
				return nil
			})
			if ztocDgst != "" && len(artifacts) == 0 {
				return fmt.Errorf("the specified ztoc doesn't exist or it's not with the specified image")
			}
		}

		if quiet {
			for _, ae := range artifacts {
				fmt.Fprintf(os.Stdout, "%s\n", ae.Digest)
			}
			return nil
		}

		writer := tabwriter.NewWriter(os.Stdout, 8, 8, 4, ' ', 0)
		writer.Write([]byte("DIGEST\tSIZE\tLAYER DIGEST\t\n"))
		for _, artifact := range artifacts {
			fmt.Fprintf(writer, "%s\t%d\t%s\t\n", artifact.Digest, artifact.Size, artifact.OriginalDigest)
		}
		writer.Flush()
		return nil
	},
}
