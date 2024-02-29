#!/bin/bash

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

# A script to generate release artifacts.
# This will create a folder in your project root called release.
# This will contain the dynamic + static binaries
# as well as their respective sha256 checksums.
# NOTE: this will mutate your $SOCI_SNAPSHOTTER_PROJECT_ROOT/out folder.

set -eux -o pipefail

CUR_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
SOCI_SNAPSHOTTER_PROJECT_ROOT="$(cd -- "$CUR_DIR"/.. && pwd)"
OUT_DIR="${SOCI_SNAPSHOTTER_PROJECT_ROOT}/out"
RELEASE_DIR="${SOCI_SNAPSHOTTER_PROJECT_ROOT}/release"
LICENSE_FILE=${SOCI_SNAPSHOTTER_PROJECT_ROOT}/THIRD_PARTY_LICENSES
NOTICE_FILE=${SOCI_SNAPSHOTTER_PROJECT_ROOT}/NOTICE.md
TAG_REGEX="v[0-9]+.[0-9]+.[0-9]+"

ARCH=""
case $(uname -m) in
    x86_64) ARCH="amd64" ;;
    aarch64) ARCH="arm64" ;;
    *) echo "Error: Unsupported arch"; exit 1 ;;
esac

if [ "$#" -ne 1 ]; then
    echo "Expected 1 parameter, got $#."
    echo "Usage: $0 [release_tag]"
    exit 1
fi

if ! [[ "$1" =~ $TAG_REGEX ]]; then
    echo "Improper tag format. Format should match regex $TAG_REGEX"
    exit 1
fi

if [ -d "$RELEASE_DIR" ]; then
    rm -rf "${RELEASE_DIR:?}"/*
else
    mkdir "$RELEASE_DIR"
fi

release_version=${1/v/} # Remove v from tag name
dynamic_binary_name=soci-snapshotter-${release_version}-linux-${ARCH}.tar.gz
static_binary_name=soci-snapshotter-${release_version}-linux-${ARCH}-static.tar.gz

make build
cp "$NOTICE_FILE" "$LICENSE_FILE" "${OUT_DIR}"
pushd "$OUT_DIR"
tar -czvf "$RELEASE_DIR"/"$dynamic_binary_name" -- *
popd
rm -rf "{$OUT_DIR:?}"/*

STATIC=1 make build
cp "$NOTICE_FILE" "$LICENSE_FILE" "$OUT_DIR"
pushd "$OUT_DIR"
tar -czvf "$RELEASE_DIR"/"$static_binary_name" -- *
popd
rm -rf "{$OUT_DIR:?}"/*

pushd "$RELEASE_DIR"
sha256sum "$dynamic_binary_name" > "$RELEASE_DIR"/"$dynamic_binary_name".sha256sum
sha256sum "$static_binary_name" > "$RELEASE_DIR"/"$static_binary_name".sha256sum
popd
