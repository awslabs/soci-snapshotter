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

	"github.com/awslabs/soci-snapshotter/soci/artifacts"
	bolt "go.etcd.io/bbolt"
)

func TestArtifactDBWalk(t *testing.T) {
	db, err := newTestableDb()
	if err != nil {
		t.Fatalf("can't create a test db")
	}
	const (
		dgst1 = "sha256:10d6aec48c0a74635a5f3dc555528c1673afaa21ed6e1270a9a44de66e8ffa55"
		dgst2 = "sha256:20d6a9c48c0a74635a5f3dc555528c1673afaa21ed6e1270a9a44de66e8ffa55"
		dgst3 = "sha256:80d6aec48caaaaaaaa5f3dc555528c1673afaa21ed6e1270a9a44de66e8ffa55"
	)
	entries := []artifacts.Entry{
		{Digest: dgst1, Size: 10, Type: artifacts.EntryTypeIndex},
		{Digest: dgst2, Size: 20, Type: artifacts.EntryTypeIndex},
		{Digest: dgst3, Size: 30, Type: artifacts.EntryTypeLayer},
	}
	for _, entry := range entries {
		if err := db.Write(t.Context(), &entry); err != nil {
			t.Fatalf("can't write entry")
		}
	}
	var walked []*artifacts.Entry
	err = db.Walk(t.Context(), func(e *artifacts.Entry) error {
		walked = append(walked, e)
		return nil
	})
	if err != nil {
		t.Fatalf("walk failed")
	}
	if len(walked) != len(entries) {
		t.Fatalf("expected %d entries, got %d", len(entries), len(walked))
	}
}

func TestArtifactDBFind(t *testing.T) {
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
		dgst5         = "sha256:a7f3c9d2e8b4f1a6c5d8e9f2b3a4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2"
		dgst6         = "sha256:3e8f7a2b9c4d1e6f5a8b7c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9"
		imageDigest   = "sha256:0000000000000000000000000000000000000000000000000000000000000000"
		platformAmd64 = "linux/amd64"
		platformArm64 = "linux/arm64"
	)
	entries := []artifacts.Entry{
		{
			Size:           10,
			Digest:         dgst1,
			OriginalDigest: originalDgst1,
			Location:       "/var/soci-snapshotter/test1",
			Type:           artifacts.EntryTypeIndex,
			ImageDigest:    imageDigest,
			Platform:       platformAmd64,
		},
		{
			Size:           20,
			Digest:         dgst2,
			OriginalDigest: originalDgst1,
			Location:       "/var/soci-snapshotter/test2",
			Type:           artifacts.EntryTypeIndex,
			ImageDigest:    imageDigest,
			Platform:       platformAmd64,
		},
		{
			Size:           15,
			Digest:         dgst3,
			OriginalDigest: originalDgst1,
			Location:       "/var/soci-snapshotter/test3",
			Type:           artifacts.EntryTypeIndex,
			ImageDigest:    imageDigest,
			Platform:       platformAmd64,
		},
		{
			Size:           10,
			Digest:         dgst4,
			OriginalDigest: originalDgst1,
			Location:       "/var/soci-snapshotter/test1",
			Type:           artifacts.EntryTypeLayer,
			ImageDigest:    imageDigest,
			Platform:       platformAmd64,
		},
		{
			Size:           25,
			Digest:         dgst5,
			OriginalDigest: originalDgst3,
			Location:       "/var/soci-snapshotter/test1",
			Type:           artifacts.EntryTypeLayer,
			ImageDigest:    imageDigest,
			Platform:       platformArm64,
			MediaType:      "test1",
		},
		{
			Size:           30,
			Digest:         dgst6,
			OriginalDigest: originalDgst3,
			Location:       "/var/soci-snapshotter/test1",
			Type:           artifacts.EntryTypeLayer,
			ImageDigest:    imageDigest,
			Platform:       platformArm64,
			MediaType:      "test2",
		},
	}
	for _, entry := range entries {
		err = db.Write(t.Context(), &entry)
		if err != nil {
			t.Fatalf("can't put ArtifactEntry to a bucket")
		}
	}

	tests := []struct {
		name             string
		filter           artifacts.FilterFn
		expectedArtifact *artifacts.Entry
	}{
		{
			name:             "test find with digest",
			filter:           artifacts.WithDigest(dgst1),
			expectedArtifact: &entries[0],
		},
		{
			name:             "test find with entry type index",
			filter:           artifacts.WithEntryType(artifacts.EntryTypeIndex),
			expectedArtifact: &entries[0],
		},
		{
			name:             "test find with original digest",
			filter:           artifacts.WithOriginalDigest(originalDgst1),
			expectedArtifact: &entries[0],
		},
		{
			name:             "test find with image digest",
			filter:           artifacts.WithImageDigest(imageDigest),
			expectedArtifact: &entries[0],
		},
		{
			name:             "test find with platform",
			filter:           artifacts.WithPlatform(platformAmd64),
			expectedArtifact: &entries[0],
		},
		{
			name:             "test find with media type",
			filter:           artifacts.WithMediaType("test1"),
			expectedArtifact: &entries[4],
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			artifact, err := db.Find(t.Context(), test.filter)
			if err != nil {
				t.Fatalf("cannot find artifact entry")
			}
			if *artifact != *test.expectedArtifact {
				t.Fatalf("retrieved artifact entry doesn't match")
			}
		})
	}
}

