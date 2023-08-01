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

import (
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/opencontainers/go-digest"
)

const (
	rootRelGOPATH   = "/src/github.com/awslabs/soci-snapshotter"
	projectRootEnv  = "SOCI_SNAPSHOTTER_PROJECT_ROOT"
	BuildKitVersion = "v0.8.1"
)

// TestingL is a Logger instance used during testing. This allows tests to prints logs in realtime.
var TestingL = log.New(os.Stdout, "testing: ", log.Ldate|log.Ltime)

// TestingLogDest returns Writes of Testing.T.
func TestingLogDest() (io.Writer, io.Writer) {
	return TestingL.Writer(), TestingL.Writer()
}

// StreamTestingLogToFile allows TestingL to stream the logging output to the speicified file.
func StreamTestingLogToFile(destPath string) (func() error, error) {
	if !filepath.IsAbs(destPath) {
		return nil, fmt.Errorf("log destination must be an absolute path: got %v", destPath)
	}
	f, err := os.Create(destPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create %v: %w", destPath, err)
	}
	TestingL.SetOutput(io.MultiWriter(f, os.Stdout))
	return f.Close, nil
}

// GetProjectRoot returns the path to the directory where the source code of this project reside.
func GetProjectRoot() (string, error) {
	pRoot := os.Getenv(projectRootEnv)
	if pRoot == "" {
		gopath := os.Getenv("GOPATH")
		if gopath == "" {
			gopathB, err := exec.Command("go", "env", "GOPATH").Output()
			if len(gopathB) == 0 || err != nil {
				return "", fmt.Errorf("project unknown; specify %v or GOPATH: %v", projectRootEnv, err)
			}
			gopath = strings.TrimSpace(string(gopathB))
		}
		pRoot = filepath.Join(gopath, rootRelGOPATH)
		if _, err := os.Stat(pRoot); err != nil {
			return "", fmt.Errorf("project (%v) unknown; specify %v", pRoot, projectRootEnv)
		}
	}
	if _, err := os.Stat(filepath.Join(pRoot, "Dockerfile")); err != nil {
		return "", fmt.Errorf("no Dockerfile was found under project root")
	}
	return pRoot, nil
}

const TestRandomSeed = 1658503010463818386

// ThreadsafeRandom is like rand.Rand with thread safety.
// rand.Rand is not threadsafe except for the global rand.Rand which is only accessible through
// the exported function on the rand package (e.g. rand.Int63()). This is done by special casing
// a non-exported rand.lockedSource which is locked before doing any modification of the source or rand.
// It's not possible to create our own lockedSource that implements `rand.Source` because `rand.Rand`
// itself is not threadsafe. The actual implementation gets around this by locking the `rand.Rand`'s source
// which effectively locks the `rand.Rand` as well.
// There is an expermiental version of rand that exports `rand.LockedSource`. If that ever lands, then we can
// remove all of this code and just use `r := rand.New(rand.NewLockedSource(seed))`.
// https://pkg.go.dev/golang.org/x/exp@v0.0.0-20230801115018-d63ba01acd4b/rand#LockedSource
type ThreadsafeRandom struct {
	l sync.Mutex
	r *rand.Rand
}

func NewThreadsafeRandom() *ThreadsafeRandom {
	return &ThreadsafeRandom{
		l: sync.Mutex{},
		r: rand.New(rand.NewSource(TestRandomSeed)),
	}
}

func (tsr *ThreadsafeRandom) Intn(n int) int {
	tsr.l.Lock()
	defer tsr.l.Unlock()
	return tsr.r.Intn(n)
}

func (tsr *ThreadsafeRandom) Int63() int64 {
	tsr.l.Lock()
	defer tsr.l.Unlock()
	return tsr.r.Int63()
}

func (tsr *ThreadsafeRandom) Read(b []byte) (int, error) {
	tsr.l.Lock()
	defer tsr.l.Unlock()
	return tsr.r.Read(b)
}

var r = NewThreadsafeRandom()

// RandomUInt64 returns a random uint64 value generated from /dev/uramdom.
func RandomUInt64() (uint64, error) {
	f, err := os.Open("/dev/urandom")
	if err != nil {
		return 0, fmt.Errorf("failed to open /dev/urandom")
	}
	defer f.Close()
	b := make([]byte, 8)
	if _, err := f.Read(b); err != nil {
		return 0, fmt.Errorf("failed to read /dev/urandom")
	}
	return binary.LittleEndian.Uint64(b), nil
}

// RandomByteData returns a byte slice with `size` populated with random generated data
func RandomByteData(size int64) []byte {
	b := make([]byte, size)
	r.Read(b)
	return b
}

// RandomByteDataRange returns a byte slice with `size` between minBytes and maxBytes exclusive populated with random data
func RandomByteDataRange(minBytes int, maxBytes int) []byte {
	const charset = "abcdefghijklmnopqrstuvwxyz" +
		"ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789" + " "

	r := NewThreadsafeRandom()
	randByteNum := r.Intn(maxBytes-minBytes) + minBytes
	randBytes := make([]byte, randByteNum)
	for i := range randBytes {
		randBytes[i] = charset[r.Intn(len(charset))]
	}
	return randBytes
}

// RandomDigest generates a random digest from a random sequence of bytes
func RandomDigest() string {
	d := digest.FromBytes(RandomByteData(10))
	return d.String()
}
