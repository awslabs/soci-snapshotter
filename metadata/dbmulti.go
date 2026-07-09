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

package metadata

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/awslabs/soci-snapshotter/ztoc"
	"github.com/rs/xid"
	bolt "go.etcd.io/bbolt"
)

// DBMultiOptions configures the per-layer bbolt databases created by the
// "db-multi" metadata store. The zero value is valid and applies bbolt's own
// defaults.
type DBMultiOptions struct {
	// BoltOptions are passed to bolt.Open for each per-layer database. If nil,
	// bbolt defaults are used.
	BoltOptions *bolt.Options
}

// multiReader wraps the standard bbolt-backed reader but owns the bolt.DB it
// runs against. Unlike the shared "db" store, "db-multi" opens a separate
// database file per layer, so concurrent layer initializations do not contend
// on a single bbolt writer lock. Because the database is per-layer (not shared
// across the daemon), multiReader is responsible for closing the handle and
// removing the file when the reader is closed.
type multiReader struct {
	Reader          // the underlying *reader
	db     *bolt.DB // owned by this multiReader (nil for clones)
	dbPath string   // path of the database file to remove on close (empty for clones)
}

var _ Reader = (*multiReader)(nil)

// NewMultiReader parses a TOC and persists filesystem metadata to a freshly
// created, per-layer bbolt database under dir. The database file is removed
// when Close is called. It uses the same on-disk format and code path as the
// shared "db" store; the only difference is that each layer gets its own
// database, avoiding the single-writer-lock contention of the shared store.
func NewMultiReader(dir string, opts DBMultiOptions) func(*io.SectionReader, ztoc.TOC, ...Option) (Reader, error) {
	return func(sr *io.SectionReader, toc ztoc.TOC, rOpts ...Option) (Reader, error) {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return nil, fmt.Errorf("failed to create metadata dir %q: %w", dir, err)
		}
		// A unique, collision-free filename per layer database.
		dbPath := filepath.Join(dir, "metadata-"+xid.New().String()+".db")
		db, err := bolt.Open(dbPath, 0600, opts.BoltOptions)
		if err != nil {
			return nil, fmt.Errorf("failed to open per-layer metadata db %q: %w", dbPath, err)
		}
		r, err := NewReader(db, sr, toc, rOpts...)
		if err != nil {
			// Clean up the freshly created db on failure.
			db.Close()
			os.Remove(dbPath)
			return nil, err
		}
		return &multiReader{Reader: r, db: db, dbPath: dbPath}, nil
	}
}

// Clone returns a reader that shares the same underlying per-layer database but
// does not own it: closing a clone must not close the shared handle or remove
// the file (only the owning multiReader does that on Close).
func (r *multiReader) Clone(sr *io.SectionReader) (Reader, error) {
	inner, err := r.Reader.Clone(sr)
	if err != nil {
		return nil, err
	}
	return &multiReader{Reader: inner}, nil
}

// NumOfNodes reports the number of nodes in the underlying reader, if it
// supports it. It exists so db-multi can be exercised by the same reader
// correctness suite as the shared "db" store.
func (r *multiReader) NumOfNodes() (int, error) {
	if n, ok := r.Reader.(interface{ NumOfNodes() (int, error) }); ok {
		return n.NumOfNodes()
	}
	return 0, fmt.Errorf("underlying reader does not support NumOfNodes")
}

// Close closes the underlying reader and, if this reader owns the database
// (i.e. it is not a clone), closes the bolt handle and removes the file.
func (r *multiReader) Close() error {
	rErr := r.Reader.Close()
	if r.db != nil {
		if cErr := r.db.Close(); cErr != nil && rErr == nil {
			rErr = cErr
		}
		if r.dbPath != "" {
			if rmErr := os.Remove(r.dbPath); rmErr != nil && !os.IsNotExist(rmErr) && rErr == nil {
				rErr = rmErr
			}
		}
	}
	return rErr
}
