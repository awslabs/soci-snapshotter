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

package ztoc

import (
	"compress/gzip"
	"io"
	"os"
	"testing"

	"github.com/awslabs/soci-snapshotter/util/testutil"
	"github.com/awslabs/soci-snapshotter/ztoc/compression"
	"github.com/klauspost/compress/zstd"
)

func TestTocBuilder(t *testing.T) {
	t.Parallel()

	r := testutil.NewTestRand(t)
	tarEntries := []testutil.TarEntry{
		testutil.File("test1", string(r.RandomByteData(10000000))),
		testutil.File("test2", string(r.RandomByteData(20000000))),
	}

	tarReader := func(entries []testutil.TarEntry) io.Reader {
		return testutil.BuildTar(entries)
	}

	gzipTarReader := func(entries []testutil.TarEntry) io.Reader {
		return testutil.BuildTarGz(entries, gzip.BestCompression)
	}

	zstdTarReader := func(entries []testutil.TarEntry) io.Reader {
		return testutil.BuildTarZstd(entries, int(zstd.SpeedDefault))
	}

	testCases := []struct {
		name          string
		algorithm     string
		tarEntries    []testutil.TarEntry
		makeTarReader func(entries []testutil.TarEntry) io.Reader
		expectErr     bool
	}{
		{
			name:          "TocBuilder supports gzip",
			algorithm:     compression.Gzip,
			tarEntries:    tarEntries,
			makeTarReader: gzipTarReader,
			expectErr:     false,
		},
		{
			name:          "TocBuilder supports zstd",
			algorithm:     compression.Zstd,
			tarEntries:    tarEntries,
			makeTarReader: zstdTarReader,
			expectErr:     false,
		},
		{
			name:          "TocBuilder supports uncompressed layer (tar)",
			algorithm:     compression.Uncompressed,
			tarEntries:    tarEntries,
			makeTarReader: tarReader,
			expectErr:     false,
		},
		{
			name:          "TocBuilder doesn't support foobar",
			algorithm:     "foobar",
			tarEntries:    tarEntries,
			makeTarReader: tarReader,
			expectErr:     true,
		},
		{
			name:          "TocBuilder returns error if given tar file and algorithm mismatch",
			algorithm:     compression.Zstd,
			tarEntries:    tarEntries,
			makeTarReader: gzipTarReader,
			expectErr:     true,
		},
	}

	builder := NewTocBuilder()
	builder.RegisterTarProvider(compression.Gzip, TarProviderGzip)
	builder.RegisterTarProvider(compression.Zstd, TarProviderZstd)
	builder.RegisterTarProvider(compression.Uncompressed, TarProviderTar)

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			tarReader := tt.makeTarReader(tt.tarEntries)
			tarFile, _, err := testutil.WriteTarToTempFile("toc_builder", tarReader)
			if err != nil {
				t.Fatalf("failed to write content to tar file: %v", err)
			}
			defer os.Remove(tarFile)

			if toc, _, err := builder.TocFromFile(tt.algorithm, tarFile); err != nil {
				if !tt.expectErr {
					t.Fatalf("unexpected error: %v", err)
				}
			} else {
				if len(toc.FileMetadata) != len(tt.tarEntries) {
					t.Fatalf("count of file metadata mismatch, expect: %d, actual: %d", len(tt.tarEntries), len(toc.FileMetadata))
				}
			}
		})
	}
}
