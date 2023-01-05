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
	"io/fs"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/awslabs/soci-snapshotter/benchmark/framework"
	"github.com/awslabs/soci-snapshotter/fs/source"
	"github.com/containerd/containerd"
)

var (
	outputFilePerm fs.FileMode = 0777
)

type SociContainerdProcess struct {
	*framework.ContainerdProcess
}

type SociProcess struct {
	command *exec.Cmd
	address string
	root    string
	stdout  *os.File
	stderr  *os.File
}

func StartSoci(
	sociBinary string,
	sociAddress string,
	sociRoot string,
	containerdAddress string,
	configFile string,
	outputDir string) (*SociProcess, error) {
	sociCmd := exec.Command(sociBinary,
		"-address", sociAddress,
		"-config", configFile,
		"-log-level", "debug",
		"-root", sociRoot)
	err := os.MkdirAll(outputDir, outputFilePerm)
	if err != nil {
		return nil, err
	}
	stdoutFile, err := os.Create(outputDir + "/soci-snapshotter-stdout")
	if err != nil {
		return nil, err
	}
	sociCmd.Stdout = stdoutFile
	stderrFile, err := os.Create(outputDir + "/soci-snapshotter-stderr")
	if err != nil {
		return nil, err
	}
	sociCmd.Stderr = stderrFile
	err = sociCmd.Start()
	if err != nil {
		fmt.Printf("Soci Failed to Start %v\n", err)
		return nil, err
	}

	// The soci-snapshotter-grpc is not ready to be used until the
	// unix socket file is created
	sleepCount := 0
	loopExit := false
	for !loopExit {
		time.Sleep(1 * time.Second)
		sleepCount++
		if _, err := os.Stat(sociAddress); err == nil {
			loopExit = true
		}
		if sleepCount > 15 {
			return nil, errors.New("Could not create .sock in time")
		}
	}

	return &SociProcess{
		command: sociCmd,
		address: sociAddress,
		root:    sociRoot,
		stdout:  stdoutFile,
		stderr:  stderrFile}, nil
}

func (proc *SociProcess) StopProcess() {
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
		fmt.Printf("Error removing Address: %v\n", err)
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
		fmt.Printf("Error removing root: %v\n", err)
	}
}

func (proc *SociContainerdProcess) SociRPullImageFromECR(
	ctx context.Context,
	imageRef string,
	sociIndexDigest string,
	awsSecretFile string) (containerd.Image, error) {
	resolver, err := framework.GetECRResolver(ctx, awsSecretFile)
	if err != nil {
		return nil, err
	}
	image, err := proc.Client.Pull(ctx, imageRef, []containerd.RemoteOpt{
		containerd.WithResolver(resolver),
		containerd.WithSchema1Conversion,
		containerd.WithPullUnpack,
		containerd.WithPullSnapshotter("soci"),
		containerd.WithImageHandlerWrapper(source.AppendDefaultLabelsHandlerWrapper(
			imageRef,
			sociIndexDigest)),
	}...)
	if err != nil {
		fmt.Printf("Soci Pull Failed %v\n", err)
		return nil, err
	}
	return image, nil
}

func (proc *SociContainerdProcess) CreateSociContainer(
	ctx context.Context,
	image containerd.Image) (containerd.Container, func(), error) {
	return proc.CreateContainer(ctx, image, containerd.WithSnapshotter("soci"))
}
