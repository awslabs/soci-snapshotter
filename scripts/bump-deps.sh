#!/bin/bash

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

pushd ${SOCI_SNAPSHOTTER_PROJECT_ROOT}

# skip k8s deps since they use the latest go version/features that may not be in the go version soci uses
# skip grpc because it's not compatible with containerd 1.7
# Also ignored in /dependabot.yml
go get $(go list -m -f '{{if not (or .Indirect .Main)}}{{.Path}}{{end}}' all | \
    grep -v "^google.golang.org/grpc" | \
    grep -v "^k8s.io/")
make vendor

pushd ./cmd
# skip k8s deps and soci-snapshotter itself
# skip grpc because it's not compatible with containerd 1.7
# Also ignored in /dependabot.yml
go get $(go list -m -f '{{if not (or .Indirect .Main)}}{{.Path}}{{end}}' all | \
    grep -v "^github.com/awslabs/soci-snapshotter" | \
    grep -v "^google.golang.org/grpc" | \
    grep -v "^k8s.io/")
popd
make vendor

popd