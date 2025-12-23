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

package artifacts

import (
	"context"
	"time"

	"github.com/awslabs/soci-snapshotter/soci/store"
	"github.com/containerd/containerd/content"
)

// ArtifactStore is a store for SOCI artifact metadata
type ArtifactStore interface {
	// GetArtifactEntry loads a single ArtifactEntry from the ArtifactStore by digest
	GetArtifactEntry(digest string) (*ArtifactEntry, error)
	// GetArtifactType gets Type of an ArtifactEntry from the ArtifactStore by digest
	GetArtifactType(digest string) (ArtifactEntryType, error)
	// WriteArtifactEntry stores a single ArtifactEntry into the ArtifactStore.
	// If there is already an artifact in the ArtifactStore with the same Digest,
	// the old data is overwritten.
	WriteArtifactEntry(entry *ArtifactEntry) error
	// Walk applys a function to all ArtifactEntries in the ArtifactStore
	Walk(fn func(*ArtifactEntry) error) error
	// GetIndexArtifactEntries returns all the artifact entries matching the indexDigest
	GetIndexArtifactEntries(indexDigest string) ([]ArtifactEntry, error)
	// RemoveArtifactEntryByIndexDigest removes an index's artifact entry using its digest
	RemoveArtifactEntryByIndexDigest(digest []byte) error
	// GetArtifactEntriesByImageDigest returns all index digests greated from a given image digest
	GetArtifactEntriesByImageDigest(digest string) ([][]byte, error)
	// updateSociV2ArtifactReference updates the image and manifest digests associated with the SOCI index digest in the artifacts db.
	// the indexDigest is the SOCI index to updated, the manifestDigest is the specific image manifest that the SOCI index is bound to,
	// and the imageDigest is the target of the image (this is the same as the manifestDigest for single platform images, but different for mult-platform)
	UpdateSociV2ArtifactReference(indexDigest string, manifestDigest string, imageDigest string) error
}

// RemoteArtifactStore is a remote (w.r.t containerd) store for SOCI artifact metadata
type RemoteArtifactStore interface {
	ArtifactStore
	// SyncWithLocalStore will sync the artifacts databse with SOCIs local content store, either adding new or removing old artifacts.
	SyncWithLocalStore(ctx context.Context, blobStore store.Store, blobStorePath string, cs content.Store) error
	// RemoveOldArtifacts will remove any artifacts from the artifacts database that
	// no longer exist in content store.
	RemoveOldArtifacts(ctx context.Context, blobStore store.Store) error
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
	// Span Size used to generate the SOCI artifact.
	SpanSize int64
}

// ArtifactEntryType is the type of SOCI artifact represented by the ArtifactEntry
type ArtifactEntryType string

const (
	// ArtifactEntryTypeIndex indicates that an ArtifactEntry is a SOCI index artifact
	ArtifactEntryTypeIndex ArtifactEntryType = "soci_index"
	// ArtifactEntryTypeLayer indicates that an ArtifactEntry is a SOCI layer artifact
	ArtifactEntryTypeLayer ArtifactEntryType = "soci_layer"
)