func TestArtifactDBFilter(t *testing.T) {
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
		dgst5         = "sha256:a7f3c9d2e8b4f1a6c5d8e9f2b3a4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2"
		dgst6         = "sha256:3e8f7a2b9c4d1e6f5a8b7c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9"
		imageDigest   = "sha256:0000000000000000000000000000000000000000000000000000000000000000"
		platformAmd64 = "linux/amd64"
		platformArm64 = "linux/arm64"
	)
	entries := []artifacts.Entry{
		{
			Size:           10,
			Digest:         dgst1,
			OriginalDigest: originalDgst1,
			Location:       "/var/soci-snapshotter/test1",
			Type:           artifacts.EntryTypeIndex,
			ImageDigest:    imageDigest,
			Platform:       platformAmd64,
		},
		{
			Size:           20,
			Digest:         dgst2,
			OriginalDigest: originalDgst1,
			Location:       "/var/soci-snapshotter/test2",
			Type:           artifacts.EntryTypeIndex,
			ImageDigest:    imageDigest,
			Platform:       platformAmd64,
		},
		{
			Size:           15,
			Digest:         dgst3,
			OriginalDigest: originalDgst1,
			Location:       "/var/soci-snapshotter/test3",
			Type:           artifacts.EntryTypeIndex,
			ImageDigest:    imageDigest,
			Platform:       platformAmd64,
		},
		{
			Size:           10,
			Digest:         dgst4,
			OriginalDigest: originalDgst1,
			Location:       "/var/soci-snapshotter/test1",
			Type:           artifacts.EntryTypeLayer,
			ImageDigest:    imageDigest,
			Platform:       platformAmd64,
		},
		{
			Size:           25,
			Digest:         dgst5,
			OriginalDigest: originalDgst3,
			Location:       "/var/soci-snapshotter/test1",
			Type:           artifacts.EntryTypeLayer,
			ImageDigest:    imageDigest,
			Platform:       platformArm64,
			MediaType:      "test1",
		},
		{
			Size:           30,
			Digest:         dgst6,
			OriginalDigest: originalDgst3,
			Location:       "/var/soci-snapshotter/test1",
			Type:           artifacts.EntryTypeLayer,
			ImageDigest:    imageDigest,
			Platform:       platformArm64,
			MediaType:      "test2",
		},
	}
	for _, entry := range entries {
		err = db.Write(t.Context(), &entry)
		if err != nil {
			t.Fatalf("can't put ArtifactEntry to a bucket")
		}
	}

	tests := []struct {
		name            string
		filter          artifacts.FilterFn
		expectedLength  int
		expectedEntries []*artifacts.Entry
	}{
		{
			name:            "test filter with digest",
			filter:          artifacts.WithDigest(dgst1),
			expectedEntries: []*artifacts.Entry{&entries[0]},
		},
		{
			name:            "test filter with entry type index",
			filter:          artifacts.WithEntryType(artifacts.EntryTypeIndex),
			expectedEntries: []*artifacts.Entry{&entries[0], &entries[1], &entries[2]},
		},
		{
			name:            "test filter with entry type layer",
			filter:          artifacts.WithEntryType(artifacts.EntryTypeLayer),
			expectedEntries: []*artifacts.Entry{&entries[3], &entries[4], &entries[5]},
		},
		{
			name:            "test filter with original digest",
			filter:          artifacts.WithOriginalDigest(originalDgst1),
			expectedEntries: []*artifacts.Entry{&entries[0], &entries[1], &entries[2], &entries[3]},
		},
		{
			name:            "test filter with image digest",
			filter:          artifacts.WithImageDigest(imageDigest),
			expectedEntries: []*artifacts.Entry{&entries[0], &entries[1], &entries[2], &entries[3], &entries[4], &entries[5]},
		},
		{
			name:            "test filter with platform",
			filter:          artifacts.WithPlatform(platformAmd64),
			expectedEntries: []*artifacts.Entry{&entries[0], &entries[1], &entries[2], &entries[3]},
		},
		{
			name:            "test filter with media type",
			filter:          artifacts.WithMediaType("test1"),
			expectedEntries: []*artifacts.Entry{&entries[4]},
		},
		{
			name:            "test filter with all filters",
			filter:          artifacts.WithAllFilters(artifacts.WithEntryType(artifacts.EntryTypeIndex), artifacts.WithOriginalDigest(originalDgst1)),
			expectedEntries: []*artifacts.Entry{&entries[0], &entries[1], &entries[2]},
		},
		{
			name:            "test filter with any filters",
			filter:          artifacts.WithAnyFilters(artifacts.WithPlatform(platformAmd64), artifacts.WithPlatform(platformArm64)),
			expectedEntries: []*artifacts.Entry{&entries[0], &entries[1], &entries[2], &entries[3], &entries[4], &entries[5]},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			entries, err := db.Filter(t.Context(), test.filter)
			if err != nil {
				t.Fatalf("cannot filter artifact entries")
			}
			if len(entries) != len(test.expectedEntries) {
				t.Fatalf("the length of filtered entries should be %d, but equals to %d", test.expectedLength, len(entries))
			}
			for _, expected := range test.expectedEntries {
				found := false
				for _, actual := range entries {
					if *expected == *actual {
						found = true
						break
					}
				}
				if !found {
					t.Fatalf("expected entry not found: %+v", expected)
				}
			}
		})
	}
}

