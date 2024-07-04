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
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"

	containerd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/contrib/nvidia"
	"github.com/containerd/containerd/v2/pkg/oci"
	runtimespec "github.com/opencontainers/runtime-spec/specs-go"
)

const testContainerID = "TEST_RUN_CONTAINER"

type ImageDescriptor struct {
	ShortName       string       `json:"short_name"`
	ImageRef        string       `json:"image_ref"`
	SociIndexDigest string       `json:"soci_index_digest"`
	ReadyLine       string       `json:"ready_line"`
	TimeoutSec      int64        `json:"timeout_sec"`
	ImageOptions    ImageOptions `json:"options"`
}

func (i *ImageDescriptor) Timeout() time.Duration {
	if i.TimeoutSec <= 0 {
		return 180 * time.Second
	}
	return time.Duration(i.TimeoutSec) * time.Second
}

// ImageOptions contains image-specific options needed to run the tests
type ImageOptions struct {
	// Net indicicates the container's network mode. If set to "host" then the container will have host networking, otherwise no networking.
	Net string `json:"net"`
	// Mounts are any mounts needed by the container
	Mounts []runtimespec.Mount `json:"mounts"`
	// Gpu is whether the container needs GPUs. If true, all GPUs are mounted in the container
	Gpu bool `json:"gpu"`
	// Env is any environment variables needed by the containerd
	Env []string `json:"env"`
	// ShmSize is the size of /dev/shm to be used inside the container
	ShmSize int64 `json:"shm_size"`
}

// ContainerOpts creates a set of NewContainerOpts from an ImageDescriptor and a containerd.Image
// The options can be used directly when launching a container
func (i *ImageDescriptor) ContainerOpts(image containerd.Image, o ...containerd.NewContainerOpts) []containerd.NewContainerOpts {
	var opts []containerd.NewContainerOpts
	var ociOpts []oci.SpecOpts

	opts = append(opts, o...)
	id := fmt.Sprintf("%s-%d", testContainerID, time.Now().UnixNano())
	opts = append(opts, containerd.WithNewSnapshot(id, image))
	ociOpts = append(ociOpts, oci.WithImageConfig(image))
	if len(i.ImageOptions.Mounts) > 0 {
		ociOpts = append(ociOpts, oci.WithMounts(i.ImageOptions.Mounts))
	}
	if i.ImageOptions.Gpu {
		ociOpts = append(ociOpts, nvidia.WithGPUs(nvidia.WithAllDevices, nvidia.WithAllCapabilities))
	}
	if len(i.ImageOptions.Env) > 0 {
		ociOpts = append(ociOpts, oci.WithEnv(i.ImageOptions.Env))
	}
	if i.ImageOptions.ShmSize > 0 {
		ociOpts = append(ociOpts, oci.WithDevShmSize(i.ImageOptions.ShmSize))
	}
	if i.ImageOptions.Net == "host" {
		hostname, err := os.Hostname()
		if err != nil {
			panic(fmt.Errorf("get hostname: %w", err))
		}
		ociOpts = append(ociOpts,
			oci.WithHostNamespace(runtimespec.NetworkNamespace),
			oci.WithHostHostsFile,
			oci.WithHostResolvconf,
			oci.WithEnv([]string{fmt.Sprintf("HOSTNAME=%s", hostname)}),
		)
	}

	opts = append(opts, containerd.WithNewSpec(ociOpts...))
	return opts
}

func GetImageList(file string) ([]ImageDescriptor, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return GetImageListFromJSON(f)

}

func GetImageListFromJSON(r io.Reader) ([]ImageDescriptor, error) {
	var images []ImageDescriptor
	err := json.NewDecoder(r).Decode(&images)
	if err != nil {
		return nil, err
	}
	return images, nil
}

func GetCommitHash() (string, error) {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(output), nil
}

func GetDefaultWorkloads() []ImageDescriptor {
	return []ImageDescriptor{
		{
			ShortName:       "ECR-public-ffmpeg",
			ImageRef:        "public.ecr.aws/soci-workshop-examples/ffmpeg:latest",
			SociIndexDigest: "sha256:ef63578971ebd8fc700c74c96f81dafab4f3875e9117ef3c5eb7446e169d91cb",
			ReadyLine:       "Hello World",
		},
		{
			ShortName:       "ECR-public-tensorflow",
			ImageRef:        "public.ecr.aws/soci-workshop-examples/tensorflow:latest",
			SociIndexDigest: "sha256:27546e0267465279e40a8c8ebc8d34836dd5513c6e7019257855c9e0f04a9f34",
			ReadyLine:       "Hello World with TensorFlow!",
		},
		{
			ShortName:       "ECR-public-tensorflow_gpu",
			ImageRef:        "public.ecr.aws/soci-workshop-examples/tensorflow_gpu:latest",
			SociIndexDigest: "sha256:a40b70bc941216cbb29623e98970dfc84e9640666a8b9043564ca79f6d5cc137",
			ReadyLine:       "Hello World with TensorFlow!",
		},
		{
			ShortName:       "ECR-public-node",
			ImageRef:        "public.ecr.aws/soci-workshop-examples/node:latest",
			SociIndexDigest: "sha256:544d42d3447fe7833c2e798b8a342f5102022188e814de0aa6ce980e76c62894",
			ReadyLine:       "Server ready",
		},
		{
			ShortName:       "ECR-public-busybox",
			ImageRef:        "public.ecr.aws/soci-workshop-examples/busybox:latest",
			SociIndexDigest: "sha256:deaaf67bb4baa293dadcfbeb1f511c181f89a05a042ee92dd2e43e7b7295b1c0",
			ReadyLine:       "Hello World",
		},
		{
			ShortName:       "ECR-public-mongo",
			ImageRef:        "public.ecr.aws/soci-workshop-examples/mongo:latest",
			SociIndexDigest: "sha256:ecdd6dcc917d09ec7673288e8ba83270542b71959db2ac731fbeb42aa0b038e0",
			ReadyLine:       "Waiting for connections",
		},
		{
			ShortName:       "ECR-public-rabbitmq",
			ImageRef:        "public.ecr.aws/soci-workshop-examples/rabbitmq:latest",
			SociIndexDigest: "sha256:3882f9609c0c2da044173710f3905f4bc6c09228f2a5b5a0a5fdce2537677c17",
			ReadyLine:       "Server startup complete",
		},
		{
			ShortName:       "ECR-public-redis",
			ImageRef:        "public.ecr.aws/soci-workshop-examples/redis:latest",
			SociIndexDigest: "sha256:da171fda5f4ccf79f453fc0c5e1414642521c2e189f377809ca592af9458287a",
			ReadyLine:       "Ready to accept connections",
		},
	}

}
