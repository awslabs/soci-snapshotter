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
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sync"
	"time"

	"github.com/awslabs/soci-snapshotter/config"
	"github.com/awslabs/soci-snapshotter/soci/store"
	"github.com/awslabs/soci-snapshotter/util/dbutil"
	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/images"
	"github.com/containerd/errdefs"
	"github.com/containerd/log"
	"github.com/containerd/platforms"
	"github.com/opencontainers/go-digest"
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
	bucketKeyArtifactType   = []byte("artifact_type")
	bucketKeyCreatedAt      = []byte("created_at")

	// ArtifactEntryTypeIndex indicates that an ArtifactEntry is a SOCI index artifact
	ArtifactEntryTypeIndex ArtifactEntryType = "soci_index"
	// ArtifactEntryTypeLayer indicates that an ArtifactEntry is a SOCI layer artifact
	ArtifactEntryTypeLayer ArtifactEntryType = "soci_layer"

	db   *ArtifactsDb
	once sync.Once
)

var (
	ErrArtifactBucketNotFound = errors.New("soci_artifacts not found")
)

// Get the default artifacts db path
func ArtifactsDbPath(root string) string {
	if root == "" {
		root = config.DefaultSociSnapshotterRootPath
	}
	return path.Join(root, artifactsDbName)
}

// ArtifactEntry is a metadata object for a SOCI artifact.
type ArtifactEntry struct {
	// Size is the SOCI artifact's size in bytes.
	Size int64
	// Digest is the SOCI artifact's digest.
	Digest string
	// OriginalDigest is the digest of the content for which the SOCI artifact was created.
	OriginalDigest string
	// ImageDigest is the digest of the container image that was used to generate the artifact
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
	// ArtifactType is the type of artifact stored (e.g. index manifest v1 vs index manifest v2)
	ArtifactType string
	// Creation time of SOCI artifact.
	CreatedAt time.Time
}

