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

package soci

import (
	"os"

	"github.com/awslabs/soci-snapshotter/config"
)

// EnsureSnapshotterRootPath ensures that the snapshotter root path exists.
// It creates the directory with restricted permissions (0711) if it doesn't exist.
func EnsureSnapshotterRootPath(root string) error {
	if root == "" {
		root = config.DefaultSociSnapshotterRootPath
	}

	// Creating the snapshotter's root path first if it does not exist, since this ensures, that
	// it has the limited permission set as drwx--x--x.
	// The subsequent oci.New creates a root path dir with too broad permission set.
	if _, err := os.Stat(root); os.IsNotExist(err) {
		if err = os.Mkdir(root, 0700); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	return nil
}
