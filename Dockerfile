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

ARG CONTAINERD_VERSION=v1.6.4
ARG RUNC_VERSION=v1.1.1
ARG NERDCTL_VERSION=0.19.0

FROM golang:1.20-buster AS golang-base

FROM golang-base AS containerd-snapshotter-base
ARG CONTAINERD_VERSION
ARG RUNC_VERSION
ARG NERDCTL_VERSION
COPY . $GOPATH/src/github.com/awslabs/soci-snapshotter
ENV GOPROXY direct
RUN apt-get update -y && apt-get install -y libbtrfs-dev libseccomp-dev libz-dev gcc fuse && \
    git clone -b ${CONTAINERD_VERSION} --depth 1 \
              https://github.com/containerd/containerd $GOPATH/src/github.com/containerd/containerd && \
    cd $GOPATH/src/github.com/containerd/containerd && \
    GO111MODULE=off make && DESTDIR=/out/ make install && \
    cp $GOPATH/src/github.com/containerd/containerd/bin/* /usr/local/bin/
RUN cd $GOPATH/src/github.com/awslabs/soci-snapshotter && \
    PREFIX=/out/ GO111MODULE=on make && \
    cp $GOPATH/src/github.com/awslabs/soci-snapshotter/out/soci /usr/local/bin/ && \
    cp $GOPATH/src/github.com/awslabs/soci-snapshotter/out/soci-snapshotter-grpc /usr/local/bin/ && \
    mkdir /etc/soci-snapshotter-grpc && \
    mkdir /etc/containerd/ && \
    cp $GOPATH/src/github.com/awslabs/soci-snapshotter/integration/config/etc/soci-snapshotter-grpc/config.toml /etc/soci-snapshotter-grpc/ && \
    cp $GOPATH/src/github.com/awslabs/soci-snapshotter/integration/config/etc/containerd/config.toml /etc/containerd/ 
RUN git clone -b ${RUNC_VERSION} --depth 1 \
              https://github.com/opencontainers/runc $GOPATH/src/github.com/opencontainers/runc && \
    cd $GOPATH/src/github.com/opencontainers/runc && \
    GO111MODULE=off make && make install PREFIX=/out/ && \
    cp /out/sbin/* /usr/local/sbin/ && \
    curl -sSL --output /tmp/nerdctl.tgz https://github.com/containerd/nerdctl/releases/download/v${NERDCTL_VERSION}/nerdctl-${NERDCTL_VERSION}-linux-${TARGETARCH:-amd64}.tar.gz && \
    tar zxvf /tmp/nerdctl.tgz -C /usr/local/bin/ && \
    rm -f /tmp/nerdctl.tgz
