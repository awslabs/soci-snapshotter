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

package http

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"

	rhttp "github.com/hashicorp/go-retryablehttp"
)

type authRoundTripper struct {
	reqCount         *reqCounter
	expectedUsername string
	expectedPassword string
}

func (arc authRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	arc.reqCount.increment()
	if arc.reqCount.count == 2 {
		user, pass, ok := req.BasicAuth()
		if !ok {
			return nil, fmt.Errorf("missing auth header")
		}
		if user != arc.expectedUsername {
			return nil, fmt.Errorf("unexpected username; expected: %s, got: %s", arc.expectedUsername, user)
		}
		if pass != arc.expectedPassword {
			return nil, fmt.Errorf("unexpected password; expected: %s, got: %s", arc.expectedPassword, pass)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
		}, nil
	}
	return &http.Response{
		StatusCode: http.StatusUnauthorized,
	}, nil
}

type basicAuthHandler struct {
	username     string
	password     string
	encodedBasic string
}

func (m *basicAuthHandler) HandleChallenge(ctx context.Context, resp *http.Response) error {
	// Simulate fetch and cache creds
	auth, _ := base64.StdEncoding.DecodeString(m.encodedBasic)
	s := strings.Split(string(auth), ":")
	m.username = s[0]
	m.password = s[1]
	return nil
}
func (m *basicAuthHandler) AuthorizeRequest(ctx context.Context, req *http.Request) (*http.Request, error) {
	if m.username != "" && m.password != "" {
		req.SetBasicAuth(m.username, m.password)
	}
	return req, nil
}

func TestAuthHandler(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	username := "testuser"
	password := "testpassword"
	baseEncodedAuth := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", username, password)))
	rc := rhttp.NewClient()
	rc.RetryMax = 0
	rc.HTTPClient.Transport = authRoundTripper{reqCount: &reqCounter{}, expectedUsername: username, expectedPassword: password}
	ac, _ := NewAuthClient(&basicAuthHandler{encodedBasic: baseEncodedAuth}, WithRetryableClient(rc))

	req, _ := http.NewRequestWithContext(ctx, "GET", "exampleurl", nil)
	_, err := ac.Do(req)
	if err != nil {
		t.Fatal(err)
	}
}

type policyAuthHandler struct {
	authCount int
}

func (m *policyAuthHandler) HandleChallenge(ctx context.Context, resp *http.Response) error {
	m.authCount = m.authCount + 1
	return nil
}
func (m *policyAuthHandler) AuthorizeRequest(ctx context.Context, req *http.Request) (*http.Request, error) {
	return req, nil
}

type statusRoundTripper struct {
	reqCount      int
	initialStatus int
}

func (srt statusRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	srt.reqCount = srt.reqCount + 1
	if srt.reqCount == 2 {
		return &http.Response{
			StatusCode: http.StatusOK,
		}, nil
	}
	return &http.Response{
		StatusCode: srt.initialStatus,
	}, nil
}

func TestCustomAuthPolicy(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	rc := rhttp.NewClient()
	rc.RetryMax = 0
	rc.HTTPClient.Transport = statusRoundTripper{initialStatus: http.StatusForbidden}

	// Only attempt authentication on 403
	expectedAuthCount := 1
	customAuthPolicy := func(res *http.Response) bool {
		return res.StatusCode == http.StatusForbidden
	}
	authHandler := &policyAuthHandler{}
	ac, _ := NewAuthClient(authHandler, WithAuthPolicy(customAuthPolicy), WithRetryableClient(rc))
	req, _ := http.NewRequestWithContext(ctx, "GET", "exampleurl", nil)
	_, err := ac.Do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if authHandler.authCount != expectedAuthCount {
		t.Fatalf("unexpected auth attempt; expected: %v, got: %v", expectedAuthCount, authHandler.authCount)
	}
}

type headerRoundTripper struct {
	expectedHeaders http.Header
}

func (hrt headerRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	reqHeader := req.Header
	if len(hrt.expectedHeaders) != len(reqHeader) {
		return nil, fmt.Errorf("unequal header length; expected: %v, got: %v", len(hrt.expectedHeaders), len(reqHeader))
	}
	for key := range hrt.expectedHeaders {
		expectedValue := hrt.expectedHeaders.Get(key)
		reqValue := reqHeader.Get(key)
		if reqValue == "" {
			return nil, fmt.Errorf("request header missing key: %s", key)
		}
		if reqValue != expectedValue {
			return nil, fmt.Errorf("unequal value for key: %s; expected: %s, got: %s", key, expectedValue, reqValue)
		}
	}
	return &http.Response{
		StatusCode: http.StatusOK,
	}, nil

}

