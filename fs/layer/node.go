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

package layer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	commonmetrics "github.com/awslabs/soci-snapshotter/fs/metrics/common"
	"github.com/awslabs/soci-snapshotter/fs/reader"
	"github.com/awslabs/soci-snapshotter/fs/remote"
	"github.com/awslabs/soci-snapshotter/metadata"
	"github.com/containerd/log"
	fusefs "github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	digest "github.com/opencontainers/go-digest"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

const (
	blockSize = 4096
	// physicalBlockRatio is the ratio of blockSize to 512. It's used as a multiplier
	// to convert # of blockSize-byte blocks to # of 512 byte blocks.
	physicalBlockRatio = blockSize / 512
	whiteoutPrefix     = ".wh."
	whiteoutOpaqueDir  = whiteoutPrefix + whiteoutPrefix + ".opq"
	opaqueXattrValue   = "y"
	stateDirName       = ".soci-snapshotter"
	statFileMode       = syscall.S_IFREG | 0400 // -r--------
	stateDirMode       = syscall.S_IFDIR | 0500 // dr-x------
)

// OverlayOpaqueType enum possible types.
type OverlayOpaqueType int

// OverlayOpaqueType enum.
const (
	OverlayOpaqueAll OverlayOpaqueType = iota
	OverlayOpaqueTrusted
	OverlayOpaqueUser
)

var opaqueXattrs = map[OverlayOpaqueType][]string{
	OverlayOpaqueAll:     {"trusted.overlay.opaque", "user.overlay.opaque"},
	OverlayOpaqueTrusted: {"trusted.overlay.opaque"},
	OverlayOpaqueUser:    {"user.overlay.opaque"},
}

// fuse operations.
const (
	fuseOpGetattr         = "node.Getattr"
	fuseOpGetxattr        = "node.Getxattr"
	fuseOpListxattr       = "node.Listxattr"
	fuseOpLookup          = "node.Lookup"
	fuseOpOpen            = "node.Open"
	fuseOpReaddir         = "node.Readdir"
	fuseOpReadLink        = "node.Readlink"
	fuseOpFileRead        = "file.Read"
	fuseOpFileGetattr     = "file.Getattr"
	fuseOpWhiteoutGetattr = "whiteout.Getattr"
)

var fuseOpFailureMetrics = map[string]string{
	fuseOpGetattr:         commonmetrics.FuseNodeGetattrFailureCount,
	fuseOpListxattr:       commonmetrics.FuseNodeListxattrFailureCount,
	fuseOpLookup:          commonmetrics.FuseNodeLookupFailureCount,
	fuseOpOpen:            commonmetrics.FuseNodeOpenFailureCount,
	fuseOpReaddir:         commonmetrics.FuseNodeReaddirFailureCount,
	fuseOpFileRead:        commonmetrics.FuseFileReadFailureCount,
	fuseOpFileGetattr:     commonmetrics.FuseFileGetattrFailureCount,
	fuseOpWhiteoutGetattr: commonmetrics.FuseWhiteoutGetattrFailureCount,
}

// FuseOperationCounter collects number of invocations of the various FUSE implementations and emits them as metrics.
// Setting `waitPeriod` to be > 0 allows delaying the time when the metrics are emitted.
type FuseOperationCounter struct {
	opCounts    map[string]*int32
	waitPeriod  time.Duration
	imageDigest digest.Digest
}

// FuseOpsList is a list of available FUSE operations.
var FuseOpsList = []string{
	fuseOpGetattr,
	fuseOpGetxattr,
	fuseOpListxattr,
	fuseOpLookup,
	fuseOpOpen,
	fuseOpReaddir,
	fuseOpReadLink,
	fuseOpFileRead,
	fuseOpFileGetattr,
	fuseOpWhiteoutGetattr,
}

// NewFuseOperationCounter constructs a FuseOperationCounter for an image with digest imgDigest.
// waitPeriod specifies how long to wait before emitting the aggregated metrics.
func NewFuseOperationCounter(imgDigest digest.Digest, waitPeriod time.Duration) *FuseOperationCounter {
	f := &FuseOperationCounter{
		imageDigest: imgDigest,
		waitPeriod:  waitPeriod,
		opCounts:    make(map[string]*int32),
	}
	for _, m := range FuseOpsList {
		f.opCounts[m] = new(int32)
	}
	return f
}

