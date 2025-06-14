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

package fs

import (
	"context"
	"fmt"
	"time"

	"github.com/containerd/containerd/archive"
	"github.com/containerd/containerd/mount"
	"github.com/containerd/log"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"golang.org/x/sync/errgroup"
)

type parallelLayerUnpacker struct {
	*layerUnpacker
	controller            LayerUnpackResourceController
	discardUnpackedLayers bool
}

//nolint:revive
func NewParallelLayerUnpacker(fetcher Fetcher, archive Archive, controller LayerUnpackResourceController, discardUnpackedLayers bool) *parallelLayerUnpacker {
	return &parallelLayerUnpacker{
		layerUnpacker: &layerUnpacker{
			fetcher: fetcher,
			archive: archive,
		},
		controller:            controller,
		discardUnpackedLayers: discardUnpackedLayers,
	}
}

func (lu *parallelLayerUnpacker) Unpack(ctx context.Context, desc ocispec.Descriptor, mountpoint string, mounts []mount.Mount) error {
	rc, local, err := lu.fetcher.Fetch(ctx, desc)
	if err != nil {
		return fmt.Errorf("cannot fetch layer: %w", err)
	}
	defer rc.Close()

	release, err := lu.controller.AcquireUnpackLease(ctx)
	if err != nil {
		return err
	}
	defer release()

	var errGroup errgroup.Group
	if !local && !lu.discardUnpackedLayers {
		ingestReader, err := lu.controller.GetUnpackIngestReader()
		if err != nil {
			return fmt.Errorf("cannot get layer ingest: %w", err)
		}
		defer ingestReader.Close()

		errGroup.Go(func() error {
			// containerd Push should handle IsUnavailable error
			err := lu.fetcher.Store(ctx, desc, ingestReader)
			if err != nil {
				return fmt.Errorf("cannot store layer at %s: %w", mountpoint, err)
			}
			log.G(ctx).WithField("digest", desc.Digest).WithField("mountpoint", mountpoint).Debug("Layer stored")
			return nil
		})
	}

	startTime := time.Now()
	opts := []archive.ApplyOpt{
		archive.WithConvertWhiteout(archive.OverlayConvertWhiteout),
	}

	if len(mounts) > 0 {
		if parents := getLayerParents(mounts[0].Options); len(parents) > 0 {
			opts = append(opts, archive.WithParents(parents))
		}
	}

	// Fail-fast if unpack destination does not exist or has any pre-existing content.
	// Pre-existing content could be a possible poisoning attack attempt.
	if err := lu.controller.VerifyUnpackDestinationIsReady(); err != nil {
		return fmt.Errorf("cannot unpack layer %s: %w", desc.Digest, err)
	}

	_, err = lu.archive.Apply(ctx, mountpoint, rc, opts...)
	if err != nil {
		return fmt.Errorf("cannot apply layer: %w", err)
	}

	log.G(ctx).WithFields(log.Fields{"digest": desc.Digest.String(), "size": desc.Size,
		"mountpoint": mountpoint, "latency_ms": time.Since(startTime).Milliseconds()}).Debug("Layer successfully unpacked")
	return errGroup.Wait()
}
