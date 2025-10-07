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
	"os"
	"sync"
	"testing"

	bolt "go.etcd.io/bbolt"
)

func TestGetIndexArtifactEntries(t *testing.T) {
	db, err := newTestableDb()
	if err != nil {
		t.Fatalf("can't create a test db")
	}
	const (
		dgst1         = "sha256:10d6aec48c0a74635a5f3dc555528c1673afaa21ed6e1270a9a44de66e8ffa55"
		originalDgst1 = "sha256:1236aec48c0a74635a5f3dc666628c1673afaa21ed6e1270a9a44de66e811111"
		dgst2         = "sha256:20d6a9c48c0a74635a5f3dc555528c1673afaa21ed6e1270a9a44de66e8ffa55"
		dgst3         = "sha256:80d6aec48caaaaaaaa5f3dc555528c1673afaa21ed6e1270a9a44de66e8ffa55"
		originalDgst3 = "sha256:bbbbbbb48c0a74635a5f3dc666628c1673afaa21ed6e1270a9a44de66e811111"
		dgst4         = "sha256:99d6aec48caaaaaaaa5f3dc555528c1673afaa21ed6e1270a9a44de66e8ffa55"
		imageDigest   = "sha256:0000000000000000000000000000000000000000000000000000000000000000"
		platform      = "linux/amd64"
	)
	entries := []ArtifactEntry{
		{
			Size:           10,
			Digest:         dgst1,
			OriginalDigest: originalDgst1,
			Location:       "/var/soci-snapshotter/test1",
			Type:           ArtifactEntryTypeIndex,
			ImageDigest:    imageDigest,
			Platform:       platform,
		},
		{
			Size:           20,
			Digest:         dgst2,
			OriginalDigest: originalDgst1,
			Location:       "/var/soci-snapshotter/test2",
			Type:           ArtifactEntryTypeIndex,
			ImageDigest:    imageDigest,
			Platform:       platform,
		},
		{
			Size:           15,
			Digest:         dgst3,
			OriginalDigest: originalDgst3,
			Location:       "/var/soci-snapshotter/test3",
			Type:           ArtifactEntryTypeIndex,
			ImageDigest:    imageDigest,
			Platform:       platform,
		},
		{
			Size:           10,
			Digest:         dgst4,
			OriginalDigest: originalDgst1,
			Location:       "/var/soci-snapshotter/test1",
			Type:           ArtifactEntryTypeLayer,
			ImageDigest:    imageDigest,
			Platform:       platform,
		},
	}
	for _, entry := range entries {
		err = db.WriteArtifactEntry(&entry)
		if err != nil {
			t.Fatalf("can't put ArtifactEntry to a bucket")
		}
	}

	retrievedEntries, err := db.getIndexArtifactEntries(originalDgst1)
	if err != nil {
		t.Fatalf("could not retrieve artifact entries for original digest %s", originalDgst1)
	}

	if len(retrievedEntries) != 2 {
		t.Fatalf("the length of retrieved entries should be equal to 2, but equals to %d", len(retrievedEntries))
	}

	if retrievedEntries[0] != entries[0] || retrievedEntries[1] != entries[1] {
		t.Fatalf("the retrieved content should match to the original content")
	}
}

func TestArtifactDbPath(t *testing.T) {
	tests := []struct {
		name     string
		root     string
		expected string
	}{
		{
			name:     "default",
			root:     "",
			expected: "/var/lib/soci-snapshotter-grpc/artifacts.db",
		},
		{
			name:     "custom",
			root:     "/tmp",
			expected: "/tmp/artifacts.db",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := ArtifactsDbPath(test.root); got != test.expected {
				t.Errorf("ArtifactsDbPath() = %v, want %v", got, test.expected)
			}
		})
	}
}

