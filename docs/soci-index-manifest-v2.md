# SOCI Index Manifest v2

## Introduction

The SOCI snapshotter lazily loads container images using a pre-built metadata artifact called the SOCI Index. The SOCI index is used in place of the original image to configure the container’s root filesystem and to lazily load its contents. You can either specify the digest of the SOCI index when launching a container or, for convenience, the SOCI snapshotter will look up an associated SOCI index using the OCI referrers API. By using the referrers API, you can enable lazy loading with SOCI without any modification to your image or deployment.

The cost for the convenience of the referrers API is a weak, mutable reference between the SOCI index and the container image. Anyone with write access to the image registry can add or delete a SOCI index at any time which can affect the runtime characteristics of the associated image. This usage of a SOCI index happens automatically, with no user input and potentially no user awareness, as long as SOCI is configured in containerd. This can result in unpredictable performance and makes it hard to get visibility into whether SOCI was even used.

To address these issues, we introduced SOCI Index Manifest v2 - a new way to package SOCI indexes into SOCI-enabled images.

## SOCI Index Manifest v2

SOCI Index Manifest v2 uses a lightweight image conversion step to package your image and SOCI index into a single, strongly-linked, SOCI-enabled image. You can copy the new SOCI-enabled image between registries like you would for any other multi-architecture image and the SOCI index will move along with it. When you pull the SOCI-enabled image, the SOCI snapshotter uses the packaged SOCI index to lazily load the container.

Unlike before, SOCI Index Manifest v2 cannot be added to or removed from a SOCI-enabled image. This guarantees that the image’s behavior will not change after creation.
While SOCI-enabled images are additional images compared to SOCI Index Manifest v1, they reuse image contents. The layers, which make up the majority of the data in your images, are shared between a SOCI-enabled image and the original image. The storage overhead of a SOCI-enabled image is a few metadata artifacts that will only add a few kilobytes of storage.

## Migrating from SOCI Index Manifest v1

The SOCI snapshotter v0.10.0 will no longer consume SOCI Index Manifest v1 by default. Your existing images will not be lazily loaded after updating. To upgrade to SOCI Index Manifest v2, use the SOCI CLI to convert your images into SOCI-enabled images. You can push the SOCI-enabled images to your registry like any other image which will include the new SOCI indexes. You can pull the SOCI-enabled images with the SOCI snapshotter to get the benefits of lazy loading.

A sample workflow:

```
sudo nerdctl pull --all-platforms 123456789012.dkr.us-west-2.ecr.amazonaws.com/example:latest
sudo soci convert --all-platforms 123456789012.dkr.us-west-2.ecr.amazonaws.com/example:latest \
    123456789012.dkr.us-west-2.ecr.amazonaws.com/example:latest-soci
sudo nerdctl push --all-platforms \
    123456789012.dkr.us-west-2.ecr.amazonaws.com/example:latest-soci
```

In your execution environment:

```
sudo nerdctl pull --snapshotter soci \
    123456789012.dkr.us-west-2.ecr.amazonaws.com/example:latest-soci
```

See [the getting started guide](./getting-started.md) for more information.

## Continuing to Use SOCI Index Manifest v1

There are some use-cases where the dynamic nature of SOCI Index Manifest v1 adds significant value. For those users, SOCI Index Manifest v1 is still available in the SOCI snapshotter; however, it is disabled by default. By re-enabling it, you should be aware that adding a SOCI index to an existing image can change the runtime characteristics of the image. We strongly recommend that you migrate to SOCI-enabled images and SOCI Index Manifest V2 which immutably binds a SOCI index to a new image and allows you to manage changes to your production workloads through deployments, just like you would for any other change.

If you would like to enable SOCI v1, you can add the following to the soci snapshotter config (located at `/var/lib/soci-snapshotter-grpc/config.toml`):

```
[pull_modes.soci_v1]
enable = true
```

## Comparing SOCI Index Manifest v1 and SOCI Index Manifest v2

SOCI Index Manifest v1 and SOCI Index Manifest v2 both allow lazily loading container images. SOCI Index Manifest v1 is a standalone artifact that can be discovered using the OCI Referrers API. SOCI Index Manifest v2 uses SOCI-enabled images where the SOCI index is bundled with the original image into a single, multi-architecture image. The tables below highlight the benefits and tradeoffs of each approach:


**SOCI Index Manifest v2**

| Pros | Cons |
|------|------|
| Combines SOCI index and image into a single artifact | Requires extra image management in registries |
| Image properties don't change dynamically without deployments | |
| SOCI index moves across registries with the image automatically | |
| Allows managed rollout via normal deployments | |
| Shares image content with the original image | |

**SOCI Index Manifest v1**

| Pros | Cons |
|------|------|
| Standalone artifact that can be managed independently of the image | Dynamically adding and removing SOCI indexes can affect existing images outside of deployments (e.g. in scaling events, redeployments, etc) |
| Can be added and removed without affecting the original image | SOCI index is not automatically copied between registries |
| Shares image content with the original image | |
