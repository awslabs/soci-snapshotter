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

package snapshot

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"

	commonmetrics "github.com/awslabs/soci-snapshotter/fs/metrics/common"
	"github.com/awslabs/soci-snapshotter/fs/source"
	"github.com/awslabs/soci-snapshotter/idtools"
	"github.com/containerd/containerd/mount"
	"github.com/containerd/containerd/namespaces"
	ctdsnapshotters "github.com/containerd/containerd/pkg/snapshotters"
	"github.com/containerd/containerd/snapshots"
	"github.com/containerd/containerd/snapshots/overlay/overlayutils"
	"github.com/containerd/containerd/snapshots/storage"
	"github.com/containerd/continuity/fs"
	"github.com/containerd/errdefs"
	"github.com/containerd/log"
	"github.com/moby/sys/mountinfo"
	"github.com/opencontainers/go-digest"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

const (
	targetSnapshotLabel = "containerd.io/snapshot.ref"
	remoteLabel         = "containerd.io/snapshot/remote"
	remoteLabelVal      = "remote snapshot"

	// remoteSnapshotLogKey is a key for log line, which indicates whether
	// `Prepare` method successfully prepared targeting remote snapshot or not, as
	// defined in the following:
	// - "true"  : indicates the snapshot has been successfully prepared as a
	//             remote snapshot
	// - "false" : indicates the snapshot failed to be prepared as a remote
	//             snapshot
	// - null    : undetermined
	remoteSnapshotLogKey = "remote-snapshot-prepared"
	prepareSucceeded     = "true"
	prepareFailed        = "false"
)

var (
	// ErrNoIndex is returned by `fs.Mount` when an image should not be lazy loaded
	// because a SOCI index was not found
	ErrNoIndex = errors.New("no valid SOCI index found")
	// ErrDeferToContainerRuntime is called when we cannot prepare a remote or local snapshot,
	// and must ask the container runtime to handle it instead.
	ErrDeferToContainerRuntime = errors.New("deferring to container runtime")
	// ErrNoZtoc is returned by `fs.Mount` when there is no zTOC for a particular layer.
	ErrNoZtoc = errors.New("no ztoc for layer")
	// ErrNoNamespace is used when the snapshot label is not present in the request
	ErrNoNamespace = errors.New("context has no namespace attached")
)

// FileSystem is a backing filesystem abstraction.
//
// Mount() tries to mount a remote snapshot to the specified mount point
// directory. If succeed, the mountpoint directory will be treated as a layer
// snapshot. If Mount() fails, the mountpoint directory MUST be cleaned up.
// Check() is called to check the connectibity of the existing layer snapshot
// every time the layer is used by containerd.
// Unmount() is called to unmount a remote snapshot from the specified mount point
// directory.
// MountLocal() is called to download and decompress a layer to a mount point
// directory. After that it applies the difference to the parent layers if there are any.
// If succeeded, the mountpoint directory will be treated as a regular layer snapshot.
// If MountLocal() fails, the mountpoint directory MUST be cleaned up.
type FileSystem interface {
	Mount(ctx context.Context, mountpoint string, labels map[string]string) error
	Check(ctx context.Context, mountpoint string, labels map[string]string) error
	Unmount(ctx context.Context, mountpoint string) error
	MountLocal(ctx context.Context, mountpoint string, labels map[string]string, mounts []mount.Mount) error
	IDMapMount(ctx context.Context, mountpoint, activeLayerID string, idmap idtools.IDMap) (string, error)
	IDMapMountLocal(ctx context.Context, mountpoint, activeLayerID string, idmap idtools.IDMap) (string, error)
}

// SnapshotterConfig is used to configure the remote snapshotter instance
type SnapshotterConfig struct {
	asyncRemove bool
	// minLayerSize skips remote mounting of smaller layers
	minLayerSize                int64
	allowInvalidMountsOnRestart bool
}

// Opt is an option to configure the remote snapshotter
type Opt func(config *SnapshotterConfig) error

// WithAsynchronousRemove defers removal of filesystem content until
// the Cleanup method is called. Removals will make the snapshot
// referred to by the key unavailable and make the key immediately
// available for re-use.
func WithAsynchronousRemove(config *SnapshotterConfig) error {
	config.asyncRemove = true
	return nil
}

// WithMinLayerSize sets the smallest layer that will be mounted remotely.
func WithMinLayerSize(minLayerSize int64) Opt {
	return func(config *SnapshotterConfig) error {
		config.minLayerSize = minLayerSize
		return nil
	}
}

func AllowInvalidMountsOnRestart(config *SnapshotterConfig) error {
	config.allowInvalidMountsOnRestart = true
	return nil
}

