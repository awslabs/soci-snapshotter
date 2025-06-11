# Debugging the SOCI snapshotter

This document outlines where to find/access logs and metrics for the snapshotter. It attempts to provide some common error paths that a user might run into while using the snapshotter and provides some guidance on how to either root-cause the issue or resolve it.

<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->

- [Finding Logs / Metrics](#finding-logs--metrics)
  - [Logs](#logs)
  - [Metrics](#metrics)
    - [Accessing Metrics](#accessing-metrics)
    - [Metrics Emitted](#metrics-emitted)
- [Common Scenarios](#common-scenarios)
  - [Pulling an Image](#pulling-an-image)
    - [Determining How an Image Was Pulled](#determining-how-an-image-was-pulled)
      - [Explicit Index Digest](#explicit-index-digest)
      - [SOCI Index Manifest v2 via SOCI-Enabled Images](#soci-index-manifest-v2-via-soci-enabled-images)
      - [SOCI Index Manifest v1 via The Referrers API](#soci-index-manifest-v1-via-the-referrers-api)
      - [Ahead of Time Without a SOCI Index](#ahead-of-time-without-a-soci-index)
    - [No lazy-loading](#no-lazy-loading)
    - [Pull Taking An Abnormal Amount Of Time](#pull-taking-an-abnormal-amount-of-time)
    - [Background Fetching](#background-fetching)
  - [Running Container](#running-container)
    - [FUSE Read Failures](#fuse-read-failures)
  - [Removing an image](#removing-an-image)
  - [Restarting the snapshotter](#restarting-the-snapshotter)
  - [Creating a clean slate](#creating-a-clean-slate)
- [Debugging Tools](#debugging-tools)
  - [CLI](#cli)
  - [CPU Profiling](#cpu-profiling)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

# Finding Logs / Metrics

## Logs

For the most part, the `soci-snapshotter-grpc` logs will be the most important place to look when debugging. If `soci-snapshotter-grpc` was started via `systemd` then you can obtain logs using `journalctl`: 

```shell
sudo journalctl -u soci-snapshotter.unit
```

> **Note**
> The command above assumes that you have used the unit file definition [soci-snapshotter.service](../soci-snapshotter.service) we have provided. If you have created your own unit file for `soci-snapshotter-grpc` and replace `soci-snapshotter.unit` with the one you have made.
 
If you have started `soci-snapshotter-grpc` manually, logs will either be emitted to stderr/stdout or to the destination of your choice.

## Metrics

### Accessing Metrics

The snapshotter emits [Prometheus](https://prometheus.io/) metrics. To collect and access these metrics, you need to configure `metrics_address` within SOCIs' `config.toml` (located at `/etc/soci-snapshotter-grpc` by default) before starting the snapshotter. You can provide any local address(TCP) or UNIX socket (if you are using a TCP address make sure the port is not in use). To view the metrics you can send a `GET` request via any HTTP client to the `/metrics` endpoint and Prometheus will return all the metrics emitted by the snapshotter.

```shell
$ cat /etc/soci-snapshotter-grpc/config.toml
metrics_address="localhost:8000"

$ curl localhost:8000/metrics

soci_fs_operation_duration_milliseconds_bucket{layer="sha256:328b9d3248edeb3ae6e7f9c347bcdb5632c122be218e6ecd89543cca9c8f1997",operation_type="init_metadata_store",le="1"} 1
soci_fs_operation_duration_milliseconds_bucket{layer="sha256:328b9d3248edeb3ae6e7f9c347bcdb5632c122be218e6ecd89543cca9c8f1997",operation_type="init_metadata_store",le="2"} 1
soci_fs_operation_duration_milliseconds_bucket{layer="sha256:328b9d3248edeb3ae6e7f9c347bcdb5632c122be218e6ecd89543cca9c8f1997",operation_type="init_metadata_store",le="4"} 1
soci_fs_operation_duration_milliseconds_bucket{layer="sha256:328b9d3248edeb3ae6e7f9c347bcdb5632c122be218e6ecd89543cca9c8f1997",operation_type="init_metadata_store",le="8"} 1
...
```

### Metrics Emitted

Below are a list of metrics emitted by the snapshotter:

* Mount
    * **operation_duration_mount (ms)** - defines how long does it take to mount a layer during pull. Pulling should only take a couple of seconds. If this value is higher than 3-5 seconds this can indicate an issue while mounting.
    * **operation_duration_init_metadata_store (ms)** - measures the time it takes to parse a zTOC and prepare the respective metadata records in metadata bbolt db (it records layer digest as well). This is one of the components of pulling, therefore there should be a correlation between the time to parse a zTOC with updating of metadata db and the duration of layer mount operation. 
* Fetch from remote registry
    * **operation_duration_remote_registry_get (ms)** - measures the time it takes to complete a `GET` operation from remote registry for a specific layer. This metric should help in identifying network issues, when lazily fetching layer data and seeing increased container start time.
* FUSE
    * **operation_duration_node_readdir (us)** - measures the time it takes to complete readdir() operation for a file from a specific layer. The per-layer granularity is to point out that each layer has its own `FUSE` mount, so it doesn’t make sense to generalize. The unit is microseconds. Large times in readdir may indicate that there are problems with the request speed from metadata db or issues with the `FUSE` implementation (less likely, since this part is least likely to get modified).
    * **operation_duration_synchronous_read (us)** - measures the duration of `FUSE` read() operation for the specific `FUSE` mountpoint, defined by the layer digest. The unit of measurement is microseconds.
    * **synchronous_read_count** - measures how many read() operations were issued for the specific `FUSE` mountpoint (defined by the layer digest) to date. The  same value can be obtained from `operation_duration_synchronous_read` as the Count property.
    * **synchronous_bytes_served** - measures the number of bytes served for synchronous reads. 
    * **fuse_mount_failure_count** - number of times the snapshotter falls back to use a normal overlay mount instead of mounting the layer as a `FUSE` mount.
    * **background_span_fetch_failure_count** - number of errors of span fetch by background fetcher.
    * **background_span_fetch_count** - number of spans fetched by background fetcher.
    * **background_fetch_work_queue_size** - number of items in the work queue of background fetcher.
    * **operation_duration_background_fetch** - time in milliseconds to complete background fetch for a layer.
    * Individual `FUSE` operation failure counts:
      * fuse_node_getattr_failure_count
      * fuse_node_listxattr_failure_count
      * fuse_node_lookup_failure_count
      * fuse_node_open_failure_count
      * fuse_node_readdir_failure_count
      * fuse_file_read_failure_count
      * fuse_file_getattr_failure_count
      * fuse_whiteout_getattr_failure_count
      * fuse_unknown_operation_failure_count

# Common Scenarios

Below are some common scenarios that may occur during pulling and the lifetime of running a container. For scenarios not covered, please feel free to [open an issue](https://github.com/awslabs/soci-snapshotter/issues/new/choose).

> **Note**
> To allow for more verbose logging you can set the `--log-level` flag to `debug` when starting the snapshotter.

## Pulling an Image

While pulling, the image manifest, config, and layers without zTOCs' are fetched from the remote registry directly. Layers that have a zTOC are mounted as a `FUSE` file system and will be pulled lazily when launching a container.

Below are a list of common error paths that may occur in this phase:

### Determining How an Image Was Pulled

The SOCI snapshotter can pull an image using one of four modes:
1) Lazily using an explicitly provided SOCI index digest
2) Lazily using a SOCI index manifest v2 via SOCI-enabled images
3) Lazily using a SOCI index manifest v1 via the Referrers API
4) Ahead of time without a SOCI index

Determining which of these mode was used is often a good first step for debugging image pull issues.

When the SOCI snapshotter starts to pull an image, it will emit a set of debug logs to indicate which modes were tried and which was ultimately used.

#### Explicit Index Digest
If the SOCI snapshotter finds an explicit index digest, it will use it and log the following message:
```
using provided soci index digest
```

#### SOCI Index Manifest v2 via SOCI-Enabled Images
If an explicit index digest was not provided. The SOCI snapshotter will try to use a SOCI-enabled image.

If `pull_modes.soci_v2.enabled` is `false`, the snapshotter will log:
```
soci v2 is disabled
```

Otherwise, if it find that the image is SOCI-enabled, it will log:
```
using soci v2 index annotation
```

#### SOCI Index Manifest v1 via The Referrers API
If the image is not SOCI-enabled, the SOCI snapshotter will try to use the referrers API.

If `pull_modes.soci_v1.enabled` is `false` (it is false by default), the SOCI snapshotter will log:
```
soci v1 is disabled
```

Otherwise, if it finds a SOCI index via the referrers API, it will log:
```
using soci v1 index via referrers API
```

#### Ahead of Time Without a SOCI Index
If none of the above apply, the SOCI snapshotter will pull the image ahead of time and log:

```
deferring to container runtime
```

### No lazy-loading

If you notice that all layers are being fetched for an image or that `FUSE` mounts are not being created for layers with a zTOC than that means that remote mounting has failed for those layers.

Once you inspect the logs you should come across an error log that contains the message `failed to prepare remote snapshot` with an `error` key describing the error that was propagated up within the snapshotter.

Some possible error keys include:

* `skipping mounting layer <layer_digest> as FUSE mount: no zTOC for layer`
  
  This "error" message is not really indicative of a true error, but rather implies that the current layer does not have an associated zTOC.
  This is expected for layers that do not meet the minimum-layer size criteria established when creating the soci-index/zTOCs. 

* `unable to fetch SOCI artifacts: <error>`
  
  This error indicates that the soci index along with the corresponding zTOC could not be fetched from the remote registry. This can be for a variety of different reasons. The most common reason is that the resolver could not authenticate against the remote registry. The snapshotter uses the docker resolver to resolve blobs in the remote so you must authenticate with docker first. If you are using `ECR` as your registry you can:

  ```shell
  export ECR_PASS=$(aws ecr get-login-password --region <region>)
  echo $ECR_PASS | sudo docker login -u AWS --password-stdin $ECR_REGISTRY 
  ```
  
  > **Note**
  > SOCI artifacts are only fetched when preparing the first layer. If they cannot be fetched the snapshotter will fallback to default snapshotter configured (eg: overlayfs) entirely.
  
### Pull Taking An Abnormal Amount Of Time

If you notice that pulling takes a considerable amount of time you can:

* Look for `failed to resolve layer (timeout)` within the logs. Remote mounts may take too long if something’s wrong with layer resolving. By default remote mounts time out after 30 seconds if a layer can’t be resolved.

* Check the `operation_duration_mount` metric to see if it takes unusual long time to mount a layer. Pull should be taking a couple of seconds, so one can be checking if any of these operations are taking more than 3-5 seconds.

* Parsing zTOC and initializing the metadata db is part of the pull command. You can check the  `operation_duration_init_metadata_store` metric to see if initializing the metadata bbolt db is too slow.  

* Look for HTTP failure codes in the log. Such logs are in this format: `Received status code`:

### Background Fetching

The background fetcher is initialized as soon as the snapshotter starts. If you have not explicitly disabled it via the the snapshotters config, it will be performing network requests to fetch data during/after pulling. To analyze the background fetcher you can:

* Look at the `background_span_fetch_failure_count` to determine how many times a background fetch failed.
* Look at `background_span_fetch_count` metric to determine how many spans were fetched by the background fetcher. If this number is 0 this may indicate network failures. 
  * Look for `Retrying request` within the logs to determine the error and response returned from the remote registry.

## Running Container

A running container produces many read requests. If there is a read request for a file residing within a lazy-loaded layer than the read request is routed through the layers' `FUSE` filesystem. This path can produce several different errors:

### FUSE Read Failures

Look for  `failed to read the file` or `unexpected copied data size for on-demand fetch` in the logs. 

**Corrupt Data**

* Span verification failures can occur if the fetched data is corrupt or has been altered since zTOC creation. You can look for `span digests do not match` within logs to verify that this is the root cause.
* Check to see if the zTOC contains appropriate data. You can do this by running the `soci ztoc info <digest>` command to inspect the zTOC. If the dictionaries are all 0-ed, the zTOC initially generated and subsequently pulled was corrupt.

**Network Failures**

The snapshotter contains custom retry logic when fetching spans(data) from the remote registry. By default it will try to fetch from remote a maximum of 9 times before returning an error.

* You can look for `retrying request` within the logs to determine the error and response returned from the remote registry.
* You can also check `operation_duration_remote_registry_get` metric to see how long it takes to complete `GET` from remote registry.

## Removing an image
Removing an image additionally removes any associated snapshots. A simple `sudo nerdctl image rm [image tag]` should remove all snapshots associated with the image before the image itself is removed. You can confirm the image is gone by ensuring it is no longer present in `sudo nerdctl image ls`.

## Restarting the snapshotter
While a graceful stop/restart of the process is preferred, sometimes it is not possible or simply easier to just kill the process. However, oftentimes it comes with a multitude of issues that do not allow you to start the snapshotter properly, particularly if a previously loaded snapshot is still present. For instance, if you pull from a repository that requires credentials and stop the snapshotter, if credentials are expired upon the daemon's startup, the snapshotter will fail to start properly.

Many errors related to loaded snapshots can be surpassed by setting `allow_invalid_mounts_on_restart=true` in `/etc/soci-snapshotter-grpc/config.toml`. Note that using the snapshotter will likely load in a broken state and you will be unable to do common functionality (such as pulling another image) until the currently loaded snapshot is removed.

## Creating a clean slate
If all else fails, a clean slate can help to get you back to square one. These steps should bring you to a clean slate. (NOTE: This includes wiping your entire container store clean, so be sure to back up any important files.)

```bash
sudo killall -2 soci-snapshotter-grpc # SIGINT allows for a more graceful cleanup, can omit the -2 flag to send a SIGTERM

# If necessary, unmount any remaining fuse mounts, though a SIGINT to the daemon should automatically handle that

sudo rm -rf /var/lib/containerd
sudo rm -rf /var/lib/soci-snapshotter-grpc

sudo systemctl restart containerd

sudo soci-snapshotter-grpc
```

# Debugging Tools

## CLI

Here are some SOCI CLI commands that can be helpful when debugging issues relating to SOCI indices/zTOCs'

| SOCI CLI Command                         | Description                                                                                          |  
| ----------------                         | -----------                                                                                          |
| soci ztoc get-file <digest> <file-name>  | retrieve a file from a local image layer using a specified ztoc                                      |
| soci ztoc info <digest>                  | get detailed info about a ztoc (list of files+offsets, num of spans, ...etc)                         |
| soci ztoc list                           | list all ztocs                                                                                       |
| soci index info <digest>                 | retrieve the contents of an index                                                                    |
| soci index list [options] —ref           | list ztocs across all images / filter indices to those that are associated with a specific image ref |
| soci index rm [options] —ref	           | remove an index from local db / only remove indices that are associated with a specific image ref    |

## CPU Profiling

We can use Golangs `pprof` tool to profile the snapshotter. To enable profiling you must set the `debug_address` within the snapshotters config (default: `/etc/soci-snapshotter-grpc/config.toml`):

```toml
debug_address = "localhost:6060"
```

Once you have configured the debug address you can send a `GET` to the `/debug/pprof/profile` endpoint to receive a CPU profile of the snapshotter. You can specify an optional argument `seconds` to limit the results to a certain time span:

```shell
curl http://localhost:6060/debug/pprof/profile?seconds=40 > out.pprof
```

You can use the `pprof` tool provided by the Go CLI to visualize the data within a web browser:

```shell
go tool pprof -http=:8080 out.pprof
```