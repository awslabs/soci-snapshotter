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
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/awslabs/soci-snapshotter/soci/store"
	shell "github.com/awslabs/soci-snapshotter/util/dockershell"
)

func TestRunWithDefaultConfig(t *testing.T) {
	b, err := os.ReadFile("../config/config.toml") // example toml file
	if err != nil {
		t.Fatalf("error fetching example toml")
	}
	config := string(b)

	sh, c := newSnapshotterBaseShell(t)
	defer c()

	rebootContainerd(t, sh, getContainerdConfigToml(t, false), getSnapshotterConfigToml(t, false, config))
	// This will error internally if it fails to boot. If it boots successfully,
	// the config was successfully parsed and snapshotter is running
}

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
				sh.X(append(runSociCmd, "--name", getTestContainerName(index, container), "-d", image)...)
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
	shell.X("nerdctl", "exec", containerName,
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
			sh.X(append(runSociCmd, "--name", "test-container", "-d", image, "sleep", "infinity")...)

			sh.X("apt-get", "--no-install-recommends", "install", "-qy", "iptables")

			// TODO: Wait for the container to be up and running

			// Block network access to the registry
			if tt.config.networkDisableMsec > 0 {
				sh.X("iptables", "-A", "OUTPUT", "-d", registryHostIP, "-j", "DROP")
			}

			// Do something in the container that should work without network access
			commandSucceedStdout, _, err := sh.R("nerdctl", "exec", "test-container", "sh", "-c", "times")
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
			commandNetworkStdout, _, err := sh.R("nerdctl", "exec", "test-container", "cat", "/etc/hosts")
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
					if len(data) > 0 {
						t.Fatalf("network bound task produced unexpected output: %s", string(data))
					}
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
					if len(data) > 0 {
						t.Fatalf("network bound task produced unexpected output: %s", string(data))
					}
				}
			case <-time.After(100 * time.Millisecond):
				if tt.config.expectedSuccess {
					t.Fatal("network bound task produced no output when expecting success")
				}
			}
		})
	}
}

// TestRootFolderPermissions tests that non-root users can read "/".
// This is a regression test to verify that SOCI has the same behavior as the containerd
// overlayfs snapshotter and the stargz-snapshotter https://github.com/awslabs/soci-snapshotter/issues/664
func TestRootFolderPermission(t *testing.T) {
	image := alpineImage
	containerName := "TestRootFolderPermission"

	regConfig := newRegistryConfig()
	sh, done := newShellWithRegistry(t, regConfig)
	defer done()

	rebootContainerd(t, sh, getContainerdConfigToml(t, false), getSnapshotterConfigToml(t, false, tcpMetricsConfig))
	sh.X("soci", "image", "rpull", dockerhub(image).ref)
	// This should have all been pulled ahead of time.
	checkFuseMounts(t, sh, 0)
	// Verify that the mount permissions allow non-root to open "/"
	subfolders := sh.O(append(runSociCmd, "--name", containerName, "-d", "--user", "1000", dockerhub(image).ref, "ls", "/")...)
	if string(subfolders) == "" {
		t.Fatal("non-root user should be able to `ls /`")
	}
}

