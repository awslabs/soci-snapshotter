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

package crialpha

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/awslabs/soci-snapshotter/service/resolver"
	"github.com/containerd/containerd/reference"
	distribution "github.com/containerd/containerd/reference/docker"
	runtime_alpha "github.com/containerd/containerd/third_party/k8s.io/cri-api/pkg/apis/runtime/v1alpha2"
	"github.com/containerd/log"
)

// NewCRIAlphaKeychain provides creds passed through CRI PullImage API.
// This also returns a CRI image service server that works as a proxy backed by the specified CRI service.
// This server reads all PullImageRequest and uses PullImageRequest.AuthConfig for authenticating snapshots.
func NewCRIAlphaKeychain(ctx context.Context, connectCRI func() (runtime_alpha.ImageServiceClient, error)) (resolver.Credential, runtime_alpha.ImageServiceServer) {
	server := &instrumentedAlphaService{config: make(map[string]*runtime_alpha.AuthConfig)}
	go func() {
		log.G(ctx).Debugf("Waiting for CRI service to start...")
		// Attempt to establish a gRPC connection with the CRI backend.
		for i := 0; i < 100; i++ {
			client, err := connectCRI()
			if err == nil {
				server.criMu.Lock()
				server.cri = client
				server.criMu.Unlock()
				log.G(ctx).Info("connected to backend CRI service")
				return
			}
			log.G(ctx).WithError(err).Warnf("failed to connect to CRI")
			time.Sleep(10 * time.Second)
		}
		log.G(ctx).Errorf("no connection is available to CRI")
	}()
	return server.credentials, server
}

type instrumentedAlphaService struct {
	runtime_alpha.UnimplementedImageServiceServer

	cri   runtime_alpha.ImageServiceClient
	criMu sync.Mutex

	config   map[string]*runtime_alpha.AuthConfig
	configMu sync.Mutex
}

func (in *instrumentedAlphaService) credentials(imgRefSpec reference.Spec, host string) (string, string, error) {
	if host == "docker.io" || host == "registry-1.docker.io" {
		// Creds of "docker.io" is stored keyed by "https://index.docker.io/v1/".
		host = "index.docker.io"
	}
	in.configMu.Lock()
	defer in.configMu.Unlock()
	if cfg, ok := in.config[imgRefSpec.String()]; ok {
		return resolver.ParseAlphaAuth(cfg, host)
	}
	return "", "", nil
}

func (in *instrumentedAlphaService) getCRI() (c runtime_alpha.ImageServiceClient) {
	in.criMu.Lock()
	c = in.cri
	in.criMu.Unlock()
	return
}

func (in *instrumentedAlphaService) ListImages(ctx context.Context, r *runtime_alpha.ListImagesRequest) (res *runtime_alpha.ListImagesResponse, err error) {
	cri := in.getCRI()
	if cri == nil {
		return nil, errors.New("server is not initialized yet")
	}
	return cri.ListImages(ctx, r)
}

func (in *instrumentedAlphaService) ImageStatus(ctx context.Context, r *runtime_alpha.ImageStatusRequest) (res *runtime_alpha.ImageStatusResponse, err error) {
	cri := in.getCRI()
	if cri == nil {
		return nil, errors.New("server is not initialized yet")
	}
	return cri.ImageStatus(ctx, r)
}

func (in *instrumentedAlphaService) PullImage(ctx context.Context, r *runtime_alpha.PullImageRequest) (res *runtime_alpha.PullImageResponse, err error) {
	cri := in.getCRI()
	if cri == nil {
		return nil, errors.New("server is not initialized yet")
	}
	imgRefSpec, err := parseReference(r.GetImage().GetImage())
	if err != nil {
		return nil, err
	}
	in.configMu.Lock()
	in.config[imgRefSpec.String()] = r.GetAuth()
	in.configMu.Unlock()
	return cri.PullImage(ctx, r)
}

func (in *instrumentedAlphaService) RemoveImage(ctx context.Context, r *runtime_alpha.RemoveImageRequest) (_ *runtime_alpha.RemoveImageResponse, err error) {
	cri := in.getCRI()
	if cri == nil {
		return nil, errors.New("server is not initialized yet")
	}
	imgRefSpec, err := parseReference(r.GetImage().GetImage())
	if err != nil {
		return nil, err
	}
	in.configMu.Lock()
	delete(in.config, imgRefSpec.String())
	in.configMu.Unlock()
	return cri.RemoveImage(ctx, r)
}

func (in *instrumentedAlphaService) ImageFsInfo(ctx context.Context, r *runtime_alpha.ImageFsInfoRequest) (res *runtime_alpha.ImageFsInfoResponse, err error) {
	cri := in.getCRI()
	if cri == nil {
		return nil, errors.New("server is not initialized yet")
	}
	return cri.ImageFsInfo(ctx, r)
}

func parseReference(ref string) (reference.Spec, error) {
	namedRef, err := distribution.ParseDockerRef(ref)
	if err != nil {
		return reference.Spec{}, fmt.Errorf("failed to parse image reference %q: %w", ref, err)
	}
	return reference.Parse(namedRef.String())
}
