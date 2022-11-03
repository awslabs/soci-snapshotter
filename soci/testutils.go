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

// #include "zinfo.h"
// #include <stdlib.h>
// #include <stdio.h>
import "C"

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"math/rand"
	"os"
	"sort"
	"unsafe"

	"github.com/awslabs/soci-snapshotter/util/testutil"
	"github.com/containerd/containerd/content"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

const windowSize = 32768

type gzipCheckpoint struct {
	out    int64  /* corresponding offset in uncompressed data */
	in     int64  /* offset in input file of first full byte */
	bits   int8   /* number of bits (1-7) from byte at in - 1, or 0 */
	window []byte /* preceding 32K of uncompressed data */
}

type gzipZinfo struct {
	have     uint64           /* number of list entries filled in */
	size     uint64           /* number of list entries allocated */
	list     []gzipCheckpoint /* allocated list */
	spanSize uint64
}

type TempDirMaker interface {
	TempDir() string
}

func parseDigest(digestString string) digest.Digest {
	dgst, _ := digest.Parse(digestString)
	return dgst
}

func unmarshalGzipZinfo(blob byte) (*gzipZinfo, error) {
	var index *C.struct_gzip_zinfo = C.blob_to_zinfo(unsafe.Pointer(&blob))

	if index == nil {
		return nil, fmt.Errorf("cannot convert blob to gzip_zinfo")
	}

	defer C.free_zinfo(index)

	list := make([]gzipCheckpoint, 0)
	lst := unsafe.Slice(index.list, int(index.have))
	for i := 0; i < int(index.have); i++ {
		indexPoint := lst[i]
		window := C.GoBytes(unsafe.Pointer(&indexPoint.window[0]), windowSize)
		listEntry := gzipCheckpoint{
			out:    int64(indexPoint.out),
			in:     int64(indexPoint.in),
			bits:   int8(indexPoint.bits),
			window: window,
		}
		list = append(list, listEntry)
	}

	return &gzipZinfo{
		have:     uint64(index.have),
		size:     uint64(index.size),
		spanSize: uint64(index.span_size),
		list:     list,
	}, nil
}

type fileContent struct {
	fileName string
	content  []byte
}

func buildTempTarGz(contents []fileContent, targzName string) (*string, []string, error) {
	// build a temporary directory with two files named file1 and file2
	// the files will be filled in with the contents passed in the arguments
	dir, err := os.MkdirTemp("", "test")
	if err != nil {
		return nil, nil, err
	}
	defer os.RemoveAll(dir)

	resultingFileNames := []string{}

	for _, fc := range contents {
		file, err := os.CreateTemp(dir, fc.fileName)
		if err != nil {
			break
		}
		resultingFileNames = append(resultingFileNames, file.Name())
		if _, err := file.Write(fc.content); err != nil {
			break
		}
	}

	if err != nil {
		return nil, nil, err
	}

	// build tar.gzip
	name, err := tempTarGz(dir, targzName)
	if err != nil {
		return nil, nil, err
	}
	return name, resultingFileNames, nil
}

func writeTempTarGz(filePath string, tw *tar.Writer, fi os.FileInfo) error {
	fr, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer fr.Close()

	h := new(tar.Header)
	h.Name = filePath
	h.Size = fi.Size()
	h.Mode = int64(fi.Mode())
	h.ModTime = fi.ModTime()

	err = tw.WriteHeader(h)
	if err != nil {
		return err
	}

	_, err = io.Copy(tw, fr)
	if err != nil {
		return err
	}

	return nil
}

func iterDirectory(dirPath string, tw *tar.Writer) error {
	dir, err := os.Open(dirPath)
	if err != nil {
		return err
	}
	defer dir.Close()
	fis, err := dir.Readdir(0)
	if err != nil {
		return err
	}
	sort.Slice(fis, func(i, j int) bool {
		return fis[i].Name() < fis[j].Name()
	})
	for _, fi := range fis {
		curPath := dirPath + "/" + fi.Name()
		if fi.IsDir() {
			iterDirectory(curPath, tw)
		} else {
			fmt.Printf("adding... %s\n", curPath)
			writeTempTarGz(curPath, tw, fi)
		}
	}

	return nil
}

func tempTarGz(inputDir string, targzName string) (*string, error) {
	// create an output file
	fw, err := os.CreateTemp("", targzName)
	if err != nil {
		return nil, err
	}
	defer fw.Close()

	gw := gzip.NewWriter(fw)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	err = iterDirectory(inputDir, tw)
	if err != nil {
		return nil, err
	}
	outputFileName := fw.Name()
	return &outputFileName, nil
}

