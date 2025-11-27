# SOCI Prefetch

> The prefetch feature is currently experimental and subject to change. APIs, command-line interfaces, and artifact formats may change in future releases.

## Overview

SOCI prefetch is a mechanism that allows users to specify which files should be prioritized and prefetched at container startup time. By prefetching critical data before the container starts, you can significantly reduce first-access latency and improve time to readiness for cold starts.

## Motivation

Container startup is often dominated by data transfer, but only a small portion of bytes are needed initially. While lazy loading helps, many workloads still benefit from warming up a small, targeted set of files or sub-file spans. In practice:

1. **Different workloads, different needs**: Different workloads sharing one image access different files at startup
2. **Storage efficiency**: Producing workload-specific image copies increases storage, reduces cache efficiency, and complicates updates
3. **Sub-file granularity**: Some workloads need sub-file warm-up (e.g., reading headers from many large files)

The prefetch feature provides an opt-in, workload-specific solution with:
- **Flexibility**: Different prefetch sets for the same image
- **Granularity**: File-level and span-level prefetch
- **Compatibility**: No changes required in consumers unaware of the metadata
- **Performance**: Parallel prefetch with fault tolerance

## How It Works

### Architecture Overview

```
┌─────────────────────────────────────────────────────────────┐
│ 1. Index Build Time                                         │
│                                                             │
│  User specifies files → SOCI create/convert                 │
│         ↓                                                   │
│  Find topmost layer per file (respecting overrides)         │
│         ↓                                                   │
│  Compute span ranges for requested files                    │
│         ↓                                                   │
│  Create prefetch artifact (separate from zTOC)              │
│         ↓                                                   │
│  Store in SOCI index as separate layer                      │
└─────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│ 2. Container Startup (Runtime)                              │
│                                                             │
│  containerd → SOCI Snapshotter: Prepare(imageRef)           │
│         ↓                                                   │
│  Download SOCI Index + zTOC + Prefetch Artifacts            │
│         ↓                                                   │
│  Parse prefetch metadata                                    │
│         ↓                                                   │
│  Prefetch specified spans in parallel (best-effort)         │
│         ↓                                                   │
│  Create snapshot mountpoint                                 │
│         ↓                                                   │
│  Container starts (critical data already cached)            │
└─────────────────────────────────────────────────────────────┘
```

### Runtime Behavior

When mounting a layer with prefetch metadata:
1. **Filter**: Select prefetch entries matching the current layer digest
2. **Merge**: Deduplicate and merge overlapping span ranges
3. **Prefetch**: Download spans in parallel with bounded concurrency

## Creating Prefetch Artifacts

### CLI Flags

When building SOCI indices with `soci create` or `soci convert`, you can specify files to prefetch using the `--prefetch-file` flag:

```bash
# Specify individual files (can be used multiple times)
soci create \
  --prefetch-file /app/config.json \
  --prefetch-file /app/lib/core.so \
  --prefetch-file /usr/lib/python3.9/site-packages/torch/__init__.py \
  myimage:tag

# or specify a JSON file
soci create \
  --prefetch-files-json /path/to/prefetch.json
  myimage:tag
```

## Prefetch Artifact Structure

Prefetch artifacts are stored as separate JSON blobs in the SOCI index with media type `application/vnd.amazon.soci.prefetch.v1+json`.

### Data Model

```go
type PrefetchArtifact struct {
    Version       string         `json:"version"`
    PrefetchSpans []PrefetchSpan `json:"prefetch_spans"`
}

type PrefetchSpan struct {
    StartSpan compression.SpanID `json:"start_span"`
    EndSpan   compression.SpanID `json:"end_span"`
    Priority  int                `json:"priority,omitempty"`
}
```

A prefetch span specifies which spans should be prefetched. It contains:
- `start_span`: The first span ID in the range (inclusive)
- `end_span`: The last span ID in the range (inclusive)
- `priority` (optional): Lower values indicate higher priority for future prioritized prefetching support

### Example Prefetch Artifact

