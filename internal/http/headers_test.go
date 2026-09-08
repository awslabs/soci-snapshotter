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
	"sync"
	"testing"
)

type captureRoundTripper struct{ got http.Header }

func (c *captureRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	c.got = req.Header.Clone()
	return &http.Response{StatusCode: http.StatusOK}, nil
}

// echoRoundTripper returns the x-request-id it saw in a response header. It is
// safe to share across goroutines because it keeps no state.
type echoRoundTripper struct{}

func (echoRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	h := make(http.Header)
	h.Set("x-seen-id", req.Header.Get("x-request-id"))
	return &http.Response{StatusCode: http.StatusOK, Header: h}, nil
}

// Guarantee: headers placed on the request context by WithCustomHeaders are attached
// to the outgoing request by AuthClient.Do, alongside the global headers — proving a
// shared, daemon-global AuthClient can still carry per-request (per-caller) values
// like a request id.
func TestCustomHeadersAttachedPerRequest(t *testing.T) {
	global := http.Header{}
	global.Set("User-Agent", "soci-test")
	rt := &captureRoundTripper{}
	ac := SimpleMockAuthClient(rt, global)

	custom := http.Header{}
	custom.Set("x-request-id", "caller-123")
	custom.Set("x-custom-trace", "trace-1")
	ctx := WithCustomHeaders(context.Background(), custom)

	req, _ := http.NewRequestWithContext(ctx, "GET", "http://example/blob", nil)
	if _, err := ac.Do(req); err != nil {
		t.Fatal(err)
	}
	if got := rt.got.Get("x-request-id"); got != "caller-123" {
		t.Fatalf("x-request-id not attached: got %q", got)
	}
	if got := rt.got.Get("x-custom-trace"); got != "trace-1" {
		t.Fatalf("x-custom-trace not attached: got %q", got)
	}
	if got := rt.got.Get("User-Agent"); got != "soci-test" {
		t.Fatalf("global header lost: got %q", got)
	}
}

// With no custom headers, Do must behave exactly as before (nil-safe).
func TestCustomHeadersAbsent(t *testing.T) {
	rt := &captureRoundTripper{}
	ac := SimpleMockAuthClient(rt, http.Header{})
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "http://example/blob", nil)
	if _, err := ac.Do(req); err != nil {
		t.Fatal(err)
	}
	if got := rt.got.Get("x-request-id"); got != "" {
		t.Fatalf("unexpected x-request-id: %q", got)
	}
}

// Do must drop a reserved header even when it is put on the context directly,
// bypassing the label parser. This is the defense-in-depth guarantee at the sink.
func TestCustomHeadersReservedDroppedAtSink(t *testing.T) {
	rt := &captureRoundTripper{}
	ac := SimpleMockAuthClient(rt, http.Header{})

	custom := http.Header{}
	custom.Set("x-request-id", "caller-123")
	custom.Set(HeaderRange, "bytes=999-999") // reserved
	custom.Set(HeaderAccept, "text/plain")   // reserved (set by oras-go)
	ctx := WithCustomHeaders(context.Background(), custom)

	req, _ := http.NewRequestWithContext(ctx, "GET", "http://example/blob", nil)
	if _, err := ac.Do(req); err != nil {
		t.Fatal(err)
	}
	if got := rt.got.Get("x-request-id"); got != "caller-123" {
		t.Fatalf("non-reserved header lost: got %q", got)
	}
	if got := rt.got.Get(HeaderRange); got != "" {
		t.Fatalf("reserved Range must be dropped at the sink: got %q", got)
	}
	if got := rt.got.Get(HeaderAccept); got != "" {
		t.Fatalf("reserved Accept must be dropped at the sink: got %q", got)
	}
}

// Do must keep every value of a multi-value custom header, not just the last.
func TestCustomHeadersMultiValue(t *testing.T) {
	rt := &captureRoundTripper{}
	ac := SimpleMockAuthClient(rt, http.Header{})

	custom := http.Header{}
	custom.Add("X-Trace", "a")
	custom.Add("X-Trace", "b")
	ctx := WithCustomHeaders(context.Background(), custom)

	req, _ := http.NewRequestWithContext(ctx, "GET", "http://example/blob", nil)
	if _, err := ac.Do(req); err != nil {
		t.Fatal(err)
	}
	if got := rt.got.Values("X-Trace"); len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("multi-value header not preserved: got %v", got)
	}
}

// One shared, daemon-global AuthClient must attach each caller's own request id,
// with no cross-wiring between concurrent requests. Run under -race.
func TestCustomHeadersConcurrentIsolation(t *testing.T) {
	ac := SimpleMockAuthClient(echoRoundTripper{}, http.Header{})

	const n = 50
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id := fmt.Sprintf("id-%d", i)
			custom := http.Header{}
			custom.Set("x-request-id", id)
			ctx := WithCustomHeaders(context.Background(), custom)
			req, _ := http.NewRequestWithContext(ctx, "GET", "http://example/blob", nil)
			resp, err := ac.Do(req)
			if err != nil {
				errs <- fmt.Errorf("goroutine %d: %w", i, err)
				return
			}
			if got := resp.Header.Get("x-seen-id"); got != id {
				errs <- fmt.Errorf("goroutine %d saw request id %q, want %q", i, got, id)
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
}