type snapshotter struct {
	root        string
	ms          *storage.MetaStore
	asyncRemove bool

	// fs is a filesystem that this snapshotter recognizes.
	fs                          FileSystem
	userxattr                   bool  // whether to enable "userxattr" mount option
	minLayerSize                int64 // minimum layer size for remote mounting
	allowInvalidMountsOnRestart bool
	idmapped                    *sync.Map
}

// NewSnapshotter returns a Snapshotter which can use unpacked remote layers
// as snapshots. This is implemented based on the overlayfs snapshotter, so
// diffs are stored under the provided root and a metadata file is stored under
// the root as same as overlayfs snapshotter.
func NewSnapshotter(ctx context.Context, root string, targetFs FileSystem, opts ...Opt) (snapshots.Snapshotter, error) {
	if targetFs == nil {
		return nil, fmt.Errorf("specify filesystem to use")
	}

	var config SnapshotterConfig
	for _, opt := range opts {
		if err := opt(&config); err != nil {
			return nil, err
		}
	}

	if err := os.MkdirAll(root, 0700); err != nil {
		return nil, err
	}
	supportsDType, err := fs.SupportsDType(root)
	if err != nil {
		return nil, err
	}
	if !supportsDType {
		return nil, fmt.Errorf("%s does not support d_type. If the backing filesystem is xfs, please reformat with ftype=1 to enable d_type support", root)
	}
	ms, err := storage.NewMetaStore(filepath.Join(root, "metadata.db"))
	if err != nil {
		return nil, err
	}

	if err := os.Mkdir(filepath.Join(root, "snapshots"), 0700); err != nil && !os.IsExist(err) {
		return nil, err
	}

	userxattr, err := overlayutils.NeedsUserXAttr(root)
	if err != nil {
		logrus.WithError(err).Warnf("cannot detect whether \"userxattr\" option needs to be used, assuming to be %v", userxattr)
	}

	idMap := &sync.Map{}

	o := &snapshotter{
		root:                        root,
		ms:                          ms,
		asyncRemove:                 config.asyncRemove,
		fs:                          targetFs,
		userxattr:                   userxattr,
		minLayerSize:                config.minLayerSize,
		allowInvalidMountsOnRestart: config.allowInvalidMountsOnRestart,
		idmapped:                    idMap,
	}

	if err := o.restoreRemoteSnapshot(ctx); err != nil {
		return nil, fmt.Errorf("failed to restore remote snapshot: %w", err)
	}

	return o, nil
}

// Stat returns the info for an active or committed snapshot by name or
// key.
//
// Should be used for parent resolution, existence checks and to discern
// the kind of snapshot.
func (o *snapshotter) Stat(ctx context.Context, key string) (snapshots.Info, error) {
	log.G(ctx).WithField("key", key).Debug("stat")
	ctx, t, err := o.ms.TransactionContext(ctx, false)
	if err != nil {
		return snapshots.Info{}, err
	}
	defer t.Rollback()
	_, info, _, err := storage.GetInfo(ctx, key)
	if err != nil {
		return snapshots.Info{}, err
	}

	return info, nil
}

func (o *snapshotter) Update(ctx context.Context, info snapshots.Info, fieldpaths ...string) (snapshots.Info, error) {
	log.G(ctx).WithField("info", info).Debug("update")
	ctx, t, err := o.ms.TransactionContext(ctx, true)
	if err != nil {
		return snapshots.Info{}, err
	}

	info, err = storage.UpdateInfo(ctx, info, fieldpaths...)
	if err != nil {
		t.Rollback()
		return snapshots.Info{}, err
	}

	if err := t.Commit(); err != nil {
		return snapshots.Info{}, err
	}

	return info, nil
}

// Usage returns the resources taken by the snapshot identified by key.
//
// For active snapshots, this will scan the usage of the overlay "diff" (aka
// "upper") directory and may take some time.
// for remote snapshots, no scan will be held and recognise the number of inodes
// and these sizes as "zero".
//
// For committed snapshots, the value is returned from the metadata database.
func (o *snapshotter) Usage(ctx context.Context, key string) (snapshots.Usage, error) {
	log.G(ctx).WithField("key", key).Debug("usage")
	ctx, t, err := o.ms.TransactionContext(ctx, false)
	if err != nil {
		return snapshots.Usage{}, err
	}
	id, info, usage, err := storage.GetInfo(ctx, key)
	t.Rollback() // transaction no longer needed at this point.

	if err != nil {
		return snapshots.Usage{}, err
	}

	upperPath := o.upperPath(id)

	if info.Kind == snapshots.KindActive {
		du, err := fs.DiskUsage(ctx, upperPath)
		if err != nil {
			// TODO(stevvooe): Consider not reporting an error in this case.
			return snapshots.Usage{}, err
		}

		usage = snapshots.Usage(du)
	}

	return usage, nil
}