// Inc atomically increase the count of FUSE operation op.
// Noop if op is not in FuseOpsList.
func (f *FuseOperationCounter) Inc(op string) {
	opCount, ok := f.opCounts[op]
	if !ok {
		return
	}
	atomic.AddInt32(opCount, 1)
}

// Run waits for f.waitPeriod to pass before emitting a log and metric for each
// operation in FuseOpsList. Should be started in different goroutine so that it
// doesn't block the current goroutine.
func (f *FuseOperationCounter) Run(ctx context.Context) {
	select {
	case <-ctx.Done():
		return
	case <-time.After(f.waitPeriod):
		for op, opCount := range f.opCounts {
			// We want both an aggregated metric (e.g. p90) and an image specific metric so that we can compare
			// how a specific image is behaving to a larger dataset. When the image cardinality is small,
			// we can just include the image digest as a label on the metric itself, however, when the cardinality
			// is large, this can be very expensive. Here we give consumers options by emitting both logs and
			// metrics. A low cardinality use case can rely on metrics. A high cardinality use case can
			// aggregate the metrics across all images, but still get the per-image info via logs.
			count := atomic.LoadInt32(opCount)
			commonmetrics.AddImageOperationCount(op, f.imageDigest, count)
			log.G(ctx).Infof("fuse operation count for image %s: %s = %d", f.imageDigest, op, count)
		}
	}
}

// logFSOperations may cause sensitive information to be emitted to logs
// e.g. filenames and paths within an image
func newNode(layerDgst digest.Digest, r reader.Reader, blob remote.Blob, baseInode uint32, opaque OverlayOpaqueType, logFSOperations bool, opCounter *FuseOperationCounter) (fusefs.InodeEmbedder, error) {
	rootID := r.Metadata().RootID()
	rootAttr, err := r.Metadata().GetAttr(rootID)
	if err != nil {
		return nil, err
	}
	opq, ok := opaqueXattrs[opaque]
	if !ok {
		return nil, fmt.Errorf("unknown overlay opaque type")
	}
	ffs := &fs{
		r:                r,
		layerDigest:      layerDgst,
		baseInode:        baseInode,
		rootID:           rootID,
		opaqueXattrs:     opq,
		logFSOperations:  logFSOperations,
		operationCounter: opCounter,
	}
	ffs.s = ffs.newState(layerDgst, blob)
	return &node{
		id:   rootID,
		attr: rootAttr,
		fs:   ffs,
	}, nil
}

// fs contains global metadata used by nodes
type fs struct {
	r                reader.Reader
	s                *state
	layerDigest      digest.Digest
	baseInode        uint32
	rootID           uint32
	opaqueXattrs     []string
	logFSOperations  bool
	operationCounter *FuseOperationCounter
}

func (fs *fs) inodeOfState() uint64 {
	return (uint64(fs.baseInode) << 32) | 1 // reserved
}

func (fs *fs) inodeOfStatFile() uint64 {
	return (uint64(fs.baseInode) << 32) | 2 // reserved
}

// logAndIncrementOpCounter handles logging as well as incrementing
// the FuseOperationCounter.
func (fs *fs) logAndIncrementOpCounter(ctx context.Context, operationName string, path string) {
	if fs.logFSOperations {
		log.G(ctx).WithFields(logrus.Fields{
			"operation": operationName,
			"path":      path,
		}).Debug("FUSE operation")
	}
	if fs.operationCounter != nil {
		fs.operationCounter.Inc(operationName)
	}
}

// reportFailure handles telemetry operations pertaining to FUSE failures
// as well as writing an error to the state file.
func (fs *fs) reportFailure(operationName string, stateError error) {
	metric, ok := fuseOpFailureMetrics[operationName]
	if !ok {
		metric = commonmetrics.FuseUnknownFailureCount
	}
	commonmetrics.ReportFuseFailure(metric, fs.layerDigest)
	fs.s.report(stateError)
}

