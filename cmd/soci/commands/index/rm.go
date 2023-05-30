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
	"context"
	"fmt"

	"github.com/awslabs/soci-snapshotter/soci"
	"github.com/containerd/containerd/cmd/ctr/commands"
	"github.com/urfave/cli"
)

var rmCommand = cli.Command{
	Name:        "remove",
	Aliases:     []string{"rm"},
	Usage:       "remove index manifests",
	Description: "remove one or more index manifests from local db",
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "ref",
			Usage: "only remove index manifests that are associated with a specific image ref",
		},
		cli.BoolFlag{
			Name:  "keep-orphan-ztocs, k",
			Usage: "skip deleting newly orphaned zTOCs",
		},
	},
	Action: func(cliContext *cli.Context) error {
		args := cliContext.Args()
		ref := cliContext.String("ref")
		keepOrphanZtocs := cliContext.Bool("keep-orphan-ztocs")

		if len(args) != 0 && ref != "" {
			return fmt.Errorf("please provide either index digest(s) or image ref, but not both")
		}

		db, err := soci.NewDB(soci.ArtifactsDbPath())
		if err != nil {
			return err
		}
		if ref == "" {
			// one or more index manifest digests specified as command line arguments
			ctx, cancelTimeout := context.WithTimeout(context.Background(), cliContext.GlobalDuration("timeout"))
			defer cancelTimeout()

			err = soci.RemoveIndexes(ctx, args, !keepOrphanZtocs)
			if err != nil {
				return err
			}
		} else {
			// one image ref specified as command line argument
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
			indexDigests, err := db.GetIndexDigestsByImageDigest(img.Target.Digest.String())
			if err != nil {
				return err
			}
			err = soci.RemoveIndexes(ctx, *indexDigests, !keepOrphanZtocs)
			if err != nil {
				return err
			}
		}
		return nil
	},
}
