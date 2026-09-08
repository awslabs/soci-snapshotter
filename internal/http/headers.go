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
	"net/http"
)

// These constants name the HTTP headers that the snapshotter or a shared library
// sets on its own requests. A caller must not override them through a custom
// header. ReservedHeaders lists them.
const (
	HeaderRange          = "Range"
	HeaderAcceptEncoding = "Accept-Encoding"
	HeaderAccept         = "Accept"
	HeaderContentType    = "Content-Type"
	HeaderContentLength  = "Content-Length"
	HeaderAuthorization  = "Authorization"
	HeaderUserAgent      = "User-Agent"
	HeaderReferer        = "Referer"
)

// ReservedHeaders holds the reserved header names in canonical form for lookup.
// A reserved name is set by the snapshotter itself, or by a library it shares the
// client with. oras-go and the containerd docker fetcher set Accept and
// Content-Type for content negotiation, so a custom header must not replace them.
// HeadersFromLabels drops a custom header whose name is in this set, and
// AuthClient.Do never lets one override it.
var ReservedHeaders = map[string]struct{}{
	http.CanonicalHeaderKey(HeaderRange):          {},
	http.CanonicalHeaderKey(HeaderAcceptEncoding): {},
	http.CanonicalHeaderKey(HeaderAccept):         {},
	http.CanonicalHeaderKey(HeaderContentType):    {},
	http.CanonicalHeaderKey(HeaderContentLength):  {},
	http.CanonicalHeaderKey(HeaderAuthorization):  {},
	http.CanonicalHeaderKey(HeaderUserAgent):      {},
	http.CanonicalHeaderKey(HeaderReferer):        {},
}

// customHeadersKey is the context key for the custom request headers.
type customHeadersKey struct{}

// WithCustomHeaders returns a context that adds h to every request the AuthClient
// sends under it.
func WithCustomHeaders(ctx context.Context, h http.Header) context.Context {
	if len(h) == 0 {
		return ctx
	}
	return context.WithValue(ctx, customHeadersKey{}, h)
}

// CustomHeaders returns the headers set by WithCustomHeaders, or nil.
func CustomHeaders(ctx context.Context) http.Header {
	h, _ := ctx.Value(customHeadersKey{}).(http.Header)
	return h
}
