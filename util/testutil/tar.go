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

package testutil

// This utility helps test codes to generate sample tar blobs.

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/klauspost/compress/zstd"
)

// TarEntry is an entry of tar.
type TarEntry interface {
	AppendTar(tw *tar.Writer, opts BuildTarOptions) error
}

// BuildTarOptions is a set of options used during building blob.
type BuildTarOptions struct {

	// Prefix is the prefix string need to be added to each file name (e.g. "./", "/", etc.)
	Prefix string

	GzipComment  string
	GzipFilename string
	GzipExtra    []byte
}

// BuildTarOption is an option used during building blob.
type BuildTarOption func(o *BuildTarOptions)

// WithPrefix is an option to add a prefix string to each file name (e.g. "./", "/", etc.)
func WithPrefix(prefix string) BuildTarOption {
	return func(o *BuildTarOptions) {
		o.Prefix = prefix
	}
}

func WithGzipComment(comment string) BuildTarOption {
	return func(o *BuildTarOptions) {
		o.GzipComment = comment
	}
}

func WithGzipFilename(filename string) BuildTarOption {
	return func(o *BuildTarOptions) {
		o.GzipFilename = filename
	}
}

func WithGzipExtra(extra []byte) BuildTarOption {
	return func(o *BuildTarOptions) {
		o.GzipExtra = extra
	}
}

// BuildTar builds a tar given a list of tar entries and returns an io.Reader
func BuildTar(ents []TarEntry, opts ...BuildTarOption) io.Reader {
	var bo BuildTarOptions
	for _, o := range opts {
		o(&bo)
	}
	pr, pw := io.Pipe()
	go func() {
		tw := tar.NewWriter(pw)
		for _, ent := range ents {
			if err := ent.AppendTar(tw, bo); err != nil {
				pw.CloseWithError(err)
				return
			}
		}
		if err := tw.Close(); err != nil {
			pw.CloseWithError(err)
			return
		}
		pw.Close()
	}()
	return pr
}

// BuildTarGz builds a tar.gz given a list of tar entries and returns an io.Reader
func BuildTarGz(ents []TarEntry, compressionLevel int, opts ...BuildTarOption) io.Reader {
	var bo BuildTarOptions
	for _, o := range opts {
		o(&bo)
	}
	pr, pw := io.Pipe()
	go func() {
		gw, err := gzip.NewWriterLevel(pw, compressionLevel)
		if err != nil {
			pw.CloseWithError(err)
			return
		}
		gw.Comment = bo.GzipComment
		gw.Name = bo.GzipFilename
		gw.Extra = bo.GzipExtra
		tw := tar.NewWriter(gw)

		for _, ent := range ents {
			if err := ent.AppendTar(tw, bo); err != nil {
				pw.CloseWithError(err)
				return
			}
		}

		if err := tw.Close(); err != nil {
			pw.CloseWithError(err)
			return
		}
		if err := gw.Close(); err != nil {
			pw.CloseWithError(err)
			return
		}
		pw.Close()
	}()
	return pr
}

// BuildTarZstd builds a tar blob with zstd compression.
func BuildTarZstd(ents []TarEntry, compressionLevel int, opts ...BuildTarOption) io.Reader {
	var bo BuildTarOptions
	for _, o := range opts {
		o(&bo)
	}
	pr, pw := io.Pipe()
	go func() {
		zw, err := zstd.NewWriter(pw, zstd.WithEncoderLevel(zstd.EncoderLevelFromZstd(compressionLevel)))
		if err != nil {
			pw.CloseWithError(err)
			return
		}
		tw := tar.NewWriter(zw)
		for _, ent := range ents {
			if err := ent.AppendTar(tw, bo); err != nil {
				pw.CloseWithError(err)
				return
			}
		}
		if err := tw.Close(); err != nil {
			pw.CloseWithError(err)
			return
		}
		if err := zw.Close(); err != nil {
			pw.CloseWithError(err)
			return
		}
		pw.Close()
	}()
	return pr
}

