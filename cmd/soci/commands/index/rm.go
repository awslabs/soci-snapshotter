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
	"errors"
	"fmt"

	"github.com/awslabs/soci-snapshotter/fs/config"
	"github.com/awslabs/soci-snapshotter/soci"
	"github.com/containerd/containerd/cmd/ctr/commands"
	"github.com/opencontainers/go-digest"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/urfave/cli"
	"oras.land/oras-go/v2/content/oci"
)

func fetchIndex(ctx context.Context, storage *oci.Store, digestString string) (*soci.Index, error) {
	dgst, err := digest.Parse(digestString)
	if err != nil {
		return nil, err
	}
	reader, err := storage.Fetch(ctx, v1.Descriptor{Digest: dgst})
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	var index soci.Index
	if err := soci.DecodeIndex(reader, &index); err != nil {
		return nil, err
	}

	return &index, nil

}

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
			ctx, cancelTimeout := context.WithTimeout(context.Background(), cliContext.GlobalDuration("timeout"))
			defer cancelTimeout()
			storage, err := oci.New(config.SociContentStorePath)
			if err != nil {
				return err
			}
			maybeOrphanZtocDigests := make(map[digest.Digest]bool)
			for _, desc := range args {
				index, err := fetchIndex(ctx, storage, desc)
				if err != nil {
					return err
				}

				// add all zTOC digests from the index manifest to the set of potential orphans
				for _, blob := range index.Blobs {
					maybeOrphanZtocDigests[blob.Digest] = true
				}

				// TODO remove the index manifest file

				// remove the index manifest artifact
				err = db.RemoveArtifactEntryByIndexDigest(desc)
				if err != nil {
					return err
				}
			}
			err = db.Walk(func(ae *soci.ArtifactEntry) error {
				if ae.Type == soci.ArtifactEntryTypeIndex {
					index, err := fetchIndex(ctx, storage, ae.Digest)
					if err != nil {
						return err
					}

					// remove still-referenced ztocs from the list of potentially orphaned ztoc digests
					for _, blob := range index.Blobs {
						// FIXME why does go-staticcheck think this guard is unnecessary for just {delete()}?
						if _, ok := maybeOrphanZtocDigests[blob.Digest]; ok {
							fmt.Printf("keeping ztoc digest %s referenced by index manifest %s\n", blob.Digest.String(), ae.Digest)
							delete(maybeOrphanZtocDigests, blob.Digest)
							// bail out early if there are no more potential orphans
							if len(maybeOrphanZtocDigests) == 0 {
								return soci.ErrWalkBailout
							}
						}
					}
				}
				return nil
			})
			if err != nil && !errors.Is(err, soci.ErrWalkBailout) {
				return err
			}

			// all remaining potential orphans are actually orphaned
			for dgst := range maybeOrphanZtocDigests {
				fmt.Printf("removing orphaned ztoc digest %s\n", dgst.String())
				err = db.RemoveArtifactEntryByZtocDigest(dgst.String())
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
			// TODO use the new logic above here as well
			return db.RemoveArtifactEntryByImageDigest(img.Target.Digest.String())
		}
		return nil
	},
}
