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
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

const (
	// mockURL is a fake URL modeling soci-snapshotter fetching content from S3.
	mockURL = "https://s3.us-east-1.amazonaws.com/981ebdad55863b3631dce86a228a3ea230dc87673a06a7d216b1275d4dd707c9/12d7153d7eee2fd595a25e5378384f1ae4b6a1658298a54c5bd3f951ec50b7cb"

	// mockQuery is a fake HTTP query with sensitive information which should be redacted.
	mockQuery = "?username=admin&password=admin"

	// redactedQuery is the expected result of redacting mockQuery.
	// The query values will be sorted by key as a side-effect of encoding the URL query string back into the URL.
	// See https://pkg.go.dev/net/url#Values.Encode
	redactedQuery = "?password=redacted&username=redacted"
)

func TestHandleHTTPErrorRedactsHTTPQueries(t *testing.T) {
	createHTTPResponse := func(path string, query string) *http.Response {
		url, err := url.Parse(path + query)
		if err != nil {
			panic(err)
		}
		return &http.Response{
			Body: &mockBody{},
			Request: &http.Request{
				Method: "GET",
				URL:    url,
			},
		}
	}

	testcases := []struct {
		Name        string
		Description string
		Response    *http.Response
		Err         error
		Attempts    int
		Assert      func(*testing.T, *http.Response, error)
	}{
		{
			Name:        "NilResponseOrError",
			Description: "Handler should gracefully handle a nil response or error",
			Response:    nil,
			Err:         nil,
			Attempts:    10,
			Assert: func(t *testing.T, response *http.Response, err error) {
				if response != nil {
					t.Fatalf("Expected nil response, got '%v'", response)
				}

				const expected = "unknown \"unknown\": giving up request after 10 attempt(s)"
				if strings.Compare(expected, err.Error()) != 0 {
					t.Fatalf("Expected '%s', got '%s'", expected, err.Error())
				}
			},
		},
		{
			Name:        "RedactURLInResponse",
			Description: "Handler should redact HTTP queries in response",
			Response:    createHTTPResponse(mockURL, mockQuery),
			Err:         errors.New("connect: connection refused"),
			Attempts:    10,
			Assert: func(t *testing.T, response *http.Response, err error) {
				if response != nil {
					t.Fatalf("Expected nil response, got '%v'", response)
				}

				const expected = "GET \"" + mockURL + redactedQuery + "\": giving up request after 10 attempt(s): connect: connection refused"
				if strings.Compare(expected, err.Error()) != 0 {
					t.Fatalf("Expected '%s', got '%s'", expected, err.Error())
				}
			},
		},
		{
			Name:        "RedactURLInError",
			Description: "Handler should redact HTTP queries in error",
			Response:    createHTTPResponse(mockURL, ""),
			Err: &url.Error{
				Op:  "GET",
				URL: mockURL + mockQuery,
				Err: errors.New("connect: connection refused"),
			},
			Attempts: 10,
			Assert: func(t *testing.T, response *http.Response, err error) {
				if response != nil {
					t.Fatalf("Expected nil response, got '%v'", response)
				}

				const expected = "GET \"" + mockURL + "\": giving up request after 10 attempt(s): GET \"" + mockURL + redactedQuery + "\": connect: connection refused"
				if strings.Compare(expected, err.Error()) != 0 {
					t.Fatalf("Expected '%s', got '%s'", expected, err.Error())
				}
			},
		},
	}

	for _, testcase := range testcases {
		t.Run(testcase.Name, func(t *testing.T) {
			response, err := HandleHTTPError(testcase.Response, testcase.Err, testcase.Attempts)
			testcase.Assert(t, response, err)
		})
	}
}

func TestHandleHTTPErrorReadsAndClosesResponseBody(t *testing.T) {
	body := &mockBody{}
	response := &http.Response{
		Body: body,
	}
	err := errors.New("connect: connection refused")

	_, _ = HandleHTTPError(response, err, 0)

	if !body.WasRead {
		t.Fatalf("The response body was not read by handler")
	}

	if !body.Closed {
		t.Fatalf("The response body was not closed by handler")
	}
}

type mockBody struct {
	Closed  bool
	WasRead bool
}

func (b *mockBody) Read(_ []byte) (int, error) {
	b.WasRead = true
	return 0, io.EOF
}

func (b *mockBody) Close() error {
	b.Closed = true
	return nil
}
