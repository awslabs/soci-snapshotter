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
	"net/url"
	"sync"

	rhttp "github.com/hashicorp/go-retryablehttp"
)

// AuthHandler defines an interface for handling challenge-response
// based HTTP authentication.
//
// See: https://datatracker.ietf.org/doc/html/rfc9110#section-11
type AuthHandler interface {
	// HandleChallenge is responsible for parsing the challenge defined
	// by the origin server and preparing a valid response/answer.
	HandleChallenge(context.Context, *http.Response) error
	// AuthorizeRequest is responsible for authorizing the request to be
	// sent to the origin server.
	AuthorizeRequest(context.Context, *http.Request) (*http.Request, error)
}

// AuthPolicy defines an authentication policy. It takes a response
// and determines whether or not it warrants authentication.
type AuthPolicy func(*http.Response) bool

// DefaultAuthPolicy defines the default AuthPolicy, where by only a "401
// Unauthorized" warrants authentication.
var DefaultAuthPolicy = func(resp *http.Response) bool {
	return resp.StatusCode == http.StatusUnauthorized
}

// AuthReqContextFunc takes the original request context and
// returns a context that will be used by the new authenticated
// request.
type AuthReqContextFunc func(reqCtx context.Context) context.Context

// DefaultAuthReqContext is the default AuthReqContextFunc. It returns
// an entirely new context.
var DefaultAuthReqContext = func(reqCtx context.Context) context.Context {
	return context.Background()
}

// AuthClient provides a HTTP client that is capable of authenticating
// with origin servers. It contains an AuthHandler type that is responsible
// for preparing valid responses/answers to challenges as well authenticating
// requests. It wraps an inner retryable client, that is uses to send requests.
//
// Note: The AuthClient does provide a mechanism for caching credentials/tokens,
// but these can expire. Ideally, this should be handled by the underlying AuthHandler.
type AuthClient struct {
	client     *rhttp.Client
	handler    AuthHandler
	policy     AuthPolicy
	getAuthCtx AuthReqContextFunc
	header     http.Header
	init       sync.Once
	redirMap   map[string]string
	redirMu    sync.Mutex
	cacheRedir bool
}

type AuthClientOpt func(*AuthClient)

// WithHeader adds a http.Header to the AuthClient that will
// be attached to every request.
func WithHeader(header http.Header) AuthClientOpt {
	return func(ac *AuthClient) {
		ac.header = header
	}
}

// WithAuthPolicy attaches an AuthPolicy to the AuthClient.
func WithAuthPolicy(policy AuthPolicy) AuthClientOpt {
	return func(ac *AuthClient) {
		ac.policy = policy
	}
}

// WithRetryableClient attaches a retryable client to the AuthClient.
func WithRetryableClient(client *rhttp.Client) AuthClientOpt {
	return func(ac *AuthClient) {
		ac.client = client
	}
}

// WithAuthRequestCtxFunc attaches a AuthReqContextFunc to the AuthClient.
func WithAuthRequestCtxFunc(arc AuthReqContextFunc) AuthClientOpt {
	return func(ac *AuthClient) {
		ac.getAuthCtx = arc
	}
}

// NewAuthClient creates a new AuthClient given an AuthHandler.
//
// An AuthHandler must be provided. If no retryable client is provided
// a default one will be created. If no AuthPolicy is provided the
// DefaultAuthPolicy will be used. If no AuthReqCtxFunc is provided
// the DefaultAuthReqContext is used.
func NewAuthClient(authHandler AuthHandler, opts ...AuthClientOpt) (*AuthClient, error) {
	if authHandler == nil {
		return nil, ErrMissingAuthHandler
	}
	ac := &AuthClient{
		handler: authHandler,
	}
	for _, opt := range opts {
		opt(ac)
	}
	ac.init.Do(ac.initClient)
	return ac, nil
}

