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

package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/awslabs/soci-snapshotter/soci"
	shell "github.com/awslabs/soci-snapshotter/util/dockershell"
	"github.com/awslabs/soci-snapshotter/util/dockershell/compose"
	"github.com/awslabs/soci-snapshotter/util/testutil"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// TestSnapshotterStartup tests to run containerd + snapshotter and check plugin is
// recognized by containerd
func TestSnapshotterStartup(t *testing.T) {
	t.Parallel()
	sh, done := newSnapshotterBaseShell(t)
	defer done()
	rebootContainerd(t, sh, "", "")
	found := false
	err := sh.ForEach(shell.C("ctr", "plugin", "ls"), func(l string) bool {
		info := strings.Fields(l)
		if len(info) < 4 {
			t.Fatalf("malformed plugin info: %v", info)
		}
		if info[0] == "io.containerd.snapshotter.v1" && info[1] == "soci" && info[3] == "ok" {
			found = true
			return false
		}
		return true
	})
	if err != nil || !found {
		t.Fatalf("failed to get soci snapshotter status using ctr plugin ls: %v", err)
	}
}

// TestOptimizeConsistentSociArtifact tests if the Soci artifact is produced consistently across runs.
// This test does the following:
// 1. Generate Soci artifact
// 2. Copy the local content store to another folder
// 3. Generate Soci artifact for the same image again
// 4. Do the comparison of the Soci artifact blobs
// 5. Clean up the local content store folder and the folder used for comparison
// Due to the reason that this test will be doing manipulations with local content store folder,
// it should be never run in parallel with the other tests.
func TestOptimizeConsistentSociArtifact(t *testing.T) {
	regConfig := newRegistryConfig()

	// Setup environment
	sh, done := newShellWithRegistry(t, regConfig)
	defer done()

	tests := []struct {
		name           string
		containerImage string
	}{
		{
			name:           "soci artifact is consistently built for ubuntu",
			containerImage: ubuntuImage,
		},
		{
			name:           "soci artifact is consistently built for nginx",
			containerImage: nginxImage,
		},
		{
			name:           "soci artifact is consistently built for alpine",
			containerImage: alpineImage,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rebootContainerd(t, sh, "", "")
			copyImage(sh, dockerhub(tt.containerImage), regConfig.mirror(tt.containerImage))
			// optimize for the first time
			sh.
				X("rm", "-rf", blobStorePath)
			buildIndex(sh, regConfig.mirror(tt.containerImage))
			// move the artifact to a folder
			sh.
				X("rm", "-rf", "copy").
				X("mkdir", "copy").
				X("cp", "-r", blobStorePath, "copy") // move the contents of soci dir to another folder

			// optimize for the second time
			buildIndex(sh, regConfig.mirror(tt.containerImage))

			currContent := sh.O("ls", blobStorePath)
			prevContent := sh.O("ls", "copy/sha256")
			if !bytes.Equal(currContent, prevContent) {
				t.Fatalf("local content store: previously generated artifact is different")
			}

			fileNames := strings.Fields(string(currContent))
			for _, fn := range fileNames {
				if fn == "artifacts.db" {
					// skipping artifacts.db, since this is bbolt file and we have no control over its internals
					continue
				}
				out, _ := sh.OLog("cmp", filepath.Join("soci", fn), filepath.Join("copy", "soci", fn))
				if string(out) != "" {
					t.Fatalf("the artifact is different: %v", string(out))
				}
			}

			sh.X("rm", "-rf", blobStorePath).X("rm", "-rf", "copy")
		})
	}
}