func TestArtifactDB_DoesNotExist(t *testing.T) {
	resetArtifactDBInit()
	t.Cleanup(resetArtifactDBInit)
	once.Do(func() {
		// Fail db initialization.
		db = nil
	})
	_, err := NewDB(ArtifactsDbPath(t.TempDir()))
	if err == nil {
		t.Fatalf("getArtifactEntry should fail since artifacts.db doesn't exist")
	}
}

func TestArtifactEntry_ReadWrite_Using_ArtifactsDb(t *testing.T) {
	db, err := newTestableDb()
	if err != nil {
		t.Fatalf("can't create a test db")
	}
	var (
		dgst         = "sha256:80d6aec48c0a74635a5f3dc555528c1673afaa21ed6e1270a9a44de66e8ffa55"
		originalDgst = "sha256:1236aec48c0a74635a5f3dc666628c1673afaa21ed6e1270a9a44de66e811111"
		imageDigest  = "sha256:0000000000000000000000000000000000000000000000000000000000000000"
		platform     = "linux/amd64"
	)
	ae := &ArtifactEntry{
		Size:           10,
		Digest:         dgst,
		OriginalDigest: originalDgst,
		Location:       "/var/soci-snapshotter/test",
		Type:           ArtifactEntryTypeIndex,
		ImageDigest:    imageDigest,
		Platform:       platform,
		SpanSize:       10,
	}
	err = db.WriteArtifactEntry(ae)
	if err != nil {
		t.Fatalf("can't put ArtifactEntry to a bucket")
	}
	readArtifactEntry, err := db.GetArtifactEntry(dgst)
	if err != nil {
		t.Fatalf("cannot get artifact entry with the digest=%s", dgst)
	}
	if *ae != *readArtifactEntry {
		t.Fatalf("the retrieved artifact entry is not valid")
	}
}

func TestArtifactEntry_ReadWrite_AtomicDbOperations(t *testing.T) {
	db, err := newTestableDb()
	if err != nil {
		t.Fatalf("can't create a test db")
	}
	var (
		dgst         = "sha256:80d6aec48c0a74635a5f3dc106328c1673afaa21ed6e1270a9a44de66e8ffa55"
		originalDgst = "sha256:1236aec48c0a74635a5f3dc106328c1673afaa21ed6e1270a9a44de66e811111"
		imageDigest  = "sha256:0000000000000000000000000000000000000000000000000000000000000000"
		platform     = "linux/amd64"
	)
	ae := ArtifactEntry{
		Size:           10,
		Digest:         dgst,
		OriginalDigest: originalDgst,
		Location:       "/var/soci-snapshotter/test",
		ImageDigest:    imageDigest,
		Platform:       platform,
		SpanSize:       10,
	}
	err = db.db.Update(func(tx *bolt.Tx) error {
		root, err := getArtifactsBucket(tx)
		if err != nil {
			return err
		}
		err = putArtifactEntry(root, &ae)
		return err
	})
	if err != nil {
		t.Fatalf("can't put ArtifactEntry to a bucket")
	}
	db.db.View(func(tx *bolt.Tx) error {
		root, err := getArtifactsBucket(tx)
		if err != nil {
			return err
		}
		readArtifactEntry, err := getArtifactEntryByDigest(root, dgst)
		if err != nil {
			t.Fatalf("cannot get artifact entry with the digest=%s", dgst)
			return err
		}
		if ae != *readArtifactEntry {
			t.Fatalf("the retrieved artifact entry is not valid")
		}

		return nil
	})
}

func newTestableDb() (*ArtifactsDb, error) {
	f, err := os.CreateTemp("", "readertestdb")
	if err != nil {
		return nil, err
	}
	defer f.Close()
	defer os.Remove(f.Name())
	db, err := bolt.Open(f.Name(), 0600, nil)
	if err != nil {
		return nil, err
	}
	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(bucketKeySociArtifacts)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &ArtifactsDb{db: db}, nil
}

func resetArtifactDBInit() {
	once = sync.Once{}
	db = nil
}