func (o *snapshotter) setupIDMap(ctx context.Context, s storage.Snapshot, parent string, labels map[string]string) error {
	// load id-map if appropriate labels are present.
	idmap, err := idtools.LoadIDMap(s.ID, labels)
	if err != nil {
		log.G(ctx).WithError(err).Error("failed to load id-map")
		return err
	}

	if !idmap.Empty() {
		parentSnapshot, err := o.Stat(ctx, parent)
		if err != nil {
			log.G(ctx).WithError(err).Error("failed to stat parent snapshot")
			return err
		}

		// If there is no SOCI index, you can safely mount from the root without copying over every single layer
		if _, ok := parentSnapshot.Labels[source.HasSociIndexDigest]; !ok {
			// Fallback to overlay
			log.G(ctx).Debug("no SOCI index found, remapping from root")
			mounts, err := o.mounts(ctx, s, parent)
			if err != nil {
				return err
			}

			err = idtools.RemapRootFS(ctx, mounts, idmap)
			if err != nil {
				return err
			}
		} else {
			o.idmapped.Store(s.ID, struct{}{})
			err = o.createIDMapMounts(ctx, s, idmap)
			if err != nil {
				log.G(ctx).WithError(err).Error("failed to create id-mapped mounts")
				return err
			}
		}

		log.G(ctx).Debug("id-mapping successful")
	}
	return nil
}

func (o *snapshotter) Prepare(ctx context.Context, key, parent string, opts ...snapshots.Opt) ([]mount.Mount, error) {
	log.G(ctx).WithField("key", key).WithField("parent", parent).Debug("prepare")
	s, err := o.createSnapshot(ctx, snapshots.KindActive, key, parent, opts)
	if err != nil {
		return nil, err
	}

	// Try to prepare the remote snapshot. If succeeded, we commit the snapshot now
	// and return ErrAlreadyExists.
	var base snapshots.Info
	for _, opt := range opts {
		if err := opt(&base); err != nil {
			return nil, err
		}
	}

	target, ok := base.Labels[targetSnapshotLabel]
	// !ok means we are in an active snapshot
	if !ok {
		// Setup id-mapped mounts if config allows.
		// Any error here needs to stop the container from starting.
		if err := o.setupIDMap(ctx, s, parent, base.Labels); err != nil {
			return nil, err
		}
		return o.mounts(ctx, s, parent)
	}

	// Get namespace to save into snapshot
	ns, ok := namespaces.Namespace(ctx)
	if !ok {
		return nil, ErrNoNamespace
	}
	base.Labels[source.TargetNamespace] = ns

	// NOTE: If passed labels include a target of the remote snapshot, `Prepare`
	//       must log whether this method succeeded to prepare that remote snapshot
	//       or not, using the key `remoteSnapshotLogKey` defined in the above. This
	//       log is used by tests in this project.
	lCtx := log.WithLogger(ctx, log.G(ctx).WithField("key", key).WithField("parent", parent))
	log.G(lCtx).Debug("preparing snapshot")

	var skipLazyLoadingImage bool

	// remote snapshot prepare
	if !o.skipRemoteSnapshotPrepare(lCtx, base.Labels) {
		err := o.prepareRemoteSnapshot(lCtx, key, base.Labels)
		if err == nil {
			base.Labels[remoteLabel] = remoteLabelVal       // Mark this snapshot as remote
			base.Labels[source.HasSociIndexDigest] = "true" // Mark that this snapshot was loaded with a SOCI index
			err := o.commit(ctx, true, target, key, append(opts, snapshots.WithLabels(base.Labels))...)
			if err == nil || errdefs.IsAlreadyExists(err) {
				// count also AlreadyExists as "success"
				log.G(lCtx).WithField(remoteSnapshotLogKey, prepareSucceeded).Info("remote snapshot successfully prepared.")
				return nil, fmt.Errorf("target snapshot %q: %w", target, errdefs.ErrAlreadyExists)
			}
			log.G(lCtx).WithField(remoteSnapshotLogKey, prepareFailed).WithError(err).Warn("failed to internally commit remote snapshot")
			// Don't fallback here (= prohibit to use this key again) because the FileSystem
			// possible has done some work on this "upper" directory.
			return nil, err
		}

		log.G(lCtx).WithField(remoteSnapshotLogKey, prepareFailed).WithError(err).Warn("failed to prepare remote snapshot")
		switch {
		case errors.Is(err, ErrNoZtoc):
			// no-op
		case errors.Is(err, ErrNoIndex):
			skipLazyLoadingImage = true
		default:
			commonmetrics.IncOperationCount(commonmetrics.FuseMountFailureCount, digest.Digest(""))
		}
	}

	// fall back to local snapshot
	mounts, err := o.mounts(ctx, s, parent)
	if err != nil {
		// don't fallback here, since there was an error getting mounts
		return nil, err
	}

	// If the underlying FileSystem deems that the image is unable to be lazy loaded,
	// then we should completely fallback to the container runtime to handle
	// pulling and unpacking all the layers in the image.
	if skipLazyLoadingImage {
		log.G(lCtx).WithError(err).Warnf("%v; %v", ErrNoIndex, ErrDeferToContainerRuntime)
		return mounts, nil
	}

	log.G(ctx).WithField("layerDigest", base.Labels[ctdsnapshotters.TargetLayerDigestLabel]).Info("preparing snapshot as local snapshot")
	err = o.prepareLocalSnapshot(lCtx, key, base.Labels, mounts)
	if err == nil {
		base.Labels[source.HasSociIndexDigest] = "true" // Mark that this snapshot was loaded with a SOCI index
		err := o.commit(ctx, false, target, key, append(opts, snapshots.WithLabels(base.Labels))...)
		if err == nil || errdefs.IsAlreadyExists(err) {
			// count also AlreadyExists as "success"
			// there's no need to provide any details on []mount.Mount because mounting is already taken care of
			// by snapshotter
			log.G(lCtx).Info("local snapshot successfully prepared")
			return nil, fmt.Errorf("target snapshot %q: %w", target, errdefs.ErrAlreadyExists)
		}
		log.G(lCtx).WithError(err).Warn("failed to internally commit local snapshot")
		// Don't fallback here (= prohibit to use this key again) because the FileSystem
		// possible has done some work on this "upper" directory.
		return nil, err
	}

	// Local snapshot setup failed. Generally means something critical has gone wrong.
	log.G(lCtx).WithError(err).Warnf("failed to prepare local snapshot; %v", ErrDeferToContainerRuntime)
	commonmetrics.IncOperationCount(commonmetrics.LocalMountFailureCount, digest.Digest(""))
	return mounts, nil
}

