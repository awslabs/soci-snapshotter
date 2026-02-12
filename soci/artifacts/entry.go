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

import "time"

// Entry is a metadata object for a SOCI artifact.
type Entry struct {
	// Size is the SOCI artifact's size in bytes.
	Size int64
	// Digest is the SOCI artifact's (ztoc or SOCI index) digest.
	Digest string
	// OriginalDigest is the digest of the content (layer for ztoc, manifest for SOCI index) for which the SOCI artifact was created.
	OriginalDigest string
	// ImageDigest is the digest of the container image that was used to generate the artifact.
	// This is the image manifest's digest for single platform images or the image index's digest for multi-platform images.
	ImageDigest string
	// Platform is the platform for which the artifact was generated.
	Platform string
	// Location is the file path for the SOCI artifact.
	Location string
	// Type is the type of SOCI artifact.
	Type EntryType
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
type EntryType string

const (
	// EntryTypeIndex indicates that an ArtifactEntry is a SOCI index artifact
	EntryTypeIndex EntryType = "soci_index"
	// EntryTypeLayer indicates that an ArtifactEntry is a SOCI layer artifact
	EntryTypeLayer EntryType = "soci_layer"
	// EntryTypePrefetch indicates that an ArtifactEntry is a SOCI prefetch artifact
	EntryTypePrefetch EntryType = "soci_prefetch"
)
