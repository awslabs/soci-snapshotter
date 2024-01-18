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

package metadata

import (
	"compress/gzip"
	"fmt"
	_ "net/http/pprof"
	"os"
	"testing"
	"time"

	"github.com/awslabs/soci-snapshotter/util/testutil"
	"github.com/awslabs/soci-snapshotter/ztoc"
	"golang.org/x/sync/errgroup"
)

var allowedPrefix = [4]string{"", "./", "/", "../"}

var srcCompressions = map[string]int{
	"gzip-nocompression":      gzip.NoCompression,
	"gzip-bestspeed":          gzip.BestSpeed,
	"gzip-bestcompression":    gzip.BestCompression,
	"gzip-defaultcompression": gzip.DefaultCompression,
	"gzip-huffmanonly":        gzip.HuffmanOnly,
}

func TestMetadataReader(t *testing.T) {
	sampleTime := time.Now().Truncate(time.Second)
	tests := []struct {
		name string
		in   []testutil.TarEntry
		want []check
	}{
		{
			name: "files",
			in: []testutil.TarEntry{
				testutil.File("file1", "foofoo", testutil.WithFileMode(0644|os.ModeSetuid)),
				testutil.Dir("dir1/"),
				testutil.File("dir1/file2.txt", "bazbazbaz", testutil.WithFileOwner(1000, 1000)),
				testutil.File("file3.txt", "xxxxx", testutil.WithFileModTime(sampleTime)),
				testutil.File("file4.txt", "", testutil.WithFileXattrs(map[string]string{"testkey": "testval"})),
			},
			want: []check{
				numOfNodes(6), // root dir + 1 dir + 4 files
				hasFile("file1", 6),
				hasMode("file1", 0644|os.ModeSetuid),
				hasFile("dir1/file2.txt", 9),
				hasOwner("dir1/file2.txt", 1000, 1000),
				hasFile("file3.txt", 5),
				hasModTime("file3.txt", sampleTime),
				hasFile("file4.txt", 0),
				// For details on the keys of Xattrs, see https://pkg.go.dev/archive/tar#Header
				hasXattrs("file4.txt", map[string]string{"testkey": "testval"}),
			},
		},
		{
			name: "dirs",
			in: []testutil.TarEntry{
				testutil.Dir("dir1/", testutil.WithDirMode(os.ModeDir|0600|os.ModeSticky)),
				testutil.Dir("dir1/dir2/", testutil.WithDirOwner(1000, 1000)),
				testutil.File("dir1/dir2/file1.txt", "testtest"),
				testutil.File("dir1/dir2/file2", "x"),
				testutil.File("dir1/dir2/file3", "yyy"),
				testutil.Dir("dir1/dir3/", testutil.WithDirModTime(sampleTime)),
				testutil.Dir("dir1/dir3/dir4/", testutil.WithDirXattrs(map[string]string{"testkey": "testval"})),
				testutil.File("dir1/dir3/dir4/file4", "1111111111"),
			},
			want: []check{
				numOfNodes(9), // root dir + 4 dirs + 4 files
				hasDirChildren("dir1", "dir2", "dir3"),
				hasDirChildren("dir1/dir2", "file1.txt", "file2", "file3"),
				hasDirChildren("dir1/dir3", "dir4"),
				hasDirChildren("dir1/dir3/dir4", "file4"),
				hasMode("dir1", os.ModeDir|0600|os.ModeSticky),
				hasOwner("dir1/dir2", 1000, 1000),
				hasModTime("dir1/dir3", sampleTime),
				hasXattrs("dir1/dir3/dir4", map[string]string{"testkey": "testval"}),
				hasFile("dir1/dir2/file1.txt", 8),
				hasFile("dir1/dir2/file2", 1),
				hasFile("dir1/dir2/file3", 3),
				hasFile("dir1/dir3/dir4/file4", 10),
			},
		},
		{
			name: "hardlinks",
			in: []testutil.TarEntry{
				testutil.File("file1", "foofoo", testutil.WithFileOwner(1000, 1000)),
				testutil.Dir("dir1/"),
				testutil.Link("dir1/link1", "file1"),
				testutil.Link("dir1/link2", "dir1/link1"),
				testutil.Dir("dir1/dir2/"),
				testutil.File("dir1/dir2/file2.txt", "testtest"),
				testutil.Link("link3", "dir1/dir2/file2.txt"),
				testutil.Symlink("link4", "dir1/link2"),
			},
			want: []check{
				numOfNodes(6), // root dir + 2 dirs + 1 file(linked) + 1 file(linked) + 1 symlink
				hasFile("file1", 6),
				hasOwner("file1", 1000, 1000),
				hasFile("dir1/link1", 6),
				hasOwner("dir1/link1", 1000, 1000),
				hasFile("dir1/link2", 6),
				hasOwner("dir1/link2", 1000, 1000),
				hasFile("dir1/dir2/file2.txt", 8),
				hasFile("link3", 8),
				hasDirChildren("dir1", "link1", "link2", "dir2"),
				hasDirChildren("dir1/dir2", "file2.txt"),
				sameNodes("file1", "dir1/link1", "dir1/link2"),
				sameNodes("dir1/dir2/file2.txt", "link3"),
				linkName("link4", "dir1/link2"),
				hasNumLink("file1", 3), // parent dir + 2 links
				hasNumLink("link3", 2), // parent dir + 1 link
				hasNumLink("dir1", 3),  // parent + "." + child's ".."
			},
		},
		{
			name: "various files",
			in: []testutil.TarEntry{
				testutil.Dir("dir1/"),
				testutil.File("dir1/../dir1///////////////////file1", ""),
				testutil.Chardev("dir1/cdev", 10, 11),
				testutil.Blockdev("dir1/bdev", 100, 101),
				testutil.Fifo("dir1/fifo"),
			},
			want: []check{
				numOfNodes(6), // root dir + 1 file + 1 dir + 1 cdev + 1 bdev + 1 fifo
				hasFile("dir1/file1", 0),
				hasChardev("dir1/cdev", 10, 11),
				hasBlockdev("dir1/bdev", 100, 101),
				hasFifo("dir1/fifo"),
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		for _, prefix := range allowedPrefix {
			prefix := prefix
			for srcCompresionName, srcCompression := range srcCompressions {
				srcCompresionName, srcCompression := srcCompresionName, srcCompression
				t.Run(tt.name+"-"+srcCompresionName, func(t *testing.T) {
					opts := []testutil.BuildTarOption{
						testutil.WithPrefix(prefix),
					}

					ztoc, sr, err := ztoc.BuildZtocReader(t, tt.in, srcCompression, 64, opts...)
					if err != nil {
						t.Fatalf("failed to build ztoc: %v", err)
					}
					telemetry, checkCalled := newCalledTelemetry()

					// create a metadata reader
					r, err := newTestableReader(sr, ztoc.TOC, WithTelemetry(telemetry))
					if err != nil {
						t.Fatalf("failed to create new reader: %v", err)
					}
					defer r.Close()
					t.Logf("vvvvv Node tree vvvvv")
					t.Logf("[%d] ROOT", r.RootID())
					dumpNodes(t, r, r.RootID(), 1)
					t.Logf("^^^^^^^^^^^^^^^^^^^^^")
					for _, want := range tt.want {
						want(t, r)
					}
					if err := checkCalled(); err != nil {
						t.Errorf("telemetry failure: %v", err)
					}
				})
			}
		}
	}
}

func BenchmarkMetadataReader(b *testing.B) {
	testCases := []struct {
		name    string
		entries int
	}{
		{
			name:    "Create metadata.Reader with 1,000 TOC entries",
			entries: 1000,
		},
		{
			name:    "Create metadata.Reader with 10,000 TOC entries",
			entries: 10_000,
		},
		{
			name:    "Create metadata.Reader with 50,000 TOC entries",
			entries: 50_000,
		},
		{
			name:    "Create metadata.Reader with 100,000 TOC entries",
			entries: 100_000,
		},
	}
	cwdPath, err := os.Getwd()
	if err != nil {
		b.Fatal(err)
	}
	for _, tc := range testCases {
		toc, err := generateTOC(tc.entries)
		if err != nil {
			b.Fatalf("failed to generate TOC: %v", err)
		}
		b.ResetTimer()
		b.Run(tc.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				tempDB, clean, err := newTempDB(cwdPath)
				defer func() {
					b.StopTimer()
					clean()
					b.StartTimer()
				}()
				if err != nil {
					b.Fatalf("failed to initialize temp db: %v", err)
				}
				b.StartTimer()
				if _, err := NewReader(tempDB, nil, toc); err != nil {
					b.Fatalf("failed to create new reader: %v", err)
				}
			}

		})
	}
}

