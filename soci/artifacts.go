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
	"strconv"
	"sync"
	"time"

	"github.com/awslabs/soci-snapshotter/config"
	"github.com/awslabs/soci-snapshotter/soci/artifacts"
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

// ArtifactsDb is a remote (w.r.t containerd), bolt-db based store for
// SOCI artifact metadata that implements the artifacts.RemoteStore interface.
// It stores SOCI artifacts info in the following schema.
//
// - soci_artifacts
//   - soci_artifact_digest: <string>     : bucket for each soci layer keyed by a unique string.
//   - size: <varint>                     : size of the artifact.
//   - originalDigest: <string>           : the digest for the image manifest or layer
//   - imageDigest: <string>              : the digest of the image index
//   - platform: <string>                 : the platform for the index
//   - location: <string>                 : the location of the artifact
//   - type: <string>                     : the type of the artifact (can be either "soci_index" or "soci_layer")
type ArtifactsDb struct {
	db *bolt.DB
}

const (
	artifactsDbName = "artifacts.db"
)

var (
	ErrArtifactBucketNotFound = errors.New("soci_artifacts not found")
	db                        *ArtifactsDb
	once                      sync.Once
	bucketKeySociArtifacts    = []byte("soci_artifacts")
	bucketKeySize             = []byte("size")
	bucketKeyOriginalDigest   = []byte("oci_digest")
	bucketKeyImageDigest      = []byte("image_digest")
	bucketKeyPlatform         = []byte("platform")
	bucketKeyLocation         = []byte("location")
	bucketKeyType             = []byte("type")
	bucketKeyMediaType        = []byte("media_type")
	bucketKeyArtifactType     = []byte("artifact_type")
	bucketKeyCreatedAt        = []byte("created_at")
	bucketKeySpanSize         = []byte("span_size")
	errArtifactBucketNotFound = errors.New("soci_artifacts not found")
)

// Get the default artifacts db path
func ArtifactsDbPath(root string) string {
	if root == "" {
		root = config.DefaultSociSnapshotterRootPath
	}
	return path.Join(root, artifactsDbName)
}

// NewDB returns an instance of an ArtifactsDB
func NewDB(path string) (*ArtifactsDb, error) {
	once.Do(func() {
		f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			log.L.WithError(err).WithField("path", path).Error("Cannot create or open file")
			return
		}
		defer f.Close()
		database, err := bolt.Open(f.Name(), 0600, nil)
		if err != nil {
			log.L.WithError(err).Error("Cannot open the db")
			return
		}
		db = &ArtifactsDb{db: database}
	})

	if db == nil {
		return nil, errors.New("artifacts.db is not available")
	}

	return db, nil
}

func (db *ArtifactsDb) Get(ctx context.Context, digest string) (*artifacts.Entry, error) {
	var entry *artifacts.Entry
	err := db.db.View(func(tx *bolt.Tx) error {
		var err error
		entry, err = db.getArtifactEntry(tx, digest)
		return err
	})

	return entry, err
}

func (db *ArtifactsDb) Write(ctx context.Context, entry *artifacts.Entry) error {
	if entry == nil {
		return fmt.Errorf("no entry to write")
	}
	return db.db.Update(func(tx *bolt.Tx) error {
		return db.writeArtifactEntry(tx, entry)
	})
}

func (db *ArtifactsDb) Walk(ctx context.Context, walkFn artifacts.WalkFn) error {
	err := db.db.View(func(tx *bolt.Tx) error {
		bucket, err := getArtifactsBucket(tx)
		if err != nil {
			return nil
		}
		return bucket.ForEachBucket(func(k []byte) error {
			artifactBkt := bucket.Bucket(k)
			ae, err := loadArtifact(artifactBkt, string(k))
			if err != nil {
				return err
			}
			return walkFn(ae)
		})
	})
	return err
}

