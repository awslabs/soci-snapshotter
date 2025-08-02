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
	"mime"
	"net/http"
	"net/url"
	"strconv"

	sociremote "github.com/awslabs/soci-snapshotter/fs/remote"
	socihttp "github.com/awslabs/soci-snapshotter/internal/http"
	"github.com/awslabs/soci-snapshotter/soci"
	"github.com/awslabs/soci-snapshotter/soci/store"
	"github.com/awslabs/soci-snapshotter/util/ioutils"
	"github.com/containerd/containerd/reference"
	"github.com/containerd/containerd/remotes/docker"
	"github.com/containerd/log"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"golang.org/x/sync/errgroup"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/registry"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/errcode"
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
	localStore  store.BasicStore
	refspec     reference.Spec
}

// This is a wrapper for the ORAS remote repository.
// We only need this to overwrite the Resolve call.
// By default ORAS will attempt to resolve manifests,
// so this allows us to resolve layers instead.
// However, ORAS uses a HEAD request to get layer info,
// which is not allowed in some repos, so we also
// add a manual retry with a GET call should we
// get a 401 or 403 error.
type orasBlobStore struct {
	*remote.Repository
}

func newRemoteBlobStore(refspec reference.Spec, client *http.Client) (*orasBlobStore, error) {
	repo, err := newRemoteStore(refspec, client)
	if err != nil {
		return nil, fmt.Errorf("cannot create remote store: %w", err)
	}
	return &orasBlobStore{repo}, nil
}

// Logic mostly taken from oras-go. Try to resolve with a HEAD, then a GET request.
// https://github.com/oras-project/oras-go/blob/d51a392ff5432a9090c64ffec6ca6a8690b55e18/registry/remote/repository.go#L944
func (r *orasBlobStore) Resolve(ctx context.Context, reference string) (ocispec.Descriptor, error) {
	ref, err := registry.ParseReference(reference)
	if err != nil {
		return ocispec.Descriptor{}, err
	}
	refDigest, err := ref.Digest()
	if err != nil {
		return ocispec.Descriptor{}, err
	}

	tr := &clientWrapper{r.Client}
	url := sociremote.CraftBlobURL(reference, ref)
	resp, err := sociremote.GetHeader(ctx, url, tr)
	if err != nil {
		return ocispec.Descriptor{}, err
	}

	// Construct the descriptor
	mediaType, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if mediaType == "" {
		mediaType = "application/octet-stream"
	}

	size, err := sociremote.ParseSize(resp)
	if err != nil {
		return ocispec.Descriptor{}, err
	}

	return ocispec.Descriptor{
		MediaType: mediaType,
		Digest:    refDigest,
		Size:      size,
	}, nil
}

// We use our own Fetch function to ensure sensitive information gets redacted from any Fetch calls
func (r *orasBlobStore) Fetch(ctx context.Context, target ocispec.Descriptor) (io.ReadCloser, error) {
	rc, err := r.Repository.Fetch(ctx, target)
	if err != nil {
		switch retErr := err.(type) {
		// Redact URLs from ORAS errors, as they might have sensitive info cached
		case *errcode.ErrorResponse:
			socihttp.RedactHTTPQueryValuesFromURL(retErr.URL)
			return nil, retErr
		// Eat URL errors as a malformed URL might still have credentials.
		case *url.Error:
			return nil, errors.New("URL error during fetch")
		// Otherwise it should be safe to print
		default:
			return nil, err
		}
	}
	return rc, nil
}

// doInitialFetch makes a dummy call to the specified content, allowing the authClient
// to make a single request to pre-populate fields for future requests for the same content.
// This is only called in the ParallelPull path as sparse index cases will only ever call each layer sequentially.
func (r *orasBlobStore) doInitialFetch(ctx context.Context, reference string) error {
	ref, err := registry.ParseReference(reference)
	if err != nil {
		return err
	}

	tr := &clientWrapper{r.Client}
	url := sociremote.CraftBlobURL(reference, ref)
	rc, err := sociremote.GetHeaderWithGet(ctx, url, tr)
	if err != nil {
		return fmt.Errorf("error getting header info: %v", err)
	}
	socihttp.Drain(rc.Body)

	return nil
}

// This wrapper is to allow a [remote.Client] to implement the
// [http.RoundTripper] interface by calling Client.Do() in place of RoundTrip.
type clientWrapper struct {
	remote.Client
}

func (c *clientWrapper) RoundTrip(req *http.Request) (*http.Response, error) {
	return c.Client.Do(req)
}

