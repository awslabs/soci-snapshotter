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
	"fmt"
	"os"
	"sort"

	"github.com/awslabs/soci-snapshotter/util/dbutil"
	"github.com/awslabs/soci-snapshotter/ztoc/compression"
	bolt "go.etcd.io/bbolt"
)

// Metadata package stores filesystem metadata in the following schema.
//
// - filesystems
//   - *filesystem id*                      : bucket for each filesystem keyed by a unique string.
//     - nodes
//       - *node id*                        : bucket for each node keyed by a uniqe uint64.
//         - size : <varint>                : size of the regular node.
//         - modtime : <varint>             : modification time of the node.
//         - linkName : <string>            : link target of symlink
//         - mode : <uvarint>               : permission and mode bits (os.FileMode).
//         - uid : <varint>                 : uid of the owner.
//         - gid : <varint>                 : gid of the owner.
//         - devMajor : <varint>            : the major device number for device
//         - devMinor : <varint>            : the minor device number for device
//         - xattrKey : <string>            : key of the first extended attribute.
//         - xattrValue : <string>          : value of the first extended attribute
//         - xattrsExtra                    : 2nd and the following extended attribute.
//           - *key* : <string>             : map of key to value string
//         - numLink : <varint>             : the number of links pointing to this node.
//     - metadata
//       - *node id*                        : bucket for each node keyed by a uniqe uint64.
//         - childName : <string>           : base name of the first child
//         - childID   : <node id>          : id of the first child
//         - childrenExtra                  : 2nd and following child nodes of directory.
//           - *basename* : <node id>       : map of basename string to the child node id
//         - name                           : the name of the file as recorded in the TAR header
//         - uncompressedOffset : <varint>  : the offset in the uncompressed data, where the node is stored.
//         - tarHeaderOffset : <varint>     : the offset of the tar header
//         - tarHeaderSize : <varint>       : the size of the tar header

var (
	bucketKeyFilesystems = []byte("filesystems")

	bucketKeyNodes       = []byte("nodes")
	bucketKeySize        = []byte("size")
	bucketKeyModTime     = []byte("modtime")
	bucketKeyLinkName    = []byte("linkName")
	bucketKeyMode        = []byte("mode")
	bucketKeyUID         = []byte("uid")
	bucketKeyGID         = []byte("gid")
	bucketKeyDevMajor    = []byte("devMajor")
	bucketKeyDevMinor    = []byte("devMinor")
	bucketKeyXattrKey    = []byte("xattrKey")
	bucketKeyXattrValue  = []byte("xattrValue")
	bucketKeyXattrsExtra = []byte("xattrsExtra")
	bucketKeyNumLink     = []byte("numLink")

	bucketKeyMetadata      = []byte("metadata")
	bucketKeyChildName     = []byte("childName")
	bucketKeyChildID       = []byte("childID")
	bucketKeyChildrenExtra = []byte("childrenExtra")

	bucketKeyName               = []byte("name")
	bucketKeyUncompressedOffset = []byte("uncompressedOffset")
	bucketKeyTarHeaderOffset    = []byte("tarHeaderOffset")
	bucketKeyTarHeaderSize      = []byte("tarHeaderSize")
)

type childEntry struct {
	base string
	id   uint32
}

type metadataEntry struct {
	children           map[string]childEntry
	UncompressedOffset compression.Offset
	UncompressedSize   compression.Offset
	TarName            string
	TarHeaderOffset    compression.Offset
	TarHeaderSize      compression.Offset
}

// getNodesBucket returns the top-level nodes bucket that contains each node
// as a sub-bucket.
func getNodesBucket(tx *bolt.Tx, fsID string) (*bolt.Bucket, error) {
	filesystems := tx.Bucket(bucketKeyFilesystems)
	if filesystems == nil {
		return nil, fmt.Errorf("fs %q not found: no fs is registered", fsID)
	}
	lbkt := filesystems.Bucket([]byte(fsID))
	if lbkt == nil {
		return nil, fmt.Errorf("fs bucket for %q not found", fsID)
	}
	nodes := lbkt.Bucket(bucketKeyNodes)
	if nodes == nil {
		return nil, fmt.Errorf("nodes bucket for %q not found", fsID)
	}
	return nodes, nil
}

// getMetadataBucket returns the top-level metadata bucket that contains each node
// as a sub-bucket.
func getMetadataBucket(tx *bolt.Tx, fsID string) (*bolt.Bucket, error) {
	filesystems := tx.Bucket(bucketKeyFilesystems)
	if filesystems == nil {
		return nil, fmt.Errorf("fs %q not found: no fs is registered", fsID)
	}
	lbkt := filesystems.Bucket([]byte(fsID))
	if lbkt == nil {
		return nil, fmt.Errorf("fs bucket for %q not found", fsID)
	}
	md := lbkt.Bucket(bucketKeyMetadata)
	if md == nil {
		return nil, fmt.Errorf("metadata bucket for fs %q not found", fsID)
	}
	return md, nil
}

