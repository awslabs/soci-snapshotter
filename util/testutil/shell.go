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

// This file contains some utilities that supports to manipulate dockershell.

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/awslabs/soci-snapshotter/soci/store"
	shell "github.com/awslabs/soci-snapshotter/util/dockershell"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/rs/xid"
	"golang.org/x/sync/errgroup"
)

// TestingReporter is an implementation of dockershell.Reporter backed by testing.T and TestingL.
type TestingReporter struct {
	t *testing.T
}

// NewTestingReporter returns a new TestingReporter instance for the specified testing.T.
func NewTestingReporter(t *testing.T) *TestingReporter {
	return &TestingReporter{t}
}

// Errorf prints the provided message to TestingL and stops the test using testing.T.Fatalf.
func (r *TestingReporter) Errorf(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	_, file, lineNum, ok := runtime.Caller(2)
	if ok {
		r.t.Fatalf("%s:%d: %s", file, lineNum, msg)
	} else {
		r.t.Fatalf(format, v...)
	}
}

// Logf prints the provided message to TestingL testing.T.
func (r *TestingReporter) Logf(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	_, file, lineNum, ok := runtime.Caller(2)
	if ok {
		r.t.Logf("%s:%d: %s", file, lineNum, msg)
	} else {
		r.t.Logf(format, v...)
	}
}

// Stdout returns the writer to TestingL as stdout. This enables to print command logs realtime.
func (r *TestingReporter) Stdout() io.Writer {
	return TestingL.Writer()
}

// Stderr returns the writer to TestingL as stderr. This enables to print command logs realtime.
func (r *TestingReporter) Stderr() io.Writer {
	return TestingL.Writer()
}

// LogMonitor manages a list of functions that should scan lines coming from stdout and stderr Readers
type LogMonitor struct {
	monitorFuncs map[string]func(string)
}

// NewLogMonitor creates a LogMonitor for a given pair of stdout and stderr Readers
func NewLogMonitor(r shell.Reporter, stdout, stderr io.Reader) *LogMonitor {
	m := &LogMonitor{}
	m.monitorFuncs = make(map[string]func(string))
	go m.scanLog(io.TeeReader(stdout, r.Stdout()))
	go m.scanLog(io.TeeReader(stderr, r.Stderr()))
	return m
}

// Add registers a new log monitor function
func (m *LogMonitor) Add(name string, monitorFunc func(string)) error {
	if _, ok := m.monitorFuncs[name]; ok {
		return fmt.Errorf("attempted to add log monitor with already existing name: %s", name)
	}
	m.monitorFuncs[name] = monitorFunc
	return nil
}

// Remove unregisters a log monitor function
func (m *LogMonitor) Remove(name string) error {
	if _, ok := m.monitorFuncs[name]; ok {
		delete(m.monitorFuncs, name)
		return nil
	}
	return fmt.Errorf("attempted to remove nonexistent log monitor: %s", name)
}

// scanLog calls each registered log monitor function for each new line of the Reader
func (m *LogMonitor) scanLog(inputR io.Reader) {
	scanner := bufio.NewScanner(inputR)
	for scanner.Scan() {
		rawL := scanner.Text()
		for _, monitorFunc := range m.monitorFuncs {
			monitorFunc(rawL)
		}
	}
}

// RemoteSnapshotMonitor scans log of soci snapshotter and provides the way to check
// if all snapshots are prepared as remote snpashots.
type RemoteSnapshotMonitor struct {
	remote uint64
	local  uint64
}

// NewRemoteSnapshotMonitor creates a new instance of RemoteSnapshotMonitor and registers it
// with the LogMonitor
func NewRemoteSnapshotMonitor(m *LogMonitor) (*RemoteSnapshotMonitor, func()) {
	rsm := &RemoteSnapshotMonitor{}
	m.Add("remote snapshot", rsm.MonitorFunc)
	return rsm, func() { m.Remove("remote snapshot") }
}

type RemoteSnapshotPreparedLogLine struct {
	RemoteSnapshotPrepared string `json:"remote-snapshot-prepared"`
}

// MonitorFunc counts remote/local snapshot preparation totals
func (m *RemoteSnapshotMonitor) MonitorFunc(rawL string) {
	var logline RemoteSnapshotPreparedLogLine
	if i := strings.Index(rawL, "{"); i > 0 {
		rawL = rawL[i:] // trim garbage chars; expects "{...}"-styled JSON log
	}
	if err := json.Unmarshal([]byte(rawL), &logline); err == nil {
		if logline.RemoteSnapshotPrepared == "true" {
			atomic.AddUint64(&m.remote, 1)
		} else if logline.RemoteSnapshotPrepared == "false" {
			atomic.AddUint64(&m.local, 1)
		}
	}
}

