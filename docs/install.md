# Install soci-snapshotter

This doc walks through how to install soci-snapshotter as a component managed by systemd.

The soci-snapshotter project produces 2 binaries:

- `soci`: the CLI tool used to build/manage SOCI indices.
- `soci-snapshotter-grpc`: the daemon (a containerd snapshotter plugin) used for lazy loading.

You can get the prebuilt binaries from our [release page](https://github.com/awslabs/soci-snapshotter/releases)
or [build them from source](./build.md).

<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->

- [Dependencies](#dependencies)
- [Configure soci-snapshotter (optional)](#configure-soci-snapshotter-optional)
- [Install soci-snapshotter for containerd with systemd](#install-soci-snapshotter-for-containerd-with-systemd)
- [Config containerd](#config-containerd)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

## Dependencies

soci-snapshotter has the following dependencies. Please follow the links or commands
to install them on your machine:

> We only mention the direct dependencies of the project. Some dependencies may
> have their own dependencies (e.g., containerd depends on runc/cni). Please refer
> to their doc for a complete installation guide (mainly containerd).

- **[containerd](https://github.com/containerd/containerd/blob/main/docs/getting-started.md) >= 1.4** -
required to run soci-snapshotter; to confirm please check with `sudo containerd --version`.
- **fuse** - used for mounting without root access (`sudo yum install fuse`).

For fuse/zlib, they can be installed by your Linux package manager (e.g., `yum` or `apt-get`).

## Configure soci-snapshotter (optional)

Similar to containerd, soci-snapshotter has a toml config file which is located at
`/etc/soci-snapshotter-grpc/config.toml` by default. If such a file doesn't exist,
soci-snapshotter will use default values for all configurations.

> Whenever you make changes to the config file, you need to stop the snapshotter
> first before making changes, and restart the snapshotter after the changes.

## Install soci-snapshotter for containerd with systemd

If you plan to use systemd to manage your soci-snapshotter process, you can download
the [`soci-snapshotter.service` unit file](../soci-snapshotter.service) in the
repository root directory into `/usr/local/lib/systemd/system/soci-snapshotter.service`,
and run the following commands:

```shell
sudo systemctl daemon-reload
sudo systemctl enable --now soci-snapshotter
```

To validate soci-snapshotter is running, let's check the snapshotter's version.
The output should show the version that you installed.

```shell
$ sudo soci-snapshotter-grpc --version
soci-snapshotter-grpc version f855ff1.m f855ff1bcf7e161cf0e8d3282dc3d797e733ada0.m
```

## Config containerd

We need to configure and restart containerd to enable soci-snapshotter (this
section assume your containerd is also managed by `systemd`):

- Stop containerd: `sudo systemctl stop containerd`;
- Update containerd config to include soci-snapshotter plugin. The config file
is usually in `/etc/containerd/config.toml`, and you need to add the following:

```toml
[proxy_plugins]
  [proxy_plugins.soci]
    type = "snapshot"
    address = "/run/soci-snapshotter-grpc/soci-snapshotter-grpc.sock"
```

- Restart containerd: `sudo systemctl restart containerd`;
- (Optional) Check soci-snapshotter is recognized by containerd: `sudo ctr plugin ls id==soci`.
You will see output like below. If not, consult containerd logs to determine the cause
or reach out on [our discussion](https://github.com/awslabs/soci-snapshotter/discussions).

```shell
TYPE                            ID      PLATFORMS    STATUS
io.containerd.snapshotter.v1    soci    -            ok
```
