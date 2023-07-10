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
	"io"
	golog "log"
	"math/rand"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	socifs "github.com/awslabs/soci-snapshotter/fs"
	"github.com/awslabs/soci-snapshotter/fs/config"
	"github.com/awslabs/soci-snapshotter/fs/layer"
	"github.com/awslabs/soci-snapshotter/fs/source"
	"github.com/awslabs/soci-snapshotter/metadata"
	"github.com/awslabs/soci-snapshotter/service/keychain/dockerconfig"
	"github.com/awslabs/soci-snapshotter/service/keychain/kubeconfig"
	"github.com/awslabs/soci-snapshotter/service/keychain/local_keychain"
	"github.com/awslabs/soci-snapshotter/service/resolver"
	"github.com/awslabs/soci-snapshotter/store"
	"github.com/awslabs/soci-snapshotter/ztoc"
	"github.com/containerd/containerd/log"
	"github.com/pelletier/go-toml"

	sddaemon "github.com/coreos/go-systemd/v22/daemon"
	"github.com/hashicorp/go-multierror"
	"github.com/sirupsen/logrus"
	bolt "go.etcd.io/bbolt"
)

const (
	defaultLogLevel   = logrus.InfoLevel
	defaultConfigPath = "/etc/soci-store/config.toml"
	defaultRootDir    = "/var/lib/soci-store"
)

var (
	configPath = flag.String("config", defaultConfigPath, "path to the configuration file")
	logLevel   = flag.String("log-level", defaultLogLevel.String(), "set the logging level [trace, debug, info, warn, error, fatal, panic]")
	rootDir    = flag.String("root", defaultRootDir, "path to the root directory for this snapshotter")
)

type Config struct {
	config.Config

	// KubeconfigKeychainConfig is config for kubeconfig-based keychain.
	KubeconfigKeychainConfig `toml:"kubeconfig_keychain"`

	// ResolverConfig is config for resolving registries.
	ResolverConfig `toml:"resolver"`
}

type KubeconfigKeychainConfig struct {
	EnableKeychain bool   `toml:"enable_keychain"`
	KubeconfigPath string `toml:"kubeconfig_path"`
}

type ResolverConfig resolver.Config

func main() {
	rand.Seed(time.Now().UnixNano())
	flag.Parse()
	mountPoint := flag.Arg(0)
	lvl, err := logrus.ParseLevel(*logLevel)
	if err != nil {
		log.L.WithError(err).Fatal("failed to prepare logger")
	}
	logrus.SetLevel(lvl)
	logrus.SetFormatter(&logrus.JSONFormatter{
		TimestampFormat: log.RFC3339NanoFixed,
	})
	var (
		ctx    = log.WithLogger(context.Background(), log.L)
		config Config
	)
	// Streams log of standard lib (go-fuse uses this) into debug log
	// Snapshotter should use "github.com/containerd/containerd/log" otherwise
	// logs are always printed as "debug" mode.
	golog.SetOutput(log.G(ctx).WriterLevel(logrus.DebugLevel))

	if mountPoint == "" {
		log.G(ctx).Fatalf("mount point must be specified")
	}

	// Get configuration from specified file
	if *configPath != "" {
		tree, err := toml.LoadFile(*configPath)
		if err != nil && !(os.IsNotExist(err) && *configPath == defaultConfigPath) {
			log.G(ctx).WithError(err).Fatalf("failed to load config file %q", *configPath)
		}
		if err := tree.Unmarshal(&config); err != nil {
			log.G(ctx).WithError(err).Fatalf("failed to unmarshal config file %q", *configPath)
		}
	}

	// Prepare kubeconfig-based keychain if required
	credsFuncs := []resolver.Credential{local_keychain.Keychain(ctx).GetCredentials}
	credsFuncs = append(credsFuncs, dockerconfig.NewDockerConfigKeychain(ctx))
	if config.KubeconfigKeychainConfig.EnableKeychain {
		var opts []kubeconfig.Option
		if kcp := config.KubeconfigKeychainConfig.KubeconfigPath; kcp != "" {
			opts = append(opts, kubeconfig.WithKubeconfigPath(kcp))
		}
		credsFuncs = append(credsFuncs, kubeconfig.NewKubeconfigKeychain(ctx, opts...))
	}

	// Use RegistryHosts based on ResolverConfig and keychain
	hosts := resolver.RegistryHostsFromConfig(resolver.Config(config.ResolverConfig), credsFuncs...)

	// Configure and mount filesystem
	if _, err := os.Stat(mountPoint); err != nil {
		if err2 := os.MkdirAll(mountPoint, 0755); err2 != nil && !os.IsExist(err2) {
			log.G(ctx).WithError(err).WithError(err2).
				Fatalf("failed to prepare mountpoint %q", mountPoint)
		}
	}
	if !config.Config.DisableVerification {
		log.G(ctx).Warnf("content verification is not supported; switching to non-verification mode")
		config.Config.DisableVerification = true
	}
	mt, err := getMetadataStore(*rootDir, config)
	if err != nil {
		log.G(ctx).WithError(err).Fatalf("failed to configure metadata store")
	}

	// Configure filesystem and snapshotter
	var fsOpts []socifs.Option
	opq := layer.OverlayOpaqueTrusted
	fsOpts = append(fsOpts, socifs.WithGetSources(
		source.FromDefaultLabels(hosts),    // provides source info based on default labels
	), socifs.WithOverlayOpaqueType(opq))
	fs, err := socifs.NewFilesystem(ctx, defaultRootDir, config.Config, fsOpts...)
	if err != nil {
		log.G(ctx).WithError(err).Fatalf("failed to prepare fs")
	}

	layerManager, err := store.NewLayerManager(ctx, *rootDir, hosts, mt, fs, config.Config)
	if err != nil {
		log.G(ctx).WithError(err).Fatalf("failed to prepare pool")
	}
	if err := store.Mount(ctx, mountPoint, layerManager, config.Config.Debug); err != nil {
		log.G(ctx).WithError(err).Fatalf("failed to mount fs at %q", mountPoint)
	}
	defer func() {
		syscall.Unmount(mountPoint, 0)
		log.G(ctx).Info("Exiting")
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

	waitForSIGINT()
	log.G(ctx).Info("Got SIGINT")
}

func waitForSIGINT() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c
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

func getMetadataStore(rootDir string, config Config) (metadata.Store, error) {
	bOpts := bolt.Options{
		NoFreelistSync:  true,
		InitialMmapSize: 64 * 1024 * 1024,
		FreelistType:    bolt.FreelistMapType,
	}
	db, err := bolt.Open(filepath.Join(rootDir, "metadata.db"), 0600, &bOpts)
	if err != nil {
		return nil, err
	}
	return func(sr *io.SectionReader, ztoc ztoc.TOC, opts ...metadata.Option) (metadata.Reader, error) {
		return metadata.NewReader(db, sr, ztoc, opts...)
	}, nil
}
