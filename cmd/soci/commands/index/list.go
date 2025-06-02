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

package index

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"text/tabwriter"
	"time"

	"github.com/awslabs/soci-snapshotter/cmd/soci/commands/internal"
	"github.com/awslabs/soci-snapshotter/soci"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/images"
	"github.com/containerd/platforms"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/urfave/cli/v2"
)

type filter func(ae *soci.ArtifactEntry) bool

func indexFilter(ae *soci.ArtifactEntry) bool {
	return ae.Type == soci.ArtifactEntryTypeIndex
}

func platformFilter(platform specs.Platform) filter {
	return func(ae *soci.ArtifactEntry) bool {
		return indexFilter(ae) && ae.Platform == platforms.Format(platform)
	}
}

func originalDigestFilter(digest string) filter {
	return func(ae *soci.ArtifactEntry) bool {
		return indexFilter(ae) && ae.OriginalDigest == digest
	}
}

func anyMatch(fns []filter) filter {
	return func(ae *soci.ArtifactEntry) bool {
		for _, f := range fns {
			if f(ae) {
				return true
			}
		}
		return false
	}
}

var listCommand = &cli.Command{
	Name:    "list",
	Usage:   "list indices",
	Aliases: []string{"ls"},
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "ref",
			Usage: "filter indices to those that are associated with a specific image ref",
		},
		&cli.BoolFlag{
			Name:    "quiet",
			Aliases: []string{"q"},
			Usage:   "only display the index digests",
		},
		&cli.StringSliceFlag{
			Name:    "platform",
			Aliases: []string{"p"},
			Usage:   "filter indices to a specific platform",
		},
	},
	Action: func(cliContext *cli.Context) error {
		var artifacts []*soci.ArtifactEntry
		ref := cliContext.String("ref")
		quiet := cliContext.Bool("quiet")
		var plats []specs.Platform
		for _, p := range cliContext.StringSlice("platform") {
			pp, err := platforms.Parse(p)
			if err != nil {
				return err
			}
			plats = append(plats, pp)
		}

		client, ctx, cancel, err := internal.NewClient(cliContext)
		if err != nil {
			return err
		}
		defer cancel()

		f := indexFilter

		is := client.ImageService()
		if ref != "" {
			img, err := is.Get(ctx, ref)
			if err != nil {
				return err
			}

			if len(plats) == 0 {
				plats, err = images.Platforms(ctx, client.ContentStore(), img.Target)
				if err != nil {
					return err
				}
			}

			cs := client.ContentStore()
			var filters []filter
			for _, plat := range plats {
				desc, err := soci.GetImageManifestDescriptor(ctx, cs, img.Target, platforms.OnlyStrict(plat))
				if err != nil {
					if errors.Is(err, errdefs.ErrNotFound) {
						return fmt.Errorf("image manifest for platform %s: %w", platforms.Format(plat), err)
					}
					return err
				}
				filters = append(filters, originalDigestFilter(desc.Digest.String()))
			}
			f = anyMatch(filters)
		} else if len(plats) != 0 {
			var filters []filter
			for _, plat := range plats {
				filters = append(filters, platformFilter(plat))
			}
			f = anyMatch(filters)
		}

		db, err := soci.NewDB(soci.ArtifactsDbPath(cliContext.String("root")))
		if err != nil {
			return err
		}
		db.Walk(func(ae *soci.ArtifactEntry) error {
			if f(ae) {
				artifacts = append(artifacts, ae)
			}
			return nil
		})

		sort.Slice(artifacts, func(i, j int) bool {
			return artifacts[i].CreatedAt.After(artifacts[j].CreatedAt)
		})

		if quiet {
			for _, ae := range artifacts {
				fmt.Fprintf(os.Stdout, "%s\n", ae.Digest)
			}
			return nil
		}

		writer := tabwriter.NewWriter(os.Stdout, 8, 8, 4, ' ', 0)
		writer.Write([]byte("DIGEST\tSIZE\tIMAGE REF\tPLATFORM\tMEDIA TYPE\tMANIFEST VERSION\tCREATED\t\n"))

		for _, ae := range artifacts {
			imgs, _ := is.List(ctx, fmt.Sprintf("target.digest==%s", ae.ImageDigest))
			if len(imgs) > 0 {
				for _, img := range imgs {
					writeArtifactEntry(writer, ae, img.Name)
				}
			} else {
				writeArtifactEntry(writer, ae, "")
			}
		}
		writer.Flush()
		return nil
	},
}

func writeArtifactEntry(w io.Writer, ae *soci.ArtifactEntry, imageRef string) {
	version := ""
	switch ae.ArtifactType {
	case soci.SociIndexArtifactTypeV1:
		version = "v1"
	case soci.SociIndexArtifactTypeV2:
		version = "v2"
	}
	fmt.Fprintf(w,
		"%s\t%d\t%s\t%s\t%s\t%s\t%s\t\n",
		ae.Digest,
		ae.Size,
		imageRef,
		ae.Platform,
		ae.MediaType,
		version,
		getDuration(ae.CreatedAt),
	)
}

func getDuration(t time.Time) string {
	if t.IsZero() {
		return "n/a"
	}
	return fmt.Sprintf("%s ago", time.Since(t).Round(time.Second).String())
}