func BenchmarkConcurrentMetadataReader(b *testing.B) {
	smallTOC, err := generateTOC(1000)
	if err != nil {
		b.Fatalf("failed to generate TOC: %v", err)
	}
	mediumTOC, err := generateTOC(10_000)
	if err != nil {
		b.Fatalf("failed to generate TOC: %v", err)
	}
	largeTOC, err := generateTOC(50_000)
	if err != nil {
		b.Fatalf("failed to generate TOC: %v", err)
	}
	cwdPath, err := os.Getwd()
	if err != nil {
		b.Fatal(err)
	}
	tocs := []ztoc.TOC{smallTOC, mediumTOC, largeTOC}
	var eg errgroup.Group
	b.ResetTimer()
	b.Run("Write small, medium and large TOC concurrently", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			b.StopTimer()
			tempDB, clean, err := newTempDB(cwdPath)
			defer func() {
				b.StopTimer()
				clean()
				b.StartTimer()
			}()
			if err != nil {
				b.Fatalf("failed to initialize temp db: %v", err)
			}
			b.StartTimer()
			for _, toc := range tocs {
				toc := toc
				eg.Go(func() error {
					if _, err := NewReader(tempDB, nil, toc); err != nil {
						return fmt.Errorf("failed to create new reader: %v", err)
					}
					return nil
				})
			}
			if err := eg.Wait(); err != nil {
				b.Fatal(err)
			}
		}
	})
}
