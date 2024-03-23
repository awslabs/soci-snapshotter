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

CUR_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
SOCI_SNAPSHOTTER_PROJECT_ROOT="${CUR_DIR}/.."
FBS_FILE_PATH=${SOCI_SNAPSHOTTER_PROJECT_ROOT}/ztoc/fbs/ztoc.fbs

# check if flatbuffers needs to be generated again
TMPDIR=$(mktemp -d)
flatc -o "${TMPDIR}" -g "${FBS_FILE_PATH}"
diff -qr "${TMPDIR}/ztoc" "${SOCI_SNAPSHOTTER_PROJECT_ROOT}/ztoc/fbs/ztoc" || (printf "\n\nThe Ztoc schema seems to be modified. Please run 'make flatc' to re-generate Go files\n\n"; rm -rf "${TMPDIR}"; exit 1)
rm -rf "${TMPDIR}"
