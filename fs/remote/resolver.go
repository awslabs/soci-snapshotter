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

/*
   Copyright The containerd Authors.

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

/*
   Copyright 2019 The Go Authors. All rights reserved.
   Use of this source code is governed by a BSD-style
   license that can be found in the NOTICE.md file.
*/

package remote

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/awslabs/soci-snapshotter/cache"
	"github.com/awslabs/soci-snapshotter/config"
	commonmetrics "github.com/awslabs/soci-snapshotter/fs/metrics/common"
	socihttp "github.com/awslabs/soci-snapshotter/internal/http"
	"github.com/awslabs/soci-snapshotter/service/resolver"
	"github.com/containerd/containerd/reference"
	"github.com/containerd/containerd/remotes/docker"
	"github.com/containerd/log"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/registry"
)

type fetcher interface {
	fetch(ctx context.Context, rs []region, retry bool) (multipartReadCloser, error)
	check() error
	genID(reg region) string
}
type Handler interface {
	Handle(ctx context.Context, desc ocispec.Descriptor) (fetcher Fetcher, size int64, err error)
}

type fetcherConfig struct {
	hosts        []docker.RegistryHost
	refspec      reference.Spec
	desc         ocispec.Descriptor
	fetchTimeout time.Duration
	maxRetries   int
	minWait      time.Duration
	maxWait      time.Duration
}

type Resolver struct {
	blobConfig config.BlobConfig
	handlers   map[string]Handler
}

func NewResolver(cfg config.BlobConfig, handlers map[string]Handler) *Resolver {
	return &Resolver{
		blobConfig: cfg,
		handlers:   handlers,
	}
}

func (r *Resolver) Resolve(ctx context.Context, hosts []docker.RegistryHost, refspec reference.Spec, desc ocispec.Descriptor, blobCache cache.BlobCache) (Blob, error) {

	var (
		validInterval = time.Duration(r.blobConfig.ValidInterval) * time.Second
		fetchTimeout  = time.Duration(r.blobConfig.FetchTimeoutSec) * time.Second
		minWait       = time.Duration(r.blobConfig.MinWaitMsec) * time.Millisecond
		maxWait       = time.Duration(r.blobConfig.MaxWaitMsec) * time.Millisecond
		maxRetries    = r.blobConfig.MaxRetries
	)

	f, size, err := r.resolveFetcher(ctx, &fetcherConfig{
		hosts:        hosts,
		refspec:      refspec,
		desc:         desc,
		fetchTimeout: fetchTimeout,
		maxRetries:   maxRetries,
		minWait:      minWait,
		maxWait:      maxWait,
	})
	if err != nil {
		return nil, err
	}
	return makeBlob(
			f,
			size,
			time.Now(),
			validInterval,
			r),
		nil
}

func (r *Resolver) resolveFetcher(ctx context.Context, fc *fetcherConfig) (f fetcher, size int64, err error) {
	var handlersErr error
	for name, p := range r.handlers {
		// TODO: allow to configure the selection of readers based on the hostname in refspec
		r, size, err := p.Handle(ctx, fc.desc)
		if err != nil {
			handlersErr = errors.Join(handlersErr, err)
			continue
		}
		log.G(ctx).WithField("handler name", name).WithField("ref", fc.refspec.String()).WithField("digest", fc.desc.Digest).
			Debugf("contents is provided by a handler")
		return &remoteFetcher{r}, size, nil
	}

	logger := log.G(ctx)
	if handlersErr != nil {
		logger = logger.WithError(handlersErr)
	}
	logger.WithField("ref", fc.refspec.String()).WithField("digest", fc.desc.Digest).Debugf("using default handler")

	hf, err := newHTTPFetcher(ctx, fc)
	if err != nil {
		return nil, 0, err
	}
	if fc.desc.Size == 0 {
		logger.WithField("ref", fc.refspec.String()).WithField("digest", fc.desc.Digest).
			Debugf("layer size not found in labels; making a request to remote to get size")

		fc.desc.Size, err = getLayerSize(ctx, hf)
		if err != nil {
			return nil, 0, fmt.Errorf("%w from %s: %w", ErrFailedToRetrieveLayerSize, socihttp.RedactHTTPQueryValuesFromString(hf.realURL), err)
		}
	}
	if r.blobConfig.ForceSingleRangeMode {
		hf.singleRangeMode()
	}
	return hf, fc.desc.Size, err
}

