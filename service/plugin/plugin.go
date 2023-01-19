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

package plugin

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/awslabs/soci-snapshotter/service"
	"github.com/awslabs/soci-snapshotter/service/keychain/cri"
	"github.com/awslabs/soci-snapshotter/service/keychain/dockerconfig"
	"github.com/awslabs/soci-snapshotter/service/keychain/kubeconfig"
	"github.com/awslabs/soci-snapshotter/service/resolver"
	"github.com/containerd/containerd/defaults"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/pkg/dialer"
	"github.com/containerd/containerd/platforms"
	ctdplugin "github.com/containerd/containerd/plugin"
	grpc "google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/credentials/insecure"
	runtime "k8s.io/cri-api/pkg/apis/runtime/v1alpha2"
)

// Config represents configuration for the soci snapshotter plugin.
type Config struct {
	service.Config

	// RootPath is the directory for the plugin
	RootPath string `toml:"root_path"`

	// CRIKeychainImageServicePath is the path to expose CRI service wrapped by CRI keychain
	CRIKeychainImageServicePath string `toml:"cri_keychain_image_service_path"`

	// Registry is CRI-plugin-compatible registry configuration
	Registry resolver.Registry `toml:"registry"`
}

func init() {
	ctdplugin.Register(&ctdplugin.Registration{
		Type:   ctdplugin.SnapshotPlugin,
		ID:     "soci",
		Config: &Config{},
		InitFn: func(ic *ctdplugin.InitContext) (interface{}, error) {
			ic.Meta.Platforms = append(ic.Meta.Platforms, platforms.DefaultSpec())
			ctx := ic.Context

			config, ok := ic.Config.(*Config)
			if !ok {
				return nil, errors.New("invalid soci snapshotter configuration")
			}

			root := ic.Root
			if config.RootPath != "" {
				root = config.RootPath
			}
			ic.Meta.Exports["root"] = root

			// Configure keychain
			credsFuncs := []resolver.Credential{dockerconfig.NewDockerConfigKeychain(ctx)}
			if config.Config.KubeconfigKeychainConfig.EnableKeychain {
				var opts []kubeconfig.Option
				if kcp := config.Config.KubeconfigKeychainConfig.KubeconfigPath; kcp != "" {
					opts = append(opts, kubeconfig.WithKubeconfigPath(kcp))
				}
				credsFuncs = append(credsFuncs, kubeconfig.NewKubeconfigKeychain(ctx, opts...))
			}
			if addr := config.CRIKeychainImageServicePath; config.Config.CRIKeychainConfig.EnableKeychain && addr != "" {
				// connects to the backend CRI service (defaults to containerd socket)
				criAddr := ic.Address
				if cp := config.Config.CRIKeychainConfig.ImageServicePath; cp != "" {
					criAddr = cp
				}
				if criAddr == "" {
					return nil, errors.New("backend CRI service address is not specified")
				}
				connectCRI := func() (runtime.ImageServiceClient, error) {
					// TODO: make gRPC options configurable from config.toml
					backoffConfig := backoff.DefaultConfig
					backoffConfig.MaxDelay = 3 * time.Second
					connParams := grpc.ConnectParams{
						Backoff: backoffConfig,
					}
					gopts := []grpc.DialOption{
						grpc.WithTransportCredentials(insecure.NewCredentials()),
						grpc.WithConnectParams(connParams),
						grpc.WithContextDialer(dialer.ContextDialer),
						grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(defaults.DefaultMaxRecvMsgSize)),
						grpc.WithDefaultCallOptions(grpc.MaxCallSendMsgSize(defaults.DefaultMaxSendMsgSize)),
					}
					conn, err := grpc.Dial(dialer.DialAddress(criAddr), gopts...)
					if err != nil {
						return nil, err
					}
					return runtime.NewImageServiceClient(conn), nil
				}
				criCreds, criServer := cri.NewCRIKeychain(ctx, connectCRI)
				// Create a gRPC server
				rpc := grpc.NewServer()
				runtime.RegisterImageServiceServer(rpc, criServer)
				// Prepare the directory for the socket
				if err := os.MkdirAll(filepath.Dir(addr), 0700); err != nil {
					return nil, fmt.Errorf("failed to create directory %q: %w", filepath.Dir(addr), err)
				}
				// Try to remove the socket file to avoid EADDRINUSE
				if err := os.RemoveAll(addr); err != nil {
					return nil, fmt.Errorf("failed to remove %q: %w", addr, err)
				}
				// Listen and serve
				l, err := net.Listen("unix", addr)
				if err != nil {
					return nil, fmt.Errorf("error on listen socket %q: %w", addr, err)
				}
				go func() {
					if err := rpc.Serve(l); err != nil {
						log.G(ctx).WithError(err).Warnf("error on serving via socket %q", addr)
					}
				}()
				credsFuncs = append(credsFuncs, criCreds)
			}

			// TODO(ktock): print warn if old configuration is specified.
			// TODO(ktock): should we respect old configuration?
			return service.NewSociSnapshotterService(ctx, root, &config.Config,
				service.WithCustomRegistryHosts(resolver.RegistryHostsFromCRIConfig(ctx, config.Registry, credsFuncs...)))
		},
	})
}
