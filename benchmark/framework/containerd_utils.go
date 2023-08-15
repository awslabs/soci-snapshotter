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

package framework

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/oci"
	"github.com/sirupsen/logrus"
)

var (
	testNamespace               = "BENCHMARK_TESTING"
	testContainerID             = "TEST_RUN_CONTAINER"
	testEnvironment             = "TEST_RUNTIME"
	outputFilePerm  fs.FileMode = 0777
)

type ContainerdProcess struct {
	command *exec.Cmd
	address string
	root    string
	state   string
	stdout  *os.File
	stderr  *os.File
	Client  *containerd.Client
}

func StartContainerd(
	containerdAddress string,
	containerdRoot string,
	containerdState string,
	containerdConfig string,
	containerdOutput string) (*ContainerdProcess, error) {
	containerdCmd := exec.Command("containerd",
		"-a", containerdAddress,
		"--root", containerdRoot,
		"--state", containerdState,
		"-c", containerdConfig)
	err := os.MkdirAll(containerdOutput, outputFilePerm)
	if err != nil {
		return nil, err
	}
	stdoutFile, err := os.Create(containerdOutput + "/containerd-stdout")
	if err != nil {
		return nil, err
	}
	containerdCmd.Stdout = stdoutFile
	stderrFile, err := os.Create(containerdOutput + "/containerd-stderr")
	if err != nil {
		return nil, err
	}
	containerdCmd.Stderr = stderrFile
	err = containerdCmd.Start()
	if err != nil {
		return nil, err
	}
	client, err := newClient(containerdAddress)
	if err != nil {
		return nil, err
	}

	return &ContainerdProcess{
		command: containerdCmd,
		address: containerdAddress,
		root:    containerdRoot,
		stdout:  stdoutFile,
		stderr:  stderrFile,
		state:   containerdState,
		Client:  client}, nil
}

func (proc *ContainerdProcess) StopProcess() {
	if proc.Client != nil {
		proc.Client.Close()
	}
	if proc.stdout != nil {
		proc.stdout.Close()
	}
	if proc.stderr != nil {
		proc.stderr.Close()
	}
	if proc.command != nil {
		proc.command.Process.Kill()
	}
	os.RemoveAll(proc.root)
	os.RemoveAll(proc.state)
	os.RemoveAll(proc.address)
}

func (proc *ContainerdProcess) PullImage(
	ctx context.Context,
	imageRef string,
	platform string) (containerd.Image, error) {
	image, pullErr := proc.Client.Pull(ctx, imageRef, GetRemoteOpts(ctx, platform)...)
	if pullErr != nil {
		return nil, pullErr
	}
	return image, nil
}

func (proc *ContainerdProcess) DeleteImage(ctx context.Context, imageRef string) error {
	imageService := proc.Client.ImageService()
	err := imageService.Delete(ctx, imageRef, images.SynchronousDelete())
	if err != nil {
		return err
	}
	return nil
}

func (proc *ContainerdProcess) CreateContainer(
	ctx context.Context,
	image containerd.Image,
	opts ...containerd.NewContainerOpts) (containerd.Container, func(), error) {
	id := fmt.Sprintf("%s-%d", testContainerID, time.Now().UnixNano())
	opts = append(opts, containerd.WithNewSnapshot(id, image))
	opts = append(opts, containerd.WithNewSpec(oci.WithImageConfig(image)))
	container, err := proc.Client.NewContainer(
		ctx,
		id,
		opts...)
	if err != nil {
		return nil, nil, err
	}
	cleanupFunc := func() {
		err = container.Delete(ctx, containerd.WithSnapshotCleanup)
		if err != nil {
			fmt.Printf("Error deleting container: %v\n", err)
		}
	}
	return container, cleanupFunc, nil
}

type TaskDetails struct {
	task         containerd.Task
	stdoutReader io.Reader
	stderrReader io.Reader
}