func TestCustomAuthHeaders(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	customHeader := http.Header{}
	customHeader.Set("User-Agent", "test")
	customHeader.Set("Accept", "text/html")
	ac := SimpleMockAuthClient(&headerRoundTripper{expectedHeaders: customHeader}, customHeader)
	req, _ := http.NewRequestWithContext(ctx, "GET", "exampleurl", nil)
	_, err := ac.Do(req)
	if err != nil {
		t.Fatal(err)
	}
}

type reqCounter struct {
	count int
}

func (rc *reqCounter) increment() {
	rc.count++
}

type emptyAuthHandler struct{}

func (m *emptyAuthHandler) HandleChallenge(ctx context.Context, resp *http.Response) error {
	return nil
}
func (m *emptyAuthHandler) AuthorizeRequest(ctx context.Context, req *http.Request) (*http.Request, error) {
	return req, nil
}

func SimpleMockAuthClient(tr http.RoundTripper, header http.Header) *AuthClient {
	rc := rhttp.NewClient()
	rc.RetryMax = 0
	rc.HTTPClient.Transport = tr
	ac, _ := NewAuthClient(&emptyAuthHandler{}, WithRetryableClient(rc), WithHeader(header))
	return ac
}

// redirectRoundTripper simulates a server that redirects requests.
// On the first request it returns a response whose Request.URL differs
// from the original (simulating a followed redirect). On subsequent
// requests it records the incoming request for inspection.
type redirectRoundTripper struct {
	callCount   int
	redirectURL string
	lastRequest *http.Request
}

func (rt *redirectRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	rt.callCount++
	rt.lastRequest = req

	// Simulate that the HTTP client followed a redirect:
	// resp.Request.URL is the final URL after redirect.
	finalURL, _ := http.NewRequest(req.Method, rt.redirectURL, nil)
	return &http.Response{
		StatusCode: http.StatusOK,
		Request:    finalURL,
	}, nil
}

func TestRedirectCacheSetsRefererHeader(t *testing.T) {
	originalURL := "http://registry.example.com/v2/repo/blobs/sha256:abc123"
	redirectTarget := "http://storage.example.com/blob?sig=xyz"

	rt := &redirectRoundTripper{redirectURL: redirectTarget}
	rc := rhttp.NewClient()
	rc.RetryMax = 0
	rc.HTTPClient.Transport = rt

	ac, err := NewAuthClient(&emptyAuthHandler{}, WithRetryableClient(rc))
	if err != nil {
		t.Fatal(err)
	}
	ac.CacheRedirects(true)

	ctx := context.Background()

	// First request: populates the redirect cache
	req1, _ := http.NewRequestWithContext(ctx, "GET", originalURL, nil)
	_, err = ac.Do(req1)
	if err != nil {
		t.Fatalf("first request failed: %v", err)
	}

	// Second request: should hit the cache and set Referer
	req2, _ := http.NewRequestWithContext(ctx, "GET", originalURL, nil)
	_, err = ac.Do(req2)
	if err != nil {
		t.Fatalf("second request failed: %v", err)
	}

	// Verify the second request was sent to the redirect target
	if rt.lastRequest.URL.String() != redirectTarget {
		t.Fatalf("expected request URL %q, got %q", redirectTarget, rt.lastRequest.URL.String())
	}

	// Verify the Referer header is set to the original URL
	referer := rt.lastRequest.Header.Get("Referer")
	if referer != originalURL {
		t.Fatalf("expected Referer header %q, got %q", originalURL, referer)
	}
}

type allowlistCaptureRoundTripper struct{ got http.Header }

func (c *allowlistCaptureRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	c.got = req.Header.Clone()
	return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody, Request: req}, nil
}

