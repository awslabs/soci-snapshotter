#!/usr/bin/env bash

#   Copyright The Soci Snapshotter Authors.

#   Licensed under the Apache License, Version 2.0 (the "License");
#   you may not use this file except in compliance with the License.
#   You may obtain a copy of the License at

#       http://www.apache.org/licenses/LICENSE-2.0

#   Unless required by applicable law or agreed to in writing, software
#   distributed under the License is distributed on an "AS IS" BASIS,
#   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
#   See the License for the specific language governing permissions and
#   limitations under the License.

set -eux -o pipefail

cur_dir="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
soci_snapshotter_project_root="$(cd -- "$cur_dir"/.. && pwd)"
config_loc="${soci_snapshotter_project_root}"/config/config.toml
out_dir="${soci_snapshotter_project_root}"/out
soci_snapshotter_grpc="${out_dir}"/soci-snapshotter-grpc

# Check if default config has changed.
tmpdir=$(mktemp -d)
tmpconfig="${tmpdir}"/config.toml
"${soci_snapshotter_grpc}" config default > "${tmpconfig}"
diff -q "${tmpconfig}" "${config_loc}" || (printf "\n\nThe default config seems to have changed. Please run 'make gen-config' to re-generate default config in %s.\n\n" "${config_loc}"; rm -rf "${tmpdir}"; exit 1)
rm -rf "${tmpdir}"
