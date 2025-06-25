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

package store

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/awslabs/soci-snapshotter/config"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/defaults"
	"github.com/containerd/errdefs"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/content/oci"
)

// BasicStore describes the functionality common to oras-go oci.Store, oras-go memory.Store, and containerd ContentStore.
type BasicStore interface {
	Exists(ctx context.Context, target ocispec.Descriptor) (bool, error)
	Fetch(ctx context.Context, target ocispec.Descriptor) (io.ReadCloser, error)
	Push(ctx context.Context, expected ocispec.Descriptor, reader io.Reader) error
}

// Store extends BasicStore with functionality that in not present in some BasicStore
// implementations and may be stubbed in some Store implementations
type Store interface {
	BasicStore
	Label(ctx context.Context, target ocispec.Descriptor, label string, value string) error
	Delete(ctx context.Context, dgst digest.Digest) error
	// BatchOpen starts a series of operations that should not be interrupted by garbage collection.
	// It returns a cleanup function that ends the batch, which should be called after
	// all associated content operations are finished.
	BatchOpen(ctx context.Context) (context.Context, CleanupFunc, error)
}

type ContentStoreType = config.ContentStoreType

const (
	ContainerdContentStoreType = config.ContainerdContentStoreType
	SociContentStoreType       = config.SociContentStoreType
)

// ContentStoreTypes returns a slice of all supported content store types.
func ContentStoreTypes() []ContentStoreType {
	return []ContentStoreType{SociContentStoreType, ContainerdContentStoreType}
}

const (
	// Default path to containerd content addressable storage
	DefaultContainerdContentStorePath = "/var/lib/containerd/io.containerd.content.v1.content"

	// Default path to soci content addressable storage
	DefaultSociContentStorePath = "/var/lib/soci-snapshotter-grpc/content"
)

func ErrUnknownContentStoreType(contentStoreType ContentStoreType) error {
	return fmt.Errorf("unknown content store type: %s; must be one of %v",
		contentStoreType, ContentStoreTypes())
}

func ErrCouldNotCreateClient(address string) error {
	return fmt.Errorf("could not create containerd client at %s", address)
}

// This struct allows SOCI to create a connection to the containerd on-demand
// instead of on startup, allowing the daemon to start without containerd
type ContainerdClient struct {
	containerdAddress string
	ctdClient         *containerd.Client
	ctdClientLock     *sync.Mutex
}

func NewContainerdClient(containerdAddress string) *ContainerdClient {
	return &ContainerdClient{
		containerdAddress: containerdAddress,
		ctdClientLock:     &sync.Mutex{},
	}
}

func (c *ContainerdClient) Client() (*containerd.Client, error) {
	if c.ctdClient != nil {
		return c.ctdClient, nil
	}
	c.ctdClientLock.Lock()
	defer c.ctdClientLock.Unlock()
	if c.ctdClient == nil {
		var err error
		c.ctdClient, err = containerd.New(c.containerdAddress)
		if err != nil {
			return nil, ErrCouldNotCreateClient(c.containerdAddress)
		}
	}
	return c.ctdClient, nil
}

type ContentStoreConfig struct {
	config.ContentStoreConfig
	ctdClient *ContainerdClient
}

func NewStoreConfig(opts ...Option) ContentStoreConfig {
	storeConfig := ContentStoreConfig{
		ContentStoreConfig: config.ContentStoreConfig{
			Type:              config.DefaultContentStoreType,
			ContainerdAddress: defaults.DefaultAddress,
		},
	}
	for _, o := range opts {
		o(&storeConfig)
	}
	return storeConfig
}

type Option func(*ContentStoreConfig)

func WithType(contentStoreType ContentStoreType) Option {
	return func(sc *ContentStoreConfig) {
		sc.Type = contentStoreType
	}
}

// This func will trim the leading 'unix://' if it is provided
func WithContainerdAddress(address string) Option {
	return func(sc *ContentStoreConfig) {
		sc.ContainerdAddress = config.TrimSocketAddress(address)
	}
}

