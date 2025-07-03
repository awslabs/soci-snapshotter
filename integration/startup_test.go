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

package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/awslabs/soci-snapshotter/config"
	shell "github.com/awslabs/soci-snapshotter/util/dockershell"
	"github.com/awslabs/soci-snapshotter/util/testutil"
	"github.com/rs/xid"
)

// example toml file
const defaultConfigFileLocation = "../config/config.toml"

// Use a custom metrics config to test that the snapshotter
// correctly starts up when the metrics address is next to the socket address.
// This tests a regression with the first implementation of systemd socket activation
// where we moved creation of the directory later which caused the metrics address
// bind to fail. This verifies that the directory gets created before binding the metrics socket.
func withCustomMetricsConfig(cfg *config.Config) {
	cfg.MetricsAddress = "/run/soci-snapshotter-grpc/metrics.sock"
	cfg.MetricsNetwork = "unix"
}

// TestSnapshotterStartup tests to run containerd + snapshotter and check plugin is
// recognized by containerd
func TestSnapshotterStartup(t *testing.T) {
	t.Parallel()
	sh, done := newSnapshotterBaseShell(t)
	defer done()
	rebootContainerd(t, sh, "", getSnapshotterConfigToml(t, withCustomMetricsConfig))
	found := false
	err := sh.ForEach(shell.C("ctr", "plugin", "ls"), func(l string) bool {
		info := strings.Fields(l)
		if len(info) < 4 {
			t.Fatalf("malformed plugin info: %v", info)
		}
		if info[0] == "io.containerd.snapshotter.v1" && info[1] == "soci" && info[3] == "ok" {
			found = true
			return false
		}
		return true
	})
	if err != nil || !found {
		t.Fatalf("failed to get soci snapshotter status using ctr plugin ls: %v", err)
	}
}

// TestSnapshotterSystemdStartup tests that SOCI interacts with systemd socket activation correctly.
// It verifies that SOCI works when using socket activation and that SOCI starts up correctly when
// it is configured for socket activation, but it launches directly.
func TestSnapshotterSystemdStartup(t *testing.T) {
	if os.Getenv("SKIP_SYSTEMD_TESTS") != "" {
		t.Skip("Skipping systemd tests")
	}

	tests := []struct {
		name                 string
		init                 func(*shell.Shell)
		expectedErrorMatcher *regexp.Regexp
	}{
		{
			name: "fails when soci is not started at all",
			init: func(s *shell.Shell) {
				// SOCI is not started at all. We expect a timeout
				// when preparing a snapshot
			},
			expectedErrorMatcher: regexp.MustCompile("timeout"),
		},
		{
			name: "succeeds when soci is started by systemd socket activation",
			init: func(s *shell.Shell) {
				// systemd listens on the soci socket, but the snapshotter doesn't start immediately.
				// When containerd first opens the soci socket, systemd forks the snapshotter process
				// with the socket fd open which the snapshotter then uses to listen for snapshot requests.
				// We expect the snapshotter to launch and work when we make the prepare request
				s.X("systemctl", "start", "soci-snapshotter.socket")
			},
			expectedErrorMatcher: nil,
		},
		{
			name: "succeeds when soci is started manually when expecting socket activation",
			init: func(s *shell.Shell) {
				// The snapshotter is started directly, but it's expecting an open file and info
				// from systemd. This tests that SOCI will correctly fallback to the default socket address
				// if the SOCI systemd unit says to use systemd socket activation, but the snapshotter is
				// started directly instead.
				// We expect the snapshotter to launch during this init and be available when we make the prepare request.
				s.X("systemctl", "start", "soci-snapshotter")
			},
			expectedErrorMatcher: nil,
		},
	}

	var isExpectedError = func(output []byte, err error, matcher *regexp.Regexp) bool {
		if err == nil {
			return matcher == nil
		}
		if matcher == nil {
			return false
		}
		return matcher.Match(output)
	}

	for _, tc := range tests {
		t.Run(tc.name, func(tt *testing.T) {
			sh, cleanup := newSnapshotterBaseShell(tt, withEntrypoint("/usr/lib/systemd/systemd"))
			defer cleanup()

			sh.X("containerd", "--version")
			// We're not using `rebootContainerd` here because that also restarts the soci-snapshotter
			sh.Gox("containerd", "--log-level", containerdLogLevel)
			// wait for contaienrd to start up
			sh.Retry(100, "ctr", "snapshots", "prepare", "connectiontest-dummy-"+xid.New().String(), "")
			tc.init(sh)
			// 2s timeout is arbitrary so that the negative test doesn't take too long. It does mean that
			// the snapshotter has to start up and prepare a snapshot within 2s in the socket activation case.
			output, err := sh.CombinedOLog("ctr", "--timeout", "2s", "snapshot", "--snapshotter", "soci", "prepare", "test")
			if !isExpectedError(output, err, tc.expectedErrorMatcher) {
				tt.Fatalf("unexpected error preparing snapshot: %v", string(output))
			}
		})
	}
}