// buildZtocReader creates the tar gz file for tar entries.
// It returns ztoc and io.SectionReader of the file.
func BuildZtocReader(ents []testutil.TarEntry, compressionLevel int, spanSize int64, opts ...testutil.BuildTarOption) (*Ztoc, *io.SectionReader, error) {
	// build tar gz file
	tarReader := testutil.BuildTarGz(ents, compressionLevel, opts...)

	// build ztoc
	tarFile, err := os.CreateTemp("", "tmp.*")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create temp file: %v", err)
	}
	defer os.Remove(tarFile.Name())
	tarBuf := new(bytes.Buffer)
	w := io.MultiWriter(tarFile, tarBuf)
	_, err = io.Copy(w, tarReader)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to write tar file: %v", err)
	}
	tarData := tarBuf.Bytes()
	sr := io.NewSectionReader(bytes.NewReader(tarData), 0, int64(len(tarData)))
	cfg := &buildConfig{}
	ztoc, err := BuildZtoc(tarFile.Name(), spanSize, cfg.buildToolIdentifier)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to build sample ztoc: %v", err)
	}
	return ztoc, sr, nil
}

func GenerateTempTestingDir(dirMaker TempDirMaker) (string, error) {
	tempDir := dirMaker.TempDir()
	err := createRandFile(tempDir+"/smallfile", 1, 100)
	if err != nil {
		return "", fmt.Errorf("failed to create small random file: %w", err)
	}
	err = createRandFile(tempDir+"/mediumfile", 10000, 128000)
	if err != nil {
		return "", fmt.Errorf("failed to create medium random file: %w", err)
	}
	err = createRandFile(tempDir+"/largefile", 350000, 500000)
	if err != nil {
		return "", fmt.Errorf("failed to create large random file: %w", err)
	}
	err = createRandFile(tempDir+"/jumbofile", 3000000, 5000000)
	if err != nil {
		return "", fmt.Errorf("failed to create jumbo random file: %w", err)
	}

	return tempDir, nil
}

func createRandFile(name string, minBytes int, maxBytes int) error {
	f, err := os.Create(name)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer f.Close()

	const charset = "abcdefghijklmnopqrstuvwxyz" +
		"ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789" + " "
	const randSeed = 1658503010463818386

	rand.Seed(randSeed)
	randByteNum := rand.Intn(maxBytes-minBytes) + minBytes
	randBytes := make([]byte, randByteNum)
	for i := range randBytes {
		randBytes[i] = charset[rand.Intn(len(charset))]
	}

	_, err = f.WriteString(string(randBytes))
	if err != nil {
		return fmt.Errorf("failed to write string: %w", err)
	}
	f.Sync()
	return nil
}

type fakeContentStore struct {
}

// Abort implements content.Store
func (fakeContentStore) Abort(ctx context.Context, ref string) error {
	return nil
}

// ListStatuses implements content.Store
func (fakeContentStore) ListStatuses(ctx context.Context, filters ...string) ([]content.Status, error) {
	panic("unimplemented")
}

// Status implements content.Store
func (fakeContentStore) Status(ctx context.Context, ref string) (content.Status, error) {
	panic("unimplemented")
}

// Writer implements content.Store
func (fakeContentStore) Writer(ctx context.Context, opts ...content.WriterOpt) (content.Writer, error) {
	return fakeWriter{}, nil
}

// Delete implements content.Store
func (fakeContentStore) Delete(ctx context.Context, dgst digest.Digest) error {
	return nil
}

// Info implements content.Store
func (fakeContentStore) Info(ctx context.Context, dgst digest.Digest) (content.Info, error) {
	panic("unimplemented")
}

// Update implements content.Store
func (fakeContentStore) Update(ctx context.Context, info content.Info, fieldpaths ...string) (content.Info, error) {
	panic("unimplemented")
}

// Walk implements content.Store
func (fakeContentStore) Walk(ctx context.Context, fn content.WalkFunc, filters ...string) error {
	return nil
}

// ReaderAt implements content.Store
func (fakeContentStore) ReaderAt(ctx context.Context, desc ocispec.Descriptor) (content.ReaderAt, error) {
	return newFakeReaderAt(desc), nil
}

func newFakeContentStore() content.Store {
	return fakeContentStore{}
}

type fakeReaderAt struct {
	size int64
}

// Close implements content.ReaderAt
func (fakeReaderAt) Close() error {
	return nil
}

// ReadAt implements content.ReaderAt
func (r fakeReaderAt) ReadAt(p []byte, off int64) (n int, err error) {
	return int(r.size), nil
}

// Size implements content.ReaderAt
func (r fakeReaderAt) Size() int64 {
	return r.size
}

func newFakeReaderAt(desc ocispec.Descriptor) content.ReaderAt {
	return fakeReaderAt{size: desc.Size}
}

type fakeWriter struct {
	io.Writer
	status     content.Status
	commitFunc func() error
}

func (f fakeWriter) Write(p []byte) (n int, err error) {
	return len(p), nil
}

func (f fakeWriter) Close() error {
	return nil
}

func (f fakeWriter) Commit(ctx context.Context, size int64, expected digest.Digest, opts ...content.Opt) error {
	if f.commitFunc == nil {
		return nil
	}
	return f.commitFunc()
}

func (f fakeWriter) Digest() digest.Digest {
	return digest.FromString("")
}

func (f fakeWriter) Status() (content.Status, error) {
	return f.status, nil
}

func (f fakeWriter) Truncate(size int64) error {
	return nil
}
