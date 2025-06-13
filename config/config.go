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

/*
   Copyright 2019 The Go Authors. All rights reserved.
   Use of this source code is governed by a BSD-style
   license that can be found in the NOTICE.md file.
*/

package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/awslabs/soci-snapshotter/config/internal/merge"
	"github.com/pelletier/go-toml/v2"
)

const (
	// DefaultSociSnapshotterRootPath is the default filesystem path for the snapshotter root directory.
	DefaultSociSnapshotterRootPath = "/var/lib/soci-snapshotter-grpc/"

	// DefaultConfigPath is the default filesystem path for the snapshotter configuration file.
	DefaultConfigPath = "/etc/soci-snapshotter-grpc/config.toml"
)

type Config struct {
	ServiceConfig

	// Imports is a list of additional configuration filesystem paths.
	Imports []string `toml:"imports"`

	// MetricsAddress is address for the metrics API
	MetricsAddress string `toml:"metrics_address"`

	// MetricsNetwork is the type of network for the metrics API (e.g. tcp or unix)
	MetricsNetwork string `toml:"metrics_network"`

	// NoPrometheus is a flag to disable the emission of the metrics
	NoPrometheus bool `toml:"no_prometheus"`

	// DebugAddress is a Unix domain socket address where the snapshotter exposes /debug/ endpoints.
	DebugAddress string `toml:"debug_address"`

	// MetadataStore is the type of the metadata store to use.
	MetadataStore string `toml:"metadata_store"`

	// SkipCheckSnapshotterSupported is a flag to skip check for overlayfs support needed to confirm if SOCI can work
	SkipCheckSnapshotterSupported bool `toml:"skip_check_snapshotter_supported"`
}
type configParser func(*Config)

var parsers = []configParser{parseRootConfig, parseServiceConfig, parseFSConfig}

// NewConfig returns an initialized Config with default values set.
func NewConfig() *Config {
	cfg := &Config{}

	// Set any defaults which do not align with Go zero values.
	var initParsers = []configParser{defaultPullModes, defaultDirectoryCacheConfig}
	for _, p := range append(initParsers, parsers...) {
		p(cfg)
	}

	return cfg
}

// NewConfigFromToml loads the soci-snapshotter service configuration from the provided filepath.
func NewConfigFromToml(path string) (*Config, error) {
	mergedCfg := NewConfig()

	var (
		loaded  = map[string]bool{}
		pending = []string{path}
	)

	for len(pending) > 0 {
		path, pending = pending[0], pending[1:]

		// Check if a file at the given path has already been loaded to prevent circular imports.
		if _, ok := loaded[path]; ok {
			continue
		}

		// Get configuration from specified file
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to read config file %q: %w", path, err)
		}

		cfg := map[string]any{}
		if err = toml.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("failed to parse config file %q: %w", path, err)
		}

		if err := merge.Merge(mergedCfg, &cfg); err != nil {
			return nil, fmt.Errorf("failed to merge config file %q: %w", path, err)
		}

		if importsVal, ok := cfg["imports"]; ok {
			var importPaths []string

			// Convert to []string regardless of whether it's []any or []string
			switch v := importsVal.(type) {
			case []string:
				importPaths = v
			case []any:
				for i, item := range v {
					s, ok := item.(string)
					if !ok {
						return nil, fmt.Errorf("imports[%d] must be a string, got %T", i, item)
					}
					importPaths = append(importPaths, s)
				}
			default:
				return nil, fmt.Errorf("imports must be an array, got %T", importsVal)
			}

			// Process all import paths
			for _, s := range importPaths {
				if strings.Contains(s, "*") {
					paths, err := filepath.Glob(filepath.Clean(filepath.Join(filepath.Dir(path), s)))
					if err != nil {
						return nil, fmt.Errorf("failed to resolve import path pattern %q: %w", s, err)
					}
					pending = append(pending, paths...)
				} else {
					importPath := filepath.Clean(s)
					if !filepath.IsAbs(importPath) {
						importPath = filepath.Join(filepath.Dir(path), importPath)
					}
					pending = append(pending, importPath)
				}
			}
		}

		loaded[path] = true
	}
	parseConfig(mergedCfg)
	return mergedCfg, nil
}

func parseConfig(cfg *Config) {
	for _, p := range parsers {
		p(cfg)
	}
}

func parseRootConfig(cfg *Config) {
	if cfg.MetricsNetwork == "" {
		cfg.MetricsNetwork = defaultMetricsNetwork
	}
	if cfg.MetadataStore == "" {
		cfg.MetadataStore = defaultMetadataStore
	}
}
