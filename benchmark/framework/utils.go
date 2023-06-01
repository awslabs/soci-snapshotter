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

package framework

import (
	"context"
	"io"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/reference"
	"github.com/containerd/containerd/remotes"
	"github.com/containerd/containerd/remotes/docker"
	"github.com/containerd/containerd/remotes/docker/config"
	dockercliconfig "github.com/docker/cli/cli/config"
)

func (proc *ContainerdProcess) PullImageFromRegistry(
	ctx context.Context,
	imageRef string,
	platform string) (containerd.Image, error) {
	opts := GetRemoteOpts(ctx, platform)
	opts = append(opts, containerd.WithResolver(GetResolver(ctx, imageRef)))
	image, pullErr := proc.Client.Pull(ctx, imageRef, opts...)
	if pullErr != nil {
		return nil, pullErr
	}
	return image, nil
}

func GetResolver(ctx context.Context, imageRef string) remotes.Resolver {

	var username string
	var secret string
	refspec, err := reference.Parse(imageRef)
	if err != nil {
		panic("Failed to parse image ref")
	}
	cf := dockercliconfig.LoadDefaultConfigFile(io.Discard)
	if cf.ContainsAuth() {
		if ac, err := cf.GetAuthConfig(refspec.Hostname()); err == nil {
			username = ac.Username
			secret = ac.Password
		}
	}

	hostOptions := config.HostOptions{}
	hostOptions.Credentials = func(host string) (string, string, error) {
		return username, secret, nil
	}
	var PushTracker = docker.NewInMemoryTracker()
	options := docker.ResolverOptions{
		Tracker: PushTracker,
	}
	options.Hosts = config.ConfigureHosts(ctx, hostOptions)

	return docker.NewResolver(options)
}
