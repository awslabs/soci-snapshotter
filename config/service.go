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

package config

type ServiceConfig struct {
	FSConfig

	// KubeconfigKeychainConfig is config for kubeconfig-based keychain.
	KubeconfigKeychainConfig `toml:"kubeconfig_keychain"`

	// CRIKeychainConfig is config for CRI-based keychain.
	CRIKeychainConfig `toml:"cri_keychain"`

	// ResolverConfig is config for resolving registries.
	ResolverConfig `toml:"resolver"`

	// SnapshotterConfig is snapshotter-related config.
	SnapshotterConfig `toml:"snapshotter"`
}

// KubeconfigKeychainConfig is config for kubeconfig-based keychain.
type KubeconfigKeychainConfig struct {
	// EnableKeychain enables kubeconfig-based keychain
	EnableKeychain bool `toml:"enable_keychain"`

	// KubeconfigPath is the path to kubeconfig which can be used to sync
	// secrets on the cluster into this snapshotter.
	KubeconfigPath string `toml:"kubeconfig_path"`
}

// CRIKeychainConfig is config for CRI-based keychain.
type CRIKeychainConfig struct {
	// EnableKeychain enables CRI-based keychain
	EnableKeychain bool `toml:"enable_keychain"`

	// ImageServicePath is the path to the unix socket of backing CRI Image Service (e.g. containerd CRI plugin)
	ImageServicePath string `toml:"image_service_path"`
}

// SnapshotterConfig is snapshotter-related config.
type SnapshotterConfig struct {
	// MinLayerSize skips remote mounting of smaller layers
	MinLayerSize int64 `toml:"min_layer_size"`

	// AllowInvalidMountsOnRestart allows that there are snapshot mounts that cannot access to the
	// data source when restarting the snapshotter.
	// NOTE: User needs to manually remove the snapshots from containerd's metadata store using
	//       ctr (e.g. `ctr snapshot rm`).
	AllowInvalidMountsOnRestart bool `toml:"allow_invalid_mounts_on_restart"`
}

func parseServiceConfig(cfg *Config) {
	if cfg.CRIKeychainConfig.ImageServicePath == "" {
		cfg.CRIKeychainConfig.ImageServicePath = DefaultImageServiceAddress
	}
}
