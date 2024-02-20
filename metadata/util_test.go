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
	"go.etcd.io/bbolt"
)

type closeDB func() error

type testableReader interface {
	Reader
	NumOfNodes() (i int, _ error)
}

// newTestableReader creates a test bbolt db as-well as a metadata.Reader given a TOC.
func newTestableReader(sr *io.SectionReader, toc ztoc.TOC, opts ...Option) (testableReader, error) {
	db, clean, err := newTempDB("")
	if err != nil {
		clean()
		return nil, err
	}
	r, err := NewReader(db, sr, toc, opts...)
	if err != nil {
		return nil, err
	}
	return &testableReadCloser{
		testableReader: r.(*reader),
		closeFn:        clean,
	}, nil
}

type testableReadCloser struct {
	testableReader
	closeFn closeDB
}

func (r *testableReadCloser) Close() error {
	r.closeFn()
	return r.testableReader.Close()
}

// newTempDB creates a test bbolt db.
func newTempDB(d string) (*bbolt.DB, func() error, error) {
	f, err := os.CreateTemp(d, "readertestdb")
	if err != nil {
		return nil, func() error { return nil }, err
	}
	db, err := bbolt.Open(f.Name(), 0600, nil)
	if err != nil {
		return nil, func() error {
			return os.Remove(f.Name())
		}, err
	}
	return db, func() error {
		err := db.Close()
		if err != nil {
			return err
		}
		return os.Remove(f.Name())
	}, err
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

func hasChardev(name string, maj, min int) check {
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
		if attr.DevMajor != maj || attr.DevMinor != min {
			t.Errorf("unexpected major/minor of chardev %q: %d/%d want %d/%d", name, attr.DevMajor, attr.DevMinor, maj, min)
			return
		}
	}
}

func hasBlockdev(name string, maj, min int) check {
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
		if attr.DevMajor != maj || attr.DevMinor != min {
			t.Errorf("unexpected major/minor of blockdev %q: %d/%d want %d/%d", name, attr.DevMajor, attr.DevMinor, maj, min)
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

// generateTOC generates a random TOC with a given amount of entries.
func generateTOC(numEntries int, opts ...testutil.TarEntriesOption) (ztoc.TOC, error) {
	tarEntries, err := testutil.GenerateTarEntries(numEntries, opts...)
	if err != nil {
		return ztoc.TOC{}, err
	}
	ztoc, _, err := ztoc.BuildZtocReader(nil, tarEntries, gzip.DefaultCompression, 64)
	return ztoc.TOC, err
}
