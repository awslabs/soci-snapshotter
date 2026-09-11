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
	"testing"
)

func TestCustomHeadersContext(t *testing.T) {
	ctx := context.Background()
	if headers := CustomHeaders(ctx); headers != nil {
		t.Fatalf("custom headers should not be present: got %v", headers)
	}

	headers := http.Header{"X-Request-Id": {"caller-123"}}
	ctx = WithCustomHeaders(ctx, headers)
	if got := CustomHeaders(ctx).Get("X-Request-Id"); got != "caller-123" {
		t.Fatalf("X-Request-Id = %q, want %q", got, "caller-123")
	}
}
