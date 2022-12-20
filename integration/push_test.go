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

	shell "github.com/awslabs/soci-snapshotter/util/dockershell"
	"github.com/awslabs/soci-snapshotter/util/testutil"
	"github.com/containerd/containerd/platforms"
	"github.com/opencontainers/go-digest"
)

func TestSociArtifactsPushAndPull(t *testing.T) {
	regConfig := newRegistryConfig()
	sh, done := newShellWithRegistry(t, regConfig)
	defer done()

	getContainerdConfigYaml := func(disableVerification bool) []byte {
		additionalConfig := ""
		if !isTestingBuiltinSnapshotter() {
			additionalConfig = proxySnapshotterConfig
		}
		return []byte(testutil.ApplyTextTemplate(t, `
version = 2

[plugins."io.containerd.snapshotter.v1.soci"]
root_path = "/var/lib/soci-snapshotter-grpc/"
disable_verification = {{.DisableVerification}}

[plugins."io.containerd.snapshotter.v1.soci".blob]
check_always = true

[debug]
format = "json"
level = "debug"

{{.AdditionalConfig}}
`, struct {
			DisableVerification bool
			AdditionalConfig    string
		}{
			DisableVerification: disableVerification,
			AdditionalConfig:    additionalConfig,
		}))
	}
	getSnapshotterConfigYaml := func(disableVerification bool) []byte {
		return []byte(fmt.Sprintf("disable_verification = %v", disableVerification))
	}

	if err := testutil.WriteFileContents(sh, defaultContainerdConfigPath, getContainerdConfigYaml(false), 0600); err != nil {
		t.Fatalf("failed to write %v: %v", defaultContainerdConfigPath, err)
	}
	if err := testutil.WriteFileContents(sh, defaultSnapshotterConfigPath, getSnapshotterConfigYaml(false), 0600); err != nil {
		t.Fatalf("failed to write %v: %v", defaultSnapshotterConfigPath, err)
	}

	tests := []struct {
		Name     string
		Platform string
	}{
		{
			Name:     "amd64",
			Platform: "linux/amd64",
		},
		{
			Name:     "arm64",
			Platform: "linux/arm64",
		},
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			rebootContainerd(t, sh, "", "")

			platform, err := platforms.Parse(tt.Platform)
			if err != nil {
				t.Fatalf("could not parse platform %s: %v", tt.Platform, err)
			}

			imageName := ubuntuImage
			copyImage(sh, dockerhub(imageName, withPlatform(platform)), regConfig.mirror(imageName, withPlatform(platform)))
			indexDigest := optimizeImage(sh, regConfig.mirror(imageName, withPlatform(platform)))
			artifactsStoreContentDigest := getSociLocalStoreContentDigest(sh)
			sh.X("soci", "push", "--user", regConfig.creds(), "--platform", tt.Platform, regConfig.mirror(imageName).ref)
			sh.X("rm", "-rf", "/var/lib/soci-snapshotter-grpc/content/blobs/sha256")

			sh.X("soci", "image", "rpull", "--user", regConfig.creds(), "--soci-index-digest", indexDigest, "--platform", tt.Platform, regConfig.mirror(imageName).ref)
			artifactsStoreContentDigestAfterRPull := getSociLocalStoreContentDigest(sh)

			if artifactsStoreContentDigest != artifactsStoreContentDigestAfterRPull {
				t.Fatalf("unexpected digests before and after rpull; before = %v, after = %v", artifactsStoreContentDigest, artifactsStoreContentDigestAfterRPull)
			}
		})
	}
}

func TestPushAlwaysMostRecentlyCreatedIndex(t *testing.T) {
	regConfig := newRegistryConfig()
	sh, done := newShellWithRegistry(t, regConfig)
	defer done()

	getContainerdConfigYaml := func(disableVerification bool) []byte {
		additionalConfig := ""
		if !isTestingBuiltinSnapshotter() {
			additionalConfig = proxySnapshotterConfig
		}
		return []byte(testutil.ApplyTextTemplate(t, `
version = 2

[plugins."io.containerd.snapshotter.v1.soci"]
root_path = "/var/lib/soci-snapshotter-grpc/"
disable_verification = {{.DisableVerification}}

[plugins."io.containerd.snapshotter.v1.soci".blob]
check_always = true

[debug]
format = "json"
level = "debug"

{{.AdditionalConfig}}
`, struct {
			DisableVerification bool
			AdditionalConfig    string
		}{
			DisableVerification: disableVerification,
			AdditionalConfig:    additionalConfig,
		}))
	}
	getSnapshotterConfigYaml := func(disableVerification bool) []byte {
		return []byte(fmt.Sprintf("disable_verification = %v", disableVerification))
	}

	if err := testutil.WriteFileContents(sh, defaultContainerdConfigPath, getContainerdConfigYaml(false), 0600); err != nil {
		t.Fatalf("failed to write %v: %v", defaultContainerdConfigPath, err)
	}
	if err := testutil.WriteFileContents(sh, defaultSnapshotterConfigPath, getSnapshotterConfigYaml(false), 0600); err != nil {
		t.Fatalf("failed to write %v: %v", defaultSnapshotterConfigPath, err)
	}

	type buildOpts struct {
		spanSize     int
		minLayerSize int
	}

	testCases := []struct {
		name  string
		image string
		opts  []buildOpts
	}{
		{
			name: "rabbitmq",
			// Pinning a specific image, so that this test is guaranteed to fail in case of any regressions.
			image: "rabbitmq@sha256:603be6b7fd5f1d8c6eab8e7a234ed30d664b9356ec1b87833f3a46bb6725458e",
			opts: []buildOpts{
				{
					spanSize:     1 << 22,  // 4MiB
					minLayerSize: 10 << 20, // 10MiB
				},
				{
					spanSize:     128000,
					minLayerSize: 10 << 20,
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rebootContainerd(t, sh, "", "")

			copyImage(sh, dockerhub(tc.image), regConfig.mirror(tc.image))

			for _, opt := range tc.opts {
				index := optimizeImageWithOpts(sh, regConfig.mirror(tc.image), opt.spanSize, opt.minLayerSize)
				index = strings.Split(index, "\n")[0]
				out := sh.O("soci", "push", "--user", regConfig.creds(), regConfig.mirror(tc.image).ref, "-q")
				pushedIndex := strings.Trim(string(out), "\n")

				if index != pushedIndex {
					t.Fatalf("incorrect index pushed to remote registry; expected %s, got %s", index, pushedIndex)
				}
			}
		})
	}
}

// TODO: There is a bit of duplication going on here.
// The `optimizeImage` function does not allow passing a specific span size.
// Should refactor that function to do so and remove this function.
func optimizeImageWithOpts(sh *shell.Shell, src imageInfo, spanSize, minLayerSize int) string {
	pullOpts := encodeImageInfo(src)
	indexDigest := sh.
		X(append([]string{"ctr", "i", "pull", "--platform", platforms.Format(src.platform)}, pullOpts[0]...)...).
		X("soci", "create", src.ref, "--min-layer-size", fmt.Sprintf("%d", minLayerSize), "--span-size", fmt.Sprintf("%d", spanSize), "--platform", platforms.Format(src.platform)).
		O("soci", "index", "list", "-q", "--ref", src.ref, "--platform", platforms.Format(src.platform)) // this will make SOCI artifact available locally
	return strings.Trim(string(indexDigest), "\n")
}

func getSociLocalStoreContentDigest(sh *shell.Shell) digest.Digest {
	content := sh.O("ls", "/var/lib/soci-snapshotter-grpc/content/blobs/sha256")
	return digest.FromBytes(content)
}
