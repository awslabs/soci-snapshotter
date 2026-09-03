package fs

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/awslabs/soci-snapshotter/config"
	"github.com/containerd/containerd/v2/pkg/reference"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/content/memory"
)

// buildLayer returns a real gzipped tar containing one file, plus its
// compressed digest and its diffID.
func buildLayer(t *testing.T) (gz []byte, compressed, uncompressed digest.Digest) {
	t.Helper()
	var raw bytes.Buffer
	tw := tar.NewWriter(&raw)
	body := []byte("soci regression fixture\n")
	if err := tw.WriteHeader(&tar.Header{Name: "hello.txt", Mode: 0o644, Size: int64(len(body))}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(body); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}

	var zipped bytes.Buffer
	zw := gzip.NewWriter(&zipped)
	if _, err := zw.Write(raw.Bytes()); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return zipped.Bytes(), digest.FromBytes(zipped.Bytes()), digest.FromBytes(raw.Bytes())
}

// TestLocalContentStoreHitWithDiscardUnpackedLayers reproduces awslabs/soci-snapshotter#2035.
//
// With discard_unpacked_layers set, fs.go constructs a compressed verifier. When the
// layer is already present in the shared content store, parallelArtifactFetcher.Fetch
// short-circuits and never starts that verifier, so Verified() reports false for a
// layer whose bytes are provably correct, and Apply rejects it as a digest mismatch.
func TestLocalContentStoreHitWithDiscardUnpackedLayers(t *testing.T) {
	ctx := context.Background()
	gz, compressedDigest, uncompressedDigest := buildLayer(t)
	desc := ocispec.Descriptor{
		MediaType: ocispec.MediaTypeImageLayerGzip,
		Digest:    compressedDigest,
		Size:      int64(len(gz)),
	}

	// The layer is already in the content store, as it would be if any other
	// image on the node had pulled it. This is the case #2035 describes.
	local := &fakeLocalStore{Store: memory.New()}
	if err := local.Push(ctx, desc, bytes.NewReader(gz)); err != nil {
		t.Fatalf("seeding the local store: %v", err)
	}

	parallelCfg := &config.Parallel{
		Enable: true,
		ParallelConfig: config.ParallelConfig{
			MaxConcurrentDownloads:         4,
			MaxConcurrentDownloadsPerImage: 2,
			MaxConcurrentUnpacks:           2,
			MaxConcurrentUnpacksPerImage:   1,
			DiscardUnpackedLayers:          true,
		},
	}
	storage, err := newLayerUnpackDiskStorage(t.TempDir())
	if err != nil {
		t.Fatalf("newLayerUnpackDiskStorage: %v", err)
	}
	jobs, err := newUnpackJobs(ctx, parallelCfg, storage)
	if err != nil {
		t.Fatalf("newUnpackJobs: %v", err)
	}
	imageJob := jobs.GetOrAddImageJob(compressedDigest.String(), func(error) {})
	layerJob, err := jobs.AddLayerJob(imageJob, compressedDigest.String())
	if err != nil {
		t.Fatalf("AddLayerJob: %v", err)
	}

	// discard_unpacked_layers = true means fs.go builds a compressed verifier
	// and hands the SAME pointer to both the archive and the fetcher (fs.go:626,628).
	compressedVerifier := newAsyncVerifier(compressedDigest.Verifier())
	layerArchive := NewLayerArchive(compressedVerifier, newAsyncVerifier(uncompressedDigest.Verifier()), nil, nil)

	fetcher, err := newParallelArtifactFetcher(
		reference.Spec{Locator: "example.com/repo"},
		local, newFakeRemoteStore(gz), layerJob, 0, compressedVerifier,
	)
	if err != nil {
		t.Fatalf("newParallelArtifactFetcher: %v", err)
	}

	rc, fromLocal, err := fetcher.Fetch(ctx, desc)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	defer rc.Close()
	if !fromLocal {
		t.Fatal("expected the layer to be served from the local content store")
	}

	root := t.TempDir()
	if _, err := layerArchive.Apply(ctx, root, rc); err != nil {
		if strings.Contains(err.Error(), "compressed digests did not match") {
			t.Fatalf("#2035: correct local content rejected as a digest mismatch: %v", err)
		}
		t.Fatalf("Apply: %v", err)
	}
	// Prove the layer was really unpacked, not just that Apply returned nil.
	if got, err := os.ReadFile(filepath.Join(root, "hello.txt")); err != nil {
		t.Fatalf("layer was not applied: %v", err)
	} else if string(got) != "soci regression fixture\n" {
		t.Fatalf("unexpected file contents: %q", got)
	}
}

