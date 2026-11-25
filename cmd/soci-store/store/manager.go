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

// manager.go is adapted from stargz-snapshotter's store/manager.go, backed by
// soci's fs/layer and keyed by ztoc (TOC) digest.

package store

import (
	"context"
	"fmt"
	"sync"

	"github.com/awslabs/soci-snapshotter/config"
	"github.com/awslabs/soci-snapshotter/fs/layer"
	"github.com/awslabs/soci-snapshotter/metadata"
	"github.com/awslabs/soci-snapshotter/service/resolver"
	socistore "github.com/awslabs/soci-snapshotter/soci/store"
	"github.com/awslabs/soci-snapshotter/util/namedmutex"
	"github.com/containerd/containerd/v2/pkg/reference"
	"github.com/containerd/log"
	digest "github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// NewLayerManager creates a LayerManager backed by soci's fs/layer.Resolver.
// The content store must be the same one the refPool fetches SOCI artifacts into.
func NewLayerManager(ctx context.Context, root string, hosts resolver.RegistryHosts, metadataStore metadata.Store, contentStore socistore.Store, cfg *config.Config) (*LayerManager, error) {
	refPool, err := newRefPool(ctx, root, hosts, contentStore)
	if err != nil {
		return nil, err
	}

	r, err := layer.NewResolver(root, cfg.FSConfig, nil, metadataStore, contentStore, layer.OverlayOpaqueAll, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to setup resolver: %w", err)
	}

	return &LayerManager{
		refPool:     refPool,
		hosts:       hosts,
		resolver:    r,
		resolveLock: new(namedmutex.NamedMutex),
		layer:       make(map[string]map[string]layer.Layer),
		refcounter:  make(map[string]map[string]int),
	}, nil
}

// LayerManager manages layers of images and their resource lifetime.
type LayerManager struct {
	refPool  *refPool
	hosts    resolver.RegistryHosts
	resolver *layer.Resolver

	resolveLock *namedmutex.NamedMutex

	mu         sync.Mutex
	layer      map[string]map[string]layer.Layer // keyed by image ref and TOC digest
	refcounter map[string]map[string]int         // keyed by image ref and TOC digest
}

func (m *LayerManager) cacheLayer(refspec reference.Spec, tocDigest digest.Digest, l layer.Layer) (_ layer.Layer, added bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.layer[refspec.String()] == nil {
		m.layer[refspec.String()] = make(map[string]layer.Layer)
	}
	if cl, ok := m.layer[refspec.String()][tocDigest.String()]; ok {
		return cl, false // already exists
	}
	m.layer[refspec.String()][tocDigest.String()] = l
	return l, true
}

func (m *LayerManager) getCachedLayer(refspec reference.Spec, tocDigest digest.Digest) layer.Layer {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.layer[refspec.String()] == nil {
		return nil
	}
	return m.layer[refspec.String()][tocDigest.String()]
}

// getLayerInfo returns the "info" JSON for the layer identified by tocDigest.
func (m *LayerManager) getLayerInfo(ctx context.Context, refspec reference.Spec, tocDigest digest.Digest) (Layer, error) {
	img, err := m.refPool.loadImage(ctx, refspec)
	if err != nil {
		return Layer{}, fmt.Errorf("failed to load image %q: %w", refspec.String(), err)
	}
	entry, ok := img.entries[tocDigest.String()]
	if !ok {
		return Layer{}, fmt.Errorf("no SOCI-indexed layer with TOC digest %q for %q", tocDigest, refspec.String())
	}
	return genLayerInfo(img.manifest, img.config, entry.layerDesc, tocDigest)
}

// getLayer resolves (or returns the cached) layer for tocDigest.
func (m *LayerManager) getLayer(ctx context.Context, refspec reference.Spec, tocDigest digest.Digest) (layer.Layer, error) {
	if l := m.getCachedLayer(refspec, tocDigest); l != nil {
		return l, nil
	}

	key := refspec.String() + "/" + tocDigest.String()
	m.resolveLock.Lock(key)
	defer m.resolveLock.Unlock(key)

	// Re-check after acquiring the lock in case a concurrent call resolved it.
	if l := m.getCachedLayer(refspec, tocDigest); l != nil {
		return l, nil
	}

	img, err := m.refPool.loadImage(ctx, refspec)
	if err != nil {
		return nil, fmt.Errorf("failed to load image %q: %w", refspec.String(), err)
	}
	entry, ok := img.entries[tocDigest.String()]
	if !ok {
		return nil, fmt.Errorf("no SOCI-indexed layer with TOC digest %q for %q", tocDigest, refspec.String())
	}

	hosts, err := m.hosts(refspec)
	if err != nil {
		return nil, err
	}

	// disableVerification=false: keep ztoc verification on.
	l, err := m.resolver.Resolve(ctx, hosts, refspec, entry.layerDesc, entry.sociDesc, nil, false, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve layer %q (toc %q): %w", entry.layerDesc.Digest, tocDigest, err)
	}

	cached, added := m.cacheLayer(refspec, tocDigest, l)
	if !added {
		l.Done() // lost the race; use the cached one and discard this.
	}
	return cached, nil
}

// use increments the reference counter for a layer.
func (m *LayerManager) use(refspec reference.Spec, tocDigest digest.Digest) int {
	m.refPool.use(refspec)

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.refcounter[refspec.String()] == nil {
		m.refcounter[refspec.String()] = make(map[string]int)
	}
	m.refcounter[refspec.String()][tocDigest.String()]++
	return m.refcounter[refspec.String()][tocDigest.String()]
}

// release decrements a layer's reference counter, evicting the layer once it
// reaches zero. It never errors, so the FUSE layer won't crash on GC signals.
func (m *LayerManager) release(ctx context.Context, refspec reference.Spec, tocDigest digest.Digest) (int, error) {
	m.refPool.release(refspec)

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.refcounter[refspec.String()] != nil {
		if _, ok := m.refcounter[refspec.String()][tocDigest.String()]; ok {
			m.refcounter[refspec.String()][tocDigest.String()]--
		}
	}
	i := 0
	if m.refcounter[refspec.String()] != nil {
		i = m.refcounter[refspec.String()][tocDigest.String()]
	}
	if i > 0 {
		return i, nil
	}

	// No more references: release the layer and evict it.
	if m.refcounter[refspec.String()] != nil {
		delete(m.refcounter[refspec.String()], tocDigest.String())
		if len(m.refcounter[refspec.String()]) == 0 {
			delete(m.refcounter, refspec.String())
		}
	}
	if m.layer[refspec.String()] != nil {
		if l, ok := m.layer[refspec.String()][tocDigest.String()]; ok {
			l.Done()
			m.resolver.Evict(l.GetCacheRefKey())
			delete(m.layer[refspec.String()], tocDigest.String())
			if len(m.layer[refspec.String()]) == 0 {
				delete(m.layer, refspec.String())
			}
			log.G(ctx).WithField("refcounter", i).Infof("layer %v/%v is released due to no reference", refspec, tocDigest)
		}
	}
	return 0, nil
}

// Layer represents the layer information. Its JSON form is the one required by
// the "additional layer store" of github.com/containers/storage.
type Layer struct {
	UncompressedSize int64             `json:"diff-size,omitempty"`
	CompressionType  int               `json:"compression,omitempty"`
	TOCDigest        digest.Digest     `json:"toc-digest,omitempty"`
	Flags            map[string]string `json:"flags,omitempty"`
}

// Defined in https://github.com/containers/storage/blob/main/pkg/archive/archive.go
const gzipTypeMagicNum = 2

// genLayerInfo builds the `info` JSON for the layer described by layerDesc.
func genLayerInfo(manifest ocispec.Manifest, config ocispec.Image, layerDesc ocispec.Descriptor, tocDigest digest.Digest) (Layer, error) {
	if len(manifest.Layers) != len(config.RootFS.DiffIDs) {
		return Layer{}, fmt.Errorf(
			"len(manifest.Layers) != len(config.Rootfs): %d != %d",
			len(manifest.Layers), len(config.RootFS.DiffIDs))
	}
	layerIndex := -1
	for i, l := range manifest.Layers {
		if l.Digest == layerDesc.Digest {
			layerIndex = i
		}
	}
	if layerIndex == -1 {
		return Layer{}, fmt.Errorf("layer %q not found in the manifest", layerDesc.Digest.String())
	}

	layerFlags := map[string]string{
		"expected-layer-diffid": config.RootFS.DiffIDs[layerIndex].String(),
	}
	return Layer{
		UncompressedSize: -1, // means unknown
		CompressionType:  gzipTypeMagicNum,
		TOCDigest:        tocDigest,
		Flags:            layerFlags,
	}, nil
}
