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
	"testing"

	"github.com/awslabs/soci-snapshotter/benchmark/framework"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/log"
	"github.com/google/uuid"
)

var (
	outputDir              = "./output"
	containerdAddress      = "/tmp/containerd-grpc/containerd.sock"
	containerdRoot         = "/tmp/lib/containerd"
	containerdState        = "/tmp/containerd"
	containerdSociConfig   = "../containerd_soci_config.toml"
	containerdStargzConfig = "../containerd_stargz_config.toml"
	platform               = "linux/amd64"
	sociBinary             = "../../out/soci-snapshotter-grpc"
	sociAddress            = "/tmp/soci-snapshotter-grpc/soci-snapshotter-grpc.sock"
	sociRoot               = "/tmp/lib/soci-snapshotter-grpc"
	sociConfig             = "../soci_config.toml"
	awsSecretFile          = "../aws_secret"
	stargzAddress          = "/tmp/containerd-stargz-grpc/containerd-stargz-grpc.sock"
	stargzConfig           = "../stargz_config.toml"
	stargzRoot             = "/tmp/lib/containerd-stargz-grpc"
)

func PullImageFromECR(ctx context.Context, b *testing.B, imageRef string) {
	containerdProcess, err := getContainerdProcess(ctx, containerdSociConfig)
	if err != nil {
		b.Fatalf("Error Starting Containerd: %v\n", err)
	}
	defer containerdProcess.StopProcess()
	b.ResetTimer()
	_, err = containerdProcess.PullImageFromECR(ctx, imageRef, platform, awsSecretFile)
	if err != nil {
		b.Fatalf("Error Pulling Image: %v\n", err)
	}
	b.StopTimer()
	err = containerdProcess.DeleteImage(ctx, imageRef)
	if err != nil {
		b.Fatalf("Error Deleting Image: %v\n", err)
	}
}

func SociRPullPullImage(
	ctx context.Context,
	b *testing.B,
	imageRef string,
	indexDigest string) {
	containerdProcess, err := getContainerdProcess(ctx, containerdSociConfig)
	if err != nil {
		b.Fatalf("Failed to create containerd proc: %v\n", err)
	}
	defer containerdProcess.StopProcess()
	sociProcess, err := getSociProcess()
	if err != nil {
		b.Fatalf("Failed to create soci proc: %v\n", err)
	}
	defer sociProcess.StopProcess()
	sociContainerdProc := SociContainerdProcess{containerdProcess}
	b.ResetTimer()
	_, err = sociContainerdProc.SociRPullImageFromECR(ctx, imageRef, indexDigest, awsSecretFile)
	if err != nil {
		b.Fatalf("%s", err)
	}
	b.StopTimer()
}