func TestLazyPullWithSparseIndex(t *testing.T) {
	regConfig := newRegistryConfig()
	// Prepare config for containerd and snapshotter

	sh, done := newShellWithRegistry(t, regConfig)
	defer done()
	if err := testutil.WriteFileContents(sh, defaultContainerdConfigPath, getContainerdConfigYaml(t, false), 0600); err != nil {
		t.Fatalf("failed to write %v: %v", defaultContainerdConfigPath, err)
	}
	if err := testutil.WriteFileContents(sh, defaultSnapshotterConfigPath, getSnapshotterConfigYaml(t, false), 0600); err != nil {
		t.Fatalf("failed to write %v: %v", defaultSnapshotterConfigPath, err)
	}

	const imageName = "rethinkdb@sha256:4452aadba3e99771ff3559735dab16279c5a352359d79f38737c6fdca941c6e5"
	const imageManifestDigest = "sha256:4452aadba3e99771ff3559735dab16279c5a352359d79f38737c6fdca941c6e5"
	const minLayerSize = 10000000

	rebootContainerd(t, sh, "", "")
	copyImage(sh, dockerhub(imageName), regConfig.mirror(imageName))
	indexDigest := buildSparseIndex(sh, regConfig.mirror(imageName), minLayerSize, defaultSpanSize)

	fromNormalSnapshotter := func(image string) tarPipeExporter {
		return func(t *testing.T, tarExportArgs ...string) {
			rebootContainerd(t, sh, "", "")
			sh.X("ctr", "i", "pull", "--user", regConfig.creds(), image)
			sh.Pipe(nil, shell.C("ctr", "run", "--rm", image, "test", "tar", "-zc", "/usr"), tarExportArgs)
		}
	}
	export := func(sh *shell.Shell, image string, tarExportArgs []string) {
		sh.X("soci", "image", "rpull", "--user", regConfig.creds(), "--soci-index-digest", indexDigest, image)
		sh.Pipe(nil, shell.C("soci", "run", "--rm", "--snapshotter=soci", image, "test", "tar", "-zc", "/usr"), tarExportArgs)
	}

	imageManifestJSON := fetchContentByDigest(sh, imageManifestDigest)
	imageManifest := new(ocispec.Manifest)
	if err := json.Unmarshal(imageManifestJSON, imageManifest); err != nil {
		t.Fatalf("cannot unmarshal index manifest: %v", err)
	}

	layersToDownload := make([]ocispec.Descriptor, 0)
	for _, layerBlob := range imageManifest.Layers {
		if layerBlob.Size < minLayerSize {
			layersToDownload = append(layersToDownload, layerBlob)
		}
	}
	remoteSnapshotsExpectedCount := len(imageManifest.Layers) - len(layersToDownload)

	tests := []struct {
		name string
		want tarPipeExporter
		test tarPipeExporter
	}{
		{
			name: "Soci",
			want: fromNormalSnapshotter(regConfig.mirror(imageName).ref),
			test: func(t *testing.T, tarExportArgs ...string) {
				image := regConfig.mirror(imageName).ref
				rebootContainerd(t, sh, "", "")
				buildSparseIndex(sh, regConfig.mirror(imageName), minLayerSize, defaultSpanSize)
				sh.X("ctr", "i", "rm", imageName)
				export(sh, image, tarExportArgs)
				checkFuseMounts(t, sh, remoteSnapshotsExpectedCount)
				checkLayersInSnapshottersContentStore(t, sh, layersToDownload)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testSameTarContents(t, sh, tt.want, tt.test)
		})
	}
}

func checkFuseMounts(t *testing.T, sh *shell.Shell, remoteSnapshotsExpectedCount int) {
	mounts := string(sh.O("mount"))
	remoteSnapshotsActualCount := strings.Count(mounts, "fuse.rawBridge")
	if remoteSnapshotsExpectedCount != remoteSnapshotsActualCount {
		t.Fatalf("incorrect number of remote snapshots; expected=%d, actual=%d",
			remoteSnapshotsExpectedCount, remoteSnapshotsActualCount)
	}
}

func checkLayersInSnapshottersContentStore(t *testing.T, sh *shell.Shell, layers []ocispec.Descriptor) {
	for _, layer := range layers {
		layerPath := filepath.Join(blobStorePath, trimSha256Prefix(layer.Digest.String()))
		existenceResult := strings.TrimSuffix(string(sh.O("ls", layerPath)), "\n")
		if layerPath != existenceResult {
			t.Fatalf("layer file %s was not found in snapshotter's local content store, the result of ls=%s", layerPath, existenceResult)
		}
	}
}