// TestSnapshotterStartupWithBadConfig ensures snapshotter does not start if snapshotter values are incorrect.
// Note that incorrect fields are ignored by TOML and thus are expected to work
func TestSnapshotterStartupWithBadConfig(t *testing.T) {
	tests := []struct {
		name           string
		opts           []snapshotterConfigOpt
		expectedErrStr string
	}{
		{
			name:           "Bad parallel pull size",
			opts:           []snapshotterConfigOpt{withParallelPullMode(), withConcurrentDownloadChunkSizeStr("badstring")},
			expectedErrStr: "invalid size format",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(tt *testing.T) {
			sh, cleanup := newSnapshotterBaseShell(t)
			defer cleanup()
			// rebootContainerd would be ideal here, but that always uses a config,
			// so we just manually start SOCI with/without a config file.
			err := testutil.KillMatchingProcess(sh, "soci-snapshotter-grpc")
			if err != nil {
				sh.Fatal("failed to kill soci: %v", err)
			}
			configToml := getSnapshotterConfigToml(t, tc.opts...)
			snRunCmd := []string{"/usr/local/bin/soci-snapshotter-grpc", "--log-level", sociLogLevel}
			snRunCmd = addConfig(t, sh, configToml, snRunCmd...)

			errCh := make(chan error, 1)
			go func() {
				_, err := sh.OLog(snRunCmd...)
				errCh <- err
			}()

			select {
			case <-errCh:
			case <-time.After(2 * time.Second):
				t.Fatalf("expected err %s but snapshotter did not fail within 2 seconds", tc.expectedErrStr)
			}
		})
	}
}

func TestStartWithDefaultConfig(t *testing.T) {
	defaultConfigToml, err := os.ReadFile(defaultConfigFileLocation)
	if err != nil {
		t.Fatalf("error fetching example toml: %v", err)
	}

	sh, c := newSnapshotterBaseShell(t)
	defer c()

	rebootContainerd(t, sh, getContainerdConfigToml(t, false), string(defaultConfigToml))
	// This will error internally if it fails to boot. If it boots successfully,
	// the config was successfully parsed and snapshotter is running
}

// TestStartWithoutConfig checks that SOCI can start with specified config paths
func TestStartWithConfigPaths(t *testing.T) {
	sh, done := newSnapshotterBaseShell(t)
	defer done()

	snapshotterSocket := "/run/soci-snapshotter-grpc/soci-snapshotter-grpc.sock"

	tests := []struct {
		name             string
		defaultPath      bool
		fileExists       bool
		expectedErrorStr string
	}{
		{
			name:        "should start if config file is present in default location",
			defaultPath: true,
			fileExists:  true,
		},
		{
			name:        "should start if config file is not present in default location",
			defaultPath: true,
			fileExists:  false,
		},
		{
			name:        "should start if config file is present in non-default specified location",
			defaultPath: false,
			fileExists:  true,
		},
		{
			name:             "should not start if config file is not present in non-default specified location",
			defaultPath:      false,
			fileExists:       false,
			expectedErrorStr: "failed to open config file",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// rebootContainerd would be ideal here, but that always uses a config,
			// so we just manually start SOCI with/without a config file.
			err := testutil.KillMatchingProcess(sh, "soci-snapshotter-grpc")
			if err != nil {
				sh.Fatal("failed to kill soci: %v", err)
			}

			snRunCmd := []string{"/usr/local/bin/soci-snapshotter-grpc", "--log-level", sociLogLevel,
				"--address", snapshotterSocket}

			var configPath string
			if tc.defaultPath {
				configPath = defaultSnapshotterConfigPath
			} else {
				dir, err := testutil.TempDir(sh)
				if err != nil {
					t.Fatalf("error creating temp dir: %v", err)
				}
				defer func() {
					sh.X("rm", "-rf", dir)
				}()

				configPath = filepath.Join(dir, "config.toml")
				snRunCmd = append(snRunCmd, "--config", configPath)
			}

			sh.X("rm", "-rf", configPath)
			if tc.fileExists {
				sh.X("touch", configPath)
			}

			outR, errR, err := sh.R(snRunCmd...)
			if err != nil {
				t.Fatalf("failed to create pipe: %v", err)
			}
			reporter := testutil.NewTestingReporter(t)
			m := testutil.NewLogMonitor(reporter, outR, errR)
			errMatch := false
			errStr := ""
			if tc.expectedErrorStr != "" {
				m.Add("config", func(rawL string) {
					if i := strings.Index(rawL, "{"); i > 0 {
						rawL = rawL[i:] // trim garbage chars; expects "{...}"-styled JSON log
					}
					var logline testutil.LevelLogLine
					if err := json.Unmarshal([]byte(rawL), &logline); err == nil {
						if logline.Level == "fatal" {
							if strings.Contains(logline.Msg, tc.expectedErrorStr) {
								errMatch = true
							} else {
								errStr = logline.Msg
							}
						}
					}
				})
				defer m.Remove("config")
			}

			err = testutil.LogConfirmStartup(m)
			// LogConfirmStartup has a 10 second timeout, so we can reasonably expect the LogMonitor func above
			// to have caught the config at this point, and if not we can assume it failed.

			if err == nil {
				if tc.expectedErrorStr != "" {
					t.Fatalf("snapshotter startup expected to fail with string \"%v\" but incorrectly succeeded", tc.expectedErrorStr)
				}
			} else {
				if tc.expectedErrorStr == "" {
					t.Fatalf("snapshotter unexpectedly failed: %v", err)
				} else if !errMatch {
					t.Fatalf("snapshotter startup expected to fail with string \"%v\" but got %s", tc.expectedErrorStr, errStr)
				}
			}
		})
	}
}
