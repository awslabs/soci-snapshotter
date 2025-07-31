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

/*
   Copyright 2019 The Go Authors. All rights reserved.
   Use of this source code is governed by a BSD-style
   license that can be found in the NOTICE.md file.
*/

package layer

import (
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/awslabs/soci-snapshotter/cache"
	"github.com/awslabs/soci-snapshotter/fs/reader"
	"github.com/awslabs/soci-snapshotter/fs/remote"
	spanmanager "github.com/awslabs/soci-snapshotter/fs/span-manager"
	"github.com/awslabs/soci-snapshotter/idtools"
	"github.com/awslabs/soci-snapshotter/metadata"
	"github.com/awslabs/soci-snapshotter/util/testutil"
	"github.com/awslabs/soci-snapshotter/ztoc"
	"github.com/containerd/containerd/reference"
	"github.com/containerd/containerd/remotes/docker"
	"github.com/google/go-cmp/cmp"
	fusefs "github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	digest "github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"golang.org/x/sys/unix"
)

const (
	sampleSpanSize     = 3
	sampleData1        = "0123456789"
	sampleMiddleOffset = sampleSpanSize / 2
	lastSpanOffset1    = sampleSpanSize * (int64(len(sampleData1)) / sampleSpanSize)
)

var testStateLayerDigest = digest.FromString("dummy")
var spanSizeCond = [3]int64{64, 128, 256}

func testNodeRead(t *testing.T, factory metadata.Store) {
	sizeCond := map[string]int64{
		"single_span": sampleSpanSize - sampleMiddleOffset,
		"multi_spans": sampleSpanSize + sampleMiddleOffset,
	}
	innerOffsetCond := map[string]int64{
		"at_top":    0,
		"at_middle": sampleMiddleOffset,
	}
	baseOffsetCond := map[string]int64{
		"of_1st_span":  sampleSpanSize * 0,
		"of_2nd_span":  sampleSpanSize * 1,
		"of_last_span": lastSpanOffset1,
	}
	fileSizeCond := map[string]int64{
		"in_1_span_file":   sampleSpanSize * 1,
		"in_2_span_file":   sampleSpanSize * 2,
		"in_max_size_file": int64(len(sampleData1)),
	}
	for sn, size := range sizeCond {
		for in, innero := range innerOffsetCond {
			for bo, baseo := range baseOffsetCond {
				for fn, filesize := range fileSizeCond {
					t.Run(fmt.Sprintf("reading_%s_%s_%s_%s", sn, in, bo, fn), func(t *testing.T) {
						if filesize > int64(len(sampleData1)) {
							t.Fatal("sample file size is larger than sample data")
						}

						wantN := size
						offset := baseo + innero

						if remain := filesize - offset; remain < wantN {
							if wantN = remain; wantN < 0 {
								wantN = 0
							}
						}

						// use constant string value as a data source.
						want := strings.NewReader(sampleData1)

						// data we want to get.
						wantData := make([]byte, wantN)
						_, err := want.ReadAt(wantData, offset)
						if err != nil && err != io.EOF {
							t.Fatalf("want.ReadAt (offset=%d,size=%d): %v", offset, wantN, err)
						}

						// data we get from the file node.
						f, closeFn := makeNodeReader(t, []byte(sampleData1)[:filesize], sampleSpanSize, factory)
						defer closeFn()
						tmpbuf := make([]byte, size) // fuse library can request bigger than remain
						rr, errno := f.Read(context.Background(), tmpbuf, offset)
						if errno != 0 {
							t.Errorf("failed to read off=%d, size=%d, filesize=%d: %v", offset, size, filesize, err)
							return
						}
						if rsize := rr.Size(); int64(rsize) != wantN {
							t.Errorf("read size: %d; want: %d; passed %d", rsize, wantN, size)
							return
						}
						tmpbuf = make([]byte, len(tmpbuf))
						respData, fs := rr.Bytes(tmpbuf)
						if fs != fuse.OK {
							t.Errorf("failed to read result data for off=%d, size=%d, filesize=%d: %v", offset, size, filesize, err)
						}

						if diff := cmp.Diff(wantData, respData); diff != "" {
							t.Errorf("off=%d, filesize=%d; read data and want data mismatch. diff=%+v", offset, filesize, diff)
							return
						}
					})
				}
			}
		}
	}
}

