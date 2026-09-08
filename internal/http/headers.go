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
