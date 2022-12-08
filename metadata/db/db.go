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

package db

import (
	"encoding/binary"
	"fmt"
	"os"

	"github.com/awslabs/soci-snapshotter/compression"
	"github.com/awslabs/soci-snapshotter/metadata"
	"github.com/awslabs/soci-snapshotter/util/dbutil"
	"github.com/pkg/errors"
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
//         - uncompressedOffset : <varint>  : the offset in the uncompressed data, where the node is stored.

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

	bucketKeyUncompressedOffset = []byte("uncompressedOffset")
)

type childEntry struct {
	base string
	id   uint32
}

type metadataEntry struct {
	children           map[string]childEntry
	UncompressedOffset compression.Offset
	UncompressedSize   compression.Offset
}

func getNodes(tx *bolt.Tx, fsID string) (*bolt.Bucket, error) {
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

func getMetadata(tx *bolt.Tx, fsID string) (*bolt.Bucket, error) {
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

func getNodeBucketByID(nodes *bolt.Bucket, id uint32) (*bolt.Bucket, error) {
	b := nodes.Bucket(encodeID(id))
	if b == nil {
		return nil, fmt.Errorf("node bucket for %d not found", id)
	}
	return b, nil
}

func getMetadataBucketByID(md *bolt.Bucket, id uint32) (*bolt.Bucket, error) {
	b := md.Bucket(encodeID(id))
	if b == nil {
		return nil, fmt.Errorf("metadata bucket for %d not found", id)
	}
	return b, nil
}

func writeAttr(b *bolt.Bucket, attr *metadata.Attr) error {
	for _, v := range []struct {
		key []byte
		val int64
	}{
		{bucketKeySize, attr.Size},
		{bucketKeyUID, int64(attr.UID)},
		{bucketKeyGID, int64(attr.GID)},
		{bucketKeyDevMajor, int64(attr.DevMajor)},
		{bucketKeyDevMinor, int64(attr.DevMinor)},
		{bucketKeyNumLink, int64(attr.NumLink - 1)}, // numLink = 0 means num link = 1 in DB
	} {
		if v.val != 0 {
			val, err := dbutil.EncodeInt(v.val)
			if err != nil {
				return err
			}
			if err := b.Put(v.key, val); err != nil {
				return err
			}
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
	if len(attr.Xattrs) > 0 {
		var firstK string
		var firstV []byte
		for k, v := range attr.Xattrs {
			firstK, firstV = k, v
			break
		}
		if err := b.Put(bucketKeyXattrKey, []byte(firstK)); err != nil {
			return err
		}
		if err := b.Put(bucketKeyXattrValue, firstV); err != nil {
			return err
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
				return errors.Wrapf(err, "failed to set xattr %q=%q", k, string(v))
			}
		}
	}

	return nil
}

func readAttr(b *bolt.Bucket, attr *metadata.Attr) error {
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
		return 0, fmt.Errorf("children %q not found", base)
	}
	return decodeID(eid), nil
}

func writeMetadataEntry(md *bolt.Bucket, m *metadataEntry) error {
	if len(m.children) > 0 {
		var firstChildName string
		var firstChild childEntry
		for name, child := range m.children {
			firstChildName, firstChild = name, child
			break
		}
		if err := md.Put(bucketKeyChildID, encodeID(firstChild.id)); err != nil {
			return errors.Wrapf(err, "failed to put id of first child %q", firstChildName)
		}
		if err := md.Put(bucketKeyChildName, []byte(firstChildName)); err != nil {
			return errors.Wrapf(err, "failed to put name first child %q", firstChildName)
		}
		if len(m.children) > 1 {
			var cbkt *bolt.Bucket
			for k, c := range m.children {
				if k == firstChildName {
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
				if err := cbkt.Put([]byte(c.base), encodeID(c.id)); err != nil {
					return errors.Wrapf(err, "failed to add child ID %q", c.id)
				}
			}
		}
	}
	if err := putFileSize(md, bucketKeyUncompressedOffset, m.UncompressedOffset); err != nil {
		return errors.Wrapf(err, "failed to set UncompressedOffset value %d", m.UncompressedOffset)
	}

	return nil
}

func putFileSize(b *bolt.Bucket, k []byte, v compression.Offset) error {
	return putInt(b, k, int64(v))
}

func putInt(b *bolt.Bucket, k []byte, v int64) error {
	i, err := dbutil.EncodeInt(v)
	if err != nil {
		return err
	}
	return b.Put(k, i)
}

func encodeID(id uint32) []byte {
	b := [4]byte{}
	binary.BigEndian.PutUint32(b[:], id)
	return b[:]
}

func decodeID(b []byte) uint32 {
	return binary.BigEndian.Uint32(b)
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