// getNodeBucketByID returns the node sub-bucket with the appropriate node id inside the top-level nodes bucket.
func getNodeBucketByID(nodes *bolt.Bucket, id uint32) (*bolt.Bucket, error) {
	b := nodes.Bucket(encodeID(id))
	if b == nil {
		return nil, fmt.Errorf("node bucket for %d not found", id)
	}
	return b, nil
}

// getMetadataBucketByID returns the node sub-bucket with the appropriate node id inside the top-level metadata bucket.
func getMetadataBucketByID(md *bolt.Bucket, id uint32) (*bolt.Bucket, error) {
	b := md.Bucket(encodeID(id))
	if b == nil {
		return nil, fmt.Errorf("metadata bucket for %d not found", id)
	}
	return b, nil
}

// writeNodeEntry writes node metadata to the appropriate node sub-bucket inside the top-level nodes bucket.
func writeNodeEntry(b *bolt.Bucket, attr *Attr) error {
	if attr.DevMajor != 0 {
		putInt(b, bucketKeyDevMajor, int64(attr.DevMajor))
	}
	if attr.DevMinor != 0 {
		putInt(b, bucketKeyDevMinor, int64(attr.DevMinor))
	}
	if attr.GID != 0 {
		putInt(b, bucketKeyGID, int64(attr.GID))
	}
	if len(attr.LinkName) > 0 {
		if err := b.Put(bucketKeyLinkName, []byte(attr.LinkName)); err != nil {
			return err
		}
	}
	if attr.Mode != 0 {
		val, err := encodeUint(uint64(attr.Mode))
		if err != nil {
			return err
		}
		if err := b.Put(bucketKeyMode, val); err != nil {
			return err
		}
	}
	if !attr.ModTime.IsZero() {
		te, err := attr.ModTime.GobEncode()
		if err != nil {
			return err
		}
		if err := b.Put(bucketKeyModTime, te); err != nil {
			return err
		}
	}
	if attr.NumLink != 0 {
		putInt(b, bucketKeyNumLink, int64(attr.NumLink-1)) // numLink = 0 means num link = 1 in DB
	}
	if attr.Size != 0 {
		putInt(b, bucketKeySize, attr.Size)
	}
	if attr.UID != 0 {
		putInt(b, bucketKeyUID, int64(attr.UID))
	}
	if len(attr.Xattrs) > 0 {
		var firstK string
		var firstV []byte
		for k, v := range attr.Xattrs {
			firstK, firstV = k, v
			break
		}
		var xbkt *bolt.Bucket
		for k, v := range attr.Xattrs {
			if k == firstK || len(v) == 0 {
				continue
			}
			if xbkt == nil {
				if xbkt := b.Bucket(bucketKeyXattrsExtra); xbkt != nil {
					// Reset
					if err := b.DeleteBucket(bucketKeyXattrsExtra); err != nil {
						return err
					}
				}
				var err error
				xbkt, err = b.CreateBucket(bucketKeyXattrsExtra)
				if err != nil {
					return err
				}
			}
			if err := xbkt.Put([]byte(k), v); err != nil {
				return fmt.Errorf("failed to set xattr: %w", err)
			}
		}
		if err := b.Put(bucketKeyXattrKey, []byte(firstK)); err != nil {
			return err
		}
		if err := b.Put(bucketKeyXattrValue, firstV); err != nil {
			return err
		}

	}
	return nil
}

func readNodeEntryToAttr(b *bolt.Bucket, attr *Attr) error {
	return b.ForEach(func(k, v []byte) error {
		switch string(k) {
		case string(bucketKeySize):
			attr.Size, _ = binary.Varint(v)
		case string(bucketKeyModTime):
			if err := (&attr.ModTime).GobDecode(v); err != nil {
				return err
			}
		case string(bucketKeyLinkName):
			attr.LinkName = string(v)
		case string(bucketKeyMode):
			mode, _ := binary.Uvarint(v)
			attr.Mode = os.FileMode(uint32(mode))
		case string(bucketKeyUID):
			i, _ := binary.Varint(v)
			attr.UID = int(i)
		case string(bucketKeyGID):
			i, _ := binary.Varint(v)
			attr.GID = int(i)
		case string(bucketKeyDevMajor):
			i, _ := binary.Varint(v)
			attr.DevMajor = int(i)
		case string(bucketKeyDevMinor):
			i, _ := binary.Varint(v)
			attr.DevMinor = int(i)
		case string(bucketKeyNumLink):
			i, _ := binary.Varint(v)
			attr.NumLink = int(i) + 1 // numLink = 0 means num link = 1 in DB
		case string(bucketKeyXattrKey):
			if attr.Xattrs == nil {
				attr.Xattrs = make(map[string][]byte)
			}
			attr.Xattrs[string(v)] = b.Get(bucketKeyXattrValue)
		case string(bucketKeyXattrsExtra):
			if err := b.Bucket(k).ForEach(func(k, v []byte) error {
				if attr.Xattrs == nil {
					attr.Xattrs = make(map[string][]byte)
				}
				attr.Xattrs[string(k)] = v
				return nil
			}); err != nil {
				return err
			}
		}
		return nil
	})
}

