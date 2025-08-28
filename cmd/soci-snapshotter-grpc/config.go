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

package main

import (
	"context"
	"os"

	"github.com/awslabs/soci-snapshotter/config"
	"github.com/containerd/log"
	"github.com/pelletier/go-toml/v2"
	"github.com/urfave/cli/v3"
)

var ConfigCommand = &cli.Command{
	Name:  "config",
	Usage: "Manage configuration",
	Commands: []*cli.Command{
		{
			Name:  "dump",
			Usage: "Dump configuration",
			Action: func(ctx context.Context, cmd *cli.Command) error {
				cfg, err := config.NewConfigFromToml(cmd.String("config"))
				if err != nil {
					log.G(ctx).WithError(err).Fatal(err)
				}

				toml.NewEncoder(os.Stdout).SetIndentTables(true).Encode(cfg)
				return nil
			},
		},
	},
}