// TestLazyPull tests if lazy pulling works.
func TestLazyPull(t *testing.T) {
	regConfig := newRegistryConfig()
	// Prepare config for containerd and snapshotter

	sh, done := newShellWithRegistry(t, regConfig)
	defer done()
	if err := testutil.WriteFileContents(sh, defaultContainerdConfigPath, getContainerdConfigYaml(t, false), 0600); err != nil {
		t.Fatalf("failed to write %v: %v", defaultContainerdConfigPath, err)
	}
	if err := testutil.WriteFileContents(sh, defaultSnapshotterConfigPath, getSnapshotterConfigYaml(t, false), 0600); err != nil {
		t.Fatalf("failed to write %v: %v", defaultSnapshotterConfigPath, err)
	}

	optimizedImageName1 := alpineImage
	optimizedImageName2 := nginxImage
	nonOptimizedImageName := ubuntuImage

	// Mirror images
	rebootContainerd(t, sh, "", "")
	copyImage(sh, dockerhub(optimizedImageName1), regConfig.mirror(optimizedImageName1))
	copyImage(sh, dockerhub(optimizedImageName2), regConfig.mirror(optimizedImageName2))
	copyImage(sh, dockerhub(nonOptimizedImageName), regConfig.mirror(nonOptimizedImageName))
	indexDigest1 := buildIndex(sh, regConfig.mirror(optimizedImageName1))
	indexDigest2 := buildIndex(sh, regConfig.mirror(optimizedImageName2))

	// Test if contents are pulled
	fromNormalSnapshotter := func(image string) tarPipeExporter {
		return func(t *testing.T, tarExportArgs ...string) {
			rebootContainerd(t, sh, "", "")
			sh.X("ctr", "i", "pull", "--user", regConfig.creds(), image)
			sh.Pipe(nil, shell.C("ctr", "run", "--rm", image, "test", "tar", "-zc", "/usr"), tarExportArgs)
		}
	}
	export := func(sh *shell.Shell, image string, tarExportArgs []string) {
		sh.X("soci", "image", "rpull", "--user", regConfig.creds(), "--soci-index-digest", indexDigest1, image)
		sh.Pipe(nil, shell.C("soci", "run", "--rm", "--snapshotter=soci", image, "test", "tar", "-zc", "/usr"), tarExportArgs)
	}

	// NOTE: these tests must be executed sequentially.
	tests := []struct {
		name string
		want tarPipeExporter
		test tarPipeExporter
	}{
		{
			name: "normal",
			want: fromNormalSnapshotter(regConfig.mirror(nonOptimizedImageName).ref),
			test: func(t *testing.T, tarExportArgs ...string) {
				image := regConfig.mirror(nonOptimizedImageName).ref
				rebootContainerd(t, sh, "", "")
				export(sh, image, tarExportArgs)
			},
		},
		{
			name: "Soci",
			want: fromNormalSnapshotter(regConfig.mirror(optimizedImageName1).ref),
			test: func(t *testing.T, tarExportArgs ...string) {
				image := regConfig.mirror(optimizedImageName1).ref
				m := rebootContainerd(t, sh, "", "")
				buildIndex(sh, regConfig.mirror(optimizedImageName1))
				sh.X("ctr", "i", "rm", optimizedImageName1)
				export(sh, image, tarExportArgs)
				m.CheckAllRemoteSnapshots(t)
			},
		},
		{
			name: "multi-image",
			want: fromNormalSnapshotter(regConfig.mirror(optimizedImageName1).ref),
			test: func(t *testing.T, tarExportArgs ...string) {
				image := regConfig.mirror(optimizedImageName1).ref
				m := rebootContainerd(t, sh, "", "")
				buildIndex(sh, regConfig.mirror(optimizedImageName2))
				sh.X("soci", "image", "rpull", "--user", regConfig.creds(), "--soci-index-digest", indexDigest2, regConfig.mirror(optimizedImageName2).ref)
				buildIndex(sh, regConfig.mirror(optimizedImageName1))
				sh.X("ctr", "i", "rm", optimizedImageName1)
				export(sh, image, tarExportArgs)
				m.CheckAllRemoteSnapshots(t)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testSameTarContents(t, sh, tt.want, tt.test)
		})
	}
}