func readNumLink(b *bolt.Bucket) int {
	// numLink = 0 means num link = 1 in BD
	numLink, _ := binary.Varint(b.Get(bucketKeyNumLink))
	return int(numLink) + 1
}

func readChild(md *bolt.Bucket, base string) (uint32, error) {
	if base == string(md.Get(bucketKeyChildName)) {
		return decodeID(md.Get(bucketKeyChildID)), nil
	}
	cbkt := md.Bucket(bucketKeyChildrenExtra)
	if cbkt == nil {
		return 0, fmt.Errorf("extra children not found")
	}
	eid := cbkt.Get([]byte(base))
	if len(eid) == 0 {
		return 0, fmt.Errorf("children not found")
	}
	return decodeID(eid), nil
}

// writeMetadataEntry writes a metadata entry to the appropriate node sub-bucket inside the top-level metadata bucket.
func writeMetadataEntry(md *bolt.Bucket, m *metadataEntry) error {
	if len(m.children) > 0 {
		var firstChildName string
		var firstChild childEntry
		for name, child := range m.children {
			firstChildName, firstChild = name, child
			break
		}
		if len(m.children) > 1 {
			// Sort children by base name.
			keys := make([]string, 0, len(m.children))
			for k := range m.children {
				keys = append(keys, k)
			}
			sort.Slice(keys, func(i, j int) bool {
				return bytes.Compare([]byte(m.children[keys[i]].base), []byte(m.children[keys[j]].base)) < 0
			})
			var cbkt *bolt.Bucket
			for _, key := range keys {
				if key == firstChildName {
					continue
				}
				if cbkt == nil {
					if cbkt := md.Bucket(bucketKeyChildrenExtra); cbkt != nil {
						// Reset
						if err := md.DeleteBucket(bucketKeyChildrenExtra); err != nil {
							return err
						}
					}
					var err error
					cbkt, err = md.CreateBucket(bucketKeyChildrenExtra)
					if err != nil {
						return err
					}
				}
				if err := cbkt.Put([]byte(m.children[key].base), encodeID(m.children[key].id)); err != nil {
					return fmt.Errorf("failed to add child ID %q: %w", m.children[key].id, err)
				}
			}
		}
		if err := md.Put(bucketKeyChildID, encodeID(firstChild.id)); err != nil {
			return fmt.Errorf("failed to put id of first child: %w", err)
		}
		if err := md.Put(bucketKeyChildName, []byte(firstChildName)); err != nil {
			return fmt.Errorf("failed to put name first child: %w", err)
		}

	}
	if err := md.Put(bucketKeyName, []byte(m.TarName)); err != nil {
		return fmt.Errorf("failed to set TarName value %s: %w", m.TarName, err)
	}
	if err := putInt(md, bucketKeyTarHeaderOffset, int64(m.TarHeaderOffset)); err != nil {
		return fmt.Errorf("failed to set TarHeaderOffset value %d: %w", m.TarHeaderOffset, err)
	}
	if err := putInt(md, bucketKeyTarHeaderSize, int64(m.TarHeaderSize)); err != nil {
		return fmt.Errorf("failed to set TarHeaderSize value %d: %w", m.TarHeaderSize, err)
	}
	if err := putInt(md, bucketKeyUncompressedOffset, int64(m.UncompressedOffset)); err != nil {
		return fmt.Errorf("failed to set UncompressedOffset value %d: %w", m.UncompressedOffset, err)
	}
	return nil
}

func getMetadataEntry(md *bolt.Bucket) metadataEntry {
	ucompOffset, _ := binary.Varint(md.Get(bucketKeyUncompressedOffset))
	tarHeaderOffset, _ := binary.Varint(md.Get(bucketKeyTarHeaderOffset))
	tarHeaderSize, _ := binary.Varint(md.Get(bucketKeyTarHeaderSize))
	tarName := md.Get(bucketKeyName)
	return metadataEntry{nil,
		compression.Offset(ucompOffset),
		0,
		string(tarName),
		compression.Offset(tarHeaderOffset),
		compression.Offset(tarHeaderSize)}
}

func encodeID(id uint32) []byte {
	b := [4]byte{}
	binary.BigEndian.PutUint32(b[:], id)
	return b[:]
}

func decodeID(b []byte) uint32 {
	return binary.BigEndian.Uint32(b)
}

func putInt(b *bolt.Bucket, k []byte, v int64) error {
	i, err := dbutil.EncodeInt(v)
	if err != nil {
		return err
	}
	return b.Put(k, i)
}

func encodeUint(i uint64) ([]byte, error) {
	var (
		buf      [binary.MaxVarintLen64]byte
		iEncoded = buf[:]
	)
	iEncoded = iEncoded[:binary.PutUvarint(iEncoded, i)]

	if len(iEncoded) == 0 {
		return nil, fmt.Errorf("failed encoding integer = %v", i)
	}
	return iEncoded, nil
}