// Do sends a request using the underlying retryable client. If no
// error is returned and the AuthPolicy deems that the response
// warrants authentication, it will invoke the AuthHandler to handle
// the challenge, re-authorize and re-send the request.
func (ac *AuthClient) Do(req *http.Request) (*http.Response, error) {
	ac.init.Do(ac.initClient)
	if ac.handler == nil {
		return nil, ErrMissingAuthHandler
	}
	ctx := req.Context()
	roundTrip := func(req *http.Request) (*http.Response, error) {
		// Attach global headers to the request.
		for k := range ac.header {
			req.Header.Set(k, ac.header.Get(k))
		}
		authReq, err := ac.handler.AuthorizeRequest(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrFailedToAuthorizeRequest, err)
		}
		// Convert the auth request to be a "retryable" request.
		rAuthReq, err := rhttp.FromRequest(authReq)
		if err != nil {
			return nil, err
		}
		resp, err := ac.client.Do(rAuthReq)
		if err != nil {
			return nil, err
		}

		if ac.shouldCache(req, resp) {
			ac.redirMu.Lock()
			defer ac.redirMu.Unlock()
			ac.redirMap[req.URL.String()] = resp.Request.URL.String()
		}
		return resp, nil
	}

	if rd := ac.redirected(req); rd != nil {
		req = rd
	}
	resp, err := roundTrip(req)
	if err != nil {
		return nil, err
	}

	if ac.policy(resp) {
		err = ac.handler.HandleChallenge(ctx, resp)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrFailedToHandleChallenge, err)
		}
		Drain(resp.Body)
		return roundTrip(req.Clone(ac.getAuthCtx(ctx)))
	}

	return resp, nil
}

// StandardClient returns a standard http.Client with the AuthClient set as its
// inner Transport.
//
// Consumers should use this when dealing with API's that strictly accept http.Client's.
func (ac *AuthClient) StandardClient() *http.Client {
	return &http.Client{Transport: ac}
}

// RoundTrip calls the AuthClient's underlying Do method. It exists
// so that the AuthClient can satisfy the http.RoundTripper interface.
func (ac *AuthClient) RoundTrip(req *http.Request) (*http.Response, error) {
	return ac.Do(req)
}

// CloneWithNewClient returns a clone of the AuthClient with a new inner
// retryable client. The new AuthClient will share the same headers, auth handler
// and auth policy.
func (ac *AuthClient) CloneWithNewClient(client *rhttp.Client) *AuthClient {
	nc := &AuthClient{
		client:  client,
		policy:  ac.policy,
		handler: ac.handler,
		header:  ac.header,
	}
	nc.init.Do(nc.initClient)
	return nc
}

// Client returns the inner retryable client.
func (ac *AuthClient) Client() *rhttp.Client {
	return ac.client
}

// CacheRedirects tells the client whether or not it should cache any future requests.
// It does NOT affect any existing items in the cache.
func (ac *AuthClient) CacheRedirects(b bool) {
	ac.cacheRedir = b
}

// initClient populates the AuthClient with a set of default values if they aren't already set.
func (ac *AuthClient) initClient() {
	if ac.client == nil {
		ac.client = rhttp.NewClient()
	}
	if ac.policy == nil {
		ac.policy = DefaultAuthPolicy
	}
	if ac.getAuthCtx == nil {
		ac.getAuthCtx = DefaultAuthReqContext
	}
	ac.redirMap = make(map[string]string)
}

func (ac *AuthClient) redirected(req *http.Request) *http.Request {
	if req.Method != http.MethodGet {
		return nil
	}

	ac.redirMu.Lock()
	u, ok := ac.redirMap[req.URL.String()]
	ac.redirMu.Unlock()
	if !ok {
		return nil
	}

	newURL, err := url.Parse(u)
	if err != nil {
		return nil
	}
	r := req.Clone(ac.getAuthCtx(req.Context()))
	r.URL = newURL
	r.Host = newURL.Host
	return r
}

func (ac *AuthClient) shouldCache(req *http.Request, resp *http.Response) bool {
	if !ac.cacheRedir {
		return false
	}

	// We only want to cache GET requests, as those are used to fetch content
	if req.Method != http.MethodGet {
		return false
	}

	if req.URL.String() == resp.Request.URL.String() {
		return false
	}

	// Avoid caching non-200/206 responses.
	switch resp.StatusCode {
	case http.StatusOK, http.StatusPartialContent:
	default:
		return false
	}

	return true
}