// TestLazyPull tests if lazy pulling works when no index digest is provided (makes a Referrers API call)
func TestLazyPullNoIndexDigest(t *testing.T) {
	// Prepare config for containerd and snapshotter

	regConfig := newRegistryConfig()

	// Setup environment
	sh, done := newShellWithRegistry(t, regConfig)
	defer done()
	if err := testutil.WriteFileContents(sh, defaultContainerdConfigPath, getContainerdConfigYaml(t, false), 0600); err != nil {
		t.Fatalf("failed to write %v: %v", defaultContainerdConfigPath, err)
	}
	if err := testutil.WriteFileContents(sh, defaultSnapshotterConfigPath, getSnapshotterConfigYaml(t, false), 0600); err != nil {
		t.Fatalf("failed to write %v: %v", defaultSnapshotterConfigPath, err)
	}

	optimizedImageName := alpineImage
	nonOptimizedImageName := ubuntuImage

	// Mirror images
	rebootContainerd(t, sh, "", "")
	copyImage(sh, dockerhub(optimizedImageName), regConfig.mirror(optimizedImageName))
	copyImage(sh, dockerhub(nonOptimizedImageName), regConfig.mirror(nonOptimizedImageName))
	buildIndex(sh, regConfig.mirror(optimizedImageName))
	sh.X("soci", "push", "--user", regConfig.creds(), regConfig.mirror(optimizedImageName).ref)

	// Test if contents are pulled
	fromNormalSnapshotter := func(image string) tarPipeExporter {
		return func(t *testing.T, tarExportArgs ...string) {
			rebootContainerd(t, sh, "", "")
			sh.X("ctr", "i", "pull", "--user", regConfig.creds(), image)
			sh.Pipe(nil, shell.C("ctr", "run", "--rm", image, "test", "tar", "-zc", "/usr"), tarExportArgs)
		}
	}
	export := func(sh *shell.Shell, image string, tarExportArgs []string) {
		sh.X("soci", "image", "rpull", "--user", regConfig.creds(), image)
		sh.Pipe(nil, shell.C("soci", "run", "--rm", "--snapshotter=soci", image, "test", "tar", "-zc", "/usr"), tarExportArgs)
	}

	// NOTE: these tests must be executed sequentially.
	tests := []struct {
		name                    string
		want                    tarPipeExporter
		test                    tarPipeExporter
		checkAllRemoteSnapshots bool
	}{
		{
			name: "normal",
			want: fromNormalSnapshotter(regConfig.mirror(nonOptimizedImageName).ref),
			test: func(t *testing.T, tarExportArgs ...string) {
				image := regConfig.mirror(nonOptimizedImageName).ref
				export(sh, image, tarExportArgs)
			},
		},
		{
			name: "soci",
			want: fromNormalSnapshotter(regConfig.mirror(optimizedImageName).ref),
			test: func(t *testing.T, tarExportArgs ...string) {
				image := regConfig.mirror(optimizedImageName).ref
				sh.X("ctr", "i", "rm", regConfig.mirror(optimizedImageName).ref)
				export(sh, image, tarExportArgs)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := rebootContainerd(t, sh, "", "")
			testSameTarContents(t, sh, tt.want, tt.test)
			if tt.checkAllRemoteSnapshots {
				m.CheckAllRemoteSnapshots(t)
			}
		})
	}
}