func (fs *fs) inodeOfID(id uint32) (uint64, error) {
	// 0 is reserved by go-fuse 1 and 2 are reserved by the state dir
	if id > ^uint32(0)-3 {
		return 0, fmt.Errorf("too many inodes")
	}
	return (uint64(fs.baseInode) << 32) | uint64(3+id), nil
}

// node is a filesystem inode abstraction.
type node struct {
	fusefs.Inode
	fs   *fs
	id   uint32
	attr metadata.Attr

	ents       []fuse.DirEntry
	entsCached bool
	entsMu     sync.Mutex
}

func (n *node) isRootNode() bool {
	return n.id == n.fs.rootID
}

func (n *node) isOpaque() bool {
	if _, _, err := n.fs.r.Metadata().GetChild(n.id, whiteoutOpaqueDir); err == nil {
		return true
	}
	return false
}

var _ = (fusefs.InodeEmbedder)((*node)(nil))

var _ = (fusefs.NodeReaddirer)((*node)(nil))

func (n *node) Readdir(ctx context.Context) (fusefs.DirStream, syscall.Errno) {
	n.fs.logAndIncrementOpCounter(ctx, fuseOpReaddir, n.Path(nil))

	ents, errno := n.readdir()
	if errno != 0 {
		return nil, errno
	}
	return fusefs.NewListDirStream(ents), 0
}

func (n *node) readdir() ([]fuse.DirEntry, syscall.Errno) {
	// Measure how long node_readdir operation takes (in microseconds).
	start := time.Now() // set start time
	defer commonmetrics.MeasureLatencyInMicroseconds(commonmetrics.NodeReaddir, n.fs.layerDigest, start)

	n.entsMu.Lock()
	if n.entsCached {
		ents := n.ents
		n.entsMu.Unlock()
		return ents, 0
	}
	n.entsMu.Unlock()

	var ents []fuse.DirEntry
	whiteouts := map[string]uint32{}
	normalEnts := map[string]bool{}
	var lastErr error
	if err := n.fs.r.Metadata().ForeachChild(n.id, func(name string, id uint32, mode os.FileMode) bool {

		// We don't want to show whiteouts.
		if strings.HasPrefix(name, whiteoutPrefix) {
			if name == whiteoutOpaqueDir {
				return true
			}
			// Add the overlayfs-compiant whiteout later.
			whiteouts[name] = id
			return true
		}

		// This is a normal entry.
		normalEnts[name] = true
		ino, err := n.fs.inodeOfID(id)
		if err != nil {
			lastErr = err
			return false
		}
		ents = append(ents, fuse.DirEntry{
			Mode: fileModeToSystemMode(mode),
			Name: name,
			Ino:  ino,
		})
		return true
	}); err != nil || lastErr != nil {
		n.fs.reportFailure(fuseOpReaddir, fmt.Errorf("%s: err = %v; lastErr = %v", fuseOpReaddir, err, lastErr))
		return nil, syscall.EIO
	}

	// Append whiteouts if no entry replaces the target entry in the lower layer.
	for w, id := range whiteouts {
		if !normalEnts[w[len(whiteoutPrefix):]] {
			ino, err := n.fs.inodeOfID(id)
			if err != nil {
				n.fs.reportFailure(fuseOpReaddir, fmt.Errorf("%s: err = %v; lastErr = %v", fuseOpReaddir, err, lastErr))
				return nil, syscall.EIO
			}
			ents = append(ents, fuse.DirEntry{
				Mode: syscall.S_IFCHR,
				Name: w[len(whiteoutPrefix):],
				Ino:  ino,
			})

		}
	}

	// Avoid undeterministic order of entries on each call
	sort.Slice(ents, func(i, j int) bool {
		return ents[i].Name < ents[j].Name
	})
	n.entsMu.Lock()
	defer n.entsMu.Unlock()
	n.ents, n.entsCached = ents, true // cache it

	return ents, 0
}

var _ = (fusefs.NodeLookuper)((*node)(nil))

