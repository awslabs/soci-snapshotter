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

ARG CONTAINERD_VERSION=1.6.38
ARG RUNC_VERSION=1.2.5
ARG NERDCTL_VERSION=1.7.1
ARG IGZIP_VERSION=2.31.1
ARG RAPIDGZIP_VERSION=0.14.3

FROM public.ecr.aws/docker/library/registry:3.0.0 AS registry

# Build stage for Intel ISA-L (igzip)
FROM public.ecr.aws/amazonlinux/amazonlinux:2023 AS igzip-builder

ARG IGZIP_VERSION

RUN dnf update -y && dnf install -y \
    autoconf \
    automake \
    gcc \
    gcc-c++ \
    git \
    libtool \
    make \
    nasm \
    yasm

RUN git clone https://github.com/intel/isa-l.git && \
    cd isa-l && \
    git checkout "v${IGZIP_VERSION}" && \
    ./autogen.sh && \
    # Configure with static libraries only
    ./configure --enable-static --disable-shared && \
    make && \
    make install DESTDIR=/opt/igzip && \
    # No need for ld.so.conf.d with static libraries
    cd .. && \
    rm -rf isa-l

# Build stage for rapidgzip
FROM public.ecr.aws/amazonlinux/amazonlinux:2023 AS rapidgzip-builder

ARG RAPIDGZIP_VERSION
ARG TARGETARCH

RUN dnf update -y && dnf install -y \
    binutils \
    cmake \
    gcc \
    gcc-c++ \
    git \
    make \
    nasm \
    yasm \
    zlib-devel

RUN mkdir -p /opt/rapidgzip/usr/local/bin

RUN git clone https://github.com/mxmlnkn/rapidgzip.git && \
    cd rapidgzip && \
    git checkout "rapidgzip-v${RAPIDGZIP_VERSION}" && \
    if [ "$TARGETARCH" = "arm64" ] || [ "$(uname -m)" = "aarch64" ]; then \
        # Disable ISA-L on ARM due to linking errors.
        export ISAL_FLAGS="-DWITH_ISAL=OFF"; \

        # Disable fcf-protection which is not supported on ARM.
        sed -i 's/-fcf-protection=full/-fcf-protection=none/g' CMakeLists.txt; \
    else \
        export ISAL_FLAGS=""; \
    fi && \
    mkdir build && \
    cd build && \
    cmake -DCMAKE_DISABLE_FIND_PACKAGE_NASM=TRUE \
          -DCMAKE_CXX_FLAGS="-O2 -fPIC" \
          -DCMAKE_C_FLAGS="-O2 -fPIC" \
          ${ISAL_FLAGS} \
          -DCMAKE_BUILD_TYPE=Release .. && \
    make -j$(nproc) && \
    cp src/tools/rapidgzip /opt/rapidgzip/usr/local/bin/ && \
    chmod +x /opt/rapidgzip/usr/local/bin/rapidgzip && \
    cd ../.. && \
    rm -rf rapidgzip

FROM public.ecr.aws/amazonlinux/amazonlinux:2023 AS containerd-snapshotter-base

ARG CONTAINERD_VERSION
ARG RUNC_VERSION
ARG NERDCTL_VERSION
ARG TARGETARCH
ENV GOPROXY direct
ENV GOCOVERDIR /test_coverage

COPY ./integ_entrypoint.sh /integ_entrypoint.sh
COPY . $GOPATH/src/github.com/awslabs/soci-snapshotter
RUN dnf update && dnf upgrade && dnf install -y \
    diffutils \
    findutils \
    gzip \
    iptables \
    pigz \
    procps \
    systemd \
    tar \
    util-linux-core

# Copy igzip and rapidgzip from builder stages
COPY --from=igzip-builder /opt/igzip/usr /usr/local
COPY --from=rapidgzip-builder /opt/rapidgzip/usr/local /usr/local

RUN cp $GOPATH/src/github.com/awslabs/soci-snapshotter/out/soci /usr/local/bin/ && \
    cp $GOPATH/src/github.com/awslabs/soci-snapshotter/out/soci-snapshotter-grpc /usr/local/bin/ && \
    mkdir /etc/soci-snapshotter-grpc && \
    mkdir /etc/containerd/ && \
    cp $GOPATH/src/github.com/awslabs/soci-snapshotter/integration/config/etc/soci-snapshotter-grpc/config.toml /etc/soci-snapshotter-grpc/ && \
    cp $GOPATH/src/github.com/awslabs/soci-snapshotter/integration/config/etc/containerd/config.toml /etc/containerd/ && \
    cp $GOPATH/src/github.com/awslabs/soci-snapshotter/soci-snapshotter.service /etc/systemd/system && \
    cp $GOPATH/src/github.com/awslabs/soci-snapshotter/soci-snapshotter.socket /etc/systemd/system
RUN curl -sSL --output /tmp/containerd.tgz https://github.com/containerd/containerd/releases/download/v${CONTAINERD_VERSION}/containerd-${CONTAINERD_VERSION}-linux-${TARGETARCH:-amd64}.tar.gz && \
    tar zxvf /tmp/containerd.tgz -C /usr/local/ && \
    rm -f /tmp/containerd.tgz && \
    cp $GOPATH/src/github.com/awslabs/soci-snapshotter/out/nerdctl-with-idmapping /usr/local/bin && \
    chmod +x /usr/local/bin/nerdctl-with-idmapping
RUN curl -sSL --output /tmp/runc https://github.com/opencontainers/runc/releases/download/v${RUNC_VERSION}/runc.${TARGETARCH:-amd64} && \
    cp /tmp/runc /usr/local/bin/ && \
    chmod +x /usr/local/bin/runc && \
    rm -f /tmp/runc
RUN curl -sSL --output /tmp/nerdctl.tgz https://github.com/containerd/nerdctl/releases/download/v${NERDCTL_VERSION}/nerdctl-${NERDCTL_VERSION}-linux-${TARGETARCH:-amd64}.tar.gz && \
    tar zxvf /tmp/nerdctl.tgz -C /usr/local/bin/ && \
    rm -f /tmp/nerdctl.tgz
