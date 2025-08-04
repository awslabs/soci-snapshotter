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
	"github.com/awslabs/soci-snapshotter/soci/store"
	"github.com/awslabs/soci-snapshotter/util/dockershell"
	"github.com/awslabs/soci-snapshotter/util/testutil"
	"github.com/containerd/containerd/reference"
	"github.com/containerd/platforms"
)

var (
	pullModesImages = []string{nginxImage, rabbitmqImage, drupalImage, ubuntuImage}
)

func createAndPushV2Index(t *testing.T, sh *dockershell.Shell, srcInfo imageInfo, dstInfo imageInfo) string {
	sh.X("nerdctl", "pull", "-q", "--all-platforms", srcInfo.ref)
	sh.X("soci", "convert", "--all-platforms", "--min-layer-size", "0", srcInfo.ref, dstInfo.ref)
	sh.X("nerdctl", "push", "--all-platforms", dstInfo.ref)

	indexDigest, err := sh.OLog("soci",
		"index", "list",
		"-q", "--ref", dstInfo.ref,
		"--platform", platforms.Format(platforms.DefaultSpec()),
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(indexDigest) == 0 {
		t.Fatal("index digest is empty")
	}

	return strings.Trim(string(indexDigest), "\n")
}

func createAndPushV1Index(sh *dockershell.Shell, dstInfo imageInfo) string {
	sh.X("nerdctl", "pull", "-q", dstInfo.ref)
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
		name             string
		pullModes        config.PullModes
		contentStoreType store.ContentStoreType
		pullDigest       string
		expectedDigest   string
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
			name: "parallel pull unpack works",
			pullModes: config.PullModes{
				SOCIv1: config.V1{
					Enable: false,
				},
				SOCIv2: config.V2{
					Enable: false,
				},
				Parallel: config.Parallel{
					Enable: true,
				},
			},
			contentStoreType: store.ContainerdContentStoreType,
			expectedDigest:   "",
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
			opts := []snapshotterConfigOpt{withPullModes(test.pullModes)}
			// parallel pull/unpack requires containerd content store
			if test.contentStoreType == store.ContainerdContentStoreType {
				opts = append(opts, withContainerdContentStore())
			}
			m := rebootContainerd(t, sh, "", getSnapshotterConfigToml(t, opts...))
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
			args := imagePullCmd
			if test.pullDigest != "" {
				args = append(args, "--soci-index-digest", test.pullDigest)
			}
			args = append(args, dstInfo.ref)
			sh.X(args...)
			if test.expectedDigest != "" {
				rsm.CheckAllRemoteSnapshots(t)
			} else if test.pullModes.Parallel.Enable {
				rsm.CheckAllLocalSnapshots(t)
			} else {
				rsm.CheckAllDeferredSnapshots(t)
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
	idm := testutil.NewIndexDigestMonitor(m)
	defer idm.Close()
	sh.X(append(imagePullCmd, dstInfo.ref)...)
	rsm.CheckAllDeferredSnapshots(t)
	if idm.IndexDigest != "" {
		t.Fatalf("expected no digest, got %s", idm.IndexDigest)
	}
}

func TestDanglingV2Annotation(t *testing.T) {
	for _, imgName := range pullModesImages {
		t.Run(imgName, func(t *testing.T) {
			testDanglingV2Annotation(t, imgName)
		})
	}
}

func testDanglingV2Annotation(t *testing.T, imgName string) {
	regConfig := newRegistryConfig()
	sh, done := newShellWithRegistry(t, regConfig)
	defer done()

	rebootContainerd(t, sh, "", "")

	srcInfo := dockerhub(imgName)
	dstInfo := regConfig.mirror(imgName)

	sh.X("nerdctl", "pull", "-q", "--all-platforms", srcInfo.ref)
	sh.X("soci", "convert", "--all-platforms", "--min-layer-size", "0", srcInfo.ref, dstInfo.ref)

	manifest, err := getManifestDigest(sh, dstInfo.ref, platforms.DefaultSpec())
	if err != nil {
		t.Fatalf("could not get manifest digest: %v", err)
	}

	v2IndexDigest, err := sh.OLog("soci",
		"index", "list",
		"-q", "--ref", dstInfo.ref,
		"--platform", platforms.Format(platforms.DefaultSpec()),
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(v2IndexDigest) == 0 {
		t.Fatal("index digest is empty")
	}

	// Neither nerdctl nor ctr expose a way to "elevate" a platform-specific
	// manifest to a top level image directly, so we do a little registry dance:
	// push image:tag
	// pull image@sha256:... (the specific manifest we want to elevate)
	// tag image@sha256:... image/dangling:tag
	// push image/dangling:tag
	//
	// After this, image/dangling:tag refers to a single manifest that is platform specific.
	// We use this to separate an image manifest that contains a reference to a SOCI index
	// from the SOCI index itself to verify that it correctly pulls the image without lazy loading.
	platformManifestRef, err := reference.Parse(dstInfo.ref)
	if err != nil {
		t.Fatalf("could not get parse destination ref: %v", err)
	}
	danglingRef := platformManifestRef
	danglingRef.Locator += "/dangling" // image/dangling:tag

	platformManifestRef.Object = "@" + manifest // image@sha256...

	sh.X("nerdctl", "login", "--username", regConfig.user, "--password", regConfig.pass, platformManifestRef.String())
	sh.X("nerdctl", "push", "--platform", platforms.DefaultString(), dstInfo.ref)
	sh.X("nerdctl", "pull", "-q", platformManifestRef.String())
	sh.X("nerdctl", "image", "tag", platformManifestRef.String(), danglingRef.String())
	// Push a v1 index as well to verify that we do not fall back if we don't find the SOCI v2 index
	sh.X("nerdctl", "push", "--snapshotter", "soci", "--soci-min-layer-size", "0", danglingRef.String())

	m := rebootContainerd(t, sh, "", getSnapshotterConfigToml(t, withPullModes(config.DefaultPullModes())))

	rsm, doneRsm := testutil.NewRemoteSnapshotMonitor(m)
	defer doneRsm()
	var foundNoIndexMessage bool
	var indexDigestUsed string
	m.Add("Look for digest", func(s string) {
		if strings.Contains(s, "no valid SOCI index found") {
			foundNoIndexMessage = true
		}
		structuredLog := make(map[string]string)
		err := json.Unmarshal([]byte(s), &structuredLog)
		if err != nil {
			return
		}
		if structuredLog["msg"] == "fetching SOCI artifacts using index descriptor" {
			indexDigestUsed = structuredLog["digest"]
		}
	})
	sh.X(append(imagePullCmd, danglingRef.String())...)
	rsm.CheckAllDeferredSnapshots(t)
	if !foundNoIndexMessage {
		t.Fatalf("did not find the message that no index was found")
	}
	if strings.Trim(string(v2IndexDigest), "\n") != indexDigestUsed {
		t.Fatalf("expected v2 index digest %s, got %s", v2IndexDigest, indexDigestUsed)
	}
}
