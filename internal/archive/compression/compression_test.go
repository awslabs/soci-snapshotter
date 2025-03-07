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

// The following code was copied from https://github.com/containerd/containerd/blob/bcc810d6b9066471b0b6fa75f557a15a1cbf31bb/archive/compression/compression_test.go
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
	"fmt"
	"io"
	"os/exec"
	"sync"
	"testing"

	"github.com/awslabs/soci-snapshotter/config"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

func resetDecompressStreams() {
	decompressStreams = make(map[decompressionKey]DecompressStream)
	initDecompressors = sync.Once{}
}

// TestInitializeDecompressStreams asserts the snapshotters decompress streams can be initialized from configuration.
func TestInitializeDecompressStreams(t *testing.T) {
	tests := []struct {
		name                    string
		decompressStreamsConfig map[string]config.DecompressStream
		assert                  func(t *testing.T, err error)
	}{
		{
			name:                    "NoDecompressStreamsByDefault",
			decompressStreamsConfig: map[string]config.DecompressStream{},
			assert: func(t *testing.T, err error) {
				if len(decompressStreams) != 0 {
					t.Fatalf("expected no decompress streams, got %d", len(decompressStreams))
				}
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
			},
		},
		{
			name: "GzipDecompressStream",
			decompressStreamsConfig: map[string]config.DecompressStream{
				"gzip": {
					Path: "/usr/bin/gzip",
					Args: []string{
						"-d",
						"-c",
					},
				},
			},
			assert: func(t *testing.T, err error) {
				// Both docker and OCI image layers supports gzip compression.
				// See https://github.com/opencontainers/image-spec/blob/c05acf7eb327dae4704a4efe01253a0e60af6b34/media-types.md#applicationvndociimagelayerv1targzip
				if len(decompressStreams) != 2 {
					t.Fatalf("expected two decompress streams, got %d", len(decompressStreams))
				}
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
				ds, ok := decompressStreams[ociGzipDecompression]
				if !ok {
					t.Fatal("expected gzip decompression stream for OCI layers")
				}
				if ds == nil {
					t.Fatal("expected non-nil gzip decompression stream for OCI layers")
				}
				ds, ok = decompressStreams[dockerGzipDecompression]
				if !ok {
					t.Fatal("expected gzip decompression stream for Docker layers")
				}
				if ds == nil {
					t.Fatal("expected non-nil gzip decompression stream for Docker layers")
				}
			},
		},
		{
			name: "IgnoreInvalidDecompressStream",
			decompressStreamsConfig: map[string]config.DecompressStream{
				"supercompress": {
					Path: "/usr/bin/gzip",
					Args: []string{
						"-d",
						"-c",
					},
				},
			},
			assert: func(t *testing.T, err error) {
				if len(decompressStreams) != 0 {
					t.Fatalf("expected no decompress streams, got %d", len(decompressStreams))
				}
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
			},
		},
		{
			name: "GzipDecompressStreamNotFound",
			decompressStreamsConfig: map[string]config.DecompressStream{
				"gzip": {
					Path: "/usr/bin/superfastgzip",
					Args: []string{
						"-d",
						"-c",
					},
				},
			},
			assert: func(t *testing.T, err error) {
				if len(decompressStreams) != 0 {
					t.Fatalf("expected no decompress streams, got %d", len(decompressStreams))
				}
				if err == nil {
					t.Fatal("expected error, got nil")
				}
			},
		},
		{
			name: "ZstdDecompressStreamNotFound",
			decompressStreamsConfig: map[string]config.DecompressStream{
				"zstd": {
					Path: "/usr/bin/superfastzstd",
					Args: []string{
						"-d",
						"-c",
					},
				},
			},
			assert: func(t *testing.T, err error) {
				if len(decompressStreams) != 0 {
					t.Fatalf("expected no decompress streams, got %d", len(decompressStreams))
				}
				if err == nil {
					t.Fatal("expected error, got nil")
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resetDecompressStreams()

			err := InitializeDecompressStreams(test.decompressStreamsConfig)
			test.assert(t, err)
		})
	}
}

// TestGetDecompressStream asserts decompress streams for a media type can be retrieved.
func TestGetDecompressStream(t *testing.T) {
	resetDecompressStreams()

	tests := []struct {
		name           string
		layerMediaType string
	}{
		{
			name:           "DockerGzipCompressedLayer",
			layerMediaType: "application/vnd.docker.image.rootfs.diff.tar.gzip",
		},
		{
			name:           "OCIGzipCompressedLayer",
			layerMediaType: ocispec.MediaTypeImageLayerGzip,
		},
		{
			name:           "OCIZstdCompressLayer",
			layerMediaType: ocispec.MediaTypeImageLayerZstd,
		},
		{
			name:           "EmptyMediaType",
			layerMediaType: "",
		},
		{
			name:           "UnsupportedMediaType",
			layerMediaType: "application/vnd.oci.image.layer.v1.tar+unknown",
		},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("no streams initialized[%s]", test.name), func(t *testing.T) {
			ds, ok := GetDecompressStream(test.layerMediaType)
			if ok || ds != nil {
				t.Fatalf("expected no decompress stream for media type %s", test.layerMediaType)
			}
		})
	}

	InitializeDecompressStreams(map[string]config.DecompressStream{
		"gzip": {
			Path: "/usr/bin/gzip",
			Args: []string{
				"-d",
				"-c",
			},
		},
	})

	tests = []struct {
		name           string
		layerMediaType string
	}{
		{
			name:           "DockerGzipCompressedLayer",
			layerMediaType: "application/vnd.docker.image.rootfs.diff.tar.gzip",
		},
		{
			name:           "OCIGzipCompressedLayer",
			layerMediaType: ocispec.MediaTypeImageLayerGzip,
		},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("streams initialized[%s]", test.name), func(t *testing.T) {
			ds, ok := GetDecompressStream(test.layerMediaType)
			if !ok || ds == nil {
				t.Fatalf("expected decompress stream for media type %s", test.layerMediaType)
			}
		})
	}
}

func TestCmdStream(t *testing.T) {
	out, err := cmdStream(exec.Command("sh", "-c", "echo hello; exit 0"), nil)
	if err != nil {
		t.Fatal(err)
	}

	buf, err := io.ReadAll(out)
	if err != nil {
		t.Fatalf("failed to read from stdout: %s", err)
	}

	if string(buf) != "hello\n" {
		t.Fatalf("unexpected command output ('%s' != '%s')", string(buf), "hello\n")
	}
}

func TestCmdStreamBad(t *testing.T) {
	out, err := cmdStream(exec.Command("sh", "-c", "echo hello; echo >&2 bad result; exit 1"), nil)
	if err != nil {
		t.Fatalf("failed to start command: %v", err)
	}

	if buf, err := io.ReadAll(out); err == nil {
		t.Fatal("command should have failed")
	} else if err.Error() != "exit status 1: bad result\n" {
		t.Fatalf("wrong error: %s", err.Error())
	} else if string(buf) != "hello\n" {
		t.Fatalf("wrong output: %s", string(buf))
	}
}