func makeNodeReader(t *testing.T, contents []byte, spanSize int64, factory metadata.Store) (_ *file, closeFn func() error) {
	testName := "test"
	tarEntry := []testutil.TarEntry{testutil.File(testName, string(contents))}
	ztoc, sr, err := ztoc.BuildZtocReader(t, tarEntry, gzip.DefaultCompression, spanSize)
	if err != nil {
		t.Fatalf("failed to build ztoc: %v", err)
	}
	mr, err := factory(sr, ztoc.TOC)
	if err != nil {
		t.Fatalf("failed to create reader: %v", err)
	}
	spanManager := spanmanager.New(ztoc, sr, cache.NewMemoryCache(), 0)
	r, err := reader.NewReader(mr, digest.FromString(""), spanManager, false)
	if err != nil {
		mr.Close()
		t.Fatalf("failed to make new reader: %v", err)
	}
	rootNode := getRootNode(t, r, OverlayOpaqueAll)
	var eo fuse.EntryOut
	inode, errno := rootNode.Lookup(context.Background(), testName, &eo)
	if errno != 0 {
		r.Close()
		t.Fatalf("failed to lookup test node; errno: %v", errno)
	}
	f, _, errno := inode.Operations().(fusefs.NodeOpener).Open(context.Background(), 0)
	if errno != 0 {
		r.Close()
		t.Fatalf("failed to open test file; errno: %v", errno)
	}
	return f.(*file), r.Close
}

func testExistence(t *testing.T, factory metadata.Store) {
	for _, o := range []OverlayOpaqueType{OverlayOpaqueAll, OverlayOpaqueTrusted, OverlayOpaqueUser} {
		testExistenceWithOpaque(t, factory, o)
	}
}

