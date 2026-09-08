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
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	socihttp "github.com/awslabs/soci-snapshotter/internal/http"
	"github.com/awslabs/soci-snapshotter/version"
	"github.com/containerd/containerd/v2/core/remotes/docker"
	"github.com/containerd/containerd/v2/pkg/reference"
	rhttp "github.com/hashicorp/go-retryablehttp"
	digest "github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

func TestMirror(t *testing.T) {
	ref := "dummyexample.com/library/test"
	refspec, err := reference.Parse(ref)
	if err != nil {
		t.Fatalf("failed to prepare dummy reference: %v", err)
	}
	var (
		blobDigest = digest.FromString("dummy")
		blobPath   = filepath.Join("/v2", strings.TrimPrefix(refspec.Locator, refspec.Hostname()+"/"), "blobs", blobDigest.String())
		refHost    = refspec.Hostname()
	)

	tests := []struct {
		name     string
		tr       http.RoundTripper
		mirrors  []string
		wantHost string
		error    bool
	}{
		{
			name:     "no-mirror",
			tr:       &sampleRoundTripper{okURLs: []string{refHost}},
			mirrors:  nil,
			wantHost: refHost,
		},
		{
			name:     "valid-mirror",
			tr:       &sampleRoundTripper{okURLs: []string{"mirrorexample.com"}},
			mirrors:  []string{"mirrorexample.com"},
			wantHost: "mirrorexample.com",
		},
		{
			name: "invalid-mirror",
			tr: &sampleRoundTripper{
				withCode: map[string]int{
					"mirrorexample1.com": http.StatusInternalServerError,
					"mirrorexample2.com": http.StatusUnauthorized,
					"mirrorexample3.com": http.StatusNotFound,
				},
				okURLs: []string{"mirrorexample4.com", refHost},
			},
			mirrors: []string{
				"mirrorexample1.com",
				"mirrorexample2.com",
				"mirrorexample3.com",
				"mirrorexample4.com",
			},
			wantHost: "mirrorexample4.com",
		},
		{
			name: "invalid-all-mirror",
			tr: &sampleRoundTripper{
				withCode: map[string]int{
					"mirrorexample1.com": http.StatusInternalServerError,
					"mirrorexample2.com": http.StatusUnauthorized,
					"mirrorexample3.com": http.StatusNotFound,
				},
				okURLs: []string{refHost},
			},
			mirrors: []string{
				"mirrorexample1.com",
				"mirrorexample2.com",
				"mirrorexample3.com",
			},
			wantHost: refHost,
		},
		{
			name: "invalid-hostname-of-mirror",
			tr: &sampleRoundTripper{
				okURLs: []string{`.*`},
			},
			mirrors:  []string{"mirrorexample.com/somepath/"},
			wantHost: refHost,
		},
		{
			name: "redirected-mirror",
			tr: &sampleRoundTripper{
				redirectURL: map[string]string{
					regexp.QuoteMeta(fmt.Sprintf("mirrorexample.com%s", blobPath)): "https://backendexample.com/blobs/" + blobDigest.String(),
				},
				okURLs: []string{`.*`},
			},
			mirrors:  []string{"mirrorexample.com"},
			wantHost: "backendexample.com",
		},
		{
			name:     "fail-all",
			tr:       &sampleRoundTripper{},
			mirrors:  []string{"mirrorexample.com"},
			wantHost: "",
			error:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var regHosts []docker.RegistryHost
			for _, m := range tt.mirrors {
				regHosts = append(regHosts, docker.RegistryHost{
					Client:       &http.Client{Transport: tt.tr},
					Host:         m,
					Scheme:       "https",
					Path:         "/v2",
					Capabilities: docker.HostCapabilityPull,
				})
			}
			regHosts = append(regHosts, docker.RegistryHost{
				Client:       &http.Client{Transport: tt.tr},
				Host:         refHost,
				Scheme:       "https",
				Path:         "/v2",
				Capabilities: docker.HostCapabilityPull,
			})

			fetcher, err := newHTTPFetcher(ctx, &fetcherConfig{
				hosts:   regHosts,
				refspec: refspec,
				desc:    ocispec.Descriptor{Digest: blobDigest},
			})
			if err != nil {
				if tt.error {
					return
				}
				t.Fatalf("failed to resolve reference: %v", err)
			}
			nurl, err := url.Parse(fetcher.realURL)
			if err != nil {
				t.Fatalf("failed to parse url %q: %v", fetcher.realURL, err)
			}
			if nurl.Hostname() != tt.wantHost {
				t.Errorf("invalid hostname %q(%q); want %q",
					nurl.Hostname(), nurl.String(), tt.wantHost)
			}
		})
	}
}

