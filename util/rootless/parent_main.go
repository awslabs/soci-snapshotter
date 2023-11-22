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

package rootless

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/containerd/nerdctl/pkg/rootlessutil"
)

func ParentMain() error {
	if !rootlessutil.IsRootlessParent() {
		return errors.New("should not be called when !IsRootlessParent()")
	}
	stateDir, err := rootlessutil.RootlessKitStateDir()
	if err != nil {
		return fmt.Errorf("rootless containerd not running? (hint: use `containerd-rootless-setuptool.sh install` to start rootless containerd): %w", err)
	}
	childPid, err := rootlessutil.RootlessKitChildPid(stateDir)
	if err != nil {
		return err
	}

	wd, err := os.Getwd()
	if err != nil {
		return err
	}

	// FIXME: remove dependency on `nsenter` binary
	arg0, err := exec.LookPath("nsenter")
	if err != nil {
		return err
	}
	// args are compatible with both util-linux nsenter and busybox nsenter
	args := []string{
		"-r/",     // root dir (busybox nsenter wants this to be explicitly specified),
		"-w" + wd, // work dir
		"--preserve-credentials",
		"-m", "-n", "-U",
		"-t", strconv.Itoa(childPid),
		"-F", // no fork
	}

	// replace the args[0] with full path
	var originalCmd = os.Args[0]
	originalCmdFullPath, err := filepath.Abs(originalCmd)
	if err != nil {
		fmt.Printf("failed to get full path: %s\n", err)
	}
	os.Args[0] = originalCmdFullPath

	args = append(args, os.Args...)

	// Env vars corresponds to RootlessKit spec:
	// https://github.com/rootless-containers/rootlesskit/tree/v0.13.1#environment-variables
	os.Setenv("ROOTLESSKIT_STATE_DIR", stateDir)
	os.Setenv("ROOTLESSKIT_PARENT_EUID", strconv.Itoa(os.Geteuid()))
	os.Setenv("ROOTLESSKIT_PARENT_EGID", strconv.Itoa(os.Getegid()))
	return syscall.Exec(arg0, args, os.Environ())
}