func SociFullRun(
	ctx context.Context,
	b *testing.B,
	imageRef string,
	indexDigest string,
	readyLine string,
	testName string) {
	testUUID := uuid.New().String()
	ctx = log.WithLogger(ctx, log.G(ctx).WithField("test_name", testName))
	ctx = log.WithLogger(ctx, log.G(ctx).WithField("uuid", testUUID))
	containerdProcess, err := getContainerdProcess(ctx, containerdSociConfig)
	if err != nil {
		b.Fatalf("Failed to create containerd proc: %v\n", err)
	}
	defer containerdProcess.StopProcess()
	sociProcess, err := getSociProcess()
	if err != nil {
		b.Fatalf("Failed to create soci proc: %v\n", err)
	}
	defer sociProcess.StopProcess()
	sociContainerdProc := SociContainerdProcess{containerdProcess}
	b.ResetTimer()
	log.G(ctx).WithField("benchmark", "Test").WithField("event", "Start").Infof("Start Test")
	log.G(ctx).WithField("benchmark", "Pull").WithField("event", "Start").Infof("Start Pull Image")
	image, err := sociContainerdProc.SociRPullImageFromECR(ctx, imageRef, indexDigest, awsSecretFile)
	log.G(ctx).WithField("benchmark", "Pull").WithField("event", "Stop").Infof("Stop Pull Image")
	if err != nil {
		b.Fatalf("%s", err)
	}
	log.G(ctx).WithField("benchmark", "CreateContainer").WithField("event", "Start").Infof("Start Create Container")
	container, cleanupContainer, err := sociContainerdProc.CreateSociContainer(ctx, image)
	log.G(ctx).WithField("benchmark", "CreateContainer").WithField("event", "Stop").Infof("Stop Create Container")
	if err != nil {
		b.Fatalf("%s", err)
	}
	defer cleanupContainer()
	log.G(ctx).WithField("benchmark", "CreateTask").WithField("event", "Start").Infof("Start Create Task")
	taskDetails, cleanupTask, err := sociContainerdProc.CreateTask(ctx, container)
	log.G(ctx).WithField("benchmark", "CreateTask").WithField("event", "Stop").Infof("Stop Create Task")
	if err != nil {
		b.Fatalf("%s", err)
	}
	defer cleanupTask()
	log.G(ctx).WithField("benchmark", "RunTask").WithField("event", "Start").Infof("Start Run Task")
	cleanupRun, err := sociContainerdProc.RunContainerTaskForReadyLine(ctx, taskDetails, readyLine)
	log.G(ctx).WithField("benchmark", "RunTask").WithField("event", "Stop").Infof("Stop Run Task")
	if err != nil {
		b.Fatalf("%s", err)
	}
	defer cleanupRun()
	containerSecondRun, cleanupContainerSecondRun, err := sociContainerdProc.CreateSociContainer(ctx, image)
	if err != nil {
		b.Fatalf("%s", err)
	}
	defer cleanupContainerSecondRun()
	taskDetailsSecondRun, cleanupTaskSecondRun, err := sociContainerdProc.CreateTask(ctx, containerSecondRun)
	if err != nil {
		b.Fatalf("%s", err)
	}
	defer cleanupTaskSecondRun()
	log.G(ctx).WithField("benchmark", "RunTaskTwice").WithField("event", "Start").Infof("Start Run Task Twice")
	cleanupRunSecond, err := sociContainerdProc.RunContainerTaskForReadyLine(ctx, taskDetailsSecondRun, readyLine)
	log.G(ctx).WithField("benchmark", "RunTaskTwice").WithField("event", "Stop").Infof("Stop Run Task Twice")
	if err != nil {
		b.Fatalf("%s", err)
	}
	defer cleanupRunSecond()
	log.G(ctx).WithField("benchmark", "Test").WithField("event", "Stop").Infof("Stop Test")
	b.StopTimer()
}

