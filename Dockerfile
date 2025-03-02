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

ARG CONTAINERD_VERSION=1.6.36
ARG RUNC_VERSION=1.2.5
ARG NERDCTL_VERSION=1.7.1

FROM public.ecr.aws/docker/library/registry:3.0.0-alpha.1 AS registry

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
