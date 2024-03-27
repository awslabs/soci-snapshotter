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

echo "Install soci dependencies"
set -eux -o pipefail

# the installation shouldn't assume the script is executed in a specific directory.
# move to tmp in case there is leftover while installing dependencies.
TMPDIR=$(mktemp -d)
pushd "${TMPDIR}"

arch="$(uname -m)"

# install cmake
cmake_ver="3.24.1"
if ! command -v cmake &> /dev/null
then
    wget https://github.com/Kitware/CMake/releases/download/v"${cmake_ver}"/cmake-"${cmake_ver}"-Linux-"${arch}".sh -O cmake.sh
    wget https://github.com/Kitware/CMake/releases/download/v"${cmake_ver}"/cmake-"${cmake_ver}"-SHA-256.txt -O shasums.txt

    # Verify integrity

    # The sha256sum will come in the format like the following:
    # sha256hash filename
    # Since we rename the script for convenience purposes,
    # we only want to compare hashes to verify file integrity.
    cmake_expected_shasum="$(grep "${arch}".sh < shasums.txt | awk '{print $1}')"
    cmake_actual_shasum="$(sha256sum cmake.sh | awk '{print $1}')"
    if [ "${cmake_expected_shasum}" != "${cmake_actual_shasum}" ]
    then
        echo "error: cmake sha256sum did not match"
        exit 1
    fi
    
    sh cmake.sh --prefix=/usr/local/ --exclude-subdir
    rm -rf cmake.sh
else
    echo "cmake is installed, skip..."
fi

# install flatc
flatc_ver="2.0.8"
flatc_expected_shasum="f97965a727d26386afaefff950badef2db3ab6af9afe23ed6d94bfb65f95f37e"
if ! command -v flatc &> /dev/null
then
    wget https://github.com/google/flatbuffers/archive/refs/tags/v"${flatc_ver}".tar.gz -O flatbuffers.tar.gz

    flatc_actual_shasum="$(sha256sum flatbuffers.tar.gz | awk '{print $1}')"
    if [ "${flatc_expected_shasum}" != "${flatc_actual_shasum}" ]
    then
        echo "error: flatc sha256sum did not match"
        exit 1
    fi

    tar xzvf flatbuffers.tar.gz
    cd flatbuffers-"${flatc_ver}" && cmake -G "Unix Makefiles" -DCMAKE_BUILD_TYPE=Release && make && sudo make install && cd ..
    rm -f flatbuffers.tar.gz
    rm -rf flatbuffers-"${flatc_ver}"
else
    echo "flatc is installed, skip..."
fi

# install-zlib
zlib_ver="1.2.12"
zlib_expected_shasum="91844808532e5ce316b3c010929493c0244f3d37593afd6de04f71821d5136d9"

wget https://zlib.net/fossils/zlib-"${zlib_ver}".tar.gz -O zlib.tar.gz
zmake_actual_shasum="$(sha256sum zlib.tar.gz | awk '{print $1}')"
if [ "${zlib_expected_shasum}" != "${zmake_actual_shasum}" ]
then
    echo "error: zmake sha256sum did not match"
    exit 1
fi

tar xzvf zlib.tar.gz
cd zlib-"${zlib_ver}" && ./configure && sudo make install && cd ..
rm -rf zlib-"${zlib_ver}"
rm -f zlib.tar.gz

popd
