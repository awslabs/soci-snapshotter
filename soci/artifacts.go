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

package soci

import (
	"context"
	"fmt"
	"os"
	"path"
	"sync"
	"time"

	"github.com/awslabs/soci-snapshotter/fs/config"
	"github.com/awslabs/soci-snapshotter/util/dbutil"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/log"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	bolt "go.etcd.io/bbolt"
)

// Artifacts package stores SOCI artifacts info in the following schema.
//
// - soci_artifacts
//       - *soci_artifact_digest*       : bucket for each soci layer keyed by a unique string.
//         - size : <varint>            : size of the artifact.
//         - originalDigest : <string>  : the digest for the image manifest or layer
//         - imageDigest: <string>      : the digest of the image index
//         - platform: <string>         : the platform for the index
//         - location: <string>         : the location of the artifact
//         - type: <string>             : the type of the artifact (can be either "soci_index" or "soci_layer")

// ArtifactsDB is a store for SOCI artifact metadata
type ArtifactsDb struct {
	db *bolt.DB
}

// ArtifactEntryType is the type of SOCI artifact represented by the ArtifactEntry
type ArtifactEntryType string

const (
	artifactsDbName = "artifacts.db"
)

var (
	bucketKeySociArtifacts  = []byte("soci_artifacts")
	bucketKeySize           = []byte("size")
	bucketKeyOriginalDigest = []byte("oci_digest")
	bucketKeyImageDigest    = []byte("image_digest")
	bucketKeyPlatform       = []byte("platform")
	bucketKeyLocation       = []byte("location")
	bucketKeyType           = []byte("type")
	bucketKeyMediaType      = []byte("media_type")
	bucketKeyCreatedAt      = []byte("created_at")

	// ArtifactEntryTypeIndex indicates that an ArtifactEntry is a SOCI index artifact
	ArtifactEntryTypeIndex ArtifactEntryType = "soci_index"
	// ArtifactEntryTypeLayer indicates that an ArtifactEntry is a SOCI layer artifact
	ArtifactEntryTypeLayer ArtifactEntryType = "soci_layer"

	db   *ArtifactsDb
	once sync.Once
)

// Get the default artifacts db path
func ArtifactsDbPath() string {
	return path.Join(config.SociSnapshotterRootPath, artifactsDbName)
}

// ArtifactEntry is a metadata object for a SOCI artifact.
type ArtifactEntry struct {
	// Size is the SOCI artifact's size in bytes.
	Size int64
	// Digest is the SOCI artifact's digest.
	Digest string
	// OriginalDigest is the digest of the content for which the SOCI artifact was created.
	OriginalDigest string
	// ImageDigest is the digest of the container image that was used to generat the artifact
	// ImageDigest refers to the image, OriginalDigest refers to the specific content within that
	// image that was used to generate the Artifact.
	ImageDigest string
	// Platform is the platform for which the artifact was generated.
	Platform string
	// Location is the file path for the SOCI artifact.
	Location string
	// Type is the type of SOCI artifact.
	Type ArtifactEntryType
	// Media Type of the stored artifact.
	MediaType string
	// Creation time of SOCI artifact.
	CreatedAt time.Time
}

// NewDB returns an instance of an ArtifactsDB
func NewDB(path string) (*ArtifactsDb, error) {
	once.Do(func() {
		f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			log.G(context.Background()).Errorf("can't create or open the file %s", path)
			return
		}
		defer f.Close()
		database, err := bolt.Open(f.Name(), 0600, nil)
		if err != nil {
			log.G(context.Background()).Errorf("can't open the db")
			return
		}
		db = &ArtifactsDb{db: database}
	})

	if db == nil {
		return nil, fmt.Errorf("artifacts.db is not available")
	}

	return db, nil
}

func (db *ArtifactsDb) getIndexArtifactEntries(indexDigest string) ([]ArtifactEntry, error) {
	artifactEntries := []ArtifactEntry{}
	err := db.Walk(func(ae *ArtifactEntry) error {
		if ae.Type == ArtifactEntryTypeIndex && ae.OriginalDigest == indexDigest {
			artifactEntries = append(artifactEntries, *ae)
		}
		return nil
	})

	return artifactEntries, err

}

// Walk applys a function to all ArtifactEntries in the ArtifactsDB
func (db *ArtifactsDb) Walk(f func(*ArtifactEntry) error) error {
	err := db.db.View(func(tx *bolt.Tx) error {
		bucket, err := getArtifactsBucket(tx)
		if err != nil {
			return nil
		}
		bucket.ForEach(func(k, v []byte) error {
			// Skip non-buckets
			if v != nil {
				return nil
			}
			artifactBkt := bucket.Bucket(k)
			ae, err := loadArtifact(artifactBkt, string(k))
			if err != nil {
				return err
			}
			return f(ae)
		})
		return nil
	})
	return err
}

// GetArtifactEntry loads a single ArtifactEntry from the ArtifactsDB by digest
func (db *ArtifactsDb) GetArtifactEntry(digest string) (*ArtifactEntry, error) {
	entry := ArtifactEntry{}
	err := db.db.View(func(tx *bolt.Tx) error {
		bucket, err := getArtifactsBucket(tx)
		if err != nil {
			return err
		}
		e, err := getArtifactEntryByDigest(bucket, digest)
		if err != nil {
			return err
		}
		entry = *e
		return nil
	})

	if err != nil {
		return nil, err
	}
	return &entry, nil
}

// GetArtifactType gets Type of an ArtifactEntry from the ArtifactsDB by digest
func (db *ArtifactsDb) GetArtifactType(digest string) (ArtifactEntryType, error) {
	ae, err := db.GetArtifactEntry(digest)
	if err != nil {
		return "", err
	}
	return ae.Type, nil
}