func (n *node) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fusefs.Inode, syscall.Errno) {
	n.fs.logAndIncrementOpCounter(ctx, fuseOpLookup, n.Path(nil))

	isRoot := n.isRootNode()

	// We don't want to show whiteouts.
	if strings.HasPrefix(name, whiteoutPrefix) {
		return nil, syscall.ENOENT
	}

	// state directory
	if isRoot && name == stateDirName {
		return n.NewInode(ctx, n.fs.s, n.fs.stateToAttr(&out.Attr)), 0
	}

	// lookup on memory nodes
	if cn := n.GetChild(name); cn != nil {
		switch tn := cn.Operations().(type) {
		case *node:
			ino, err := n.fs.inodeOfID(tn.id)
			if err != nil {
				n.fs.reportFailure(fuseOpLookup, fmt.Errorf("%s: %v", fuseOpLookup, err))
				return nil, syscall.EIO
			}
			entryToAttr(ino, tn.attr, &out.Attr)
		case *whiteout:
			ino, err := n.fs.inodeOfID(tn.id)
			if err != nil {
				n.fs.reportFailure(fuseOpLookup, fmt.Errorf("%s: %v", fuseOpLookup, err))
				return nil, syscall.EIO
			}
			entryToAttr(ino, tn.attr, &out.Attr)
		default:
			n.fs.reportFailure(fuseOpLookup, fmt.Errorf("%s: unknown node type detected", fuseOpLookup))
			return nil, syscall.EIO
		}
		return cn, 0
	}

	// early return if this entry doesn't exist
	n.entsMu.Lock()
	if n.entsCached {
		var found bool
		for _, e := range n.ents {
			if e.Name == name {
				found = true
			}
		}
		if !found {
			n.entsMu.Unlock()
			return nil, syscall.ENOENT
		}
	}
	n.entsMu.Unlock()

	id, ce, err := n.fs.r.Metadata().GetChild(n.id, name)
	if err != nil {
		// If the entry exists as a whiteout, show an overlayfs-styled whiteout node.
		if whID, wh, err := n.fs.r.Metadata().GetChild(n.id, fmt.Sprintf("%s%s", whiteoutPrefix, name)); err == nil {
			ino, err := n.fs.inodeOfID(whID)
			if err != nil {
				n.fs.reportFailure(fuseOpLookup, fmt.Errorf("%s: %v", fuseOpLookup, err))
				return nil, syscall.EIO
			}
			return n.NewInode(ctx, &whiteout{
				id:   whID,
				fs:   n.fs,
				attr: wh,
			}, entryToWhAttr(ino, wh, &out.Attr)), 0
		}
		n.readdir() // This code path is very expensive. Cache child entries here so that the next call don't reach here.
		return nil, syscall.ENOENT
	}

	ino, err := n.fs.inodeOfID(id)
	if err != nil {
		n.fs.reportFailure(fuseOpLookup, fmt.Errorf("%s: %v", fuseOpLookup, err))
		return nil, syscall.EIO
	}
	return n.NewInode(ctx, &node{
		id:   id,
		fs:   n.fs,
		attr: ce,
	}, entryToAttr(ino, ce, &out.Attr)), 0
}

var _ = (fusefs.NodeOpener)((*node)(nil))

func (n *node) Open(ctx context.Context, flags uint32) (fh fusefs.FileHandle, fuseFlags uint32, errno syscall.Errno) {
	n.fs.logAndIncrementOpCounter(ctx, fuseOpOpen, n.Path(nil))

	ra, err := n.fs.r.OpenFile(n.id)
	if err != nil {
		n.fs.reportFailure(fuseOpOpen, fmt.Errorf("%s: %v", fuseOpOpen, err))
		return nil, 0, syscall.EIO
	}
	return &file{
		n:  n,
		ra: ra,
	}, fuse.FOPEN_KEEP_CACHE, 0
}

var _ = (fusefs.NodeGetattrer)((*node)(nil))

func (n *node) Getattr(ctx context.Context, f fusefs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	n.fs.logAndIncrementOpCounter(ctx, fuseOpGetattr, n.Path(nil))

	ino, err := n.fs.inodeOfID(n.id)
	if err != nil {
		n.fs.reportFailure(fuseOpGetattr, fmt.Errorf("%s: %v", fuseOpGetattr, err))
		return syscall.EIO
	}
	entryToAttr(ino, n.attr, &out.Attr)
	return 0
}