type httpFetcher struct {
	roundTripper http.RoundTripper
	scope        string
	// registryURL is the distribution spec compliant blob URL.
	registryURL string
	// realURL is the real blob URL. For registries, with single storage
	// backends it is the same as registryURL.
	realURL       string
	urlMu         sync.Mutex
	digest        digest.Digest
	singleRange   bool
	singleRangeMu sync.Mutex
}

func newHTTPFetcher(ctx context.Context, fc *fetcherConfig) (*httpFetcher, error) {
	desc := fc.desc
	if desc.Digest.String() == "" {
		return nil, fmt.Errorf("missing digest; a digest is mandatory in layer descriptor")
	}
	digest := desc.Digest

	pullScope, err := docker.RepositoryScope(fc.refspec, false)
	if err != nil {
		return nil, err
	}

	// Try to create a fetcher
	var createFetcherErr error
	for _, host := range fc.hosts {
		if host.Host == "" || strings.Contains(host.Host, "/") {
			createFetcherErr = errors.Join(
				fmt.Errorf("%w: (host %q, ref:%q, digest:%q)",
					ErrInvalidHost, host.Host, fc.refspec, digest),
				createFetcherErr,
			)
			// Try another
			continue
		}

		tr := host.Client.Transport
		if authClient, ok := tr.(*socihttp.AuthClient); ok {
			// Get the inner retryable client.
			retryClient := authClient.Client()
			// If the Blob specific HTTP configurations are different
			// than the ones present in our retryable client, we will
			// need to create a new one.
			if retryClient.RetryMax != fc.maxRetries ||
				retryClient.RetryWaitMin != fc.minWait ||
				retryClient.RetryWaitMax != fc.maxWait ||
				retryClient.HTTPClient.Timeout != fc.fetchTimeout {

				// Get the inner concrete HTTP client.
				standardClient := retryClient.HTTPClient
				if globalTransport, ok := standardClient.Transport.(*http.Transport); ok {
					newRetryClient := resolver.CloneRetryableClient(retryClient)
					// Set new retry options/timeout
					newRetryClient.RetryMax = fc.maxRetries
					newRetryClient.RetryWaitMin = fc.minWait
					newRetryClient.RetryWaitMax = fc.maxWait
					newRetryClient.HTTPClient.Timeout = fc.fetchTimeout
					// Re-use the same transport so we can use a single
					// global connection pool.
					newRetryClient.HTTPClient.Transport = globalTransport
					// Create a new AuthClient with the same authentication
					// policies.
					tr = authClient.CloneWithNewClient(newRetryClient)
				}
			}
		}

		registryURL := fmt.Sprintf("%s://%s/%s/blobs/%s",
			host.Scheme,
			path.Join(host.Host, host.Path),
			strings.TrimPrefix(fc.refspec.Locator, fc.refspec.Hostname()+"/"),
			digest,
		)

		// Get the real blob URL
		ctx = docker.WithScope(ctx, pullScope)
		realURL, err := redirect(ctx, registryURL, tr)
		if err != nil {
			createFetcherErr = errors.Join(
				fmt.Errorf("%w: %w (host %q, ref:%q, digest:%q)",
					ErrFailedToRedirect, err, host.Host, fc.refspec, digest),
				createFetcherErr,
			)
			// Try another
			continue
		}

		// Hit one destination
		return &httpFetcher{
			roundTripper: tr,
			scope:        pullScope,
			registryURL:  registryURL,
			realURL:      realURL,
			digest:       digest,
		}, nil
	}

	return nil, fmt.Errorf("%w: %w", ErrUnableToCreateFetcher, createFetcherErr)
}

