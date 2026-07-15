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
