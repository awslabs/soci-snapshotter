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

	"github.com/awslabs/soci-snapshotter/cmd/soci-cli/commands"
	"github.com/awslabs/soci-snapshotter/cmd/soci-cli/commands/image"
	"github.com/awslabs/soci-snapshotter/cmd/soci-cli/commands/index"
	"github.com/awslabs/soci-snapshotter/cmd/soci-cli/commands/ztoc"
	"github.com/containerd/containerd/cmd/ctr/commands/run"
	"github.com/containerd/containerd/defaults"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/pkg/seed"
	"github.com/urfave/cli"
)

func init() {
	seed.WithTimeAndRand()
}

func main() {
	app := cli.NewApp()
	app.Name = "soci-cli"
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
	}

	app.Commands = []cli.Command{
		image.Command,
		index.Command,
		ztoc.Command,
		commands.CreateCommand,
		commands.PushCommand,
		run.Command,
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "soci-cli: %v\n", err)
		os.Exit(1)
	}
}
