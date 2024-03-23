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

# install cmake
if ! command -v cmake &> /dev/null
then
    wget https://github.com/Kitware/CMake/releases/download/v3.24.1/cmake-3.24.1-Linux-x86_64.sh -O cmake.sh
    sh cmake.sh --prefix=/usr/local/ --exclude-subdir
    rm -rf cmake.sh
else
    echo "cmake is installed, skip..."
fi

# install flatc
if ! command -v flatc &> /dev/null
then
    wget https://github.com/google/flatbuffers/archive/refs/tags/v2.0.8.tar.gz -O flatbuffers.tar.gz
    tar xzvf flatbuffers.tar.gz
    cd flatbuffers-2.0.8 && cmake -G "Unix Makefiles" -DCMAKE_BUILD_TYPE=Release && make && sudo make install && cd ..
    rm -f flatbuffers.tar.gz
    rm -rf flatbuffers-2.0.8
else
    echo "flatc is installed, skip..."
fi

# install-zlib
wget https://zlib.net/fossils/zlib-1.2.12.tar.gz
tar xzvf zlib-1.2.12.tar.gz
cd zlib-1.2.12 && ./configure && sudo make install && cd ..
rm -rf zlib-1.2.12
rm -f zlib-1.2.12.tar.gz

popd
