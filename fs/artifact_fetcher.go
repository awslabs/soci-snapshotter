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
	"path"
	"strconv"
	"sync"
	"time"

	commonmetrics "github.com/awslabs/soci-snapshotter/fs/metrics/common"
	sociremote "github.com/awslabs/soci-snapshotter/fs/remote"
	socihttp "github.com/awslabs/soci-snapshotter/internal/http"
	"github.com/awslabs/soci-snapshotter/service/resolver"
	"github.com/awslabs/soci-snapshotter/soci"
	"github.com/awslabs/soci-snapshotter/soci/store"
	"github.com/awslabs/soci-snapshotter/util/ioutils"
	"github.com/containerd/containerd/v2/core/remotes/docker"
	"github.com/containerd/containerd/v2/pkg/reference"
	"github.com/containerd/log"
	"github.com/opencontainers/go-digest"
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

	// hosts are the endpoints blobs are fetched from, in order: configured
	// mirrors first and the image's registry last. When empty, blobs are
	// fetched from the repository's registry only.
	hosts []blobHost
	// hostIndex remembers, per blob digest, which host served the initial
	// fetch so that the ranged fetches for that blob start from the same host.
	hostIndex sync.Map
}

// blobHost is a registry endpoint that can serve an image's blobs:
// a configured mirror or the image's own registry.
type blobHost struct {
	client remote.Client
	scheme string
	host   string
	path   string
	// ns is the image's registry. It is sent to mirrors as the "ns" query
	// parameter, as containerd does, so a mirror knows the upstream registry.
	ns string
}

func (h blobHost) blobURL(repository, dgst string) string {
	u := url.URL{
		Scheme: h.scheme,
		Host:   h.host,
		Path:   path.Join(h.path, repository, "blobs", dgst),
	}
	if h.ns != "" {
		u.RawQuery = url.Values{"ns": []string{h.ns}}.Encode()
	}
	return u.String()
}

// newBlobHosts converts registry hosts, as returned by the resolver with mirrors
// first and the image's registry last, into blob endpoints.
// Hosts without pull capability are skipped.
func newBlobHosts(refspec reference.Spec, hosts []docker.RegistryHost) ([]blobHost, error) {
	registryHost, err := docker.DefaultHost(refspec.Hostname())
	if err != nil {
		return nil, err
	}
	var blobHosts []blobHost
	for _, h := range hosts {
		if h.Capabilities&docker.HostCapabilityPull == 0 {
			continue
		}
		bh := blobHost{
			client: h.Client,
			scheme: h.Scheme,
			host:   h.Host,
			path:   h.Path,
		}
		if bh.client == nil {
			bh.client = http.DefaultClient
		}
		if bh.scheme == "" {
			bh.scheme = resolver.DefaultScheme(h.Host)
		}
		if bh.path == "" {
			bh.path = "/v2"
		}
		if h.Host != registryHost {
			bh.ns = refspec.Hostname()
		}
		blobHosts = append(blobHosts, bh)
	}
	return blobHosts, nil
}

// newRemoteBlobStoreFromHosts creates a blob store that fetches blobs from
// the given registry hosts in order, falling back to the next host when a host
// cannot serve a blob.
func newRemoteBlobStoreFromHosts(refspec reference.Spec, hosts []docker.RegistryHost) (*orasBlobStore, error) {
	blobHosts, err := newBlobHosts(refspec, hosts)
	if err != nil {
		return nil, fmt.Errorf("cannot create blob hosts for %s: %w", refspec.Locator, err)
	}
	if len(blobHosts) == 0 {
		return nil, fmt.Errorf("no registry hosts with pull capability for %s", refspec.Locator)
	}
	registryHost := blobHosts[len(blobHosts)-1]
	client, ok := registryHost.client.(*http.Client)
	if !ok {
		client = http.DefaultClient
	}
	repo, err := newRemoteStore(refspec, client, registryHost.scheme == "http")
	if err != nil {
		return nil, fmt.Errorf("cannot create remote store: %w", err)
	}
	return &orasBlobStore{Repository: repo, hosts: blobHosts}, nil
}