func (o *snapshotter) skipRemoteSnapshotPrepare(ctx context.Context, labels map[string]string) bool {
	if o.minLayerSize > 0 {
		if strVal, ok := labels[source.TargetSizeLabel]; ok {
			if intVal, err := strconv.ParseInt(strVal, 10, 64); err == nil {
				if intVal < o.minLayerSize {
					log.G(ctx).Info("layer size less than runtime min_layer_size, skipping remote snapshot preparation")
					return true
				}
			} else {
				log.G(ctx).WithError(err).Errorf("config min_layer_size cannot be converted to int: %s", strVal)
			}
		}
	}
	return false
}

func (o *snapshotter) View(ctx context.Context, key, parent string, opts ...snapshots.Opt) ([]mount.Mount, error) {
	log.G(ctx).WithField("key", key).Debug("view")
	s, err := o.createSnapshot(ctx, snapshots.KindView, key, parent, opts)
	if err != nil {
		return nil, err
	}
	return o.mounts(ctx, s, parent)
}

// Mounts returns the mounts for the transaction identified by key. Can be
// called on an read-write or readonly transaction.
//
// This can be used to recover mounts after calling View or Prepare.
func (o *snapshotter) Mounts(ctx context.Context, key string) ([]mount.Mount, error) {
	log.G(ctx).WithField("key", key).Debug("mounts")
	ctx, t, err := o.ms.TransactionContext(ctx, false)
	if err != nil {
		return nil, err
	}
	s, err := storage.GetSnapshot(ctx, key)
	t.Rollback()
	if err != nil {
		return nil, fmt.Errorf("failed to get active mount: %w", err)
	}
	return o.mounts(ctx, s, key)
}

func (o *snapshotter) Commit(ctx context.Context, name, key string, opts ...snapshots.Opt) error {
	log.G(ctx).WithField("key", key).Debug("commit")
	return o.commit(ctx, false, name, key, opts...)
}

