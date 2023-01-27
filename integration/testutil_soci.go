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
	defaultSpanSize     = int64(1 << 22) // 4MiB
	defaultMinLayerSize = 10 << 20       // 10MiB
)

// indexBuildConfig represents the values of the CLI flags that should be used
// when creating an index with `buildIndex`
type indexBuildConfig struct {
	spanSize     int64
	minLayerSize int64
}

// indexBuildOption is a functional argument to update `indexBuildConfig`
type indexBuildOption func(*indexBuildConfig)

// withSpanSize overrides the default span size to use when creating an index with `buildIndex`
func withSpanSize(spanSize int64) indexBuildOption {
	return func(ibc *indexBuildConfig) {
		ibc.spanSize = spanSize
	}
}

// withMinLayerSize overrides the minimum layer size for which to create a ztoc
// when creating an index with `buildIndex`
func withMinLayerSize(minLayerSize int64) indexBuildOption {
	return func(ibc *indexBuildConfig) {
		ibc.minLayerSize = minLayerSize
	}
}

// defaultIndexBuildConfig is the default parameters when creating and index with `buildIndex`
func defaultIndexBuildConfig() indexBuildConfig {
	return indexBuildConfig{
		spanSize:     defaultSpanSize,
		minLayerSize: defaultMinLayerSize,
	}
}

// buildIndex builds an index for the source image with given options. By default, it will build with
// min-layer-size = 0 and span-size = CLI default
func buildIndex(sh *shell.Shell, src imageInfo, opt ...indexBuildOption) string {
	indexBuildConfig := defaultIndexBuildConfig()
	for _, o := range opt {
		o(&indexBuildConfig)
	}
	opts := encodeImageInfo(src)
	indexDigest := sh.
		X(append([]string{"ctr", "i", "pull", "--platform", platforms.Format(src.platform)}, opts[0]...)...).
		X("soci", "create", src.ref,
			"--min-layer-size", fmt.Sprintf("%d", indexBuildConfig.minLayerSize),
			"--span-size", fmt.Sprintf("%d", indexBuildConfig.spanSize),
			"--platform", platforms.Format(src.platform)).
		O("soci", "index", "list",
			"-q", "--ref", src.ref,
			"--platform", platforms.Format(src.platform)) // this will make SOCI artifact available locally
	return strings.Trim(string(indexDigest), "\n")
}

func getSociLocalStoreContentDigest(sh *shell.Shell) digest.Digest {
	content := sh.O("ls", blobStorePath)
	return digest.FromBytes(content)
}