// endpoints returns the hosts to try for a blob, in order.
func (r *orasBlobStore) endpoints(reference string, ref registry.Reference) []blobHost {
	if len(r.hosts) > 0 {
		return r.hosts
	}
	scheme := resolver.DefaultScheme(reference)
	if r.PlainHTTP {
		scheme = "http"
	}
	return []blobHost{{
		client: r.Client,
		scheme: scheme,
		host:   ref.Host(),
		path:   "/v2",
	}}
}

func newRemoteBlobStore(refspec reference.Spec, client *http.Client, plainHTTP bool) (*orasBlobStore, error) {
	repo, err := newRemoteStore(refspec, client, plainHTTP)
	if err != nil {
		return nil, fmt.Errorf("cannot create remote store: %w", err)
	}
	return &orasBlobStore{Repository: repo}, nil
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
	url := sociremote.CraftBlobURL(reference, ref, r.PlainHTTP)
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
	// Try mirrors first. The last host is the image's registry, fetched with ORAS below.
	for i := 0; i+1 < len(r.hosts); i++ {
		rc, err := r.fetchFromHost(ctx, r.hosts[i], target)
		if err == nil {
			return rc, nil
		}
		log.G(ctx).WithField("host", r.hosts[i].host).WithField("digest", target.Digest).WithError(err).
			Debug("cannot fetch blob from mirror, trying next host")
	}
	rc, err := r.Repository.Fetch(ctx, target)
	if err != nil {
		return nil, cleanFetchErrors(err)
	}
	return rc, nil
}

// fetchFromHost fetches a whole blob from a single host.
func (r *orasBlobStore) fetchFromHost(ctx context.Context, h blobHost, target ocispec.Descriptor) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, h.blobURL(r.Reference.Repository, target.Digest.String()), nil)
	if err != nil {
		return nil, err
	}
	resp, err := h.client.Do(req)
	if err != nil {
		return nil, cleanFetchErrors(err)
	}
	if resp.StatusCode != http.StatusOK {
		socihttp.Drain(resp.Body)
		return nil, fmt.Errorf("unexpected status code %d", resp.StatusCode)
	}
	if resp.ContentLength != -1 && resp.ContentLength != target.Size {
		socihttp.Drain(resp.Body)
		return nil, fmt.Errorf("unexpected content length %d, expected %d", resp.ContentLength, target.Size)
	}
	return resp.Body, nil
}

// GetContentWithRange gets the requested content in the byte range [lower, upper]
func GetContentWithRange(ctx context.Context, realURL string, rt http.RoundTripper, lower, upper int64) (*http.Response, error) {
	if lower < 0 || upper < lower {
		return nil, fmt.Errorf("illogical content range [%d, %d]", lower, upper)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, realURL, nil)
	if err != nil {
		return nil, err
	}
	r := fmt.Sprintf("bytes=%d-%d", lower, upper)
	req.Header.Set("Range", r)

	resp, err := rt.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusPartialContent {
		return resp, nil
	}
	socihttp.Drain(resp.Body)
	return nil, fmt.Errorf("error getting range: unexpected status code %d", resp.StatusCode)
}

// FetchRange returns the response body of the range requested.
// This assumes the upstream repo supports ranged GET calls.
// If it does not, return an error.
// TODO: Unify this with the artifact fetching done in fs/remote/resolver.go
func (r *orasBlobStore) FetchRange(ctx context.Context, reference string, lower, upper int64) (io.ReadCloser, error) {
	ref, err := registry.ParseReference(reference)
	if err != nil {
		return nil, err
	}

	hosts := r.endpoints(reference, ref)
	start := 0
	if i, ok := r.hostIndex.Load(ref.Reference); ok {
		start = i.(int)
	}
	var errs error
	for i := start; i < len(hosts); i++ {
		h := hosts[i]
		resp, err := GetContentWithRange(ctx, h.blobURL(ref.Repository, ref.Reference), &clientWrapper{h.client}, lower, upper)
		if err != nil {
			errs = errors.Join(errs, cleanFetchErrors(err))
			continue
		}
		// Check if upstream allows for ranged GET requests
		if rangeUnit := resp.Header.Get("Accept-Ranges"); rangeUnit != "bytes" {
			resp.Body.Close()
			errs = errors.Join(errs, fmt.Errorf("upstream repo does not support ranged GET requests"))
			continue
		}
		return resp.Body, nil
	}
	return nil, errs
}

