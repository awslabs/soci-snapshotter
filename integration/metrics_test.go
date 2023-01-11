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
	"fmt"
	"strings"
	"testing"

	commonmetrics "github.com/awslabs/soci-snapshotter/fs/metrics/common"
	shell "github.com/awslabs/soci-snapshotter/util/dockershell"
	"github.com/awslabs/soci-snapshotter/util/testutil"
)

const (
	tcpMetricsAddress  = "localhost:1338"
	unixMetricsAddress = "/var/lib/soci-snapshotter-grpc/metrics.sock"
	metricsPath        = "/metrics"
)

const tcpMetricsConfig = `
metrics_address="` + tcpMetricsAddress + `"
`

const unixMetricsConfig = `
metrics_address="` + unixMetricsAddress + `"
metrics_network="unix"
`

func TestMetrics(t *testing.T) {
	tests := []struct {
		name    string
		config  string
		command []string
	}{
		{
			name:    "tcp",
			config:  tcpMetricsConfig,
			command: []string{"curl", "--fail", tcpMetricsAddress + metricsPath},
		},
		{
			name:    "unix",
			config:  unixMetricsConfig,
			command: []string{"curl", "--fail", "--unix-socket", unixMetricsAddress, "http://localhost" + metricsPath},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sh, done := newSnapshotterBaseShell(t)
			defer done()
			rebootContainerd(t, sh, "", tt.config)
			sh.X(tt.command...)
			if err := sh.Err(); err != nil {
				t.Fatal(err)
			}
		})
	}

}

func TestOverlayFallbackMetric(t *testing.T) {
	regConfig := newRegistryConfig()

	sh, done := newShellWithRegistry(t, regConfig)
	defer done()

	if err := testutil.WriteFileContents(sh, defaultContainerdConfigPath, getContainerdConfigYaml(t, false), 0600); err != nil {
		t.Fatalf("failed to write %v: %v", defaultContainerdConfigPath, err)
	}
	if err := testutil.WriteFileContents(sh, defaultSnapshotterConfigPath, getSnapshotterConfigYaml(t, false, tcpMetricsConfig), 0600); err != nil {
		t.Fatalf("failed to write %v: %v", defaultSnapshotterConfigPath, err)
	}

	testCases := []struct {
		name                  string
		image                 string
		indexDigestFn         func(*shell.Shell, imageInfo) string
		expectedFallbackCount int
	}{
		{
			name:  "image with all layers having ztocs and no fs.Mount error results in 0 overlay fallback",
			image: rabbitmqImage,
			indexDigestFn: func(sh *shell.Shell, image imageInfo) string {
				return buildSparseIndex(sh, image, 0, defaultSpanSize)
			},
			expectedFallbackCount: 0,
		},
		{
			name:  "image with some layers not having ztoc and no fs.Mount results in 0 overlay fallback",
			image: rabbitmqImage,
			indexDigestFn: func(sh *shell.Shell, image imageInfo) string {
				return buildSparseIndex(sh, image, defaultMinLayerSize, defaultSpanSize)
			},
			expectedFallbackCount: 0,
		},
		{
			name:  "image with fs.Mount errors results in non-zero overlay fallback",
			image: rabbitmqImage,
			indexDigestFn: func(_ *shell.Shell, _ imageInfo) string {
				return "dwadwadawdad"
			},
			expectedFallbackCount: 10,
		},
	}

	checkOverlayFallbackCount := func(output string, expected int) error {
		lines := strings.Split(output, "\n")
		for _, line := range lines {
			if !strings.Contains(line, commonmetrics.FuseMountFailureCount) {
				continue
			}
			var got int
			_, err := fmt.Sscanf(line, `soci_fs_operation_count{layer="",operation_type="fuse_mount_failure_count"} %d`, &got)
			if err != nil {
				return err
			}
			if got != expected {
				return fmt.Errorf("unexpected overlay fallbacks: got %d, expected %d", got, expected)
			}
			return nil
		}
		if expected != 0 {
			return fmt.Errorf("expected %d overlay fallbacks but got 0", expected)
		}
		return nil
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rebootContainerd(t, sh, "", "")

			imgInfo := dockerhub(tc.image)

			sh.X("ctr", "i", "pull", imgInfo.ref)
			indexDigest := tc.indexDigestFn(sh, imgInfo)

			sh.X("soci", "image", "rpull", "--soci-index-digest", indexDigest, imgInfo.ref)
			curlOutput := string(sh.O("curl", tcpMetricsAddress+metricsPath))

			if err := checkOverlayFallbackCount(curlOutput, tc.expectedFallbackCount); err != nil {
				t.Fatal(err)
			}
		})
	}
}