var _ = (fusefs.NodeGetxattrer)((*node)(nil))

func (n *node) Getxattr(ctx context.Context, attr string, dest []byte) (uint32, syscall.Errno) {
	n.fs.logAndIncrementOpCounter(ctx, fuseOpGetxattr, n.Path(nil))

	ent := n.attr
	opq := n.isOpaque()
	for _, opaqueXattr := range n.fs.opaqueXattrs {
		if attr == opaqueXattr && opq {
			// This node is an opaque directory so give overlayfs-compliant indicator.
			if len(dest) < len(opaqueXattrValue) {
				return uint32(len(opaqueXattrValue)), syscall.ERANGE
			}
			return uint32(copy(dest, opaqueXattrValue)), 0
		}
	}
	if v, ok := ent.Xattrs[attr]; ok {
		if len(dest) < len(v) {
			return uint32(len(v)), syscall.ERANGE
		}
		return uint32(copy(dest, v)), 0
	}
	return 0, syscall.ENODATA
}

var _ = (fusefs.NodeListxattrer)((*node)(nil))

func (n *node) Listxattr(ctx context.Context, dest []byte) (uint32, syscall.Errno) {
	n.fs.logAndIncrementOpCounter(ctx, fuseOpListxattr, n.Path(nil))

	ent := n.attr
	opq := n.isOpaque()
	var attrs []byte
	if opq {
		// This node is an opaque directory so add overlayfs-compliant indicator.
		for _, opaqueXattr := range n.fs.opaqueXattrs {
			attrs = append(attrs, []byte(opaqueXattr+"\x00")...)
		}
	}
	for k := range ent.Xattrs {
		attrs = append(attrs, []byte(k+"\x00")...)
	}
	if len(dest) < len(attrs) {
		return uint32(len(attrs)), syscall.ERANGE
	}
	return uint32(copy(dest, attrs)), 0
}

var _ = (fusefs.NodeReadlinker)((*node)(nil))

func (n *node) Readlink(ctx context.Context) ([]byte, syscall.Errno) {
	n.fs.logAndIncrementOpCounter(ctx, fuseOpReadLink, n.Path(nil))

	ent := n.attr
	return []byte(ent.LinkName), 0
}

var _ = (fusefs.NodeStatfser)((*node)(nil))

func (n *node) Statfs(ctx context.Context, out *fuse.StatfsOut) syscall.Errno {
	defaultStatfs(out)
	return 0
}

// file is a file abstraction which implements file handle in go-fuse.
type file struct {
	n  *node
	ra io.ReaderAt
}

var _ = (fusefs.FileReader)((*file)(nil))

func (f *file) Read(ctx context.Context, dest []byte, off int64) (fuse.ReadResult, syscall.Errno) {
	f.n.fs.logAndIncrementOpCounter(ctx, fuseOpFileRead, f.n.Path(nil))

	defer commonmetrics.MeasureLatencyInMicroseconds(commonmetrics.SynchronousRead, f.n.fs.layerDigest, time.Now()) // measure time for synchronous file reads (in microseconds)
	defer commonmetrics.IncOperationCount(commonmetrics.SynchronousReadCount, f.n.fs.layerDigest)                   // increment the counter for synchronous file reads
	n, err := f.ra.ReadAt(dest, off)
	if err != nil && err != io.EOF {
		f.n.fs.reportFailure(fuseOpFileRead, fmt.Errorf("%s: %v", fuseOpFileRead, err))
		return nil, syscall.EIO
	}
	return fuse.ReadResultData(dest[:n]), 0
}

var _ = (fusefs.FileGetattrer)((*file)(nil))

func (f *file) Getattr(ctx context.Context, out *fuse.AttrOut) syscall.Errno {
	f.n.fs.logAndIncrementOpCounter(ctx, fuseOpFileGetattr, f.n.Path(nil))

	ino, err := f.n.fs.inodeOfID(f.n.id)
	if err != nil {
		f.n.fs.reportFailure(fuseOpFileGetattr, fmt.Errorf("%s: %v", fuseOpFileGetattr, err))
		return syscall.EIO
	}
	entryToAttr(ino, f.n.attr, &out.Attr)
	return 0
}

