#!/bin/bash

set -ex
set -o pipefail

: "${max_concurrent_downloads:=-1}"
: "${max_concurrent_downloads_per_image:=3}"
: "${concurrent_download_chunk_size:=16mb}"
: "${max_concurrent_unpacks:=-1}"
: "${max_concurrent_unpacks_per_image:=1}"
: "${discard_unpacked_layers:=true}"
: "${soci_version:=0.11.1}"
: "${soci_root_dir:=/var/lib/soci-snapshotter-grpc}"
: "${decompress_library:=/usr/bin/unpigz}"

ARCH=$(uname -m | sed s/aarch64/arm64/ | sed s/x86_64/amd64/)
ARCHIVE=soci-snapshotter-${soci_version}-linux-${ARCH}.tar.gz
pushd /tmp
curl --silent --location --fail --output "${ARCHIVE}" https://github.com/awslabs/soci-snapshotter/releases/download/v"${soci_version}"/"${ARCHIVE}"
curl --silent --location --fail --output "${ARCHIVE}".sha256sum https://github.com/awslabs/soci-snapshotter/releases/download/v"${soci_version}"/"${ARCHIVE}".sha256sum
sha256sum ./"${ARCHIVE}".sha256sum
tar xzvf ./"${ARCHIVE}" -C /usr/local/bin soci-snapshotter-grpc
rm ./"${ARCHIVE}"
rm ./"${ARCHIVE}".sha256sum
mkdir -p /etc/soci-snapshotter-grpc
cat <<EOF >/etc/soci-snapshotter-grpc/config.toml
debug = false
[content_store]
type = "containerd"
[cri_keychain]
enable_keychain = true
image_service_path = "/run/containerd/containerd.sock"
[pull_modes.parallel_pull_unpack]
enable = true
discard_unpacked_layers = $discard_unpacked_layers
max_concurrent_downloads = $max_concurrent_downloads
max_concurrent_downloads_per_image = $max_concurrent_downloads_per_image
concurrent_download_chunk_size = "${concurrent_download_chunk_size}"
max_concurrent_unpacks = $max_concurrent_unpacks
max_concurrent_unpacks_per_image = $max_concurrent_unpacks_per_image
[pull_modes.parallel_pull_unpack.decompress_streams."gzip"]
args = ['-d', '-c']
path = "${decompress_library}"
EOF
cat <<EOF >/etc/systemd/system/soci-snapshotter.service
[Unit]
Description=soci snapshotter containerd plugin
Documentation=https://github.com/awslabs/soci-snapshotter
Before=containerd.service

[Service]
Type=notify
ExecStart=/usr/local/bin/soci-snapshotter-grpc --root ${soci_root_dir} --address fd://
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF
cat <<EOF >/etc/systemd/system/soci-snapshotter.socket
[Unit]
Description=soci snapshotter containerd plugin (socket)
Documentation=https://github.com/awslabs/soci-snapshotter

[Socket]
ListenStream=/run/soci-snapshotter-grpc/soci-snapshotter-grpc.sock
SocketMode=0660

[Install]
WantedBy=sockets.target
EOF
systemctl daemon-reload
systemctl enable --now soci-snapshotter
popd
