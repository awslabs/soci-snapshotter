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

package metadata

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/awslabs/soci-snapshotter/ztoc"
	"github.com/awslabs/soci-snapshotter/ztoc/compression"
	"github.com/rs/xid"
	bolt "go.etcd.io/bbolt"
	errbbolt "go.etcd.io/bbolt/errors"
	"golang.org/x/sync/errgroup"
)

const (
	// bboltInsertionChunkSize roughly determines the maximum number of insertions to
	// bbolt per transaction. We chose 5K to be the chunk size as it yielded the best
	// balance between performance and memory usage compared to other chunk sizes (1k and 10k)
	// that were benchmarked.
	bboltInsertionChunkSize = 5000
)

// reader stores filesystem metadata parsed from a TOC to metadata DB
// and provides methods to read them.
type reader struct {
	db     *bolt.DB
	fsID   string
	rootID uint32
	sr     *io.SectionReader

	curID   uint32
	curIDMu sync.Mutex
	initG   *errgroup.Group
}

func (r *reader) nextID() (uint32, error) {
	r.curIDMu.Lock()
	defer r.curIDMu.Unlock()
	if r.curID == math.MaxUint32 {
		return 0, fmt.Errorf("sequence id too large")
	}
	r.curID++
	return r.curID, nil
}

// NewReader parses a TOC and persists filesystem metadata to the provided bbolt DB.
func NewReader(db *bolt.DB, sr *io.SectionReader, toc ztoc.TOC, opts ...Option) (Reader, error) {
	var rOpts Options
	for _, o := range opts {
		if err := o(&rOpts); err != nil {
			return nil, fmt.Errorf("failed to apply option: %w", err)
		}
	}

	r := &reader{sr: sr, db: db, initG: new(errgroup.Group)}
	start := time.Now()
	if rOpts.Telemetry != nil && rOpts.Telemetry.InitMetadataStoreLatency != nil {
		rOpts.Telemetry.InitMetadataStoreLatency(start)
	}

	if err := r.init(toc, rOpts); err != nil {
		return nil, fmt.Errorf("failed to initialize metadata: %w", err)
	}
	return r, nil
}

func (r *reader) init(toc ztoc.TOC, rOpts Options) (retErr error) {
	// Initialize root node
	var ok bool
	for i := 0; i < 100; i++ {
		fsID := xid.New().String()
		if err := r.initRootNode(fsID); err != nil {
			if errors.Is(err, errbbolt.ErrBucketExists) {
				continue // try with another id
			}
			return fmt.Errorf("failed to initialize root node %q: %w", fsID, err)
		}
		ok = true
		break
	}
	if !ok {
		return fmt.Errorf("failed to get a unique id for metadata reader")
	}

	return r.initNodes(toc)
}

func (r *reader) initRootNode(fsID string) error {
	return r.db.Batch(func(tx *bolt.Tx) (err error) {
		filesystems, err := tx.CreateBucketIfNotExists(bucketKeyFilesystems)
		if err != nil {
			return err
		}
		lbkt, err := filesystems.CreateBucket([]byte(fsID))
		if err != nil {
			return err
		}
		r.fsID = fsID
		if _, err := lbkt.CreateBucket(bucketKeyMetadata); err != nil {
			return err
		}
		nodes, err := lbkt.CreateBucket(bucketKeyNodes)
		if err != nil {
			return err
		}
		rootID, err := r.nextID()
		if err != nil {
			return err
		}
		rootBucket, err := nodes.CreateBucket(encodeID(rootID))
		if err != nil {
			return err
		}
		if err := writeNodeEntry(rootBucket, &Attr{
			Mode:    os.ModeDir | 0755,
			NumLink: 2, // The directory itself(.) and the parent link to this directory.
		}); err != nil {
			return err
		}
		r.rootID = rootID
		return err
	})
}