func testExistenceWithOpaque(t *testing.T, factory metadata.Store, opaque OverlayOpaqueType) {
	hasOpaque := func(entry string) check {
		return func(t *testing.T, root *node) {
			for _, k := range opaqueXattrs[opaque] {
				hasNodeXattrs(entry, k, opaqueXattrValue)(t, root)
			}
		}
	}
	tests := []struct {
		name string
		in   []testutil.TarEntry
		want []check
	}{
		{
			name: "1_whiteout_with_sibling",
			in: []testutil.TarEntry{
				testutil.Dir("foo/"),
				testutil.File("foo/bar.txt", ""),
				testutil.File("foo/.wh.foo.txt", ""),
			},
			want: []check{
				hasValidWhiteout("foo/foo.txt"),
				fileNotExist("foo/.wh.foo.txt"),
			},
		},
		{
			name: "1_whiteout_with_duplicated_name",
			in: []testutil.TarEntry{
				testutil.Dir("foo/"),
				testutil.File("foo/bar.txt", "test"),
				testutil.File("foo/.wh.bar.txt", ""),
			},
			want: []check{
				hasFileDigest("foo/bar.txt", digestFor("test")),
				fileNotExist("foo/.wh.bar.txt"),
			},
		},
		{
			name: "1_opaque",
			in: []testutil.TarEntry{
				testutil.Dir("foo/"),
				testutil.File("foo/.wh..wh..opq", ""),
			},
			want: []check{
				hasOpaque("foo/"),
				fileNotExist("foo/.wh..wh..opq"),
			},
		},
		{
			name: "1_opaque_with_sibling",
			in: []testutil.TarEntry{
				testutil.Dir("foo/"),
				testutil.File("foo/.wh..wh..opq", ""),
				testutil.File("foo/bar.txt", "test"),
			},
			want: []check{
				hasOpaque("foo/"),
				hasFileDigest("foo/bar.txt", digestFor("test")),
				fileNotExist("foo/.wh..wh..opq"),
			},
		},
		{
			name: "1_opaque_with_xattr",
			in: []testutil.TarEntry{
				testutil.Dir("foo/", testutil.WithDirXattrs(map[string]string{"foo": "bar"})),
				testutil.File("foo/.wh..wh..opq", ""),
			},
			want: []check{
				hasOpaque("foo/"),
				hasNodeXattrs("foo/", "foo", "bar"),
				fileNotExist("foo/.wh..wh..opq"),
			},
		},
		{
			name: "state_file",
			in: []testutil.TarEntry{
				testutil.File("test", "test"),
			},
			want: []check{
				hasFileDigest("test", digestFor("test")),
				hasStateFile(t, testStateLayerDigest.String()+".json"),
			},
		},
		{
			name: "file_suid",
			in: []testutil.TarEntry{
				testutil.File("test", "test", testutil.WithFileMode(0644|os.ModeSetuid)),
			},
			want: []check{
				hasExtraMode("test", os.ModeSetuid),
			},
		},
		{
			name: "dir_sgid",
			in: []testutil.TarEntry{
				testutil.Dir("test/", testutil.WithDirMode(0755|os.ModeSetgid)),
			},
			want: []check{
				hasExtraMode("test/", os.ModeSetgid),
			},
		},
		{
			name: "file_sticky",
			in: []testutil.TarEntry{
				testutil.File("test", "test", testutil.WithFileMode(0644|os.ModeSticky)),
			},
			want: []check{
				hasExtraMode("test", os.ModeSticky),
			},
		},
		{
			name: "symlink_size",
			in: []testutil.TarEntry{
				testutil.Symlink("test", "target"),
			},
			want: []check{
				hasSize("test", len("target")),
			},
		},
	}

	for _, tt := range tests {
		for _, spanSize := range spanSizeCond {
			t.Run(fmt.Sprintf("testExistence_%s_spansize_%d", tt.name, spanSize), func(t *testing.T) {
				ztoc, sr, err := ztoc.BuildZtocReader(t, tt.in, gzip.DefaultCompression, spanSize)
				if err != nil {
					t.Fatalf("failed to build sample ztoc: %v", err)
				}

				mr, err := factory(sr, ztoc.TOC)
				if err != nil {
					t.Fatalf("failed to create reader: %v", err)
				}
				defer mr.Close()
				spanManager := spanmanager.New(ztoc, sr, cache.NewMemoryCache(), 0)
				r, err := reader.NewReader(mr, digest.FromString(""), spanManager, false)
				if err != nil {
					t.Fatalf("failed to make new reader: %v", err)
				}
				defer r.Close()
				rootNode := getRootNode(t, r, opaque)
				for _, want := range tt.want {
					want(t, rootNode)
				}
			})
		}
	}
}

func hasSize(name string, size int) check {
	return func(t *testing.T, root *node) {
		_, n, err := getDirentAndNode(t, root, name)
		if err != nil {
			t.Fatalf("failed to get node %q: %v", name, err)
		}
		var ao fuse.AttrOut
		if errno := n.Operations().(fusefs.NodeGetattrer).Getattr(context.Background(), nil, &ao); errno != 0 {
			t.Fatalf("failed to get attributes of node %q: %v", name, errno)
		}
		if ao.Attr.Size != uint64(size) {
			t.Fatalf("got size = %d, want %d", ao.Attr.Size, size)
		}
	}
}

func getRootNode(t *testing.T, r reader.Reader, opaque OverlayOpaqueType) *node {
	rootNode, err := newNode(testStateLayerDigest, &testReader{r}, &testBlobState{10, 5}, 100, opaque, false, nil, idtools.IDMap{})
	if err != nil {
		t.Fatalf("failed to get root node: %v", err)
	}
	fusefs.NewNodeFS(rootNode, &fusefs.Options{}) // initializes root node
	return rootNode.(*node)
}