func (o *snapshotter) commit(ctx context.Context, isRemote bool, name, key string, opts ...snapshots.Opt) error {
	ctx, t, err := o.ms.TransactionContext(ctx, true)
	if err != nil {
		return err
	}

	defer func() {
		if err != nil {
			if rerr := t.Rollback(); rerr != nil {
				log.G(ctx).WithError(rerr).Warn("failed to rollback transaction")
			}
		}
	}()

	// grab the existing id
	id, _, usage, err := storage.GetInfo(ctx, key)
	if err != nil {
		return err
	}

	if !isRemote { // skip diskusage for remote snapshots for allowing lazy preparation of nodes
		du, err := fs.DiskUsage(ctx, o.upperPath(id))
		if err != nil {
			return err
		}
		usage = snapshots.Usage(du)
	}

	if _, err = storage.CommitActive(ctx, key, name, usage, opts...); err != nil {
		return fmt.Errorf("failed to commit snapshot: %w", err)
	}

	return t.Commit()
}

// Remove abandons the snapshot identified by key. The snapshot will
// immediately become unavailable and unrecoverable. Disk space will
// be freed up on the next call to `Cleanup`.
func (o *snapshotter) Remove(ctx context.Context, key string) (err error) {
	log.G(ctx).WithField("key", key).Debug("remove")
	ctx, t, err := o.ms.TransactionContext(ctx, true)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			if rerr := t.Rollback(); rerr != nil {
				log.G(ctx).WithError(rerr).Warn("failed to rollback transaction")
			}
		}
	}()

	_, _, err = storage.Remove(ctx, key)
	if err != nil {
		return fmt.Errorf("failed to remove: %w", err)
	}

	if !o.asyncRemove {
		var removals []string
		const cleanupCommitted = false
		removals, err = o.getCleanupDirectories(ctx, t, cleanupCommitted)
		if err != nil {
			return fmt.Errorf("unable to get directories for removal: %w", err)
		}

		// Remove directories after the transaction is closed, failures must not
		// return error since the transaction is committed with the removal
		// key no longer available.
		defer func() {
			if err == nil {
				for _, dir := range removals {
					if err := o.cleanupSnapshotDirectory(ctx, dir); err != nil {
						log.G(ctx).WithError(err).WithField("path", dir).Warn("failed to remove directory")
					}
				}
			}
		}()

	}

	return t.Commit()
}

// Walk the snapshots.
func (o *snapshotter) Walk(ctx context.Context, fn snapshots.WalkFunc, fs ...string) error {
	log.G(ctx).Debug("walk")
	ctx, t, err := o.ms.TransactionContext(ctx, false)
	if err != nil {
		return err
	}
	defer t.Rollback()
	return storage.WalkInfo(ctx, fn, fs...)
}

// Cleanup cleans up disk resources from removed or abandoned snapshots
func (o *snapshotter) Cleanup(ctx context.Context) error {
	log.G(ctx).Debug("cleanup")
	const cleanupCommitted = false
	return o.cleanup(ctx, cleanupCommitted)
}

func (o *snapshotter) cleanup(ctx context.Context, cleanupCommitted bool) error {
	cleanup, err := o.cleanupDirectories(ctx, cleanupCommitted)
	if err != nil {
		return err
	}

	log.G(ctx).Debugf("cleanup: dirs=%v", cleanup)
	for _, dir := range cleanup {
		if err := o.cleanupSnapshotDirectory(ctx, dir); err != nil {
			log.G(ctx).WithError(err).WithField("path", dir).Warn("failed to remove directory")
		}
	}

	return nil
}

func (o *snapshotter) cleanupDirectories(ctx context.Context, cleanupCommitted bool) ([]string, error) {
	// Get a write transaction to ensure no other write transaction can be entered
	// while the cleanup is scanning.
	ctx, t, err := o.ms.TransactionContext(ctx, true)
	if err != nil {
		return nil, err
	}

	defer t.Rollback()
	return o.getCleanupDirectories(ctx, t, cleanupCommitted)
}

func (o *snapshotter) getCleanupDirectories(ctx context.Context, t storage.Transactor, cleanupCommitted bool) ([]string, error) {
	ids, err := storage.IDMap(ctx)
	if err != nil {
		return nil, err
	}

	snapshotDir := filepath.Join(o.root, "snapshots")
	fd, err := os.Open(snapshotDir)
	if err != nil {
		return nil, err
	}
	defer fd.Close()

	dirs, err := fd.Readdirnames(0)
	if err != nil {
		return nil, err
	}

	cleanup := []string{}
	for _, d := range dirs {
		if !cleanupCommitted {
			// If the directory name is just a number (e.g '2'),
			// we want to check if the dir name (2) must be cleaned
			// If the directory has an underscore (e.g. '1_2'),
			// we want to check the suffix (2) to determine if
			// the directory must be cleaned
			cleanupID := d
			temp := strings.Split(d, "_")
			if len(temp) > 1 {
				cleanupID = temp[1]
			}

			if _, ok := ids[cleanupID]; ok {
				continue
			}
		}

		cleanup = append(cleanup, filepath.Join(snapshotDir, d))
	}

	return cleanup, nil
}

