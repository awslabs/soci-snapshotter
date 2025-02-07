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
	"github.com/opencontainers/go-digest"
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
	Apply(ctx context.Context, root string, r io.Reader, verifier *layerVerifier, opts ...archive.ApplyOpt) (int64, error)
}

type layerArchive struct {
}

func NewLayerArchive() Archive {
	return &layerArchive{}
}

// wrapReader uses a TeeReader to feed content into verifier (compressed or uncompressed).
// If verification is disabled, this is a no-op.
func wrapReader(r io.Reader, verifier *layerVerifier, compressed bool) io.Reader {
	if verifier != nil {
		if compressed {
			r = io.TeeReader(r, verifier.compressed)
		} else {
			r = io.TeeReader(r, verifier.uncompressed)
		}
	}
	return r
}

func (la *layerArchive) Apply(ctx context.Context, root string, r io.Reader, verifier *layerVerifier, opts ...archive.ApplyOpt) (int64, error) {
	// we use containerd implementation here
	// decompress first and then apply
	r = wrapReader(r, verifier, true)
	decompressReader, err := compression.DecompressStream(r)
	if err != nil {
		return 0, fmt.Errorf("cannot decompress the stream: %w", err)
	}
	defer decompressReader.Close()

	r = wrapReader(decompressReader, verifier, false)
	n, err := archive.Apply(ctx, root, r, opts...)
	if err != nil {
		return 0, err
	}

	if verifier != nil {
		if !verifier.compressed.Verified() {
			return 0, fmt.Errorf("compressed digests did not match")
		}
		if !verifier.uncompressed.Verified() {
			return 0, fmt.Errorf("uncompressed digests did not match")
		}
	}

	return n, nil
}

type layerUnpacker struct {
	fetcher   Fetcher
	archive   Archive
	diffIDMap map[string]digest.Digest
}

type layerVerifier struct {
	compressed   digest.Verifier
	uncompressed digest.Verifier
}

func NewLayerUnpacker(fetcher Fetcher, archive Archive, diffIDMap map[string]digest.Digest) Unpacker {
	return &layerUnpacker{
		fetcher:   fetcher,
		archive:   archive,
		diffIDMap: diffIDMap,
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
	parents, err := getLayerParents(mounts[0].Options)
	if err != nil {
		return fmt.Errorf("cannot get layer parents: %w", err)
	}
	opts := []archive.ApplyOpt{
		archive.WithConvertWhiteout(archive.OverlayConvertWhiteout),
	}
	if len(parents) > 0 {
		opts = append(opts, archive.WithParents(parents))
	}

	var verifier *layerVerifier
	if lu.diffIDMap != nil {
		uncompressedDigest, ok := lu.diffIDMap[desc.Digest.String()]
		if !ok {
			return fmt.Errorf("error getting diff ID")
		}

		verifier = &layerVerifier{
			compressed:   desc.Digest.Verifier(),
			uncompressed: uncompressedDigest.Verifier(),
		}
	}
	_, err = lu.archive.Apply(ctx, mountpoint, rc, verifier, opts...)
	if err != nil {
		return fmt.Errorf("cannot apply layer: %w", err)
	}

	return nil
}

func getLayerParents(options []string) (lower []string, err error) {
	const lowerdirPrefix = "lowerdir="

	for _, o := range options {
		if strings.HasPrefix(o, lowerdirPrefix) {
			lower = strings.Split(strings.TrimPrefix(o, lowerdirPrefix), ":")
		}
	}
	return
}
