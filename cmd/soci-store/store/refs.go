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

// refs.go is adapted from stargz-snapshotter's store/refs.go, using SOCI's index
// for the layer->ztoc mapping and an in-memory ref cache.

package store

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	socifs "github.com/awslabs/soci-snapshotter/fs"
	"github.com/awslabs/soci-snapshotter/service/resolver"
	"github.com/awslabs/soci-snapshotter/soci"
	socistore "github.com/awslabs/soci-snapshotter/soci/store"
	"github.com/awslabs/soci-snapshotter/util/namedmutex"
	"github.com/containerd/containerd/v2/core/images"
	"github.com/containerd/containerd/v2/core/remotes"
	"github.com/containerd/containerd/v2/core/remotes/docker"
	"github.com/containerd/containerd/v2/pkg/reference"
	"github.com/containerd/log"
	"github.com/containerd/platforms"
	digest "github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/registry/remote"
)

// manifestSizeLimit caps the bytes read for a manifest/index (4MiB per the distribution spec).
const manifestSizeLimit = 4 * 1024 * 1024

// layerEntry holds the descriptors needed to resolve a layer through soci's fs/layer.Resolver.
type layerEntry struct {
	layerDesc ocispec.Descriptor // image layer descriptor
	sociDesc  ocispec.Descriptor // ztoc descriptor from the SOCI index
}

// refImage caches the per-image data needed to serve the ALS tree.
type refImage struct {
	manifest ocispec.Manifest
	config   ocispec.Image
	entries  map[string]layerEntry // keyed by ztoc (TOC) digest
}

// refPool caches per-image manifest, config and the TOC-digest -> layer mapping
// derived from the SOCI index.
type refPool struct {
	path         string
	hosts        resolver.RegistryHosts
	contentStore socistore.Store

	fetchLock *namedmutex.NamedMutex

	mu         sync.Mutex
	images     map[string]*refImage
	refcounter map[string]int
}

func newRefPool(_ context.Context, root string, hosts resolver.RegistryHosts, contentStore socistore.Store) (*refPool, error) {
	poolroot := filepath.Join(root, "pool")
	if err := os.MkdirAll(poolroot, 0700); err != nil {
		return nil, err
	}
	return &refPool{
		path:         poolroot,
		hosts:        hosts,
		contentStore: contentStore,
		fetchLock:    new(namedmutex.NamedMutex),
		images:       make(map[string]*refImage),
		refcounter:   make(map[string]int),
	}, nil
}

func (p *refPool) root() string {
	return p.path
}

func (p *refPool) cached(refspec reference.Spec) (*refImage, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	img, ok := p.images[refspec.String()]
	return img, ok
}

// loadImage returns the cached refImage for refspec, building it on a cache miss.
func (p *refPool) loadImage(ctx context.Context, refspec reference.Spec) (*refImage, error) {
	if img, ok := p.cached(refspec); ok {
		return img, nil
	}

	// Serialize fetches for the same reference.
	p.fetchLock.Lock(refspec.String())
	defer p.fetchLock.Unlock(refspec.String())

	if img, ok := p.cached(refspec); ok {
		return img, nil
	}

	img, err := p.fetchImage(ctx, refspec)
	if err != nil {
		return nil, err
	}

	p.mu.Lock()
	p.images[refspec.String()] = img
	p.mu.Unlock()
	return img, nil
}

func (p *refPool) fetchImage(ctx context.Context, refspec reference.Spec) (*refImage, error) {
	manifest, config, err := p.fetchManifestAndConfig(ctx, refspec)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch manifest and config: %w", err)
	}

	// The layer->ztoc mapping lives in a SOCI index referenced by a manifest annotation.
	indexDigestStr := manifest.Annotations[soci.ImageAnnotationSociIndexDigest]
	if indexDigestStr == "" {
		return nil, fmt.Errorf("image %q has no SOCI index annotation %q", refspec.String(), soci.ImageAnnotationSociIndexDigest)
	}
	indexDigest, err := digest.Parse(indexDigestStr)
	if err != nil {
		return nil, fmt.Errorf("invalid SOCI index digest %q: %w", indexDigestStr, err)
	}

	remoteStore, err := p.remoteStore(refspec)
	if err != nil {
		return nil, err
	}

	// Fetch the SOCI index and its ztocs into the content store the resolver reads from.
	index, err := socifs.FetchSociArtifacts(ctx, refspec, ocispec.Descriptor{Digest: indexDigest}, p.contentStore, remoteStore)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch SOCI artifacts for %q: %w", refspec.String(), err)
	}

	// Build image-layer-digest -> ztoc descriptor from the SOCI index blobs.
	layerToSoci := make(map[string]ocispec.Descriptor, len(index.Blobs))
	for _, b := range index.Blobs {
		if b.MediaType != soci.SociLayerMediaType {
			continue
		}
		if layerDigest, ok := b.Annotations[soci.IndexAnnotationImageLayerDigest]; ok {
			layerToSoci[layerDigest] = b
		}
	}

	// Build ztoc(TOC)-digest -> {layerDesc, sociDesc}; the ztoc digest is the ALS lookup key.
	entries := make(map[string]layerEntry)
	for _, layerDesc := range manifest.Layers {
		sociDesc, ok := layerToSoci[layerDesc.Digest.String()]
		if !ok {
			continue
		}
		entries[sociDesc.Digest.String()] = layerEntry{
			layerDesc: layerDesc,
			sociDesc:  sociDesc,
		}
	}

	log.G(ctx).WithField("ref", refspec.String()).WithField("indexedLayers", len(entries)).Debug("loaded SOCI index for image")

	return &refImage{
		manifest: manifest,
		config:   config,
		entries:  entries,
	}, nil
}