func (o *snapshotter) cleanupSnapshotDirectory(ctx context.Context, dir string) error {
	if err := o.unmountSnapshotDirectory(ctx, dir); err != nil {
		return fmt.Errorf("failed to unmount directory %q: %w", dir, err)
	}
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("failed to remove directory %q: %w", dir, err)
	}
	return nil
}

func (o *snapshotter) unmountSnapshotDirectory(ctx context.Context, dir string) error {
	// On a remote snapshot, the layer is mounted on the "fs" directory.
	// We use Filesystem's Unmount API so that it can do necessary finalization
	// before/after the unmount.
	mp := filepath.Join(dir, "fs")
	mounted, err := mountinfo.Mounted(mp)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// nothing to unmount as directory does not exist
			return nil
		}
		return err
	}
	if mounted {
		return o.fs.Unmount(ctx, mp)
	}
	return nil
}

func (o *snapshotter) createSnapshot(ctx context.Context, kind snapshots.Kind, key, parent string, opts []snapshots.Opt) (_ storage.Snapshot, err error) {
	ctx, t, err := o.ms.TransactionContext(ctx, true)
	if err != nil {
		return storage.Snapshot{}, err
	}

	var td, path string
	defer func() {
		if err != nil {
			if td != "" {
				if err1 := o.cleanupSnapshotDirectory(ctx, td); err1 != nil {
					log.G(ctx).WithError(err1).Warn("failed to cleanup temp snapshot directory")
				}
			}
			if path != "" {
				if err1 := o.cleanupSnapshotDirectory(ctx, path); err1 != nil {
					log.G(ctx).WithError(err1).WithField("path", path).Error("failed to reclaim snapshot directory, directory may need removal")
					err = fmt.Errorf("failed to remove path: %v: %w", err1, err)
				}
			}
		}
	}()

	snapshotDir := filepath.Join(o.root, "snapshots")
	td, err = o.prepareDirectory(ctx, snapshotDir, kind)
	if err != nil {
		if rerr := t.Rollback(); rerr != nil {
			log.G(ctx).WithError(rerr).Warn("failed to rollback transaction")
		}
		return storage.Snapshot{}, fmt.Errorf("failed to create prepare snapshot dir: %w", err)
	}
	rollback := true
	defer func() {
		if rollback {
			if rerr := t.Rollback(); rerr != nil {
				log.G(ctx).WithError(rerr).Warn("failed to rollback transaction")
			}
		}
	}()

	s, err := storage.CreateSnapshot(ctx, kind, key, parent, opts...)
	if err != nil {
		return storage.Snapshot{}, fmt.Errorf("failed to create snapshot: %w", err)
	}

	if len(s.ParentIDs) > 0 {
		st, err := os.Stat(o.upperPath(s.ParentIDs[0]))
		if err != nil {
			return storage.Snapshot{}, fmt.Errorf("failed to stat parent: %w", err)
		}

		stat := st.Sys().(*syscall.Stat_t)

		if err := os.Lchown(filepath.Join(td, "fs"), int(stat.Uid), int(stat.Gid)); err != nil {
			if rerr := t.Rollback(); rerr != nil {
				log.G(ctx).WithError(rerr).Warn("failed to rollback transaction")
			}
			return storage.Snapshot{}, fmt.Errorf("failed to chown: %w", err)
		}
	}

	path = filepath.Join(snapshotDir, s.ID)
	if err = os.Rename(td, path); err != nil {
		return storage.Snapshot{}, fmt.Errorf("failed to rename: %w", err)
	}
	td = ""

	rollback = false
	if err = t.Commit(); err != nil {
		return storage.Snapshot{}, fmt.Errorf("commit failed: %w", err)
	}

	return s, nil
}

func (o *snapshotter) prepareDirectory(ctx context.Context, snapshotDir string, kind snapshots.Kind) (string, error) {
	td, err := os.MkdirTemp(snapshotDir, "new-")
	if err != nil {
		return "", fmt.Errorf("failed to create temp dir: %w", err)
	}

	if err := os.Mkdir(filepath.Join(td, "fs"), 0755); err != nil {
		return td, err
	}

	if kind == snapshots.KindActive {
		if err := os.Mkdir(filepath.Join(td, "work"), 0711); err != nil {
			return td, err
		}
	}

	return td, nil
}