// RemoveArtifactEntryByIndexDigest removes an index's artifact entry using its digest
func (db *ArtifactsDb) RemoveArtifactEntryByIndexDigest(digest string) error {
	return db.db.Update(func(tx *bolt.Tx) error {
		bucket, err := getArtifactsBucket(tx)
		if err != nil {
			return err
		}

		dgstBucket := bucket.Bucket([]byte(digest))
		if dgstBucket == nil {
			return fmt.Errorf("the index of the digest %v doesn't exist", digest)
		}

		if indexBucket(dgstBucket) {
			return bucket.DeleteBucket([]byte(digest))
		}
		return fmt.Errorf("the digest %v does not correspond to an index", digest)
	})
}

// RemoveArtifactEntryByIndexDigest removes an index's artifact entry using the image digest
func (db *ArtifactsDb) RemoveArtifactEntryByImageDigest(digest string) error {
	return db.db.Update(func(tx *bolt.Tx) error {
		bucket, err := getArtifactsBucket(tx)
		if err != nil {
			return err
		}

		c := bucket.Cursor()
		for k, _ := c.First(); k != nil; k, _ = c.Next() {
			artifactBucket := bucket.Bucket(k)
			if indexBucket(artifactBucket) && hasImageDigest(artifactBucket, digest) {
				bucket.DeleteBucket(k)
			}
		}
		return nil
	})
}

// Determines whether a bucket represents an index, as opposed to a zTOC
func indexBucket(b *bolt.Bucket) bool {
	mt := string(b.Get(bucketKeyMediaType))
	return mt == ocispec.MediaTypeArtifactManifest || mt == ocispec.MediaTypeImageManifest
}

// Determines whether a bucket's image digest is the same as digest
func hasImageDigest(b *bolt.Bucket, digest string) bool {
	imgDigest := string(b.Get(bucketKeyImageDigest))
	return digest == imgDigest
}

// WriteArtifactEntry stores a single ArtifactEntry into the ArtifactsDB.
// If there is already an artifact in the ArtifactsDB with the same Digest,
// the old data is overwritten.
func (db *ArtifactsDb) WriteArtifactEntry(entry *ArtifactEntry) error {
	if entry == nil {
		return fmt.Errorf("no entry to write")
	}
	err := db.db.Update(func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists(bucketKeySociArtifacts)
		if err != nil {
			return err
		}
		err = putArtifactEntry(bucket, entry)
		return err
	})
	return err
}

func getArtifactsBucket(tx *bolt.Tx) (*bolt.Bucket, error) {
	artifacts := tx.Bucket(bucketKeySociArtifacts)
	if artifacts == nil {
		return nil, fmt.Errorf("soci_artifacts not found")
	}

	return artifacts, nil
}

func getArtifactEntryByDigest(artifacts *bolt.Bucket, digest string) (*ArtifactEntry, error) {
	artifactBkt := artifacts.Bucket([]byte(digest))
	if artifactBkt == nil {
		return nil, fmt.Errorf("couldn't retrieve artifact for %s, %w", digest, errdefs.ErrNotFound)
	}
	return loadArtifact(artifactBkt, digest)
}

func loadArtifact(artifactBkt *bolt.Bucket, digest string) (*ArtifactEntry, error) {
	ae := ArtifactEntry{Digest: digest}
	encodedSize := artifactBkt.Get(bucketKeySize)
	size, err := dbutil.DecodeInt(encodedSize)
	if err != nil {
		return nil, err
	}
	createdAt := time.Time{}
	createdAtBytes := artifactBkt.Get(bucketKeyCreatedAt)
	if createdAtBytes != nil {
		err := createdAt.UnmarshalBinary(createdAtBytes)
		if err != nil {
			return nil, fmt.Errorf("cannot unmarshal CreatedAt time: %w", err)
		}
	}
	ae.Size = size
	ae.Location = string(artifactBkt.Get(bucketKeyLocation))
	ae.Type = ArtifactEntryType(artifactBkt.Get(bucketKeyType))
	ae.OriginalDigest = string(artifactBkt.Get(bucketKeyOriginalDigest))
	ae.ImageDigest = string(artifactBkt.Get(bucketKeyImageDigest))
	ae.Platform = string(artifactBkt.Get(bucketKeyPlatform))
	ae.MediaType = string(artifactBkt.Get(bucketKeyMediaType))
	ae.CreatedAt = createdAt
	return &ae, nil
}

func putArtifactEntry(artifacts *bolt.Bucket, ae *ArtifactEntry) error {
	if artifacts == nil {
		return fmt.Errorf("can't write ArtifactEntry: the bucket does not exist")
	}

	artifactBkt, err := artifacts.CreateBucketIfNotExists([]byte(ae.Digest))
	if err != nil {
		return err
	}

	sizeInBytes, err := dbutil.EncodeInt(ae.Size)
	if err != nil {
		return err
	}

	createdAt, err := ae.CreatedAt.MarshalBinary()
	if err != nil {
		return err
	}

	updates := []struct {
		key []byte
		val []byte
	}{
		{bucketKeySize, sizeInBytes},
		{bucketKeyLocation, []byte(ae.Location)},
		{bucketKeyOriginalDigest, []byte(ae.OriginalDigest)},
		{bucketKeyImageDigest, []byte(ae.ImageDigest)},
		{bucketKeyPlatform, []byte(ae.Platform)},
		{bucketKeyType, []byte(ae.Type)},
		{bucketKeyMediaType, []byte(ae.MediaType)},
		{bucketKeyCreatedAt, createdAt},
	}

	for _, update := range updates {
		if err := artifactBkt.Put(update.key, update.val); err != nil {
			return err
		}
	}

	return nil
}
