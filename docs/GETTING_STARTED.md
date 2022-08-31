# Getting Started With SOCI Snapshotter
This document is a guide to walk through an example of using the
soci-snapshotter.  To get everything running we will need to do a few things:
- prerequisites
- start a local references-enabled OCI registry
- push an image to our registry using ctr
- create and push a SOCI index to our registry
- run a lazily loaded container from the registry

Along the way we will make sure to inspect each of our actions to ensure that
they worked.  This is valuable to help learn about the tool and debugging.

## Prerequisites
**docker** - to confirm that you have docker working please check with `docker
version` and `docker images` - both should succeed.

**containerd** - to confirm that you have containerd working please check with
`ctr version` and `ctr i ls`.  The version needs to be 1.4 or greater

**ctr** - to confirm that you have ctr, the containerd CLI installed, check with
`ctr version` and `ctr i ls`

**soci-snapshotter** - this you will need to make sure you can build from
source. You should be able to run `make` in the soci-snapshotter dir.
Some common issues:
- getting the github repository `git clone
https://github.com/awslabs/soci-snapshotter.git`
- having the zlib developer tools (zlib is used for decompression and ztoc creation).
Often fixed with `sudo yum install zlib-devel`
- having the proper go version installed, as of this document please use 1.18+

## Start a local ORAS registry
Most OCI registries do not currently support the "referrers" feature.
SOCI relies on this feature to link its indices and manifests.  The [ORAS
project](https://oras.land/) tries to fill this gap; in their words:

> Registries are evolving as generic artifact stores. To enable this goal, the ORAS project provides a way to push and pull OCI Artifacts to and from OCI Registries.

[The longer term solution for this issue is using the OCI Reference
Types](https://github.com/awslabs/soci-snapshotter/issues/42).  For now we are
relying on the ORAS project for the referrers feature.

Given this we want to start our registry using an OCI image from the ORAS 
project.  We will run it on port 5000 as an HTTP (**NOT** HTTPS) server without
certificates.  To do this we will use Docker:

```
docker run -d -p 5000:5000 --restart=always --name registry ghcr.io/oras-project/registry:v1.0.0-rc
```

Since this is just a locally run HTTP server we can interact with
it using standard `curl` commands. To confirm this is running lets
use the following commands:

```
docker ps
```

which should yield a running container at 0.0.0.0:5000 for the ORAS image. Now
to check that we can interact with the server run the following:
```
curl http://localhost:5000/v2/_catalog
```
This should return with `{"repositories":[]}` given that we haven't put anything
in there yet

## Push an image to the registry using ctr
First things first, let's choose our favorite image.  For the purpose of this
document we will use the latest image of rabbitmq from DockerHub, aptly named
`docker.io/library/rabbitmq:latest`.

To do this with ctr use the following command:
```
ctr i pull docker.io/library/rabbitmq:latest
```
To confirm this image (or the image you have chosen) is in containerd's local
content store please run:
```
ctr i ls
```
and you should see the image.  Now we need to add an image reference it so that
it can be added to our repository.
```
ctr i tag docker.io/library/rabbitmq:latest localhost:5000/rabbitmq:latest
```
Note that the `localhost:5000` is the address of our registry, `rabbitmq` will
be the name of the repository, and `latest` will be the tag within the
repository.

To confirm this worked once more run `ctr i ls` and see that there is now a new
line with the tag you just created.  Also confirm that the digest, the long
alphanumeric hash that is prefixed by "sha256:", matches the base image.

Finally, we can push to the local registry:

```
ctr i push --platform linux/amd64 --plain-http localhost:5000/rabbitmq:latest
```

**NOTE**: the platform tag might be different depending what environment you
are working in.

Once again let's confirm via a curl command:
```
curl http://localhost:5000/v2/_catalog
```
We should now see that our repository has been created
`{"repositories":["rabbitmq"]}`.  We can further inspect and see what tags were
created.
```
curl http://localhost:5000/v2/rabbitmq/tags/list
```
We can also look at the details of the tag using:
```
curl http://localhost:5000/v2/rabbitmq/manifests/latest
```
## Create and push a SOCI index
Now that we have confidence that our registry contains the image let's work on
the SOCI index.  This is the pre-built index that allows random access of the
compressed layer. For more detail please see [our
README](https://github.com/awslabs/soci-snapshotter#no-image-conversion).

First off, we need to build the SOCI project using the `make` command.  This
will generate two binaries in the `out` directory.  You should see:
- soci (this is the CLI tool)
- soci-snapshotter-grpc (this is the daemon used for lazy loading, we will use
this later)

Many of the `soci` CLI tool commands will need to be run as sudo, this is
because the metadata is saved in directories that a non-root user often
does not have access to.

### Creating the SOCI index

To create the index run:
```
sudo ./soci create localhost:5000/rabbitmq:latest
```
Behind the scenes SOCI is interacting with containerd.  So the tag
`localhost:5000/rabbitmq` has to exist in containerd - that is what SOCI is
referencing.  This created two kinds of objects.  The first is a series of ztocs
(one per layer).  A ztoc is a table of contents for compressed data (z is the
compression, toc is Table Of Contents).  The other is a manifest that relates
the ztocs to the various layers.  Example output is:
```
layer sha256:aa5c1807c64faef5411fdf8d572336478d2ae55881a348ca98d27de0c1031012 -> ztoc sha256:7482d8b46f52b9abc85b417e5cd6ce596fe078de428b46afad2943ec1ea1110c
layer sha256:a2c7ff857687a23fe0c41f456c2b8f42359fbf35f3a5a6f3dc2cabee26aa4e9c -> ztoc sha256:4bbc8d97f9007e4023355f6a5634ad922d377256e94ccc322c975f19db351d3d
layer sha256:08ed900bdfb06a0db4f07f002103a52dc480d98f448f939723cd49670e44a43d -> ztoc sha256:2d62d75c5342abfdab9acdeaa66f2f7d44032b70ac31469984da4e8bbb455605
layer sha256:688ea927e54f8ad42dd715f3803db20f58f804970da295dbafa45ae8e094b588 -> ztoc sha256:d0864573c4166a91595bf19799d1e931ac15573ef4e535a61348f3afc7723a63
layer sha256:11e66458f619be5b2e7ca677d6a82ea650682c6554487438e6f68ba3039248db -> ztoc sha256:50c548ae1967542cd50fd7796f20add555e24acbb03ea12608e3697c153afa01
layer sha256:2f11156fa5ac4b97d38486ef618b292e112c7822d29b5dd7b425cb34a96b6594 -> ztoc sha256:d941838ee3b9862d0dfe4d0b50724818040584f0375a7c09257702d7fe40c9f5
layer sha256:87d3a0863f984408afb6caf67a12832bc45cc3b3acbbf98c28ec916499c38a33 -> ztoc sha256:9e1c3ddb273db141e57fed0af6a0ba4a3b864adbf7bd42608715a6fb95a9e3b1
layer sha256:3b65ec22a9e96affe680712973e88355927506aa3f792ff03330f3a3eb601a98 -> ztoc sha256:f9d786ee3e082fc671dac3e4b38dd1458a20e0425be31b6b09dfaa727925c3d2
layer sha256:7e1cc2fa8c69560f02a99729c513ec7e3f49257d893bf8d30b5c6e7f50992644 -> ztoc sha256:4c1d63f476d4907e0db42b8736f578e79432a28d304935708c918c95e0e4df00
```
We can inspect one of these ztoc's with the following command (you will need to
replace the digest with one of the ones created above):
```
sudo ./soci ztoc info sha256:4c1d63f476d4907e0db42b8736f578e79432a28d304935708c918c95e0e4df00
```

This will print to STDOUT the ztoc, which contains all of the information that
SOCI needs to find a given file in the layer.  We can also view the
index manifests by running:

```
sudo ./soci index list
```

This will list out all of our index manifests.  To inspect an individual one we
can use following (as always replace the digest with your own):

```
sudo ./soci index info sha256:f5f2a8558d0036c0a316638c5575607c01d1fa1588dbe56c6a5a7253e30ce107
```

This will dump out the index manifest in json.

### Pushing the manifest to the registry
Next we need to push the manifest to the registry with the following command.
Just like with ctr we need to use `--plain-http` flag:

```
sudo ./soci push --plain-http localhost:5000/rabbitmq:latest
```

This will push all of the SOCI related artifacts.

## Running the image

### Configuring containerd
To have containerd use the soci-snapshotter we need to reconfigure it.
Containerd is a daemon managed using the systemd / systemctl tools.
First let's stop containerd

```
sudo systemctl stop containerd
```

Next we need to edit the config file that manages all of containerd's settings.
This file is /etc/containerd/config.toml.  By default the file has an example
configuration that is all commented out (please see all of the # signs). Let's
edit this file to include SOCI snapshotter as a plugin with the following:

```
[proxy_plugins]
  [proxy_plugins.soci]
    type = "snapshot"
    address = "/run/soci-snapshotter-grpc/soci-snapshotter-grpc.sock"
```

These lines tell containerd that there is a snapshot plugin named SOCI and to
communicate with it via a socket file.  Now let's restart containerd

```
sudo systemctl restart containerd
```

To confirm that containerd knows about SOCI run

```
ctr plugin ls
```

This will list all of the plugins confirm that there is a **soci**
plugin of type **io.containerd.snapshotter.v1**

### Preparing SOCI snapshotter
First we need to start the snapshotter binary, it lives in the `out` directory.
In the example command we arbitrarily are pushing the logs to a file named
`~/soci-snapshotter-logs`, but please feel free to choose a different location:

```
sudo ./soci-snapshotter-grpc&> ~/soci-snapshotter-logs &
```

Once the snapshotter is running we can call the rpull command from the SOCI CLI.
This command reads the manfiest from the registry and mounts FUSE filesystems
for each layer.

In this command the `--soci-index-digest` needs to be the digest of the SOCI
index manifest.  The final argument is the tag of the image:

```
sudo ./soci image rpull --plain-http --soci-index-digest sha256:f5f2a8558d0036c0a316638c5575607c01d1fa1588dbe56c6a5a7253e30ce107 localhost:5000/rabbitmq:latest
```

After running this command you will see a minimal output.  If you see all of the
layers being fetched now, then SOCI has not worked.  It should look something
like:

```
using SOCI index digest: sha256:f5f2a8558d0036c0a316638c5575607c01d1fa1588dbe56c6a5a7253e30ce107
fetching sha256:7e2a3a93... application/vnd.docker.distribution.manifest.v2+json
fetching sha256:31b721ac... application/vnd.docker.container.image.v1+json
```

Now we should also be able to see the mounts that were created for the FUSE
filesystems.  There should be one mount for each layer.  To see this use the
`mount` command.  Within the output you should see lines like:

```
/home/ec2-user/soci-snapshotter/out/soci on /var/lib/soci-snapshotter-grpc/snapshotter/snapshots/1/fs type fuse.rawBridge (rw,relatime,user_id=0,group_id=0,allow_other)
fusectl on /sys/fs/fuse/connections type fusectl (rw,relatime)
/home/ec2-user/soci-snapshotter/out/soci on /var/lib/soci-snapshotter-grpc/snapshotter/snapshots/2/fs type fuse.rawBridge (rw,relatime,user_id=0,group_id=0,allow_other)
.....
```

### Running the Container
Now that all of the mounts are set up we can run the image using the following
command in ctr.  We need to specify which snapshotter we shall use and we will
use the `--net-host` binary flag.  Then we pass in the two main arguments, our
image registry and the id of the container:

```
sudo ctr run --snapshotter soci --net-host localhost:5000/rabbitmq:latest sociExample
```

## Well done
Thank you very much for trying out SOCI!
