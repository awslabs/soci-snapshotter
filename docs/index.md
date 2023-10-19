# SOCI Index

<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->

- [Conceptual Data Model](#conceptual-data-model)
  - [SOCI Index Manifest](#soci-index-manifest)
  - [zTOC](#ztoc)
    - [TOC](#toc)
    - [zInfo](#zinfo)
- [Physical Data Model](#physical-data-model)
  - [SOCI Index Property Descriptions](#soci-index-property-descriptions)
  - [Entities and Relationships](#entities-and-relationships)
  - [Example Serialized SOCI Index](#example-serialized-soci-index)
  - [Example Serialized zTOC](#example-serialized-ztoc)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

## Conceptual Data Model

The high-level components of the SOCI index include:

* [SOCI Index Manifest](#soci-index-manifest) - an OCI image manifest describing the
content of the SOCI Index.
* [zTOC](#ztoc) - a table of contents for compressed data.

### SOCI Index Manifest

The SOCI index manifest contains a subject reference to the image and a list of zTOCs.

* `Subject` _[descriptor](https://github.com/opencontainers/image-spec/blob/v1.1.0-rc5/descriptor.md)_

    Reference to the image from which the index was generated.

* `zTOCs` _array of [descriptor](https://github.com/opencontainers/image-spec/blob/v1.1.0-rc5/descriptor.md)_

![manifest-concept](images/soci-index-manifest-conceptual-data-model.drawio.svg)

### zTOC

A compression table of contents, or zTOC, is composed of 2 parts:

1. TOC - a table of contents containing file metadata and the file's offset
in the decompressed TAR.
1. zInfo - a collection of checkpoints of the state of the compression
engine at various points in the layer. Also referred to as compression info.

#### TOC

The table of contents contains the file metadata for each file in the TAR
needed to lazy load file content.

File metadata such as:

* Name
* Type
* Size
* Permissions
* Timestamp

In addition to the "normal" Linux filesystem metadata, the TOC contains:

* Offset - the offset of the file in the uncompressed TAR file of the layer

#### zInfo

Each checkpoint in the collection contains the state needed
to decompress a specific region of the layer TAR.
This region is known as a span.

The following figure builds a data model for a layer TAR and its zTOC:

![ztoc-concept](images/soci-index-ztoc-conceptual-data-model.drawio.svg)

## Physical Data Model

The SOCI index is packaged as an
[OCI image manifest](https://github.com/opencontainers/image-spec/blob/v1.1.0-rc5/manifest.md).
This enables the SOCI index to be stored and distributed alongside its OCI image.

See [guidelines for artifact usage](https://github.com/opencontainers/image-spec/blob/v1.1.0-rc5/manifest.md#guidelines-for-artifact-usage)
for more information on packaging content using OCI image manifest.

### SOCI Index Property Descriptions

* `schemaVersion` _int_

    This value MUST be `2` for backwards compatibility with Docker.

* `mediaType` _string_

    This value MUST contain the media type `application/vnd.oci.image.manifest.v1+json`.

* `config` _[descriptor](https://github.com/opencontainers/image-spec/blob/v1.1.0-rc5/descriptor.md)_

    A REQUIRED property for OCI image manifests to maintain portability. See
    [guidance for an empty descriptor](https://github.com/opencontainers/image-spec/blob/v1.1.0-rc5/manifest.md#guidance-for-an-empty-descriptor)
    for more information.

    * `mediaType` _string_

        This REQUIRED property MUST be set to `application/vnd.amazon.soci.index.v1+json`.

    * `digest` _string_

        This REQUIRED property is the digest of the empty JSON payload: `{}`.

    * `size` _int64_

        This REQUIRED descriptor property specifies the size, in bytes, of the empty JSON payload (`size` of 2).

* `subject` _[descriptor](https://github.com/opencontainers/image-spec/blob/v1.1.0-rc5/descriptor.md)_

    A REQUIRED property which is used to specify the descriptor of the
    image manifest from which the index was generated. This value is used by the
    [referrers API](https://github.com/opencontainers/distribution-spec/blob/v1.1.0-rc3/spec.md#listing-referrers)
    and associates a SOCI index with an OCI image.

    * `mediaType` _string_

        This REQUIRED property specifies the media type of the OCI image
        manifest from which the SOCI index was generated. The media type MUST
        [be compatible](https://github.com/opencontainers/image-spec/blob/v1.1.0-rc5/media-types.md#compatibility-matrix)
        with OCI image manifest.

    * `digest` _string_

        This REQUIRED property specifies the digest of the OCI image manifest
        from which the SOCI index was generated.

    * `size` _int64_

        This REQUIRED property specifies the size, in bytes, of the OCI image
        manifest from which the SOCI index was generated.

* `layers` _array of [descriptor](https://github.com/opencontainers/image-spec/blob/v1.1.0-rc5/descriptor.md)_

    An ordered array where each item in the array is a serialized representation
    of a zTOC. See [zTOC](#ztoc) section.

    **Note: the ordering MUST be consistent with the ordering of the layers from the OCI image. This consistent ordering ensures index building is deterministic.**

* `annotations` _string-string map_

    An OPTIONAL OCI image manifest property which contains arbitrary metadata for the SOCI index.

    * `"com.amazon.soci.build-tool-identifier"` can be used to identify the version of SOCI CLI used to generate the index.

### Entities and Relationships

The following figure shows the relationship between an OCI image and its SOCI index:

![entities-and-relations](images/soci-index-entities-and-relationships.drawio.svg)

### Example Serialized SOCI Index

_Default SOCI Index for
public.ecr.aws/docker/library/rabbitmq@sha256:8af9cc0bd40bcd466e5935e25c08ad2306f9a25091b0d145492d0696888851dc:_

```
{
    "annotations": {
        "com.amazon.soci.build-tool-identifier": "AWS SOCI CLI v0.1"
    },
    "config": {
        "digest": "sha256:44136fa355b3678a1146ad16f7e8649e94fb4fc21fe77e8310c060f61caaff8a",
        "mediaType": "application/vnd.amazon.soci.index.v1+json",
        "size": 2
    },
    "layers": [
        {
            "annotations": {
                "com.amazon.soci.image-layer-digest": "sha256:aece8493d3972efa43bfd4ee3cdba659c0f787f8f59c82fb3e48c87cbb22a12e",
                "com.amazon.soci.image-layer-mediaType": "application/vnd.oci.image.layer.v1.tar+gzip"
            },
            "digest": "sha256:cb011209b93fde5a7a698302ca2e2dd4928278da32a31995c5df6d16cb27c638",
            "mediaType": "application/octet-stream",
            "size": 1229704
        },
        {
            "annotations": {
                "com.amazon.soci.image-layer-digest": "sha256:e8b79c92d68de7b64f81149d112547255d8edecba2f64054a533954ee3c106fe",
                "com.amazon.soci.image-layer-mediaType": "application/vnd.oci.image.layer.v1.tar+gzip"
            },
            "digest": "sha256:7ae9f4713a78a4895a7a522117c552a519d8289eecc446820b1b528cf9257b82",
            "mediaType": "application/octet-stream",
            "size": 841192
        },
        {
            "annotations": {
                "com.amazon.soci.image-layer-digest": "sha256:bb157f88c818b56b154696493271c0d7a2704ed253fc2111d0a96fa89bf44984",
                "com.amazon.soci.image-layer-mediaType": "application/vnd.oci.image.layer.v1.tar+gzip"
            },
            "digest": "sha256:715ee958c9cff9bca859e940cb05a3937bd2a6d2570fec910602df095339b067",
            "mediaType": "application/octet-stream",
            "size": 1086672
        }
    ],
    "mediaType": "application/vnd.oci.image.manifest.v1+json",
    "schemaVersion": 2,
    "subject": {
        "digest": "sha256:ab5a62720aa71c964aae400825284ca3e8229934928394e68a06e395a225faae",
        "mediaType": "application/vnd.oci.image.manifest.v1+json",
        "size": 2752
    }
}
```

### Example Serialized zTOC

**Note: the following is a *human readable* format of a zTOC which
is serialized as a binary file on disk. Some pieces, like compression
checkpoint information, have been redacted. For brievity, some data
has been excluded. Denoted by ellipsis (...).**

_zTOC sha256:715ee958c9cff9bca859e940cb05a3937bd2a6d2570fec910602df095339b067 for
public.ecr.aws/docker/library/rabbitmq@sha256:8af9cc0bd40bcd466e5935e25c08ad2306f9a25091b0d145492d0696888851dc:_

```
{
  "version": "0.9",
  "build_tool": "AWS SOCI CLI v0.1",
  "size": 1086672,
  "span_size": 4194304,
  "num_spans": 9,
  "num_files": 4102,
  "num_multi_span_files": 8,
  "files": [
    {
      "filename": "etc/",
      "offset": 512,
      "size": 0,
      "type": "dir",
      "start_span": 0,
      "end_span": 0
    },
    {
      "filename": "etc/ca-certificates/",
      "offset": 1024,
      "size": 0,
      "type": "dir",
      "start_span": 0,
      "end_span": 0
    },
    ... ,
    {
      "filename": "var/log/dpkg.log",
      "offset": 34542592,
      "size": 202359,
      "type": "reg",
      "start_span": 8,
      "end_span": 8
    }
  ]
}
```
