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
	"net/http"
	"sync"

	"github.com/awslabs/soci-snapshotter/config"
	socihttp "github.com/awslabs/soci-snapshotter/pkg/http"

	"github.com/containerd/containerd/remotes/docker"
)

// Credential returns a set of credentials for a given host.
type Credential func(string) (string, string, error)

// RegistryManager contains the configurations that outline how remote
// registry operations should behave. It contains a global http.Client
// that will be used for all registry requests.
type RegistryManager struct {
	// client is the global http.Client
	client *http.Client
	// httpConfig is the global HTTP config
	httpConfig config.RetryableHTTPClientConfig
	// registryConfig is the per-host registry config
	registryConfig config.ResolverConfig
	// registryHostMap is a map of host to registry configuration
	registryHostMap *sync.Map
}

// NewRegistryManager returns a new RegistryManager
func NewRegistryManager(httpConfig config.RetryableHTTPClientConfig, registryConfig config.ResolverConfig, credsFuncs []Credential) (*RegistryManager, error) {
	registryManager := &RegistryManager{
		httpConfig:      httpConfig,
		registryConfig:  registryConfig,
		registryHostMap: &sync.Map{},
	}
	// Create a global HTTP client.
	globalClient, err := newGlobalClient(httpConfig, multiCredsFuncs(credsFuncs...))
	if err != nil {
		return nil, err
	}
	registryManager.client = globalClient
	return registryManager, nil
}

// ConfigureRegistries returns a RegistryHosts type responsible for returning
// registry configurations for a specific host.
func (rm *RegistryManager) ConfigureRegistries() docker.RegistryHosts {
	return func(host string) ([]docker.RegistryHost, error) {
		// Check whether registry host configurations exist for this host
		// in the cache.
		if hostConfigurations, ok := rm.registryHostMap.Load(host); ok {
			return hostConfigurations.([]docker.RegistryHost), nil
		}
		registryHosts := []docker.RegistryHost{}
		// If mirrors exist for this host, create new `RegistryHost` configurations
		// for them.
		if hostConfig, ok := rm.registryConfig.Host[host]; ok {
			for _, mirror := range hostConfig.Mirrors {
				client := rm.client
				scheme := "https"
				if localhost, _ := docker.MatchLocalhost(mirror.Host); localhost || mirror.Insecure {
					scheme = "http"
				}
				if mirror.RequestTimeoutSec > 0 {
					httpCfg := rm.httpConfig
					httpCfg.RequestTimeoutMsec = mirror.RequestTimeoutSec * 1000
					if authClient, ok := client.Transport.(*socihttp.AuthClient); ok {
						retryClient := newRetryableClientFromConfig(httpCfg)
						client = authClient.CloneWithNewClient(retryClient).StandardClient()
					}
				}
				registryHosts = append(registryHosts, docker.RegistryHost{
					Client:       client,
					Host:         mirror.Host,
					Scheme:       scheme,
					Path:         "/v2",
					Capabilities: docker.HostCapabilityPull | docker.HostCapabilityResolve,
				})
			}
		}

		if host == "docker.io" {
			host = "registry-1.docker.io"
		}
		// Create a `RegistryHost` configuration for this host.
		registryHosts = append(registryHosts, docker.RegistryHost{
			Client:       rm.client,
			Host:         host,
			Scheme:       "https",
			Path:         "/v2",
			Capabilities: docker.HostCapabilityPull | docker.HostCapabilityResolve,
		})

		// Cache all `RegistryHost` configurations for this host.
		rm.registryHostMap.Store(host, registryHosts)

		return registryHosts, nil
	}
}

// multiCredsFuncs joins a list of credential functions into single credential function.
func multiCredsFuncs(credsFuncs ...Credential) func(string) (string, string, error) {
	return func(host string) (string, string, error) {
		for _, f := range credsFuncs {
			if username, secret, err := f(host); err != nil {
				return "", "", err
			} else if !(username == "" && secret == "") {
				return username, secret, nil
			}
		}
		return "", "", nil
	}
}
