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

	"github.com/awslabs/soci-snapshotter/soci/store"
	"github.com/containerd/containerd/content"
)

type WalkFn func(*Entry) error

// Store is a store for SOCI artifact metadata
// TODO: add comments
type Store interface {
	// Get gets an entry from the store by its digest
	Get(ctx context.Context, digest string) (*Entry, error)
	// Write writes an entry into the store
	Write(ctx context.Context, entry *Entry) error
	// Walk walks all entries in the store, calling walkFn for each entry
	// Returning an error from the walkFn, stops the walk.
	Walk(ctx context.Context, walkFn WalkFn) error
	// Remove removes an entry from the store by its digest
	Remove(ctx context.Context, digest string) error
	// Filter filters entries in the store, returning only those that match the filter function
	Filter(ctx context.Context, filterFn FilterFn) ([]*Entry, error)
	// Find finds the first (in storage order) entry in the store that matches the given filter function
	Find(ctx context.Context, filterFn FilterFn) (*Entry, error)
}

// RemoteStore is a remote (w.r.t containerd) store for SOCI artifact metadata
type RemoteStore interface {
	Store
	// Sync will sync the artifacts databse with SOCIs local content store, either adding new or removing old artifacts.
	// TODO: do we really need all these params?
	Sync(ctx context.Context, blobStore store.Store, blobStorePath string, cs content.Store) error
}
