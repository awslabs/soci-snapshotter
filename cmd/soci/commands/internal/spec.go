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
	LegacyRegistryFlagName = "legacy-registry"
)

var LegacyRegistryFlag = cli.BoolFlag{
	Name: LegacyRegistryFlagName,
	Usage: `Whether to create the SOCI index for a legacy registry. OCI 1.1 added support for associating artifacts such as soci indices with images.
     There is a mechanism to emulate this behavior with OCI 1.0 registries by pretending that the SOCI index
     is itself an image. This option should only be use if the SOCI index will be pushed to a
     registry which does not support OCI 1.1 features.`,
}
