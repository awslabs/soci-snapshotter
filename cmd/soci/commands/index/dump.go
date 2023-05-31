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
	"strconv"

	"github.com/awslabs/soci-snapshotter/soci"
	"github.com/urfave/cli"
)

var dumpCommand = cli.Command{
	Name:        "dump",
	Aliases:     []string{"dump"},
	Usage:       "dump the artifact db",
	Description: "prints the contents of the artifact db to stdout",
	Flags:       []cli.Flag{},
	Action: func(cliContext *cli.Context) error {
		db, err := soci.NewDB(soci.ArtifactsDbPath())
		if err != nil {
			return err
		}
		db.Walk(func(ae *soci.ArtifactEntry) error {
			fmt.Println(" ArtifactEntry:")
			fmt.Println("          Size: " + strconv.FormatInt(ae.Size, 10))
			fmt.Println("        Digest: " + ae.Digest)
			fmt.Println("OriginalDigest: " + ae.OriginalDigest)
			fmt.Println("   ImageDigest: " + ae.ImageDigest)
			fmt.Println("      Platform: " + ae.Platform)
			fmt.Println("      Location: " + ae.Location)
			fmt.Println("          Type: " + ae.Type)
			fmt.Println("     MediaType: " + ae.MediaType)
			fmt.Println("     CreatedAt: " + ae.CreatedAt.String())
			fmt.Println()
			return nil
		})
		return nil
	},
}
