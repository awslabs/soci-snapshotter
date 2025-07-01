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
type configParser func(*Config) error

var parsers = []configParser{parseRootConfig, parseServiceConfig, parseFSConfig, parseParallelConfig}

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

func NewConfigFromToml(cfgPath string) (*Config, error) {
	f, err := os.Open(cfgPath)
	if err != nil {
		if os.IsNotExist(err) && cfgPath == DefaultConfigPath {
			return NewConfig(), nil
		}
		return nil, fmt.Errorf("failed to open config file %q: %w", cfgPath, err)
	}
	defer f.Close()

	cfg := NewConfig()
	// Get configuration from specified file
	if err = toml.NewDecoder(f).Decode(cfg); err != nil {
		return nil, fmt.Errorf("failed to load config file %q: %w", cfgPath, err)
	}
	parseConfig(cfg)
	return cfg, nil
}

func parseConfig(cfg *Config) {
	for _, p := range parsers {
		p(cfg)
	}
}

func parseRootConfig(cfg *Config) error {
	if cfg.MetricsNetwork == "" {
		cfg.MetricsNetwork = defaultMetricsNetwork
	}
	if cfg.MetadataStore == "" {
		cfg.MetadataStore = defaultMetadataStore
	}
	return nil
}