// remoteStore builds an ORAS remote repository for refspec using soci's
// auth-aware RegistryManager client, for fetching the SOCI index and ztoc blobs.
func (p *refPool) remoteStore(refspec reference.Spec) (*remote.Repository, error) {
	rhosts, err := p.hosts(refspec)
	if err != nil {
		return nil, err
	}
	if len(rhosts) == 0 {
		return nil, fmt.Errorf("no registry hosts configured for %q", refspec.String())
	}
	repo, err := remote.NewRepository(refspec.Locator)
	if err != nil {
		return nil, fmt.Errorf("cannot create repository %s: %w", refspec.Locator, err)
	}
	repo.Client = rhosts[0].Client
	repo.PlainHTTP, err = docker.MatchLocalhost(refspec.Hostname())
	if err != nil {
		return nil, fmt.Errorf("cannot create repository %s: %w", refspec.Locator, err)
	}
	return repo, nil
}

// fetchManifestAndConfig fetches the platform-specific image manifest and config
// via a containerd docker.Resolver backed by soci's RegistryManager hosts.
func (p *refPool) fetchManifestAndConfig(ctx context.Context, refspec reference.Spec) (ocispec.Manifest, ocispec.Image, error) {
	// Temporary resolver; should only be used for resolving refspec.
	res := docker.NewResolver(docker.ResolverOptions{
		Hosts: func(host string) ([]docker.RegistryHost, error) {
			if host != refspec.Hostname() {
				return nil, fmt.Errorf("unexpected host %q for image ref %q", host, refspec.String())
			}
			return p.hosts(refspec)
		},
	})
	_, img, err := res.Resolve(ctx, refspec.String())
	if err != nil {
		return ocispec.Manifest{}, ocispec.Image{}, err
	}
	fetcher, err := res.Fetcher(ctx, refspec.String())
	if err != nil {
		return ocispec.Manifest{}, ocispec.Image{}, err
	}
	manifest, err := fetchManifestPlatform(ctx, fetcher, img, platforms.DefaultSpec())
	if err != nil {
		return ocispec.Manifest{}, ocispec.Image{}, err
	}
	r, err := fetcher.Fetch(ctx, manifest.Config)
	if err != nil {
		return ocispec.Manifest{}, ocispec.Image{}, err
	}
	defer r.Close()
	var config ocispec.Image
	if err := json.NewDecoder(r).Decode(&config); err != nil {
		return ocispec.Manifest{}, ocispec.Image{}, err
	}
	return manifest, config, nil
}

// fetchManifestPlatform fetches a manifest, recursing into an index to select the platform.
func fetchManifestPlatform(ctx context.Context, fetcher remotes.Fetcher, desc ocispec.Descriptor, platform ocispec.Platform) (ocispec.Manifest, error) {
	rc, err := fetcher.Fetch(ctx, desc)
	if err != nil {
		return ocispec.Manifest{}, err
	}
	defer rc.Close()

	b, err := io.ReadAll(io.LimitReader(rc, manifestSizeLimit))
	if err != nil {
		return ocispec.Manifest{}, err
	}

	switch desc.MediaType {
	case ocispec.MediaTypeImageManifest, images.MediaTypeDockerSchema2Manifest:
		var m ocispec.Manifest
		if err := json.Unmarshal(b, &m); err != nil {
			return ocispec.Manifest{}, err
		}
		return m, nil
	case ocispec.MediaTypeImageIndex, images.MediaTypeDockerSchema2ManifestList:
		var idx ocispec.Index
		if err := json.Unmarshal(b, &idx); err != nil {
			return ocispec.Manifest{}, err
		}
		matcher := platforms.NewMatcher(platform)
		for _, m := range idx.Manifests {
			if m.Platform == nil || matcher.Match(*m.Platform) {
				return fetchManifestPlatform(ctx, fetcher, m, platform)
			}
		}
		return ocispec.Manifest{}, fmt.Errorf("no manifest found for platform %q", platforms.Format(platform))
	default:
		return ocispec.Manifest{}, fmt.Errorf("unsupported manifest media type %q", desc.MediaType)
	}
}

// use increments the per-image reference counter.
func (p *refPool) use(refspec reference.Spec) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.refcounter[refspec.String()]++
	return p.refcounter[refspec.String()]
}

// release decrements the per-image reference counter, dropping the cache at zero.
func (p *refPool) release(refspec reference.Spec) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.refcounter[refspec.String()] > 0 {
		p.refcounter[refspec.String()]--
	}
	n := p.refcounter[refspec.String()]
	if n <= 0 {
		delete(p.refcounter, refspec.String())
		delete(p.images, refspec.String())
	}
	return n
}
