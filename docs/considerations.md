# Considerations When Using SOCI

Lazy loading with SOCI is a powerful technique to reduce image pull times. However, lazy loading comes with tradeoffs that you should be aware of when deploying SOCI for your production workloads.

## Performance

SOCI’s performance gains come from spreading image pulls across container startup. In a traditional image pull, there is a distinct pull phase where the container runtime downloads and prepares your container image and a run phase where your container starts up and then does work. SOCI blends these together and starts running your container while it is still pulling your image. Because the image pull happens in parallel, individual filesystem accesses may be slower, especially before the image pull completes. This may also result in longer start up time for your container. However, since SOCI removes the separate pull phase, most workloads see an overall reduction in time from when a pull starts to when a container performs useful work.

Here are a few things to consider as a result of SOCI’s performance characteristics:

**Filesystem Latency** - SOCI will have higher filesystem latency during image pulls. If your application is sensitive to latency variation, it may not be a good fit for SOCI.

**Healthchecks** - SOCI may cause the total container start up time to increase. You may need to relax healthcheck timeouts to avoid container startup issues.

## Discovery

The SOCI snapshotter is configured to automatically discover and use SOCI indexes when they are present. As a recommended best practice, you should verify the source of any image and SOCI indexes you use is a known trusted source. This includes when pulling images directly from third-party repositories.

There are differences between how SOCI Index Manifest v1 and SOCI Index Manifest v2 are discovered.

### SOCI Index Manifest v1
SOCI Index Manifest v1 are discovered using the OCI referrers API. When you launch your container, the SOCI snapshotter will check for a SOCI Index Manifest v1 in the registry. It will use the SOCI index if it finds one.

Since the SOCI Index Manifest v1 can be added to the registry outside of a deployment and the SOCI snapshotter will discover it automatically, SOCI Index Manifest v1 support is disabled by default since SOCI v0.10.0.

### SOCI Index Manifest v2
SOCI Index Manifest v2 are discovered in annotations on the image itself. When you launch your container, the SOCI snapshotter will check for an index in the image itself. It will use the SOCI index if it finds one.

SOCI Index Manifest v2 is part of the image which means that you can be sure a SOCI index will not be added or removed without creating a new image. You can use standard deployment mechanisms to safely roll out or roll back your SOCI-enabled images with SOCI Index Manifest v2 by changing the image that you deploy.

> **NOTE**
> Scaling events can still unexpectedly deploy a SOCI Index Manifest v2 if you allow mutable tags (such as `latest`). To address this, you can either require immutable tags or deploy your images by digest instead of by tag.

## Registry Interaction

SOCI is compatible with most container registries.

### SOCI Index Manifest v1

SOCI Index Manifest v1 uses the OCI 1.1 referrers API (or a compatibility fallback for older registries) to associate a SOCI index with an image. The association is one direction: the SOCI index points to the image, but the image is unaware of the SOCI index. As a result, copying the image between registries will not automatically copy the SOCI index. You will need to explicitly copy the SOCI index.

If you copy an image without copying the SOCI index, the SOCI snapshotter will not be able to lazily load it.

### SOCI Index Manifest v2
SOCI Index Manifest v2 combines the image and the SOCI index into a single, multi-platform image. Copying the image between registries will automatically copy the SOCI index as long as the tool you use understands multiple images. This means tools like finch/nerdctl (with the --all-platforms flag), skopeo, and crane will copy the full image including the SOCI index. Docker and Podman will only pull the current platform and therefore will not copy the SOCI index.

If you only copy a single platform, the SOCI snapshotter will not be able to lazy load it. To resolve this, you can re-convert your image and push it using a tool that supports multi-platform images like the ones listed above.