// WriteTarToTempFile writes the contents of a tar archive to a specified path and
// return the temp filename and the tar data (as []byte).
//
// It's the caller's responsibility to remove the genreated temp file.
func WriteTarToTempFile(tarNamePattern string, tarReader io.Reader) (string, []byte, error) {
	tarFile, err := os.CreateTemp("", tarNamePattern)
	if err != nil {
		return "", nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	defer tarFile.Close()

	tarBuf := new(bytes.Buffer)
	w := io.MultiWriter(tarFile, tarBuf)
	_, err = io.Copy(w, tarReader)
	if err != nil {
		return "", nil, fmt.Errorf("failed to write tar file: %w", err)
	}

	return tarFile.Name(), tarBuf.Bytes(), nil
}

// GetFilesAndContentsWithinTarGz takes a path to a targz archive and returns a list of its files and their contents
func GetFilesAndContentsWithinTarGz(tarGz string) (map[string][]byte, []string, error) {
	f, err := os.Open(tarGz)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	gr, err := gzip.NewReader(f)
	if err != nil {
		return nil, nil, err
	}
	tr := tar.NewReader(gr)
	return getFilesAndContentsFromTarReader(tr)
}

// GetFilesAndContentsWithinTar takes a path to a tar archive and returns a list of its files and their contents
func GetFilesAndContentsWithinTar(tarFile string) (map[string][]byte, []string, error) {
	f, err := os.Open(tarFile)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	tr := tar.NewReader(f)
	return getFilesAndContentsFromTarReader(tr)
}

func getFilesAndContentsFromTarReader(tr *tar.Reader) (map[string][]byte, []string, error) {
	m := make(map[string][]byte)
	var files []string

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if header.Typeflag == tar.TypeReg {
			files = append(files, header.Name)
			contents, err := io.ReadAll(tr)
			if err != nil {
				return nil, nil, err
			}
			m[header.Name] = contents
		}

	}
	return m, files, nil
}

type tarEntryFunc func(*tar.Writer, BuildTarOptions) error

// AppendTar appends a file to a tar writer
func (f tarEntryFunc) AppendTar(tw *tar.Writer, opts BuildTarOptions) error { return f(tw, opts) }

// DirectoryBuildTarOption is an option for a directory entry.
type DirectoryBuildTarOption func(o *dirOpts)

type dirOpts struct {
	uid     int
	gid     int
	xattrs  map[string]string
	mode    *os.FileMode
	modTime time.Time
}

// WithDirModTime specifies the modtime of the dir.
func WithDirModTime(modTime time.Time) DirectoryBuildTarOption {
	return func(o *dirOpts) {
		o.modTime = modTime
	}
}

// WithDirOwner specifies the owner of the directory.
func WithDirOwner(uid, gid int) DirectoryBuildTarOption {
	return func(o *dirOpts) {
		o.uid = uid
		o.gid = gid
	}
}

// WithDirXattrs specifies the extended attributes of the directory.
func WithDirXattrs(xattrs map[string]string) DirectoryBuildTarOption {
	return func(o *dirOpts) {
		o.xattrs = xattrs
	}
}

// WithDirMode specifies the mode of the directory.
func WithDirMode(mode os.FileMode) DirectoryBuildTarOption {
	return func(o *dirOpts) {
		o.mode = &mode
	}
}

// Dir is a directory entry
func Dir(name string, opts ...DirectoryBuildTarOption) TarEntry {
	return tarEntryFunc(func(tw *tar.Writer, buildOpts BuildTarOptions) error {
		var dOpts dirOpts
		for _, o := range opts {
			o(&dOpts)
		}
		if !strings.HasSuffix(name, "/") {
			panic(fmt.Sprintf("missing trailing slash in dir %q ", name))
		}
		var mode int64 = 0755
		if dOpts.mode != nil {
			mode = permAndExtraMode2TarMode(*dOpts.mode)
		}
		return tw.WriteHeader(&tar.Header{
			Typeflag: tar.TypeDir,
			Name:     buildOpts.Prefix + name,
			Mode:     mode,
			ModTime:  dOpts.modTime,
			Xattrs:   dOpts.xattrs,
			Uid:      dOpts.uid,
			Gid:      dOpts.gid,
		})
	})
}

// FileBuildTarOption is an option for a file entry.
type FileBuildTarOption func(o *fileOpts)

type fileOpts struct {
	uid     int
	gid     int
	xattrs  map[string]string
	mode    *os.FileMode
	modTime time.Time
}

// WithFileOwner specifies the owner of the file.
func WithFileOwner(uid, gid int) FileBuildTarOption {
	return func(o *fileOpts) {
		o.uid = uid
		o.gid = gid
	}
}

