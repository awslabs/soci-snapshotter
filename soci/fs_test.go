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
	"testing"
)

func TestEnsureSnapshotterRootPath(t *testing.T) {
	t.Run("root path does not exist", func(t *testing.T) {
		testRoot := t.TempDir()
		if err := os.MkdirAll(testRoot+"/var/lib", 0755); err != nil {
			t.Fatalf("expected no error, got %q", err)
		}

		root := testRoot + "/var/lib/soci-snapshotter-grpc"
		if err := EnsureSnapshotterRootPath(root); err != nil {
			t.Fatalf("expected no error, got %q", err)
		}

		if _, err := os.Stat(root); err != nil {
			t.Fatalf("expected %q to exist, got %q", root, err)
		}
	})

	t.Run("root path already exists", func(t *testing.T) {
		testRoot := t.TempDir()
		root := testRoot + "/var/lib/soci-snapshotter-grpc"

		if err := os.MkdirAll(root, 0755); err != nil {
			t.Fatalf("expected no error, got %q", err)
		}

		if err := EnsureSnapshotterRootPath(root); err != nil {
			t.Fatalf("expected no error, got %q", err)
		}

		if _, err := os.Stat(root); err != nil {
			t.Fatalf("expected %q to exist, got %q", root, err)
		}
	})
}
