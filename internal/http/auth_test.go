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
	username := "testuser"
	password := "testpassword"
	baseEncodedAuth := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", username, password)))
	rc := rhttp.NewClient()
	rc.RetryMax = 0
	rc.HTTPClient.Transport = authRoundTripper{reqCount: &reqCounter{}, expectedUsername: username, expectedPassword: password}
	ac, _ := NewAuthClient(&basicAuthHandler{encodedBasic: baseEncodedAuth}, WithRetryableClient(rc))

	req, _ := http.NewRequestWithContext(context.Background(), "GET", "exampleurl", nil)
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
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "exampleurl", nil)
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
	customHeader := http.Header{}
	customHeader.Set("User-Agent", "test")
	customHeader.Set("Accept", "text/html")
	ac := SimpleMockAuthClient(&headerRoundTripper{expectedHeaders: customHeader}, customHeader)
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "exampleurl", nil)
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
