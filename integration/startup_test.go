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
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/awslabs/soci-snapshotter/config"
	shell "github.com/awslabs/soci-snapshotter/util/dockershell"
)

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
			tc.init(sh)
			// 2s timeout is arbitrary so that the negative test doesn't take too long. It does mean that
			// the snapshotter has to start up and prepare a snapshot within 2s in the socket activation case.
			output, err := sh.CombinedOLog("ctr", "--timeout", "2s", "snapshot", "--snapshotter", "soci", "prepare", "test")
			if !isExpectedError(output, err, tc.expectedErrorMatcher) {
				tt.Fatalf("unexpected error preparing snapshot: %v", output)
			}
		})
	}
}
