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

package fs

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/awslabs/soci-snapshotter/service/keychain/dockerconfig"
	"github.com/awslabs/soci-snapshotter/soci"
	socihttp "github.com/awslabs/soci-snapshotter/util/http"
	"github.com/awslabs/soci-snapshotter/util/ioutils"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/reference"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"golang.org/x/sync/errgroup"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/errdef"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
)

type Fetcher interface {
	// Fetch fetches the artifact identified by the descriptor. It first checks the local content store
	// and returns a `ReadCloser` from there. Otherwise it fetches from the remote, saves in the local content store
	// and then returns a `ReadCloser`.
	Fetch(ctx context.Context, desc ocispec.Descriptor) (io.ReadCloser, bool, error)
	// Store takes in a descriptor and io.Reader and stores it in the local store.
	Store(ctx context.Context, desc ocispec.Descriptor, reader io.Reader) error
}
type resolverStorage interface {
	content.Resolver
	content.Storage
}

// artifactFetcher is responsible for fetching and storing artifacts in the provided artifact store.
type artifactFetcher struct {
	remoteStore resolverStorage
	localStore  content.Storage
	refspec     reference.Spec
}

// Constructs a new artifact fetcher
// Takes in the image reference, the local store and the resolver
func newArtifactFetcher(refspec reference.Spec, localStore content.Storage, remoteStore resolverStorage) (*artifactFetcher, error) {
	return &artifactFetcher{
		localStore:  localStore,
		remoteStore: remoteStore,
		refspec:     refspec,
	}, nil
}

func newRemoteStore(refspec reference.Spec) (*remote.Repository, error) {
	repo, err := remote.NewRepository(refspec.Locator)
	if err != nil {
		return nil, fmt.Errorf("cannot create repository %s: %w", refspec.Locator, err)
	}

	authClient := auth.Client{
		Client: socihttp.NewRetryableClient(socihttp.NewRetryableClientConfig()),
		Cache:  auth.DefaultCache,
		Credential: func(_ context.Context, host string) (auth.Credential, error) {
			username, secret, err := dockerconfig.DockerCreds(host)
			if err != nil {
				return auth.EmptyCredential, err
			}
			if username == "" && secret != "" {
				return auth.Credential{
					RefreshToken: secret,
				}, nil
			}
			return auth.Credential{
				Username: username,
				Password: secret,
			}, nil
		},
	}
	repo.Client = &authClient
	return repo, nil
}

// Takes in a descriptor and returns the associated ref to fetch from remote.
// i.e. <hostname>/<repo>@<digest>
func (f *artifactFetcher) constructRef(desc ocispec.Descriptor) string {
	return fmt.Sprintf("%s@%s", f.refspec.Locator, desc.Digest.String())
}

type nopCloser struct {
	io.Reader
}

func (nopCloser) Close() error { return nil }

func NopCloser(r io.Reader) io.ReadCloser {
	return nopCloser{r}
}

