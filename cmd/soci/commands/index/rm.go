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

	"github.com/awslabs/soci-snapshotter/soci"
	"github.com/containerd/containerd/cmd/ctr/commands"
	"github.com/urfave/cli"
)

var rmCommand = cli.Command{
	Name:        "remove",
	Aliases:     []string{"rm"},
	Usage:       "remove indices",
	Description: "remove an index from local db",
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "ref",
			Usage: "only remove indices that are associated with a specific image ref",
		},
	},
	Action: func(cliContext *cli.Context) error {
		args := cliContext.Args()
		ref := cliContext.String("ref")

		if len(args) != 0 && ref != "" {
			return fmt.Errorf("please provide either index digests or image ref, but not both")
		}

		db, err := soci.NewDB(soci.ArtifactsDbPath())
		if err != nil {
			return err
		}
		if ref == "" {
			for _, desc := range args {
				err := db.RemoveArtifactEntryByIndexDigest(desc)
				if err != nil {
					return err
				}
			}
		} else {
			client, ctx, cancel, err := commands.NewClient(cliContext)
			if err != nil {
				return err
			}
			defer cancel()

			is := client.ImageService()
			img, err := is.Get(ctx, ref)
			if err != nil {
				return err
			}
			return db.RemoveArtifactEntryByImageDigest(img.Target.Digest.String())
		}
		return nil
	},
}
