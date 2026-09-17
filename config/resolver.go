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

/*
   Copyright The containerd Authors.

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

/*
   Copyright 2019 The Go Authors. All rights reserved.
   Use of this source code is governed by a BSD-style
   license that can be found in the NOTICE.md file.
*/

package config

import (
	"fmt"
	"net/http"

	socihttp "github.com/awslabs/soci-snapshotter/internal/http"
	"golang.org/x/net/http/httpguts"
)

// ResolverConfig is config for resolving registries.
type ResolverConfig struct {
	Host map[string]HostConfig `toml:"host"`

	// AuthClientTTLSec is how long (in seconds) cached registry auth clients
	// (and their resolved registry-host configurations) are reused before
	// being discarded and rebuilt. Rebuilding re-resolves credentials and
	// re-authenticates, so the TTL bounds both memory growth of the caches
	// and the lifetime of any credential-derived state. Negative means cache
	// entries never expire. Default: 3600.
	AuthClientTTLSec int64 `toml:"auth_client_ttl_sec"`

	// EnableAuthClientSharing, when true, shares auth clients between image
	// references that target the same registry host with identical
	// credentials, so same-registry images pay a single auth token exchange
	// instead of one per image. When false (the default), every image gets
	// its own auth client and token exchange.
	EnableAuthClientSharing bool `toml:"enable_auth_client_sharing"`

	// CustomHeaders is an allowlist of custom request-header names the
	// snapshotter may attach to its registry fetches. Custom headers arrive as
	// snapshot labels which any image can also set (see containerd
	// FilterInheritedLabels), so this list is the trust boundary. Only these
	// names pass; when unset or empty, no custom header is attached. Reserved
	// or invalid names cause configuration loading to fail. Case-insensitive.
	CustomHeaders []string `toml:"custom_headers"`
}

type HostConfig struct {
	Mirrors []MirrorConfig `toml:"mirrors"`
}

type MirrorConfig struct {

	// Host is the hostname of the host.
	Host string `toml:"host"`

	// Insecure is true means use http scheme instead of https.
	Insecure bool `toml:"insecure"`

	// RequestTimeoutSec is timeout seconds of each request to the registry.
	// RequestTimeoutSec == 0 indicates the default timeout (defaultRequestTimeoutSec).
	// RequestTimeoutSec < 0 indicates no timeout.
	RequestTimeoutSec int64 `toml:"request_timeout_sec"`
}

func parseResolverConfig(cfg *Config) error {
	for _, name := range cfg.ResolverConfig.CustomHeaders {
		if !httpguts.ValidHeaderFieldName(name) {
			return fmt.Errorf("resolver.custom_headers: invalid HTTP header name %q", name)
		}
		if _, reserved := socihttp.ReservedHeaders[http.CanonicalHeaderKey(name)]; reserved {
			return fmt.Errorf("resolver.custom_headers: reserved header %q cannot be customized", name)
		}
	}
	return nil
}