type sampleRoundTripper struct {
	withCode    map[string]int
	redirectURL map[string]string
	okURLs      []string
}

func (tr *sampleRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	for host, code := range tr.withCode {
		if ok, _ := regexp.Match(host, []byte(req.URL.String())); ok {
			return &http.Response{
				StatusCode: code,
				Header:     make(http.Header),
				Body:       io.NopCloser(bytes.NewReader([]byte{})),
				Request:    req,
			}, nil
		}
	}
	for host, rurl := range tr.redirectURL {
		if ok, _ := regexp.Match(host, []byte(req.URL.String())); ok {
			rURL, _ := url.Parse(rurl)
			req.URL = rURL
			return &http.Response{
				Request:    req,
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte{})),
			}, nil
		}
	}
	for _, host := range tr.okURLs {
		if ok, _ := regexp.Match(host, []byte(req.URL.String())); ok {
			header := make(http.Header)
			header.Add("Content-Length", "1")
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     header,
				Body:       io.NopCloser(bytes.NewReader([]byte{0})),
				Request:    req,
			}, nil
		}
	}
	return &http.Response{
		StatusCode: http.StatusNotFound,
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewReader([]byte{})),
		Request:    req,
	}, nil
}

func TestCheck(t *testing.T) {
	tr := &breakRoundTripper{}
	f := &httpFetcher{
		realURL:      "test",
		roundTripper: tr,
	}
	tr.success = true
	if err := f.check(); err != nil {
		t.Errorf("connection failed; wanted to succeed")
	}

	tr.success = false
	if err := f.check(); err == nil {
		t.Errorf("connection succeeded; wanted to fail")
	}
}

type breakRoundTripper struct {
	success bool
}

func (b *breakRoundTripper) RoundTrip(req *http.Request) (res *http.Response, err error) {
	if b.success {
		res = &http.Response{
			StatusCode: http.StatusPartialContent,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewReader([]byte("test"))),
		}
	} else {
		res = &http.Response{
			StatusCode: http.StatusInternalServerError,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewReader([]byte{})),
		}
	}
	return
}

func TestRetry(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	tr := &retryRoundTripper{}
	rclient := rhttp.NewClient()
	rclient.HTTPClient.Transport = tr
	rclient.Backoff = rhttp.DefaultBackoff
	rclient.RetryWaitMin = time.Millisecond
	rclient.RetryWaitMax = 10 * time.Millisecond
	f := &httpFetcher{
		realURL:      "test",
		roundTripper: &rhttp.RoundTripper{Client: rclient},
	}

	regions := []region{{b: 0, e: 1}}

	_, err := f.fetch(ctx, regions, true)

	if err != nil {
		t.Fatalf("unexpected error = %v", err)
	}

	if tr.retryCount != 4 {
		t.Fatalf("unexpected retryCount; expected=4 got=%d", tr.retryCount)
	}
}

type retryRoundTripper struct {
	retryCount int
}

func (r *retryRoundTripper) RoundTrip(req *http.Request) (res *http.Response, err error) {
	defer func() {
		r.retryCount++
	}()

	switch r.retryCount {
	case 0:
		err = fmt.Errorf("dummy error")
	case 1:
		res = &http.Response{
			StatusCode: http.StatusTooManyRequests,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewReader([]byte{})),
		}
	case 2:
		res = &http.Response{
			StatusCode: http.StatusServiceUnavailable,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewReader([]byte{})),
		}
	default:
		header := make(http.Header)
		header.Add("Content-Length", "4")
		res = &http.Response{
			StatusCode: http.StatusOK,
			Header:     header,
			Body:       io.NopCloser(bytes.NewReader([]byte("test"))),
		}
	}
	return
}

type emptyAuthHandler struct{}

func (m *emptyAuthHandler) HandleChallenge(ctx context.Context, resp *http.Response) error {
	return nil
}
func (m *emptyAuthHandler) AuthorizeRequest(ctx context.Context, req *http.Request) (*http.Request, error) {
	return req, nil
}

