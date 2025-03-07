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

// The following code was copied from https://github.com/containerd/containerd/blob/bcc810d6b9066471b0b6fa75f557a15a1cbf31bb/archive/compression/compression.go
// and modified to allow for configurable decompression streams.

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

package compression

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"sync"

	"github.com/awslabs/soci-snapshotter/config"
	intos "github.com/awslabs/soci-snapshotter/internal/os"
	"github.com/containerd/containerd/archive/compression"
	"github.com/containerd/log"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

const (
	gzipCompression = "gzip"
	zstdCompression = "zstd"
)

var (
	initDecompressorsErr error
	initDecompressors    = sync.Once{}
	decompressStreams    = map[decompressionKey]DecompressStream{}
)

type decompressionKey string

const (
	// dockerGzipDecompression represents gzip compressed container image layers in Docker images.
	// It is copied from https://github.com/distribution/distribution/blob/e827ce2772e08f38884e5774846ffb1610965a0d/manifest/schema2/manifest.go#L27
	dockerGzipDecompression decompressionKey = "application/vnd.docker.image.rootfs.diff.tar.gzip"

	ociGzipDecompression decompressionKey = ocispec.MediaTypeImageLayerGzip
	ociZstdDecompression decompressionKey = ocispec.MediaTypeImageLayerZstd
)

// DecompressStream is any function which decompresses an archive.
//
// It is defined to match [github.com/containerd/containerd/archive/compression.DecompressStream] for easier swapping
// of decompress stream functionality.
type DecompressStream func(io.Reader) (compression.DecompressReadCloser, error)

// InitializeDecompressStreams initializes the snapshotter's decompress stream functions.
//
// This function is a singleton function; after the first call follow on calls will
// produce the same result.
func InitializeDecompressStreams(streams map[string]config.DecompressStream) error {
	initDecompressors.Do(func() {
		for alg, c := range streams {
			// Sanitize and validate the path
			sanitizedPath, err := intos.SanitizeExecutablePath(c.Path)
			if err != nil {
				initDecompressorsErr = fmt.Errorf("%s decompressor path validation failed: %w", alg, err)
				return
			}

			// Create a copy of the config with the sanitized path
			sanitizedConfig := config.DecompressStream{
				Path: sanitizedPath,
				Args: c.Args,
			}

			switch alg {
			case gzipCompression:
				decompressStreams[ociGzipDecompression] = decompress(newGzipDecompressor(sanitizedConfig))
				decompressStreams[dockerGzipDecompression] = decompressStreams[ociGzipDecompression]
			case zstdCompression:
				decompressStreams[ociZstdDecompression] = decompress(newZstdDecompressor(sanitizedConfig))
			default:
				log.L.WithField("algorithm", alg).Warn("Unsupported compression algorithm")
			}
		}
	})

	return initDecompressorsErr
}

// GetDecompressStream returns the decompress stream function for a provided layer media type
// if the snapshotter has been configured to decompress it.
//
// If no decompression stream is found, then return false.
func GetDecompressStream(layerMediaType string) (DecompressStream, bool) {
	dec, ok := decompressStreams[decompressionKey(layerMediaType)]
	return dec, ok
}

type readCloserWrapper struct {
	io.Reader
	compression compression.Compression
	closer      func() error
}

func (r *readCloserWrapper) Close() error {
	if r.closer != nil {
		return r.closer()
	}
	return nil
}

func (r *readCloserWrapper) GetCompression() compression.Compression {
	return r.compression
}

type decompressor interface {
	Compression() compression.Compression
	Path() string
	Args() []string
}

type gzipDecompressor struct {
	path string
	args []string
}

func newGzipDecompressor(c config.DecompressStream) decompressor {
	return &gzipDecompressor{
		path: c.Path,
		args: c.Args,
	}
}

func (gz gzipDecompressor) Compression() compression.Compression {
	return compression.Gzip
}

func (gz gzipDecompressor) Path() string {
	return gz.path
}

func (gz gzipDecompressor) Args() []string {
	return gz.args
}

type zstdDecompressor struct {
	path string
	args []string
}

func newZstdDecompressor(c config.DecompressStream) decompressor {
	return &zstdDecompressor{
		path: c.Path,
		args: c.Args,
	}
}

func (z zstdDecompressor) Compression() compression.Compression {
	return compression.Zstd
}

func (z zstdDecompressor) Path() string {
	return z.path
}

func (z zstdDecompressor) Args() []string {
	return z.args
}

func decompress(decompressor decompressor) DecompressStream {
	return func(r io.Reader) (compression.DecompressReadCloser, error) {
		switch decompressor.Compression() {
		case compression.Gzip:
			fallthrough
		case compression.Zstd:
			ctx, cancel := context.WithCancel(context.Background())
			r, err := cmdStream(exec.CommandContext(ctx, decompressor.Path(), decompressor.Args()...), r)
			if err != nil {
				cancel()
				return nil, err
			}
			return &readCloserWrapper{
				Reader:      r,
				compression: decompressor.Compression(),
				closer: func() error {
					cancel()
					return r.Close()
				},
			}, nil
		default:
			return nil, fmt.Errorf("unsupported compression algorithm: %v", decompressor.Compression())
		}
	}
}

func cmdStream(cmd *exec.Cmd, in io.Reader) (io.ReadCloser, error) {
	reader, writer := io.Pipe()

	cmd.Stdin = in
	cmd.Stdout = writer

	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	go func() {
		if err := cmd.Wait(); err != nil {
			writer.CloseWithError(fmt.Errorf("%s: %s", err, errBuf.String()))
		} else {
			writer.Close()
		}
	}()

	return reader, nil
}
