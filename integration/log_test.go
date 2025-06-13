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
	"regexp"
	"testing"
	"time"
)

const sensitiveInfoMonitorKey = "sensitive-info-monitor"

// TestSnapshotterDoesNotLogSensitiveInformation verifies that the snapshotter
// doesn't log sensitive information during various operations.
func TestSnapshotterDoesNotLogSensitiveInformation(t *testing.T) {
	image := rabbitmqImage

	testCases := []struct {
		name string
		opts []snapshotterConfigOpt
	}{
		{
			name: "basic image pull",
			opts: []snapshotterConfigOpt{},
		},
		{
			name: "default parallel image pull",
			opts: []snapshotterConfigOpt{withContainerdContentStore(), withParallelPullMode()},
		},
		{
			name: "unbounded parallel image pull",
			opts: []snapshotterConfigOpt{withContainerdContentStore(), withParallelPullMode(), withUnboundedPullUnpack()},
		},
		{
			name: "discard unpacked layers",
			opts: []snapshotterConfigOpt{withContainerdContentStore(), withDiscardUnpackedLayers()},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			regConfig := newRegistryConfig()
			sh, done := newShellWithRegistry(t, regConfig)
			t.Cleanup(func() {
				done()
			})

			logMonitor := rebootContainerd(t, sh, getContainerdConfigToml(t, false), getSnapshotterConfigToml(t, tc.opts...))
			logMonitor.Add(sensitiveInfoMonitorKey, sensitiveInfoMonitor(t, sensitivePatterns...))
			t.Cleanup(func() {
				logMonitor.Remove(sensitiveInfoMonitorKey)
			})

			copyImage(sh, dockerhub(image), regConfig.mirror(image))
			imageRef := regConfig.mirror(image).ref

			sh.X(append(imagePullCmd, imageRef)...)

			// Give some time for logs to be flushed
			time.Sleep(5 * time.Second)
		})
	}
}

// Patterns that should never appear in logs.
var sensitivePatterns = []*regexp.Regexp{
	// Authentication credentials
	regexp.MustCompile(`(?i)password`),
	regexp.MustCompile(`(?i)passwd`),
	regexp.MustCompile(`(?i)secret`),
	regexp.MustCompile(`(?i)credential`),
	regexp.MustCompile(`(?i)token`),
	regexp.MustCompile(`(?i)api[_-]?key`),

	// AWS specific
	regexp.MustCompile(`AKIA[0-9A-Z]{16}`), // AWS Access Key ID
	regexp.MustCompile(`(?i)aws[_-]?access[_-]?key`),
	regexp.MustCompile(`(?i)aws[_-]?secret[_-]?key`),

	// Private keys and certificates
	regexp.MustCompile(`-----BEGIN (?:RSA )?PRIVATE KEY-----`),
	regexp.MustCompile(`-----BEGIN CERTIFICATE-----`),

	// OAuth tokens
	regexp.MustCompile(`(?i)oauth`),
	regexp.MustCompile(`(?i)bearer`),

	// Common sensitive environment variables
	regexp.MustCompile(`(?i)basic auth`),
	regexp.MustCompile(`(?i)authorization: basic`),
}

func sensitiveInfoMonitor(t *testing.T, patterns ...*regexp.Regexp) func(string) {
	return func(log string) {
		for _, pattern := range patterns {
			if pattern.MatchString(log) {
				t.Errorf("Found sensitive information in log: %s", log)
			}
		}
	}
}