func TestCustomUserAgent(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	userAgent := fmt.Sprintf("soci-snapshotter/%s", version.Version)
	rt := &userAgentRoundTripper{expectedUserAgent: userAgent}
	header := http.Header{}
	header.Set("User-Agent", userAgent)

	// We need an AuthClient since it its responsible for attaching
	// global headers to requests.
	retryClient := rhttp.NewClient()
	retryClient.HTTPClient.Transport = rt
	ac, _ := socihttp.NewAuthClient(&emptyAuthHandler{}, socihttp.WithRetryableClient(retryClient), socihttp.WithHeader(header))

	f := &httpFetcher{
		realURL:      "dummyregistry",
		roundTripper: ac,
	}
	regions := []region{{b: 0, e: 1}}
	_, err := f.fetch(ctx, regions, true)
	if err != nil {
		t.Fatalf("unexpected error = %v", err)
	}
	if rt.roundTripUserAgent != rt.expectedUserAgent {
		t.Fatalf("unexpected User-Agent; expected %s; got %s", rt.expectedUserAgent, rt.roundTripUserAgent)
	}
}

type userAgentRoundTripper struct {
	expectedUserAgent  string
	roundTripUserAgent string
}

func (u *userAgentRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	u.roundTripUserAgent = req.UserAgent()
	header := make(http.Header)
	header.Add("Content-Length", "4")
	return &http.Response{
		StatusCode: http.StatusOK,
		Request:    req,
		Header:     header,
		Body:       io.NopCloser(bytes.NewReader([]byte("test"))),
	}, nil
}

// headerCaptureRoundTripper records the headers of the request it sees.
type headerCaptureRoundTripper struct{ got http.Header }

func (h *headerCaptureRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	h.got = req.Header.Clone()
	header := make(http.Header)
	header.Add("Content-Length", "4")
	return &http.Response{
		StatusCode: http.StatusOK,
		Request:    req,
		Header:     header,
		Body:       io.NopCloser(bytes.NewReader([]byte("test"))),
	}, nil
}

// newTestAuthClient wraps rt in an AuthClient
func newTestAuthClient(rt http.RoundTripper) *socihttp.AuthClient {
	retryClient := rhttp.NewClient()
	retryClient.HTTPClient.Transport = rt
	ac, _ := socihttp.NewAuthClient(&emptyAuthHandler{}, socihttp.WithRetryableClient(retryClient), socihttp.WithHeader(http.Header{}))
	return ac
}

// TestCustomHeaderReachesWireOnFetch proves a custom header stored on the fetcher
// reaches the outgoing request, and that a reserved custom header does not
// override the byte-range the fetcher sets itself.
func TestCustomHeaderReachesWireOnFetch(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	custom := http.Header{}
	custom.Set("x-request-id", "caller-123")
	custom.Set(socihttp.HeaderRange, "bytes=999-999") // reserved: must be ignored

	rt := &headerCaptureRoundTripper{}
	f := &httpFetcher{
		realURL:       "dummyregistry",
		roundTripper:  newTestAuthClient(rt),
		customHeaders: custom,
	}
	if _, err := f.fetch(ctx, []region{{b: 0, e: 1}}, true); err != nil {
		t.Fatalf("unexpected error = %v", err)
	}
	if got := rt.got.Get("x-request-id"); got != "caller-123" {
		t.Fatalf("custom header not on the wire: got %q", got)
	}
	if got := rt.got.Get(socihttp.HeaderRange); got != "bytes=0-1" {
		t.Fatalf("reserved custom Range overrode the fetcher's own range: got %q", got)
	}
}

