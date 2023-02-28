# Getting Started With soci-snapshotter

This document walks through how to use soci-snapshotter, including building SOCI
index, pushing/pulling an image and associated SOCI index, and running a container
with soci-snapshotter.

<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->

- [Prerequisites](#prerequisites)
- [Install soci-snapshotter](#install-soci-snapshotter)
- [Push an image to your registry](#push-an-image-to-your-registry)
- [Create and push SOCI index](#create-and-push-soci-index)
  - [Create SOCI index](#create-soci-index)
  - [(Optional) Inspect SOCI index and ztoc](#optional-inspect-soci-index-and-ztoc)
  - [Push SOCI index to registry](#push-soci-index-to-registry)
- [Run container with soci-snapshotter](#run-container-with-soci-snapshotter)
  - [Configure containerd](#configure-containerd)
  - [Start soci-snapshotter](#start-soci-snapshotter)
  - [Lazily pull image](#lazily-pull-image)
  - [Run container](#run-container)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

## Prerequisites

- **[go](https://go.dev/doc/install) >= 1.18** - to confirm please check with `go version`.
- **[containerd](https://github.com/containerd/containerd/blob/main/docs/getting-started.md) >= 1.4** - to confirm that you have containerd working please check with
`sudo ctr version`.
- **[ctr](https://github.com/containerd/containerd/blob/main/docs/getting-started.md)/[nerdctl](https://github.com/containerd/nerdctl#install)** - you need one of the containerd clients to interact with containerd/registry.
- **fuse** - used for mounting without root access (`sudo yum install fuse`).
- **zlib** - used for decompression and ztoc creation (`sudo yum install zlib-devel`).
- **gcc** - used for compiling SOCI's c code, gzip's zinfo implementation (`sudo yum install gcc`).

## Install soci-snapshotter

The soci-snapshotter project consists of 2 main components:

- `soci`: the CLI tool used to build/manage SOCI indices.
- `soci-snapshotter-grpc`: the daemon (a containerd snapshotter plugin) used for lazy loading.

Currently to get the binaries, we need to build the project from source after cloing the repo:

```shell
git clone https://github.com/awslabs/soci-snapshotter.git
cd soci-snapshotter
make
```

This builds the project binaries into the `./out` directory. You can install them
to a `PATH` directory (`/usr/local/bin`) with:

```shell
sudo make install
# check soci can be found in PATH
sudo soci --help
```

Many `soci` CLI commands need to be run as `sudo`, because the metadata is saved
in directories that a non-root user often does not have access to.

> This doc assumes SOCI binaries are installed into `PATH`. If not, please use
> the full path of the binaries (e.g. `./out/soci`).

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
sudo ctr i pull docker.io/library/rabbitmq:latest
sudo ctr i tag docker.io/library/rabbitmq:latest $REGISTRY/rabbitmq:latest
sudo ctr i push --user $REGISTRY_USER:$REGISTRY_PASSWORD --platform linux/amd64 $REGISTRY/rabbitmq:latest
```

After this step, please check your registry to confirm the image is pushed to it.
You can go to your registry console or use your registry's CLI (e.g. for ECR, you
can use `aws ecr describe-images --repository-name rabbitmq --region $AWS_REGION`).

## Create and push SOCI index

Instead of converting the image format, soci-snapshotter uses the SOCI index
associated with an image to implement its lazy loading. For more detail
please see [README](../README.md#no-image-conversion).

### Create SOCI index

Let's create a SOCI index, which later will be pushed to your registry:

```shell
sudo soci create $REGISTRY/rabbitmq:latest

# output
layer sha256:57315aaee690b22265ebb83b5443587443398a7cd99dd2a43985c28868d34053 -> ztoc skipped
layer sha256:ed46dea0429646ca97e7a90d273159154ab8c28e631f2582d32713e584d98ace -> ztoc skipped
layer sha256:3f0e404c1d688448c1c3947d91d6e0926c67212f4d647369518077513ebdfd91 -> ztoc skipped
layer sha256:626e07084b41a102f8bcedf05172676423d1c37b8391be76eee2d7bbf56ec31e -> ztoc skipped
layer sha256:b49348aba7cfd44d33b07730fd8d3b44ac97d16a268f2d74f7bfb78c4c9d1ff7 -> ztoc skipped
layer sha256:ec66df5c883fd24406c6ef53864970f628b51216e8e1f3f5981c439ed6e4ed41 -> ztoc skipped
layer sha256:8147f1b064ec70039aad0068f71b316b42cf515d2ba87e6668cb66de4f042f5a -> ztoc skipped
layer sha256:f63218e95551afe34f3107b1769a556a3c9a39279cb66979914215e03f4e2754 -> ztoc sha256:ccae6b7217b73ae9caf80bff4c5411dada341739c8b443791fba227b226c61d0
layer sha256:7608715873ec5c02d370e963aa9b19a149023ce218887221d93fe671b3abbf58 -> ztoc sha256:740374aa7cac1764593430843d428a73a30d4a6a0d45fb171c369f3914a638eb
layer sha256:96fb4c28b2c1fc1528bf053e2938d5173990eb12097d51f66c2bb3d01a2c9a39 -> ztoc sha256:dc9a2ca27d2b680279fc8052228772b9c03a779d0b7cc61012d2ad833ad1ff5e
...
```

Behind the scene SOCI created two kinds of objects. One is a series of ztocs
(one per layer). A ztoc is a table of contents for compressed data. The other is
a manifest that relates the ztocs to their corresponding image layers and relates
the entire SOCI index to a particular image manifest (i.e. a particular image for a particular platform).

> We skip building ztocs for smaller layers (controlled by `--min-layer-size` of
> `soci create`) because small layers don't benefit much from lazy loading.)

From the above output, we can see that SOCI creates ztocs for 3 layers and skips
7 layers, which means only the 3 layers with ztocs will be lazily pulled.

### (Optional) Inspect SOCI index and ztoc

We can inspect one of these ztoc's from the output of previous command (replace
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

### Push SOCI index to registry

Next we need to push the manifest to the registry with the following command.
This will push all of the SOCI related artifacts (index manifest, ztoc):

```shell
sudo soci push --user $REGISTRY_USER:$REGISTRY_PASSWORD $REGISTRY/rabbitmq:latest
```

## Run container with soci-snapshotter

### Configure containerd

We need to reconfigure and restart containerd to enable soci-snapshotter. This
section assume your containerd is managed by `systemd`. First let's stop containerd:

```shell
sudo systemctl stop containerd
```

Next we need to modify containerd's config file (`/etc/containerd/config.toml`).
Let's add the following config to the file to enable soci-snapshotter as a plugin:

```toml
[proxy_plugins]
  [proxy_plugins.soci]
    type = "snapshot"
    address = "/run/soci-snapshotter-grpc/soci-snapshotter-grpc.sock"
```

This config section tells containerd that there is a snapshot plugin named `soci`
and to communicate with it via a socket file.

Now let's restart containerd and confirm containerd knows about soci-snapshotter plugin:

```shell
sudo systemctl restart containerd
sudo ctr plugin ls id==soci
```

`ctr plugin ls` lists all of the plugins from which you should see there is a
`soci` plugin of type `io.containerd.snapshotter.v1`.

### Start soci-snapshotter

First we need to start the snapshotter grpc service binary (`soci-snapshotter-grpc`).
Here we start the binary in background and simply redirect logs to an arbitrary file:

```shell
sudo soci-snapshotter-grpc &> ~/soci-snapshotter-logs &
```

Alternately, you can split up stdout (json logs) and stderr (plain text errors):

```shell
sudo soci-snapshotter-grpc 2> ~/soci-snapshotter-errors 1> ~/soci-snapshotter-logs &
```

### Lazily pull image

Once the snapshotter is running we can call the `rpull` command from SOCI CLI.
This command reads the manfiest from the registry and mounts FUSE filesystems
for each layer.

> The optional flag `--soci-index-digest` needs to be the digest of the SOCI index manifest.
> If not provided, the snapshotter will use the OCI distribution-spec's [Referrers API](https://github.com/opencontainers/distribution-spec/blob/main/spec.md#listing-referrers)
> (if available, otherwise the spec's [fallback mechanism](https://github.com/opencontainers/distribution-spec/blob/main/spec.md#unavailable-referrers-api)) to fetch a list of available indices.

```shell
sudo soci image rpull --user $REGISTRY_USER:$REGISTRY_PASSWORD --soci-index-digest sha256:f5f2a8558d0036c0a316638c5575607c01d1fa1588dbe56c6a5a7253e30ce107 $REGISTRY/rabbitmq:latest

# output
fetching sha256:a9072496... application/vnd.docker.distribution.manifest.v2+json
fetching sha256:4027609f... application/vnd.docker.container.image.v1+json
fetching sha256:9e3ecea6... application/vnd.docker.image.rootfs.diff.tar.gzip
fetching sha256:32a25b60... application/vnd.docker.image.rootfs.diff.tar.gzip
fetching sha256:7ed5ffe2... application/vnd.docker.image.rootfs.diff.tar.gzip
fetching sha256:e02f149b... application/vnd.docker.image.rootfs.diff.tar.gzip
fetching sha256:9f1239da... application/vnd.docker.image.rootfs.diff.tar.gzip
fetching sha256:0d64e0d6... application/vnd.docker.image.rootfs.diff.tar.gzip
fetching sha256:6d7e0f86... application/vnd.docker.image.rootfs.diff.tar.gzip
```

After running this command you will see a minimal output as the example, because
with lazy pulling, not all layers are pulled during the `pull` step. From
previous step we created 3 ztocs for 3 layers and skipped 7 layers. Here we see
exactly 7 layers are pulled during `pull` step and not the 3 layers with ztocs.

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
command in ctr.  We need to specify which snapshotter we shall use and we will
use the `--net-host` binary flag.  Then we pass in the two main arguments, our
image registry and the id of the container:

```shell
sudo ctr run --user $REGISTRY_USER:$REGISTRY_PASSWORD --snapshotter soci --net-host $REGISTRY/rabbitmq:latest sociExample
```
