# Pull modes of soci-snapshotter

soci-snapshotter is a remote snapshotter. It is able to lazily load the contents
of a container image when a *SOCI index* is present in the remote registry. If
a SOCI index is not found, it will download and uncompress the layers at launch
time, just like the default snapshotter does.

SOCI indices can also be "sparse", meaning that any individual layer may not be
indexed. In that case, that layer will be downloaded at launch time, while the
indexed layers will be lazily loaded.

A layer will be mounted as a fuse mountpoint if it's being lazily loaded, or as
a normal overlay layer if it's not.

When you pull an image, there can be the following cases:

1. For lazy loading the snapshotter accepts an optional flag `--soci-index-digest`,
which is the sha256 of the SOCI index manifest. If
not provided, the snapshotter will use the OCI distribution-spec's
[Referrers API](https://github.com/opencontainers/distribution-spec/blob/main/spec.md#listing-referrers)
(if available, otherwise the spec's
[fallback mechanism](https://github.com/opencontainers/distribution-spec/blob/main/spec.md#unavailable-referrers-api))
to fetch a list of available indices. An index will be chosen from the list of available indices,
but the selection process is undefined and it may not choose the same index every time.
2. If 1 goes well, the launch is a SOCI use case. On the first layer mount, there
will be an attempt to download and parse the SOCI manifest. If this doesn’t go well,
there will be the following error in the log: `unable to fetch SOCI artifacts:`,
which will indicate that the container image will not be lazily loaded.
3. If 1 and 2 go well. There can be a case, where there’s no zTOC for a specific
layer. In this case, there will be an error with the text `{"error":"failed to resolve layer`
which will indicate that the specific layer will be synchronously downloaded at launch time.
4. With debug logging enabled, you can see an entry in the logs for each layer.
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
