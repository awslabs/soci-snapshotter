# Getting Started With the SOCI Snapshotter

This document walks through how to use the SOCI snapshotter, including building a SOCI
index, pushing/pulling an image and associated SOCI index, and running a container
with the SOCI snapshotter.

<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->

- [Dependencies](#dependencies)
- [Install the SOCI snapshotter](#install-the-soci-snapshotter)
- [Push an image to your registry](#push-an-image-to-your-registry)
  - [About the SOCI index](#about-the-soci-index)
  - [(Optional) Inspect SOCI index and zTOC](#optional-inspect-soci-index-and-ztoc)
- [Run container with the SOCI snapshotter](#run-container-with-the-soci-snapshotter)
  - [Configure containerd](#configure-containerd)
  - [Start the SOCI snapshotter](#start-the-soci-snapshotter)
  - [Lazily pull image](#lazily-pull-image)
  - [Run container](#run-container)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

## Dependencies

The SOCI snapshotter has the following runtime dependencies. Please follow the links or commands
to install them on your machine:

> **Note**
> We only mention the direct dependencies of the project. Some dependencies may
> have their own dependencies (e.g., containerd depends on runc/cni). Please refer
> to their doc for a complete installation guide (mainly containerd).

- **[containerd](https://github.com/containerd/containerd/blob/main/docs/getting-started.md) >= 1.4** -
required to run the SOCI snapshotter; to confirm please check with `sudo nerdctl system info`.
- **[nerdctl](https://github.com/containerd/nerdctl#install) >= v1.6.0** - required for this doc to interact with containerd/registry. You do not need any of the additional components mentioned in the install documentation for this getting started, but you might if you want complex networking in the future. Please note that SOCI will not work with rootless nerdctl.
- **fuse** - used for mounting without root access (`sudo yum install fuse` or
other Linux package manager like `apt-get`, depending on your Linux distro).

## Install the SOCI snapshotter

The SOCI project produces 2 binaries:

- `soci`: the CLI tool used to build/manage SOCI indices.
- `soci-snapshotter-grpc`: the daemon (a containerd snapshotter plugin) used for lazy loading.

Note that while the SOCI CLI is never explicitly used, nerdctl (used in this doc) uses it under the hood when given the flag `--snapshotter soci`.

You can download prebuilt binaries from our [release page](https://github.com/awslabs/soci-snapshotter/releases)
or [build them from source](./build.md).

In this doc, let's just download the release binaries and move them to a `PATH`
directory (`/usr/local/bin`):

> You can find other download link in the release page that matches your machine.

```shell
version="0.5.0"
wget https://github.com/awslabs/soci-snapshotter/releases/download/v${version}/soci-snapshotter-${version}-linux-amd64.tar.gz
sudo tar -C /usr/local/bin -xvf soci-snapshotter-${version}-linux-amd64.tar.gz soci soci-snapshotter-grpc
```

Now you should be able to use the `soci` CLI (and `soci-snapshotter-grpc` containerd plugin shortly):

```shell
# check soci can be found in PATH
sudo soci --help
```

Many `soci` CLI commands need to be run as `sudo`, because the metadata is saved
in directories that a non-root user often does not have access to.

## Push an image to your registry

In this document we will use `rabbitmq` from DockerHub `docker.io/library/rabbitmq:latest`.
We use [AWS ECR](https://docs.aws.amazon.com/AmazonECR/latest/userguide/getting-started-console.html)
as the public registry for demonstration. Other OCI 1.0 compatible registries such
as dockerhub should also work.

First let's pull the image from docker into containerd's data store, then (tag and)
push it up to your registry:

> The example assumes you have created an ECR repository called `rabbitmq` and
> have credentials available to the AWS CLI. You just need to update `AWS_ACCOUNT` and `AWS_REGION`.
>
> If you are using a different registry, you will need to set `REGISTRY` and `REGISTRY_USER/REGISTRY_PASSWORD` appropriately
> (and the `rabbitmq` repository is created or can be created automatically while pushing).
>
> The platform tag might be different depending on your machine.

```shell
export AWS_ACCOUNT=000000000000
export AWS_REGION=us-east-1
export REGISTRY_USER=AWS
export REGISTRY_PASSWORD=$(aws ecr get-login-password --region $AWS_REGION)
export REGISTRY=$AWS_ACCOUNT.dkr.ecr.$AWS_REGION.amazonaws.com
# needed for pushing images / SOCI indexes which run as the current user
echo $REGISTRY_PASSWORD | nerdctl login -u $REGISTRY_USER --password-stdin $REGISTRY
# needed the SOCI snapshotter which runs as root
echo $REGISTRY_PASSWORD | sudo nerdctl login -u $REGISTRY_USER --password-stdin $REGISTRY
sudo nerdctl pull docker.io/library/rabbitmq:latest
sudo nerdctl image tag docker.io/library/rabbitmq:latest $REGISTRY/rabbitmq:latest
sudo nerdctl push --platform linux/amd64 --snapshotter soci $REGISTRY/rabbitmq:latest
```

Instead of converting the image format, the SOCI snapshotter uses the SOCI index
associated with an image to implement its lazy loading. (For more details
please see [README](../README.md#no-image-conversion).)
Upon pushing with nerdctl, the `--snapshotter soci` flag causes it to
create a SOCI index and manifest before pushing all associated files to the registry
(the original image, the SOCI index, and manifest).

After this step, please check your registry to confirm the image and SOCI index are present.
You can go to your registry console or use your registry's CLI (e.g. for ECR, you
can use `aws ecr describe-images --repository-name rabbitmq --region $AWS_REGION`).

### About the SOCI index

Behind the scene SOCI created two kinds of objects. One is a series of ztocs
(one per layer). A ztoc is a table of contents for compressed data. The other is
a manifest that relates the ztocs to their corresponding image layers and relates
the entire SOCI index to a particular image manifest (i.e. a particular image for a particular platform).

> We skip building ztocs for smaller layers (controlled by `--soci-min-layer-size` in
> `nerdctl push`) because small layers don't benefit much from lazy loading.)
>
> When all layers are smaller than `min-layer-size`, soci CLI would fail.

From the above output, we can see that SOCI creates ztocs for 3 layers and skips
7 layers, which means only the 3 layers with ztocs will be lazily pulled.

### (Optional) Inspect SOCI index and zTOC

We can inspect one of these ztocs from the output of previous command (replace
the digest with one in your command output). This command will print the ztoc,
which contains all of the information that SOCI needs to find a given file in the layer:

```shell
sudo soci ztoc info sha256:4c1d63f476d4907e0db42b8736f578e79432a28d304935708c918c95e0e4df00
```

We can also view the SOCI index manifests. This command list out all of our index manifests:

```shell
sudo soci index list
```

To inspect an individual SOCI index, we can use the following command, which dump
out the index manifest in json:

```shell
sudo soci index info sha256:f5f2a8558d0036c0a316638c5575607c01d1fa1588dbe56c6a5a7253e30ce107
```

## Run container with the SOCI snapshotter

### Configure containerd

We need to reconfigure and restart containerd to enable the SOCI snapshotter. This
section assume your containerd is managed by `systemd`. First let's stop containerd:

```shell
sudo systemctl stop containerd
```

Next we need to modify containerd's config file (`/etc/containerd/config.toml`).
Let's add the following config to the file to enable the SOCI snapshotter as a plugin:

```toml
[proxy_plugins]
  [proxy_plugins.soci]
    type = "snapshot"
    address = "/run/soci-snapshotter-grpc/soci-snapshotter-grpc.sock"
```

This config section tells containerd that there is a snapshot plugin named `soci`
and to communicate with it via a socket file.

Now let's restart containerd and confirm containerd knows about the SOCI snapshotter plugin:

```shell
sudo systemctl restart containerd
sudo nerdctl system info
```

You should see `soci` under Server -> Plugins -> Storage

### Start the SOCI snapshotter

First we need to start the snapshotter grpc service by running the `soci-snapshotter-grpc` binary in background and simply redirecting logs to an arbitrary file:

```shell
sudo soci-snapshotter-grpc &> ~/soci-snapshotter-logs &
```

Alternately, you can split up stdout (json logs) and stderr (plain text errors):

```shell
sudo soci-snapshotter-grpc 2> ~/soci-snapshotter-errors 1> ~/soci-snapshotter-logs &
```

### Lazily pull image

Once the snapshotter is running we can call the `pull` command from nerdctl.
This command reads the manifest from the registry and mounts a FUSE filesystem
for each layer.

> The snapshotter will use the OCI distribution-spec's [Referrers API](https://github.com/opencontainers/distribution-spec/blob/main/spec.md#listing-referrers)
> (if available, otherwise the spec's [fallback mechanism](https://github.com/opencontainers/distribution-spec/blob/main/spec.md#unavailable-referrers-api)) to fetch a list of available indices.

```shell
sudo nerdctl pull --snapshotter soci $REGISTRY/rabbitmq:latest

#output
$Registry/rabbitmq:latest:   resolved       |++++++++++++++++++++++++++++++++++++++|
manifest-sha256:a9072496...: done           |++++++++++++++++++++++++++++++++++++++|
config-sha256:4027609f...:   done           |++++++++++++++++++++++++++++++++++++++|
elapsed: 9.8 s               total:  10.3 K (1.1 KiB/s)
```

After running this command you will see a minimal output as the example, because
with lazy pulling, not all layers are pulled during the `pull` step. From
previous step we created 3 ztocs for 3 layers.

Now let's check the mounts for the FUSE filesystems. There should be one mount per layer
for layers with ztoc. In our rabbitmq example, there should be 3 mounts.

```shell
mount | grep fuse

# output
fusectl on /sys/fs/fuse/connections type fusectl (rw,relatime)
/home/ec2-user/code/soci-snapshotter/soci on /var/lib/soci-snapshotter-grpc/snapshotter/snapshots/57/fs type fuse.rawBridge (rw,nodev,relatime,user_id=0,group_id=0,allow_other)
/home/ec2-user/code/soci-snapshotter/soci on /var/lib/soci-snapshotter-grpc/snapshotter/snapshots/60/fs type fuse.rawBridge (rw,nodev,relatime,user_id=0,group_id=0,allow_other)
/home/ec2-user/code/soci-snapshotter/soci on /var/lib/soci-snapshotter-grpc/snapshotter/snapshots/62/fs type fuse.rawBridge (rw,nodev,relatime,user_id=0,group_id=0,allow_other)
```

### Run container

Now that all of the mounts are set up we can run the image using the following
command in nerdctl.  We need to specify which snapshotter we shall use and we will
use the `--net host` flag.  Then we pass in the two main arguments, our
image registry and the id of the container:

```shell
sudo nerdctl run --snapshotter soci --net host --rm $REGISTRY/rabbitmq:latest
```
