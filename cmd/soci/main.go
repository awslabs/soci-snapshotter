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

/*
   Copyright The containerd Authors.

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
	"fmt"
	"os"

	"github.com/awslabs/soci-snapshotter/cmd/soci/commands"
	"github.com/awslabs/soci-snapshotter/cmd/soci/commands/index"
	"github.com/awslabs/soci-snapshotter/cmd/soci/commands/ztoc"
	"github.com/awslabs/soci-snapshotter/config"
	"github.com/awslabs/soci-snapshotter/version"
	"github.com/containerd/containerd/defaults"
	"github.com/containerd/containerd/namespaces"

	//nolint:staticcheck
	"github.com/containerd/containerd/pkg/seed"
	"github.com/urfave/cli"
)

func init() {
	//nolint:staticcheck
	seed.WithTimeAndRand() //lint:ignore SA1019, WithTimeAndRand is deprecated and we should remove it.
}

func main() {
	app := cli.NewApp()
	app.Name = "soci"
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:   "address, a",
			Usage:  "address for containerd's GRPC server",
			Value:  defaults.DefaultAddress,
			EnvVar: "CONTAINERD_ADDRESS",
		},
		cli.StringFlag{
			Name:   "namespace, n",
			Usage:  "namespace to use with commands",
			Value:  namespaces.Default,
			EnvVar: namespaces.NamespaceEnvVar,
		},
		cli.DurationFlag{
			Name:  "timeout",
			Usage: "timeout for commands",
		},
		cli.BoolFlag{
			Name:  "debug",
			Usage: "enable debug output",
		},
		cli.StringFlag{
			Name:  "content-store",
			Usage: "use a specific content store (soci or containerd)",
			Value: string(config.DefaultContentStoreType),
		},
	}

	app.Version = fmt.Sprintf("%s %s", version.Version, version.Revision)

	app.Commands = []cli.Command{
		index.Command,
		ztoc.Command,
		commands.CreateCommand,
		commands.ConvertCommand,
		commands.PushCommand,
		commands.RebuildDBCommand,
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "soci: %v\n", err)
		os.Exit(1)
	}
}
