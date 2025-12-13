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

package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/awslabs/soci-snapshotter/config"
	"github.com/sirupsen/logrus"
)

func TestEnvVarOverridesDefaultConfigPath(t *testing.T) {
	const (
		outputFileName         = "output.toml"
		customImageServicePath = "/custom/test/containerd.sock"
		customConfig           = `
[cri_keychain]
  image_service_path = "` + customImageServicePath + `"
`
	)

	tempDir := t.TempDir()
	configPath := writeTestConfig(t, tempDir, customConfig)

	t.Setenv(envConfig, configPath)

	outputPath := filepath.Join(tempDir, outputFileName)
	outputFile, err := os.Create(outputPath)
	if err != nil {
		t.Fatalf("failed to create output file: %v", err)
	}
	defer outputFile.Close()

	oldStdout := os.Stdout
	os.Stdout = outputFile
	defer func() { os.Stdout = oldStdout }()

	app := buildApp()
	ctx := context.Background()
	args := []string{"soci-snapshotter-grpc", "config", "dump"}

	if err := app.Run(ctx, args); err != nil {
		t.Fatalf("failed to run config dump: %v", err)
	}

	cfg, err := config.NewConfigFromToml(outputPath)
	if err != nil {
		t.Fatalf("failed to load config from output: %v", err)
	}

	if cfg.CRIKeychainConfig.ImageServicePath != customImageServicePath {
		t.Errorf("expected image_service_path: %q, got: %q",
			customImageServicePath, cfg.CRIKeychainConfig.ImageServicePath)
	}
}

func TestEnvVarOverridesDefaultLogLevel(t *testing.T) {
	const newLogLevel = logrus.DebugLevel

	tempDir := t.TempDir()
	configPath := writeTestConfig(t, tempDir, "skip_check_snapshotter_supported = true")
	sockPath := filepath.Join(tempDir, "test.sock")

	originalLevel := logrus.GetLevel()
	defer logrus.SetLevel(originalLevel)

	t.Setenv(envConfig, configPath)
	t.Setenv(envLogLevel, newLogLevel.String())
	t.Setenv(envRoot, tempDir)
	t.Setenv(envAddress, sockPath)

	app := buildApp()
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	args := []string{"soci-snapshotter-grpc"}

	go func() {
		app.Run(ctx, args)
	}()

	time.Sleep(100 * time.Millisecond)

	if actualLevel := logrus.GetLevel(); actualLevel != newLogLevel {
		t.Errorf("expected log level: %s, got: %s", newLogLevel, actualLevel)
	}
}

func writeTestConfig(t *testing.T, dir, content string) string {
	t.Helper()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}
	return path
}
