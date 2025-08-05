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

package integration

import (
	"os"
	"testing"

	shell "github.com/awslabs/soci-snapshotter/util/dockershell"
	"github.com/awslabs/soci-snapshotter/util/dockershell/compose"
	dexec "github.com/awslabs/soci-snapshotter/util/dockershell/exec"
	"github.com/awslabs/soci-snapshotter/util/testutil"
	"github.com/sirupsen/logrus"
)

const (
	enableTestEnv         = "ENABLE_INTEGRATION_TEST"
	containerdLogLevelEnv = "CONTAINERD_LOG_LEVEL"
	sociLogLevelEnv       = "SOCI_LOG_LEVEL"
)

// this can be overwritten by setting up env variables specified by
// `containerdLogLevelEnv`/`sociLogLevelEnv`.
var (
	containerdLogLevel = "warn"
	sociLogLevel       = "debug"
)

// TestMain is a main function for integration tests.
// This checks the system requirements the run tests.
func TestMain(m *testing.M) {
	if os.Getenv(enableTestEnv) != "true" {
		testutil.TestingL.Printf("%s is not true. skipping integration test", enableTestEnv)
		return
	}

	if logLevel := os.Getenv(containerdLogLevelEnv); logLevel != "" {
		if _, err := logrus.ParseLevel(logLevel); err != nil {
			testutil.TestingL.Printf("unsupported log level: %s. skipping integration test", logLevel)
			return
		}
		containerdLogLevel = logLevel
	}
	if logLevel := os.Getenv(sociLogLevelEnv); logLevel != "" {
		if _, err := logrus.ParseLevel(logLevel); err != nil {
			testutil.TestingL.Printf("unsupported log level: %s. skipping integration test", logLevel)
			return
		}
		sociLogLevel = logLevel
	}

	if err := shell.Supported(); err != nil {
		testutil.TestingL.Fatalf("shell pkg is not supported: %v", err)
	}
	if err := compose.Supported(); err != nil {
		testutil.TestingL.Fatalf("compose pkg is not supported: %v", err)
	}
	if err := dexec.Supported(); err != nil {
		testutil.TestingL.Fatalf("dockershell/exec pkg is not supported: %v", err)
	}

	cleanups, err := setup()
	if err != nil {
		testutil.TestingL.Fatalf("failed integration test set up: %v", err)
	}

	c := m.Run()

	err = teardown(cleanups)
	if err != nil {
		testutil.TestingL.Fatalf("failed integration test tear down: %v", err)
	}

	os.Exit(c)
}

// setup can be used to initialize things before integration tests start (as of now it only builds the services used by the integration tests so they can be referenced)
func setup() ([]func() error, error) {
	var (
		serviceName   = "testing"
		targetStage   = "containerd-snapshotter-base"
		registryStage = "registry"
	)
	pRoot, err := testutil.GetProjectRoot()
	if err != nil {
		return nil, err
	}
	buildArgs, err := getBuildArgsFromEnv()
	if err != nil {
		return nil, err
	}

	composeYaml, err := testutil.ApplyTextTemplate(composeBuildTemplate, dockerComposeYaml{
		ServiceName:     serviceName,
		ImageContextDir: pRoot,
		TargetStage:     targetStage,
		RegistryStage:   registryStage,
	})
	if err != nil {
		return nil, err
	}
	stdout, stderr := testutil.BufferedTestingLogDest()
	cOpts := []compose.Option{
		compose.WithBuildArgs(buildArgs...),
		compose.WithStdio(stdout, stderr),
	}

	cleanup, err := compose.Build(composeYaml, cOpts...)
	if err != nil {
		stdout.Flush()
		stderr.Flush()
	}
	return cleanup, err

}

// teardown takes a list of cleanup functions and executes them after integration tests have ended
func teardown(cleanups []func() error) error {
	for i := 0; i < len(cleanups); i++ {
		err := cleanups[i]()
		if err != nil {
			return err
		}
	}
	return nil
}
