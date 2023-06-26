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
	"encoding/csv"
	"errors"
	"os"
	"os/exec"
)

type ImageDescriptor struct {
	ShortName            string
	ImageRef             string
	SociIndexManifestRef string
	ReadyLine            string
}

func GetImageListFromCsv(csvLoc string) ([]ImageDescriptor, error) {
	csvFile, err := os.Open(csvLoc)
	if err != nil {
		return nil, err
	}
	csv, err := csv.NewReader(csvFile).ReadAll()
	if err != nil {
		return nil, err
	}
	var images []ImageDescriptor
	for _, image := range csv {
		if len(image) < 3 {
			return nil, errors.New("image input is not sufficient")
		}
		var sociIndexManifestRef string
		if len(image) == 4 {
			sociIndexManifestRef = image[2]
		}
		images = append(images, ImageDescriptor{
			ShortName:            image[0],
			ImageRef:             image[1],
			ReadyLine:            image[3],
			SociIndexManifestRef: sociIndexManifestRef})
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
			ShortName:            "ECR-public-ffmpeg",
			ImageRef:             "public.ecr.aws/soci-workshop-examples/ffmpeg:latest",
			SociIndexManifestRef: "ef63578971ebd8fc700c74c96f81dafab4f3875e9117ef3c5eb7446e169d91cb",
			ReadyLine:            "Hello World",
		},
		{
			ShortName:            "ECR-public-tensorflow",
			ImageRef:             "public.ecr.aws/soci-workshop-examples/tensorflow:latest",
			SociIndexManifestRef: "27546e0267465279e40a8c8ebc8d34836dd5513c6e7019257855c9e0f04a9f34",
			ReadyLine:            "Hello World with TensorFlow!",
		},
		{
			ShortName:            "ECR-public-tensorflow_gpu",
			ImageRef:             "public.ecr.aws/soci-workshop-examples/tensorflow_gpu:latest",
			SociIndexManifestRef: "a40b70bc941216cbb29623e98970dfc84e9640666a8b9043564ca79f6d5cc137",
			ReadyLine:            "Hello World with TensorFlow!",
		},
		{
			ShortName:            "ECR-public-node",
			ImageRef:             "public.ecr.aws/soci-workshop-examples/node:latest",
			SociIndexManifestRef: "544d42d3447fe7833c2e798b8a342f5102022188e814de0aa6ce980e76c62894",
			ReadyLine:            "Server ready",
		},
		{
			ShortName:            "ECR-public-busybox",
			ImageRef:             "public.ecr.aws/soci-workshop-examples/busybox:latest",
			SociIndexManifestRef: "deaaf67bb4baa293dadcfbeb1f511c181f89a05a042ee92dd2e43e7b7295b1c0",
			ReadyLine:            "Hello World",
		},
		{
			ShortName:            "ECR-public-mongo",
			ImageRef:             "public.ecr.aws/soci-workshop-examples/mongo:latest",
			SociIndexManifestRef: "ecdd6dcc917d09ec7673288e8ba83270542b71959db2ac731fbeb42aa0b038e0",
			ReadyLine:            "Waiting for connections",
		},
		{
			ShortName:            "ECR-public-rabbitmq",
			ImageRef:             "public.ecr.aws/soci-workshop-examples/rabbitmq:latest",
			SociIndexManifestRef: "3882f9609c0c2da044173710f3905f4bc6c09228f2a5b5a0a5fdce2537677c17",
			ReadyLine:            "Server startup complete",
		},
		{
			ShortName:            "ECR-public-redis",
			ImageRef:             "public.ecr.aws/soci-workshop-examples/redis:latest",
			SociIndexManifestRef: "da171fda5f4ccf79f453fc0c5e1414642521c2e189f377809ca592af9458287a",
			ReadyLine:            "Ready to accept connections",
		},
	}

}