func newRemoteStore(refspec reference.Spec, client *http.Client) (*remote.Repository, error) {
	repo, err := remote.NewRepository(refspec.Locator)
	if err != nil {
		return nil, fmt.Errorf("cannot create repository %s: %w", refspec.Locator, err)
	}
	repo.Client = client
	repo.PlainHTTP, err = docker.MatchLocalhost(refspec.Hostname())
	if err != nil {
		return nil, fmt.Errorf("cannot create repository %s: %w", refspec.Locator, err)
	}

	return repo, nil
}

// Constructs a new artifact fetcher
// Takes in the image reference, the local store and the resolver
func newArtifactFetcher(refspec reference.Spec, localStore store.BasicStore, remoteStore resolverStorage) (*artifactFetcher, error) {
	return &artifactFetcher{
		localStore:  localStore,
		remoteStore: remoteStore,
		refspec:     refspec,
	}, nil
}

// Takes in a descriptor and returns the associated ref to fetch from remote.
// i.e. <hostname>/<repo>@<digest>
func (f *artifactFetcher) constructRef(desc ocispec.Descriptor) string {
	return constructRef(f.refspec, desc)
}

func constructRef(refspec reference.Spec, desc ocispec.Descriptor) string {
	return fmt.Sprintf("%s@%s", refspec.Locator, desc.Digest.String())
}

// Fetches the artifact identified by the descriptor.
// It first checks the local store for the artifact.
// If not found, if constructs the ref and fetches it from remote.
func (f *artifactFetcher) Fetch(ctx context.Context, desc ocispec.Descriptor) (io.ReadCloser, bool, error) {

	// Check local store first
	rc, err := f.localStore.Fetch(ctx, desc)
	if err == nil {
		return rc, true, nil
	}

	log.G(ctx).WithField("digest", desc.Digest.String()).Infof("fetching artifact from remote")
	if desc.Size == 0 {
		// Digest verification fails is desc.Size == 0
		// Therefore, we try to use the resolver to resolve the descriptor
		// and hopefully get the size.
		// Note that the resolve would fail for size > 4MiB, since that's the limit
		// for the manifest size when using the Docker resolver.
		log.G(ctx).WithField("digest", desc.Digest).Warnf("size of descriptor is 0, trying to resolve it...")
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
	if err != nil && !store.IsErrAlreadyExists(err) {
		return fmt.Errorf("unable to push to local store: %w", err)
	}
	return nil
}

func FetchSociArtifacts(ctx context.Context, refspec reference.Spec, indexDesc ocispec.Descriptor, localStore store.Store, remoteStore resolverStorage) (*soci.Index, error) {
	fetcher, err := newArtifactFetcher(refspec, localStore, remoteStore)
	if err != nil {
		return nil, fmt.Errorf("could not create an artifact fetcher: %w", err)
	}

	log.G(ctx).WithField("digest", indexDesc.Digest).Infof("fetching SOCI index from remote registry")

	indexReader, local, err := fetcher.Fetch(ctx, indexDesc)
	if err != nil {
		return nil, fmt.Errorf("unable to fetch SOCI index: %w", err)
	}
	defer indexReader.Close()

	tr := ioutils.NewPositionTrackerReader(indexReader)

	var index soci.Index
	err = soci.DecodeIndex(tr, &index)
	if err != nil {
		return nil, fmt.Errorf("cannot deserialize byte data to index: %w", err)
	}

	desc := ocispec.Descriptor{
		Digest: indexDesc.Digest,
		Size:   tr.CurrentPos(),
	}

	// batch will prevent content from being garbage collected in the middle of the following operations
	ctx, batchDone, err := localStore.BatchOpen(ctx)
	if err != nil {
		return nil, err
	}
	defer batchDone(ctx)

	if !local {
		b, err := soci.MarshalIndex(&index)
		if err != nil {
			return nil, err
		}

		err = localStore.Push(ctx, desc, bytes.NewReader(b))
		if err != nil && !store.IsErrAlreadyExists(err) {
			return nil, fmt.Errorf("unable to store index in local store: %w", err)
		}

		err = store.LabelGCRoot(ctx, localStore, desc)
		if err != nil {
			return nil, fmt.Errorf("unable to label index to prevent garbage collection: %w", err)
		}
	}

	eg, ctx := errgroup.WithContext(ctx)
	for i, blob := range index.Blobs {
		eg.Go(func() error {
			rc, local, err := fetcher.Fetch(ctx, blob)
			if err != nil {
				return fmt.Errorf("cannot fetch artifact: %w", err)
			}
			defer rc.Close()
			if local {
				return nil
			}
			if err := fetcher.Store(ctx, blob, rc); err != nil && !store.IsErrAlreadyExists(err) {
				return fmt.Errorf("unable to store ztoc in local store: %w", err)
			}
			return store.LabelGCRefContent(ctx, localStore, desc, "ztoc."+strconv.Itoa(i), blob.Digest.String())
		})
	}

	if err := eg.Wait(); err != nil {
		return nil, err
	}

	return &index, nil
}
