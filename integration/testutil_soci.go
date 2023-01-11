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

/*
   Copyright The containerd Authors.

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

	shell "github.com/awslabs/soci-snapshotter/util/dockershell"
	"github.com/containerd/containerd/platforms"
	"github.com/opencontainers/go-digest"
)

const (
	// copied from `soci/soci_index.go` for convenience so we don't always need to
	// import the `soci` pkg only to use the default values.
	// default span size (4MiB)
	defaultSpanSize = int64(1 << 22)
	// min layer size (10MiB)
	defaultMinLayerSize = 10 << 20
)

// buildIndex builds a soci index with default span size and 0 min-layer-size, so every layer
// will have a corresponding ztoc.
func buildIndex(sh *shell.Shell, src imageInfo) string {
	return buildSparseIndex(sh, src, 0, defaultSpanSize) // we build an index with min-layer-size 0
}

// buildSparseIndex builds a soci index by passing `--min-layer-size` and `--span-size`.
func buildSparseIndex(sh *shell.Shell, src imageInfo, minLayerSize, spanSize int64) string {
	opts := encodeImageInfo(src)
	indexDigest := sh.
		X(append([]string{"ctr", "i", "pull", "--platform", platforms.Format(src.platform)}, opts[0]...)...).
		X("soci", "create", src.ref, "--min-layer-size", fmt.Sprintf("%d", minLayerSize), "--span-size", fmt.Sprintf("%d", spanSize), "--platform", platforms.Format(src.platform)).
		O("soci", "index", "list", "-q", "--ref", src.ref, "--platform", platforms.Format(src.platform)) // this will make SOCI artifact available locally
	return strings.Trim(string(indexDigest), "\n")
}

func getSociLocalStoreContentDigest(sh *shell.Shell) digest.Digest {
	content := sh.O("ls", blobStorePath)
	return digest.FromBytes(content)
}