func (o *snapshotter) mounts(ctx context.Context, s storage.Snapshot, checkKey string) ([]mount.Mount, error) {
	// Make sure that all layers lower than the target layer are available
	if checkKey != "" && !o.checkAvailability(ctx, checkKey) {
		return nil, fmt.Errorf("layer %q unavailable: %w", s.ID, errdefs.ErrUnavailable)
	}

	if len(s.ParentIDs) == 0 {
		// if we only have one layer/no parents then just return a bind mount as overlay
		// will not work
		roFlag := "rw"
		if s.Kind == snapshots.KindView {
			roFlag = "ro"
		}

		return []mount.Mount{
			{
				Source: o.upperPath(s.ID),
				Type:   "bind",
				Options: []string{
					roFlag,
					"rbind",
				},
			},
		}, nil
	}
	var options []string

	if s.Kind == snapshots.KindActive {
		options = append(options,
			fmt.Sprintf("workdir=%s", o.workPath(s.ID)),
			fmt.Sprintf("upperdir=%s", o.upperPath(s.ID)),
		)
	} else if len(s.ParentIDs) == 1 {
		return []mount.Mount{
			{
				Source: o.upperPath(s.ParentIDs[0]),
				Type:   "bind",
				Options: []string{
					"ro",
					"rbind",
				},
			},
		}, nil
	}

	parentPaths, err := o.getParentPaths(s)
	if err != nil {
		return nil, err
	}

	options = append(options, fmt.Sprintf("lowerdir=%s", strings.Join(parentPaths, ":")))
	if o.userxattr {
		options = append(options, "userxattr")
	}

	return []mount.Mount{
		{
			Type:    "overlay",
			Source:  "overlay",
			Options: options,
		},
	}, nil
}

func (o *snapshotter) getParentPaths(s storage.Snapshot) ([]string, error) {
	parentPaths := make([]string, len(s.ParentIDs))

	for i, id := range s.ParentIDs {
		if _, ok := o.idmapped.Load(s.ID); ok {
			id = fmt.Sprintf("%s_%s", id, s.ID)
		}
		parentPaths[i] = o.upperPath(id)
	}

	return parentPaths, nil
}

func (o *snapshotter) createIDMapMounts(ctx context.Context, s storage.Snapshot, idmap idtools.IDMap) error {
	log.G(ctx).Debug("mapping ids")

	for _, id := range s.ParentIDs {
		err := o.createIDMapMount(ctx, o.upperPath(id), s.ID, idmap)
		if err != nil {
			return err
		}
	}

	return idtools.RemapRoot(ctx, o.upperPath(s.ID), idmap)
}

func (o *snapshotter) createIDMapMount(ctx context.Context, path, id string, idmap idtools.IDMap) error {
	// s.ID is the shortest unique identifier for each new container,
	// so append it to the end of the new mountpoint
	_, err := o.fs.IDMapMount(ctx, path, id, idmap)
	if errdefs.IsNotFound(err) {
		// Remote mount failed, attempt to create a local id-mapped mount

		// Cleanup dirty snapshot folder — perhaps we can have a return cleanup func?
		dirtyDir := fmt.Sprintf("%s_%s", filepath.Dir(path), id)
		if err := os.RemoveAll(dirtyDir); err != nil {
			return err
		}
		_, err = o.fs.IDMapMountLocal(ctx, path, id, idmap)
	}
	return err
}

// upperPath produces a file path like "{snapshotter.root}/snapshots/{id}/fs"
func (o *snapshotter) upperPath(id string) string {
	return filepath.Join(o.root, "snapshots", id, "fs")
}

// workPath produces a file path like "{snapshotter.root}/snapshots/{id}/work"
func (o *snapshotter) workPath(id string) string {
	return filepath.Join(o.root, "snapshots", id, "work")
}

// Close closes the snapshotter
func (o *snapshotter) Close() error {
	log.L.Debug("close")
	// unmount all mounts including Committed
	const cleanupCommitted = true
	ctx := context.Background()
	if err := o.unmountAllSnapshots(ctx, cleanupCommitted); err != nil {
		log.G(ctx).WithError(err).Warn("failed to unmount snapshots on close")
	}
	return o.ms.Close()
}

func (o *snapshotter) unmountAllSnapshots(ctx context.Context, cleanupCommitted bool) error {
	cleanup, err := o.cleanupDirectories(ctx, cleanupCommitted)
	if err != nil {
		return err
	}

	log.G(ctx).Debugf("unmount: dirs=%v", cleanup)
	for _, dir := range cleanup {
		if err := o.unmountSnapshotDirectory(ctx, dir); err != nil {
			log.G(ctx).WithError(err).WithField("path", dir).Warn("failed to unmount directory")
		}
	}

	return nil
}

