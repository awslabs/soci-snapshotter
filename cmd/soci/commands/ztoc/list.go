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
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"text/tabwriter"

	"github.com/awslabs/soci-snapshotter/cmd/soci/commands/global"
	"github.com/awslabs/soci-snapshotter/cmd/soci/commands/internal"
	"github.com/awslabs/soci-snapshotter/soci"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/images"
	"github.com/containerd/log"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/urfave/cli/v3"
)

const (
	ztocDigestFlag = "ztoc-digest"
	imageRefFlag   = "image-ref"
	quietFlag      = "quiet"
)

var listCommand = &cli.Command{
	Name:        "list",
	Description: "list ztocs",
	Aliases:     []string{"ls"},
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  ztocDigestFlag,
			Usage: "filter ztocs by digest",
		},
		&cli.StringFlag{
			Name:  imageRefFlag,
			Usage: "filter ztocs to those that are associated with a specific image",
		},
		&cli.BoolFlag{
			Name:    quietFlag,
			Aliases: []string{"q"},
			Usage:   "only display the index digests",
		},
	},
	Action: func(ctx context.Context, cmd *cli.Command) error {
		db, err := soci.NewDB(soci.ArtifactsDbPath(cmd.String(global.RootFlag)))
		if err != nil {
			return err
		}

		ztocDgst := cmd.String(ztocDigestFlag)
		imgRef := cmd.String(imageRefFlag)
		quiet := cmd.Bool(quietFlag)

		var artifacts []*soci.ArtifactEntry
		if imgRef == "" {
			_, cancel := internal.AppContext(ctx, cmd)
			defer cancel()
			db.Walk(func(ae *soci.ArtifactEntry) error {
				if ae.Type == soci.ArtifactEntryTypeLayer && (ztocDgst == "" || ae.Digest == ztocDgst) {
					artifacts = append(artifacts, ae)
				}
				return nil
			})
		} else {
			client, ctx, cancel, err := internal.NewClient(ctx, cmd)
			if err != nil {
				return err
			}
			defer cancel()

			cs := client.ContentStore()
			is := client.ImageService()
			img, err := is.Get(ctx, imgRef)
			if err != nil {
				return err
			}

			supportedPlatforms, err := images.Platforms(ctx, client.ContentStore(), img.Target)
			if err != nil {
				return fmt.Errorf("failed to get supported platforms for the image: %w", err)
			}
			indexInfos, _, err := soci.GetIndexDescriptorCollection(ctx, cs, db, img, supportedPlatforms)
			if err != nil {
				return fmt.Errorf("failed to get soci indexes for the image: %w", err)
			}

			var ztocDescs []ocispec.Descriptor
			for _, indexInfo := range indexInfos {
				readerAt, err := cs.ReaderAt(ctx, indexInfo.Descriptor)
				if err != nil {
					if errors.Is(err, errdefs.ErrNotFound) {
						log.G(ctx).Debug("a soci index was found in the artifact store but not in the content store, skipping")
						continue
					}
					return fmt.Errorf("failed to read a soci index with digest %s from the content store: %w", indexInfo.Descriptor.Digest, err)
				}
				var sociIndex soci.Index
				err = soci.DecodeIndex(io.NewSectionReader(readerAt, 0, indexInfo.Descriptor.Size), &sociIndex)
				if err != nil {
					return fmt.Errorf("failed to decode a soci index with digest %s: %w", indexInfo.Descriptor.Digest, err)
				}
				for _, blob := range sociIndex.Blobs {
					if blob.MediaType != soci.SociLayerMediaType {
						continue
					}
					ztocDescs = append(ztocDescs, blob)
				}
			}

			// at this point we already have the ztoc digests for the image
			// but we have to query to artifacts db to get the associated layer digests
			for _, ztocDesc := range ztocDescs {
				entry, err := db.GetArtifactEntry(string(ztocDesc.Digest))
				if err != nil {
					return fmt.Errorf("failed to get ztoc from artifacts store (try running \"soci rebuild-db\" first): %w", err)
				}
				if ztocDgst == "" || ztocDgst == entry.Digest {
					artifacts = append(artifacts, entry)
				}
			}

			if ztocDgst != "" && len(artifacts) == 0 {
				return fmt.Errorf("the specified ztoc doesn't exist or is not associated with the specified image")
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
