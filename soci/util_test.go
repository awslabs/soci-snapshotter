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
	"io"

	"github.com/awslabs/soci-snapshotter/soci/store"
	"github.com/containerd/containerd/v2/core/content"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/content/memory"
)

func parseDigest(digestString string) digest.Digest {
	dgst, _ := digest.Parse(digestString)
	return dgst
}

type OrasMemoryStore struct {
	s *memory.Store
}

func (*OrasMemoryStore) BatchOpen(ctx context.Context) (context.Context, store.CleanupFunc, error) {
	return ctx, store.NopCleanup, nil
}

func (m *OrasMemoryStore) Exists(ctx context.Context, target ocispec.Descriptor) (bool, error) {
	return m.s.Exists(ctx, target)
}

func (m *OrasMemoryStore) Fetch(ctx context.Context, target ocispec.Descriptor) (io.ReadCloser, error) {
	return m.s.Fetch(ctx, target)
}

func (m *OrasMemoryStore) Push(ctx context.Context, expected ocispec.Descriptor, reader io.Reader) error {
	return m.s.Push(ctx, expected, reader)
}

func (m *OrasMemoryStore) Label(ctx context.Context, target ocispec.Descriptor, label string, value string) error {
	return nil
}

func (m *OrasMemoryStore) Delete(ctx context.Context, dgst digest.Digest) error {
	return nil
}

func NewOrasMemoryStore() *OrasMemoryStore {
	return &OrasMemoryStore{
		s: memory.New(),
	}
}

type fakeContentStore struct {
}

// Abort implements content.Store
func (fakeContentStore) Abort(ctx context.Context, ref string) error {
	return nil
}

// ListStatuses implements content.Store
func (fakeContentStore) ListStatuses(ctx context.Context, filters ...string) ([]content.Status, error) {
	panic("unimplemented")
}

// Status implements content.Store
func (fakeContentStore) Status(ctx context.Context, ref string) (content.Status, error) {
	panic("unimplemented")
}

// Writer implements content.Store
func (fakeContentStore) Writer(ctx context.Context, opts ...content.WriterOpt) (content.Writer, error) {
	return fakeWriter{}, nil
}

// Delete implements content.Store
func (fakeContentStore) Delete(ctx context.Context, dgst digest.Digest) error {
	return nil
}

// Info implements content.Store
func (fakeContentStore) Info(ctx context.Context, dgst digest.Digest) (content.Info, error) {
	panic("unimplemented")
}

// Update implements content.Store
func (fakeContentStore) Update(ctx context.Context, info content.Info, fieldpaths ...string) (content.Info, error) {
	panic("unimplemented")
}

// Walk implements content.Store
func (fakeContentStore) Walk(ctx context.Context, fn content.WalkFunc, filters ...string) error {
	return nil
}

// ReaderAt implements content.Store
func (fakeContentStore) ReaderAt(ctx context.Context, desc ocispec.Descriptor) (content.ReaderAt, error) {
	return newFakeReaderAt(desc), nil
}

func newFakeContentStore() content.Store {
	return fakeContentStore{}
}

type fakeReaderAt struct {
	size int64
}

// Close implements content.ReaderAt
func (fakeReaderAt) Close() error {
	return nil
}

// ReadAt implements content.ReaderAt
func (r fakeReaderAt) ReadAt(p []byte, off int64) (n int, err error) {
	return int(r.size), nil
}

// Size implements content.ReaderAt
func (r fakeReaderAt) Size() int64 {
	return r.size
}

func newFakeReaderAt(desc ocispec.Descriptor) content.ReaderAt {
	return fakeReaderAt{size: desc.Size}
}

type fakeWriter struct {
	io.Writer
	status     content.Status
	commitFunc func() error
}

func (f fakeWriter) Write(p []byte) (n int, err error) {
	return len(p), nil
}

func (f fakeWriter) Close() error {
	return nil
}

func (f fakeWriter) Commit(ctx context.Context, size int64, expected digest.Digest, opts ...content.Opt) error {
	if f.commitFunc == nil {
		return nil
	}
	return f.commitFunc()
}

func (f fakeWriter) Digest() digest.Digest {
	return digest.FromString("")
}

func (f fakeWriter) Status() (content.Status, error) {
	return f.status, nil
}

func (f fakeWriter) Truncate(size int64) error {
	return nil
}