// Fetches the artifact identified by the descriptor.
// It first checks the local store for the artifact.
// If not found, if constructs the ref and fetches it from remote.
func (f *artifactFetcher) Fetch(ctx context.Context, desc ocispec.Descriptor) (io.ReadCloser, bool, error) {
	// Try to read the requested artifact from the local filesystem first.
	// This is faster and lets us bypass all of the container registry interaction when available.
	// TODO(iain): pass this directory as an argument or parameter.
	localFilename := "/var/lib/soci-snapshotter-grpc/content/blobs/sha256/" + desc.Digest.Encoded()
	if _, err := os.Stat(localFilename); err == nil {
		file, err := os.Open(localFilename)
		if err != nil {
			return nil, false, fmt.Errorf("error reading local file %s: %w", localFilename, err)
		}
		log.G(ctx).WithField("digest", desc.Digest).Debug("fetched artifact on local filesystem, skipping local oras store and remote fetch")
		return NopCloser(file), true, nil
	} else {
		log.G(ctx).WithField("digest", desc.Digest).Debug("failed to locate artifact on local filesystem, falling back to local oras store")
	}

	// Check local oras store first
	rc, err := f.localStore.Fetch(ctx, desc)
	if err == nil {
		log.G(ctx).WithField("digest", desc.Digest).Debug("fetched artifact from local oras store, skipping remote fetch")
		return rc, true, nil
	}

	log.G(ctx).WithField("digest", desc.Digest.String()).Debug("fetching artifact from remote")
	if desc.Size == 0 {
		// Digest verification fails is desc.Size == 0
		// Therefore, we try to use the resolver to resolve the descriptor
		// and hopefully get the size.
		// Note that the resolve would fail for size > 4MiB, since that's the limit
		// for the manifest size when using the Docker resolver.
		log.G(ctx).WithField("digest", desc.Digest).Warn("size of descriptor is 0, trying to resolve it...")
		desc, err = f.resolve(ctx, desc)
		if err != nil {
			return nil, false, fmt.Errorf("size of descriptor is 0; unable to resolve: %w", err)
		}
	}
	rc, err = f.remoteStore.Fetch(ctx, desc)
	if err != nil {
		return nil, false, fmt.Errorf("unable to fetch descriptor (%v) from remote store: %w", desc.Digest, err)
	}

	return rc, false, nil
}

func (f *artifactFetcher) resolve(ctx context.Context, desc ocispec.Descriptor) (ocispec.Descriptor, error) {
	ref := f.constructRef(desc)
	desc, err := f.remoteStore.Resolve(ctx, ref)
	if err != nil {
		return desc, fmt.Errorf("unable to resolve ref (%s): %w", ref, err)
	}
	return desc, nil
}

// Store takes in an descriptor and io.Reader and stores it in the local store.
func (f *artifactFetcher) Store(ctx context.Context, desc ocispec.Descriptor, reader io.Reader) error {
	err := f.localStore.Push(ctx, desc, reader)
	if err != nil {
		return fmt.Errorf("unable to push to local store: %w", err)
	}
	return nil
}

func FetchSociArtifacts(ctx context.Context, refspec reference.Spec, indexDesc ocispec.Descriptor, localStore content.Storage, remoteStore resolverStorage) (*soci.Index, error) {

	fetcher, err := newArtifactFetcher(refspec, localStore, remoteStore)
	if err != nil {
		return nil, fmt.Errorf("could not create an artifact fetcher: %w", err)
	}

	log.G(ctx).WithField("digest", indexDesc.Digest).Debug("fetching SOCI index")

	indexReader, local, err := fetcher.Fetch(ctx, indexDesc)
	if err != nil {
		return nil, fmt.Errorf("unable to fetch SOCI index: %w", err)
	}
	defer indexReader.Close()

	cw := new(ioutils.CountWriter)
	tee := io.TeeReader(indexReader, cw)

	var index soci.Index
	err = soci.DecodeIndex(tee, &index)
	if err != nil {
		return nil, fmt.Errorf("cannot deserialize byte data to index: %w", err)
	}

	if !local {
		b, err := soci.MarshalIndex(&index)
		if err != nil {
			return nil, err
		}
		err = localStore.Push(ctx, ocispec.Descriptor{
			Digest: indexDesc.Digest,
			Size:   cw.Size(),
		}, bytes.NewReader(b))

		if err != nil && !errors.Is(err, errdef.ErrAlreadyExists) {
			return nil, fmt.Errorf("unable to store index in local store: %w", err)
		}
	}

	eg, ctx := errgroup.WithContext(ctx)
	for _, blob := range index.Blobs {
		blob := blob
		eg.Go(func() error {
			rc, local, err := fetcher.Fetch(ctx, blob)
			if err != nil {
				return fmt.Errorf("cannot fetch artifact: %w", err)
			}
			defer rc.Close()
			if local {
				return nil
			}
			if err := fetcher.Store(ctx, blob, rc); err != nil && !errors.Is(err, errdef.ErrAlreadyExists) {
				return fmt.Errorf("unable to store ztoc in local store: %w", err)
			}
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return nil, err
	}

	return &index, nil
}
