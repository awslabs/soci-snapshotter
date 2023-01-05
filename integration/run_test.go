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

package integration

import (
	"fmt"
	"regexp"
	"testing"

	shell "github.com/awslabs/soci-snapshotter/util/dockershell"
	"github.com/awslabs/soci-snapshotter/util/testutil"
)

// TestRunMultipleContainers runs multiple containers at the same time and performs a test in each
func TestRunMultipleContainers(t *testing.T) {

	tests := []struct {
		name       string
		containers []containerImageAndTestFunc
	}{
		{
			name: "Run multiple containers from the same image",
			containers: []containerImageAndTestFunc{
				{
					containerImage: nginxImage,
					testFunc:       testWebServiceContainer,
				},
				{
					containerImage: nginxImage,
					testFunc:       testWebServiceContainer,
				},
			},
		}, {
			name: "Run multiple containers from different images",
			containers: []containerImageAndTestFunc{
				{
					containerImage: nginxImage,
					testFunc:       testWebServiceContainer,
				},
				{
					containerImage: drupalImage,
					testFunc:       testWebServiceContainer,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			regConfig := newRegistryConfig()
			sh, done := newShellWithRegistry(t, regConfig)
			defer done()

			// Initialize config files for containerd and snapshotter
			additionalConfig := ""
			if !isTestingBuiltinSnapshotter() {
				additionalConfig = proxySnapshotterConfig
			}
			containerdConfigYaml := testutil.ApplyTextTemplate(t, `
version = 2

[plugins."io.containerd.snapshotter.v1.soci"]
root_path = "/var/lib/soci-snapshotter-grpc/"

[plugins."io.containerd.snapshotter.v1.soci".blob]
check_always = true

{{.AdditionalConfig}}
`, struct {
				AdditionalConfig string
			}{
				AdditionalConfig: additionalConfig,
			})

			// Setup environment
			if err := testutil.WriteFileContents(sh, defaultContainerdConfigPath, []byte(containerdConfigYaml), 0600); err != nil {
				t.Fatalf("failed to write %v: %v", defaultContainerdConfigPath, err)
			}
			sh.
				X("update-ca-certificates").
				Retry(100, "nerdctl", "login", "-u", regConfig.user, "-p", regConfig.pass, regConfig.host)

			rebootContainerd(t, sh, "", "")
			for _, container := range deduplicateByContainerImage(tt.containers) {
				// Mirror image
				copyImage(sh, dockerhub(container.containerImage), regConfig.mirror(container.containerImage))
				// Pull image, create SOCI index
				indexDigest := optimizeImage(sh, regConfig.mirror(container.containerImage))

				sh.X("soci", "image", "rpull", "--user", regConfig.creds(), "--soci-index-digest", indexDigest, regConfig.mirror(container.containerImage).ref)
			}

			// Run the containers
			for index, container := range tt.containers {
				image := regConfig.mirror(container.containerImage).ref
				containerName := "test_" + fmt.Sprint(index) + "_" + makeImageNameValid(container.containerImage)
				sh.X("soci", "run", "-d", "--rm", "--snapshotter=soci", image, containerName)
			}

			// Do something in each container
			for index, container := range tt.containers {
				containerName := "test_" + fmt.Sprint(index) + "_" + makeImageNameValid(container.containerImage)
				container.testFunc(sh, containerName)
			}
		})
	}
}

func deduplicateByContainerImage(origList []containerImageAndTestFunc) []containerImageAndTestFunc {
	foundItems := make(map[string]bool)
	newList := []containerImageAndTestFunc{}
	for _, item := range origList {
		if _, exists := foundItems[item.containerImage]; !exists {
			foundItems[item.containerImage] = true
			newList = append(newList, item)
		}
	}
	return newList
}

// makeImageNameValid replaces special characters other than "_.-", and leading ".-", with "_"
func makeImageNameValid(imageName string) string {
	return regexp.MustCompile(`^[.-]|[^a-zA-Z0-9_.-]+`).ReplaceAllString(imageName, "_")
}

type containerImageAndTestFunc struct {
	containerImage string
	testFunc       func(*shell.Shell, string)
}

func testWebServiceContainer(shell *shell.Shell, containerName string) {
	shell.X("ctr", "task", "exec", "--exec-id", "test-curl", containerName,
		"curl", "--retry", "5", "--retry-connrefused", "--retry-max-time", "30", "http://127.0.0.1",
	)
}