// corruptLocalStore reports a local hit but hands back bytes that do not match
// the descriptor, standing in for at-rest corruption of the content store.
type corruptLocalStore struct {
	*fakeLocalStore
	payload []byte
}

func (s *corruptLocalStore) Fetch(context.Context, ocispec.Descriptor) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(s.payload)), nil
}

// TestCorruptLocalContentIsStillRejected guards the fix. Skipping the compressed
// verifier on a local hit must not make corrupt content acceptable: the uncompressed
// verifier is constructed unconditionally and started inline in Apply, so it still
// checks whatever is actually applied to the filesystem.
func TestCorruptLocalContentIsStillRejected(t *testing.T) {
	ctx := context.Background()
	good, compressedDigest, uncompressedDigest := buildLayer(t)
	desc := ocispec.Descriptor{
		MediaType: ocispec.MediaTypeImageLayerGzip,
		Digest:    compressedDigest,
		Size:      int64(len(good)),
	}

	// Valid gzip and tar, but different bytes, so both digests are wrong.
	corrupt, _, _ := buildCorruptLayer(t)

	parallelCfg := &config.Parallel{
		Enable: true,
		ParallelConfig: config.ParallelConfig{
			MaxConcurrentDownloads:         4,
			MaxConcurrentDownloadsPerImage: 2,
			MaxConcurrentUnpacks:           2,
			MaxConcurrentUnpacksPerImage:   1,
			DiscardUnpackedLayers:          true,
		},
	}
	storage, err := newLayerUnpackDiskStorage(t.TempDir())
	if err != nil {
		t.Fatalf("newLayerUnpackDiskStorage: %v", err)
	}
	jobs, err := newUnpackJobs(ctx, parallelCfg, storage)
	if err != nil {
		t.Fatalf("newUnpackJobs: %v", err)
	}
	imageJob := jobs.GetOrAddImageJob(compressedDigest.String(), func(error) {})
	layerJob, err := jobs.AddLayerJob(imageJob, compressedDigest.String())
	if err != nil {
		t.Fatalf("AddLayerJob: %v", err)
	}

	compressedVerifier := newAsyncVerifier(compressedDigest.Verifier())
	layerArchive := NewLayerArchive(compressedVerifier, newAsyncVerifier(uncompressedDigest.Verifier()), nil, nil)

	local := &corruptLocalStore{
		fakeLocalStore: &fakeLocalStore{Store: memory.New()},
		payload:        corrupt,
	}
	fetcher, err := newParallelArtifactFetcher(
		reference.Spec{Locator: "example.com/repo"},
		local, newFakeRemoteStore(good), layerJob, 0, compressedVerifier,
	)
	if err != nil {
		t.Fatalf("newParallelArtifactFetcher: %v", err)
	}

	rc, fromLocal, err := fetcher.Fetch(ctx, desc)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	defer rc.Close()
	if !fromLocal {
		t.Fatal("expected a local hit")
	}

	if _, err := layerArchive.Apply(ctx, t.TempDir(), rc); err == nil {
		t.Fatal("corrupt local content was accepted")
	} else {
		t.Logf("corrupt local content rejected with: %v", err)
	}
}

// buildCorruptLayer returns a valid gzipped tar whose contents differ from buildLayer.
func buildCorruptLayer(t *testing.T) (gz []byte, compressed, uncompressed digest.Digest) {
	t.Helper()
	var raw bytes.Buffer
	tw := tar.NewWriter(&raw)
	body := []byte("tampered\n")
	if err := tw.WriteHeader(&tar.Header{Name: "hello.txt", Mode: 0o644, Size: int64(len(body))}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(body); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	var zipped bytes.Buffer
	zw := gzip.NewWriter(&zipped)
	if _, err := zw.Write(raw.Bytes()); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return zipped.Bytes(), digest.FromBytes(zipped.Bytes()), digest.FromBytes(raw.Bytes())
}
