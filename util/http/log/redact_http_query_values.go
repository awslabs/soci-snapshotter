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
	"errors"
	"net/url"
)

// RedactHTTPQueryValuesFromError is a log utility to parse an error as a URL error and redact
// HTTP query values to prevent leaking sensitive information like encoded credentials or tokens.
func RedactHTTPQueryValuesFromError(err error) error {
	var urlErr *url.Error

	if err != nil && errors.As(err, &urlErr) {
		url, urlParseErr := url.Parse(urlErr.URL)
		if urlParseErr == nil {
			RedactHTTPQueryValuesFromURL(url)
			urlErr.URL = url.Redacted()
			return urlErr
		}
	}

	return err
}

// RedactHTTPQueryValuesFromURL redacts HTTP query values from a URL.
func RedactHTTPQueryValuesFromURL(url *url.URL) {
	if url != nil {
		if query := url.Query(); len(query) > 0 {
			for k := range query {
				query.Set(k, "redacted")
			}
			url.RawQuery = query.Encode()
		}
	}
}