func cleanFetchErrors(err error) error {
	{
		var retErr *errcode.ErrorResponse
		var retErr1 *url.Error
		switch {
		// Redact URLs from ORAS errors, as they might have sensitive info cached
		case errors.As(err, &retErr):
			socihttp.RedactHTTPQueryValuesFromURL(retErr.URL)
			return retErr
		// Eat URL errors as a malformed URL might still have credentials.
		case errors.As(err, &retErr1):
			return errors.New("URL error during fetch")
		// Otherwise it should be safe to print
		default:
			return err
		}
	}
}

// doInitialFetch makes a dummy call to the specified content, allowing the authClient
// to make a single request to pre-populate fields for future requests for the same content.
// This is only called in the ParallelPull path as sparse index cases will only ever call each layer sequentially.
func (r *orasBlobStore) doInitialFetch(ctx context.Context, reference string) (bool, error) {
	ref, err := registry.ParseReference(reference)
	if err != nil {
		return false, err
	}

	var errs error
	for i, h := range r.endpoints(reference, ref) {
		resp, err := sociremote.GetHeaderWithGet(ctx, h.blobURL(ref.Repository, ref.Reference), &clientWrapper{h.client})
		if err != nil {
			errs = errors.Join(errs, cleanFetchErrors(err))
			log.G(ctx).WithField("host", h.host).WithField("digest", ref.Reference).WithError(err).
				Debug("cannot get blob header from host, trying next host")
			continue
		}
		socihttp.Drain(resp.Body)
		r.hostIndex.Store(ref.Reference, i)

		// Check if upstream allows for ranged GET requests
		return resp.Header.Get("Accept-Ranges") == "bytes", nil
	}
	return false, fmt.Errorf("error getting header info: %w", errs)
}

// This wrapper is to allow a [remote.Client] to implement the
// [http.RoundTripper] interface by calling Client.Do() in place of RoundTrip.
type clientWrapper struct {
	remote.Client
}

func (c *clientWrapper) RoundTrip(req *http.Request) (*http.Response, error) {
	return c.Client.Do(req)
}

func newRemoteStore(refspec reference.Spec, client *http.Client, plainHTTP bool) (*remote.Repository, error) {
	repo, err := remote.NewRepository(refspec.Locator)
	if err != nil {
		return nil, fmt.Errorf("cannot create repository %s: %w", refspec.Locator, err)
	}
	repo.Client = client
	if plainHTTP {
		repo.PlainHTTP = true
	} else {
		repo.PlainHTTP, err = docker.MatchLocalhost(refspec.Hostname())
		if err != nil {
			return nil, fmt.Errorf("cannot create repository %s: %w", refspec.Locator, err)
		}
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
	// Measure the total index+zTOC fetch. This blocks the first Mount for the
	// image, so it is on the pull critical path.
	indexFetchStart := time.Now()
	defer commonmetrics.MeasureLatencyInMilliseconds(commonmetrics.SociIndexFetch, indexDesc.Digest, indexFetchStart)

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
			// Key the metric by the image layer digest the zTOC belongs to so
			// it aligns with the other per-layer metrics; fall back to the zTOC
			// blob digest if the annotation is absent.
			layerDigest := digest.Digest(blob.Annotations[soci.IndexAnnotationImageLayerDigest])
			if layerDigest == "" {
				layerDigest = blob.Digest
			}
			ztocFetchStart := time.Now()
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
			commonmetrics.MeasureLatencyInMilliseconds(commonmetrics.ZtocFetch, layerDigest, ztocFetchStart)
			return store.LabelGCRefContent(ctx, localStore, desc, "ztoc."+strconv.Itoa(i), blob.Digest.String())
		})
	}

	if err := eg.Wait(); err != nil {
		return nil, err
	}

	return &index, nil
}