// CheckAllRemoteSnapshots checks if the scanned log reports that all snapshots are prepared
// as remote snapshots.
func (m *RemoteSnapshotMonitor) CheckAllRemoteSnapshots(t *testing.T) {
	remote := atomic.LoadUint64(&m.remote)
	local := atomic.LoadUint64(&m.local)
	result := fmt.Sprintf("(local:%d,remote:%d)", local, remote)
	if local > 0 {
		t.Fatalf("some local snapshots creation have been reported %v", result)
	} else if remote > 0 {
		t.Logf("all layers have been reported as remote snapshots %v", result)
		return
	} else {
		t.Fatalf("no log for checking remote snapshot was provided; Is the log-level = debug?")
	}
}

// LogConfirmStartup registers a LogMonitor function to scan until startup succeeds or fails
func LogConfirmStartup(m *LogMonitor) error {
	errs := make(chan error, 1)
	m.Add("startup", monitorStartup(errs))
	defer m.Remove("startup")
	select {
	case err := <-errs:
		if err != nil {
			return err
		}
	case <-time.After(10 * time.Second): // timeout
		return fmt.Errorf("log did not produce success or fatal error within 10 seconds")
	}
	return nil
}

type LevelLogLine struct {
	Level string `json:"level"`
	Msg   string `json:"msg"`
}

// monitorStartup creates a LogMonitor function to pass success or failure back through the given channel
func monitorStartup(errs chan error) func(string) {
	return func(rawL string) {
		if i := strings.Index(rawL, "{"); i > 0 {
			rawL = rawL[i:] // trim garbage chars; expects "{...}"-styled JSON log
		}
		var logline LevelLogLine
		if err := json.Unmarshal([]byte(rawL), &logline); err == nil {
			if logline.Level == "fatal" {
				errs <- errors.New("fatal snapshotter log entry encountered, snapshotter failed to start")
				return
			}
			// Looking for "soci-snapshotter-grpc successfully started"
			if strings.Contains(logline.Msg, "successfully") {
				errs <- nil
				return
			}
		}
	}
}

