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

/*
   Copyright 2019 The Go Authors. All rights reserved.
   Use of this source code is governed by a BSD-style
   license that can be found in the NOTICE.md file.
*/

package fs

import (
	"context"
	"fmt"
	"testing"

	"github.com/awslabs/soci-snapshotter/fs/layer"
	"github.com/awslabs/soci-snapshotter/fs/remote"
	"github.com/awslabs/soci-snapshotter/fs/source"
	"github.com/containerd/containerd/reference"
	"github.com/containerd/containerd/remotes/docker"
	fusefs "github.com/hanwen/go-fuse/v2/fs"
	digest "github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

func TestCheck(t *testing.T) {
	bl := &breakableLayer{}
	fs := &filesystem{
		layer: map[string]layer.Layer{
			"test": bl,
		},
		getSources: source.FromDefaultLabels(func(host string) (hosts []docker.RegistryHost, _ error) {
			return docker.ConfigureDefaultRegistries(docker.WithPlainHTTP(docker.MatchLocalhost))(host)
		}),
	}
	bl.success = true
	if err := fs.Check(context.TODO(), "test", nil); err != nil {
		t.Errorf("connection failed; wanted to succeed: %v", err)
	}

	bl.success = false
	if err := fs.Check(context.TODO(), "test", nil); err == nil {
		t.Errorf("connection succeeded; wanted to fail")
	}
}

type breakableLayer struct {
	success bool
}

func (l *breakableLayer) Info() layer.Info                                    { return layer.Info{} }
func (l *breakableLayer) RootNode(uint32) (fusefs.InodeEmbedder, error)       { return nil, nil }
func (l *breakableLayer) Verify(tocDigest digest.Digest) error                { return nil }
func (l *breakableLayer) SkipVerify()                                         {}
func (l *breakableLayer) ReadAt([]byte, int64, ...remote.Option) (int, error) { return 0, nil }
func (l *breakableLayer) BackgroundFetch() error                              { return fmt.Errorf("fail") }
func (l *breakableLayer) Check() error {
	if !l.success {
		return fmt.Errorf("failed")
	}
	return nil
}
func (l *breakableLayer) Refresh(ctx context.Context, hosts []docker.RegistryHost, refspec reference.Spec, desc ocispec.Descriptor) error {
	if !l.success {
		return fmt.Errorf("failed")
	}
	return nil
}
func (l *breakableLayer) Done() {}
