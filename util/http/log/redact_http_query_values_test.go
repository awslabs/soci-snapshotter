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

package log

import (
	"bytes"
	"errors"
	"net/url"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
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

func TestRedactHTTPQueryValuesFromError(t *testing.T) {
	testcases := []struct {
		Name        string
		Description string
		Err         error
		Assert      func(*testing.T, error)
	}{
		{
			Name:        "NilError",
			Description: "Utility should handle nil error gracefully",
			Err:         nil,
			Assert: func(t *testing.T, actual error) {
				if actual != nil {
					t.Fatalf("Expected nil error, got '%v'", actual)
				}
			},
		},
		{
			Name:        "NonURLError",
			Description: "Utility should not modify an error if error is not a URL error",
			Err:         errors.New("this error is not a URL error"),
			Assert: func(t *testing.T, actual error) {
				const expected = "this error is not a URL error"
				if strings.Compare(expected, actual.Error()) != 0 {
					t.Fatalf("Expected '%s', got '%v'", expected, actual)
				}
			},
		},
		{
			Name:        "ErrorWithNoHTTPQuery",
			Description: "Utility should not modify an error if no HTTP queries are present.",
			Err: &url.Error{
				Op:  "GET",
				URL: mockURL,
				Err: errors.New("connect: connection refused"),
			},
			Assert: func(t *testing.T, actual error) {
				const expected = "GET \"" + mockURL + "\": connect: connection refused"
				if strings.Compare(expected, actual.Error()) != 0 {
					t.Fatalf("Expected '%s', got '%v'", expected, actual)
				}
			},
		},
		{
			Name:        "ErrorWithHTTPQuery",
			Description: "Utility should redact HTTP query values in errors to prevent logging sensitive information.",
			Err: &url.Error{
				Op:  "GET",
				URL: mockURL + mockQuery,
				Err: errors.New("connect: connection refused"),
			},
			Assert: func(t *testing.T, actual error) {
				const expected = "GET \"" + mockURL + redactedQuery + "\": connect: connection refused"
				if strings.Compare(expected, actual.Error()) != 0 {
					t.Fatalf("Expected '%s', got '%v'", expected, actual)
				}
			},
		},
	}

	for _, testcase := range testcases {
		t.Run(testcase.Name, func(t *testing.T) {
			actual := RedactHTTPQueryValuesFromError(testcase.Err)
			testcase.Assert(t, actual)
		})
	}
}

func TestRedactHTTPQueryValuesFromURL(t *testing.T) {
	testcases := []struct {
		Name        string
		Description string
		URL         *url.URL
		Assert      func(*testing.T, *url.URL)
	}{
		{
			Name:        "NilURL",
			Description: "Utility should handle nil URL gracefully",
			URL:         nil,
			Assert: func(t *testing.T, url *url.URL) {
				if url != nil {
					t.Fatalf("Expected <nil> got '%v'", url)
				}
			},
		},
		{
			Name:        "URLWithNoQueries",
			Description: "Utility should not modify a URL with no queries",
			URL:         &url.URL{},
			Assert: func(t *testing.T, url *url.URL) {
				if len(url.RawQuery) != 0 {
					t.Fatalf("Expected '' got '%s'", url.RawQuery)
				}
			},
		},
		{
			Name:        "URLWithQueries",
			Description: "Utility should not modify a URL with no queries",
			URL:         &url.URL{RawQuery: "key1=value1&key2=value2"},
			Assert: func(t *testing.T, url *url.URL) {
				const expected = "key1=redacted&key2=redacted"
				if strings.Compare(expected, url.RawQuery) != 0 {
					t.Fatalf("Expected '%s', got '%s'", expected, url.RawQuery)
				}
			},
		},
	}

	for _, testcase := range testcases {
		t.Run(testcase.Name, func(t *testing.T) {
			RedactHTTPQueryValuesFromURL(testcase.URL)
			testcase.Assert(t, testcase.URL)
		})
	}
}

func BenchmarkRedactHTTPQueryValuesOverhead(b *testing.B) {
	benchmarks := []struct {
		Name        string
		Description string
		Err         error
		Log         func(*logrus.Entry, error)
	}{
		{
			Name:        "BaselineLogging",
			Description: "Log a message to memory without redaction to measure baseline.",
			Err: &url.Error{
				Op:  "GET",
				URL: mockURL + mockQuery,
				Err: errors.New("connect: connection refused"),
			},
			Log: func(logger *logrus.Entry, err error) {
				logger.WithError(err).Info("Error on HTTP Get")
			},
		},
		{
			Name:        "WithoutReplacement",
			Description: "Log a message with no HTTP query values to memory with redaction utility to measure the flat overhead.",
			Err: &url.Error{
				Op:  "GET",
				URL: mockURL,
				Err: errors.New("connect: connection refused"),
			},
			Log: func(logger *logrus.Entry, err error) {
				logger.WithError(RedactHTTPQueryValuesFromError(err)).Info("Error on HTTP Get")
			},
		},
		{
			Name:        "WithErrorReplacement",
			Description: "Log a message with HTTP query values to memory with redaction utility to measure replacement overhead.",
			Err: &url.Error{
				Op:  "GET",
				URL: mockURL + mockQuery,
				Err: errors.New("connect: connection refused"),
			},
			Log: func(logger *logrus.Entry, err error) {
				logger.WithError(RedactHTTPQueryValuesFromError(err)).Info("Error on HTTP Get")
			},
		},
	}

	setupUUT := func() *logrus.Entry {
		entry := &logrus.Entry{
			Logger: logrus.New(),
		}

		entry.Logger.Out = bytes.NewBuffer([]byte{})

		return entry
	}

	for _, benchmark := range benchmarks {
		b.Run(benchmark.Name, func(b *testing.B) {
			uut := setupUUT()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				benchmark.Log(uut, benchmark.Err)
			}
		})
	}
}
