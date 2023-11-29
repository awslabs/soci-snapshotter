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

import "errors"

var (
	ErrMissingAuthHandler       = errors.New("missing auth handler")
	ErrFailedToAuthorizeRequest = errors.New("failed to authorize request")
	ErrFailedToHandleChallenge  = errors.New("failed to handle challenge")
)
