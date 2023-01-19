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
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/awslabs/soci-snapshotter/soci"
	"github.com/urfave/cli"
)

var listCommand = cli.Command{
	Name:        "list",
	Description: "list ztocs",
	Aliases:     []string{"ls"},
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "digest",
			Usage: "filter ztocs by digest",
		},
	},
	Action: func(cliContext *cli.Context) error {
		db, err := soci.NewDB()
		if err != nil {
			return err
		}
		digest := cliContext.String("digest")

		var artifacts []*soci.ArtifactEntry
		db.Walk(func(ae *soci.ArtifactEntry) error {
			if ae.Type == soci.ArtifactEntryTypeLayer &&
				(digest == "" || ae.Digest == digest) {
				artifacts = append(artifacts, ae)
			}
			return nil
		})

		writer := tabwriter.NewWriter(os.Stdout, 8, 8, 4, ' ', 0)
		writer.Write([]byte("DIGEST\tSIZE\tLAYER DIGEST\t\n"))
		for _, artifact := range artifacts {
			writer.Write([]byte(fmt.Sprintf("%s\t%d\t%s\t\n", artifact.Digest, artifact.Size, artifact.OriginalDigest)))
		}
		writer.Flush()
		return nil
	},
}
