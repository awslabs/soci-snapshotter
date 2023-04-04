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
	"bufio"
	"bytes"
	"fmt"
	"regexp"
	"testing"

	shell "github.com/awslabs/soci-snapshotter/util/dockershell"
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
		},
		{
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
		{
			name: "Run multiple containers from different images with shared layers",
			containers: []containerImageAndTestFunc{
				{
					containerImage: nginxAlpineImage,
					testFunc:       testWebServiceContainer,
				},
				{
					containerImage: nginxAlpineImage2,
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

			rebootContainerd(t, sh, getContainerdConfigToml(t, false), getSnapshotterConfigToml(t, false, tcpMetricsConfig))
			for _, container := range deduplicateByContainerImage(tt.containers) {
				// Mirror image
				copyImage(sh, dockerhub(container.containerImage), regConfig.mirror(container.containerImage))
				// Pull image, create SOCI index
				indexDigest := buildIndex(sh, regConfig.mirror(container.containerImage), withMinLayerSize(0))

				sh.X("soci", "image", "rpull", "--user", regConfig.creds(), "--soci-index-digest", indexDigest, regConfig.mirror(container.containerImage).ref)
			}

			var getTestContainerName = func(index int, container containerImageAndTestFunc) string {
				return "test_" + fmt.Sprint(index) + "_" + makeImageNameValid(container.containerImage)
			}
			// Run the containers
			for index, container := range tt.containers {
				image := regConfig.mirror(container.containerImage).ref
				sh.X("soci", "run", "-d", "--snapshotter=soci", image, getTestContainerName(index, container))
			}

			// Verify that no mounts fallback to overlayfs
			curlOutput := string(sh.O("curl", tcpMetricsAddress+metricsPath))
			if err := checkOverlayFallbackCount(curlOutput, 0); err != nil {
				t.Fatal(err)
			}

			// Do something in each container
			for index, container := range tt.containers {
				container.testFunc(sh, getTestContainerName(index, container))
			}

			// Check for independent writeable snapshots for each container
			mountsScanner := bufio.NewScanner(bufio.NewReader(bytes.NewReader(sh.O("mount"))))
			upperdirs := make(map[string]bool)
			workdirs := make(map[string]bool)
			mountRegex := regexp.MustCompile(`^overlay on \/run\/containerd\/io.containerd.runtime.v2.task\/default\/(?P<containerName>\w+)\/rootfs type overlay \(rw,.*,lowerdir=(?P<lowerdirs>.*),upperdir=(?P<upperdir>.*),workdir=(?P<workdir>.*)\)$`)
			mountRegexGroupNames := mountRegex.SubexpNames()
			for mountsScanner.Scan() {
				findResult := mountRegex.FindAllStringSubmatch(mountsScanner.Text(), -1)
				if findResult == nil {
					continue
				}
				matches := findResult[0]
				for i, match := range matches {
					if mountRegexGroupNames[i] == "upperdir" {
						if upperdirs[match] {
							t.Fatalf("Duplicate overlay mount upperdir: %s", match)
						} else {
							upperdirs[match] = true
						}
					} else if mountRegexGroupNames[i] == "workdir" {
						if workdirs[match] {
							t.Fatalf("Duplicate overlay mount workdir: %s", match)
						} else {
							workdirs[match] = true
						}
					}
				}
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
