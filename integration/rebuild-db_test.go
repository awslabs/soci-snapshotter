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

	"github.com/awslabs/soci-snapshotter/soci"
)

func TestRebuildArtifactsDB(t *testing.T) {
	regConfig := newRegistryConfig()
	sh, done := newShellWithRegistry(t, regConfig)
	defer done()
	rebootContainerd(t, sh, "", "")
	img := rabbitmqImage
	copyImage(sh, dockerhub(img), regConfig.mirror(img))
	indexDigest := buildIndex(sh, regConfig.mirror(img), withMinLayerSize(0))
	indexBytes := sh.O("cat", filepath.Join(blobStorePath, trimSha256Prefix(indexDigest)))
	var sociIndex soci.Index
	err := soci.DecodeIndex(bytes.NewBuffer(indexBytes), &sociIndex)
	if err != nil {
		t.Fatal(err)
	}
	sh.X("soci", "push", "--user", regConfig.creds(), regConfig.mirror(img).ref)

	rebuildDb := []string{"soci", "rebuild-db"}

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
		cmd                []string
		afterContent       bool
		expectedIndexCount int
		exptectedZtocCount int
	}{
		{
			name:               "Rpull and rebuild",
			cmd:                []string{"soci", "image", "rpull", "--user", regConfig.creds(), regConfig.mirror(img).ref},
			afterContent:       true,
			expectedIndexCount: 1,
			exptectedZtocCount: len(sociIndex.Blobs),
		},
		{
			name:               "Remove artifacts from content store and rebuild",
			cmd:                []string{"rm", "-rf", blobStorePath},
			expectedIndexCount: 0,
			exptectedZtocCount: 0,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rebootContainerd(t, sh, "", "")
			if !tc.afterContent {
				copyImage(sh, dockerhub(img), regConfig.mirror(img))
				buildIndex(sh, regConfig.mirror(img), withMinLayerSize(0))
			}
			sh.X(tc.cmd...)
			sh.X(rebuildDb...)
			err := verifyArtifacts(tc.expectedIndexCount, tc.exptectedZtocCount)
			if err != nil {
				t.Fatal(err)
			}
		})
	}

}