func (f *httpFetcher) fetch(ctx context.Context, rs []region, retry bool) (multipartReadCloser, error) {
	ctx = docker.WithScope(ctx, f.scope)
	if len(rs) == 0 {
		return nil, ErrNoRegion
	}

	singleRangeMode := f.isSingleRangeMode()

	// squash requesting regions for reducing the total size of request header
	// (servers generally have limits for the size of headers)
	// TODO: when our request has too many ranges, we need to divide it into
	//       multiple requests to avoid huge header.
	var s regionSet
	for _, reg := range rs {
		s.add(reg)
	}
	requests := s.rs
	if singleRangeMode {
		// Squash requests if the layer doesn't support multi range.
		requests = []region{superRegion(requests)}
	}

	// Request to the registry
	f.urlMu.Lock()
	url := f.realURL
	f.urlMu.Unlock()
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	var ranges string
	for _, reg := range requests {
		ranges += fmt.Sprintf("%d-%d,", reg.b, reg.e)
	}
	req.Header.Add("Range", fmt.Sprintf("bytes=%s", ranges[:len(ranges)-1]))
	req.Header.Add("Accept-Encoding", "identity")

	// Recording the roundtrip latency for remote registry GET operation.
	start := time.Now()
	res, err := f.roundTripper.RoundTrip(req)
	commonmetrics.MeasureLatencyInMilliseconds(commonmetrics.RemoteRegistryGet, f.digest, start)
	if err != nil {
		return nil, err
	}

	switch res.StatusCode {
	case http.StatusOK:
		// We are getting the whole blob in one part (= status 200)
		size, err := strconv.ParseInt(res.Header.Get("Content-Length"), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrCannotParseContentLength, err)
		}
		return newSinglePartReader(region{0, size - 1}, res.Body), nil
	case http.StatusPartialContent:
		mediaType, params, err := mime.ParseMediaType(res.Header.Get("Content-Type"))
		if err != nil {
			return nil, fmt.Errorf("%w: invalid media type %q: %w", ErrCannotParseContentType, mediaType, err)
		}
		if strings.HasPrefix(mediaType, "multipart/") {
			// We are getting a set of regions as a multipart body.
			return newMultiPartReader(res.Body, params["boundary"]), nil
		}
		// We are getting single range
		reg, _, err := parseRange(res.Header.Get("Content-Range"))
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrCannotParseContentRange, err)
		}
		return newSinglePartReader(reg, res.Body), nil
	case http.StatusUnauthorized, http.StatusForbidden:
		// 401 response: The underlying AuthClient should have already handled a 401 response.
		// This may indicate token expiry for the blob URL, so we will refresh the URL.
		// 403 response: Although a 403 response generally indicates authorization issues that
		// cannot be resolved client-side, we will still attempt a URL refresh as a last resort.
		if retry {
			log.G(ctx).Infof("Received status code: %v. Refreshing URL and retrying...", res.Status)
			if err := f.refreshURL(ctx); err != nil {
				return nil, fmt.Errorf("%w: status %v: %w", ErrFailedToRefreshURL, res.Status, err)
			}
			return f.fetch(ctx, rs, false)
		}
	case http.StatusBadRequest:
		// gcr.io (https://storage.googleapis.com) returns 400 on multi-range request (2020 #81)
		if retry && !singleRangeMode {
			log.G(ctx).Infof("Received status code: %v. Setting single range mode and retrying...", res.Status)
			// fallback and retry with  range request mode
			f.singleRangeMode()
			return f.fetch(ctx, rs, false)
		}
	}
	return nil, fmt.Errorf("%w on fetch: %v", ErrUnexpectedStatusCode, res.Status)
}

