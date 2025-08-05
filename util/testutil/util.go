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
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const (
	rootRelGOPATH   = "/src/github.com/awslabs/soci-snapshotter"
	projectRootEnv  = "SOCI_SNAPSHOTTER_PROJECT_ROOT"
	BuildKitVersion = "v0.8.1"
)

// TestingL is a Logger instance used during testing. This allows tests to prints logs in realtime.
// This should only be used when there is no testing.TB available (e.g. TestMain). Other usecases
// should use a TestingReporter.
var TestingL = log.New(os.Stdout, "testing: ", log.Ldate|log.Ltime)

func BufferedTestingLogDest() (*BufferedWriter, *BufferedWriter) {
	stdout, stderr := TestingLogDest()
	return NewBufferedWriter(stdout), NewBufferedWriter(stderr)
}

// TestingLogDest returns Writes of Testing.T.
func TestingLogDest() (io.Writer, io.Writer) {
	return TestingL.Writer(), TestingL.Writer()
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

// TestWriter adapts a testing.TB into an io.Writer
type TestWriter struct {
	t testing.TB
}

func NewTestWriter(t testing.TB) *TestWriter {
	return &TestWriter{t: t}
}

func (t *TestWriter) Write(p []byte) (n int, err error) {
	t.t.Log(string(p))
	return len(p), nil
}

// BufferedWriter is a writer that buffers all writes until Flush is called.
// It is similar to a bufio.Writer except that it will never write to the underlying
// writer unless Flush is called.
type BufferedWriter struct {
	b []byte
	w io.Writer
}

func NewBufferedWriter(w io.Writer) *BufferedWriter {
	return &BufferedWriter{w: w}
}

// Write writes a log into the buffer
func (t *BufferedWriter) Write(p []byte) (n int, err error) {
	t.b = append(t.b, p...)
	return len(p), nil
}

// Flush flushes the buffer to the underlying writer
func (t *BufferedWriter) Flush() {
	io.Copy(t.w, bytes.NewReader(t.b))
	t.b = nil
}
