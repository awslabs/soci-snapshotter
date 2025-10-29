# SOCI Snapshotter

[![PkgGoDev](https://pkg.go.dev/badge/github.com/awslabs/soci-snapshotter)](https://pkg.go.dev/github.com/awslabs/soci-snapshotter)
[![Go Report Card](https://goreportcard.com/badge/github.com/awslabs/soci-snapshotter)](https://goreportcard.com/report/github.com/awslabs/soci-snapshotter)
[![Build](https://github.com/awslabs/soci-snapshotter/actions/workflows/build.yml/badge.svg)](https://github.com/awslabs/soci-snapshotter/actions/workflows/build.yml)
[![Static Badge](https://img.shields.io/badge/Website-Benchmarks-blue)](https://awslabs.github.io/soci-snapshotter/dev/benchmarks/)

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

> **NOTE**
> This section describes SOCI Index Manifest v1. While the lack of an image conversion step is appealing, for production scenarios it also creates the potential for performance changes across multiple dimensions if an index is added to or removed from an existing image that is widely deployed. To address these downsides, we introduced SOCI Index Manifest v2 which does use a build-time conversion step. For more information see the [SOCI Index Manifest v2 documentation](./docs/soci-index-manifest-v2.md)

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

Another big consideration that we haven't implemented/integrated
into SOCI is image load order based on your specific workload. See [design README](./docs/design-docs/README.md#workload-specific-load-order-optimization)
for more details.

## Documentation

- [Getting Started](docs/getting-started.md): walk through SOCI setups and features.
- [Build](docs/build.md): how to build SOCI from source, test SOCI (and contribute).
- [Install](docs/install.md): how to install SOCI as a systemd unit.
- [Debug and Useful Commands](docs/debug.md): accessing logs/metrics and debugging common errors.
- [Glossary](docs/glossary.md): glossary we use in the project.

### Integration-specific documentation

- [SOCI on Kubernetes](docs/kubernetes.md): an overview of how to use SOCI on Kubernetes in general
- [SOCI on Amazon Elastic Kubernetes Service (EKS)](docs/eks.md): a walk through for setting up SOCI on Amazon EKS

## Project Origin

There are a few different lazy loading projects in the containerd snapshotter community.  This project began as a
fork of the popular [Stargz-snapshotter project](https://github.com/containerd/stargz-snapshotter) from
commit 743e5e70a7fdec9cd4ab218e1d4782fbbd253803 with the intention of an upstream patch.  During development
the changes were fundamental enough that the decision was made to create soci-snapshotter as a standalone
project.  Soci-snapshotter builds on stargz's success and innovative ideas.  Long term, this project intends
and hopes to join [containerd](https://github.com/containerd/containerd) as a non-core project and intends to
follow CNCF best practices.