type testReader struct {
	r reader.Reader
}

func (tr *testReader) OpenFile(id uint32) (io.ReaderAt, error) { return tr.r.OpenFile(id) }
func (tr *testReader) Metadata() metadata.Reader               { return tr.r.Metadata() }
func (tr *testReader) Cache(opts ...reader.CacheOption) error  { return nil }
func (tr *testReader) Close() error                            { return nil }
func (tr *testReader) LastOnDemandReadTime() time.Time         { return time.Now() }

type testBlobState struct {
	size        int64
	fetchedSize int64
}

func (tb *testBlobState) Check() error       { return nil }
func (tb *testBlobState) Size() int64        { return tb.size }
func (tb *testBlobState) FetchedSize() int64 { return tb.fetchedSize }
func (tb *testBlobState) ReadAt(p []byte, offset int64, opts ...remote.Option) (int, error) {
	return 0, nil
}
func (tb *testBlobState) Cache(offset int64, size int64, opts ...remote.Option) error { return nil }
func (tb *testBlobState) Refresh(ctx context.Context, hosts []docker.RegistryHost, refspec reference.Spec, desc ocispec.Descriptor) error {
	return nil
}
func (tb *testBlobState) Close() error { return nil }

type check func(*testing.T, *node)

func fileNotExist(file string) check {
	return func(t *testing.T, root *node) {
		if _, _, err := getDirentAndNode(t, root, file); err == nil {
			t.Errorf("Node %q exists", file)
		}
	}
}

func hasFileDigest(filename string, digest string) check {
	return func(t *testing.T, root *node) {
		_, n, err := getDirentAndNode(t, root, filename)
		if err != nil {
			t.Fatalf("failed to get node %q: %v", filename, err)
		}
		ni := n.Operations().(*node)
		attr, err := ni.fs.r.Metadata().GetAttr(ni.id)
		if err != nil {
			t.Fatalf("failed to get attr %q(%d): %v", filename, ni.id, err)
		}
		fh, _, errno := ni.Open(context.Background(), 0)
		if errno != 0 {
			t.Fatalf("failed to open node %q: %v", filename, errno)
		}
		rr, errno := fh.(*file).Read(context.Background(), make([]byte, attr.Size), 0)
		if errno != 0 {
			t.Fatalf("failed to read node %q: %v", filename, errno)
		}
		res, status := rr.Bytes(make([]byte, attr.Size))
		if status != fuse.OK {
			t.Fatalf("failed to get read result of node %q: %v", filename, status)
		}
		if ndgst := digestFor(string(res)); ndgst != digest {
			t.Fatalf("Digest(%q) = %q, want %q", filename, ndgst, digest)
		}
	}
}

func hasExtraMode(name string, mode os.FileMode) check {
	return func(t *testing.T, root *node) {
		_, n, err := getDirentAndNode(t, root, name)
		if err != nil {
			t.Fatalf("failed to get node %q: %v", name, err)
		}
		var ao fuse.AttrOut
		if errno := n.Operations().(fusefs.NodeGetattrer).Getattr(context.Background(), nil, &ao); errno != 0 {
			t.Fatalf("failed to get attributes of node %q: %v", name, errno)
		}
		a := ao.Attr
		gotMode := a.Mode & (syscall.S_ISUID | syscall.S_ISGID | syscall.S_ISVTX)
		wantMode := extraModeToTarMode(mode)
		if gotMode != uint32(wantMode) {
			t.Fatalf("got mode = %b, want %b", gotMode, wantMode)
		}
	}
}

