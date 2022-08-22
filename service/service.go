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

package service

import (
	"context"
	"path/filepath"

	socifs "github.com/awslabs/soci-snapshotter/fs"
	"github.com/awslabs/soci-snapshotter/fs/layer"
	"github.com/awslabs/soci-snapshotter/fs/source"
	"github.com/awslabs/soci-snapshotter/service/resolver"
	snbase "github.com/awslabs/soci-snapshotter/snapshot"
	"github.com/awslabs/soci-snapshotter/snapshot/overlayutils"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/snapshots"
	"github.com/hashicorp/go-multierror"
)

type Option func(*options)

type options struct {
	credsFuncs    []resolver.Credential
	registryHosts source.RegistryHosts
	fsOpts        []socifs.Option
}

// WithCredsFuncs specifies credsFuncs to be used for connecting to the registries.
func WithCredsFuncs(creds ...resolver.Credential) Option {
	return func(o *options) {
		o.credsFuncs = append(o.credsFuncs, creds...)
	}
}

// WithCustomRegistryHosts is registry hosts to use instead.
func WithCustomRegistryHosts(hosts source.RegistryHosts) Option {
	return func(o *options) {
		o.registryHosts = hosts
	}
}

// WithFilesystemOptions allowes to pass filesystem-related configuration.
func WithFilesystemOptions(opts ...socifs.Option) Option {
	return func(o *options) {
		o.fsOpts = opts
	}
}

// NewSociSnapshotterService returns soci snapshotter.
func NewSociSnapshotterService(ctx context.Context, root string, config *Config, opts ...Option) (snapshots.Snapshotter, error) {
	var sOpts options
	for _, o := range opts {
		o(&sOpts)
	}

	hosts := sOpts.registryHosts
	if hosts == nil {
		// Use RegistryHosts based on ResolverConfig and keychain
		hosts = resolver.RegistryHostsFromConfig(resolver.Config(config.ResolverConfig), sOpts.credsFuncs...)
	}
	userxattr, err := overlayutils.NeedsUserXAttr(snapshotterRoot(root))
	if err != nil {
		log.G(ctx).WithError(err).Warnf("cannot detect whether \"userxattr\" option needs to be used, assuming to be %v", userxattr)
	}
	opq := layer.OverlayOpaqueTrusted
	if userxattr {
		opq = layer.OverlayOpaqueUser
	}
	// Configure filesystem and snapshotter
	fsOpts := append(sOpts.fsOpts, socifs.WithGetSources(sources(
		sourceFromCRILabels(hosts),      // provides source info based on CRI labels
		source.FromDefaultLabels(hosts), // provides source info based on default labels
	)), socifs.WithOverlayOpaqueType(opq))
	fs, err := socifs.NewFilesystem(fsRoot(root), config.Config, fsOpts...)
	if err != nil {
		log.G(ctx).WithError(err).Fatalf("failed to configure filesystem")
	}

	var snapshotter snapshots.Snapshotter

	snapshotter, err = snbase.NewSnapshotter(ctx, snapshotterRoot(root), fs, snbase.AsynchronousRemove)
	if err != nil {
		log.G(ctx).WithError(err).Fatalf("failed to create new snapshotter")
	}

	return snapshotter, err
}

func snapshotterRoot(root string) string {
	return filepath.Join(root, "snapshotter")
}

func fsRoot(root string) string {
	return filepath.Join(root, "soci")
}

func sources(ps ...source.GetSources) source.GetSources {
	return func(labels map[string]string) (source []source.Source, allErr error) {
		for _, p := range ps {
			src, err := p(labels)
			if err == nil {
				return src, nil
			}
			allErr = multierror.Append(allErr, err)
		}
		return
	}
}

// Supported returns nil when the remote snapshotter is functional on the system with the root directory.
// Supported is not called during plugin initialization, but exposed for downstream projects which uses
// this snapshotter as a library.
func Supported(root string) error {
	// Remote snapshotter is implemented based on overlayfs snapshotter.
	return overlayutils.Supported(snapshotterRoot(root))
}
