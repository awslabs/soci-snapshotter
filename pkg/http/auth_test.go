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
	"fmt"
	"net/http"
	"testing"

	rhttp "github.com/hashicorp/go-retryablehttp"
)

type emptyAuthHandler struct{}

func (m *emptyAuthHandler) HandleChallenge(ctx context.Context, resp *http.Response) error {
	return nil
}
func (m *emptyAuthHandler) AuthorizeRequest(ctx context.Context, req *http.Request) (*http.Request, error) {
	return req, nil
}

func SimpleMockAuthClient(tr http.RoundTripper, header http.Header) *AuthClient {
	rc := rhttp.NewClient()
	rc.HTTPClient.Transport = tr
	ac, _ := NewAuthClient(&emptyAuthHandler{}, WithRetryableClient(rc), WithHeader(header))
	return ac
}

type authRoundTripper struct {
	reqCount                int
	expectedBasicAuthHeader string
}

func (arc authRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	arc.reqCount = arc.reqCount + 1
	if arc.reqCount == 2 {
		authHeader := req.Header.Get("Authorization")

		if authHeader != arc.expectedBasicAuthHeader {
			return nil, fmt.Errorf("unexpected auth header; expected: %s, got %s", arc.expectedBasicAuthHeader, authHeader)
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
	setCreds       string
	basicAuthCreds string
}

func (m *basicAuthHandler) HandleChallenge(ctx context.Context, resp *http.Response) error {
	m.basicAuthCreds = m.setCreds
	return nil
}
func (m *basicAuthHandler) AuthorizeRequest(ctx context.Context, req *http.Request) (*http.Request, error) {
	authHeader := fmt.Sprintf("Basic %s", m.basicAuthCreds)
	req.Header.Set("Authorization", authHeader)
	return req, nil
}

func TestAuthHandler(t *testing.T) {
	basicAuthCreds := "abcd"
	rt := authRoundTripper{expectedBasicAuthHeader: fmt.Sprintf("Basic %s", basicAuthCreds)}
	rc := rhttp.NewClient()
	rc.HTTPClient.Transport = rt

	ac, _ := NewAuthClient(&basicAuthHandler{setCreds: basicAuthCreds}, WithRetryableClient(rc))

	req, _ := http.NewRequestWithContext(context.Background(), "GET", "exampleurl", nil)
	_, err := ac.Do(req)
	if err != nil {
		t.Fatal(err)
	}
}

type policyAuthHandler struct {
	handleCount int
}

func (m *policyAuthHandler) HandleChallenge(ctx context.Context, resp *http.Response) error {
	m.handleCount = m.handleCount + 1
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
	// Only attempt authentication on 403
	customAuthPolicy := func(res *http.Response) bool {
		return res.StatusCode == http.StatusForbidden
	}
	rt := statusRoundTripper{initialStatus: http.StatusForbidden}
	rc := rhttp.NewClient()
	rc.HTTPClient.Transport = rt

	expectedAuthAttempt := 1

	authHandler := &policyAuthHandler{}
	ac, _ := NewAuthClient(authHandler, WithAuthPolicy(customAuthPolicy), WithRetryableClient(rc))
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "exampleurl", nil)
	_, err := ac.Do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if authHandler.handleCount != expectedAuthAttempt {
		t.Fatalf("unexpected auth attempt; expected %v, got %v", expectedAuthAttempt, authHandler.handleCount)
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
	customHeader := http.Header{}
	customHeader.Set("User-Agent", "test")
	customHeader.Set("Accept", "text/html")
	rt := &headerRoundTripper{expectedHeaders: customHeader}
	ac := SimpleMockAuthClient(rt, customHeader)
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "exampleurl", nil)
	_, err := ac.Do(req)
	if err != nil {
		t.Fatal(err)
	}
}
