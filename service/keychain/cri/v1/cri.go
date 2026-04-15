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

package cri

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/awslabs/soci-snapshotter/service/resolver"
	"github.com/containerd/containerd/reference"
	"github.com/containerd/log"
	distribution "github.com/distribution/reference"
	runtime "k8s.io/cri-api/pkg/apis/runtime/v1"
)

const (
	maxPullRecordsPerRef = 5
	maxRefsPerHost       = 50
)

// pullRecord tracks a set of credentials from a PullImage call.
type pullRecord struct {
	auth *runtime.AuthConfig
	time time.Time
}

// NewCRIKeychain provides creds passed through CRI PullImage API.
// Returns:
//   - credentials function for use in multiCredsFuncs
//   - HostRefs function returning all known image refs for a registry host (latest first)
//   - RemoveLatestAuth function to remove the most recent credential for a host+ref
//   - CRI ImageServiceServer
func NewCRIKeychain(ctx context.Context, connectCRI func() (runtime.ImageServiceClient, error)) (resolver.Credential, func(string) []string, func(string, string), runtime.ImageServiceServer) {
	server := &instrumentedService{
		hostCreds: make(map[string]map[string][]*pullRecord),
	}
	go func() {
		log.G(ctx).Debugf("Waiting for CRI service to start...")
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
	return server.credentials, server.HostRefs, server.RemoveLatestAuth, server
}

type instrumentedService struct {
	runtime.UnimplementedImageServiceServer

	cri   runtime.ImageServiceClient
	criMu sync.Mutex

	// hostCreds is the unified credential pool: host → ref → []*pullRecord
	hostCreds map[string]map[string][]*pullRecord
	configMu  sync.Mutex
}

// credentials returns the latest stored credentials for the given image reference.
func (in *instrumentedService) credentials(imgRefSpec reference.Spec, host string) (string, string, error) {
	host = normalizeHost(host)
	in.configMu.Lock()
	defer in.configMu.Unlock()
	refMap, ok := in.hostCreds[host]
	if !ok {
		return "", "", nil
	}
	records, ok := refMap[imgRefSpec.String()]
	if !ok || len(records) == 0 {
		return "", "", nil
	}
	return resolver.ParseAuth(records[len(records)-1].auth, host)
}

// HostRefs returns all known image references for a registry host,
// ordered by most recent PullImage time (latest first).
func (in *instrumentedService) HostRefs(host string) []string {
	host = normalizeHost(host)
	in.configMu.Lock()
	defer in.configMu.Unlock()
	refMap, ok := in.hostCreds[host]
	if !ok {
		return nil
	}
	type refTime struct {
		ref    string
		latest time.Time
	}
	var refs []refTime
	for ref, records := range refMap {
		if len(records) > 0 {
			refs = append(refs, refTime{ref: ref, latest: records[len(records)-1].time})
		}
	}
	sort.SliceStable(refs, func(i, j int) bool {
		return refs[i].latest.After(refs[j].latest)
	})
	result := make([]string, len(refs))
	for i, r := range refs {
		result[i] = r.ref
	}
	return result
}

// RemoveLatestAuth removes the most recent credential for a given host and
// image reference from the pool. Called when a credential is confirmed invalid
// (4xx auth error) for a layer that the ref is known to reference.
func (in *instrumentedService) RemoveLatestAuth(host, ref string) {
	host = normalizeHost(host)
	in.configMu.Lock()
	defer in.configMu.Unlock()
	refMap, ok := in.hostCreds[host]
	if !ok {
		return
	}
	records, ok := refMap[ref]
	if !ok || len(records) == 0 {
		return
	}
	records = records[:len(records)-1]
	if len(records) == 0 {
		delete(refMap, ref)
		if len(refMap) == 0 {
			delete(in.hostCreds, host)
		}
	} else {
		refMap[ref] = records
	}
}

func (in *instrumentedService) getCRI() (c runtime.ImageServiceClient) {
	in.criMu.Lock()
	c = in.cri
	in.criMu.Unlock()
	return
}

func (in *instrumentedService) ListImages(ctx context.Context, r *runtime.ListImagesRequest) (res *runtime.ListImagesResponse, err error) {
	cri := in.getCRI()
	if cri == nil {
		return nil, errors.New("server is not initialized yet")
	}
	return cri.ListImages(ctx, r)
}

func (in *instrumentedService) ImageStatus(ctx context.Context, r *runtime.ImageStatusRequest) (res *runtime.ImageStatusResponse, err error) {
	cri := in.getCRI()
	if cri == nil {
		return nil, errors.New("server is not initialized yet")
	}
	return cri.ImageStatus(ctx, r)
}

func (in *instrumentedService) PullImage(ctx context.Context, r *runtime.PullImageRequest) (res *runtime.PullImageResponse, err error) {
	cri := in.getCRI()
	if cri == nil {
		return nil, errors.New("server is not initialized yet")
	}
	imgRefSpec, err := parseReference(r.GetImage().GetImage())
	if err != nil {
		return nil, err
	}
	// Store credentials before the pull so they are available during the pull
	// (containerd calls soci's Prepare/Check during the pull, which needs creds).
	in.configMu.Lock()
	host := normalizeHost(imgRefSpec.Hostname())
	if in.hostCreds[host] == nil {
		in.hostCreds[host] = make(map[string][]*pullRecord)
	}
	ref := imgRefSpec.String()
	in.hostCreds[host][ref] = append(in.hostCreds[host][ref], &pullRecord{
		auth: r.GetAuth(),
		time: time.Now(),
	})
	if len(in.hostCreds[host][ref]) > maxPullRecordsPerRef {
		in.hostCreds[host][ref] = in.hostCreds[host][ref][len(in.hostCreds[host][ref])-maxPullRecordsPerRef:]
	}
	if len(in.hostCreds[host]) > maxRefsPerHost {
		var oldestRef string
		var oldestTime time.Time
		for r, records := range in.hostCreds[host] {
			if len(records) > 0 {
				t := records[len(records)-1].time
				if oldestRef == "" || t.Before(oldestTime) {
					oldestRef = r
					oldestTime = t
				}
			}
		}
		if oldestRef != "" {
			delete(in.hostCreds[host], oldestRef)
		}
	}
	in.configMu.Unlock()
	return cri.PullImage(ctx, r)
}

func (in *instrumentedService) RemoveImage(ctx context.Context, r *runtime.RemoveImageRequest) (_ *runtime.RemoveImageResponse, err error) {
	cri := in.getCRI()
	if cri == nil {
		return nil, errors.New("server is not initialized yet")
	}
	imgRefSpec, err := parseReference(r.GetImage().GetImage())
	if err != nil {
		return nil, err
	}
	in.configMu.Lock()
	host := normalizeHost(imgRefSpec.Hostname())
	if in.hostCreds[host] != nil {
		delete(in.hostCreds[host], imgRefSpec.String())
		if len(in.hostCreds[host]) == 0 {
			delete(in.hostCreds, host)
		}
	}
	in.configMu.Unlock()
	return cri.RemoveImage(ctx, r)
}

func (in *instrumentedService) ImageFsInfo(ctx context.Context, r *runtime.ImageFsInfoRequest) (res *runtime.ImageFsInfoResponse, err error) {
	cri := in.getCRI()
	if cri == nil {
		return nil, errors.New("server is not initialized yet")
	}
	return cri.ImageFsInfo(ctx, r)
}

func normalizeHost(host string) string {
	if host == "docker.io" || host == "registry-1.docker.io" {
		return "index.docker.io"
	}
	return host
}

func parseReference(ref string) (reference.Spec, error) {
	namedRef, err := distribution.ParseDockerRef(ref)
	if err != nil {
		return reference.Spec{}, fmt.Errorf("failed to parse image reference %q: %w", ref, err)
	}
	return reference.Parse(namedRef.String())
}
