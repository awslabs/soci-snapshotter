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

	"github.com/awslabs/soci-snapshotter/cmd/soci/commands/internal"
	"github.com/awslabs/soci-snapshotter/soci"
	"github.com/awslabs/soci-snapshotter/soci/store"
	"github.com/opencontainers/go-digest"
	"github.com/urfave/cli/v2"
)

var rmCommand = &cli.Command{
	Name:        "remove",
	Aliases:     []string{"rm"},
	Usage:       "remove indices",
	Description: "remove an index from local db, and from content store if supported",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "ref",
			Usage: "only remove indices that are associated with a specific image ref",
		},
	},
	Action: func(cliContext *cli.Context) error {
		args := cliContext.Args()
		ref := cliContext.String("ref")

		if args.Len() != 0 && ref != "" {
			return fmt.Errorf("please provide either index digests or image ref, but not both")
		}

		contentStore, err := store.NewContentStore(internal.ContentStoreOptions(cliContext)...)
		if err != nil {
			return fmt.Errorf("cannot create local content store: %w", err)
		}

		db, err := soci.NewDB(soci.ArtifactsDbPath(cliContext.String("root")))
		if err != nil {
			return err
		}
		if ref == "" {
			ctx, cancel := internal.AppContext(cliContext)
			defer cancel()

			byteArgs := make([][]byte, args.Len())
			for i, arg := range args.Slice() {
				byteArgs[i] = []byte(arg)
			}
			err = removeArtifactsAndContent(ctx, db, contentStore, byteArgs)
			if err != nil {
				return err
			}
		} else {
			client, ctx, cancel, err := internal.NewClient(cliContext)
			if err != nil {
				return err
			}
			defer cancel()

			is := client.ImageService()
			img, err := is.Get(ctx, ref)
			if err != nil {
				return err
			}
			entries, err := db.GetArtifactEntriesByImageDigest(img.Target.Digest.String())
			if err != nil {
				return err
			}
			err = removeArtifactsAndContent(ctx, db, contentStore, entries)
			if err != nil {
				return err
			}
		}
		return nil
	},
}

// removeArtifactsAndContent takes a list of content digests and removes them from the artifact db and content store
func removeArtifactsAndContent(ctx context.Context, db *soci.ArtifactsDb, contentStore store.Store, digests [][]byte) error {
	for _, dgst := range digests {
		err := db.RemoveArtifactEntryByIndexDigest(dgst)
		if err != nil {
			return err
		}
		dgst, err := digest.Parse(string(dgst))
		if err != nil {
			return err
		}
		err = contentStore.Delete(ctx, dgst)
		if err != nil {
			return err
		}
	}
	return nil
}
