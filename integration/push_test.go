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
func getSociLocalStoreContentDigest(sh *shell.Shell) digest.Digest {
	content := sh.O("ls", "/var/lib/soci-snapshotter-grpc/content/blobs/sha256")
	return digest.FromBytes(content)
}