func (r *reader) initNodes(toc ztoc.TOC) error {
	fileMetadataChunks := partition(toc.FileMetadata, bboltInsertionChunkSize)

	md := make(map[uint32]*metadataEntry)
	for _, fileMetadataChunk := range fileMetadataChunks {
		if err := r.db.Batch(func(tx *bolt.Tx) (err error) {
			nodes, err := getNodesBucket(tx, r.fsID)
			if err != nil {
				return err
			}
			nodes.FillPercent = 1.0 // we only do sequential write to this bucket
			var attr Attr
			for _, ent := range fileMetadataChunk {
				var id uint32
				var b *bolt.Bucket

				// TAR stores trailing path separator for directory entries. We clean
				// the path to remove the trailing separator, so that we can recurse
				// parent paths using `filepath.Split`.
				cleanName := cleanEntryPath(ent.Name)
				cleanLinkName := cleanEntryPath(ent.Linkname)

				isLink := ent.Type == "hardlink"
				isDir := ent.Type == "dir"

				if isLink {
					id, err = getIDByName(md, cleanLinkName, r.rootID)
					if err != nil {
						return fmt.Errorf("%q is a hardlink but cannot get link destination %q: %w", ent.Name, ent.Linkname, err)
					}
					b, err = getNodeBucketByID(nodes, id)
					if err != nil {
						return fmt.Errorf("cannot get hardlink destination %q ==> %q (%d): %w", ent.Name, ent.Linkname, id, err)
					}
					numLink, _ := binary.Varint(b.Get(bucketKeyNumLink))
					if err := putInt(b, bucketKeyNumLink, numLink+1); err != nil {
						return fmt.Errorf("cannot put NumLink of %q ==> %q: %w", ent.Name, ent.Linkname, err)
					}
				} else {
					// Write node bucket
					var found bool
					if isDir {
						// Check if this directory is already created, if so overwrite it.
						id, err = getIDByName(md, cleanName, r.rootID)
						if err == nil {
							b, err = getNodeBucketByID(nodes, id)
							if err != nil {
								return fmt.Errorf("failed to get directory bucket %d: %w", id, err)
							}
							found = true
							attr.NumLink = readNumLink(b)
						}
					}
					if !found {
						// No existing node. Create a new one.
						id, err = r.nextID()
						if err != nil {
							return err
						}
						b, err = nodes.CreateBucket(encodeID(id))
						if err != nil {
							return err
						}
						attr.NumLink = 1 // at least the parent dir references this directory.
						if isDir {
							attr.NumLink++ // at least "." references this directory.
						}
					}
					// Write the new node object to the node bucket.
					if err := writeNodeEntry(b, attrFromZtocEntry(&ent, &attr)); err != nil {
						return fmt.Errorf("failed to set attr to %d(%q): %w", id, ent.Name, err)
					}
				}

				// Create relationship between node and its parent.
				parentDirectoryName := parentDir(cleanName)
				parentID, parentBucket, err := r.getOrCreateDir(nodes, md, parentDirectoryName, r.rootID)
				if err != nil {
					return fmt.Errorf("failed to create parent directory %q of %q: %w", parentDirectoryName, ent.Name, err)
				}
				if err := setChild(md, parentBucket, parentID, filepath.Base(cleanName), id, isDir); err != nil {
					return err
				}

				if !isLink {
					if md[id] == nil {
						md[id] = &metadataEntry{}
					}
					md[id].TarName = ent.Name
					md[id].UncompressedOffset = ent.UncompressedOffset
					md[id].TarHeaderOffset = ent.TarHeaderOffset
					md[id].TarHeaderSize = ent.UncompressedOffset - ent.TarHeaderOffset
				}
			}
			return nil
		}); err != nil {
			return err
		}
	}

	addendum := make([]struct {
		id []byte
		md *metadataEntry
	}, len(md))
	i := 0
	for id, d := range md {
		addendum[i].id, addendum[i].md = encodeID(id), d
		i++
	}
	// Sort metadata entries by node ID.
	sort.Slice(addendum, func(i, j int) bool {
		return bytes.Compare(addendum[i].id, addendum[j].id) < 0
	})

	metadataChunks := partition(addendum, bboltInsertionChunkSize)
	for _, metadataChunk := range metadataChunks {
		if err := r.db.Batch(func(tx *bolt.Tx) (err error) {
			meta, err := getMetadataBucket(tx, r.fsID)
			if err != nil {
				return err
			}
			meta.FillPercent = 1.0 // we only do sequential write to this bucket
			for _, m := range metadataChunk {
				md, err := meta.CreateBucket(m.id)
				if err != nil {
					return err
				}
				if err := writeMetadataEntry(md, m.md); err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			return err
		}

	}
	return nil
}

// getIDByName returns the node ID associated with a path (file or directory). It recurses
// the path all the way back to the root node, returning every child ID along the way.
func getIDByName(md map[uint32]*metadataEntry, path string, rootID uint32) (uint32, error) {
	if path == "" {
		return rootID, nil
	}
	parentDirectory, base := filepath.Split(cleanEntryPath(path))
	parentID, err := getIDByName(md, parentDirectory, rootID)
	if err != nil {
		return 0, err
	}
	if md[parentID] == nil {
		return 0, fmt.Errorf("not found metadata of %d", parentID)
	}
	if md[parentID].children == nil {
		return 0, fmt.Errorf("not found children of %q", parentID)
	}
	c, ok := md[parentID].children[base]
	if !ok {
		return 0, fmt.Errorf("not found child %q in %d", base, parentID)
	}
	return c.id, nil
}

// getOrCreateDir either retrieves a directory by name if it exists in the nodes bucket or creates
// one and adds itself to the children map of its parent. If the parent has not yet been created,
// it recurses the path until it finds a parent that exists, creating parent->child relationships
// along the way.
func (r *reader) getOrCreateDir(nodes *bolt.Bucket, md map[uint32]*metadataEntry, dir string, rootID uint32) (id uint32, b *bolt.Bucket, err error) {
	id, err = getIDByName(md, dir, rootID)
	// Directory exists, return the node ID and bucket
	if err == nil {
		b, err = getNodeBucketByID(nodes, id)
		if err != nil {
			return 0, nil, fmt.Errorf("failed to get dir bucket %d: %w", id, err)
		}
		return id, b, nil
	}
	id, err = r.nextID()
	if err != nil {
		return 0, nil, err
	}
	b, err = nodes.CreateBucket(encodeID(id))
	if err != nil {
		return 0, nil, err
	}
	attr := &Attr{
		Mode:    os.ModeDir | 0755,
		NumLink: 2, // The directory itself(.) and the parent link to this directory.
	}
	if err := writeNodeEntry(b, attr); err != nil {
		return 0, nil, err
	}

	parentID, parentBucket, err := r.getOrCreateDir(nodes, md, parentDir(cleanEntryPath(dir)), rootID)
	if err != nil {
		return 0, nil, err
	}
	if err := setChild(md, parentBucket, parentID, filepath.Base(dir), id, true); err != nil {
		return 0, nil, err
	}
	return id, b, nil
}

// setChild adds a child identified by base name to its parents children map as well
// as incrementing the link count of the parent if the child is a directory.
func setChild(md map[uint32]*metadataEntry, parentBucket *bolt.Bucket, parentID uint32, base string, id uint32, isDir bool) error {
	if md[parentID] == nil {
		md[parentID] = &metadataEntry{}
	}
	if md[parentID].children == nil {
		md[parentID].children = make(map[string]childEntry)
	}
	md[parentID].children[base] = childEntry{base, id}
	if isDir {
		numLink, _ := binary.Varint(parentBucket.Get(bucketKeyNumLink))
		if err := putInt(parentBucket, bucketKeyNumLink, numLink+1); err != nil {
			return fmt.Errorf("cannot add numlink for children: %w", err)
		}
	}
	return nil
}

// cleanEntryPath returns a clean file path, with trailing path separators removed.
func cleanEntryPath(path string) string {
	return strings.TrimPrefix(filepath.Clean(string(os.PathSeparator)+path), string(os.PathSeparator))
}

// parentDir returns the parent directory of a path.
func parentDir(path string) string {
	parentDirectory, _ := filepath.Split(path)
	return parentDirectory
}

// partition partitions a slice of type T into chunks of size chunkSize.
//
// If the slice is empty or if the chunkSize is <= 0, an empty slice is
// returned.
func partition[T any](data []T, chunkSize int) [][]T {
	if len(data) == 0 || chunkSize <= 0 {
		return [][]T{}
	}
	chunks := make([][]T, 0, (len(data)+chunkSize-1)/chunkSize)
	for chunkSize < len(data) {
		data, chunks = data[chunkSize:], append(chunks, data[0:chunkSize:chunkSize])
	}
	chunks = append(chunks, data)
	return chunks
}

func (r *reader) waitInit() error {
	// TODO: add timeout
	err := r.initG.Wait()
	if err != nil {
		return fmt.Errorf("initialization failed: %w", err)
	}
	return nil
}

func (r *reader) view(fn func(tx *bolt.Tx) error) error {
	if err := r.waitInit(); err != nil {
		return err
	}
	return r.db.View(func(tx *bolt.Tx) error {
		return fn(tx)
	})
}

func (r *reader) update(fn func(tx *bolt.Tx) error) error {
	if err := r.waitInit(); err != nil {
		return err
	}
	return r.db.Batch(func(tx *bolt.Tx) error {
		return fn(tx)
	})
}

// RootID returns ID of the root node.
func (r *reader) RootID() uint32 {
	return r.rootID
}

// Clone returns a new reader identical to the current reader
// but uses the provided section reader for retrieving file paylaods.
func (r *reader) Clone(sr *io.SectionReader) (Reader, error) {
	if err := r.waitInit(); err != nil {
		return nil, err
	}
	return &reader{
		db:     r.db,
		fsID:   r.fsID,
		rootID: r.rootID,
		sr:     sr,
		initG:  new(errgroup.Group),
	}, nil
}

// Close closes this reader. This removes underlying filesystem metadata as well.
func (r *reader) Close() error {
	return r.update(func(tx *bolt.Tx) (err error) {
		filesystems := tx.Bucket(bucketKeyFilesystems)
		if filesystems == nil {
			return nil
		}
		return filesystems.DeleteBucket([]byte(r.fsID))
	})
}

// GetAttr returns file attribute of specified node.
func (r *reader) GetAttr(id uint32) (attr Attr, _ error) {
	if r.rootID == id { // no need to wait for root dir
		if err := r.db.View(func(tx *bolt.Tx) error {
			nodes, err := getNodesBucket(tx, r.fsID)
			if err != nil {
				return fmt.Errorf("nodes bucket of %q not found for sarching attr %d: %w", r.fsID, id, err)
			}
			b, err := getNodeBucketByID(nodes, id)
			if err != nil {
				return fmt.Errorf("failed to get attr bucket %d: %w", id, err)
			}
			return readNodeEntryToAttr(b, &attr)
		}); err != nil {
			return Attr{}, err
		}
		return attr, nil
	}
	if err := r.view(func(tx *bolt.Tx) error {
		nodes, err := getNodesBucket(tx, r.fsID)
		if err != nil {
			return fmt.Errorf("nodes bucket of %q not found for sarching attr %d: %w", r.fsID, id, err)
		}
		b, err := getNodeBucketByID(nodes, id)
		if err != nil {
			return fmt.Errorf("failed to get attr bucket %d: %w", id, err)
		}
		return readNodeEntryToAttr(b, &attr)
	}); err != nil {
		return Attr{}, err
	}
	return
}

// GetChild returns a child node that has the specified base name.
func (r *reader) GetChild(pid uint32, base string) (id uint32, attr Attr, _ error) {
	if err := r.view(func(tx *bolt.Tx) error {
		metadataEntries, err := getMetadataBucket(tx, r.fsID)
		if err != nil {
			return fmt.Errorf("metadata bucket of %q not found for getting child of %d: %w", r.fsID, pid, err)
		}
		md, err := getMetadataBucketByID(metadataEntries, pid)
		if err != nil {
			return fmt.Errorf("failed to get parent metadata %d: %w", pid, err)
		}
		id, err = readChild(md, base)
		if err != nil {
			return fmt.Errorf("failed to read child %q of %d: %w", base, pid, err)
		}
		nodes, err := getNodesBucket(tx, r.fsID)
		if err != nil {
			return fmt.Errorf("nodes bucket of %q not found for getting child of %d: %w", r.fsID, pid, err)
		}
		child, err := getNodeBucketByID(nodes, id)
		if err != nil {
			return fmt.Errorf("failed to get child bucket %d: %w", id, err)
		}
		return readNodeEntryToAttr(child, &attr)
	}); err != nil {
		return 0, Attr{}, err
	}
	return
}

// ForeachChild calls the specified callback function for each child node.
// When the callback returns non-nil error, this stops the iteration.
func (r *reader) ForeachChild(id uint32, f func(name string, id uint32, mode os.FileMode) bool) error {
	type childInfo struct {
		id   uint32
		mode os.FileMode
	}
	children := make(map[string]childInfo)
	if err := r.view(func(tx *bolt.Tx) error {
		metadataEntries, err := getMetadataBucket(tx, r.fsID)
		if err != nil {
			return fmt.Errorf("nodes bucket of %q not found for getting child of %d: %w", r.fsID, id, err)
		}
		md, err := getMetadataBucketByID(metadataEntries, id)
		if err != nil {
			return nil // no child
		}

		var nodes *bolt.Bucket
		firstName := md.Get(bucketKeyChildName)
		if len(firstName) != 0 {
			firstID := decodeID(md.Get(bucketKeyChildID))
			if nodes == nil {
				nodes, err = getNodesBucket(tx, r.fsID)
				if err != nil {
					return fmt.Errorf("nodes bucket of %q not found for getting children of %d: %w", r.fsID, id, err)
				}
			}
			firstChild, err := getNodeBucketByID(nodes, firstID)
			if err != nil {
				return fmt.Errorf("failed to get first child bucket %d: %w", firstID, err)
			}
			mode, _ := binary.Uvarint(firstChild.Get(bucketKeyMode))
			children[string(firstName)] = childInfo{firstID, os.FileMode(uint32(mode))}
		}

		cbkt := md.Bucket(bucketKeyChildrenExtra)
		if cbkt == nil {
			return nil // no child
		}
		if nodes == nil {
			nodes, err = getNodesBucket(tx, r.fsID)
			if err != nil {
				return fmt.Errorf("nodes bucket of %q not found for getting children of %d: %w", r.fsID, id, err)
			}
		}
		return cbkt.ForEach(func(k, v []byte) error {
			id := decodeID(v)
			child, err := getNodeBucketByID(nodes, id)
			if err != nil {
				return fmt.Errorf("failed to get child bucket %d: %w", id, err)
			}
			mode, _ := binary.Uvarint(child.Get(bucketKeyMode))
			children[string(k)] = childInfo{id, os.FileMode(uint32(mode))}
			return nil
		})
	}); err != nil {
		return err
	}
	for k, e := range children {
		if !f(k, e.id, e.mode) {
			break
		}
	}
	return nil
}

// OpenFile returns a section reader of the specified node.
func (r *reader) OpenFile(id uint32) (File, error) {
	var size int64
	var mde metadataEntry

	if err := r.view(func(tx *bolt.Tx) error {
		nodes, err := getNodesBucket(tx, r.fsID)
		if err != nil {
			return fmt.Errorf("nodes bucket of %q not found for opening %d: %w", r.fsID, id, err)
		}
		b, err := getNodeBucketByID(nodes, id)
		if err != nil {
			return fmt.Errorf("failed to get file bucket %d: %w", id, err)
		}
		size, _ = binary.Varint(b.Get(bucketKeySize))
		m, _ := binary.Uvarint(b.Get(bucketKeyMode))
		if !os.FileMode(uint32(m)).IsRegular() {
			return fmt.Errorf("%q is not a regular file", id)
		}
		metadataEntries, err := getMetadataBucket(tx, r.fsID)
		if err != nil {
			return fmt.Errorf("metadata bucket of %q not found for opening %d: %w", r.fsID, id, err)
		}
		if md, err := getMetadataBucketByID(metadataEntries, id); err == nil {
			mde = getMetadataEntry(md)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return &file{mde.TarName, mde.UncompressedOffset, compression.Offset(size), mde.TarHeaderOffset, mde.TarHeaderSize}, nil
}

type file struct {
	tarName            string
	uncompressedOffset compression.Offset
	uncompressedSize   compression.Offset
	tarHeaderOffset    compression.Offset
	tarHeaderSize      compression.Offset
}

func (fr *file) GetUncompressedFileSize() compression.Offset {
	return fr.uncompressedSize
}

func (fr *file) GetUncompressedOffset() compression.Offset {
	return fr.uncompressedOffset
}

func (fr *file) TarName() string {
	return fr.tarName
}
func (fr *file) TarHeaderOffset() compression.Offset {
	return fr.tarHeaderOffset
}

func (fr *file) TarHeaderSize() compression.Offset {
	return fr.tarHeaderSize
}

func attrFromZtocEntry(src *ztoc.FileMetadata, dst *Attr) *Attr {
	dst.Size = int64(src.UncompressedSize)
	dst.ModTime = src.ModTime
	dst.LinkName = src.Linkname
	dst.Mode = src.FileMode()
	dst.UID = src.UID
	dst.GID = src.GID
	dst.DevMajor = int(src.Devmajor)
	dst.DevMinor = int(src.Devminor)
	xattrs := make(map[string][]byte)
	for k, v := range src.Xattrs() {
		xattrs[k] = []byte(v)
	}
	dst.Xattrs = xattrs
	return dst
}

func (r *reader) NumOfNodes() (i int, _ error) {
	if err := r.view(func(tx *bolt.Tx) error {
		nodes, err := getNodesBucket(tx, r.fsID)
		if err != nil {
			return err
		}
		return nodes.ForEach(func(k, v []byte) error {
			b := nodes.Bucket(k)
			if b == nil {
				return fmt.Errorf("entry bucket for %q not found", string(k))
			}
			var attr Attr
			if err := readNodeEntryToAttr(b, &attr); err != nil {
				return err
			}
			i++
			return nil
		})
	}); err != nil {
		return 0, err
	}
	return
}