func TestArtifactDBRemove(t *testing.T) {
	db, err := newTestableDb()
	if err != nil {
		t.Fatalf("can't create a test db")
	}
	const (
		dgst         = "sha256:10d6aec48c0a74635a5f3dc555528c1673afaa21ed6e1270a9a44de66e8ffa55"
		originalDgst = "sha256:1236aec48c0a74635a5f3dc666628c1673afaa21ed6e1270a9a44de66e811111"
		imageDigest  = "sha256:0000000000000000000000000000000000000000000000000000000000000000"
		platform     = "linux/amd64"
	)
	entry := &artifacts.Entry{
		Size:           10,
		Digest:         dgst,
		OriginalDigest: originalDgst,
		Location:       "/var/soci-snapshotter/test",
		Type:           artifacts.EntryTypeIndex,
		ImageDigest:    imageDigest,
		Platform:       platform,
	}
	err = db.Write(t.Context(), entry)
	if err != nil {
		t.Fatalf("can't write entry")
	}
	err = db.Remove(t.Context(), dgst)
	if err != nil {
		t.Fatalf("can't remove entry")
	}
	_, err = db.Get(t.Context(), dgst)
	if err == nil {
		t.Fatalf("entry should not exist after removal")
	}
}

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
	entries := []artifacts.Entry{
		{
			Size:           10,
			Digest:         dgst1,
			OriginalDigest: originalDgst1,
			Location:       "/var/soci-snapshotter/test1",
			Type:           artifacts.EntryTypeIndex,
			ImageDigest:    imageDigest,
			Platform:       platform,
		},
		{
			Size:           20,
			Digest:         dgst2,
			OriginalDigest: originalDgst1,
			Location:       "/var/soci-snapshotter/test2",
			Type:           artifacts.EntryTypeIndex,
			ImageDigest:    imageDigest,
			Platform:       platform,
		},
		{
			Size:           15,
			Digest:         dgst3,
			OriginalDigest: originalDgst3,
			Location:       "/var/soci-snapshotter/test3",
			Type:           artifacts.EntryTypeIndex,
			ImageDigest:    imageDigest,
			Platform:       platform,
		},
		{
			Size:           10,
			Digest:         dgst4,
			OriginalDigest: originalDgst1,
			Location:       "/var/soci-snapshotter/test1",
			Type:           artifacts.EntryTypeLayer,
			ImageDigest:    imageDigest,
			Platform:       platform,
		},
	}
	for _, entry := range entries {
		err = db.Write(t.Context(), &entry)
		if err != nil {
			t.Fatalf("can't put ArtifactEntry to a bucket")
		}
	}

	retrievedEntries, err := db.Filter(t.Context(), artifacts.WithAllFilters(
		artifacts.WithEntryType(artifacts.EntryTypeIndex),
		artifacts.WithOriginalDigest(originalDgst1),
	))
	if err != nil {
		t.Fatalf("could not retrieve artifact entries for original digest %s", originalDgst1)
	}

	if len(retrievedEntries) != 2 {
		t.Fatalf("the length of retrieved entries should be equal to 2, but equals to %d", len(retrievedEntries))
	}

	if *retrievedEntries[0] != entries[0] || *retrievedEntries[1] != entries[1] {
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
	ae := &artifacts.Entry{
		Size:           10,
		Digest:         dgst,
		OriginalDigest: originalDgst,
		Location:       "/var/soci-snapshotter/test",
		Type:           artifacts.EntryTypeIndex,
		ImageDigest:    imageDigest,
		Platform:       platform,
		SpanSize:       10,
	}
	err = db.Write(t.Context(), ae)
	if err != nil {
		t.Fatalf("can't put ArtifactEntry to a bucket")
	}
	readArtifactEntry, err := db.Get(t.Context(), dgst)
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
	ae := artifacts.Entry{
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
