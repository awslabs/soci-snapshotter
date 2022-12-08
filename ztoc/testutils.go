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
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"math/rand"
	"os"
	"sort"

	"github.com/awslabs/soci-snapshotter/util/testutil"
)

type TempDirMaker interface {
	TempDir() string
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
	ztoc, err := BuildZtoc(tarFile.Name(), spanSize, "test")
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
