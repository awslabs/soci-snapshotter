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

package resolver

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/awslabs/soci-snapshotter/config"
	socihttp "github.com/awslabs/soci-snapshotter/internal/http"
	rhttp "github.com/hashicorp/go-retryablehttp"

	"github.com/containerd/containerd/v2/core/remotes/docker"
	"github.com/containerd/containerd/v2/pkg/reference"
)

// Credential returns a set of credentials for a given image.
type Credential func(imgRefSpec reference.Spec, host string) (string, string, error)

// RegistryHosts returns configurations for registry hosts that provide a given image.
type RegistryHosts func(imgRefSpec reference.Spec) ([]docker.RegistryHost, error)

// RegistryManager contains the configurations that outline how remote
// registry operations should behave. It contains a global retryable client
// that will be used for all registry requests.
type RegistryManager struct {
	// retryClient is the global retryable client
	retryClient *rhttp.Client
	// header is the global HTTP header to be attached to every request
	header http.Header
	// registryConfig is the per-host registry config
	registryConfig config.ResolverConfig
	// creds are the list of credential providers
	creds []Credential
	// registryHostMap is a map of image reference to registry configurations
	registryHostMap *sync.Map
	// authClientMap caches AuthClients keyed by (host, credential
	// fingerprint), so images from the same registry with identical
	// credentials share one client — and therefore one auth token exchange —
	// instead of paying a 401+token round-trip per image. Images with
	// different credentials get distinct clients (see AsRegistryHosts).
	authClientMap *sync.Map
	// authClientTTL bounds how long entries in registryHostMap and
	// authClientMap are reused. Expired entries are lazily evicted on access
	// and periodically swept, so cached clients (and the image references
	// they accumulate) do not pile up indefinitely on a long-lived daemon.
	// <= 0 means entries never expire.
	authClientTTL time.Duration
}

// expiringEntry wraps a cached value with its creation time.
type expiringEntry struct {
	value     any
	createdAt time.Time
}

func (rm *RegistryManager) expired(e *expiringEntry) bool {
	return rm.authClientTTL > 0 && time.Since(e.createdAt) > rm.authClientTTL
}

// loadUnexpired returns the value for key if present and not expired.
// Expired entries are deleted on access.
func (rm *RegistryManager) loadUnexpired(m *sync.Map, key string) (any, bool) {
	v, ok := m.Load(key)
	if !ok {
		return nil, false
	}
	e := v.(*expiringEntry)
	if rm.expired(e) {
		m.Delete(key)
		return nil, false
	}
	return e.value, true
}

// sweep removes all expired entries from the caches. Called periodically so
// entries for hosts/credentials that are never accessed again still get
// reclaimed.
func (rm *RegistryManager) sweep() {
	for _, m := range []*sync.Map{rm.registryHostMap, rm.authClientMap} {
		m.Range(func(k, v any) bool {
			if rm.expired(v.(*expiringEntry)) {
				m.Delete(k)
			}
			return true
		})
	}
}

// NewRegistryManager returns a new RegistryManager
func NewRegistryManager(httpConfig config.RetryableHTTPClientConfig, registryConfig config.ResolverConfig, credsFuncs []Credential) *RegistryManager {
	rm := &RegistryManager{
		retryClient:     newRetryableClientFromConfig(httpConfig),
		header:          globalHeaders(),
		registryConfig:  registryConfig,
		creds:           credsFuncs,
		registryHostMap: &sync.Map{},
		authClientMap:   &sync.Map{},
		authClientTTL:   time.Duration(registryConfig.AuthClientTTLSec) * time.Second,
	}
	if rm.authClientTTL > 0 {
		// Periodic sweep so never-again-accessed entries are also reclaimed.
		go func() {
			ticker := time.NewTicker(rm.authClientTTL)
			defer ticker.Stop()
			for range ticker.C {
				rm.sweep()
			}
		}()
	}
	return rm
}

