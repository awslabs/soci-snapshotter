# Design docs

We use this folder to track design docs for the project and their status (proposed
, accepted, implemented, etc).

We also keep some features/ideas that we think will improve soci-snapshotter but
haven't been converted into concrete design docs.

## Workload-specific load order optimization

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
