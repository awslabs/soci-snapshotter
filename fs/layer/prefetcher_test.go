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

package layer

import (
	"compress/gzip"
	"math/rand"
	"testing"

	"github.com/awslabs/soci-snapshotter/cache"
	spanmanager "github.com/awslabs/soci-snapshotter/fs/span-manager"
	"github.com/awslabs/soci-snapshotter/soci"
	"github.com/awslabs/soci-snapshotter/util/testutil"
)

func init() {
	rand.Seed(100)
}

func TestPrefetcher(t *testing.T) {
	spanSize := 65536 // 64 KiB
	tarEntries := []testutil.TarEntry{
		testutil.File("file1.txt", string(genRandomByteData(100000))),
		testutil.Dir("dir/"),
		testutil.File("dir/file2.txt", string(genRandomByteData(100000))),
	}
	ztoc, r, err := soci.BuildZtocReader(tarEntries, gzip.BestCompression, int64(spanSize))
	if err != nil {
		t.Fatalf("failed to create ztoc: %v", err)
	}

	spanCache := cache.NewMemoryCache()
	defer spanCache.Close()
	spanManager := spanmanager.New(ztoc, r, spanCache)
	prefetcher := newPrefetcher(r, spanManager)

	err = prefetcher.prefetch()
	if err != nil {
		t.Fatalf("prefetch failed: %v", err)
	}
}

func genRandomByteData(size int) []byte {
	b := make([]byte, size)
	rand.Read(b)
	return b
}