// whiteout is a whiteout abstraction compliant to overlayfs.
type whiteout struct {
	fusefs.Inode
	id   uint32
	fs   *fs
	attr metadata.Attr
}

var _ = (fusefs.NodeGetattrer)((*whiteout)(nil))

func (w *whiteout) Getattr(ctx context.Context, f fusefs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	w.fs.logAndIncrementOpCounter(ctx, fuseOpWhiteoutGetattr, w.Path(nil))

	ino, err := w.fs.inodeOfID(w.id)
	if err != nil {
		w.fs.reportFailure(fuseOpWhiteoutGetattr, fmt.Errorf("%s: %v", fuseOpWhiteoutGetattr, err))
		return syscall.EIO
	}
	entryToWhAttr(ino, w.attr, &out.Attr)
	return 0
}

var _ = (fusefs.NodeStatfser)((*whiteout)(nil))

func (w *whiteout) Statfs(ctx context.Context, out *fuse.StatfsOut) syscall.Errno {
	defaultStatfs(out)
	return 0
}

// newState provides new state directory node.
// It creates statFile at the same time to give it stable inode number.
func (fs *fs) newState(layerDigest digest.Digest, blob remote.Blob) *state {
	return &state{
		statFile: &statFile{
			name: layerDigest.String() + ".json",
			statJSON: statJSON{
				Digest: layerDigest.String(),
				Size:   blob.Size(),
			},
			blob: blob,
			fs:   fs,
		},
		fs: fs,
	}
}

// state is a directory which contain a "state file" of this layer aiming to
// observability. This filesystem uses it to report something(e.g. error) to
// the clients(e.g. Kubernetes's livenessProbe).
// This directory has mode "dr-x------ root root".
type state struct {
	fusefs.Inode
	statFile *statFile
	fs       *fs
}

var _ = (fusefs.NodeReaddirer)((*state)(nil))

func (s *state) Readdir(ctx context.Context) (fusefs.DirStream, syscall.Errno) {
	return fusefs.NewListDirStream([]fuse.DirEntry{
		{
			Mode: statFileMode,
			Name: s.statFile.name,
			Ino:  s.fs.inodeOfStatFile(),
		},
	}), 0
}

var _ = (fusefs.NodeLookuper)((*state)(nil))

func (s *state) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fusefs.Inode, syscall.Errno) {
	if name != s.statFile.name {
		return nil, syscall.ENOENT
	}
	attr, errno := s.statFile.attr(&out.Attr)
	if errno != 0 {
		return nil, errno
	}
	return s.NewInode(ctx, s.statFile, attr), 0
}

var _ = (fusefs.NodeGetattrer)((*state)(nil))

func (s *state) Getattr(ctx context.Context, f fusefs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	s.fs.stateToAttr(&out.Attr)
	return 0
}

var _ = (fusefs.NodeStatfser)((*state)(nil))

func (s *state) Statfs(ctx context.Context, out *fuse.StatfsOut) syscall.Errno {
	defaultStatfs(out)
	return 0
}

func (s *state) report(err error) {
	s.statFile.report(err)
}

type statJSON struct {
	Error  string `json:"error,omitempty"`
	Digest string `json:"digest"`
	// URL is excluded for potential security reason
	Size           int64   `json:"size"`
	FetchedSize    int64   `json:"fetchedSize"`
	FetchedPercent float64 `json:"fetchedPercent"` // Fetched / Size * 100.0
}

// statFile is a file which contain something to be reported from this layer.
// This filesystem uses statFile.report() to report something(e.g. error) to
// the clients(e.g. Kubernetes's livenessProbe).
// This file has mode "-r-------- root root".
type statFile struct {
	fusefs.Inode
	name     string
	blob     remote.Blob
	statJSON statJSON
	mu       sync.Mutex
	fs       *fs
}

var _ = (fusefs.NodeOpener)((*statFile)(nil))

func (sf *statFile) Open(ctx context.Context, flags uint32) (fh fusefs.FileHandle, fuseFlags uint32, errno syscall.Errno) {
	return nil, 0, 0
}