func (f *httpFetcher) check() error {
	ctx := context.Background()
	f.urlMu.Lock()
	url := f.realURL
	f.urlMu.Unlock()
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("check failed: %w", err)
	}
	req.Header.Set("Range", "bytes=0-1")
	res, err := f.roundTripper.RoundTrip(req)
	if err != nil {
		return fmt.Errorf("check failed: %w: %w", ErrRequestFailed, err)
	}
	defer socihttp.Drain(res.Body)
	if res.StatusCode == http.StatusOK || res.StatusCode == http.StatusPartialContent {
		return nil
	} else if res.StatusCode == http.StatusForbidden {
		// Try to re-redirect this blob
		rCtx := context.Background()
		if err := f.refreshURL(rCtx); err == nil {
			return nil
		}
		return fmt.Errorf("%w: status %v", ErrFailedToRefreshURL, res.Status)
	}

	return fmt.Errorf("%w on check: %v", ErrUnexpectedStatusCode, res.StatusCode)
}

func (f *httpFetcher) refreshURL(ctx context.Context) error {
	newRealURL, err := redirect(ctx, f.registryURL, f.roundTripper)
	if err != nil {
		return err
	}
	f.urlMu.Lock()
	f.realURL = newRealURL
	f.urlMu.Unlock()
	return nil
}

func (f *httpFetcher) genID(reg region) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s-%d-%d", f.registryURL, reg.b, reg.e)))
	return fmt.Sprintf("%x", sum)
}

func (f *httpFetcher) singleRangeMode() {
	f.singleRangeMu.Lock()
	f.singleRange = true
	f.singleRangeMu.Unlock()
}

func (f *httpFetcher) isSingleRangeMode() bool {
	f.singleRangeMu.Lock()
	r := f.singleRange
	f.singleRangeMu.Unlock()
	return r
}

// redirect sends a GET request to a given endpoint with a given http.RoundTripper and
// returns the final URL in a redirect chain.
func redirect(ctx context.Context, blobURL string, tr http.RoundTripper) (string, error) {
	// We use GET request for redirect.
	// gcr.io returns 200 on HEAD without Location header (2020).
	// ghcr.io returns 200 on HEAD without Location header (2020).
	req, err := http.NewRequestWithContext(ctx, "GET", blobURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Range", "bytes=0-1")

	// The underlying http.Client will follow up to 10 redirects.
	// See: https://pkg.go.dev/net/http#Get
	res, err := tr.RoundTrip(req)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrRequestFailed, err)
	}
	defer socihttp.Drain(res.Body)

	// If we get an OK response, return the request URL.
	if res.StatusCode/100 == 2 {
		return res.Request.URL.String(), nil
	}
	// If we still get a redirection, return the redirect URL.
	if redir := res.Header.Get("Location"); redir != "" && res.StatusCode/100 == 3 {
		return redir, nil
	}
	return "", fmt.Errorf("%w on redirect %v", ErrUnexpectedStatusCode, res.StatusCode)
}

func CraftBlobURL(reference string, ref registry.Reference) string {
	return fmt.Sprintf("%s://%s/v2/%s/blobs/%s", resolver.DefaultScheme(reference), ref.Host(), ref.Repository, ref.Reference)
}

func GetHeader(ctx context.Context, realURL string, rt http.RoundTripper) (*http.Response, error) {
	statusCodes := []int{0, 0}

	// Some repos do not allow us to make HEAD calls (e.g.
	// ECR Public does not allow HEAD calls with default credentials),
	// so try twice — once with HEAD, once with GET.
	methods := []string{http.MethodHead, http.MethodGet}
	for i, method := range methods {
		req, err := http.NewRequestWithContext(ctx, method, realURL, nil)
		if err != nil {
			return nil, err
		}
		if method == http.MethodGet {
			req.Header.Set("Range", "bytes=0-1")
		}

		resp, err := rt.RoundTrip(req)
		if err != nil {
			return nil, err
		}
		socihttp.Drain(resp.Body)

		statusCodes[i] = resp.StatusCode
		if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusPartialContent {
			return resp, nil
		}
	}

	return nil, fmt.Errorf("failed to get header with code (HEAD=%v, GET=%v)",
		statusCodes[0], statusCodes[1])
}

// getLayerSize gets the size of a layer by sending a request to the registry
// and examining the Content-Length or Content-Range headers.
func getLayerSize(ctx context.Context, hf *httpFetcher) (int64, error) {
	resp, err := GetHeader(ctx, hf.realURL, hf.roundTripper)
	if err != nil {
		return 0, fmt.Errorf("%w: %w", ErrRequestFailed, err)
	}

	return ParseSize(resp)
}

