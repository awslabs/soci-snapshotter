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

	"github.com/awslabs/soci-snapshotter/fs/config"
	"github.com/awslabs/soci-snapshotter/soci"
	"github.com/containerd/containerd/cmd/ctr/commands"
	"github.com/urfave/cli"
	"oras.land/oras-go/v2/content/oci"
)

var RebuildDBCommand = cli.Command{
	Name:  "rebuild-db",
	Usage: `rebuild the artifacts database. You should use this command after "rpull" so that indices/ztocs can be discovered by commands like "soci index list".`,
	Action: func(cliContext *cli.Context) error {
		client, ctx, cancel, err := commands.NewClient(cliContext)
		if err != nil {
			return err
		}
		defer cancel()
		containerdContentStore := client.ContentStore()
		artifactsDb, err := soci.NewDB(soci.ArtifactsDbPath())
		if err != nil {
			return err
		}
		blobStore, err := oci.New(config.DefaultSociContentStorePath)
		if err != nil {
			return err
		}
		blobStorePath := filepath.Join(config.DefaultSociContentStorePath, "blobs")
		return artifactsDb.SyncWithLocalStore(ctx, blobStore, blobStorePath, containerdContentStore)
	},
}
