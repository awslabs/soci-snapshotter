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
	"fmt"
	"io"
	"os"
	"text/tabwriter"

	"github.com/awslabs/soci-snapshotter/soci"
	"github.com/containerd/containerd/cmd/ctr/commands"
	"github.com/containerd/containerd/platforms"
	"github.com/urfave/cli"
)

func indexFilter(ae *soci.ArtifactEntry) bool {
	return ae.Type == soci.ArtifactEntryTypeIndex
}

func originalDigestFilter(digest string) func(*soci.ArtifactEntry) bool {
	return func(ae *soci.ArtifactEntry) bool {
		return ae.Type == soci.ArtifactEntryTypeIndex && ae.OriginalDigest == digest
	}
}

var listCommand = cli.Command{
	Name:    "list",
	Usage:   "list indices",
	Aliases: []string{"ls"},
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "ref",
			Usage: "filter indices to those that are associated with a specific image ref",
		},
		cli.BoolFlag{
			Name:  "quiet, q",
			Usage: "only display the index digests",
		},
	},
	Action: func(cliContext *cli.Context) error {
		var artifacts []*soci.ArtifactEntry
		ref := cliContext.String("ref")
		quiet := cliContext.Bool("quiet")

		filter := indexFilter
		client, ctx, cancel, err := commands.NewClient(cliContext)
		if err != nil {
			return err
		}
		defer cancel()

		is := client.ImageService()
		if ref != "" {
			img, err := is.Get(ctx, ref)
			if err != nil {
				return err
			}

			cs := client.ContentStore()
			desc, err := soci.GetImageManifestDescriptor(ctx, cs, img.Target, platforms.Default())
			if err != nil {
				return err
			}
			filter = originalDigestFilter(desc.Digest.String())
		}

		db, err := soci.NewDB()
		if err != nil {
			return err
		}
		db.Walk(func(ae *soci.ArtifactEntry) error {
			if filter(ae) {
				artifacts = append(artifacts, ae)
			}
			return nil
		})

		if quiet {
			for _, ae := range artifacts {
				os.Stdout.Write([]byte(fmt.Sprintf("%s\n", ae.Digest)))
			}
			return nil
		}

		writer := tabwriter.NewWriter(os.Stdout, 8, 8, 4, ' ', 0)
		writer.Write([]byte("DIGEST\tSIZE\tIMAGE REF\tPLATFORM\tORAS ARTIFACT\n"))

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
	w.Write([]byte(fmt.Sprintf(
		"%s\t%d\t%s\t%s\t%v\t\n",
		ae.Digest,
		ae.Size,
		imageRef,
		ae.Platform,
		ae.MediaType == soci.ORASManifestMediaType,
	)))
}
