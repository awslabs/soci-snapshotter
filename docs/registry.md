 # Registry Compatibility with SOCI

SOCI is compatible with most registries. To check if your registry of choice is compatible, see [List of Registry Compatibility](#list-of-registry-compatibility).

For most use-cases, compatibility is the only concern. However, there is a difference in *how* registries work with SOCI that could cause surprising edge cases. The rest of this document is a technical dive into how SOCI indices are stored and retrieved from registries and the surprises you might encounter.

<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->

- [Registry Requirements](#registry-requirements)
- [Referrers API vs Fallback](#referrers-api-vs-fallback)
  - [Referrers API](#referrers-api)
  - [Fallback](#fallback)
- [How SOCI Indices Appear to Registries](#how-soci-indices-appear-to-registries)
- [List of Registry Compatibility](#list-of-registry-compatibility)
  - [Failure Examples](#failure-examples)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

## Registry Requirements

In order for a registry to be compatible SOCI it must support the following features of the OCI distribution and image specs:

1) Accept [OCI Image Manifests](https://github.com/opencontainers/image-spec/blob/v1.1.0/manifest.md) with [subject fields](https://github.com/opencontainers/image-spec/blob/v1.1.0/manifest.md#image-manifest-property-descriptions) and arbitrary config media types.
This allows the registry to store SOCI indices and is supported by most registries.

2) (optional) Support the [OCI referrers API](https://github.com/opencontainers/distribution-spec/blob/v1.1.0/spec.md#listing-referrers).
This adds convenience around retrieving SOCI indices from the registry. If it is not supported, there is a [fallback mechanism](https://github.com/opencontainers/distribution-spec/blob/v1.1.0/spec.md#unavailable-referrers-api) that works with all registries, but it has a few issues noted in the next section.

## Referrers API vs Fallback

The SOCI snapshotter can retrieve SOCI indices and ztocs either through the [OCI referrers API](https://github.com/opencontainers/distribution-spec/blob/v1.1.0/spec.md#listing-referrers) or a [Fallback mechanism](https://github.com/opencontainers/distribution-spec/blob/v1.1.0/spec.md#unavailable-referrers-api). The referrers API is part of the not-yet-released [OCI Distribution Spec v1.1](https://github.com/opencontainers/distribution-spec/blob/v1.1.0/spec.md) so registry support is limited. The Fallback is supported by all registries, but has notable edge cases.

The SOCI CLI and the SOCI snapshotter automatically uses the referrers API if the registry supports it or the fallback mechanism otherwise.

### Referrers API

The [referrers API](https://github.com/opencontainers/distribution-spec/blob/v1.1.0/spec.md#listing-referrers) is a registry endpoint where an agent can query for all artifacts that reference a given image digest, optionally filtering by artifact type. The registry indexes artifacts for the referrers API when the artifact is pushed. When a container is launched, the SOCI snapshotter can query this API to find SOCI indices that reference the digest of the image.

### Fallback

If the referrers API is not available, the OCI distribution spec defines a [fallback mechanism that works with existing registries](https://github.com/opencontainers/distribution-spec/blob/v1.1.0/spec.md#unavailable-referrers-api). In this mechanism, the contents that would normally be returned by the referrers API are instead put into an [OCI Image Index](https://github.com/opencontainers/image-spec/blob/v1.1.0/image-index.md) which is tagged in the registry with the digest of the manifest to which it refers.

For example, imagine you had an image `myregistry.com/image:latest` with digest `sha:123`. If you created and pushed a SOCI index for that image, there would also be a new image index `myregistry.com/image:sha-123` which contains the SOCI index' descriptor. At runtime, the SOCI snapshotter will pull the `myregistry.com/image:sha-123` index and apply client side filtering to discover the SOCI index.

For clarity in the rest of this section, we will refer to `myregistry.com/image:sha-123` as the "fallback" (as opposed to image index) to distinguish it from the SOCI index.

An important note here is that the fallback is managed on the *client side* by the tool performing the push. There is therefore a race condition when pushing a SOCI index because the fallback has to be pulled, modified to add the new SOCI index, and then pushed back to the registry. If a second artifact is pushed that references the same image digest, then one modification of the fallback could clobber the other.

To clarify the scope of this problem, the fallback is unique per image digest. Multiple artifacts (SOCI Indices, signatures, etc.) can modify the same fallback. The image digest is generally unique per image/platform pair. As an example of what this means in practice, concurrently creating a SOCI index for the image for platforms `linux/amd64` and `linux/i386` is safe because the image digests will be different. Concurrently creating a SOCI index and signature for an image and platform `linux/amd64` is unsafe because both artifacts will refer to the same image digest.

Since the fallback is managed client side, the registry does not know about the relationship between SOCI indices and the fallback. Deleting a SOCI index will not delete or modify the fallback. It is up to the user to make the necessary modifications or deletions of the fallback when deleting a SOCI index from the registry.

## How SOCI Indices Appear to Registries

Each registry will display information in a slightly different mechanism, but here we show what artifacts might show up in your repository and an explanation of what they are:


| Tag         | Type        | Explanation                                                                                                |
| ----------- | ----------- | ---------------------------------------------------------------------------------------------------------- |
| latest      | Image       | The actual image                                                                                           |
| \<untagged> | Image       | The SOCI index manifest. This may appear as type SOCI Index or Other                                       |
| sha:123     | Image Index | The fallback image index. This will only be present for registries which do not support the referrers API. |

## List of Registry Compatibility

Registries that are not listed have not been tested by the SOCI maintainers or reported by the community, but they may still be compatible SOCI.

| Registry                                                                                  | Compatible? | Mechanism     | Notes                                                |
| ----------------------------------------------------------------------------------------- | ----------- | ------------- | ---------------------------------------------------- |
| [Docker Hub](https://hub.docker.com)                                                      | No          | N/A           | Does not support image manifests with subject fields |
| [Amazon Elastic Container Registry (ECR)](https://aws.amazon.com/ecr/)                    | Yes         | Fallback      |                                                      |
| [Amazon ECR Public Gallery](https://gallery.ecr.aws)                                      | Yes         | Fallback      |                                                      |
| [Azure Container Registry](https://azure.microsoft.com/en-us/products/container-registry) | Yes         | Referrers API |                                                      |
| [GitHub Packages (GHCR)](https://github.com/features/packages)                            | Yes         | Fallback      |                                                      |
| [Google Cloud Container Registry (GCR)](https://cloud.google.com/container-registry)      | Yes         | Fallback      |                                                      |
| [Google Cloud Artifact Registry (AR)](https://cloud.google.com/artifact-registry)         | No          | N/A           | Testing the referrers API redirects to login         |
| [Quay.io](https://quay.io)                                                                | No          | N/A           | Does not support image manifests with subject fields |
| [Artifactory](https://jfrog.com/artifactory/)                                             | Yes         | Fallback      |                                                      |
| [Harbor](https://github.com/goharbor/harbor)                                              | Yes         | Referrers API (>= v2.8.1)<br/>Fallback (< v2.8.1) | For versions >= v2.8.1 and < v2.11.0, harbor had a bug in the referrers API implementation that prevents SOCI from pulling indexes. Upgrade to >= v2.11.0 to fix the issue |
| [Distribution](https://github.com/distribution/distribution)                              | Yes         | Fallback      |                                                      |
| [OCI-playground Distribution](https://github.com/oci-playground/distribution)             | Yes         | Referrers API |                                                      |

### Failure Examples

Below are some slightly redacted examples from the services that don't support the features needed to be compatible SOCI.

**Docker Hub**

```
$ sudo ./out/soci push --user $USERNAME:$PASSWORD docker.io/####/busybox:latest
checking if a soci index already exists in remote repository...
pushing soci index with digest: sha256:d6ebffd218ead37e4862172b4f19491341e72aebc3cc6d9cf1a22297c40fb3c2
pushing artifact with digest: sha256:cce4c7e12e01b32151d69348fcf52e0db7b44f6df6c23c511fa5c52eaf272c28
pushing artifact with digest: sha256:44136fa355b3678a1146ad16f7e8649e94fb4fc21fe77e8310c060f61caaff8a
skipped artifact with digest: sha256:acaddd9ed544f7baf3373064064a51250b14cfe3ec604d65765a53da5958e5f5
successfully pushed artifact with digest: sha256:44136fa355b3678a1146ad16f7e8649e94fb4fc21fe77e8310c060f61caaff8a
successfully pushed artifact with digest: sha256:cce4c7e12e01b32151d69348fcf52e0db7b44f6df6c23c511fa5c52eaf272c28
pushing artifact with digest: sha256:d6ebffd218ead37e4862172b4f19491341e72aebc3cc6d9cf1a22297c40fb3c2
soci: error pushing graph to remote: PUT "https://registry-1.docker.io/v2/####/busybox/manifests/sha256:d6ebffd218ead37e4862172b4f19491341e72aebc3cc6d9cf1a22297c40fb3c2": response status code 404: notfound: not found
```

The index manifest can't be pushed at all.

**Google Cloud Artifact Registry**

```
$ sudo ./out/soci push --user $USERNAME:$PASSWORD us-east1-docker.pkg.dev/####/busybox:latest
checking if a soci index already exists in remote repository...
soci: failed to fetch list of referrers: GET "https://accounts.google.com/v3/signin/identifier?dsh=###&continue=https%3A%2F%2Fconsole.cloud.google.com%2Fartifacts%2Ftags%2Fv2%2Fus-east1%2F####%252Fbusybox%252Freferrers%252Fsha256%2Facaddd9ed544f7baf3373064064a51250b14cfe3ec604d65765a53da5958e5f5...&service=cloudconsole&flowName=WebLiteSignIn&flowEntry=ServiceLogin": failed to decode response: invalid character '<' looking for beginning of value
```

This is before pushing the SOCI index. Looking up an existing referrer is redirecting to an auth flow.

Pushing a SOCI index works as long as the SOCI CLI receives the `--existing-index allow` flag to skip the check for existing indices. This is because Artifact Registry appears to always redirect requests to the referrers API to an auth flow, despite other authenticated requests to the registry succeeding as expected. Pulling always falls back to OverlayFS due to the redirect.

**Quay.io**

```
$ sudo ./out/soci push --user $USERNAME:$PASSWORD quay.io/####/busybox:latest
checking if a soci index already exists in remote repository...
pushing soci index with digest: sha256:3f2f40d12b70b94e43f17b3840cd0dd850d6ce497f80cee9515fe4f7253d176d
skipped artifact with digest: sha256:44136fa355b3678a1146ad16f7e8649e94fb4fc21fe77e8310c060f61caaff8a
skipped artifact with digest: sha256:acaddd9ed544f7baf3373064064a51250b14cfe3ec604d65765a53da5958e5f5
skipped artifact with digest: sha256:ca306a7641ef2ca78cb69ce48bba4381263459a86fe3efad34ad31ca1c2bc2df
pushing artifact with digest: sha256:3f2f40d12b70b94e43f17b3840cd0dd850d6ce497f80cee9515fe4f7253d176d
soci: error pushing graph to remote: failed to push referrers index tagged by sha256-acaddd9ed544f7baf3373064064a51250b14cfe3ec604d65765a53da5958e5f5: PUT "https://quay.io/v2/####/busybox/manifests/sha256-acaddd9ed544f7baf3373064064a51250b14cfe3ec604d65765a53da5958e5f5": response status code 400: Bad Request
```

The index manifest is pushed every time the command is run, indicating that it's not being found after the push. We can't push the fallback at all, presumably because it contains the index manifest digest which isn't found.
