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
	"time"

	"github.com/awslabs/soci-snapshotter/config"
	"github.com/awslabs/soci-snapshotter/fs/source"
	socihttp "github.com/awslabs/soci-snapshotter/util/http"
	"github.com/containerd/containerd/reference"
	"github.com/containerd/containerd/remotes/docker"
)

type Credential func(string, reference.Spec) (string, string, error)

// RegistryHostsFromConfig creates RegistryHosts (a set of registry configuration) from Config.
func RegistryHostsFromConfig(registryConfig config.ResolverConfig, credsFuncs ...Credential) source.RegistryHosts {
	return func(ref reference.Spec) (hosts []docker.RegistryHost, _ error) {
		host := ref.Hostname()
		for _, h := range append(registryConfig.Host[host].Mirrors, config.MirrorConfig{
			Host: host,
		}) {
			clientConfig := socihttp.NewRetryableClientConfig()
			if h.RequestTimeoutSec < 0 {
				clientConfig.RequestTimeout = 0
			}
			if h.RequestTimeoutSec > 0 {
				clientConfig.RequestTimeout = time.Duration(h.RequestTimeoutSec) * time.Second
			}
			client := socihttp.NewRetryableClient(clientConfig)
			config := docker.RegistryHost{
				Client:       client,
				Host:         h.Host,
				Scheme:       "https",
				Path:         "/v2",
				Capabilities: docker.HostCapabilityPull | docker.HostCapabilityResolve,
				Authorizer: docker.NewDockerAuthorizer(
					docker.WithAuthClient(client),
					docker.WithAuthCreds(multiCredsFuncs(ref, credsFuncs...))),
			}
			if localhost, _ := docker.MatchLocalhost(config.Host); localhost || h.Insecure {
				config.Scheme = "http"
			}
			if config.Host == "docker.io" {
				config.Host = "registry-1.docker.io"
			}
			hosts = append(hosts, config)
		}
		return
	}
}

func multiCredsFuncs(ref reference.Spec, credsFuncs ...Credential) func(string) (string, string, error) {
	return func(host string) (string, string, error) {
		for _, f := range credsFuncs {
			if username, secret, err := f(host, ref); err != nil {
				return "", "", err
			} else if !(username == "" && secret == "") {
				return username, secret, nil
			}
		}
		return "", "", nil
	}
}
