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

Some lazy loading snapshotters support load order optimization, where some files are
prioritized for prefetching. Typically, there is a one-to-one relationship between
the list of to-be-prefetched files and the image or layer artifact.

For SOCI, we wanted a bit more flexibility. Often, which files to prefetch is highly
dependent on the specific workload, not the image or base layer. For example, a customer
may have a Python3 base layer that is shared by thousands of applications. To optimize
the launch time of those applications using the traditional approach, the base
layer can no longer be shared, because each application’s load order for that layer will be
different. Registry storage costs will increase dramatically, and cache hit rates will plummet.
And when it comes time to update that base layer, each and every copy will have to be reoptimized.

Secondly, there are some workloads that need to be able to prefetch at the subfile level. For example,
we have observed machine learning workloads that launch and then immediately read a small header
from a very large number of very large files.

To meet these use-cases, SOCI will implement a separate load order document (LOD), that can specify
which files or file-segments to load. Because it is a separate artifact, a single image can have
many LODs. At container launch time, the appropriate LOD can be retrieved using business logic
specified by the administrator.

***Note:*** **SOCI Load order optimization is not yet implemented in SOCI.**

## Learning More
Check out our docs area:
- [Getting
Started](docs/GETTING_STARTED.md)
- [Glossary](docs/GLOSSARY.md)

## Project Origin

There a few different lazy loading projects in the containerd snapshotter community.  This project began as a
fork of the popular [Stargz-snapshotter project](https://github.com/containerd/stargz-snapshotter) from
commit 743e5e70a7fdec9cd4ab218e1d4782fbbd253803 with the intention of an upstream patch.  During development
the changes were fundamental enough that the decision was made to create soci-snapshotter as a standalone
project.  Soci-snapshotter builds on stargz's success and innovative ideas.  Long term, this project intends
and hopes to join [containerd](https://github.com/containerd/containerd) as a non-core project and intends to
follow CNCF best practices.
