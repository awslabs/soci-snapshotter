#!/usr/bin/env bash

#   Copyright The containerd Authors.

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
soci_snapshotter_project_root="${cur_dir}/.."
ztoc_fbs_dir="${soci_snapshotter_project_root}"/ztoc/fbs
ztoc_fbs_file="${ztoc_fbs_dir}"/ztoc.fbs
compression_fbs_dir="${soci_snapshotter_project_root}"/ztoc/compression/fbs
compression_fbs_file="${compression_fbs_dir}"/zinfo.fbs

# check if ztoc flatbuffers needs to be generated again
tmpdir=$(mktemp -d)
flatc -o "${tmpdir}" -g "${ztoc_fbs_file}"
diff -qr "${tmpdir}/ztoc" "${ztoc_fbs_dir}/ztoc" || (printf "\n\nThe Ztoc schema seems to be modified. Please run 'make flatc' to re-generate Go files\n\n"; rm -rf "${tmpdir}"; exit 1)
rm -rf "${tmpdir}"

# check if zinfo flatbuffers needs to be generated again
tmpdir=$(mktemp -d)
flatc -o "${tmpdir}" -g "${compression_fbs_file}"
diff -qr "${tmpdir}/zinfo" "${compression_fbs_dir}/zinfo" || (printf "\n\nThe Zinfo schema seems to be modified. Please run 'make flatc' to re-generate Go files\n\n"; rm -rf "${tmpdir}"; exit 1)
rm -rf "${tmpdir}"