// TestPullWithAribtraryBlobInvalidZtocFormat tests the snapshotter behavior if an arbitrary blob is passed
// as a Ztoc. In this case, the flatbuffer deserialization will fail, which will lead
// to the snapshotter mounting the layer as a normal overlayfs mount.
func TestPullWithAribtraryBlobInvalidZtocFormat(t *testing.T) {
	regConfig := newRegistryConfig()

	sh, done := newShellWithRegistry(t, regConfig)
	defer done()

	if err := testutil.WriteFileContents(sh, defaultContainerdConfigPath, getContainerdConfigYaml(t, false), 0600); err != nil {
		t.Fatalf("failed to write %v: %v", defaultContainerdConfigPath, err)
	}
	if err := testutil.WriteFileContents(sh, defaultSnapshotterConfigPath, getSnapshotterConfigYaml(t, false), 0600); err != nil {
		t.Fatalf("failed to write %v: %v", defaultSnapshotterConfigPath, err)
	}

	images := []struct {
		name   string
		digest string
	}{
		{
			name:   "rabbitmq",
			digest: "sha256:603be6b7fd5f1d8c6eab8e7a234ed30d664b9356ec1b87833f3a46bb6725458e",
		},
	}

	fromNormalSnapshotter := func(image string) tarPipeExporter {
		return func(t *testing.T, tarExportArgs ...string) {
			rebootContainerd(t, sh, "", "")
			sh.X("ctr", "i", "pull", "--user", regConfig.creds(), image)
			sh.Pipe(nil, shell.C("ctr", "run", "--rm", image, "test", "tar", "-zc", "/usr"), tarExportArgs)
		}
	}

	export := func(sh *shell.Shell, image, sociIndexDigest string, tarExportArgs []string) {
		sh.X("soci", "image", "rpull", "--user", regConfig.creds(), "--soci-index-digest", sociIndexDigest, image)
		sh.Pipe(nil, shell.C("soci", "run", "--rm", "--snapshotter=soci", image, "test", "tar", "-zc", "/usr"), tarExportArgs)
	}

	buildMaliciousIndex := func(sh *shell.Shell, imgDigest string) ([]byte, []ocispec.Descriptor, error) {
		sh.X("mkdir", "-p", blobStorePath)
		imgBytes := sh.O("ctr", "content", "get", imgDigest)
		var manifest ocispec.Manifest
		if err := json.Unmarshal(imgBytes, &manifest); err != nil {
			return nil, nil, err
		}

		var ztocDescs []ocispec.Descriptor
		for _, layer := range manifest.Layers {
			ztocBytes := testutil.RandomByteData(1000000)
			ztocDgst := digest.FromBytes(ztocBytes)
			ztocPath := fmt.Sprintf("%s/%s", blobStorePath, trimSha256Prefix(ztocDgst.String()))
			if err := testutil.WriteFileContents(sh, ztocPath, ztocBytes, 0600); err != nil {
				t.Fatalf("cannot write ztoc to path %s: %v", ztocPath, err)
			}
			ztocDescs = append(ztocDescs, ocispec.Descriptor{
				MediaType: soci.SociLayerMediaType,
				Digest:    digest.FromBytes(ztocBytes),
				Size:      100000,
				Annotations: map[string]string{
					soci.IndexAnnotationImageLayerDigest:    layer.Digest.String(),
					soci.IndexAnnotationImageLayerMediaType: layer.MediaType,
				},
			})
		}

		index := soci.Index{
			MediaType:    soci.OCIArtifactManifestMediaType,
			ArtifactType: soci.SociIndexArtifactType,
			Blobs:        ztocDescs,
			Subject: &ocispec.Descriptor{
				MediaType: soci.OCIArtifactManifestMediaType,
				Digest:    digest.Digest(imgDigest),
				Size:      int64(len(imgBytes)),
			},
		}

		b, err := json.Marshal(index)
		if err != nil {
			return nil, nil, err
		}
		return b, manifest.Layers, nil
	}

	for _, img := range images {
		t.Run(img.name, func(t *testing.T) {
			rebootContainerd(t, sh, "", "")
			imgRef := fmt.Sprintf("%s@%s", img.name, img.digest)
			sociImage := regConfig.mirror(imgRef)
			copyImage(sh, dockerhub(imgRef), sociImage)

			want := fromNormalSnapshotter(sociImage.ref)
			test := func(t *testing.T, tarExportArgs ...string) {
				image := sociImage.ref
				indexBytes, imgLayers, err := buildMaliciousIndex(sh, img.digest)
				if err != nil {
					t.Fatal(err)
				}
				sh.X("ctr", "i", "rm", image)
				indexDigest := digest.FromBytes(indexBytes)
				path := fmt.Sprintf("%s/%s", blobStorePath, trimSha256Prefix(indexDigest.String()))
				if err := testutil.WriteFileContents(sh, path, indexBytes, 0600); err != nil {
					t.Fatalf("cannot write index to path %s: %v", path, err)
				}
				export(sh, image, indexDigest.String(), tarExportArgs)
				checkFuseMounts(t, sh, 0)
				checkLayersInSnapshottersContentStore(t, sh, imgLayers)
			}

			testSameTarContents(t, sh, want, test)
		})
	}

}

