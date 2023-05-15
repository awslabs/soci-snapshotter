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

const (
	// Default path to OCI-compliant CAS
	SociContentStorePath = "/var/lib/soci-snapshotter-grpc/content/"

	// Default path to snapshotter root dir
	SociSnapshotterRootPath = "/var/lib/soci-snapshotter-grpc/"
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
	MetadataStore string `toml:"metadata_store" default:"db"`
}
