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

package internal

import (
	"context"
	"fmt"
	"io"

	"github.com/containerd/containerd/v2/core/content"
	"github.com/containerd/errdefs"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// contentStoreAdapter wraps a containerd content.Store (a richer interface -
// ReaderAt/Info/Writer - needed by soci.NewIndexBuilder for random-access
// span extraction) so it also satisfies ORAS's much smaller
// content.Storage interface (Fetch/Push/Exists, from oras.land/oras-go/v2/content).
// The two interfaces aren't structurally compatible (different method
// names/signatures), so this adapter exists purely to let
// oraslib.CopyGraph write directly into a containerd content.Store when
// pulling an image from a registry.
type contentStoreAdapter struct {
	cs content.Store
}

// newContentStoreAdapter returns an ORAS content.Storage backed by cs.
func newContentStoreAdapter(cs content.Store) *contentStoreAdapter {
	return &contentStoreAdapter{cs: cs}
}

func (a *contentStoreAdapter) Fetch(ctx context.Context, target ocispec.Descriptor) (io.ReadCloser, error) {
	ra, err := a.cs.ReaderAt(ctx, target)
	if err != nil {
		return nil, err
	}
	return &readAtCloser{
		SectionReader: io.NewSectionReader(ra, 0, target.Size),
		closer:        ra,
	}, nil
}

func (a *contentStoreAdapter) Push(ctx context.Context, expected ocispec.Descriptor, r io.Reader) error {
	ref := fmt.Sprintf("orasadapter-%s", expected.Digest)
	err := content.WriteBlob(ctx, a.cs, ref, r, expected)
	if errdefs.IsAlreadyExists(err) {
		return nil
	}
	return err
}

func (a *contentStoreAdapter) Exists(ctx context.Context, target ocispec.Descriptor) (bool, error) {
	_, err := a.cs.Info(ctx, target.Digest)
	if err == nil {
		return true, nil
	}
	if errdefs.IsNotFound(err) {
		return false, nil
	}
	return false, err
}

// readAtCloser adapts a content.ReaderAt (Read/ReadAt/Size/Close) into a
// plain io.ReadCloser via io.SectionReader, which only implements Read/Seek.
type readAtCloser struct {
	*io.SectionReader
	closer io.Closer
}

func (r *readAtCloser) Close() error {
	return r.closer.Close()
}
