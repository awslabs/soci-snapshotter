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

package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	golog "log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	_ "net/http/pprof"

	"github.com/awslabs/soci-snapshotter/config"
	"github.com/awslabs/soci-snapshotter/fs"
	"github.com/awslabs/soci-snapshotter/metadata"
	"github.com/awslabs/soci-snapshotter/service"
	"github.com/awslabs/soci-snapshotter/service/keychain/cri"
	"github.com/awslabs/soci-snapshotter/service/keychain/dockerconfig"
	"github.com/awslabs/soci-snapshotter/service/keychain/kubeconfig"
	"github.com/awslabs/soci-snapshotter/service/resolver"
	"github.com/awslabs/soci-snapshotter/version"
	"github.com/awslabs/soci-snapshotter/ztoc"
	snapshotsapi "github.com/containerd/containerd/api/services/snapshots/v1"
	"github.com/containerd/containerd/contrib/snapshotservice"
	"github.com/containerd/containerd/defaults"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/pkg/dialer"
	"github.com/containerd/containerd/snapshots"
	runtime_alpha "github.com/containerd/containerd/third_party/k8s.io/cri-api/pkg/apis/runtime/v1alpha2"
	sddaemon "github.com/coreos/go-systemd/v22/daemon"
	metrics "github.com/docker/go-metrics"
	"github.com/sirupsen/logrus"
	bolt "go.etcd.io/bbolt"
	"golang.org/x/sys/unix"
	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	defaultAddress    = "/run/soci-snapshotter-grpc/soci-snapshotter-grpc.sock"
	defaultConfigPath = "/etc/soci-snapshotter-grpc/config.toml"
	defaultLogLevel   = logrus.InfoLevel
	defaultRootDir    = "/var/lib/soci-snapshotter-grpc"
)

// logLevel of Debug or Trace may emit sensitive information
// e.g. file contents, file names and paths, network addresses and ports
var (
	address      = flag.String("address", defaultAddress, "address for the snapshotter's GRPC server")
	configPath   = flag.String("config", defaultConfigPath, "path to the configuration file")
	logLevel     = flag.String("log-level", defaultLogLevel.String(), "set the logging level [trace, debug, info, warn, error, fatal, panic]")
	rootDir      = flag.String("root", defaultRootDir, "path to the root directory for this snapshotter")
	printVersion = flag.Bool("version", false, "print the version")
)

func main() {
	rand.Seed(time.Now().UnixNano())
	flag.Parse()
	lvl, err := logrus.ParseLevel(*logLevel)
	if err != nil {
		log.L.WithError(err).Fatal("failed to prepare logger")
	}
	if *printVersion {
		fmt.Println("soci-snapshotter-grpc version", version.Version, version.Revision)
		return
	}
	logrus.SetLevel(lvl)
	logrus.SetFormatter(&logrus.JSONFormatter{
		TimestampFormat: log.RFC3339NanoFixed,
	})

	ctx, cancel := context.WithCancel(log.WithLogger(context.Background(), log.L))
	defer cancel()
	// Streams log of standard lib (go-fuse uses this) into debug log
	// Snapshotter should use "github.com/containerd/containerd/log" otherwise
	// logs are always printed as "debug" mode.
	golog.SetOutput(log.G(ctx).WriterLevel(logrus.DebugLevel))
	log.G(ctx).WithFields(logrus.Fields{
		"version":  version.Version,
		"revision": version.Revision,
	}).Info("starting soci-snapshotter-grpc")

	cfg, err := config.NewConfigFromToml(*configPath)
	if err != nil {
		log.G(ctx).WithError(err).Fatal(err)
	}

	if err := service.Supported(*rootDir); err != nil {
		log.G(ctx).WithError(err).Fatalf("snapshotter is not supported")
	}

	// Create a gRPC server
	rpc := grpc.NewServer()

	// Configure keychain
	credsFuncs := []resolver.Credential{dockerconfig.NewDockerConfigKeychain(ctx)}
	if cfg.KubeconfigKeychainConfig.EnableKeychain {
		var opts []kubeconfig.Option
		if kcp := cfg.KubeconfigKeychainConfig.KubeconfigPath; kcp != "" {
			opts = append(opts, kubeconfig.WithKubeconfigPath(kcp))
		}
		credsFuncs = append(credsFuncs, kubeconfig.NewKubeconfigKeychain(ctx, opts...))
	}
	if cfg.CRIKeychainConfig.EnableKeychain {
		// connects to the backend CRI service (defaults to containerd socket)
		connectCRI := func() (runtime_alpha.ImageServiceClient, error) {
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
			conn, err := grpc.Dial(dialer.DialAddress(cfg.CRIKeychainConfig.ImageServicePath), gopts...)
			if err != nil {
				return nil, err
			}
			return runtime_alpha.NewImageServiceClient(conn), nil
		}
		f, criServer := cri.NewCRIKeychain(ctx, connectCRI)
		runtime_alpha.RegisterImageServiceServer(rpc, criServer)
		credsFuncs = append(credsFuncs, f)
	}
	var fsOpts []fs.Option
	mt, err := getMetadataStore(*rootDir, *cfg)
	if err != nil {
		log.G(ctx).WithError(err).Fatalf("failed to configure metadata store")
	}
	fsOpts = append(fsOpts, fs.WithMetadataStore(mt))
	rs, err := service.NewSociSnapshotterService(ctx, *rootDir, &cfg.ServiceConfig,
		service.WithCredsFuncs(credsFuncs...), service.WithFilesystemOptions(fsOpts...))
	if err != nil {
		log.G(ctx).WithError(err).Fatalf("failed to configure snapshotter")
	}

	cleanup, err := serve(ctx, rpc, *address, rs, *cfg)
	if err != nil {
		log.G(ctx).WithError(err).Fatalf("failed to serve snapshotter")
	}

	if cleanup {
		log.G(ctx).Debug("Closing the snapshotter")
		rs.Close()
	}
	log.G(ctx).Info("Exiting")
}

