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

package internal

import "github.com/urfave/cli"

const (
	ManifestTypeFlagName = "manifest-type"
	ImageManifestType    = "image"
	ArtifactManifestType = "artifact"
)

var ManifestTypeFlag = cli.StringFlag{
	Name:  ManifestTypeFlagName,
	Value: ImageManifestType,
	Usage: `Generate either an OCI 1.1 artifact manifest or OCI 1.0 image manifest for the SOCI index. 
        (You should use 'artifact' only if you intend on interacting with a registry that supports OCI 1.1 artifacts)`,
}
