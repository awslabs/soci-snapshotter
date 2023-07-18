# Pull modes of the SOCI snapshotter

The SOCI snapshotter is a remote snapshotter. It is able to lazily load the contents
of a container image when a *SOCI index* is present in the remote registry. If
a SOCI index is not found, it will download and uncompress the image layers at
launch time, just like the default snapshotter does.

SOCI indices can also be "sparse", meaning that any individual layer may not be
indexed. In that case, that layer will be downloaded at launch time, while the
indexed layers will be lazily loaded.

A layer will be mounted as a FUSE mountpoint if it's being lazily loaded, or as
a normal overlay layer if it's not.

Overall, lazily pulling a container image with the SOCI snapshotter
(via the `soci image rpull` command) involves the following steps:

<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->

- [Pull modes of the SOCI snapshotter](#pull-modes-of-the-soci-snapshotter)
  - [Step 1: specify SOCI index digest](#step-1-specify-soci-index-digest)
  - [Step 2: fetch SOCI artifacts](#step-2-fetch-soci-artifacts)
  - [Step 3: fetch image layers](#step-3-fetch-image-layers)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

## Step 1: specify SOCI index digest

To enable lazy pulling and loading an image with the SOCI snapshotter, first you need
to `rpull` the image via the [`soci` CLI](./getting-started.md#install-soci-snapshotter).
The CLI accepts an optional flag `--soci-index-digest`, which is the sha256 of the
SOCI index manifest and will be passed to the snapshotter.

If not provided, the snapshotter will use the OCI distribution-spec's
[Referrers API](https://github.com/opencontainers/distribution-spec/blob/main/spec.md#listing-referrers)
(if available, otherwise the spec's
[fallback mechanism](https://github.com/opencontainers/distribution-spec/blob/main/spec.md#unavailable-referrers-api))
to fetch a list of available indices. An index will be chosen from the list of available indices,
but the selection process is undefined and it may not choose the same index every time.

> **Note**
> Check out [this doc](./getting-started.md#lazily-pull-image) for how to
> validate if this step is successful or not, and [the debug doc](./debug.md#common-scenarios)
> for the common scenarios where `rpull` might fail and how to debug/fix them.

## Step 2: fetch SOCI artifacts

During `rpull`, on the first layer mount there will be an attempt to download
and parse the SOCI manifest. If this doesn’t go well, there will be the following
error in the log: `unable to fetch SOCI artifacts:`, indicating that the
container image will not be lazily loaded. In this case, the snapshotter will
fallback to default snapshotter configured (eg: overlayfs) entirely.

> Check out [the debug doc](./debug.md#common-scenarios) for how to debug/fix it.

## Step 3: fetch image layers

The SOCI index will instruct containerd and the SOCI snapshotter when to fetch/pull
image layers. There can be two cases:

1. There’s no zTOC for a specific layer. In this case, there will be an error log:
`{"error":"failed to resolve layer`, indicating that this layer will be
synchronously downloaded at launch time.
2. There's a zTOC for a specific layer. In this case, the layer will be mounted
as a fuse mountpoint, and will be lazily loaded while a container is running.

> Whether a layer belongs to 1 or 2 depends on its size. When creating a SOCI
> index, SOCI only creates zTOC for layers larger than a given size which is
> specified by the `--min-layer-size` flag of
[`soci create` command](https://github.com/awslabs/soci-snapshotter/blob/9ff88817f3f2635b926f9fd32f6f05f389f7ecee/cmd/soci/commands/create.go#L56).

With debug logging enabled, you can see an entry in logs for each layer.
`checking mount point` indicates that the layer will be lazily loaded.
`layer is normal snapshot(overlayfs)` indicates that it will not be lazily loaded.

```shell
{"key":"sha256:5e986c80babd9591530ee7b5844f8f9cca87b991da5dbf0f489f8612228f28f6","level":"debug","mount-point":"/var/lib/soci-snapshotter-grpc/snapshotter/snapshots/17/fs","msg":"layer is normal snapshot(overlayfs)","time":"2022-08-16T18:06$
{"key":"sha256:5e986c80babd9591530ee7b5844f8f9cca87b991da5dbf0f489f8612228f28f6","level":"debug","mount-point":"/var/lib/soci-snapshotter-grpc/snapshotter/snapshots/16/fs","msg":"layer is normal snapshot(overlayfs)","time":"2022-08-16T18:06$
{"key":"sha256:5e986c80babd9591530ee7b5844f8f9cca87b991da5dbf0f489f8612228f28f6","level":"debug","mount-point":"/var/lib/soci-snapshotter-grpc/snapshotter/snapshots/15/fs","msg":"layer is normal snapshot(overlayfs)","time":"2022-08-16T18:06$
{"key":"sha256:5e986c80babd9591530ee7b5844f8f9cca87b991da5dbf0f489f8612228f28f6","level":"debug","mount-point":"/var/lib/soci-snapshotter-grpc/snapshotter/snapshots/13/fs","msg":"layer is normal snapshot(overlayfs)","time":"2022-08-16T18:06$
{"key":"sha256:5e986c80babd9591530ee7b5844f8f9cca87b991da5dbf0f489f8612228f28f6","level":"debug","mount-point":"/var/lib/soci-snapshotter-grpc/snapshotter/snapshots/12/fs","msg":"layer is normal snapshot(overlayfs)","time":"2022-08-16T18:06$
{"key":"sha256:5e986c80babd9591530ee7b5844f8f9cca87b991da5dbf0f489f8612228f28f6","level":"debug","mount-point":"/var/lib/soci-snapshotter-grpc/snapshotter/snapshots/11/fs","msg":"layer is normal snapshot(overlayfs)","time":"2022-08-16T18:06$
{"key":"sha256:5e986c80babd9591530ee7b5844f8f9cca87b991da5dbf0f489f8612228f28f6","level":"debug","mount-point":"/var/lib/soci-snapshotter-grpc/snapshotter/snapshots/18/fs","msg":"checking mount point","time":"2022-08-16T18:06:48.628108043Z"}
{"key":"sha256:5e986c80babd9591530ee7b5844f8f9cca87b991da5dbf0f489f8612228f28f6","level":"debug","mount-point":"/var/lib/soci-snapshotter-grpc/snapshotter/snapshots/14/fs","msg":"checking mount point","time":"2022-08-16T18:06:48.628124854Z"}
{"key":"sha256:5e986c80babd9591530ee7b5844f8f9cca87b991da5dbf0f489f8612228f28f6","level":"debug","mount-point":"/var/lib/soci-snapshotter-grpc/snapshotter/snapshots/10/fs","msg":"checking mount point","time":"2022-08-16T18:06:48.628164485Z"}
{"key":"sha256:5e986c80babd9591530ee7b5844f8f9cca87b991da5dbf0f489f8612228f28f6","level":"debug","mount-point":"/var/lib/soci-snapshotter-grpc/snapshotter/snapshots/9/fs","msg":"layer is normal snapshot(overlayfs)","time":"2022-08-16T18:06:$
{"key":"sha256:5e986c80babd9591530ee7b5844f8f9cca87b991da5dbf0f489f8612228f28f6","level":"debug","mount-point":"/var/lib/soci-snapshotter-grpc/snapshotter/snapshots/7/fs","msg":"layer is normal snapshot(overlayfs)","time":"2022-08-16T18:06:$
{"key":"sha256:5e986c80babd9591530ee7b5844f8f9cca87b991da5dbf0f489f8612228f28f6","level":"debug","mount-point":"/var/lib/soci-snapshotter-grpc/snapshotter/snapshots/6/fs","msg":"layer is normal snapshot(overlayfs)","time":"2022-08-16T18:06:$
{"key":"sha256:5e986c80babd9591530ee7b5844f8f9cca87b991da5dbf0f489f8612228f28f6","level":"debug","mount-point":"/var/lib/soci-snapshotter-grpc/snapshotter/snapshots/4/fs","msg":"layer is normal snapshot(overlayfs)","time":"2022-08-16T18:06:$
{"key":"sha256:5e986c80babd9591530ee7b5844f8f9cca87b991da5dbf0f489f8612228f28f6","level":"debug","mount-point":"/var/lib/soci-snapshotter-grpc/snapshotter/snapshots/2/fs","msg":"layer is normal snapshot(overlayfs)","time":"2022-08-16T18:06:$
{"key":"sha256:5e986c80babd9591530ee7b5844f8f9cca87b991da5dbf0f489f8612228f28f6","level":"debug","mount-point":"/var/lib/soci-snapshotter-grpc/snapshotter/snapshots/8/fs","msg":"checking mount point","time":"2022-08-16T18:06:48.628307230Z"}
{"key":"sha256:5e986c80babd9591530ee7b5844f8f9cca87b991da5dbf0f489f8612228f28f6","level":"debug","mount-point":"/var/lib/soci-snapshotter-grpc/snapshotter/snapshots/5/fs","msg":"checking mount point","time":"2022-08-16T18:06:48.628321040Z"}
{"key":"sha256:5e986c80babd9591530ee7b5844f8f9cca87b991da5dbf0f489f8612228f28f6","level":"debug","mount-point":"/var/lib/soci-snapshotter-grpc/snapshotter/snapshots/1/fs","msg":"checking mount point","time":"2022-08-16T18:06:48.628348072Z"}
{"key":"sha256:5e986c80babd9591530ee7b5844f8f9cca87b991da5dbf0f489f8612228f28f6","level":"debug","mount-point":"/var/lib/soci-snapshotter-grpc/snapshotter/snapshots/3/fs","msg":"checking mount point","time":"2022-08-16T18:06:48.628371627Z"}
```