// TestLazyPull tests if lazy pulling works without background fetch.
func TestLazyPullNoBackgroundFetch(t *testing.T) {
	regConfig := newRegistryConfig()
	// Prepare config for containerd and snapshotter

	backgroundFetcherConfig := `
[background_fetch]
disable = true
`

	// Setup environment
	sh, done := newShellWithRegistry(t, regConfig)
	defer done()
	if err := testutil.WriteFileContents(sh, defaultContainerdConfigPath, getContainerdConfigYaml(t, false), 0600); err != nil {
		t.Fatalf("failed to write %v: %v", defaultContainerdConfigPath, err)
	}
	if err := testutil.WriteFileContents(sh, defaultSnapshotterConfigPath, getSnapshotterConfigYaml(t, false, backgroundFetcherConfig), 0600); err != nil {
		t.Fatalf("failed to write %v: %v", defaultSnapshotterConfigPath, err)
	}

	optimizedImageName1 := alpineImage
	optimizedImageName2 := nginxImage
	nonOptimizedImageName := ubuntuImage

	// Mirror images
	rebootContainerd(t, sh, "", "")
	copyImage(sh, dockerhub(optimizedImageName1), regConfig.mirror(optimizedImageName1))
	copyImage(sh, dockerhub(optimizedImageName2), regConfig.mirror(optimizedImageName2))
	copyImage(sh, dockerhub(nonOptimizedImageName), regConfig.mirror(nonOptimizedImageName))
	indexDigest1 := buildIndex(sh, regConfig.mirror(optimizedImageName1))
	indexDigest2 := buildIndex(sh, regConfig.mirror(optimizedImageName2))

	// Test if contents are pulled
	fromNormalSnapshotter := func(image string) tarPipeExporter {
		return func(t *testing.T, tarExportArgs ...string) {
			rebootContainerd(t, sh, "", "")
			sh.X("ctr", "i", "pull", "--user", regConfig.creds(), image)
			sh.Pipe(nil, shell.C("ctr", "run", "--rm", image, "test", "tar", "-zc", "/usr"), tarExportArgs)
		}
	}
	export := func(sh *shell.Shell, image string, tarExportArgs []string) {
		sh.X("soci", "image", "rpull", "--user", regConfig.creds(), "--soci-index-digest", indexDigest1, image)
		sh.Pipe(nil, shell.C("soci", "run", "--rm", "--snapshotter=soci", image, "test", "tar", "-zc", "/usr"), tarExportArgs)
	}

	// NOTE: these tests must be executed sequentially.
	tests := []struct {
		name string
		want tarPipeExporter
		test tarPipeExporter
	}{
		{
			name: "normal",
			want: fromNormalSnapshotter(regConfig.mirror(nonOptimizedImageName).ref),
			test: func(t *testing.T, tarExportArgs ...string) {
				image := regConfig.mirror(nonOptimizedImageName).ref
				rebootContainerd(t, sh, "", "")
				export(sh, image, tarExportArgs)
			},
		},
		{
			name: "Soci",
			want: fromNormalSnapshotter(regConfig.mirror(optimizedImageName1).ref),
			test: func(t *testing.T, tarExportArgs ...string) {
				image := regConfig.mirror(optimizedImageName1).ref
				m := rebootContainerd(t, sh, "", "")
				buildIndex(sh, regConfig.mirror(optimizedImageName1))
				sh.X("ctr", "i", "rm", optimizedImageName1)
				export(sh, image, tarExportArgs)
				m.CheckAllRemoteSnapshots(t)
			},
		},
		{
			name: "multi-image",
			want: fromNormalSnapshotter(regConfig.mirror(optimizedImageName1).ref),
			test: func(t *testing.T, tarExportArgs ...string) {
				image := regConfig.mirror(optimizedImageName1).ref
				m := rebootContainerd(t, sh, "", "")
				buildIndex(sh, regConfig.mirror(optimizedImageName2))
				sh.X("soci", "image", "rpull", "--user", regConfig.creds(), "--soci-index-digest", indexDigest2, regConfig.mirror(optimizedImageName2).ref)
				buildIndex(sh, regConfig.mirror(optimizedImageName1))
				sh.X("ctr", "i", "rm", optimizedImageName1)
				export(sh, image, tarExportArgs)
				m.CheckAllRemoteSnapshots(t)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testSameTarContents(t, sh, tt.want, tt.test)
		})
	}
}