// TempDir creates a temporary directory in the specified execution environment.
func TempDir(sh *shell.Shell) (string, error) {
	out, err := sh.Command("mktemp", "-d").Output()
	if err != nil {
		return "", fmt.Errorf("failed to run mktemp: %v", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func injectContainerdContentStoreContentFromReader(sh *shell.Shell, desc ocispec.Descriptor, content io.Reader) error {
	reference := desc.Digest.String()
	cmd := sh.Command("ctr", "content", "ingest", reference)
	cmd.Stdin = content
	if err := cmd.Run(); err != nil {
		return err
	}
	// Labelling the content root is not the right thing to do. We should build the proper set of references
	// back to an image under a lease, but this should at least be safe because integration containers are
	// ephemeral for the single test.
	cmd = sh.Command("ctr", "content", "label", desc.Digest.String(), "containerd.io/gc.root")
	return cmd.Run()
}

func injectSociContentStoreContentFromReader(sh *shell.Shell, desc ocispec.Descriptor, content io.Reader) error {
	dir := filepath.Join(store.DefaultSociContentStorePath, "blobs", desc.Digest.Algorithm().String())
	if err := sh.Command("mkdir", "-p", dir).Run(); err != nil {
		return err
	}
	path := filepath.Join(dir, desc.Digest.Encoded())
	cmd := sh.Command("/bin/sh", "-c", fmt.Sprintf("cat > %s && chmod %#o %s", path, 0600, path))
	cmd.Stdin = content
	return cmd.Run()
}

func InjectContentStoreContentFromReader(sh *shell.Shell, contentStoreType store.ContentStoreType, desc ocispec.Descriptor, content io.Reader) error {
	contentStoreType, err := store.CanonicalizeContentStoreType(contentStoreType)
	if err != nil {
		return err
	}
	switch contentStoreType {
	case store.SociContentStoreType:
		injectSociContentStoreContentFromReader(sh, desc, content)
	case store.ContainerdContentStoreType:
		injectContainerdContentStoreContentFromReader(sh, desc, content)
	default:
		return store.ErrUnknownContentStoreType(contentStoreType)
	}
	return nil
}

func InjectContentStoreContentFromBytes(sh *shell.Shell, contentStoreType store.ContentStoreType, desc ocispec.Descriptor, content []byte) error {
	return InjectContentStoreContentFromReader(sh, contentStoreType, desc, bytes.NewReader(content))
}

func writeFileFromReader(sh *shell.Shell, name string, content io.Reader, mode uint32) error {
	if err := sh.Command("mkdir", "-p", filepath.Dir(name)).Run(); err != nil {
		return err
	}
	cmd := sh.Command("/bin/sh", "-c", fmt.Sprintf("cat > %s && chmod %#o %s", name, mode, name))
	cmd.Stdin = content
	return cmd.Run()
}

// WriteFileContents creates a file at the specified location in the specified execution environment
// and writes the specified contents to that file.
func WriteFileContents(sh *shell.Shell, name string, content []byte, mode uint32) error {
	return writeFileFromReader(sh, name, bytes.NewReader(content), mode)
}

// CopyInDir copies a directory into the specified location in the specified execution environment.
func CopyInDir(sh *shell.Shell, from, to string) error {
	if !filepath.IsAbs(from) || !filepath.IsAbs(to) {
		return fmt.Errorf("path %v and %v must be absolute path", from, to)
	}

	pr, pw := io.Pipe()
	cmdFrom := exec.Command("tar", "-zcf", "-", "-C", from, ".")
	cmdFrom.Stdout = pw
	var eg errgroup.Group
	eg.Go(func() error {
		if err := cmdFrom.Run(); err != nil {
			pw.CloseWithError(err)
			return err
		}
		pw.Close()
		return nil
	})

	tmpTar := "/tmptar" + xid.New().String()
	if err := writeFileFromReader(sh, tmpTar, pr, 0755); err != nil {
		return fmt.Errorf("writeFileFromReader: %w", err)
	}
	if err := eg.Wait(); err != nil {
		return fmt.Errorf("taring: %w", err)
	}
	if err := sh.Command("mkdir", "-p", to).Run(); err != nil {
		return fmt.Errorf("mkdir -p %v: %w", to, err)
	}
	if err := sh.Command("tar", "zxf", tmpTar, "-C", to).Run(); err != nil {
		return fmt.Errorf("tar zxf %v -C %v: %w", tmpTar, to, err)
	}
	return sh.Command("rm", tmpTar).Run()
}

// KillMatchingProcess kills processes that "ps" line matches the specified pattern in the
// specified execution environment.
func KillMatchingProcess(sh *shell.Shell, psLinePattern string) error {
	data, err := sh.Command("ps", "axo", "pid,command").Output()
	if err != nil {
		return fmt.Errorf("failed to run ps command: %v", err)
	}
	var targets []int
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		psline := scanner.Text()
		matched, err := regexp.Match(psLinePattern, []byte(psline))
		if err != nil {
			return err
		}
		if matched {
			es := strings.Fields(psline)
			if len(es) < 2 {
				continue
			}
			pid, err := strconv.ParseInt(es[0], 10, 32)
			if err != nil {
				continue
			}
			targets = append(targets, int(pid))
		}
	}

	var allErr error
	for _, pid := range targets {
		allErr = errors.Join(allErr, killProcess(sh, pid))

	}
	return allErr
}

func killProcess(sh *shell.Shell, pid int) error {
	// Send SIGTERM so the unit under test correctly writes integration coverage reports to Go coverage directory.
	ex := sh.Command("kill", "-2", fmt.Sprintf("%d", pid))
	if out, err := ex.CombinedOutput(); err != nil {
		// If the process disappeared between the ps and the kill, don't treat it as an error
		if !strings.Contains(string(out), "No such process") {
			return err
		}
	}
	return nil
}

func RemoveContentStoreContent(sh *shell.Shell, contentStoreType store.ContentStoreType, contentDigest string) error {
	contentStoreType, err := store.CanonicalizeContentStoreType(contentStoreType)
	if err != nil {
		return err
	}
	switch contentStoreType {
	case store.SociContentStoreType:
		removeSociContentStoreContent(sh, contentDigest)
	case store.ContainerdContentStoreType:
		removeContainerdContentStoreContent(sh, contentDigest)
	}
	return nil
}

func removeSociContentStoreContent(sh *shell.Shell, contentDigest string) {
	path, _ := GetContentStoreBlobPath(store.SociContentStoreType)
	dgst, err := digest.Parse(contentDigest)
	if err == nil {
		sh.X("rm", filepath.Join(path, dgst.Encoded()))
	}
}

func removeContainerdContentStoreContent(sh *shell.Shell, contentDigest string) {
	sh.X("ctr", "content", "rm", contentDigest)
}