func TestCustomHeadersAllowlistAtSink(t *testing.T) {
	rc := rhttp.NewClient()
	rc.RetryMax = 0
	rt := &allowlistCaptureRoundTripper{}
	rc.HTTPClient.Transport = rt
	ac, err := NewAuthClient(&emptyAuthHandler{},
		WithRetryableClient(rc), WithHeader(http.Header{}),
		WithAllowedHeaders([]string{"x-request-id"}),
	)
	if err != nil {
		t.Fatal(err)
	}
	// Clones used for blob and mirror requests must retain the allowlist.
	ac = ac.CloneWithNewClient(rc)
	custom := http.Header{}
	custom.Set("x-request-id", "caller-123")
	custom.Set("x-injected", "evil")
	ctx := WithCustomHeaders(context.Background(), custom)
	req, _ := http.NewRequestWithContext(ctx, "GET", "http://example/blob", nil)
	if _, err := ac.Do(req); err != nil {
		t.Fatal(err)
	}
	if got := rt.got.Get("x-request-id"); got != "caller-123" {
		t.Fatalf("allowlisted header dropped: got %q", got)
	}
	if got := rt.got.Get("x-injected"); got != "" {
		t.Fatalf("non-allowlisted header must be dropped at the sink: got %q", got)
	}
}

func TestCustomHeadersDeniedWithoutAllowlist(t *testing.T) {
	rc := rhttp.NewClient()
	rc.RetryMax = 0
	rt := &allowlistCaptureRoundTripper{}
	rc.HTTPClient.Transport = rt
	ac, err := NewAuthClient(&emptyAuthHandler{},
		WithRetryableClient(rc), WithHeader(http.Header{}),
	)
	if err != nil {
		t.Fatal(err)
	}
	custom := http.Header{}
	custom.Set("x-request-id", "caller-123")
	ctx := WithCustomHeaders(context.Background(), custom)
	req, _ := http.NewRequestWithContext(ctx, "GET", "http://example/blob", nil)
	if _, err := ac.Do(req); err != nil {
		t.Fatal(err)
	}
	if got := rt.got.Get("x-request-id"); got != "" {
		t.Fatalf("custom header must be dropped when no allowlist is configured: got %q", got)
	}
}

// hostAuthHandler mimics containerd's docker authorizer: challenges are
// answered per host, and requests are only authorized for challenged hosts.
type hostAuthHandler struct {
	mu    sync.Mutex
	hosts map[string]bool
}

func (h *hostAuthHandler) HandleChallenge(ctx context.Context, resp *http.Response) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.hosts[resp.Request.URL.Host] = true
	return nil
}

func (h *hostAuthHandler) AuthorizeRequest(ctx context.Context, req *http.Request) (*http.Request, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.hosts[req.URL.Host] {
		req.SetBasicAuth("user", "pass")
	}
	return req, nil
}

// crossHostRedirectRoundTripper simulates a registry that redirects to a
// different host which also requires authentication, which in turn redirects
// blobs to pre-signed storage URLs that must not receive credentials.
type crossHostRedirectRoundTripper struct {
	storageAuth string
}

func (rt *crossHostRedirectRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	redirect := func(to string) *http.Response {
		return &http.Response{
			StatusCode: http.StatusTemporaryRedirect,
			Header:     http.Header{"Location": []string{to}},
			Body:       http.NoBody,
			Request:    req,
		}
	}
	switch req.URL.Host {
	case "registry.example.com":
		return redirect("https://proxy.example.net" + req.URL.Path), nil
	case "proxy.example.net":
		if user, pass, ok := req.BasicAuth(); !ok || user != "user" || pass != "pass" {
			return &http.Response{
				StatusCode: http.StatusUnauthorized,
				Header:     http.Header{"Www-Authenticate": []string{"Basic"}},
				Body:       http.NoBody,
				Request:    req,
			}, nil
		}
		return redirect("https://storage.example.org/blob?sig=xyz"), nil
	case "storage.example.org":
		rt.storageAuth = req.Header.Get("Authorization")
		return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody, Request: req}, nil
	}
	return nil, fmt.Errorf("unexpected host %q", req.URL.Host)
}

func TestCrossHostRedirectIsReauthorized(t *testing.T) {
	rt := &crossHostRedirectRoundTripper{}
	rc := rhttp.NewClient()
	rc.RetryMax = 0
	rc.HTTPClient.Transport = rt
	rc.HTTPClient.CheckRedirect = CheckRedirect

	ac, err := NewAuthClient(&hostAuthHandler{hosts: map[string]bool{}}, WithRetryableClient(rc))
	if err != nil {
		t.Fatal(err)
	}

	req, _ := http.NewRequestWithContext(context.Background(), "GET",
		"https://registry.example.com/v2/repo/blobs/sha256:abc123", nil)
	resp, err := ac.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}
	if resp.Request.URL.Host != "storage.example.org" {
		t.Fatalf("expected final host storage.example.org, got %q", resp.Request.URL.Host)
	}
	if rt.storageAuth != "" {
		t.Fatalf("credentials leaked to storage host: %q", rt.storageAuth)
	}
}
