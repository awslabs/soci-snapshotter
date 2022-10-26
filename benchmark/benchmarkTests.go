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
)

var (
	outputDir         = "./output"
	containerdAddress = "/tmp/containerd-grpc/containerd.sock"
	containerdRoot    = "/tmp/lib/containerd"
	containerdState   = "/tmp/containerd"
	containerdConfig  = "../containerd-config.toml"
	platform          = "linux/amd64"
	sociBinary        = "../../out/soci-snapshotter-grpc"
	sociAddress       = "/tmp/soci-snapshotter-grpc/soci-snapshotter-grpc.sock"
	sociRoot          = "/tmp/lib/soci-snapshotter-grpc"
	sociConfig        = "../soci_config.toml"
	awsSecretFile     = "../aws_secret"
)

func PullImageFromECR(b *testing.B, imageRef string) {
	ctx, cancelCtx := framework.GetTestContext()
	containerdProcess, err := getContainerdProcess(ctx)
	if err != nil {
		b.Fatalf("Error Starting Containerd: %v\n", err)
	}
	defer containerdProcess.StopProcess(cancelCtx)
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
	b *testing.B,
	imageRef string,
	indexDigest string) {
	ctx, cancelCtx := framework.GetTestContext()
	containerdProcess, err := getContainerdProcess(ctx)
	if err != nil {
		b.Fatalf("Failed to create containerd proc: %v\n", err)
	}
	defer containerdProcess.StopProcess(cancelCtx)
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
	b *testing.B,
	imageRef string,
	indexDigest string,
	readyLine string) {
	ctx, cancelCtx := framework.GetTestContext()
	containerdProcess, err := getContainerdProcess(ctx)
	if err != nil {
		b.Fatalf("Failed to create containerd proc: %v\n", err)
	}
	defer containerdProcess.StopProcess(cancelCtx)
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
	b *testing.B,
	imageRef string,
	readyLine string) {
	ctx, cancelCtx := framework.GetTestContext()
	containerdProcess, err := getContainerdProcess(ctx)
	if err != nil {
		b.Fatalf("Failed to create containerd proc: %v\n", err)
	}
	defer containerdProcess.StopProcess(cancelCtx)
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

func getContainerdProcess(ctx context.Context) (*framework.ContainerdProcess, error) {
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
