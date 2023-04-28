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
	"strconv"
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
			buildIndex(sh, regConfig.mirror(tt.containerImage), withMinLayerSize(0))
			// move the artifact to a folder
			sh.
				X("rm", "-rf", "copy").
				X("mkdir", "copy").
				X("cp", "-r", blobStorePath, "copy") // move the contents of soci dir to another folder

			// optimize for the second time
			sh.
				X("rm", "-rf", blobStorePath)
			buildIndex(sh, regConfig.mirror(tt.containerImage), withMinLayerSize(0))

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
				out, _ := sh.OLog("cmp", filepath.Join(blobStorePath, fn), filepath.Join("copy", "sha256", fn))
				if string(out) != "" {
					t.Fatalf("the artifact is different: %v", string(out))
				}
			}

			sh.X("rm", "-rf", blobStorePath).X("rm", "-rf", "copy")
		})
	}
}

func TestLazyPullWithSparseIndex(t *testing.T) {
	imageName := rabbitmqImage

	regConfig := newRegistryConfig()

	sh, done := newShellWithRegistry(t, regConfig)
	defer done()

	rebootContainerd(t, sh, getContainerdConfigToml(t, false), getSnapshotterConfigToml(t, false))

	_, minLayerSize, _ := middleSizeLayerInfo(t, sh, dockerhub(imageName))

	copyImage(sh, dockerhub(imageName), regConfig.mirror(imageName))
	indexDigest := buildIndex(sh, regConfig.mirror(imageName), withMinLayerSize(minLayerSize))

	fromNormalSnapshotter := func(image string) tarPipeExporter {
		return func(t *testing.T, tarExportArgs ...string) {
			rebootContainerd(t, sh, "", "")
			sh.X("nerdctl", "pull", "-q", image)
			sh.Pipe(nil, shell.C("ctr", "run", "--rm", image, "test", "tar", "-zc", "/usr"), tarExportArgs)
		}
	}
	export := func(sh *shell.Shell, image string, tarExportArgs []string) {
		sh.X("soci", "image", "rpull", "--user", regConfig.creds(), "--soci-index-digest", indexDigest, image)
		sh.Pipe(nil, shell.C("soci", "run", "--rm", "--snapshotter=soci", image, "test", "tar", "-zc", "/usr"), tarExportArgs)
	}

	imageManifestDigest, err := getManifestDigest(sh, dockerhub(imageName).ref, dockerhub(imageName).platform)
	if err != nil {
		t.Fatalf("Failed to get manifest digest: %v", err)
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
				buildIndex(sh, regConfig.mirror(imageName), withMinLayerSize(minLayerSize))
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

	sh, done := newShellWithRegistry(t, regConfig)
	defer done()

	optimizedImageName1 := rabbitmqImage
	optimizedImageName2 := nginxImage
	nonOptimizedImageName := ubuntuImage

	// Mirror images
	rebootContainerd(t, sh, getContainerdConfigToml(t, false), getSnapshotterConfigToml(t, false))
	copyImage(sh, dockerhub(optimizedImageName1), regConfig.mirror(optimizedImageName1))
	copyImage(sh, dockerhub(optimizedImageName2), regConfig.mirror(optimizedImageName2))
	copyImage(sh, dockerhub(nonOptimizedImageName), regConfig.mirror(nonOptimizedImageName))
	indexDigest1 := buildIndex(sh, regConfig.mirror(optimizedImageName1), withMinLayerSize(0))
	indexDigest2 := buildIndex(sh, regConfig.mirror(optimizedImageName2), withMinLayerSize(0))

	// Test if contents are pulled
	fromNormalSnapshotter := func(image string) tarPipeExporter {
		return func(t *testing.T, tarExportArgs ...string) {
			rebootContainerd(t, sh, "", "")
			sh.X("nerdctl", "pull", "-q", image)
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
				rsm, done := testutil.NewRemoteSnapshotMonitor(m)
				defer done()
				buildIndex(sh, regConfig.mirror(optimizedImageName1), withMinLayerSize(0))
				sh.X("ctr", "i", "rm", optimizedImageName1)
				export(sh, image, tarExportArgs)
				rsm.CheckAllRemoteSnapshots(t)
			},
		},
		{
			name: "multi-image",
			want: fromNormalSnapshotter(regConfig.mirror(optimizedImageName1).ref),
			test: func(t *testing.T, tarExportArgs ...string) {
				image := regConfig.mirror(optimizedImageName1).ref
				m := rebootContainerd(t, sh, "", "")
				rsm, done := testutil.NewRemoteSnapshotMonitor(m)
				defer done()
				buildIndex(sh, regConfig.mirror(optimizedImageName2), withMinLayerSize(0))
				sh.X("soci", "image", "rpull", "--user", regConfig.creds(), "--soci-index-digest", indexDigest2, regConfig.mirror(optimizedImageName2).ref)
				buildIndex(sh, regConfig.mirror(optimizedImageName1), withMinLayerSize(0))
				sh.X("ctr", "i", "rm", optimizedImageName1)
				export(sh, image, tarExportArgs)
				rsm.CheckAllRemoteSnapshots(t)
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
	regConfig := newRegistryConfig()

	sh, done := newShellWithRegistry(t, regConfig)
	defer done()

	optimizedImageName := alpineImage
	nonOptimizedImageName := ubuntuImage

	// Mirror images
	rebootContainerd(t, sh, getContainerdConfigToml(t, false), getSnapshotterConfigToml(t, false))
	copyImage(sh, dockerhub(optimizedImageName), regConfig.mirror(optimizedImageName))
	copyImage(sh, dockerhub(nonOptimizedImageName), regConfig.mirror(nonOptimizedImageName))
	buildIndex(sh, regConfig.mirror(optimizedImageName), withMinLayerSize(0))
	sh.X("soci", "push", "--user", regConfig.creds(), regConfig.mirror(optimizedImageName).ref)

	// Test if contents are pulled
	fromNormalSnapshotter := func(image string) tarPipeExporter {
		return func(t *testing.T, tarExportArgs ...string) {
			rebootContainerd(t, sh, "", "")
			sh.X("nerdctl", "pull", "-q", image)
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
			rsm, done := testutil.NewRemoteSnapshotMonitor(m)
			defer done()
			testSameTarContents(t, sh, tt.want, tt.test)
			if tt.checkAllRemoteSnapshots {
				rsm.CheckAllRemoteSnapshots(t)
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
			rebootContainerd(t, sh, getContainerdConfigToml(t, false), getSnapshotterConfigToml(t, false))
			sh.X("nerdctl", "pull", "-q", image)
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

		subject := ocispec.Descriptor{
			Digest: digest.Digest(imgDigest),
			Size:   int64(len(imgBytes)),
		}
		index := soci.NewIndex(ztocDescs, &subject, nil)

		b, err := soci.MarshalIndex(index)
		if err != nil {
			return nil, nil, err
		}
		return b, manifest.Layers, nil
	}

	for _, img := range images {
		t.Run(img.name, func(t *testing.T) {
			rebootContainerd(t, sh, getContainerdConfigToml(t, false), getSnapshotterConfigToml(t, false))
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
	const backgroundFetcherConfig = `
[background_fetch]
disable = true
`

	regConfig := newRegistryConfig()

	sh, done := newShellWithRegistry(t, regConfig)
	defer done()

	optimizedImageName1 := rabbitmqImage
	optimizedImageName2 := nginxImage
	nonOptimizedImageName := ubuntuImage

	// Mirror images
	rebootContainerd(t, sh, getContainerdConfigToml(t, false), getSnapshotterConfigToml(t, false, backgroundFetcherConfig))
	copyImage(sh, dockerhub(optimizedImageName1), regConfig.mirror(optimizedImageName1))
	copyImage(sh, dockerhub(optimizedImageName2), regConfig.mirror(optimizedImageName2))
	copyImage(sh, dockerhub(nonOptimizedImageName), regConfig.mirror(nonOptimizedImageName))
	indexDigest1 := buildIndex(sh, regConfig.mirror(optimizedImageName1), withMinLayerSize(0))
	indexDigest2 := buildIndex(sh, regConfig.mirror(optimizedImageName2), withMinLayerSize(0))

	// Test if contents are pulled
	fromNormalSnapshotter := func(image string) tarPipeExporter {
		return func(t *testing.T, tarExportArgs ...string) {
			rebootContainerd(t, sh, "", "")
			sh.X("nerdctl", "pull", "-q", image)
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
				rsm, done := testutil.NewRemoteSnapshotMonitor(m)
				defer done()
				buildIndex(sh, regConfig.mirror(optimizedImageName1), withMinLayerSize(0))
				sh.X("ctr", "i", "rm", optimizedImageName1)
				export(sh, image, tarExportArgs)
				rsm.CheckAllRemoteSnapshots(t)
			},
		},
		{
			name: "multi-image",
			want: fromNormalSnapshotter(regConfig.mirror(optimizedImageName1).ref),
			test: func(t *testing.T, tarExportArgs ...string) {
				image := regConfig.mirror(optimizedImageName1).ref
				m := rebootContainerd(t, sh, "", "")
				rsm, done := testutil.NewRemoteSnapshotMonitor(m)
				defer done()
				buildIndex(sh, regConfig.mirror(optimizedImageName2), withMinLayerSize(0))
				sh.X("soci", "image", "rpull", "--user", regConfig.creds(), "--soci-index-digest", indexDigest2, regConfig.mirror(optimizedImageName2).ref)
				buildIndex(sh, regConfig.mirror(optimizedImageName1), withMinLayerSize(0))
				sh.X("ctr", "i", "rm", optimizedImageName1)
				export(sh, image, tarExportArgs)
				rsm.CheckAllRemoteSnapshots(t)
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
		caCertDir   = "/usr/local/share/ca-certificates"
		serviceName = "testing_mirror"
	)
	pRoot, err := testutil.GetProjectRoot()
	if err != nil {
		t.Fatal(err)
	}
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
	s, err := testutil.ApplyTextTemplate(composeRegistryAltTemplate, dockerComposeYaml{
		TargetStage:     targetStage,
		ServiceName:     serviceName,
		ImageContextDir: pRoot,
		RegistryHost:    regConfig.host,
		RegistryAltHost: regAltConfig.host,
		AuthDir:         authDir,
	})
	if err != nil {
		t.Fatal(err)
	}
	buildArgs, err := getBuildArgsFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	c, err := compose.Up(s,
		compose.WithBuildArgs(buildArgs...),
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
	if err := testutil.WriteFileContents(sh, filepath.Join(caCertDir, "domain.crt"), crt, 0600); err != nil {
		t.Fatalf("failed to write %v: %v", caCertDir, err)
	}
	sh.
		X("apt-get", "--no-install-recommends", "install", "-y", "iptables").
		X("update-ca-certificates").
		Retry(100, "nerdctl", "login", "-u", regConfig.user, "-p", regConfig.pass, regConfig.host)

	imageName := rabbitmqImage
	// Mirror images
	rebootContainerd(t, sh, getContainerdConfigToml(t, false, containerdMirrorConfig), getSnapshotterConfigToml(t, false, snapshotterMirrorConfig))
	copyImage(sh, dockerhub(imageName), regConfig.mirror(imageName))
	copyImage(sh, regConfig.mirror(imageName), regAltConfig.mirror(imageName))
	indexDigest := buildIndex(sh, regConfig.mirror(imageName), withMinLayerSize(0))

	// Pull images
	// NOTE: Registry connection will still be checked on each "run" because
	//       we added "check_always = true" to the configuration in the above.
	//       We use this behaviour for testing mirroring & refleshing functionality.
	rebootContainerd(t, sh, "", "")
	sh.X("nerdctl", "pull", "-q", regConfig.mirror(imageName).ref)
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

	rebootContainerd(t, sh, getContainerdConfigToml(t, false), "")

	containerImage := nginxImage

	copyImage(sh, dockerhub(containerImage), regConfig.mirror(containerImage))
	indexDigest := buildIndex(sh, regConfig.mirror(containerImage), withMinLayerSize(0))

	rawJSON := sh.O("soci", "index", "info", indexDigest)
	var sociIndex soci.Index
	if err := soci.UnmarshalIndex(rawJSON, &sociIndex); err != nil {
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

// TestRpullImageWithMinLayerSize pulls and rpulls an image with a runtime min_layer_size to confirm small layers are mounted locally
func TestRpullImageWithMinLayerSize(t *testing.T) {
	containerImage := rabbitmqImage

	regConfig := newRegistryConfig()
	sh, done := newShellWithRegistry(t, regConfig)
	defer done()

	// Start soci with default config
	rebootContainerd(t, sh, getContainerdConfigToml(t, false), getSnapshotterConfigToml(t, false))

	middleIndex, middleSize, layerCount := middleSizeLayerInfo(t, sh, dockerhub(containerImage))

	minLayerSizeConfig := `
[snapshotter]
min_layer_size=` + strconv.FormatInt(middleSize, 10) + `
`
	// Start soci with config to test
	rebootContainerd(t, sh, getContainerdConfigToml(t, false), getSnapshotterConfigToml(t, false, minLayerSizeConfig))

	copyImage(sh, dockerhub(containerImage), regConfig.mirror(containerImage))
	indexDigest := buildIndex(sh, regConfig.mirror(containerImage), withMinLayerSize(0))

	sh.X("soci", "image", "rpull", "--user", regConfig.creds(), "--soci-index-digest", indexDigest, regConfig.mirror(containerImage).ref)

	checkFuseMounts(t, sh, layerCount-middleIndex)
}
