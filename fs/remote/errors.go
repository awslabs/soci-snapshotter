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

package remote

import "errors"

var (
	ErrUnexpectedStatusCode      = errors.New("unexpected status code")
	ErrFailedToRetrieveLayerSize = errors.New("failed to retrieve layer size from remote")
	ErrInvalidHost               = errors.New("invalid host destination")
	ErrFailedToRedirect          = errors.New("failed to redirect")
	ErrUnableToCreateFetcher     = errors.New("unable to create remote fetcher")
	ErrNoRegion                  = errors.New("no regions to fetch")
	ErrCannotParseContentLength  = errors.New("failed to parse Content-Length header")
	ErrCannotParseContentRange   = errors.New("failed to parse Content-Range header")
	ErrCannotParseContentType    = errors.New("failed to parse Content-Type header")
	ErrFailedToRefreshURL        = errors.New("failed to refresh URL")
	ErrRequestFailed             = errors.New("request to registry failed")
)