// prepareLocalSnapshot tries to prepare the snapshot as a local snapshot.
func (o *snapshotter) prepareLocalSnapshot(ctx context.Context, key string, labels map[string]string, mounts []mount.Mount) error {
	ctx, t, err := o.ms.TransactionContext(ctx, false)
	if err != nil {
		return err
	}
	defer t.Rollback()
	id, _, _, err := storage.GetInfo(ctx, key)
	if err != nil {
		return err
	}
	mountpoint := o.upperPath(id)
	log.G(ctx).Infof("preparing local filesystem at mountpoint=%v", mountpoint)
	return o.fs.MountLocal(ctx, mountpoint, labels, mounts)
}

// prepareRemoteSnapshot tries to prepare the snapshot as a remote snapshot
// using filesystems registered in this snapshotter.
func (o *snapshotter) prepareRemoteSnapshot(ctx context.Context, key string, labels map[string]string) error {
	ctx, t, err := o.ms.TransactionContext(ctx, false)
	if err != nil {
		return err
	}
	defer t.Rollback()
	id, _, _, err := storage.GetInfo(ctx, key)
	if err != nil {
		return err
	}

	mountpoint := o.upperPath(id)
	log.G(ctx).Infof("preparing filesystem mount at mountpoint=%v", mountpoint)

	return o.fs.Mount(ctx, mountpoint, labels)
}

// checkAvailability checks avaiability of the specified layer and all lower
// layers using filesystem's checking functionality.
func (o *snapshotter) checkAvailability(ctx context.Context, key string) bool {
	ctx = log.WithLogger(ctx, log.G(ctx).WithField("key", key))
	log.G(ctx).Debug("checking layer availability")

	ctx, t, err := o.ms.TransactionContext(ctx, false)
	if err != nil {
		log.G(ctx).WithError(err).Warn("failed to get transaction")
		return false
	}
	defer t.Rollback()

	eg, egCtx := errgroup.WithContext(ctx)
	for cKey := key; cKey != ""; {
		id, info, _, err := storage.GetInfo(ctx, cKey)
		if err != nil {
			log.G(ctx).WithError(err).Warnf("failed to get info of %q", cKey)
			return false
		}
		mp := o.upperPath(id)
		lCtx := log.WithLogger(ctx, log.G(ctx).WithField("mount-point", mp))
		if _, ok := info.Labels[remoteLabel]; ok {
			eg.Go(func() error {
				log.G(lCtx).Debug("checking mount point")
				if err := o.fs.Check(egCtx, mp, info.Labels); err != nil {
					log.G(lCtx).WithError(err).Warn("layer is unavailable")
					return err
				}
				return nil
			})
		} else {
			log.G(lCtx).Debug("layer is normal snapshot(overlayfs)")
		}
		cKey = info.Parent
	}
	if err := eg.Wait(); err != nil {
		return false
	}
	return true
}

func (o *snapshotter) restoreRemoteSnapshot(ctx context.Context) error {
	mounts, err := mountinfo.GetMounts(nil)
	if err != nil {
		return err
	}
	for _, m := range mounts {
		if strings.HasPrefix(m.Mountpoint, filepath.Join(o.root, "snapshots")) {
			if err := syscall.Unmount(m.Mountpoint, syscall.MNT_FORCE); err != nil {
				return fmt.Errorf("failed to unmount %s: %w", m.Mountpoint, err)
			}
		}
	}

	var task []snapshots.Info
	if err := o.Walk(ctx, func(ctx context.Context, info snapshots.Info) error {
		if _, ok := info.Labels[remoteLabel]; ok {
			task = append(task, info)
		}
		return nil
	}); err != nil && !errdefs.IsNotFound(err) {
		return err
	}
	for _, info := range task {
		ns, ok := info.Labels[source.TargetNamespace]
		if !ok {
			return ErrNoNamespace
		}
		ctx = namespaces.WithNamespace(ctx, ns)
		if err := o.prepareRemoteSnapshot(ctx, info.Name, info.Labels); err != nil {
			if o.allowInvalidMountsOnRestart {
				logrus.WithError(err).Warnf("failed to restore remote snapshot %s; remove this snapshot manually", info.Name)
				// This snapshot mount is invalid but allow this.
				// NOTE: snapshotter.Mount() will fail to return the mountpoint of these invalid snapshots so
				//       containerd cannot use them anymore. User needs to manually remove the snapshots from
				//       containerd's metadata store using ctr (e.g. `ctr snapshot rm`).
				continue
			}
			return fmt.Errorf("failed to prepare remote snapshot: %s: %w", info.Name, err)
		}
	}

	return nil
}