// TestMultipleCustomHeadersReachWireOnFetch proves the fetcher carries several
// distinct custom headers at once (customHeaders is an http.Header map), and that
// a reserved name mixed in is dropped while the rest pass through.
func TestMultipleCustomHeadersReachWireOnFetch(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	custom := http.Header{}
	custom.Set("x-request-id", "caller-123")
	custom.Set("x-tenant-id", "tenant-7")
	custom.Set("x-trace", "trace-abc")
	custom.Set(socihttp.HeaderAuthorization, "Basic zzz") // reserved: must be dropped

	rt := &headerCaptureRoundTripper{}
	f := &httpFetcher{
		realURL:       "dummyregistry",
		roundTripper:  newTestAuthClient(rt),
		customHeaders: custom,
	}
	if _, err := f.fetch(ctx, []region{{b: 0, e: 1}}, true); err != nil {
		t.Fatalf("unexpected error = %v", err)
	}
	want := map[string]string{
		"x-request-id": "caller-123",
		"x-tenant-id":  "tenant-7",
		"x-trace":      "trace-abc",
	}
	for name, val := range want {
		if got := rt.got.Get(name); got != val {
			t.Fatalf("custom header %q not on the wire: got %q, want %q", name, got, val)
		}
	}
	if got := rt.got.Get(socihttp.HeaderAuthorization); got != "" {
		t.Fatalf("reserved Authorization must be dropped: got %q", got)
	}
}

// TestCustomHeaderSurvivesCheck proves the header survives check(), which runs on
// its own detached context.Background().
func TestCustomHeaderSurvivesCheck(t *testing.T) {
	custom := http.Header{}
	custom.Set("x-request-id", "caller-123")

	rt := &headerCaptureRoundTripper{}
	f := &httpFetcher{
		realURL:       "dummyregistry",
		roundTripper:  newTestAuthClient(rt),
		customHeaders: custom,
	}
	if err := f.check(); err != nil {
		t.Fatalf("check failed: %v", err)
	}
	if got := rt.got.Get("x-request-id"); got != "caller-123" {
		t.Fatalf("custom header lost on check(): got %q", got)
	}
}

// TestCustomHeaderCapturedFromContext proves the full chain: a header on the
// context passed to newHTTPFetcher is captured onto the fetcher and reaches the
// wire on a later fetch. It fails if resolver.go stops reading the header off the
// context at fetcher creation.
func TestCustomHeaderCapturedFromContext(t *testing.T) {
	refspec, err := reference.Parse("dummyexample.com/library/test")
	if err != nil {
		t.Fatalf("failed to parse reference: %v", err)
	}
	blobDigest := digest.FromString("dummy")

	custom := http.Header{}
	custom.Set("x-request-id", "caller-123")
	ctx := socihttp.WithCustomHeaders(context.Background(), custom)

	rt := &headerCaptureRoundTripper{}
	hosts := []docker.RegistryHost{{
		Client:       &http.Client{Transport: newTestAuthClient(rt)},
		Host:         refspec.Hostname(),
		Scheme:       "https",
		Path:         "/v2",
		Capabilities: docker.HostCapabilityPull,
	}}

	f, err := newHTTPFetcher(ctx, &fetcherConfig{
		hosts:   hosts,
		refspec: refspec,
		desc:    ocispec.Descriptor{Digest: blobDigest, Size: 1},
	})
	if err != nil {
		t.Fatalf("newHTTPFetcher: %v", err)
	}
	if _, err := f.fetch(context.Background(), []region{{b: 0, e: 1}}, true); err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if got := rt.got.Get("x-request-id"); got != "caller-123" {
		t.Fatalf("header not captured from context and sent on the wire: got %q", got)
	}
}

// TestCustomHeaderOverRealServer drives the real fetch path against a real HTTP
// server (a TCP round-trip, not a mock RoundTripper) and asserts the server
// receives x-amzn-requestid on the snapshotter's byte-range requests. This is
// the closest local stand-in for the worker's sigv4 proxy seeing the header.
func TestCustomHeaderOverRealServer(t *testing.T) {
	var mu sync.Mutex
	var seen []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seen = append(seen, r.Header.Get("x-amzn-requestid"))
		mu.Unlock()
		w.Header().Set("Content-Length", "4")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("test"))
	}))
	defer srv.Close()

	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("parse server url: %v", err)
	}
	refspec, err := reference.Parse(u.Host + "/repo:tag")
	if err != nil {
		t.Fatalf("parse ref: %v", err)
	}

	// Seed the header exactly as source.HeadersFromLabels would from the
	// soci.header.x-amzn-requestid label.
	custom := http.Header{}
	custom.Set("x-amzn-requestid", "caller-xyz")
	ctx := socihttp.WithCustomHeaders(context.Background(), custom)

	// A real AuthClient over the real default transport — no mock anywhere.
	retryClient := rhttp.NewClient()
	retryClient.Logger = nil
	ac, _ := socihttp.NewAuthClient(&emptyAuthHandler{}, socihttp.WithRetryableClient(retryClient), socihttp.WithHeader(http.Header{}))
	host := docker.RegistryHost{
		Client:       &http.Client{Transport: ac},
		Host:         u.Host,
		Scheme:       "http",
		Path:         "/v2",
		Capabilities: docker.HostCapabilityPull,
	}

	f, err := newHTTPFetcher(ctx, &fetcherConfig{
		hosts:   []docker.RegistryHost{host},
		refspec: refspec,
		desc:    ocispec.Descriptor{Digest: digest.FromString("dummy"), Size: 1},
	})
	if err != nil {
		t.Fatalf("newHTTPFetcher: %v", err)
	}
	if _, err := f.fetch(ctx, []region{{b: 0, e: 1}}, true); err != nil {
		t.Fatalf("fetch: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(seen) == 0 {
		t.Fatal("server received no requests")
	}
	for i, id := range seen {
		if id != "caller-xyz" {
			t.Fatalf("request %d reached the server WITHOUT x-amzn-requestid: got %q", i, id)
		}
	}
	t.Logf("real server saw x-amzn-requestid=%q on all %d byte-range requests", "caller-xyz", len(seen))
}