// TestMirror tests if mirror & refreshing functionalities of snapshotter work
func TestMirror(t *testing.T) {
	var (
		reporter    = testutil.NewTestingReporter(t)
		pRoot       = testutil.GetProjectRoot(t)
		caCertDir   = "/usr/local/share/ca-certificates"
		serviceName = "testing_mirror"
	)
	regConfig := newRegistryConfig()
	regAltConfig := newRegistryConfig(withPort(5000), withCreds(""), withPlainHTTP())

	// Setup dummy creds for test
	crt, key, err := generateRegistrySelfSignedCert(regConfig.host)
	if err != nil {
		t.Fatalf("failed to generate cert: %v", err)
	}
	htpasswd, err := generateBasicHtpasswd(regConfig.user, regConfig.pass)
	if err != nil {
		t.Fatalf("failed to generate htpasswd: %v", err)
	}

	authDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(authDir, "domain.key"), key, 0666); err != nil {
		t.Fatalf("failed to prepare key file")
	}
	if err := os.WriteFile(filepath.Join(authDir, "domain.crt"), crt, 0666); err != nil {
		t.Fatalf("failed to prepare crt file")
	}
	if err := os.WriteFile(filepath.Join(authDir, "htpasswd"), htpasswd, 0666); err != nil {
		t.Fatalf("failed to prepare htpasswd file")
	}

	targetStage := "containerd-snapshotter-base"

	// Run testing environment on docker compose
	c, err := compose.New(testutil.ApplyTextTemplate(t, `
version: "3.7"
services:
  {{.ServiceName}}:
    build:
      context: {{.ImageContextDir}}
      target: {{.TargetStage}}
    privileged: true
    init: true
    entrypoint: [ "sleep", "infinity" ]
    environment:
    - NO_PROXY=127.0.0.1,localhost,{{.RegistryHost}}:443
    tmpfs:
    - /tmp:exec,mode=777
    - /var/lib/containerd
    - /var/lib/soci-snapshotter-grpc
    volumes:
    - /dev/fuse:/dev/fuse
  registry:
    image: ghcr.io/oci-playground/registry:v3.0.0-alpha.1
    container_name: {{.RegistryHost}}
    environment:
    - REGISTRY_AUTH=htpasswd
    - REGISTRY_AUTH_HTPASSWD_REALM="Registry Realm"
    - REGISTRY_AUTH_HTPASSWD_PATH=/auth/htpasswd
    - REGISTRY_HTTP_TLS_CERTIFICATE=/auth/domain.crt
    - REGISTRY_HTTP_TLS_KEY=/auth/domain.key
    - REGISTRY_HTTP_ADDR={{.RegistryHost}}:443
    volumes:
    - {{.AuthDir}}:/auth:ro
  registry-alt:
    image: registry:2
    container_name: {{.RegistryAltHost}}
`, struct {
		TargetStage     string
		ServiceName     string
		ImageContextDir string
		RegistryHost    string
		RegistryAltHost string
		AuthDir         string
	}{
		TargetStage:     targetStage,
		ServiceName:     serviceName,
		ImageContextDir: pRoot,
		RegistryHost:    regConfig.host,
		RegistryAltHost: regAltConfig.host,
		AuthDir:         authDir,
	}),
		compose.WithBuildArgs(getBuildArgsFromEnv(t)...),
		compose.WithStdio(testutil.TestingLogDest()))
	if err != nil {
		t.Fatalf("failed to prepare compose: %v", err)
	}
	defer c.Cleanup()
	de, ok := c.Get(serviceName)
	if !ok {
		t.Fatalf("failed to get shell of service %v: %v", serviceName, err)
	}
	sh := shell.New(de, reporter)

	containerdMirrorConfig := fmt.Sprintf(`
[[plugins."io.containerd.snapshotter.v1.soci".resolver.host."%s".mirrors]]
host = "%s"
insecure = true
`, regConfig.host, regAltConfig.hostWithPort())

	snapshotterMirrorConfig := fmt.Sprintf(`
[blob]
check_always = true

[[resolver.host."%s".mirrors]]
host = "%s"
insecure = true
`, regConfig.host, regAltConfig.hostWithPort())

	// Setup environment
	if err := testutil.WriteFileContents(sh, defaultContainerdConfigPath, getContainerdConfigYaml(t, false, containerdMirrorConfig), 0600); err != nil {
		t.Fatalf("failed to write %v: %v", defaultContainerdConfigPath, err)
	}
	if err := testutil.WriteFileContents(sh, defaultSnapshotterConfigPath, getSnapshotterConfigYaml(t, false, snapshotterMirrorConfig), 0600); err != nil {
		t.Fatalf("failed to write %v: %v", defaultSnapshotterConfigPath, err)
	}
	if err := testutil.WriteFileContents(sh, filepath.Join(caCertDir, "domain.crt"), crt, 0600); err != nil {
		t.Fatalf("failed to write %v: %v", caCertDir, err)
	}
	sh.
		X("apt-get", "--no-install-recommends", "install", "-y", "iptables").
		X("update-ca-certificates").
		Retry(100, "nerdctl", "login", "-u", regConfig.user, "-p", regConfig.pass, regConfig.host)

	imageName := alpineImage
	// Mirror images
	rebootContainerd(t, sh, "", "")
	copyImage(sh, dockerhub(imageName), regConfig.mirror(imageName))
	copyImage(sh, regConfig.mirror(imageName), regAltConfig.mirror(imageName))
	indexDigest := buildIndex(sh, regConfig.mirror(imageName))

	// Pull images
	// NOTE: Registry connection will still be checked on each "run" because
	//       we added "check_always = true" to the configuration in the above.
	//       We use this behaviour for testing mirroring & refleshing functionality.
	rebootContainerd(t, sh, "", "")
	sh.X("ctr", "i", "pull", "--user", regConfig.creds(), regConfig.mirror(imageName).ref)
	sh.X("soci", "create", regConfig.mirror(imageName).ref)
	sh.X("soci", "image", "rpull", "--user", regConfig.creds(), "--soci-index-digest", indexDigest, regConfig.mirror(imageName).ref)
	registryHostIP, registryAltHostIP := getIP(t, sh, regConfig.host), getIP(t, sh, regAltConfig.host)
	export := func(image string) []string {
		return shell.C("soci", "run", "--rm", "--snapshotter=soci", image, "test", "tar", "-zc", "/usr")
	}
	sample := func(t *testing.T, tarExportArgs ...string) {
		sh.Pipe(nil, shell.C("ctr", "run", "--rm", regConfig.mirror(imageName).ref, "test", "tar", "-zc", "/usr"), tarExportArgs)
	}

	// test if mirroring is working (switching to registryAltHost)
	testSameTarContents(t, sh, sample,
		func(t *testing.T, tarExportArgs ...string) {
			sh.
				X("iptables", "-A", "OUTPUT", "-d", registryHostIP, "-j", "DROP").
				X("iptables", "-L").
				Pipe(nil, export(regConfig.mirror(imageName).ref), tarExportArgs).
				X("iptables", "-D", "OUTPUT", "-d", registryHostIP, "-j", "DROP")
		},
	)

	// test if refreshing is working (swithching back to registryHost)
	testSameTarContents(t, sh, sample,
		func(t *testing.T, tarExportArgs ...string) {
			sh.
				X("iptables", "-A", "OUTPUT", "-d", registryAltHostIP, "-j", "DROP").
				X("iptables", "-L").
				Pipe(nil, export(regConfig.mirror(imageName).ref), tarExportArgs).
				X("iptables", "-D", "OUTPUT", "-d", registryAltHostIP, "-j", "DROP")
		},
	)
}

