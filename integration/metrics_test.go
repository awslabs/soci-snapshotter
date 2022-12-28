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

import "testing"

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
