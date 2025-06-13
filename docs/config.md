# Config file options

SOCI has a multitude of variables that can customize SOCI behavior to best suit users' needs. This doc will break down the possible values for each variable, where the variable can be found, and what it can do.

This doc assumes the usage of a TOML file. Per TOML specs, Anything in [brackets] is the required header for the set of variables.

An example configuration file can be found in [config/config.toml](../config/config.toml).

The following format will be used throughout this doc:

## File path
### [toml_header]
- `toml_var_name` (type) — Brief description. Default: some value.

#

# Uncategorized

This set of variables must be at the top of your TOML file due to not belonging to any TOML header.

## config/fs.go
### FSConfig
- `resolve_result_entry` (int) — Max amount of entries allowed in the cache. Default: 30.
- `debug` (bool) — Enables debugging for go-fuse in logs. This often emits sensitive data, so this should be false in production. Default: false.
- `disable_verification` (bool) — Allows skipping TOC validation, which can give slight performance improvements if files have already been verified elsewhere. Default: false.
- `no_prometheus` (bool) — Toggle prometheus metrics. Default: false.
- `mount_timeout_sec` (int) — Timeout for mount if a layer can't be resolved. Default: 30.
- `fuse_metrics_emit_wait_duration_sec` (int) — The wait time before the snaphotter emits FUSE operation counts for an image. Default: 60.

## config/config.go
### Config
- `metrics_address` (string) — If empty, no metrics will be polled. Default: "".
- `metrics_network` (string) — Chooses protocol to send metrics over (e.g. tcp, unix, etc). Default: "tcp".
- `no_prometheus` — Defined [above](#configfsgofsconfig), cannot be redeclared.
- `debug_address` (string) — Address where [go pprof](https://pkg.go.dev/net/http/pprof) server will listen. If empty, no logs will be emitted. Default: "".
- `metadata_store` (string) — Metadata storage type. Only "db" is valid. Default: "db".
- `skip_check_snapshotter_supported` (bool) - skip check for snapshotter is supported which can give performance benefits for SOCI daemon startup time. This config should only be done if you are sure overlayfs is supported. Default: false

#

# Categorized

## config/fs.go

### [http]
- `MaxRetries` (int) — Max retries before giving up on a network request. Default: 8.
- `MinWaitMsec` (int) — Min time between network request attempts. Default: 30.
- `MaxWaitMsec` (int) — Max time between network request attempts. Default: 300000.
- `DialTimeoutMsec` (int) — Max time for a connection before timeout. Default: 3000.
- `ResponseHeaderTimeoutMsec` (int) — Maximum duration waiting for response headers before timeout. Default: 3000.
- `RequestTimeoutMsec` (int) — Maximum duration waiting for entire request before timeout. Default: 300000.

### [blob]
- `valid_interval` (int) — Checks blob regularly at this interval in seconds. Default: 60.
- `check_always` (bool) — Always check blobs. Default: false.
- `fetching_timeout_sec` (int) — Sets maximum amount of seconds to wait before failing to fetch. Default: 300.
- `max_retries` — Blob level MaxRetries. Will override the global MaxRetries set in [[http]](#http).
- `min_wait_msec` — Blob level MinWaitMsec. Will override the global MinWaitMsec set in [[http]](#http).
- `max_wait_msec` — Blob level MaxWaitMsec. Will override the global MaxWaitMsec set in in [[http]](#http).
- `max_span_verification_retries` (int) — Defines number of retries if blob fetch fails. Default: 0.

### [directory_cache]
- `max_lru_cache_entry` (int) — Max items in Least Recently Used (LRU) Cache. Default: 10.
- `max_cache_fds`  (int) — Max file descriptors in Least Recently Used (LRU) Cache. Default: 10.
- `sync_add` (bool) — When true, synchronously adds data to cache. Default: false. 

### [fuse]
- `attr_timeout` (int) — Max timeout for a file system in seconds. Default: 1.
- `entry_timeout` (int) — TTL for a directory name lookup in seconds. Default: 1.
- `negative_timeout` (int) — Defines overall entry timeout for failed lookups in seconds. Default: 1.
- `log_fuse_operations` (bool) — Similar to `debug`, enables debugging for FUSE FS in logs. This often emits sensitive data, so this should be false in production. Default: false.

### [background_fetch]
- `disable` (bool) — Disables the background fetcher. Default: false.
- `silence_period_msec` (int) — Time that the background fetcher will be paused when a new image is mounted. Default: 30000.
- `fetch_period_msec` (int) — How often spans will be fetched. Default: 500.
- `max_queue_size` (int) — Max span managers that can be queued. Default: 100.
- `emit_metric_period_sec` (int) — Interval of background fetcher metric emission. Default: 10.

### [content_store]
- `type` (string) — Sets content store (e.g. "soci", "containerd"). Default: "soci".
- `namespace` (string) — Default: "default".

## config/resolver.go

### [resolver]
#### [resolver.host]
#### [resolver.host.examplehost]
#### [[resolver.host.examplehost.mirrors]]
- `host` (string) — hostname. Default: "".
- `insecure` (bool) — Allows usage of http instead of https only. Default: true.
- `request_timeout_sec` (int) — Timeout in seconds of each request to the registry. Default: infinity.

## config/service.go

### [snapshotter]
- `min_layer_size` (int) — Sets the minimum threshold for lazy loading a layer. Any layer smaller than this value will ignore the zTOC for the layer and pull the entire layer ahead of time. We generally recommend setting it to 10MiB (10000000). Default: 0.
- `allow_invalid_mounts_on_restart` (bool) — Allows the snapshotter to start even if preexisting snapshots cannot connect to their data source on startup. Useful on unexpected daemon crashes/corruption. Default: false.
