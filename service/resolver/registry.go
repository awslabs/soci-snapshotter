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
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/awslabs/soci-snapshotter/config"
	rhttp "github.com/hashicorp/go-retryablehttp"

	"github.com/containerd/containerd/reference"
	"github.com/containerd/containerd/remotes/docker"
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
}

// NewRegistryManager returns a new RegistryManager
func NewRegistryManager(httpConfig config.RetryableHTTPClientConfig, registryConfig config.ResolverConfig, credsFuncs []Credential) *RegistryManager {
	return &RegistryManager{
		retryClient:     newRetryableClientFromConfig(httpConfig),
		header:          globalHeaders(),
		registryConfig:  registryConfig,
		creds:           credsFuncs,
		registryHostMap: &sync.Map{},
	}
}

// AsRegistryHosts returns a RegistryHosts type responsible for returning
// configurations for registries that contain a given image with respect
// to the configurations present in RegistryManager.
func (rm *RegistryManager) AsRegistryHosts() RegistryHosts {
	/*
		We create new AuthClient's for every unique image reference because credentials can be
		scoped to specific images/repositories (eg: OAuth2). Although, our underlying auth handler
		(docker.Authorizer) fetches tokens at request time, this can potentially become an issue in
		the future if the authorizer re-uses the scoped credentials it gets through one of our credential
		providers (eg: K8s through our CRI implementation). For this reason, our credential providers
		should try to store+index credentials at a more granular level than just the host name
		(ideally by the full image reference).
	*/
	return func(imgRefSpec reference.Spec) ([]docker.RegistryHost, error) {
		// Check whether registry host configurations exist for this image ref
		// in the cache.
		if hostConfigurations, ok := rm.registryHostMap.Load(imgRefSpec.String()); ok {
			return hostConfigurations.([]docker.RegistryHost), nil
		}

		var registryHosts []docker.RegistryHost

		// Create an AuthClient for this image reference.
		authClient, err := newAuthClient(rm.retryClient, rm.header, multiCredsFuncs(imgRefSpec, rm.creds...))
		if err != nil {
			return nil, err
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
		host, err = docker.DefaultHost(host)
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
		rm.registryHostMap.Store(imgRefSpec.String(), registryHosts)

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
			} else if !(username == "" && secret == "") {
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