// authClientKey returns a cache key for sharing AuthClients between images.
// Two images share an AuthClient (and therefore an auth token exchange) only
// when they target the same registry host AND resolve to identical
// credentials. Credentials are resolved once, up front, via the same
// credential providers the AuthClient would use at request time; the secret
// is hashed so it is not retained in the key.
func (rm *RegistryManager) authClientKey(imgRefSpec reference.Spec) string {
	host := imgRefSpec.Hostname()
	username, secret, err := multiCredsFuncs(imgRefSpec, rm.creds...)(host)
	if err != nil {
		// Credential resolution failed; fall back to a per-image key so we
		// never share a client across an unknown credential boundary.
		return "ref\x00" + imgRefSpec.String()
	}
	credHash := sha256.Sum256([]byte(username + "\x00" + secret))
	return fmt.Sprintf("host\x00%s\x00%x", host, credHash)
}

// sharedAuthClient is an AuthClient shared by multiple image references that
// resolved to the same (host, credential) identity. The credential providers
// can be scoped per image reference (e.g. the CRI keychain stores credentials
// keyed by image ref and removes them once a pull completes), so the shared
// client's credential function must not be bound to a single image ref.
// Instead it tries every image ref that joined this client, newest first
// (newer refs are the most likely to still have live credentials in
// ref-scoped keychains).
type sharedAuthClient struct {
	client *socihttp.AuthClient

	mu   sync.Mutex
	refs []reference.Spec
}

// addRef registers an image reference with this shared client, so request-time
// credential resolution can use its (identical) credentials.
func (s *sharedAuthClient) addRef(ref reference.Spec) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range s.refs {
		if r.String() == ref.String() {
			return
		}
	}
	s.refs = append(s.refs, ref)
}

// credsFunc returns a credential function that tries all registered image
// references, newest first, returning the first non-empty credentials.
func (s *sharedAuthClient) credsFunc(creds []Credential) func(string) (string, string, error) {
	return func(host string) (string, string, error) {
		s.mu.Lock()
		refs := make([]reference.Spec, len(s.refs))
		copy(refs, s.refs)
		s.mu.Unlock()
		for i := len(refs) - 1; i >= 0; i-- {
			username, secret, err := multiCredsFuncs(refs[i], creds...)(host)
			if err != nil {
				return "", "", err
			}
			if username != "" || secret != "" {
				return username, secret, nil
			}
		}
		return "", "", nil
	}
}

