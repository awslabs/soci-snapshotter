The SOCI project introduces several new terms that sometimes have subtle differences between them. This glossary defines these terms.

## Terminology 

* __SOCI__: Seekable OCI (pronounced so-CHEE). SOCI combines an unmodified
  [OCI Image](https://github.com/opencontainers/image-spec/blob/v1.0.2/spec.md) (or Docker v2
  image) with a SOCI index to enable the SOCI snapshotter to lazily pull the image at runtime.

* __SOCI index__: An OCI artifact consisting of a SOCI index manifest and a set of zTOCs
  that enable lazy loading of unmodified OCI images. "Index" refers to the whole set of
  objects similarly to how "image" refers to the set of image index, manifest, config, and
  layers.

* __SOCI index manifest__: An
  [ORAS manifest](https://github.com/oras-project/artifacts-spec/blob/v1.0.0-rc.2/artifact-manifest.md)
  (soon to be an [OCI Reference Types manifest](https://github.com/opencontainers/wg-reference-types/blob/256c257cc8b725fd324722ee40ead6925b1c8ad8/docs/proposals/PROPOSAL_E.md))
  containing the list of zTOCs in the SOCI Index as well as a reference to the image for
  which the manifest was generated.

* __span__: A chunk of data that can be independently decompressed. A zTOC contains
  periodic "snapshots" of compression state from which a process can resume
  decompression. The chunk of data between two checkpoints is a span.

* __zTOC__: A Table of Contents for compressed data. A zTOC is composed of 2 parts. 1) a
  table of contents containing file metadata and its offset in the decompressed TAR
  archive (the "TOC"). 2) A collection of "snapshots" of the state of the compression
  engine at various points in the layer (the "z").


## Anti-terminology

* __SOCI Image__: We generally avoid the term "SOCI Image" because there is no such thing!
  The image is an unmodified OCI image. Also, a single image may have many SOCI indices
  with different parameters such as span size, layers indexed, etc. The precise way to
  refer to an image that has a SOCI index is to refer to the index itself.
