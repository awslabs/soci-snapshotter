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
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

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
		return "", fmt.Errorf("Dockerfile not found under project root")
	}
	return pRoot, nil
}

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

// RandomByteData returns a byte slice with `size` random generated data
func RandomByteData(size int64) []byte {
	b := make([]byte, size)
	rand.Read(b)
	return b
}

func RandomDigest() string {
	d := digest.FromBytes(RandomByteData(10))
	return d.String()
}
