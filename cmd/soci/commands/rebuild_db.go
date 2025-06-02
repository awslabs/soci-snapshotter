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
	"path/filepath"

	"github.com/awslabs/soci-snapshotter/cmd/soci/commands/internal"
	"github.com/awslabs/soci-snapshotter/soci"
	"github.com/awslabs/soci-snapshotter/soci/store"
	"github.com/urfave/cli/v2"
)

var RebuildDBCommand = &cli.Command{
	Name:  "rebuild-db",
	Usage: "rebuilds the artifacts database",
	UsageText: `
	soci [global options] rebuild-db

	Use after pulling an image to discover SOCI indices/ztocs or after "index rm"
	when using the containerd content store to clear the database of removed zTOCs.
	`,
	Action: func(cliContext *cli.Context) error {
		client, ctx, cancel, err := internal.NewClient(cliContext)
		if err != nil {
			return err
		}
		defer cancel()
		containerdContentStore := client.ContentStore()
		artifactsDb, err := soci.NewDB(soci.ArtifactsDbPath(cliContext.String("root")))
		if err != nil {
			return err
		}
		blobStore, err := store.NewContentStore(internal.ContentStoreOptions(cliContext)...)
		if err != nil {
			return err
		}

		contentStorePath, err := store.GetContentStorePath(store.ContentStoreType(cliContext.String("content-store")))
		if err != nil {
			return err
		}

		blobStorePath := filepath.Join(contentStorePath, "blobs")
		return artifactsDb.SyncWithLocalStore(ctx, blobStore, blobStorePath, containerdContentStore)
	},
}