// NewDB returns an instance of an ArtifactsDB
func NewDB(path string) (*ArtifactsDb, error) {
	once.Do(func() {
		f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			log.G(context.Background()).WithError(err).WithField("path", path).Error("Cannot create or open file")
			return
		}
		defer f.Close()
		database, err := bolt.Open(f.Name(), 0600, nil)
		if err != nil {
			log.G(context.Background()).WithError(err).Error("Cannot open the db")
			return
		}
		db = &ArtifactsDb{db: database}
	})

	if db == nil {
		return nil, errors.New("artifacts.db is not available")
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
		bucket.ForEachBucket(func(k []byte) error {
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

// SyncWithLocalStore will sync the artifacts databse with SOCIs local content store, either adding new or removing old artifacts.
func (db *ArtifactsDb) SyncWithLocalStore(ctx context.Context, blobStore store.Store, blobStorePath string, cs content.Store) error {
	if err := db.RemoveOldArtifacts(ctx, blobStore); err != nil {
		return fmt.Errorf("failed to remove old artifacts from db: %w", err)
	}
	if err := db.addNewArtifacts(ctx, blobStorePath, cs); err != nil {
		return fmt.Errorf("failed to add new artifacts to db: %w", err)
	}
	return nil
}

// RemoveOldArtifacts will remove any artifacts from the artifacts database that
// no longer exist in SOCIs local content store. NOTE: Removing buckets while iterating
// (bucket.ForEach) causes unexpected behavior (see: https://github.com/boltdb/bolt/issues/426).
// This implementation works around this issue by appending buckets to a slice when
// iterating and removing them after.
func (db *ArtifactsDb) RemoveOldArtifacts(ctx context.Context, blobStore store.Store) error {
	err := db.db.Update(func(tx *bolt.Tx) error {
		bucket, err := getArtifactsBucket(tx)
		if err != nil {
			return nil
		}
		var bucketsToRemove [][]byte
		bucket.ForEachBucket(func(k []byte) error {
			artifactBucket := bucket.Bucket(k)
			ae, err := loadArtifact(artifactBucket, string(k))
			if err != nil {
				return err
			}
			existsInContentStore, err := blobStore.Exists(ctx,
				ocispec.Descriptor{MediaType: ae.MediaType, Digest: digest.Digest(ae.Digest)})
			if err != nil {
				return err
			}
			if !existsInContentStore {
				bucketsToRemove = append(bucketsToRemove, k)
			}
			return nil
		})
		// remove the buckets
		for _, k := range bucketsToRemove {
			if err := bucket.DeleteBucket(k); err != nil {
				return err
			}
		}
		return nil
	})
	return err
}

// addNewArtifacts will add any new artifacts discovered in SOCIs local content store to the artifacts database.
func (db *ArtifactsDb) addNewArtifacts(ctx context.Context, blobStorePath string, cs content.Store) error {
	addHashPrefix := func(name string) string {
		if len(name) == 64 {
			return fmt.Sprintf("sha256:%s", name)
		}
		return fmt.Sprintf("sha512:%s", name)
	}
	return filepath.WalkDir(blobStorePath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		// skip: entry is an empty config
		if info.Size() < 10 {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()

		var sociIndex Index
		// tests to ensure artifact is really an index
		if err = DecodeIndex(f, &sociIndex); err != nil {
			return nil
		}
		if sociIndex.MediaType != ocispec.MediaTypeImageManifest {
			return nil
		}
		if sociIndex.ArtifactType != SociIndexArtifactType {
			return nil
		}
		if sociIndex.Subject == nil {
			return nil
		}
		// entry is an index
		indexDigest := addHashPrefix(d.Name())
		ae, err := db.GetArtifactEntry(indexDigest)
		if err != nil && !errors.Is(err, ErrArtifactBucketNotFound) && !errors.Is(err, errdefs.ErrNotFound) {
			return err
		}
		if ae == nil {
			manifestDigest := sociIndex.Subject.Digest.String()
			platform, err := images.Platforms(ctx, cs, ocispec.Descriptor{
				MediaType: ocispec.MediaTypeImageManifest,
				Digest:    digest.Digest(manifestDigest)})
			if err != nil {
				return err
			}

			indexEntry := &ArtifactEntry{
				Size:           info.Size(),
				Digest:         indexDigest,
				OriginalDigest: manifestDigest,
				ImageDigest:    manifestDigest,
				Platform:       platforms.Format(platform[0]),
				Type:           ArtifactEntryTypeIndex,
				Location:       manifestDigest,
				MediaType:      sociIndex.MediaType,
				ArtifactType:   sociIndex.Config.MediaType,
				CreatedAt:      time.Now(),
			}
			if err = db.WriteArtifactEntry(indexEntry); err != nil {
				return err
			}
			for _, zt := range sociIndex.Blobs {
				ztocEntry := &ArtifactEntry{
					Size:           zt.Size,
					Digest:         zt.Digest.String(),
					OriginalDigest: zt.Annotations[IndexAnnotationImageLayerDigest],
					Type:           ArtifactEntryTypeLayer,
					Location:       zt.Annotations[IndexAnnotationImageLayerDigest],
					MediaType:      SociLayerMediaType,
					CreatedAt:      time.Now(),
				}
				if err := db.WriteArtifactEntry(ztocEntry); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

// GetArtifactEntry loads a single ArtifactEntry from the ArtifactsDB by digest
func (db *ArtifactsDb) GetArtifactEntry(digest string) (*ArtifactEntry, error) {
	var entry *ArtifactEntry
	err := db.db.View(func(tx *bolt.Tx) error {
		var err error
		entry, err = db.getArtifactEntry(tx, digest)
		return err
	})

	return entry, err
}

func (db *ArtifactsDb) getArtifactEntry(tx *bolt.Tx, digest string) (*ArtifactEntry, error) {
	bucket, err := getArtifactsBucket(tx)
	if err != nil {
		return nil, err
	}
	return getArtifactEntryByDigest(bucket, digest)
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
func (db *ArtifactsDb) RemoveArtifactEntryByIndexDigest(digest []byte) error {
	return db.db.Update(func(tx *bolt.Tx) error {
		bucket, err := getArtifactsBucket(tx)
		if err != nil {
			return err
		}

		dgstBucket := bucket.Bucket(digest)
		if dgstBucket == nil {
			return fmt.Errorf("the index of the digest %s doesn't exist", digest)
		}

		if indexBucket(dgstBucket) {
			return bucket.DeleteBucket(digest)
		}
		return fmt.Errorf("the digest %s does not correspond to an index", digest)
	})
}

// GetArtifactEntriesByImageDigest returns all index digests greated from a given image digest
func (db *ArtifactsDb) GetArtifactEntriesByImageDigest(digest string) ([][]byte, error) {
	entries := make([][]byte, 0)
	return entries, db.db.View(func(tx *bolt.Tx) error {
		bucket, err := getArtifactsBucket(tx)
		if err != nil {
			return err
		}

		c := bucket.Cursor()
		for k, _ := c.First(); k != nil; k, _ = c.Next() {
			artifactBucket := bucket.Bucket(k)
			if indexBucket(artifactBucket) && hasImageDigest(artifactBucket, digest) {
				entries = append(entries, k)
			}
		}
		return nil
	})
}

// Determines whether a bucket represents an index, as opposed to a zTOC
func indexBucket(b *bolt.Bucket) bool {
	mt := string(b.Get(bucketKeyMediaType))
	return mt == ocispec.MediaTypeImageManifest
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
	return db.db.Update(func(tx *bolt.Tx) error {
		return db.writeArtifactEntry(tx, entry)
	})
}

func (db *ArtifactsDb) writeArtifactEntry(tx *bolt.Tx, entry *ArtifactEntry) error {
	bucket, err := tx.CreateBucketIfNotExists(bucketKeySociArtifacts)
	if err != nil {
		return err
	}
	return putArtifactEntry(bucket, entry)
}

// updateSociV2ArtifactReference updates the image and manifest digests associated with the SOCI index digest in the artifacts db.
// the indexDigest is the SOCI index to updated, the manifestDigest is the specific image manifest that the SOCI index is bound to,
// and the imageDigest is the target of the image (this is the same as the manifestDigest for single platform images, but different for mult-platform)
func (db *ArtifactsDb) updateSociV2ArtifactReference(indexDigest string, manifestDigest string, imageDigest string) error {
	return db.db.Update(func(tx *bolt.Tx) error {
		ae, err := db.getArtifactEntry(tx, indexDigest)
		if err != nil {
			return err
		}
		ae.ImageDigest = imageDigest
		ae.OriginalDigest = manifestDigest
		return db.writeArtifactEntry(tx, ae)
	})
}

func getArtifactsBucket(tx *bolt.Tx) (*bolt.Bucket, error) {
	artifacts := tx.Bucket(bucketKeySociArtifacts)
	if artifacts == nil {
		return nil, ErrArtifactBucketNotFound
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
	ae.ArtifactType = string(artifactBkt.Get(bucketKeyArtifactType))
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
		{bucketKeyArtifactType, []byte(ae.ArtifactType)},
		{bucketKeyCreatedAt, createdAt},
	}

	for _, update := range updates {
		if err := artifactBkt.Put(update.key, update.val); err != nil {
			return err
		}
	}

	return nil
}
