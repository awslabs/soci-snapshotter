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
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/awslabs/soci-snapshotter/cmd/soci/commands"
	"github.com/awslabs/soci-snapshotter/cmd/soci/commands/image"
	"github.com/awslabs/soci-snapshotter/cmd/soci/commands/index"
	"github.com/awslabs/soci-snapshotter/cmd/soci/commands/ztoc"
	"github.com/awslabs/soci-snapshotter/config"
	"github.com/awslabs/soci-snapshotter/version"
	"github.com/containerd/containerd/defaults"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/nerdctl/pkg/rootlessutil"

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
			Value: config.DefaultContentStoreType,
		},
	}

	app.Version = fmt.Sprintf("%s %s", version.Version, version.Revision)

	app.Commands = []cli.Command{
		image.Command,
		index.Command,
		ztoc.Command,
		commands.CreateCommand,
		commands.PushCommand,
		commands.RebuildDBCommand,
	}

	app.Before = func(ctx *cli.Context) error {
		if rootlessutil.IsRootlessParent() {
			return parentMain()
		}

		return nil
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "soci: %v\n", err)
		os.Exit(1)
	}
}

func parentMain() error {
	if !rootlessutil.IsRootlessParent() {
		return errors.New("should not be called when !IsRootlessParent()")
	}
	stateDir, err := rootlessutil.RootlessKitStateDir()
	if err != nil {
		return fmt.Errorf("rootless containerd not running? (hint: use `containerd-rootless-setuptool.sh install` to start rootless containerd): %w", err)
	}
	childPid, err := rootlessutil.RootlessKitChildPid(stateDir)
	if err != nil {
		return err
	}

	wd, err := os.Getwd()
	if err != nil {
		return err
	}

	// FIXME: remove dependency on `nsenter` binary
	arg0, err := exec.LookPath("nsenter")
	if err != nil {
		return err
	}
	// args are compatible with both util-linux nsenter and busybox nsenter
	args := []string{
		"-r/",     // root dir (busybox nsenter wants this to be explicitly specified),
		"-w" + wd, // work dir
		"--preserve-credentials",
		"-m", "-n", "-U",
		"-t", strconv.Itoa(childPid),
		"-F", // no fork
	}

	// replace the args[0] with full path
	var originalCmd = os.Args[0]
	originalCmdFullPath, err := filepath.Abs(originalCmd)
	if err != nil {
		fmt.Printf("failed to get full path: %s\n", err)
	}
	os.Args[0] = originalCmdFullPath

	args = append(args, os.Args...)

	// Env vars corresponds to RootlessKit spec:
	// https://github.com/rootless-containers/rootlesskit/tree/v0.13.1#environment-variables
	os.Setenv("ROOTLESSKIT_STATE_DIR", stateDir)
	os.Setenv("ROOTLESSKIT_PARENT_EUID", strconv.Itoa(os.Geteuid()))
	os.Setenv("ROOTLESSKIT_PARENT_EGID", strconv.Itoa(os.Getegid()))
	return syscall.Exec(arg0, args, os.Environ())
}
