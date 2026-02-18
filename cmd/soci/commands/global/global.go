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

package global

import (
	"github.com/awslabs/soci-snapshotter/config"
	"github.com/containerd/containerd/defaults"
	"github.com/containerd/containerd/namespaces"
	"github.com/urfave/cli/v3"
)

// Global flags for SOCI CLI

const (
	AddressFlag      = "address"
	NamespaceFlag    = "namespace"
	TimeoutFlag      = "timeout"
	DebugFlag        = "debug"
	ContentStoreFlag = "content-store"
	RootFlag         = "root"
)

var Flags = []cli.Flag{
	&cli.StringFlag{
		Name:    AddressFlag,
		Aliases: []string{"a"},
		Usage:   "address for containerd's GRPC server",
		Value:   defaults.DefaultAddress,
		Sources: cli.EnvVars("CONTAINERD_ADDRESS"),
	},
	&cli.StringFlag{
		Name:    NamespaceFlag,
		Aliases: []string{"n"},
		Usage:   "namespace to use with commands",
		Value:   namespaces.Default,
		Sources: cli.EnvVars(namespaces.NamespaceEnvVar),
	},
	&cli.DurationFlag{
		Name:  TimeoutFlag,
		Usage: "timeout for commands",
	},
	&cli.BoolFlag{
		Name:  DebugFlag,
		Usage: "enable debug output",
	},
	&cli.StringFlag{
		Name:  ContentStoreFlag,
		Usage: "use a specific content store (soci or containerd)",
		Value: string(config.DefaultContentStoreType),
	},
	&cli.StringFlag{
		Name:  RootFlag,
		Usage: "path to the root directory for this snapshotter",
		Value: config.DefaultSociSnapshotterRootPath,
	},
}
