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
	"encoding/json"
	"fmt"
	"io"
	"math"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

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

type retryConfig struct {
	maxRetries         int
	minWaitMsec        int64
	maxWaitMsec        int64
	networkDisableMsec int64
	expectedSuccess    bool
}

// TestNetworkRetry runs a container, disables network access to the remote image, asks the container
// to do something requiring the remote image, waits for some/all requests to fail, enables the network,
// confirms retries and success/failure
func TestNetworkRetry(t *testing.T) {

	const containerImage = alpineImage

	tests := []struct {
		name   string
		config retryConfig
	}{
		{
			name: "No network interruption, no retries allowed, success",
			config: retryConfig{
				maxRetries:         -1,
				minWaitMsec:        0,
				maxWaitMsec:        0,
				networkDisableMsec: 0,
				expectedSuccess:    true,
			},
		},
		{
			name: "1s network interruption, no retries allowed, failure",
			config: retryConfig{
				maxRetries:         -1,
				minWaitMsec:        0,
				maxWaitMsec:        0,
				networkDisableMsec: 1000,
				expectedSuccess:    false,
			},
		},
		{
			name: "2s network interruption, ~9-10s retries allowed, success",
			config: retryConfig{
				maxRetries:         2,
				minWaitMsec:        100,
				maxWaitMsec:        1600,
				networkDisableMsec: 2000,
				expectedSuccess:    true,
			},
		},
		{
			name: "10s network interruption, ~6-7s retries allowed, failure",
			config: retryConfig{
				maxRetries:         1,
				minWaitMsec:        100,
				maxWaitMsec:        1600,
				networkDisableMsec: 10000,
				expectedSuccess:    false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			regConfig := newRegistryConfig()
			sh, done := newShellWithRegistry(t, regConfig)
			defer done()

			registryHostIP := getIP(t, sh, regConfig.host)

			config := `
[blob]
max_retries   = ` + strconv.Itoa(tt.config.maxRetries) + `
min_wait_msec = ` + strconv.FormatInt(tt.config.minWaitMsec, 10) + `
max_wait_msec = ` + strconv.FormatInt(tt.config.maxWaitMsec, 10) + `

[background_fetch]
disable = true
`

			m := rebootContainerd(t, sh, getContainerdConfigToml(t, false), getSnapshotterConfigToml(t, false, config))
			// Mirror image
			copyImage(sh, dockerhub(containerImage), regConfig.mirror(containerImage))
			// Pull image, create SOCI index with all layers and small (100kiB) spans
			indexDigest := buildIndex(sh, regConfig.mirror(containerImage), withMinLayerSize(0), withSpanSize(100*1024))
			sh.X("soci", "image", "rpull", "--user", regConfig.creds(), "--soci-index-digest", indexDigest, regConfig.mirror(containerImage).ref)

			// Run the container
			image := regConfig.mirror(containerImage).ref
			sh.X("soci", "run", "-d", "--snapshotter=soci", image, "test-container")

			sh.X("apt-get", "--no-install-recommends", "install", "-y", "iptables")

			// TODO: Wait for the container to be up and running

			// Block network access to the registry
			if tt.config.networkDisableMsec > 0 {
				sh.X("iptables", "-A", "OUTPUT", "-d", registryHostIP, "-j", "DROP")
			}

			// Do something in the container that should work without network access
			commandSucceedStdout, _, err := sh.R("ctr", "task", "exec", "--exec-id", "test-task-1", "test-container", "sh", "-c", "times")
			if err != nil {
				t.Fatalf("attempt to run task without network access failed: %s", err)
			}

			type ErrorLogLine struct {
				Error string `json:"error"`
				Msg   string `json:"msg"`
			}

			gaveUpChannel := make(chan bool, 1)
			defer close(gaveUpChannel)

			monitorGaveUp := func(rawL string) {
				if i := strings.Index(rawL, "{"); i > 0 {
					rawL = rawL[i:] // trim garbage chars; expects "{...}"-styled JSON log
				}
				var logLine ErrorLogLine
				if err := json.Unmarshal([]byte(rawL), &logLine); err == nil {
					if logLine.Msg == "statFile error" && strings.Contains(logLine.Error, "giving up after") {
						gaveUpChannel <- true
						return
					}
				}
			}

			m.Add("retry", monitorGaveUp)
			defer m.Remove("retry")

			// Do something in the container to access un-fetched spans, requiring network access
			commandNetworkStdout, _, err := sh.R("ctr", "task", "exec", "--exec-id", "test-task-2", "test-container", "cat", "/etc/hosts")
			if err != nil {
				t.Fatalf("attempt to run task requiring network access failed: %s", err)
			}

			// Wait with network disabled
			time.Sleep(time.Duration(tt.config.networkDisableMsec) * time.Millisecond)

			// Short wait to allow commands to complete
			time.Sleep(time.Duration(1000) * time.Millisecond)

			// Confirm first command succeeded while network was down
			buf := make([]byte, 100)
			if _, err = commandSucceedStdout.Read(buf); err != nil {
				t.Fatalf("read from expected successful task output failed: %s", err)
			}
			if !strings.Contains(string(buf), "s ") { // `times` output looks like "0m0.03s 0m0.05s"
				t.Fatalf("expected successful task produced unexpected output: %s", string(buf))
			}

			// async read from command_network_stdout
			commandNetworkStdoutChannel := make(chan []byte)
			commandNetworkErrChannel := make(chan error)
			go func() {
				defer close(commandNetworkStdoutChannel)
				defer close(commandNetworkErrChannel)
				var b []byte
				if _, err := commandNetworkStdout.Read(b); err != nil && err != io.EOF {
					commandNetworkErrChannel <- fmt.Errorf("read from network bound task output failed: %s", err)
					return
				}
				commandNetworkStdoutChannel <- b
				if err == io.EOF {
					commandNetworkErrChannel <- fmt.Errorf("read from network bound task output encountered EOF")
					return
				}
			}()

			if tt.config.networkDisableMsec > 0 {

				// Confirm second command has not succeeded while network was down
				select {
				case err := <-commandNetworkErrChannel:
					t.Fatal(err)
				case data := <-commandNetworkStdoutChannel:
					t.Fatalf("network bound task produced unexpected output: %s", string(data))
				case <-time.After(100 * time.Millisecond):
				}

				// Restore access to the registry and image
				sh.X("iptables", "-D", "OUTPUT", "-d", registryHostIP, "-j", "DROP")

				// Wait with network enabled, so a final retry has a chance to succeed
				time.Sleep(2 * time.Millisecond * time.Duration(math.Min(
					float64(tt.config.maxWaitMsec),
					math.Pow(2, float64(tt.config.maxRetries))*float64(tt.config.minWaitMsec),
				)))
			}

			// Confirm whether second command has succeeded with network restored

			select {
			case gaveUp := <-gaveUpChannel:
				if tt.config.expectedSuccess && gaveUp {
					t.Fatal("retries gave up despite test expecting retry success")
				}
			case data := <-commandNetworkStdoutChannel:
				if !tt.config.expectedSuccess {
					t.Fatalf("network bound task produced unexpected output: %s", string(data))
				}
			case <-time.After(100 * time.Millisecond):
				if tt.config.expectedSuccess {
					t.Fatal("network bound task produced no output when expecting success")
				}
			}
		})
	}
}
