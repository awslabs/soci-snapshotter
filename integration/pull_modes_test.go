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
	"strings"
	"testing"

	"github.com/awslabs/soci-snapshotter/config"
	"github.com/awslabs/soci-snapshotter/util/dockershell"
	"github.com/awslabs/soci-snapshotter/util/testutil"
	"github.com/containerd/platforms"
)

var (
	pullModesImages = []string{nginxImage, rabbitmqImage, drupalImage, ubuntuImage}
)

func createAndPushV2Index(t *testing.T, sh *dockershell.Shell, srcInfo imageInfo, dstInfo imageInfo) string {
	sh.X("nerdctl", "pull", "--all-platforms", srcInfo.ref)
	sh.X("soci", "convert", "--min-layer-size", "0", srcInfo.ref, dstInfo.ref)
	sh.X("nerdctl", "push", "--all-platforms", dstInfo.ref)

	indexDigest, err := sh.OLog("soci",
		"index", "list",
		"-q", "--ref", srcInfo.ref,
		"--platform", platforms.Format(platforms.DefaultSpec()),
	)
	if err != nil {
		t.Fatal(err)
	}

	return strings.Trim(string(indexDigest), "\n")
}

func createAndPushV1Index(sh *dockershell.Shell, dstInfo imageInfo) string {
	sh.X("nerdctl", "pull", dstInfo.ref)
	indexDigest := buildIndex(sh, dstInfo, withMinLayerSize(0))
	sh.X("soci", "push", "--user", dstInfo.creds, dstInfo.ref)
	return indexDigest
}

func TestPullModes(t *testing.T) {
	for _, imgName := range pullModesImages {
		t.Run(imgName, func(t *testing.T) {
			testPullModes(t, imgName)
		})
	}
}

func testPullModes(t *testing.T, imgName string) {
	regConfig := newRegistryConfig()
	sh, done := newShellWithRegistry(t, regConfig)
	defer done()

	srcInfo := dockerhub(imgName)
	dstInfo := regConfig.mirror(imgName)
	sh.X("nerdctl", "login", "-u", regConfig.user, "-p", regConfig.pass, dstInfo.ref)

	// Convert the image and push to the registry,
	// then create a v1 index for the converted image.
	// This way, we can pull the same image with various pull modes
	// and verify which SOCI index (if any) gets used to verify that
	// the right pull modes are getting used.
	rebootContainerd(t, sh, "", "")
	v2IndexDigest := createAndPushV2Index(t, sh, srcInfo, dstInfo)
	rebootContainerd(t, sh, "", "")
	v1IndexDigest := createAndPushV1Index(sh, dstInfo)

	tests := []struct {
		name           string
		pullModes      config.PullModes
		pullDigest     string
		expectedDigest string
	}{
		{
			name: "lazy pulling doesn't happen if both v1 and v2 are disabled",
			pullModes: config.PullModes{
				SOCIv1: config.V1{
					Enable: false,
				},
				SOCIv2: config.V2{
					Enable: false,
				},
			},
			expectedDigest: "",
		},
		{
			name: "explicit v1 digest works with everything disabled",
			pullModes: config.PullModes{
				SOCIv1: config.V1{
					Enable: false,
				},
				SOCIv2: config.V2{
					Enable: false,
				},
			},
			pullDigest:     v1IndexDigest,
			expectedDigest: v1IndexDigest,
		},
		{
			name: "explicit v2 digest works with everything disabled",
			pullModes: config.PullModes{
				SOCIv1: config.V1{
					Enable: false,
				},
				SOCIv2: config.V2{
					Enable: false,
				},
			},
			pullDigest:     v2IndexDigest,
			expectedDigest: v2IndexDigest,
		},
		{
			name: "v1 works",
			pullModes: config.PullModes{
				SOCIv1: config.V1{
					Enable: true,
				},
				SOCIv2: config.V2{
					Enable: false,
				},
			},
			expectedDigest: v1IndexDigest,
		},
		{
			name: "v2 works",
			pullModes: config.PullModes{
				SOCIv1: config.V1{
					Enable: false,
				},
				SOCIv2: config.V2{
					Enable: true,
				},
			},
			expectedDigest: v2IndexDigest,
		},
		{
			name: "v2 is preferred over v1",
			pullModes: config.PullModes{
				SOCIv1: config.V1{
					Enable: true,
				},
				SOCIv2: config.V2{
					Enable: true,
				},
			},
			expectedDigest: v2IndexDigest,
		},
		{
			name:           "v2 is used by default",
			pullModes:      config.DefaultPullModes(),
			expectedDigest: v2IndexDigest,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			m := rebootContainerd(t, sh, "", getSnapshotterConfigToml(t, withPullModes(test.pullModes)))
			rsm, done := testutil.NewRemoteSnapshotMonitor(m)
			defer done()
			var indexDigestUsed string
			m.Add("Look for digest", func(s string) {
				structuredLog := make(map[string]string)
				err := json.Unmarshal([]byte(s), &structuredLog)
				if err != nil {
					return
				}
				if structuredLog["msg"] == "fetching SOCI artifacts using index descriptor" {
					indexDigestUsed = structuredLog["digest"]
				}
			})
			args := []string{"nerdctl", "pull", "--snapshotter", "soci"}
			if test.pullDigest != "" {
				args = append(args, "--soci-index-digest", test.pullDigest)
			}
			args = append(args, dstInfo.ref)
			sh.X(args...)
			if test.expectedDigest != "" {
				rsm.CheckAllRemoteSnapshots(t)
			} else {
				rsm.CheckAllLocalSnapshots(t)
			}
			if indexDigestUsed != test.expectedDigest {
				t.Fatalf("expected digest %s, got %s", test.expectedDigest, indexDigestUsed)
			}
		})
	}
}

func TestV1IsNotUsedWhenDisabled(t *testing.T) {
	for _, imgName := range pullModesImages {
		t.Run(imgName, func(t *testing.T) {
			testV1IsNotUsedWhenDisabled(t, imgName)
		})
	}
}

func testV1IsNotUsedWhenDisabled(t *testing.T, imgName string) {
	regConfig := newRegistryConfig()
	sh, done := newShellWithRegistry(t, regConfig)
	defer done()

	rebootContainerd(t, sh, "", "")

	srcInfo := dockerhub(imgName)
	dstInfo := regConfig.mirror(imgName)
	copyImage(sh, srcInfo, dstInfo)
	buildIndex(sh, dstInfo, withMinLayerSize(0))
	sh.X("soci", "push", "--user", dstInfo.creds, dstInfo.ref)

	m := rebootContainerd(t, sh, "", getSnapshotterConfigToml(t, withPullModes(config.PullModes{
		SOCIv1: config.V1{
			Enable: false,
		},
		SOCIv2: config.V2{
			Enable: true,
		},
	})))

	rsm, doneRsm := testutil.NewRemoteSnapshotMonitor(m)
	defer doneRsm()
	var indexDigestUsed string
	m.Add("Look for digest", func(s string) {
		structuredLog := make(map[string]string)
		err := json.Unmarshal([]byte(s), &structuredLog)
		if err != nil {
			return
		}
		if structuredLog["msg"] == "fetching SOCI artifacts using index descriptor" {
			indexDigestUsed = structuredLog["digest"]
		}
	})
	sh.X("nerdctl", "pull", "--snapshotter", "soci", dstInfo.ref)
	rsm.CheckAllLocalSnapshots(t)
	if indexDigestUsed != "" {
		t.Fatalf("expected no digest, got %s", indexDigestUsed)
	}
}