// WithFileXattrs specifies the extended attributes of the file.
func WithFileXattrs(xattrs map[string]string) FileBuildTarOption {
	return func(o *fileOpts) {
		o.xattrs = xattrs
	}
}

// WithFileModTime specifies the modtime of the file.
func WithFileModTime(modTime time.Time) FileBuildTarOption {
	return func(o *fileOpts) {
		o.modTime = modTime
	}
}

// WithFileMode specifies the mode of the file.
func WithFileMode(mode os.FileMode) FileBuildTarOption {
	return func(o *fileOpts) {
		o.mode = &mode
	}
}

// File is a regular file entry
func File(name, contents string, opts ...FileBuildTarOption) TarEntry {
	return tarEntryFunc(func(tw *tar.Writer, buildOpts BuildTarOptions) error {
		var fOpts fileOpts
		for _, o := range opts {
			o(&fOpts)
		}
		if strings.HasSuffix(name, "/") {
			return fmt.Errorf("bogus trailing slash in file %q", name)
		}
		var mode int64 = 0644
		if fOpts.mode != nil {
			mode = permAndExtraMode2TarMode(*fOpts.mode)
		}
		if err := tw.WriteHeader(&tar.Header{
			Typeflag: tar.TypeReg,
			Name:     buildOpts.Prefix + name,
			Mode:     mode,
			ModTime:  fOpts.modTime,
			Xattrs:   fOpts.xattrs,
			Size:     int64(len(contents)),
			Uid:      fOpts.uid,
			Gid:      fOpts.gid,
		}); err != nil {
			return err
		}
		_, err := io.WriteString(tw, contents)
		return err
	})
}

// Symlink is a symlink entry
func Symlink(name, target string) TarEntry {
	return tarEntryFunc(func(tw *tar.Writer, buildOpts BuildTarOptions) error {
		return tw.WriteHeader(&tar.Header{
			Typeflag: tar.TypeSymlink,
			Name:     buildOpts.Prefix + name,
			Linkname: target,
			Mode:     0644,
		})
	})
}

// Link is a hard-link entry
func Link(name, linkname string) TarEntry {
	now := time.Now()
	return tarEntryFunc(func(w *tar.Writer, buildOpts BuildTarOptions) error {
		return w.WriteHeader(&tar.Header{
			Typeflag:   tar.TypeLink,
			Name:       buildOpts.Prefix + name,
			Linkname:   linkname,
			ModTime:    now,
			AccessTime: now,
			ChangeTime: now,
		})
	})
}

// Chardev is a character device entry
func Chardev(name string, major, minor int64) TarEntry {
	now := time.Now()
	return tarEntryFunc(func(w *tar.Writer, buildOpts BuildTarOptions) error {
		return w.WriteHeader(&tar.Header{
			Typeflag:   tar.TypeChar,
			Name:       buildOpts.Prefix + name,
			Devmajor:   major,
			Devminor:   minor,
			ModTime:    now,
			AccessTime: now,
			ChangeTime: now,
		})
	})
}

// Blockdev is a block device entry
func Blockdev(name string, major, minor int64) TarEntry {
	now := time.Now()
	return tarEntryFunc(func(w *tar.Writer, buildOpts BuildTarOptions) error {
		return w.WriteHeader(&tar.Header{
			Typeflag:   tar.TypeBlock,
			Name:       buildOpts.Prefix + name,
			Devmajor:   major,
			Devminor:   minor,
			ModTime:    now,
			AccessTime: now,
			ChangeTime: now,
		})
	})
}

// Fifo is a fifo entry
func Fifo(name string) TarEntry {
	now := time.Now()
	return tarEntryFunc(func(w *tar.Writer, buildOpts BuildTarOptions) error {
		return w.WriteHeader(&tar.Header{
			Typeflag:   tar.TypeFifo,
			Name:       buildOpts.Prefix + name,
			ModTime:    now,
			AccessTime: now,
			ChangeTime: now,
		})
	})
}

// suid, guid, sticky bits for archive/tar
// https://github.com/golang/go/blob/release-branch.go1.13/src/archive/tar/common.go#L607-L609
const (
	cISUID = 04000 // Set uid
	cISGID = 02000 // Set gid
	cISVTX = 01000 // Save text (sticky bit)
)

func permAndExtraMode2TarMode(fm os.FileMode) (tm int64) {
	tm = int64(fm & os.ModePerm)
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
