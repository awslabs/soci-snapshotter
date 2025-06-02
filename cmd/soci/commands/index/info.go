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
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"github.com/awslabs/soci-snapshotter/cmd/soci/commands/internal"
	"github.com/awslabs/soci-snapshotter/soci"
	"github.com/awslabs/soci-snapshotter/soci/store"

	"github.com/opencontainers/go-digest"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/urfave/cli/v2"
)

var infoCommand = &cli.Command{
	Name:        "info",
	Usage:       "display an index",
	Description: "get detailed info about an index",
	ArgsUsage:   "<digest>",
	Action: func(cliContext *cli.Context) error {
		ctx, cancel := internal.AppContext(cliContext)
		defer cancel()

		digest, err := digest.Parse(cliContext.Args().First())
		if err != nil {
			return err
		}
		db, err := soci.NewDB(soci.ArtifactsDbPath(cliContext.String("root")))
		if err != nil {
			return err
		}
		artifactType, err := db.GetArtifactType(digest.String())
		if err != nil {
			return err
		}
		if artifactType == soci.ArtifactEntryTypeLayer {
			return fmt.Errorf("the provided digest is of ztoc not SOCI index. Use \"soci ztoc info\" command to get detailed info of ztoc")
		}
		store, err := store.NewContentStore(internal.ContentStoreOptions(cliContext)...)
		if err != nil {
			return err
		}
		reader, err := store.Fetch(ctx, v1.Descriptor{Digest: digest})
		if err != nil {
			return err
		}
		defer reader.Close()

		b, err := io.ReadAll(reader)
		if err != nil {
			return err
		}

		err = prettyPrintJSON(b)
		return err
	},
}

func prettyPrintJSON(b []byte) error {
	var prettyJSON bytes.Buffer
	if err := json.Indent(&prettyJSON, b, "", "  "); err != nil {
		return err
	}
	_, err := fmt.Println(prettyJSON.String())
	return err
}
