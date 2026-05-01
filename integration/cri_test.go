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
	"testing"

	"github.com/awslabs/soci-snapshotter/util/testutil"
)

// TestCRIImagePull pulls a SOCI-converted image through containerd's CRI image endpoint
// and asserts that the layers are mounted as remote SOCI snapshots.
func TestCRIImagePull(t *testing.T) {
	regConfig := newRegistryConfig()
	sh, done := newShellWithRegistry(t, regConfig)
	defer done()

	image := alpineImage

	rebootContainerd(t, sh, getContainerdConfigToml(t, false), getSnapshotterConfigToml(t))
	copyImage(sh, dockerhub(image), regConfig.mirror(image))
	buildIndex(sh, regConfig.mirror(image), withMinLayerSize(0))
	sh.X("soci", "push", "--user", regConfig.creds(), regConfig.mirror(image).ref)

	rsm := testutil.NewRemoteSnapshotMonitor()
	m := rebootContainerd(t, sh, getCRIContainerdConfigToml(t, false), getSnapshotterConfigToml(t, withCRIKeychain), rsm.MonitorFunc)
	defer m.Cleanup(t)

	sh.X(append(crictlCmd, "pull", "--creds", regConfig.creds(), regConfig.mirror(image).ref)...)

	rsm.CheckAllRemoteSnapshots(t)
}