func TestRestartAfterSigint(t *testing.T) {
	const containerImage = alpineImage
	const killTimeout = 5
	const startTimeout = "5s"

	regConfig := newRegistryConfig()
	sh, done := newShellWithRegistry(t, regConfig)
	defer done()

	rebootContainerd(t, sh, getContainerdConfigToml(t, false), getSnapshotterConfigToml(t, false, tcpMetricsConfig))
	copyImage(sh, dockerhub(containerImage), regConfig.mirror(containerImage))
	indexDigest := buildIndex(sh, regConfig.mirror(containerImage), withMinLayerSize(0), withSpanSize(100*1024))
	sh.X("soci", "image", "rpull", "--user", regConfig.creds(), "--soci-index-digest", indexDigest, regConfig.mirror(containerImage).ref)
	sh.X("pkill", "-SIGINT", "soci-snapshotte") // pkill can only take up to 15 chars

	var buffer []byte
	timedOut := true
	for i := 0; i < killTimeout*4; i++ {
		buffer = sh.O("ps")
		if !strings.Contains(string(buffer), "soci-snapshotte") {
			timedOut = false
			break
		}
		sh.X("sleep", "0.25")
	}

	if timedOut {
		t.Fatalf("failed to kill snapshotter daemon")
	}

	timeoutCmd := []string{"timeout", "--preserve-status", startTimeout}
	cmd := shell.C("/usr/local/bin/soci-snapshotter-grpc", "--log-level", sociLogLevel,
		"--address", "/run/soci-snapshotter-grpc/soci-snapshotter-grpc.sock")
	cmd = addConfig(t, sh, getSnapshotterConfigToml(t, false, tcpMetricsConfig), cmd...)
	cmd = append(timeoutCmd, cmd...)

	// exit status 0 — command timed out, so server is running — PASS
	// exit status 1 — server crashed before time limit — FAIL
	if _, err := sh.OLog(cmd...); err != nil {
		if err.Error() != "exit status 0" {
			t.Fatalf("error starting snapshotter daemon: %v", err)
		}
	}
}

func TestRunInContentStore(t *testing.T) {
	imageName := helloImage
	sh, done := newSnapshotterBaseShell(t)
	defer done()

	for _, createContentStoreType := range store.ContentStoreTypes() {
		for _, runContentStoreType := range store.ContentStoreTypes() {
			t.Run("create in "+string(createContentStoreType)+", run in "+string(runContentStoreType), func(t *testing.T) {
				rebootContainerd(t, sh, "", getSnapshotterConfigToml(t, false, tcpMetricsConfig, GetContentStoreConfigToml(store.WithType(runContentStoreType))))
				imageInfo := dockerhub(imageName)
				indexDigest := buildIndex(sh, imageInfo, withMinLayerSize(0), withContentStoreType(createContentStoreType))
				if indexDigest == "" {
					t.Fatal("failed to build index")
				}
				sh.X("soci", "image", "rpull", "--soci-index-digest", indexDigest, imageInfo.ref)
				// Run the container
				_, err := sh.OLog(append(runSociCmd, "--name", "test", "--rm", imageInfo.ref)...)
				if err != nil {
					t.Fatalf("encountered error running container: %v", err)
				}
				if createContentStoreType == runContentStoreType {
					// same content store should succeed and use soci
					checkFuseMounts(t, sh, 1)
				} else {
					// different content store should fallback to overlayfs
					checkFuseMounts(t, sh, 0)
				}
			})
		}
	}
}

func TestRunInNamespace(t *testing.T) {
	imageName := helloImage
	sh, done := newSnapshotterBaseShell(t)
	defer done()

	namespaces := []string{"default", "test"}

	for _, contentStoreType := range store.ContentStoreTypes() {
		for _, createNamespace := range namespaces {
			for _, runNamespace := range namespaces {
				t.Run("content store "+string(contentStoreType)+", create in "+createNamespace+", run in "+runNamespace, func(t *testing.T) {
					rebootContainerd(t, sh, "", getSnapshotterConfigToml(t, false, tcpMetricsConfig, GetContentStoreConfigToml(store.WithType(contentStoreType), store.WithNamespace(runNamespace))))
					imageInfo := dockerhub(imageName)
					indexDigest := buildIndex(sh, imageInfo, withMinLayerSize(0), withContentStoreType(contentStoreType), withNamespace(createNamespace))
					if indexDigest == "" {
						t.Fatal("failed to build index")
					}
					sh.X("soci",
						"--namespace", createNamespace,
						"--content-store", string(contentStoreType),
						"image", "rpull", "--soci-index-digest", indexDigest, imageInfo.ref)
					// Run the container
					_, err := sh.OLog(append(runSociCmd, "--namespace", runNamespace, "--rm", "--name", "test", imageInfo.ref)...)
					if createNamespace == runNamespace {
						// same namespace should succeed without overlayfs fallback
						if err != nil {
							t.Fatalf("encountered error running container: %v", err)
						}
						checkFuseMounts(t, sh, 1)
					} else {
						// different namespace should fail to launch the container
						if err == nil {
							t.Fatal("container launch succeeded unexpectedly")
						}
					}
				})
			}
		}
	}
}