var _ = (fusefs.NodeReader)((*statFile)(nil))

func (sf *statFile) Read(ctx context.Context, f fusefs.FileHandle, dest []byte, off int64) (fuse.ReadResult, syscall.Errno) {
	sf.mu.Lock()
	defer sf.mu.Unlock()
	st, err := sf.updateStatUnlocked()
	if err != nil {
		return nil, syscall.EIO
	}
	n, err := bytes.NewReader(st).ReadAt(dest, off)
	if err != nil && err != io.EOF {
		return nil, syscall.EIO
	}
	return fuse.ReadResultData(dest[:n]), 0
}

var _ = (fusefs.NodeGetattrer)((*statFile)(nil))

func (sf *statFile) Getattr(ctx context.Context, f fusefs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	_, errno := sf.attr(&out.Attr)
	return errno
}

var _ = (fusefs.NodeStatfser)((*statFile)(nil))

func (sf *statFile) Statfs(ctx context.Context, out *fuse.StatfsOut) syscall.Errno {
	defaultStatfs(out)
	return 0
}

// logContents puts the contents of statFile in the log
// to keep that information accessible for troubleshooting.
// The entries naming is kept to be consistend with the field naming in statJSON.
func (sf *statFile) logContents() {
	ctx := context.Background()
	log.G(ctx).WithFields(logrus.Fields{
		"digest": sf.statJSON.Digest, "size": sf.statJSON.Size,
		"fetchedSize": sf.statJSON.FetchedSize, "fetchedPercent": sf.statJSON.FetchedPercent,
	}).WithError(errors.New(sf.statJSON.Error)).Error("statFile error")
}

func (sf *statFile) report(err error) {
	sf.mu.Lock()
	defer sf.mu.Unlock()
	sf.statJSON.Error = err.Error()
	sf.logContents()
}

func (sf *statFile) attr(out *fuse.Attr) (fusefs.StableAttr, syscall.Errno) {
	sf.mu.Lock()
	defer sf.mu.Unlock()

	st, err := sf.updateStatUnlocked()
	if err != nil {
		return fusefs.StableAttr{}, syscall.EIO
	}

	return sf.fs.statFileToAttr(uint64(len(st)), out), 0
}

func (sf *statFile) updateStatUnlocked() ([]byte, error) {
	sf.statJSON.FetchedSize = sf.blob.FetchedSize()
	sf.statJSON.FetchedPercent = float64(sf.statJSON.FetchedSize) / float64(sf.statJSON.Size) * 100.0
	j, err := json.Marshal(&sf.statJSON)
	if err != nil {
		return nil, err
	}
	j = append(j, []byte("\n")...)
	return j, nil
}

// entryToAttr converts metadata.Attr to go-fuse's Attr.
func entryToAttr(ino uint64, e metadata.Attr, out *fuse.Attr) fusefs.StableAttr {
	out.Ino = ino
	out.Size = uint64(e.Size)
	if e.Mode&os.ModeSymlink != 0 {
		out.Size = uint64(len(e.LinkName))
	}
	out.Blksize = blockSize
	out.Blocks = (out.Size + blockSize - 1) / blockSize * physicalBlockRatio
	mtime := e.ModTime
	out.SetTimes(nil, &mtime, nil)
	out.Mode = fileModeToSystemMode(e.Mode)
	out.Owner = fuse.Owner{Uid: uint32(e.UID), Gid: uint32(e.GID)}
	out.Rdev = uint32(unix.Mkdev(uint32(e.DevMajor), uint32(e.DevMinor)))
	out.Nlink = uint32(e.NumLink)
	if out.Nlink == 0 {
		out.Nlink = 1 // zero "NumLink" means one.
	}
	out.Padding = 0 // TODO

	return fusefs.StableAttr{
		Mode: out.Mode,
		Ino:  out.Ino,
		// NOTE: The inode number is unique throughout the lifetime of
		// this filesystem so we don't consider about generation at this
		// moment.
	}
}

