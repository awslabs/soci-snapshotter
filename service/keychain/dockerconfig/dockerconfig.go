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

package dockerconfig

import (
	"context"

	"github.com/awslabs/soci-snapshotter/service/resolver"
	"github.com/containerd/containerd/reference"
	"github.com/docker/cli/cli/config"
)

func DockerCreds(host string) (string, string, error) {
	cf, err := config.Load("")
	if err != nil {
		return "", "", nil
	}

	if host == "docker.io" || host == "registry-1.docker.io" {
		// Creds of docker.io is stored keyed by "https://index.docker.io/v1/".
		host = "https://index.docker.io/v1/"
	}
	ac, err := cf.GetAuthConfig(host)
	if err != nil {
		return "", "", err
	}
	if ac.IdentityToken != "" {
		return "", ac.IdentityToken, nil
	}
	return ac.Username, ac.Password, nil
}

func NewDockerConfigKeychain(ctx context.Context) resolver.Credential {
	// We do not index by image reference because the docker config only
	// supports indexing credentials by root URL/hostname.
	// eg: host.io and not host.io/namespace
	return func(_ reference.Spec, host string) (string, string, error) {
		return DockerCreds(host)
	}
}
