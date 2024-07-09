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

	"github.com/awslabs/soci-snapshotter/config"
	"github.com/awslabs/soci-snapshotter/service"
	"github.com/awslabs/soci-snapshotter/service/keychain/cri/v1"
	crialpha "github.com/awslabs/soci-snapshotter/service/keychain/cri/v1alpha"
	"github.com/awslabs/soci-snapshotter/service/keychain/dockerconfig"
	"github.com/awslabs/soci-snapshotter/service/keychain/kubeconfig"
	"github.com/awslabs/soci-snapshotter/service/resolver"
	"github.com/containerd/containerd/defaults"
	"github.com/containerd/containerd/pkg/dialer"
	ctdplugin "github.com/containerd/containerd/plugin"
	runtime_alpha "github.com/containerd/containerd/third_party/k8s.io/cri-api/pkg/apis/runtime/v1alpha2"
	"github.com/containerd/log"
	"github.com/containerd/platforms"
	grpc "google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/credentials/insecure"
	runtime "k8s.io/cri-api/pkg/apis/runtime/v1"
)

// Config represents configuration for the soci snapshotter plugin.
type Config struct {
	config.ServiceConfig

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
			if config.KubeconfigKeychainConfig.EnableKeychain {
				var opts []kubeconfig.Option
				if kcp := config.KubeconfigKeychainConfig.KubeconfigPath; kcp != "" {
					opts = append(opts, kubeconfig.WithKubeconfigPath(kcp))
				}
				credsFuncs = append(credsFuncs, kubeconfig.NewKubeconfigKeychain(ctx, opts...))
			}
			if addr := config.CRIKeychainImageServicePath; config.CRIKeychainConfig.EnableKeychain && addr != "" {
				// connects to the backend CRI service (defaults to containerd socket)
				criAddr := ic.Address
				if cp := config.CRIKeychainConfig.ImageServicePath; cp != "" {
					criAddr = cp
				}
				if criAddr == "" {
					return nil, errors.New("backend CRI service address is not specified")
				}
				// Create a gRPC server
				rpc := grpc.NewServer()

				connectV1AlphaCRI := func() (runtime_alpha.ImageServiceClient, error) {
					criConn, err := getCriConn(config.CRIKeychainConfig.ImageServicePath)
					if err != nil {
						return nil, err
					}
					return runtime_alpha.NewImageServiceClient(criConn), nil
				}

				connectV1CRI := func() (runtime.ImageServiceClient, error) {
					criConn, err := getCriConn(config.CRIKeychainConfig.ImageServicePath)
					if err != nil {
						return nil, err
					}
					return runtime.NewImageServiceClient(criConn), nil
				}

				// register v1alpha2 CRI server with the gRPC server
				fAlpha, criServerAlpha := crialpha.NewCRIAlphaKeychain(ctx, connectV1AlphaCRI)
				runtime_alpha.RegisterImageServiceServer(rpc, criServerAlpha)
				credsFuncs = append(credsFuncs, fAlpha)

				// register v1 CRI server with the gRPC server
				f, criServer := cri.NewCRIKeychain(ctx, connectV1CRI)
				runtime.RegisterImageServiceServer(rpc, criServer)
				credsFuncs = append(credsFuncs, f)

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
			}

			// TODO(ktock): print warn if old configuration is specified.
			// TODO(ktock): should we respect old configuration?
			return service.NewSociSnapshotterService(ctx, root, &config.ServiceConfig,
				service.WithCustomRegistryHosts(resolver.RegistryHostsFromCRIConfig(ctx, config.Registry, credsFuncs...)))
		},
	})
}

// getCriConn gets the gRPC client connection to the backend CRI service (defaults to containerd socket).
func getCriConn(criAddr string) (*grpc.ClientConn, error) {
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
	return grpc.Dial(dialer.DialAddress(criAddr), gopts...)
}