```json
{
  "version": "1.0",
  "prefetch_spans": [
    {
      "start_span": 10,
      "end_span": 15,
    },
    {
      "start_span": 50,
      "end_span": 55,
    }
  ]
}
```

### SOCI Index Integration

Prefetch artifacts appear in the SOCI index as separate layers:

```json
{
  "layers": [
    {
      "mediaType": "application/octet-stream",
      "digest": "sha256:c5122dc2ebdc71b2566df2ea2fac2a6ff4558e2fa81f43cad8205683b9c1c501",
      "size": 2685160,
      "annotations": {
        "com.amazon.soci.image-layer-digest": "sha256:eac484c76a4864538cedde18e9a5ced74f7659e11ae5d64bc6712bb5d83bcde8",
        "com.amazon.soci.image-layer-mediaType": "application/vnd.docker.image.rootfs.diff.tar.gzip",
        "com.amazon.soci.span-size": "4194304"
      }
    },
    {
      "mediaType": "application/vnd.amazon.soci.prefetch.v1+json",
      "digest": "sha256:4aa84dfbf34f88299a1a01786d2a6b7fe79059cfc67d0733b3fe8517419413f9",
      "size": 161,
      "annotations": {
        "com.amazon.soci.image-layer-digest": "sha256:eac484c76a4864538cedde18e9a5ced74f7659e11ae5d64bc6712bb5d83bcde8"
      }
    }
  ]
}
```

The prefetch artifact is linked to its layer via the `com.amazon.soci.image-layer-digest` annotation.

## Managing Prefetch Artifacts

### Listing Prefetch Artifacts

Use `soci prefetch ls` to list all prefetch artifacts in the local store:

```bash
$ soci prefetch ls
DIGEST                                                                   LAYER DIGEST                                                             SPANS  CREATED
sha256:4aa84dfbf34f88299a1a01786d2a6b7fe79059cfc67d0733b3fe8517419413f9  sha256:eac484c76a4864538cedde18e9a5ced74f7659e11ae5d64bc6712bb5d83bcde8  12     7h23m ago
```

The output shows:
- **DIGEST**: The unique identifier of the prefetch artifact
- **LAYER DIGEST**: The layer this prefetch artifact applies to
- **SPANS**: Total number of spans to prefetch
- **CREATED**: When the artifact was created

### Viewing Detailed Information

Use `soci prefetch info <digest>` to view detailed information about a specific prefetch artifact:

```bash
$ soci prefetch info sha256:4aa84dfbf34f88299a1a01786d2a6b7fe79059cfc67d0733b3fe8517419413f9
Digest:        sha256:4aa84dfbf34f88299a1a01786d2a6b7fe79059cfc67d0733b3fe8517419413f9
Version:       1.0
Span Ranges:   2
Layer Digest:  sha256:eac484c76a4864538cedde18e9a5ced74f7659e11ae5d64bc6712bb5d83bcde8
Size:          161 bytes
Created:       2025-11-26 18:01:26

Prefetch Spans:
  [0] StartSpan: 10, EndSpan: 15 (covers 6 spans)
      Priority: 0
  [1] StartSpan: 50, EndSpan: 55 (covers 6 spans)
      Priority: 1

Total spans to prefetch: 12
```

## Configuration

### Concurrency Control

The snapshotter implements bounded concurrency for prefetch operations to prevent overwhelming the system when multiple containers start simultaneously.

#### Snapshotter Configuration

Prefetch concurrency is controlled via the snapshotter configuration file (typically `/etc/soci-snapshotter-grpc/config.toml`):

```toml
# Maximum number of layers that can perform prefetch operations concurrently
# at the snapshotter level
# 0 = no limit (default)
# Positive value = maximum concurrent prefetch operations
prefetch_max_concurrency = 0
```

**Configuration options:**
- `0` (default): No limit on concurrent prefetch operations
- Positive integer (e.g., `10`): Maximum number of layers that can prefetch simultaneously

#### Example Configuration

```toml
# /etc/soci-snapshotter-grpc/config.toml

# Limit to 10 concurrent prefetch operations
prefetch_max_concurrency = 10
```