// AsRegistryHosts returns a RegistryHosts type responsible for returning
// configurations for registries that contain a given image with respect
// to the configurations present in RegistryManager.
func (rm *RegistryManager) AsRegistryHosts() RegistryHosts {
	/*
		By default we create new AuthClient's for every unique image reference
		because credentials can be scoped to specific images/repositories
		(eg: OAuth2, K8s pull secrets through our CRI implementation).

		When EnableAuthClientSharing is set, AuthClients are shared between
		image references that target the same registry host with identical
		credentials (see authClientKey), so same-registry images pay a single
		auth token exchange instead of one per image. Images whose credential
		providers return different credentials get distinct AuthClients and
		never share tokens.
	*/
	return func(imgRefSpec reference.Spec) ([]docker.RegistryHost, error) {
		// Check whether registry host configurations exist for this image ref
		// in the cache.
		if hostConfigurations, ok := rm.loadUnexpired(rm.registryHostMap, imgRefSpec.String()); ok {
			return hostConfigurations.([]docker.RegistryHost), nil
		}

		var registryHosts []docker.RegistryHost

		var authClient *socihttp.AuthClient
		if rm.registryConfig.EnableAuthClientSharing {
			// Reuse an AuthClient if one already exists for this host+credential
			// identity; otherwise create one and cache it.
			acKey := rm.authClientKey(imgRefSpec)
			var shared *sharedAuthClient
			if cached, ok := rm.loadUnexpired(rm.authClientMap, acKey); ok {
				shared = cached.(*sharedAuthClient)
			} else {
				newShared := &sharedAuthClient{}
				newClient, err := newAuthClient(rm.retryClient, rm.header, newShared.credsFunc(rm.creds))
				if err != nil {
					return nil, err
				}
				newShared.client = newClient
				// LoadOrStore: if another goroutine raced us, share its client so
				// both images use the same token cache.
				cached, _ := rm.authClientMap.LoadOrStore(acKey, &expiringEntry{value: newShared, createdAt: time.Now()})
				shared = cached.(*expiringEntry).value.(*sharedAuthClient)
			}
			shared.addRef(imgRefSpec)
			authClient = shared.client
		} else {
			// Per-image auth client (the default behavior, no sharing).
			var err error
			authClient, err = newAuthClient(rm.retryClient, rm.header, multiCredsFuncs(imgRefSpec, rm.creds...))
			if err != nil {
				return nil, err
			}
		}

		host := imgRefSpec.Hostname()
		// If mirrors exist for the host that provides this image, create new
		// `RegistryHost` configurations for them.
		if hostConfig, ok := rm.registryConfig.Host[host]; ok {
			for _, mirror := range hostConfig.Mirrors {
				// Ensure the mirror host is a valid host url.
				url, err := url.Parse(mirror.Host)
				if err != nil {
					return nil, fmt.Errorf("failed to parse mirror host: %q: %w", mirror.Host, err)
				}
				host, err := docker.DefaultHost(url.Host)
				if err != nil {
					return nil, err
				}
				scheme := DefaultScheme(host)
				if mirror.Insecure {
					scheme = "http"
				}
				// Create a copy of the auth and retry client's so we don't overwrite the existing ones.
				authClient := authClient
				retryClient := rm.retryClient

				// If a RequestTimeoutSec is set (non-zero) and it differs from the timeout set in
				// the global retryable client, we will need to create a new one.
				if mirror.RequestTimeoutSec != 0 && mirror.RequestTimeoutSec != int64(retryClient.HTTPClient.Timeout) {
					retryClient = CloneRetryableClient(retryClient)
					if mirror.RequestTimeoutSec < 0 {
						retryClient.HTTPClient.Timeout = 0
					} else {
						retryClient.HTTPClient.Timeout = time.Duration(mirror.RequestTimeoutSec) * time.Second
					}
					// Re-use the same transport so we can use a single
					// global connection pool.
					retryClient.HTTPClient.Transport = rm.retryClient.HTTPClient.Transport
					// Create a clone of the AuthClient with the new retryable client.
					authClient = authClient.CloneWithNewClient(retryClient)
				}
				if url.Path == "" {
					url.Path = "/v2"
				}
				registryHosts = append(registryHosts, docker.RegistryHost{
					Client:       authClient.StandardClient(),
					Host:         host,
					Scheme:       scheme,
					Path:         url.Path,
					Capabilities: docker.HostCapabilityPull | docker.HostCapabilityResolve,
				})
			}
		}

		// Create a `RegistryHost` configuration for this host.
		host, err := docker.DefaultHost(host)
		if err != nil {
			return nil, err
		}
		scheme := DefaultScheme(host)
		registryHosts = append(registryHosts, docker.RegistryHost{
			Client:       authClient.StandardClient(),
			Host:         host,
			Scheme:       scheme,
			Path:         "/v2",
			Capabilities: docker.HostCapabilityPull | docker.HostCapabilityResolve,
		})

		// Cache `RegistryHost` configurations for all hosts that provide this image.
		rm.registryHostMap.Store(imgRefSpec.String(), &expiringEntry{value: registryHosts, createdAt: time.Now()})

		return registryHosts, nil
	}
}

// multiCredsFuncs joins a list of credential functions into a single credential function.
//
// Note: We close over an image reference so that our invdidual credential providers
// can store+index credentials at an image level.
func multiCredsFuncs(imgRefSpec reference.Spec, credsFuncs ...Credential) func(string) (string, string, error) {
	return func(host string) (string, string, error) {
		for _, f := range credsFuncs {
			if username, secret, err := f(imgRefSpec, host); err != nil {
				return "", "", err
			} else if username != "" || secret != "" {
				return username, secret, nil
			}
		}
		return "", "", nil
	}
}

// DefaultScheme returns the default scheme for a registry host.
//
// Copied over from: https://github.com/containerd/containerd/blob/a901236bf00a6d0ef1fe299c9e5ae72a1dd67869/pkg/cri/server/images/image_pull.go#L474
// Original Copyright the containerd Authors. Licensed under the Apache License, Version 2.0 (the "License").
func DefaultScheme(host string) string {
	if docker.IsLocalhost(host) {
		return "http"
	}
	return "https"
}
