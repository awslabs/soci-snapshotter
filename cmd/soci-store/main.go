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

// Command soci-store is a containers/storage "additional layer store" (ALS)
// that lets CRI-O (and Podman) lazily load SOCI-converted images. It exposes a
// single FUSE mount whose tree containers/storage resolves as
//
//	<mountpoint>/<base64(image-ref)>/<TOC-digest>/{diff,info,blob,use}
//
// Configure it in storage.conf, e.g.:
//
//	additionallayerstores = ["/var/lib/soci-store/store:ref"]
//
// and run it with the mountpoint as a positional argument:
//
//	soci-store /var/lib/soci-store/store
package main

import (
	"context"
	"errors"
	"flag"
	"io"
	golog "log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	storepkg "github.com/awslabs/soci-snapshotter/cmd/soci-store/store"
	"github.com/awslabs/soci-snapshotter/config"
	"github.com/awslabs/soci-snapshotter/metadata"
	"github.com/awslabs/soci-snapshotter/service/keychain/dockerconfig"
	"github.com/awslabs/soci-snapshotter/service/resolver"
	socistore "github.com/awslabs/soci-snapshotter/soci/store"
	"github.com/awslabs/soci-snapshotter/ztoc"
	"github.com/containerd/log"
	sddaemon "github.com/coreos/go-systemd/v22/daemon"
	bolt "go.etcd.io/bbolt"
)

const (
	defaultLogLevel   = log.InfoLevel
	defaultConfigPath = "/etc/soci-store/config.toml"
	defaultRootDir    = "/var/lib/soci-store"
)

var (
	configPath = flag.String("config", defaultConfigPath, "path to the configuration file")
	logLevel   = flag.String("log-level", defaultLogLevel.String(), "set the logging level [trace, debug, info, warn, error, fatal, panic]")
	rootDir    = flag.String("root", defaultRootDir, "path to the root directory for this store")
)

func main() {
	flag.Parse()
	mountPoint := flag.Arg(0)
	if err := log.SetLevel(*logLevel); err != nil {
		log.L.WithError(err).Fatal("failed to prepare logger")
	}
	log.SetFormat(log.JSONFormat)
	ctx := log.WithLogger(context.Background(), log.L)

	// Stream logs of the standard lib (go-fuse uses this) into the debug log.
	golog.SetOutput(log.G(ctx).WriterLevel(log.DebugLevel))

	if mountPoint == "" {
		log.G(ctx).Fatal("mount point must be specified as a positional argument")
	}

	// Load configuration; a missing file falls back to built-in defaults.
	cfg, err := config.NewConfigFromToml(*configPath)
	if errors.Is(err, os.ErrNotExist) {
		cfg = config.NewConfig()
	} else if err != nil {
		log.G(ctx).WithError(err).Fatalf("failed to load config file %q", *configPath)
	}
	if cfg == nil {
		log.G(ctx).Fatal("failed to create default config")
	}
	if cfg.DisableVerification {
		log.G(ctx).Fatal("content verification can't be disabled for soci-store")
	}

	// Source credentials from the Docker/Podman config (auth.json).
	credsFuncs := []resolver.Credential{dockerconfig.NewDockerConfigKeychain(ctx)}
	hosts := resolver.NewRegistryManager(
		cfg.FSConfig.RetryableHTTPClientConfig,
		cfg.ServiceConfig.ResolverConfig,
		credsFuncs,
	).AsRegistryHosts()

	// Ensure the mountpoint exists.
	if err := os.MkdirAll(mountPoint, 0755); err != nil {
		log.G(ctx).WithError(err).Fatalf("failed to prepare mountpoint %q", mountPoint)
	}

	mt, err := getMetadataStore(*rootDir)
	if err != nil {
		log.G(ctx).WithError(err).Fatal("failed to configure metadata store")
	}

	// On-disk OCI content store under <root>/content.
	contentStore, err := socistore.NewContentStore(
		socistore.WithType(socistore.SociContentStoreType),
		socistore.WithSnapshotterRoot(*rootDir),
	)
	if err != nil {
		log.G(ctx).WithError(err).Fatal("failed to create content store")
	}

	layerManager, err := storepkg.NewLayerManager(ctx, *rootDir, hosts, mt, contentStore, cfg)
	if err != nil {
		log.G(ctx).WithError(err).Fatal("failed to prepare layer manager")
	}

	if err := storepkg.Mount(ctx, mountPoint, layerManager, cfg.Debug); err != nil {
		log.G(ctx).WithError(err).Fatalf("failed to mount fs at %q", mountPoint)
	}
	defer func() {
		syscall.Unmount(mountPoint, 0)
		log.G(ctx).Info("Exiting")
	}()

	// Support systemd Type=notify.
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

	log.G(ctx).WithField("mountpoint", mountPoint).Info("soci-store successfully started")
	waitForSignal(ctx)
}

func waitForSignal(ctx context.Context) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	s := <-c
	log.G(ctx).Infof("Got %v", s)
}

// getMetadataStore opens a bolt-backed metadata store at <rootDir>/metadata.db.
func getMetadataStore(rootDir string) (metadata.Store, error) {
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
}
