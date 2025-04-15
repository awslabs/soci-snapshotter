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

package integration

import (
	"bytes"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/awslabs/soci-snapshotter/config"
	"github.com/awslabs/soci-snapshotter/soci"
	"github.com/awslabs/soci-snapshotter/soci/store"
	"github.com/awslabs/soci-snapshotter/util/dockershell"
	"github.com/awslabs/soci-snapshotter/util/testutil"
	"github.com/opencontainers/go-digest"
)

func TestRebuildArtifactsDB(t *testing.T) {
	regConfig := newRegistryConfig()
	sh, done := newShellWithRegistry(t, regConfig)
	defer done()
	rebootContainerd(t, sh, "", "")
	img := rabbitmqImage
	copyImage(sh, dockerhub(img), regConfig.mirror(img))
	indexDigest := buildIndex(sh, regConfig.mirror(img), withMinLayerSize(0))
	blobPath, _ := testutil.GetContentStoreBlobPath(config.DefaultContentStoreType)
	dgst, err := digest.Parse(indexDigest)
	if err != nil {
		t.Fatalf("cannot parse digest: %v", err)
	}
	indexBytes := sh.O("cat", filepath.Join(blobPath, dgst.Encoded()))
	var sociIndex soci.Index
	err = soci.DecodeIndex(bytes.NewBuffer(indexBytes), &sociIndex)
	if err != nil {
		t.Fatal(err)
	}
	sh.X("soci", "push", "--user", regConfig.creds(), regConfig.mirror(img).ref)

	verifyArtifacts := func(expectedIndexCount, expectedZtocCount int) error {
		indexOutput := sh.O("soci", "index", "list")
		ztocOutput := sh.O("soci", "ztoc", "list")
		indexCount := len(bytes.Split(indexOutput, []byte("\n"))) - 2
		ztocCount := len(bytes.Split(ztocOutput, []byte("\n"))) - 2
		if indexCount != expectedIndexCount {
			return fmt.Errorf("expected %v indices; got %v", expectedIndexCount, indexCount)
		}
		if ztocCount != expectedZtocCount {
			return fmt.Errorf(" expected %v ztoc; got %v", expectedZtocCount, ztocCount)
		}
		return nil
	}

	testCases := []struct {
		name               string
		setup              func(*dockershell.Shell, store.ContentStoreType)
		afterContent       bool
		expectedIndexCount int
		exptectedZtocCount int
	}{
		{
			name: "Rpull and rebuild %s content store",
			setup: func(sh *dockershell.Shell, contentStoreType store.ContentStoreType) {
				sh.X(
					append(imagePullCmd,
						regConfig.mirror(img).ref)...)
			},
			afterContent:       true,
			expectedIndexCount: 1,
			exptectedZtocCount: len(sociIndex.Blobs),
		},
		{
			name: "Remove artifacts from %s content store and rebuild",
			setup: func(sh *dockershell.Shell, contentStoreType store.ContentStoreType) {
				testutil.RemoveContentStoreContent(sh, contentStoreType, indexDigest)
				for _, blob := range sociIndex.Blobs {
					testutil.RemoveContentStoreContent(sh, contentStoreType, blob.Digest.String())
				}
			},
			expectedIndexCount: 0,
			exptectedZtocCount: 0,
		},
	}
	for _, tc := range testCases {
		for _, contentStoreType := range store.ContentStoreTypes() {
			t.Run(fmt.Sprintf(tc.name, contentStoreType), func(t *testing.T) {
				rebootContainerd(t, sh, "", getSnapshotterConfigToml(t, withContentStoreConfig(store.WithType(contentStoreType))))
				if !tc.afterContent {
					copyImage(sh, dockerhub(img), regConfig.mirror(img))
					buildIndex(sh, regConfig.mirror(img), withMinLayerSize(0), withContentStoreType(contentStoreType))
				}
				tc.setup(sh, contentStoreType)
				sh.X("soci", "--content-store", string(contentStoreType), "rebuild-db")
				err := verifyArtifacts(tc.expectedIndexCount, tc.exptectedZtocCount)
				if err != nil {
					t.Fatal(err)
				}
			})
		}
	}

}
