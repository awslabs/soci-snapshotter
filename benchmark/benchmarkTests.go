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
	log.G(ctx).WithField("test_name", testName).WithField("benchmark", "Test").WithField("event", "Start").Infof("Start Test")
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
	image, err := sociContainerdProc.SociRPullImageFromECR(ctx, imageRef, indexDigest, awsSecretFile)
	if err != nil {
		b.Fatalf("%s", err)
	}
	container, cleanupContainer, err := sociContainerdProc.CreateSociContainer(ctx, image)
	if err != nil {
		b.Fatalf("%s", err)
	}
	defer cleanupContainer()
	taskDetails, cleanupTask, err := sociContainerdProc.CreateTask(ctx, container)
	if err != nil {
		b.Fatalf("%s", err)
	}
	defer cleanupTask()
	cleanupRun, err := sociContainerdProc.RunContainerTaskForReadyLine(ctx, taskDetails, readyLine)
	if err != nil {
		b.Fatalf("%s", err)
	}
	defer cleanupRun()
	b.StopTimer()
}

func OverlayFSFullRun(
	ctx context.Context,
	b *testing.B,
	imageRef string,
	readyLine string,
	testName string) {
	log.G(ctx).WithField("test_name", testName).WithField("benchmark", "Test").WithField("event", "Start").Infof("Start Test")
	containerdProcess, err := getContainerdProcess(ctx, containerdSociConfig)
	if err != nil {
		b.Fatalf("Failed to create containerd proc: %v\n", err)
	}
	defer containerdProcess.StopProcess()
	b.ResetTimer()
	image, err := containerdProcess.PullImageFromECR(ctx, imageRef, platform, awsSecretFile)
	if err != nil {
		b.Fatalf("%s", err)
	}
	container, cleanupContainer, err := containerdProcess.CreateContainer(ctx, image)
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
	_, cleanupContainer, err := stargzContainerdProc.CreateContainer(ctx, image, containerd.WithSnapshotter("stargz"))
	if err != nil {
		b.Fatalf("%s", err)
	}
	defer cleanupContainer()
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