func getIP(t *testing.T, sh *shell.Shell, name string) string {
	resolved := strings.Fields(string(sh.O("getent", "hosts", name)))
	if len(resolved) < 1 {
		t.Fatalf("failed to resolve name %v", name)
	}
	return resolved[0]
}

type tarPipeExporter func(t *testing.T, tarExportArgs ...string)

func testSameTarContents(t *testing.T, sh *shell.Shell, aC, bC tarPipeExporter) {
	aDir, err := testutil.TempDir(sh)
	if err != nil {
		t.Fatalf("failed to create temp dir A: %v", err)
	}
	bDir, err := testutil.TempDir(sh)
	if err != nil {
		t.Fatalf("failed to create temp dir B: %v", err)
	}
	aC(t, "tar", "-zxC", aDir)
	bC(t, "tar", "-zxC", bDir)
	sh.X("diff", "--no-dereference", "-qr", aDir+"/", bDir+"/")
}

// TestRpullImageThenRemove pulls and rpulls an image then removes it to confirm fuse mounts are unmounted
func TestRpullImageThenRemove(t *testing.T) {
	regConfig := newRegistryConfig()
	sh, done := newShellWithRegistry(t, regConfig)
	defer done()

	// Setup environment
	if err := testutil.WriteFileContents(sh, defaultContainerdConfigPath, getContainerdConfigYaml(t, false), 0600); err != nil {
		t.Fatalf("failed to write %v: %v", defaultContainerdConfigPath, err)
	}
	sh.
		X("update-ca-certificates").
		Retry(100, "nerdctl", "login", "-u", regConfig.user, "-p", regConfig.pass, regConfig.host)

	rebootContainerd(t, sh, "", "")

	containerImage := nginxImage

	copyImage(sh, dockerhub(containerImage), regConfig.mirror(containerImage))
	indexDigest := buildIndex(sh, regConfig.mirror(containerImage))

	rawJSON := sh.O("soci", "index", "info", indexDigest)
	var sociIndex soci.Index
	if err := json.Unmarshal(rawJSON, &sociIndex); err != nil {
		t.Fatalf("invalid soci index from digest %s: %v", indexDigest, rawJSON)
	}

	if len(sociIndex.Blobs) == 0 {
		t.Fatalf("soci index %s contains 0 blobs, invalidating this test", indexDigest)
	}

	sh.X("soci", "image", "rpull", "--user", regConfig.creds(), "--soci-index-digest", indexDigest, regConfig.mirror(containerImage).ref)

	checkFuseMounts(t, sh, len(sociIndex.Blobs))

	sh.X("ctr", "image", "rm", "--sync", regConfig.mirror(containerImage).ref)
	sh.X("ctr", "image", "rm", "--sync", dockerhub(containerImage).ref)

	checkFuseMounts(t, sh, 0)
}