func hasValidWhiteout(name string) check {
	return func(t *testing.T, root *node) {
		ent, n, err := getDirentAndNode(t, root, name)
		if err != nil {
			t.Fatalf("failed to get node %q: %v", name, err)
		}
		var ao fuse.AttrOut
		if errno := n.Operations().(fusefs.NodeGetattrer).Getattr(context.Background(), nil, &ao); errno != 0 {
			t.Fatalf("failed to get attributes of file %q: %v", name, errno)
		}
		a := ao.Attr
		if a.Ino != ent.Ino {
			t.Errorf("inconsistent inodes %d(Node) != %d(Dirent)", a.Ino, ent.Ino)
			return
		}

		// validate the direntry
		if ent.Mode != syscall.S_IFCHR {
			t.Errorf("whiteout entry %q isn't a char device", name)
			return
		}

		// validate the node
		if a.Mode != syscall.S_IFCHR {
			t.Errorf("whiteout %q has an invalid mode %o; want %o",
				name, a.Mode, syscall.S_IFCHR)
			return
		}
		if a.Rdev != uint32(unix.Mkdev(0, 0)) {
			t.Errorf("whiteout %q has invalid device numbers (%d, %d); want (0, 0)",
				name, unix.Major(uint64(a.Rdev)), unix.Minor(uint64(a.Rdev)))
			return
		}
	}
}

func hasNodeXattrs(entry, name, value string) check {
	return func(t *testing.T, root *node) {
		_, n, err := getDirentAndNode(t, root, entry)
		if err != nil {
			t.Fatalf("failed to get node %q: %v", entry, err)
		}

		// check xattr exists in the xattrs list.
		buf := make([]byte, 1000)
		nb, errno := n.Operations().(fusefs.NodeListxattrer).Listxattr(context.Background(), buf)
		if errno != 0 {
			t.Fatalf("failed to get xattrs list of node %q: %v", entry, err)
		}
		attrs := strings.Split(string(buf[:nb]), "\x00")
		var found bool
		for _, x := range attrs {
			if x == name {
				found = true
			}
		}
		if !found {
			t.Errorf("node %q doesn't have an opaque xattr %q", entry, value)
			return
		}

		// check the xattr has valid value.
		v := make([]byte, len(value))
		nv, errno := n.Operations().(fusefs.NodeGetxattrer).Getxattr(context.Background(), name, v)
		if errno != 0 {
			t.Fatalf("failed to get xattr %q of node %q: %v", name, entry, err)
		}
		if int(nv) != len(value) {
			t.Fatalf("invalid xattr size for file %q, value %q got %d; want %d",
				name, value, nv, len(value))
		}
		if string(v) != value {
			t.Errorf("node %q has an invalid xattr %q; want %q", entry, v, value)
			return
		}
	}
}

func hasEntry(t *testing.T, name string, ents fusefs.DirStream) (fuse.DirEntry, bool) {
	for ents.HasNext() {
		de, errno := ents.Next()
		if errno != 0 {
			t.Fatalf("faield to read entries for %q", name)
		}
		if de.Name == name {
			return de, true
		}
	}
	return fuse.DirEntry{}, false
}

