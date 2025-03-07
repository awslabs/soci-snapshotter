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
	"io"
	"strings"

	"github.com/containerd/containerd/archive"
	"github.com/containerd/containerd/archive/compression"
	"github.com/containerd/containerd/mount"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type Unpacker interface {
	// Unpack takes care of getting the layer specified by descriptor `desc`,
	// decompressing it, putting it in the directory with the path `mountpoint`
	// and applying the difference to the parent layers if there is any.
	// After that the layer can be mounted as non-remote snapshot.
	Unpack(ctx context.Context, desc ocispec.Descriptor, mountpoint string, mounts []mount.Mount) error
}

type Archive interface {
	// Apply decompresses the compressed stream represented by reader `r` and
	// applies it to the directory `root`.
	Apply(ctx context.Context, root string, r io.Reader, opts ...archive.ApplyOpt) (int64, error)
}

type layerArchive struct {
}

func NewLayerArchive() Archive {
	return &layerArchive{}
}

func (la *layerArchive) Apply(ctx context.Context, root string, r io.Reader, opts ...archive.ApplyOpt) (int64, error) {
	// we use containerd implementation here
	// decompress first and then apply
	decompressReader, err := compression.DecompressStream(r)
	if err != nil {
		return 0, fmt.Errorf("cannot decompress the stream: %w", err)
	}
	defer decompressReader.Close()
	return archive.Apply(ctx, root, decompressReader, opts...)
}

type layerUnpacker struct {
	fetcher Fetcher
	archive Archive
}

func NewLayerUnpacker(fetcher Fetcher, archive Archive) Unpacker {
	return &layerUnpacker{
		fetcher: fetcher,
		archive: archive,
	}
}

func (lu *layerUnpacker) Unpack(ctx context.Context, desc ocispec.Descriptor, mountpoint string, mounts []mount.Mount) error {
	rc, local, err := lu.fetcher.Fetch(ctx, desc)
	if err != nil {
		return fmt.Errorf("cannot fetch layer: %w", err)
	}

	if !local {
		err := lu.fetcher.Store(ctx, desc, rc)
		rc.Close()
		if err != nil {
			return fmt.Errorf("cannot store layer: %w", err)
		}
		rc, _, err = lu.fetcher.Fetch(ctx, desc)
		if err != nil {
			return fmt.Errorf("cannot fetch layer: %w", err)
		}
	}
	defer rc.Close()

	opts := []archive.ApplyOpt{
		archive.WithConvertWhiteout(archive.OverlayConvertWhiteout),
	}
	if len(mounts) > 0 {
		if parents := getLayerParents(mounts[0].Options); len(parents) > 0 {
			opts = append(opts, archive.WithParents(parents))
		}
	}
	_, err = lu.archive.Apply(ctx, mountpoint, rc, opts...)
	if err != nil {
		return fmt.Errorf("cannot apply layer: %w", err)
	}

	return nil
}

func getLayerParents(options []string) (lower []string) {
	const lowerdirPrefix = "lowerdir="

	for _, o := range options {
		if strings.HasPrefix(o, lowerdirPrefix) {
			lower = strings.Split(strings.TrimPrefix(o, lowerdirPrefix), ":")
		}
	}
	return
}