func TestParseSize(t *testing.T) {
	tc := []struct {
		name         string
		resp         *http.Response
		expectedSize int64
		expectedErr  bool
	}{
		{
			name: "should return size on 200 OK",
			resp: &http.Response{
				StatusCode: http.StatusOK,
				Header: map[string][]string{
					http.CanonicalHeaderKey("Content-Length"): {"12345"},
				},
			},
			expectedSize: 12345,
		},
		{
			name: "should return size on 206 OK",
			resp: &http.Response{
				StatusCode: http.StatusPartialContent,
				Header: map[string][]string{
					http.CanonicalHeaderKey("Content-Range"): {"bytes 0-1/12345"},
				},
			},
			expectedSize: 12345,
		},
		{
			name: "should return error on 401 Unauthorized",
			resp: &http.Response{
				StatusCode: http.StatusUnauthorized,
			},
			expectedErr: true,
		},
	}

	failOrNotFail := func(expectedErr bool) string {
		if expectedErr {
			return "fail"
		}
		return "not fail"
	}

	for _, tt := range tc {
		t.Run(tt.name, func(t *testing.T) {
			size, err := ParseSize(tt.resp)
			if (err == nil) == tt.expectedErr {
				t.Fatalf("expected status code %d to %s", tt.resp.StatusCode, failOrNotFail(tt.expectedErr))
			}
			if size != tt.expectedSize {
				t.Fatalf("expected size %d, got %d", tt.expectedSize, size)
			}
		})
	}
}

type failHeadRequestRoundTripper struct {
	failHeadRequest    []string
	succeedHeadRequest []string
}

func (rt failHeadRequestRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	failResp := &http.Response{
		StatusCode: http.StatusUnauthorized,
		Request:    req,
		Body:       io.NopCloser(&bytes.Buffer{}),
	}

	successResp := &http.Response{
		StatusCode: http.StatusOK,
		Request:    req,
		Body:       io.NopCloser(&bytes.Buffer{}),
	}

	switch req.Method {
	case http.MethodHead:
		for _, url := range rt.failHeadRequest {
			if req.URL.String() == url {
				return failResp, nil
			}
		}
		for _, url := range rt.succeedHeadRequest {
			if req.URL.String() == url {
				return successResp, nil
			}
		}
		return nil, fmt.Errorf("got HEAD request but link not found in arrays")

	case http.MethodGet:
		return successResp, nil
	default:
		return nil, fmt.Errorf("method %s not supported", req.Method)
	}
}

func TestGetHeader(t *testing.T) {
	rt := failHeadRequestRoundTripper{
		failHeadRequest:    []string{"failheadrequest.com"},
		succeedHeadRequest: []string{"successheadrequest.com"},
	}

	tc := []struct {
		name string
		link string
	}{
		{
			name: "HEAD request to link succeeds",
			link: "successheadrequest.com",
		},
		{
			name: "HEAD request to link fails, falls back to GET",
			link: "failheadrequest.com",
		},
	}

	for _, tt := range tc {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			_, err := GetHeader(ctx, tt.link, rt)
			if err != nil {
				t.Fatalf("could not fetch header")
			}
		})
	}
}