type multipartReadCloser interface {
	Next() (region, io.Reader, error)
	Close() error
}

func newSinglePartReader(reg region, rc io.ReadCloser) multipartReadCloser {
	return &singlepartReader{
		r:      rc,
		Closer: rc,
		reg:    reg,
	}
}

type singlepartReader struct {
	io.Closer
	r      io.Reader
	reg    region
	called bool
}

func (sr *singlepartReader) Next() (region, io.Reader, error) {
	if !sr.called {
		sr.called = true
		return sr.reg, sr.r, nil
	}
	return region{}, nil, io.EOF
}

func newMultiPartReader(rc io.ReadCloser, boundary string) multipartReadCloser {
	return &multipartReader{
		m:      multipart.NewReader(rc, boundary),
		Closer: rc,
	}
}

type multipartReader struct {
	io.Closer
	m *multipart.Reader
}

func (sr *multipartReader) Next() (region, io.Reader, error) {
	p, err := sr.m.NextPart()
	if err != nil {
		return region{}, nil, err
	}
	reg, _, err := parseRange(p.Header.Get("Content-Range"))
	if err != nil {
		return region{}, nil, fmt.Errorf("%w: %w", ErrCannotParseContentRange, err)
	}
	return reg, p, nil
}

func ParseSize(resp *http.Response) (int64, error) {
	if resp.StatusCode == http.StatusOK {
		return strconv.ParseInt(resp.Header.Get("Content-Length"), 10, 64)
	} else if resp.StatusCode == http.StatusPartialContent {
		_, size, err := parseRange(resp.Header.Get("Content-Range"))
		return size, err
	}

	return 0, fmt.Errorf("cannot get size with status code %d", resp.StatusCode)
}

func parseRange(header string) (region, int64, error) {
	submatches := contentRangeRegexp.FindStringSubmatch(header)
	if len(submatches) < 4 {
		return region{}, 0, fmt.Errorf("Content-Range %q doesn't have enough information", header)
	}
	begin, err := strconv.ParseInt(submatches[1], 10, 64)
	if err != nil {
		return region{}, 0, fmt.Errorf("failed to parse beginning offset %q: %w", submatches[1], err)
	}
	end, err := strconv.ParseInt(submatches[2], 10, 64)
	if err != nil {
		return region{}, 0, fmt.Errorf("failed to parse end offset %q: %w", submatches[2], err)
	}
	blobSize, err := strconv.ParseInt(submatches[3], 10, 64)
	if err != nil {
		return region{}, 0, fmt.Errorf("failed to parse blob size %q: %w", submatches[3], err)
	}

	return region{begin, end}, blobSize, nil
}

type Option func(*options)

type options struct {
	ctx       context.Context
	cacheOpts []cache.Option
}

func WithContext(ctx context.Context) Option {
	return func(opts *options) {
		opts.ctx = ctx
	}
}

func WithCacheOpts(cacheOpts ...cache.Option) Option {
	return func(opts *options) {
		opts.cacheOpts = cacheOpts
	}
}

type remoteFetcher struct {
	r Fetcher
}

func (r *remoteFetcher) fetch(ctx context.Context, rs []region, retry bool) (multipartReadCloser, error) {
	var s regionSet
	for _, reg := range rs {
		s.add(reg)
	}
	reg := superRegion(s.rs)
	rc, err := r.r.Fetch(ctx, reg.b, reg.size())
	if err != nil {
		return nil, err
	}
	return newSinglePartReader(reg, rc), nil
}

func (r *remoteFetcher) check() error {
	return r.r.Check()
}

func (r *remoteFetcher) genID(reg region) string {
	return r.r.GenID(reg.b, reg.size())
}

type Fetcher interface {
	Fetch(ctx context.Context, off int64, size int64) (io.ReadCloser, error)
	Check() error
	GenID(off int64, size int64) string
}
