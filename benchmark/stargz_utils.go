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

package benchmark

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/awslabs/soci-snapshotter/benchmark/framework"
	"github.com/containerd/containerd"
	ctdsnapshotters "github.com/containerd/containerd/pkg/snapshotters"
)

type StargzProcess struct {
	command *exec.Cmd
	address string
	root    string
	stdout  *os.File
	stderr  *os.File
}

type StargzContainerdProcess struct {
	*framework.ContainerdProcess
}

func StartStargz(
	stargzBinary string,
	stargzAddress string,
	stargzConfig string,
	stargzRoot string,
	outputDir string) (*StargzProcess, error) {
	stargzCmd := exec.Command(stargzBinary,
		"-address", stargzAddress,
		"-config", stargzConfig,
		"-log-level", "debug",
		"-root", stargzRoot)
	err := os.MkdirAll(outputDir, 0777)
	if err != nil {
		return nil, err
	}
	stdoutFile, err := os.Create(outputDir + "/stargz-snapshotter-stdout")
	if err != nil {
		return nil, err
	}
	stargzCmd.Stdout = stdoutFile
	stderrFile, err := os.Create(outputDir + "/stargz-snapshotter-stderr")
	if err != nil {
		return nil, err
	}
	stargzCmd.Stderr = stderrFile
	err = stargzCmd.Start()
	if err != nil {
		fmt.Printf("Stargz process failed to start %v\n", err)
		return nil, err
	}

	// The stargz-snapshotter-grpc is not ready to be used until the
	// unix socket file is created
	sleepCount := 0
	loopExit := false
	for !loopExit {
		time.Sleep(1 * time.Second)
		sleepCount++
		if _, err := os.Stat(stargzAddress); err == nil {
			loopExit = true
		}
		if sleepCount > 15 {
			return nil, errors.New("could not create .sock in time")
		}
	}
	return &StargzProcess{
		command: stargzCmd,
		address: stargzAddress,
		root:    stargzRoot,
		stdout:  stdoutFile,
		stderr:  stderrFile}, nil
}

func (proc *StargzProcess) StopProcess() {
	if proc.stdout != nil {
		proc.stdout.Close()
	}
	if proc.stderr != nil {
		proc.stderr.Close()
	}
	if proc.command != nil {
		proc.command.Process.Kill()
	}
	err := os.RemoveAll(proc.address)
	if err != nil {
		fmt.Printf("Error removing stargz process address: %v\n", err)
	}

	snapshotDir := proc.root + "/snapshotter/snapshots/"
	snapshots, err := os.ReadDir(snapshotDir)
	if err != nil {
		fmt.Printf("Could not read dir: %s\n", snapshotDir)
	}

	for _, s := range snapshots {
		mountpoint := snapshotDir + s.Name() + "/fs"
		_ = syscall.Unmount(mountpoint, syscall.MNT_FORCE)
	}
	err = os.RemoveAll(proc.root)
	if err != nil {
		fmt.Printf("Error removing stargz process root: %v\n", err)
	}
}

func (proc *StargzContainerdProcess) StargzRpullImageFromRegistry(
	ctx context.Context,
	imageRef string) (containerd.Image, error) {
	image, err := proc.Client.Pull(ctx, imageRef, []containerd.RemoteOpt{
		containerd.WithResolver(framework.GetResolver(ctx, imageRef)),
		//nolint:staticcheck
		containerd.WithSchema1Conversion, //lint:ignore SA1019
		containerd.WithPullUnpack,
		containerd.WithPullSnapshotter("stargz"),
		containerd.WithImageHandlerWrapper(ctdsnapshotters.AppendInfoHandlerWrapper(imageRef)),
	}...)
	if err != nil {
		fmt.Printf("Stargz rpull failed: %v\n", err)
		return nil, err
	}
	return image, nil
}