func OverlayFSFullRun(
	ctx context.Context,
	b *testing.B,
	imageRef string,
	readyLine string,
	testName string) {
	testUUID := uuid.New().String()
	ctx = log.WithLogger(ctx, log.G(ctx).WithField("test_name", testName))
	ctx = log.WithLogger(ctx, log.G(ctx).WithField("uuid", testUUID))
	containerdProcess, err := getContainerdProcess(ctx, containerdSociConfig)
	if err != nil {
		b.Fatalf("Failed to create containerd proc: %v\n", err)
	}
	defer containerdProcess.StopProcess()
	b.ResetTimer()
	log.G(ctx).WithField("benchmark", "Test").WithField("event", "Start").Infof("Start Test")
	log.G(ctx).WithField("benchmark", "Pull").WithField("event", "Start").Infof("Start Pull Image")
	image, err := containerdProcess.PullImageFromECR(ctx, imageRef, platform, awsSecretFile)
	log.G(ctx).WithField("benchmark", "Pull").WithField("event", "Stop").Infof("Stop Pull Image")
	if err != nil {
		b.Fatalf("%s", err)
	}
	log.G(ctx).WithField("benchmark", "Unpack").WithField("event", "Start").Infof("Start Unpack Image")
	err = image.Unpack(ctx, "overlayfs")
	log.G(ctx).WithField("benchmark", "Unpack").WithField("event", "Stop").Infof("Stop Unpack Image")
	if err != nil {
		b.Fatalf("%s", err)
	}
	log.G(ctx).WithField("benchmark", "CreateContainer").WithField("event", "Start").Infof("Start Create Container")
	container, cleanupContainer, err := containerdProcess.CreateContainer(ctx, image)
	log.G(ctx).WithField("benchmark", "CreateContainer").WithField("event", "Stop").Infof("Stop Create Container")
	if err != nil {
		b.Fatalf("%s", err)
	}
	defer cleanupContainer()
	log.G(ctx).WithField("benchmark", "CreateTask").WithField("event", "Start").Infof("Start Create Task")
	taskDetails, cleanupTask, err := containerdProcess.CreateTask(ctx, container)
	log.G(ctx).WithField("benchmark", "CreateTask").WithField("event", "Stop").Infof("Stop Create Task")
	if err != nil {
		b.Fatalf("%s", err)
	}
	defer cleanupTask()
	log.G(ctx).WithField("benchmark", "RunTask").WithField("event", "Start").Infof("Start Run Task")
	cleanupRun, err := containerdProcess.RunContainerTaskForReadyLine(ctx, taskDetails, readyLine)
	log.G(ctx).WithField("benchmark", "RunTask").WithField("event", "Stop").Infof("Stop Run Task")
	if err != nil {
		b.Fatalf("%s", err)
	}
	defer cleanupRun()
	containerSecondRun, cleanupContainerSecondRun, err := containerdProcess.CreateContainer(ctx, image)
	if err != nil {
		b.Fatalf("%s", err)
	}
	defer cleanupContainerSecondRun()
	taskDetailsSecondRun, cleanupTaskSecondRun, err := containerdProcess.CreateTask(ctx, containerSecondRun)
	if err != nil {
		b.Fatalf("%s", err)
	}
	defer cleanupTaskSecondRun()
	log.G(ctx).WithField("benchmark", "RunTaskTwice").WithField("event", "Start").Infof("Start Run Task Twice")
	cleanupRunSecond, err := containerdProcess.RunContainerTaskForReadyLine(ctx, taskDetailsSecondRun, readyLine)
	log.G(ctx).WithField("benchmark", "RunTaskTwice").WithField("event", "Stop").Infof("Stop Run Task Twice")
	if err != nil {
		b.Fatalf("%s", err)
	}
	defer cleanupRunSecond()
	log.G(ctx).WithField("benchmark", "Test").WithField("event", "Stop").Infof("Stop Test")
	b.StopTimer()
}

func StargzFullRun(
	ctx context.Context,
	b *testing.B,
	imageRef string,
	readyLine string,
	stargzBinary string) {
	containerdProcess, err := getContainerdProcess(ctx, containerdStargzConfig)
	if err != nil {
		b.Fatalf("Failed to create containerd proc: %v\n", err)
	}
	defer containerdProcess.StopProcess()
	stargzProcess, err := getStargzProcess(stargzBinary)
	if err != nil {
		b.Fatalf("Failed to create stargz proc: %v\n", err)
	}
	defer stargzProcess.StopProcess()
	stargzContainerdProc := StargzContainerdProcess{containerdProcess}
	b.ResetTimer()
	image, err := stargzContainerdProc.StargzRpullImageFromECR(ctx, imageRef, awsSecretFile)
	if err != nil {
		b.Fatalf("%s", err)
	}
	container, cleanupContainer, err := stargzContainerdProc.CreateContainer(ctx, image, containerd.WithSnapshotter("stargz"))
	if err != nil {
		b.Fatalf("%s", err)
	}
	defer cleanupContainer()
	taskDetails, cleanupTask, err := containerdProcess.CreateTask(ctx, container)
	if err != nil {
		b.Fatalf("%s", err)
	}
	defer cleanupTask()
	cleanupRun, err := containerdProcess.RunContainerTaskForReadyLine(ctx, taskDetails, readyLine)
	if err != nil {
		b.Fatalf("%s", err)
	}
	defer cleanupRun()
	b.StopTimer()
}

func getContainerdProcess(ctx context.Context, containerdConfig string) (*framework.ContainerdProcess, error) {
	return framework.StartContainerd(
		containerdAddress,
		containerdRoot,
		containerdState,
		containerdConfig,
		outputDir)
}

func getSociProcess() (*SociProcess, error) {
	return StartSoci(
		sociBinary,
		sociAddress,
		sociRoot,
		containerdAddress,
		sociConfig,
		outputDir)
}

func getStargzProcess(stargzBinary string) (*StargzProcess, error) {
	return StartStargz(
		stargzBinary,
		stargzAddress,
		stargzConfig,
		stargzRoot,
		outputDir)
}