func WithClient(client *ContainerdClient) Option {
	return func(sc *ContentStoreConfig) {
		sc.ctdClient = client
	}
}

// CanonicalizeContentStoreType resolves the empty string to DefaultContentStoreType,
// returns other types, or errors on unrecognized types.
func CanonicalizeContentStoreType(contentStoreType ContentStoreType) (ContentStoreType, error) {
	switch contentStoreType {
	case "":
		return config.DefaultContentStoreType, nil
	case ContainerdContentStoreType, SociContentStoreType:
		return contentStoreType, nil
	default:
		return "", ErrUnknownContentStoreType(contentStoreType)
	}
}

// GetContentStorePath returns the top level directory for the content store.
func GetContentStorePath(contentStoreType ContentStoreType) (string, error) {
	contentStoreType, err := CanonicalizeContentStoreType(contentStoreType)
	if err != nil {
		return "", err
	}
	switch contentStoreType {
	case ContainerdContentStoreType:
		return DefaultContainerdContentStorePath, nil
	case SociContentStoreType:
		return DefaultSociContentStorePath, nil
	}
	return "", errors.New("unexpectedly reached end of GetContentStorePath")
}

type CleanupFunc func(context.Context) error

func NopCleanup(context.Context) error { return nil }

func NewContentStore(opts ...Option) (Store, error) {
	storeConfig := NewStoreConfig(opts...)

	contentStoreType, err := CanonicalizeContentStoreType(storeConfig.Type)
	if err != nil {
		return nil, err
	}
	switch contentStoreType {
	case ContainerdContentStoreType:
		return NewContainerdStore(storeConfig)
	case SociContentStoreType:
		return NewSociStore()
	}
	return nil, errors.New("unexpectedly reached end of NewContentStore")
}

// SociStore wraps oci.Store and adds or stubs additional functionality of the Store interface.
type SociStore struct {
	*oci.Store
}

// assert that SociStore implements Store
var _ Store = (*SociStore)(nil)

// NewSociStore creates a sociStore.
func NewSociStore() (*SociStore, error) {
	store, err := oci.New(DefaultSociContentStorePath)
	return &SociStore{store}, err
}

// Label is a no-op for sociStore until sociStore and ArtifactsDb are better integrated.
func (s *SociStore) Label(_ context.Context, _ ocispec.Descriptor, _ string, _ string) error {
	return nil
}

// Delete is a no-op for sociStore until oci.Store provides this method.
func (s *SociStore) Delete(_ context.Context, _ digest.Digest) error {
	return nil
}

// BatchOpen is a no-op for sociStore; it does not support batching operations.
func (s *SociStore) BatchOpen(ctx context.Context) (context.Context, CleanupFunc, error) {
	return ctx, NopCleanup, nil
}

type ContainerdStore struct {
	ContentStoreConfig
}

// assert that ContainerdStore implements Store
var _ Store = (*ContainerdStore)(nil)

func NewContainerdStore(storeConfig ContentStoreConfig) (*ContainerdStore, error) {
	containerdStore := ContainerdStore{
		ContentStoreConfig: storeConfig,
	}

	if containerdStore.ctdClient == nil {
		containerdStore.ctdClient = NewContainerdClient(containerdStore.ContainerdAddress)
	}

	return &containerdStore, nil
}

