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
	"os"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/remotes"
	"github.com/containerd/containerd/remotes/docker"
	"github.com/containerd/containerd/remotes/docker/config"
)

func (proc *ContainerdProcess) PullImageFromECR(
	ctx context.Context,
	imageRef string,
	platform string,
	awsSecretFile string) (containerd.Image, error) {
	opts := GetRemoteOpts(ctx, platform)

	resolver, err := GetECRResolver(ctx, awsSecretFile)
	if err != nil {
		return nil, err
	}
	opts = append(opts, containerd.WithResolver(resolver))
	image, pullErr := proc.Client.Pull(ctx, imageRef, opts...)
	if pullErr != nil {
		return nil, pullErr
	}
	return image, nil
}

func GetECRResolver(ctx context.Context, awsSecretFile string) (remotes.Resolver, error) {
	username := "AWS"
	secretByteArray, err := os.ReadFile(awsSecretFile)
	secret := string(secretByteArray)
	if err != nil {
		return nil, err
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

	return docker.NewResolver(options), nil
}
