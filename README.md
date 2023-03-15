# SOCI Snapshotter (With Podman!)

This is a fork of the [SOCI Snapshotter](https://github.com/awslabs/soci-snapshotter) that adds support for streaming images to podman. It was forked at [3b2cc1](https://github.com/awslabs/soci-snapshotter/commit/3b2cc11be4627b097f4b225bd105100f5f7957d1) without the intention of merging upstream, per [discussion](https://github.com/awslabs/soci-snapshotter/issues/486) with the SOCI Snapshotter maintainers.

You can test this out by following the [Getting Started](https://github.com/awslabs/soci-snapshotter/blob/main/docs/getting-started.md) instructions over in the SOCI Snapshotter repository to create and push SOCI indices to your container registry. Once you've done that, you can build the code with `make`, run it with `./out/soci-store /var/lib/soci/soci-store`, update your `/etc/containers/storage.conf` to point to the soci-store:
```
[storage]
driver = "overlay"
runroot = "/run/containers/storage"
graphroot = "/var/lib/containers/storage"
[storage.options]
additionallayerstores=["/var/lib/soci-store/store:ref"]
```

And finally, run podman: `podman pull --storage-opt=additionallayerstore=/var/lib/soci-store/store:ref ubuntu:latest`.

Quick list of things for me to look at / clean up at some point:
- Still requires sudo :-( (fine for our use case for now)
- What's the `diff1` file and why is it being read so much?

Below is the readme from the SOCI Snapshotter repository at 3b2cc1.

# SOCI Snapshotter

SOCI Snapshotter is a [containerd](https://github.com/containerd/containerd)
snapshotter plugin. It enables standard OCI images to be lazily loaded without
requiring a build-time conversion step. "SOCI" is short for "Seekable OCI", and is
pronounced "so-CHEE".

The standard method for launching containers starts with a setup phase during which the
container image data is completely downloaded from a remote registry and a filesystem is assembled.
The application is not launched until this process is complete. Using a representative suite of images,
Harter et al [FAST '16](https://www.usenix.org/node/194431) found that image download accounts for 76%
of container startup time, but on average only 6.4% of the fetched data is actually needed for the
container to start doing useful work.

One approach for addressing this is to eliminate the need to download the entire image before launching
the container, and to instead lazily load data on demand, and also prefetch data in the background.

## Design considerations

### No image conversion

Existing lazy loading snapshotters rely on a build-time conversion step, to produce a new image artifact.
This is problematic for container developers who won't or can't modify their CI/CD pipeline, or don't
want to manage the cost and complexity of keeping copies of images in two formats. It also creates
problems for image signing, since the conversion step invalidates any signatures that were created against
the original OCI image.

SOCI addresses these issues by loading from the original, unmodified OCI image. Instead of
converting the image, it builds a separate index artifact (the "SOCI index"), which lives
in the remote registry, right next to the image itself. At container launch time,
SOCI Snapshotter queries the registry for the presence of the SOCI index using the mechanism
developed by the [OCI Reference Types working group](https://github.com/opencontainers/wg-reference-types).

### Workload-specific load order optimization

Another big consideration that we haven't implmented/integrated
into SOCI is to image load order based on your specific workload. See [design README](./docs/design-docs/README.md#workload-specific-load-order-optimization)
for more details.

## Documentation

- [Getting Started](docs/getting-started.md): walk through SOCI setups and features.
- [Build](docs/build.md): how to build SOCI from source, test SOCI (and contribute).
- [Install](docs/install.md): how to install SOCI as a systemd unit.
- [Debug](docs/debug.md): accessing logs/metrics and debugging common errors.
- [Glossary](docs/glossary.md): glossary we use in the project.

## Project Origin

There a few different lazy loading projects in the containerd snapshotter community.  This project began as a
fork of the popular [Stargz-snapshotter project](https://github.com/containerd/stargz-snapshotter) from
commit 743e5e70a7fdec9cd4ab218e1d4782fbbd253803 with the intention of an upstream patch.  During development
the changes were fundamental enough that the decision was made to create soci-snapshotter as a standalone
project.  Soci-snapshotter builds on stargz's success and innovative ideas.  Long term, this project intends
and hopes to join [containerd](https://github.com/containerd/containerd) as a non-core project and intends to
follow CNCF best practices.
