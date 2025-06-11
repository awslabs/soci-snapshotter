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
	"errors"
	"fmt"
	"io"
	"strings"

	socicompression "github.com/awslabs/soci-snapshotter/internal/archive/compression"
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
	Apply(ctx context.Context, root string, r io.Reader, opts ...archive.ApplyOpt) (int64, error)
}

type layerArchive struct {
	verifier         *layerVerifier
	decompressStream socicompression.DecompressStream
}

type layerVerifier struct {
	compressed   digest.Verifier
	uncompressed digest.Verifier
}

func NewLayerArchive(compressedVerifier, uncompressedVerifier digest.Verifier, decompressStream socicompression.DecompressStream) Archive {
	// If no layer decompress stream was provided, then use containerd's decompress stream implementation.
	if decompressStream == nil {
		decompressStream = compression.DecompressStream
	}
	return &layerArchive{
		verifier: &layerVerifier{
			compressed:   compressedVerifier,
			uncompressed: uncompressedVerifier,
		},
		decompressStream: decompressStream,
	}
}

func (la *layerArchive) Apply(ctx context.Context, root string, r io.Reader, opts ...archive.ApplyOpt) (int64, error) {
	// Decompress first, then apply.
	if la.verifier.compressed != nil {
		r = io.TeeReader(r, la.verifier.compressed)
	}

	decompressReader, err := la.decompressStream(r)
	if err != nil {
		return 0, fmt.Errorf("cannot decompress the stream: %w", err)
	}
	defer decompressReader.Close()

	verifiyReader := io.TeeReader(decompressReader, la.verifier.uncompressed)
	n, err := archive.Apply(ctx, root, verifiyReader, opts...)
	if err != nil {
		return 0, err
	}

	// Read any trailing data to ensure digest validation
	if _, err := io.Copy(io.Discard, verifiyReader); err != nil {
		return 0, err
	}

	if !la.verifier.uncompressed.Verified() {
		return 0, errors.New("uncompressed digests did not match")
	}
	if la.verifier.compressed != nil && !la.verifier.compressed.Verified() {
		return 0, errors.New("compressed digests did not match")
	}

	return n, nil
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
