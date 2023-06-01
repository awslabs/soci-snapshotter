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
	"context"

	"github.com/awslabs/soci-snapshotter/fs/config"
	"github.com/awslabs/soci-snapshotter/soci"
	"github.com/urfave/cli"
)

var removeOrphansCommand = cli.Command{
	Name:        "remove-orphans",
	Usage:       "remove orphan zTOCs",
	Description: "remove all zTOCs that are not referenced by any index manifest (cleanup after `soci index rm --keep-orphan-ztocs`)",
	Action: func(cliContext *cli.Context) error {
		db, err := soci.NewDB(soci.ArtifactsDbPath())
		if err != nil {
			return err
		}
		ctx, cancelTimeout := context.WithTimeout(context.Background(), cliContext.GlobalDuration("timeout"))
		defer cancelTimeout()
		db.RemoveOrphanZtocs(ctx, config.SociContentStorePath)
		return nil
	},
}
