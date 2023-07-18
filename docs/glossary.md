The SOCI project introduces several new terms that sometimes have subtle differences between them. This glossary defines these terms.

## Terminology 

* __SOCI__: Seekable OCI (pronounced so-CHEE). SOCI combines an unmodified
  [OCI Image](https://github.com/opencontainers/image-spec/blob/v1.1.0-rc4/spec.md) (or Docker v2
  image) with a SOCI index to enable the SOCI snapshotter to lazily pull the image at runtime.

* __SOCI index__: An OCI artifact consisting of a SOCI index manifest and a set of zTOCs
  that enable lazy loading of unmodified OCI images. "Index" refers to the whole set of
  objects similarly to how "image" refers to the set of image index, manifest, config, and
  layers.

* __SOCI index manifest__: An
  [OCI Image manifest](https://github.com/opencontainers/image-spec/blob/v1.1.0-rc4/manifest.md)
  containing the list of zTOCs in the SOCI Index with a Subject reference to the image for
  which the manifest was generated.

* __zTOC__: A Table of Contents for compressed data. A zTOC is composed of 2 parts. 1) a
  table of contents containing file metadata and its offset in the decompressed TAR
  archive (the "TOC"). 2) A collection of "checkpoints" of the state of the compression
  engine at various points in the layer. We refer to this collection as the "zInfo".

* __span__: A chunk of data that can be independently decompressed. Each checkpoint in the zInfo
  corresponds to exactly one span in an image layer.

## Anti-terminology

* __SOCI Image__: We generally avoid the term "SOCI Image" because there is no such thing!
  The image is an unmodified OCI image. Also, a single image may have many SOCI indices
  with different parameters such as span size, layers indexed, etc. The precise way to
  refer to an image that has a SOCI index is to refer to the index itself.