func serve(ctx context.Context, rpc *grpc.Server, addr string, rs snapshots.Snapshotter, cfg config.Config) (bool, error) {
	// Convert the snapshotter to a gRPC service,
	snsvc := snapshotservice.FromSnapshotter(rs)

	// Register the service with the gRPC server
	snapshotsapi.RegisterSnapshotsServer(rpc, snsvc)

	// Prepare the directory for the socket
	if err := os.MkdirAll(filepath.Dir(addr), 0700); err != nil {
		return false, fmt.Errorf("failed to create directory %q: %w", filepath.Dir(addr), err)
	}

	// Try to remove the socket file to avoid EADDRINUSE
	if err := os.RemoveAll(addr); err != nil {
		return false, fmt.Errorf("failed to remove %q: %w", addr, err)
	}

	errCh := make(chan error, 1)

	var cleanupFns []func() error
	defer func() {
		for _, cleanupFn := range cleanupFns {
			cleanupFn()
		}
	}()

	// We need to consider both the existence of MetricsAddress as well as NoPrometheus flag not set
	if cfg.MetricsAddress != "" && !cfg.NoPrometheus {
		l, err := net.Listen(cfg.MetricsNetwork, cfg.MetricsAddress)
		if err != nil {
			return false, fmt.Errorf("failed to get listener for metrics endpoint: %w", err)
		}
		cleanupFns = append(cleanupFns, l.Close)
		m := http.NewServeMux()
		m.Handle("/metrics", metrics.Handler())
		go func() {
			if err := http.Serve(l, m); err != nil {
				errCh <- fmt.Errorf("error on serving metrics via socket %q: %w", addr, err)
			}
		}()
	}

	if cfg.DebugAddress != "" {
		log.G(ctx).Infof("listen %q for debugging", cfg.DebugAddress)
		go func() {
			if err := http.ListenAndServe(cfg.DebugAddress, nil); err != nil {
				errCh <- fmt.Errorf("error on serving a debug endpoint via socket %q: %w", addr, err)
			}
		}()
	}

	// Listen and serve
	l, err := net.Listen("unix", addr)
	if err != nil {
		return false, fmt.Errorf("error on listen socket %q: %w", addr, err)
	}
	cleanupFns = append(cleanupFns, l.Close)
	go func() {
		if err := rpc.Serve(l); err != nil {
			errCh <- fmt.Errorf("error on serving via socket %q: %w", addr, err)
		}
	}()

	if os.Getenv("NOTIFY_SOCKET") != "" {
		notified, notifyErr := sddaemon.SdNotify(false, sddaemon.SdNotifyReady)
		log.G(ctx).Debugf("SdNotifyReady notified=%v, err=%v", notified, notifyErr)
	}
	defer func() {
		if os.Getenv("NOTIFY_SOCKET") != "" {
			notified, notifyErr := sddaemon.SdNotify(false, sddaemon.SdNotifyStopping)
			log.G(ctx).Debugf("SdNotifyStopping notified=%v, err=%v", notified, notifyErr)
		}
	}()

	var s os.Signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, unix.SIGINT, unix.SIGTERM)
	select {
	case s = <-sigCh:
		log.G(ctx).Infof("Got %v", s)
	case err := <-errCh:
		return false, err
	}
	if s == unix.SIGINT {
		return true, nil // do cleanup on SIGINT
	}
	return false, nil
}

const (
	dbMetadataType = "db"
)

func getMetadataStore(rootDir string, config config.Config) (metadata.Store, error) {
	switch config.MetadataStore {
	case "", dbMetadataType:
		bOpts := bolt.Options{
			NoFreelistSync:  true,
			InitialMmapSize: 64 * 1024 * 1024,
			FreelistType:    bolt.FreelistMapType,
		}
		db, err := bolt.Open(filepath.Join(rootDir, "metadata.db"), 0600, &bOpts)
		if err != nil {
			return nil, err
		}
		return func(sr *io.SectionReader, toc ztoc.TOC, opts ...metadata.Option) (metadata.Reader, error) {
			return metadata.NewReader(db, sr, toc, opts...)
		}, nil
	default:
		return nil, fmt.Errorf("unknown metadata store type: %v; must be %v",
			config.MetadataStore, dbMetadataType)
	}
}