func (proc *ContainerdProcess) CreateTask(
	ctx context.Context,
	container containerd.Container) (*TaskDetails, func(), error) {
	stdoutPipeReader, stdoutPipeWriter := io.Pipe()
	stderrPipeReader, stderrPipeWriter := io.Pipe()
	cioCreator := cio.NewCreator(cio.WithStreams(os.Stdin, stdoutPipeWriter, stderrPipeWriter))
	task, err := container.NewTask(ctx, cioCreator)
	if err != nil {
		return nil, nil, err
	}
	cleanupFunc := func() {
		stdoutPipeReader.Close()
		stdoutPipeWriter.Close()
		stderrPipeReader.Close()
		stderrPipeWriter.Close()
		processStatus, _ := task.Status(ctx)
		if processStatus.Status != "stopped" {
			fmt.Printf("Tried to kill task")
			err = task.Kill(ctx, syscall.SIGKILL)
			if err != nil {
				fmt.Printf("Error killing task: %v\n", err)
			}
		}
		_, err = task.Delete(ctx)
		if err != nil {
			fmt.Printf("Error deleting task: %v\n", err)
		}
	}

	return &TaskDetails{
			task,
			stdoutPipeReader,
			stderrPipeReader},
		cleanupFunc,
		err
}

func (proc *ContainerdProcess) RunContainerTaskForReadyLine(
	ctx context.Context,
	taskDetails *TaskDetails,
	readyLine string) (func(), error) {
	stdoutScanner := bufio.NewScanner(taskDetails.stdoutReader)
	stderrScanner := bufio.NewScanner(taskDetails.stderrReader)

	time.Sleep(10 * time.Second)

	exitStatusC, err := taskDetails.task.Wait(ctx)
	if err != nil {
		return nil, err
	}
	resultChannel := make(chan string, 1)
	timeoutCtx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()
	go func() {
		select {
		case <-exitStatusC:
			resultChannel <- "PROC_EXIT"
		case <-timeoutCtx.Done():
			return
		}
	}()

	go func() {
		for stderrScanner.Scan() {
			nextLine := stderrScanner.Text()
			if strings.Contains(nextLine, readyLine) {
				resultChannel <- "READYLINE_STDERR"
				return
			}
			select {
			case <-timeoutCtx.Done():
				return
			default:
			}
		}
	}()

	go func() {
		for stdoutScanner.Scan() {
			nextLine := stdoutScanner.Text()
			if strings.Contains(nextLine, readyLine) {
				resultChannel <- "READYLINE_STDOUT"
				return
			}
			select {
			case <-timeoutCtx.Done():
				return
			default:
			}
		}
	}()

	if err := taskDetails.task.Start(ctx); err != nil {
		return nil, err
	}

	select {
	case <-resultChannel:
		break
	case <-timeoutCtx.Done():
		break
	}

	cleanupFunc := func() {
		processStatus, _ := taskDetails.task.Status(ctx)
		if processStatus.Status == "running" {
			err = taskDetails.task.Kill(ctx, syscall.SIGKILL)
			if err != nil {
				fmt.Printf("Error killing task: %v\n", err)
			}
			exitChannel, _ := taskDetails.task.Wait(ctx)
			<-exitChannel
		}
	}
	return cleanupFunc, nil
}

func GetRemoteOpts(ctx context.Context, platform string) []containerd.RemoteOpt {
	var opts []containerd.RemoteOpt
	opts = append(opts, containerd.WithPlatform(platform))

	return opts
}

func GetTestContext(logFile io.Writer) (context.Context, context.CancelFunc) {
	logrus.SetLevel(logrus.InfoLevel)
	logrus.SetFormatter(&logrus.JSONFormatter{
		TimestampFormat: log.RFC3339NanoFixed,
	})
	if logFile != nil {
		logrus.SetOutput(logFile)
	} else {
		logrus.SetOutput(os.Stderr)
	}

	ctx := log.WithLogger(context.Background(), log.L)
	ctx, cancel := context.WithCancel(ctx)
	ctx = namespaces.WithNamespace(ctx, testNamespace)
	return ctx, cancel
}

func newClient(address string) (*containerd.Client, error) {
	opts := []containerd.ClientOpt{}
	if rt := os.Getenv(testEnvironment); rt != "" {
		opts = append(opts, containerd.WithDefaultRuntime(rt))
	}

	return containerd.New(address, opts...)
}