// entryToWhAttr converts metadata.Attr to go-fuse's Attr of whiteouts.
func entryToWhAttr(ino uint64, e metadata.Attr, out *fuse.Attr) fusefs.StableAttr {
	out.Ino = ino
	out.Size = 0
	out.Blksize = blockSize
	out.Blocks = 0
	mtime := e.ModTime
	out.SetTimes(nil, &mtime, nil)
	out.Mode = syscall.S_IFCHR
	out.Owner = fuse.Owner{Uid: 0, Gid: 0}
	out.Rdev = uint32(unix.Mkdev(0, 0))
	out.Nlink = 1
	out.Padding = 0 // TODO

	return fusefs.StableAttr{
		Mode: out.Mode,
		Ino:  out.Ino,
		// NOTE: The inode number is unique throughout the lifetime of
		// this filesystem so we don't consider about generation at this
		// moment.
	}
}

// stateToAttr converts state directory to go-fuse's Attr.
func (fs *fs) stateToAttr(out *fuse.Attr) fusefs.StableAttr {
	out.Ino = fs.inodeOfState()
	out.Size = 0
	out.Blksize = blockSize
	out.Blocks = 0
	out.Nlink = 1

	// root can read and open it (dr-x------ root root).
	out.Mode = stateDirMode
	out.Owner = fuse.Owner{Uid: 0, Gid: 0}

	// dummy
	out.Mtime = 0
	out.Mtimensec = 0
	out.Rdev = 0
	out.Padding = 0

	return fusefs.StableAttr{
		Mode: out.Mode,
		Ino:  out.Ino,
		// NOTE: The inode number is unique throughout the lifetime of
		// this filesystem so we don't consider about generation at this
		// moment.
	}
}

// statFileToAttr converts stat file to go-fuse's Attr.
// func statFileToAttr(id uint64, sf *statFile, size uint64, out *fuse.Attr) fusefs.StableAttr {
func (fs *fs) statFileToAttr(size uint64, out *fuse.Attr) fusefs.StableAttr {
	out.Ino = fs.inodeOfStatFile()
	out.Size = size
	out.Blksize = blockSize
	out.Blocks = (out.Size + blockSize - 1) / blockSize * physicalBlockRatio
	out.Nlink = 1

	// Root can read it ("-r-------- root root").
	out.Mode = statFileMode
	out.Owner = fuse.Owner{Uid: 0, Gid: 0}

	// dummy
	out.Mtime = 0
	out.Mtimensec = 0
	out.Rdev = 0
	out.Padding = 0

	return fusefs.StableAttr{
		Mode: out.Mode,
		Ino:  out.Ino,
		// NOTE: The inode number is unique throughout the lifetime of
		// this filesystem so we don't consider about generation at this
		// moment.
	}
}

func fileModeToSystemMode(m os.FileMode) uint32 {
	// Permission bits
	res := uint32(m & os.ModePerm)

	// File type bits
	switch m & os.ModeType {
	case os.ModeDevice:
		res |= syscall.S_IFBLK
	case os.ModeDevice | os.ModeCharDevice:
		res |= syscall.S_IFCHR
	case os.ModeDir:
		res |= syscall.S_IFDIR
	case os.ModeNamedPipe:
		res |= syscall.S_IFIFO
	case os.ModeSymlink:
		res |= syscall.S_IFLNK
	case os.ModeSocket:
		res |= syscall.S_IFSOCK
	default: // regular file.
		res |= syscall.S_IFREG
	}

	// suid, sgid, sticky bits
	if m&os.ModeSetuid != 0 {
		res |= syscall.S_ISUID
	}
	if m&os.ModeSetgid != 0 {
		res |= syscall.S_ISGID
	}
	if m&os.ModeSticky != 0 {
		res |= syscall.S_ISVTX
	}

	return res
}

func defaultStatfs(stat *fuse.StatfsOut) {

	// http://man7.org/linux/man-pages/man2/statfs.2.html
	stat.Blocks = 0 // dummy
	stat.Bfree = 0
	stat.Bavail = 0
	stat.Files = 0 // dummy
	stat.Ffree = 0
	stat.Bsize = blockSize
	stat.NameLen = 1<<32 - 1
	stat.Frsize = blockSize
	stat.Padding = 0
	stat.Spare = [6]uint32{}
}