// Exists returns true iff the described content exists.
func (s *ContainerdStore) Exists(ctx context.Context, target ocispec.Descriptor) (bool, error) {
	client, err := s.client()
	if err != nil {
		return false, err
	}
	cs := client.ContentStore()
	_, err = cs.Info(ctx, target.Digest)
	if errors.Is(err, errdefs.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

type sectionReaderAt struct {
	content.ReaderAt
	*io.SectionReader
}

// Fetch fetches the content identified by the descriptor.
func (s *ContainerdStore) Fetch(ctx context.Context, target ocispec.Descriptor) (io.ReadCloser, error) {
	client, err := s.client()
	if err != nil {
		return nil, err
	}
	cs := client.ContentStore()
	ra, err := cs.ReaderAt(ctx, target)
	if err != nil {
		return nil, err
	}
	return sectionReaderAt{ra, io.NewSectionReader(ra, 0, ra.Size())}, nil
}

// Push pushes the content, matching the expected descriptor.
// This should be done within a Batch and followed by Label calls to prevent garbage collection.
func (s *ContainerdStore) Push(ctx context.Context, expected ocispec.Descriptor, reader io.Reader) error {
	exists, err := s.Exists(ctx, expected)
	if err != nil {
		return err
	}
	if exists {
		// To be consistent with content.Copy, return nil if content already exists.
		return nil
	}

	client, err := s.client()
	if err != nil {
		return err
	}
	writer, err := content.OpenWriter(ctx, client.ContentStore(), content.WithRef(expected.Digest.String()))
	if err != nil {
		return err
	}
	defer writer.Close()

	// gRPC message size limit includes some overhead that cannot be calculated from here
	buf := make([]byte, defaults.DefaultMaxRecvMsgSize/2)
	totalWritten := 0

	for {
		n, err := reader.Read(buf)
		if n > 0 {
			written, err := writer.Write(buf[:n])
			if err != nil {
				return err
			}
			totalWritten += written
		}
		if err != nil {
			if err != io.EOF {
				return err
			}
			break
		}
		if n == 0 {
			break
		}
	}

	if expected.Size > 0 && expected.Size != int64(totalWritten) {
		return fmt.Errorf("unexpected copy size %d, expected %d: %w", totalWritten, expected.Size, errdefs.ErrFailedPrecondition)
	}

	return writer.Commit(ctx, expected.Size, expected.Digest)
}

// LabelGCRoot labels the target resource to prevent garbage collection of itself.
func LabelGCRoot(ctx context.Context, store Store, target ocispec.Descriptor) error {
	return store.Label(ctx, target, "containerd.io/gc.root", time.Now().Format(time.RFC3339))
}

// LabelGCRefContent labels the target resource to prevent garbage collection of another resource identified by digest
// with an optional ref to allow and disambiguate multiple content labels.
func LabelGCRefContent(ctx context.Context, store Store, target ocispec.Descriptor, ref string, digest string) error {
	if len(ref) > 0 {
		ref = "." + ref
	}
	return store.Label(ctx, target, "containerd.io/gc.ref.content"+ref, digest)
}

// Label creates or updates the named label with the given value.
func (s *ContainerdStore) Label(ctx context.Context, target ocispec.Descriptor, name string, value string) error {
	client, err := s.client()
	if err != nil {
		return err
	}
	cs := client.ContentStore()
	info := content.Info{
		Digest: target.Digest,
		Labels: map[string]string{name: value},
	}
	paths := []string{"labels." + name}
	_, err = cs.Update(ctx, info, paths...)
	if err != nil {
		return err
	}
	return nil
}

// Delete removes the described content.
func (s *ContainerdStore) Delete(ctx context.Context, dgst digest.Digest) error {
	client, err := s.client()
	if err != nil {
		return err
	}
	cs := client.ContentStore()
	return cs.Delete(ctx, dgst)
}

// BatchOpen creates a lease, ensuring that no content created within the batch will be garbage collected.
// It returns a cleanup function that ends the lease, which should be called after content is created and labeled.
func (s *ContainerdStore) BatchOpen(ctx context.Context) (context.Context, CleanupFunc, error) {
	client, err := s.client()
	if err != nil {
		return ctx, nil, err
	}
	ctx, leaseDone, err := client.WithLease(ctx)
	if err != nil {
		return ctx, NopCleanup, fmt.Errorf("unable to open batch: %w", err)
	}
	return ctx, leaseDone, nil
}

func (s *ContainerdStore) client() (*containerd.Client, error) {
	return s.ctdClient.Client()
}