func (db *ArtifactsDb) Remove(ctx context.Context, digest string) error {
	return db.db.Update(func(tx *bolt.Tx) error {
		bucket, err := getArtifactsBucket(tx)
		if err != nil {
			return err
		}

		dgstBucket := bucket.Bucket([]byte(digest))
		if dgstBucket == nil {
			return fmt.Errorf("the index of the digest %s doesn't exist", digest)
		}
		return bucket.DeleteBucket([]byte(digest))
	})
}

func (db *ArtifactsDb) Filter(ctx context.Context, filterFn artifacts.FilterFn) ([]*artifacts.Entry, error) {
	var filtered []*artifacts.Entry
	err := db.db.View(func(tx *bolt.Tx) error {
		bucket, err := getArtifactsBucket(tx)
		if err != nil {
			return nil
		}
		return bucket.ForEachBucket(func(k []byte) error {
			artifactBkt := bucket.Bucket(k)
			ae, err := loadArtifact(artifactBkt, string(k))
			if err != nil {
				return err
			}
			if filterFn(ae) {
				filtered = append(filtered, ae)
			}
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	return filtered, nil
}

func (db *ArtifactsDb) Find(ctx context.Context, filterFn artifacts.FilterFn) (*artifacts.Entry, error) {
	var found *artifacts.Entry
	err := db.db.View(func(tx *bolt.Tx) error {
		bucket, err := getArtifactsBucket(tx)
		if err != nil {
			return nil
		}
		return bucket.ForEachBucket(func(k []byte) error {
			artifactBkt := bucket.Bucket(k)
			ae, err := loadArtifact(artifactBkt, string(k))
			if err != nil {
				return err
			}
			if filterFn(ae) {
				found = ae
				return errors.New("found") // return error to stop iteration
			}
			return nil
		})
	})
	if err != nil && err.Error() != "found" {
		return nil, err
	}
	return found, nil
}

func (db *ArtifactsDb) Sync(ctx context.Context, blobStore store.Store, blobStorePath string, cs content.Store) error {
	if err := db.removeOldArtifacts(ctx, blobStore); err != nil {
		return fmt.Errorf("failed to remove old artifacts from db: %w", err)
	}
	if err := db.addNewArtifacts(ctx, blobStorePath, cs); err != nil {
		return fmt.Errorf("failed to add new artifacts to db: %w", err)
	}
	return nil
}

// NOTE: Removing buckets while iterating (bucket.ForEach) causes unexpected
// behavior (see: https://github.com/boltdb/bolt/issues/426).
// This implementation works around this issue by appending buckets to a slice when
// iterating and removing them after.
func (db *ArtifactsDb) removeOldArtifacts(ctx context.Context, blobStore store.Store) error {
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
		if errors.Is(err, fs.ErrNotExist) {
			// This check attempts to mitigate a race between finding content and getting said content.
			// If content is provided there's no guarantee it will still be there by the time we want to fetch it.
			// Returning should be safe as it's effectively a no-op.
			return nil
		} else if err != nil {
			return err
		}
		// skip: entry is an empty config
		if info.Size() < 10 {
			return nil
		}
		f, err := os.Open(path)
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		} else if err != nil {
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
		ae, err := db.Get(ctx, indexDigest)
		if err != nil && !errors.Is(err, errArtifactBucketNotFound) && !errors.Is(err, errdefs.ErrNotFound) {
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

			indexEntry := &artifacts.Entry{
				Size:           info.Size(),
				Digest:         indexDigest,
				OriginalDigest: manifestDigest,
				ImageDigest:    manifestDigest,
				Platform:       platforms.Format(platform[0]),
				Type:           artifacts.EntryTypeIndex,
				Location:       manifestDigest,
				MediaType:      sociIndex.MediaType,
				ArtifactType:   sociIndex.Config.MediaType,
				CreatedAt:      time.Now(),
			}
			if err = db.Write(ctx, indexEntry); err != nil {
				return err
			}
			for _, zt := range sociIndex.Blobs {
				var spanSize int64
				spanSizeStr, ok := zt.Annotations[IndexAnnotationSociSpanSize]
				if ok {
					spanSize, err = strconv.ParseInt(spanSizeStr, 10, 64)
					if err != nil {
						return fmt.Errorf("failed to parse span size from annotations for layer  %s: %w", zt.Digest.String(), err)
					}
				}
				ztocEntry := &artifacts.Entry{
					Size:           zt.Size,
					Digest:         zt.Digest.String(),
					OriginalDigest: zt.Annotations[IndexAnnotationImageLayerDigest],
					Type:           artifacts.EntryTypeLayer,
					Location:       zt.Annotations[IndexAnnotationImageLayerDigest],
					MediaType:      SociLayerMediaType,
					CreatedAt:      time.Now(),
					SpanSize:       spanSize,
				}
				if err := db.Write(ctx, ztocEntry); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func (db *ArtifactsDb) getArtifactEntry(tx *bolt.Tx, digest string) (*artifacts.Entry, error) {
	bucket, err := getArtifactsBucket(tx)
	if err != nil {
		return nil, err
	}
	return getArtifactEntryByDigest(bucket, digest)
}

func (db *ArtifactsDb) writeArtifactEntry(tx *bolt.Tx, entry *artifacts.Entry) error {
	bucket, err := tx.CreateBucketIfNotExists(bucketKeySociArtifacts)
	if err != nil {
		return err
	}
	return putArtifactEntry(bucket, entry)
}

func getArtifactsBucket(tx *bolt.Tx) (*bolt.Bucket, error) {
	artifacts := tx.Bucket(bucketKeySociArtifacts)
	if artifacts == nil {
		return nil, errArtifactBucketNotFound
	}

	return artifacts, nil
}

func getArtifactEntryByDigest(artifacts *bolt.Bucket, digest string) (*artifacts.Entry, error) {
	artifactBkt := artifacts.Bucket([]byte(digest))
	if artifactBkt == nil {
		return nil, fmt.Errorf("couldn't retrieve artifact for %s, %w", digest, errdefs.ErrNotFound)
	}
	return loadArtifact(artifactBkt, digest)
}

func loadArtifact(artifactBkt *bolt.Bucket, digest string) (*artifacts.Entry, error) {
	ae := artifacts.Entry{Digest: digest}
	encodedSize := artifactBkt.Get(bucketKeySize)
	size, err := dbutil.DecodeInt(encodedSize)
	if err != nil {
		return nil, err
	}

	var spanSize int64
	encodedSpanSize := artifactBkt.Get(bucketKeySpanSize)
	if len(encodedSpanSize) > 0 {
		spanSize, err = dbutil.DecodeInt(encodedSpanSize)
		if err != nil {
			return nil, err
		}
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
	ae.Type = artifacts.EntryType(artifactBkt.Get(bucketKeyType))
	ae.OriginalDigest = string(artifactBkt.Get(bucketKeyOriginalDigest))
	ae.ImageDigest = string(artifactBkt.Get(bucketKeyImageDigest))
	ae.Platform = string(artifactBkt.Get(bucketKeyPlatform))
	ae.MediaType = string(artifactBkt.Get(bucketKeyMediaType))
	ae.ArtifactType = string(artifactBkt.Get(bucketKeyArtifactType))
	ae.CreatedAt = createdAt
	ae.SpanSize = spanSize
	return &ae, nil
}

func putArtifactEntry(artifacts *bolt.Bucket, ae *artifacts.Entry) error {
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

	spanSizeInBytes, err := dbutil.EncodeInt(ae.SpanSize)
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
		{bucketKeySpanSize, spanSizeInBytes},
	}

	for _, update := range updates {
		if err := artifactBkt.Put(update.key, update.val); err != nil {
			return err
		}
	}

	return nil
}