func hasStateFile(t *testing.T, id string) check {
	return func(t *testing.T, root *node) {

		// Check the state dir is hidden on OpenDir for "/"
		ents, errno := root.Readdir(context.Background())
		if errno != 0 {
			t.Errorf("failed to open root directory: %v", errno)
			return
		}
		if _, ok := hasEntry(t, stateDirName, ents); ok {
			t.Errorf("state direntry %q should not be listed", stateDirName)
			return
		}

		// Check existence of state dir
		var eo fuse.EntryOut
		sti, errno := root.Lookup(context.Background(), stateDirName, &eo)
		if errno != 0 {
			t.Errorf("failed to lookup directory %q: %v", stateDirName, errno)
			return
		}
		st, ok := sti.Operations().(*state)
		if !ok {
			t.Errorf("directory %q isn't a state node", stateDirName)
			return
		}

		// Check existence of state file
		ents, errno = st.Readdir(context.Background())
		if errno != 0 {
			t.Errorf("failed to open directory %q: %v", stateDirName, errno)
			return
		}
		if _, ok := hasEntry(t, id, ents); !ok {
			t.Errorf("direntry %q not found in %q", id, stateDirName)
			return
		}
		inode, errno := st.Lookup(context.Background(), id, &eo)
		if errno != 0 {
			t.Errorf("failed to lookup node %q in %q: %v", id, stateDirName, errno)
			return
		}
		n, ok := inode.Operations().(*statFile)
		if !ok {
			t.Errorf("entry %q isn't a normal node", id)
			return
		}

		// wanted data
		r := testutil.NewTestRand(t)
		wantErr := fmt.Errorf("test-%d", r.Int64())

		// report the data
		root.fs.s.report(wantErr)

		// obtain file size (check later)
		var ao fuse.AttrOut
		errno = n.Operations().(fusefs.NodeGetattrer).Getattr(context.Background(), nil, &ao)
		if errno != 0 {
			t.Errorf("failed to get attr of state file: %v", errno)
			return
		}
		attr := ao.Attr

		// get data via state file
		tmp := make([]byte, 4096)
		res, errno := n.Read(context.Background(), nil, tmp, 0)
		if errno != 0 {
			t.Errorf("failed to read state file: %v", errno)
			return
		}
		gotState, status := res.Bytes(nil)
		if status != fuse.OK {
			t.Errorf("failed to get result bytes of state file: %v", errno)
			return
		}
		if attr.Size != uint64(len(string(gotState))) {
			t.Errorf("size %d; want %d", attr.Size, len(string(gotState)))
			return
		}

		var j statJSON
		if err := json.Unmarshal(gotState, &j); err != nil {
			t.Errorf("failed to unmarshal %q: %v", string(gotState), err)
			return
		}
		if wantErr.Error() != j.Error {
			t.Errorf("expected error %q, got %q", wantErr.Error(), j.Error)
			return
		}
	}
}

// getDirentAndNode gets dirent and node at the specified path at once and makes
// sure that the both of them exist.
func getDirentAndNode(t *testing.T, root *node, path string) (ent fuse.DirEntry, n *fusefs.Inode, err error) {
	dir, base := filepath.Split(filepath.Clean(path))

	// get the target's parent directory.
	var eo fuse.EntryOut
	d := root
	for _, name := range strings.Split(dir, "/") {
		if len(name) == 0 {
			continue
		}
		di, errno := d.Lookup(context.Background(), name, &eo)
		if errno != 0 {
			err = fmt.Errorf("failed to lookup directory %q: %v", name, errno)
			return
		}
		var ok bool
		if d, ok = di.Operations().(*node); !ok {
			err = fmt.Errorf("directory %q isn't a normal node", name)
			return
		}

	}

	// get the target's direntry.
	ents, errno := d.Readdir(context.Background())
	if errno != 0 {
		err = fmt.Errorf("failed to open directory %q: %v", path, errno)
	}
	ent, ok := hasEntry(t, base, ents)
	if !ok {
		err = fmt.Errorf("direntry %q not found in the parent directory of %q", base, path)
	}

	// get the target's node.
	n, errno = d.Lookup(context.Background(), base, &eo)
	if errno != 0 {
		err = fmt.Errorf("failed to lookup node %q: %v", path, errno)
	}

	return
}

func digestFor(content string) string {
	sum := sha256.Sum256([]byte(content))
	return fmt.Sprintf("sha256:%x", sum)
}

// suid, guid, sticky bits for archive/tar
// https://github.com/golang/go/blob/release-branch.go1.13/src/archive/tar/common.go#L607-L609
const (
	cISUID = 04000 // Set uid
	cISGID = 02000 // Set gid
	cISVTX = 01000 // Save text (sticky bit)
)

func extraModeToTarMode(fm os.FileMode) (tm int64) {
	if fm&os.ModeSetuid != 0 {
		tm |= cISUID
	}
	if fm&os.ModeSetgid != 0 {
		tm |= cISGID
	}
	if fm&os.ModeSticky != 0 {
		tm |= cISVTX
	}
	return
}
