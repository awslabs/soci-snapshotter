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
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/awslabs/soci-snapshotter/util/testutil"
	"github.com/awslabs/soci-snapshotter/ztoc"
)

var allowedPrefix = [4]string{"", "./", "/", "../"}

var srcCompressions = map[string]int{
	"gzip-nocompression":      gzip.NoCompression,
	"gzip-bestspeed":          gzip.BestSpeed,
	"gzip-bestcompression":    gzip.BestCompression,
	"gzip-defaultcompression": gzip.DefaultCompression,
	"gzip-huffmanonly":        gzip.HuffmanOnly,
}

type readerFactory func(sr *io.SectionReader, toc ztoc.TOC, opts ...Option) (r testableReader, err error)

type testableReader interface {
	Reader
	NumOfNodes() (i int, _ error)
}

// testReader tests Reader returns correct file metadata.
func testReader(t *testing.T, factory readerFactory) {
	sampleTime := time.Now().Truncate(time.Second)
	tests := []struct {
		name string
		in   []testutil.TarEntry
		want []check
	}{
		{
			name: "files",
			in: []testutil.TarEntry{
				testutil.File("foo", "foofoo", testutil.WithFileMode(0644|os.ModeSetuid)),
				testutil.Dir("bar/"),
				testutil.File("bar/baz.txt", "bazbazbaz", testutil.WithFileOwner(1000, 1000)),
				testutil.File("xxx.txt", "xxxxx", testutil.WithFileModTime(sampleTime)),
				testutil.File("y.txt", "", testutil.WithFileXattrs(map[string]string{"testkey": "testval"})),
			},
			want: []check{
				numOfNodes(6), // root dir + 1 dir + 4 files
				hasFile("foo", 6),
				hasMode("foo", 0644|os.ModeSetuid),
				hasFile("bar/baz.txt", 9),
				hasOwner("bar/baz.txt", 1000, 1000),
				hasFile("xxx.txt", 5),
				hasModTime("xxx.txt", sampleTime),
				hasFile("y.txt", 0),
				// For details on the keys of Xattrs, see https://pkg.go.dev/archive/tar#Header
				hasXattrs("y.txt", map[string]string{"testkey": "testval"}),
			},
		},
		{
			name: "dirs",
			in: []testutil.TarEntry{
				testutil.Dir("foo/", testutil.WithDirMode(os.ModeDir|0600|os.ModeSticky)),
				testutil.Dir("foo/bar/", testutil.WithDirOwner(1000, 1000)),
				testutil.File("foo/bar/baz.txt", "testtest"),
				testutil.File("foo/bar/xxxx", "x"),
				testutil.File("foo/bar/yyy", "yyy"),
				testutil.Dir("foo/a/", testutil.WithDirModTime(sampleTime)),
				testutil.Dir("foo/a/1/", testutil.WithDirXattrs(map[string]string{"testkey": "testval"})),
				testutil.File("foo/a/1/2", "1111111111"),
			},
			want: []check{
				numOfNodes(9), // root dir + 4 dirs + 4 files
				hasDirChildren("foo", "bar", "a"),
				hasDirChildren("foo/bar", "baz.txt", "xxxx", "yyy"),
				hasDirChildren("foo/a", "1"),
				hasDirChildren("foo/a/1", "2"),
				hasMode("foo", os.ModeDir|0600|os.ModeSticky),
				hasOwner("foo/bar", 1000, 1000),
				hasModTime("foo/a", sampleTime),
				hasXattrs("foo/a/1", map[string]string{"testkey": "testval"}),
				hasFile("foo/bar/baz.txt", 8),
				hasFile("foo/bar/xxxx", 1),
				hasFile("foo/bar/yyy", 3),
				hasFile("foo/a/1/2", 10),
			},
		},
		{
			name: "hardlinks",
			in: []testutil.TarEntry{
				testutil.File("foo", "foofoo", testutil.WithFileOwner(1000, 1000)),
				testutil.Dir("bar/"),
				testutil.Link("bar/foolink", "foo"),
				testutil.Link("bar/foolink2", "bar/foolink"),
				testutil.Dir("bar/1/"),
				testutil.File("bar/1/baz.txt", "testtest"),
				testutil.Link("barlink", "bar/1/baz.txt"),
				testutil.Symlink("foosym", "bar/foolink2"),
			},
			want: []check{
				numOfNodes(6), // root dir + 2 dirs + 1 flie(linked) + 1 file(linked) + 1 symlink
				hasFile("foo", 6),
				hasOwner("foo", 1000, 1000),
				hasFile("bar/foolink", 6),
				hasOwner("bar/foolink", 1000, 1000),
				hasFile("bar/foolink2", 6),
				hasOwner("bar/foolink2", 1000, 1000),
				hasFile("bar/1/baz.txt", 8),
				hasFile("barlink", 8),
				hasDirChildren("bar", "foolink", "foolink2", "1"),
				hasDirChildren("bar/1", "baz.txt"),
				sameNodes("foo", "bar/foolink", "bar/foolink2"),
				sameNodes("bar/1/baz.txt", "barlink"),
				linkName("foosym", "bar/foolink2"),
				hasNumLink("foo", 3),     // parent dir + 2 links
				hasNumLink("barlink", 2), // parent dir + 1 link
				hasNumLink("bar", 3),     // parent + "." + child's ".."
			},
		},
		{
			name: "various files",
			in: []testutil.TarEntry{
				testutil.Dir("bar/"),
				testutil.File("bar/../bar///////////////////foo", ""),
				testutil.Chardev("bar/cdev", 10, 11),
				testutil.Blockdev("bar/bdev", 100, 101),
				testutil.Fifo("bar/fifo"),
			},
			want: []check{
				numOfNodes(6), // root dir + 1 file + 1 dir + 1 cdev + 1 bdev + 1 fifo
				hasFile("bar/foo", 0),
				hasChardev("bar/cdev", 10, 11),
				hasBlockdev("bar/bdev", 100, 101),
				hasFifo("bar/fifo"),
			},
		},
	}
	for _, tt := range tests {
		for _, prefix := range allowedPrefix {
			for srcCompresionName, srcCompression := range srcCompressions {
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
					r, err := factory(sr, ztoc.TOC, WithTelemetry(telemetry))
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

func newCalledTelemetry() (telemetry *Telemetry, check func() error) {
	var initMetadataStoreLatencyCalled bool
	return &Telemetry{
			func(time.Time) { initMetadataStoreLatencyCalled = true },
		}, func() error {
			var allErr error
			if !initMetadataStoreLatencyCalled {
				allErr = errors.Join(allErr, fmt.Errorf("metrics initMetadataStoreLatency isn't called"))
			}
			return allErr
		}
}

func dumpNodes(t *testing.T, r testableReader, id uint32, level int) {
	if err := r.ForeachChild(id, func(name string, id uint32, mode os.FileMode) bool {
		ind := ""
		for i := 0; i < level; i++ {
			ind += " "
		}
		t.Logf("%v+- [%d] %q : %v", ind, id, name, mode)
		dumpNodes(t, r, id, level+1)
		return true
	}); err != nil {
		t.Errorf("failed to dump nodes %v", err)
	}
}

type check func(*testing.T, testableReader)

func numOfNodes(want int) check {
	return func(t *testing.T, r testableReader) {
		i, err := r.NumOfNodes()
		if err != nil {
			t.Errorf("num of nodes: %v", err)
		}
		if want != i {
			t.Errorf("unexpected num of nodes %d; want %d", i, want)
		}
	}
}

func sameNodes(n string, nodes ...string) check {
	return func(t *testing.T, r testableReader) {
		id, err := lookup(r, n)
		if err != nil {
			t.Errorf("failed to lookup %q: %v", n, err)
			return
		}
		for _, en := range nodes {
			eid, err := lookup(r, en)
			if err != nil {
				t.Errorf("failed to lookup %q: %v", en, err)
				return
			}
			if eid != id {
				t.Errorf("unexpected ID of %q: %d want %d", en, eid, id)
			}
		}
	}
}

func linkName(name string, linkName string) check {
	return func(t *testing.T, r testableReader) {
		id, err := lookup(r, name)
		if err != nil {
			t.Errorf("failed to lookup %q: %v", name, err)
			return
		}
		attr, err := r.GetAttr(id)
		if err != nil {
			t.Errorf("failed to get attr of %q: %v", name, err)
			return
		}
		if attr.Mode&os.ModeSymlink == 0 {
			t.Errorf("%q is not a symlink: %v", name, attr.Mode)
			return
		}
		if attr.LinkName != linkName {
			t.Errorf("unexpected link name of %q : %q want %q", name, attr.LinkName, linkName)
			return
		}
	}
}

func hasNumLink(name string, numLink int) check {
	return func(t *testing.T, r testableReader) {
		id, err := lookup(r, name)
		if err != nil {
			t.Errorf("failed to lookup %q: %v", name, err)
			return
		}
		attr, err := r.GetAttr(id)
		if err != nil {
			t.Errorf("failed to get attr of %q: %v", name, err)
			return
		}
		if attr.NumLink != numLink {
			t.Errorf("unexpected numLink of %q: %d want %d", name, attr.NumLink, numLink)
			return
		}
	}
}

func hasDirChildren(name string, children ...string) check {
	return func(t *testing.T, r testableReader) {
		id, err := lookup(r, name)
		if err != nil {
			t.Errorf("failed to lookup %q: %v", name, err)
			return
		}
		attr, err := r.GetAttr(id)
		if err != nil {
			t.Errorf("failed to get attr of %q: %v", name, err)
			return
		}
		if !attr.Mode.IsDir() {
			t.Errorf("%q is not directory: %v", name, attr.Mode)
			return
		}
		found := map[string]struct{}{}
		if err := r.ForeachChild(id, func(name string, id uint32, mode os.FileMode) bool {
			found[name] = struct{}{}
			return true
		}); err != nil {
			t.Errorf("failed to see children %v", err)
			return
		}
		if len(found) != len(children) {
			t.Errorf("unexpected number of children of %q : %d want %d", name, len(found), len(children))
		}
		for _, want := range children {
			if _, ok := found[want]; !ok {
				t.Errorf("expected child %q not found in %q", want, name)
			}
		}
	}
}

func hasChardev(name string, major, minor int) check {
	return func(t *testing.T, r testableReader) {
		id, err := lookup(r, name)
		if err != nil {
			t.Errorf("cannot find chardev %q: %v", name, err)
			return
		}
		attr, err := r.GetAttr(id)
		if err != nil {
			t.Errorf("cannot get attr of chardev %q: %v", name, err)
			return
		}
		if attr.Mode&os.ModeDevice == 0 || attr.Mode&os.ModeCharDevice == 0 {
			t.Errorf("file %q is not a chardev: %v", name, attr.Mode)
			return
		}
		if attr.DevMajor != major || attr.DevMinor != minor {
			t.Errorf("unexpected major/minor of chardev %q: %d/%d want %d/%d", name, attr.DevMajor, attr.DevMinor, major, minor)
			return
		}
	}
}

func hasBlockdev(name string, major, minor int) check {
	return func(t *testing.T, r testableReader) {
		id, err := lookup(r, name)
		if err != nil {
			t.Errorf("cannot find blockdev %q: %v", name, err)
			return
		}
		attr, err := r.GetAttr(id)
		if err != nil {
			t.Errorf("cannot get attr of blockdev %q: %v", name, err)
			return
		}
		if attr.Mode&os.ModeDevice == 0 || attr.Mode&os.ModeCharDevice != 0 {
			t.Errorf("file %q is not a blockdev: %v", name, attr.Mode)
			return
		}
		if attr.DevMajor != major || attr.DevMinor != minor {
			t.Errorf("unexpected major/minor of blockdev %q: %d/%d want %d/%d", name, attr.DevMajor, attr.DevMinor, major, minor)
			return
		}
	}
}

func hasFifo(name string) check {
	return func(t *testing.T, r testableReader) {
		id, err := lookup(r, name)
		if err != nil {
			t.Errorf("cannot find blockdev %q: %v", name, err)
			return
		}
		attr, err := r.GetAttr(id)
		if err != nil {
			t.Errorf("cannot get attr of blockdev %q: %v", name, err)
			return
		}
		if attr.Mode&os.ModeNamedPipe == 0 {
			t.Errorf("file %q is not a fifo: %v", name, attr.Mode)
			return
		}
	}
}

func hasFile(name string, size int64) check {
	return func(t *testing.T, r testableReader) {
		id, err := lookup(r, name)
		if err != nil {
			t.Errorf("cannot find file %q: %v", name, err)
			return
		}
		attr, err := r.GetAttr(id)
		if err != nil {
			t.Errorf("cannot get attr of file %q: %v", name, err)
			return
		}
		if !attr.Mode.IsRegular() {
			t.Errorf("file %q is not a regular file: %v", name, attr.Mode)
			return
		}
		f, err := r.OpenFile(id)
		if err != nil {
			t.Errorf("cannot open file %q: %v", name, err)
			return
		}
		if attr.Size != size {
			t.Errorf("unexpected size of file %q : %d want %d", name, attr.Size, size)
			return
		}
		if size != int64(f.GetUncompressedFileSize()) {
			t.Errorf("unexpected uncompressed file size of %q: %d want %d", name, f.GetUncompressedFileSize(), size)
			return
		}
	}
}

func hasMode(name string, mode os.FileMode) check {
	return func(t *testing.T, r testableReader) {
		id, err := lookup(r, name)
		if err != nil {
			t.Errorf("cannot find file %q: %v", name, err)
			return
		}
		attr, err := r.GetAttr(id)
		if err != nil {
			t.Errorf("cannot get attr of file %q: %v", name, err)
			return
		}
		if attr.Mode != mode {
			t.Errorf("unexpected mode of %q: %v want %v", name, attr.Mode, mode)
			return
		}
	}
}

func hasOwner(name string, uid, gid int) check {
	return func(t *testing.T, r testableReader) {
		id, err := lookup(r, name)
		if err != nil {
			t.Errorf("cannot find file %q: %v", name, err)
			return
		}
		attr, err := r.GetAttr(id)
		if err != nil {
			t.Errorf("cannot get attr of file %q: %v", name, err)
			return
		}
		if attr.UID != uid || attr.GID != gid {
			t.Errorf("unexpected owner of %q: (%d:%d) want (%d:%d)", name, attr.UID, attr.GID, uid, gid)
			return
		}
	}
}

func hasModTime(name string, modTime time.Time) check {
	return func(t *testing.T, r testableReader) {
		id, err := lookup(r, name)
		if err != nil {
			t.Errorf("cannot find file %q: %v", name, err)
			return
		}
		attr, err := r.GetAttr(id)
		if err != nil {
			t.Errorf("cannot get attr of file %q: %v", name, err)
			return
		}
		attrModTime := attr.ModTime
		if attrModTime.Before(modTime) || attrModTime.After(modTime) {
			t.Errorf("unexpected time of %q: %v; want %v", name, attrModTime, modTime)
			return
		}
	}
}

func hasXattrs(name string, xattrs map[string]string) check {
	return func(t *testing.T, r testableReader) {
		id, err := lookup(r, name)
		if err != nil {
			t.Errorf("cannot find file %q: %v", name, err)
			return
		}
		attr, err := r.GetAttr(id)
		if err != nil {
			t.Errorf("cannot get attr of file %q: %v", name, err)
			return
		}
		if len(attr.Xattrs) != len(xattrs) {
			t.Errorf("unexpected size of xattr of %q: %d want %d", name, len(attr.Xattrs), len(xattrs))
			return
		}
		for k, v := range attr.Xattrs {
			if xattrs[k] != string(v) {
				t.Errorf("unexpected xattr of %q: %q=%q want %q=%q", name, k, string(v), k, xattrs[k])
			}
		}
	}
}

func lookup(r testableReader, name string) (uint32, error) {
	name = strings.TrimPrefix(path.Clean("/"+name), "/")
	if name == "" {
		return r.RootID(), nil
	}
	dir, base := filepath.Split(name)
	pid, err := lookup(r, dir)
	if err != nil {
		return 0, err
	}
	id, _, err := r.GetChild(pid, base)
	return id, err
}
