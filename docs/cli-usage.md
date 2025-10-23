# Using the SOCI CLI

This document provides instructions on how to use the SOCI CLI. And a comprehensive list of all the cli commands in one place.
## Global flags

- ```--address``` , ```-a```: Address for containerd's GRPC server
- ```--namespace```, ```-n```: Namespace to use with commands
- ```--timeout```: Timeout for commands
- ```--debug```: Enable debug output
- ```--content-store```: Use a specific content store (soci or containerd)

## CLI commands
- [soci create](#soci-create)
- [soci convert](#soci-convert)
- [soci push](#soci-push)
- [soci rebuild_db](#soci-rebuild_db)
- [soci index](#soci-index)
- [soci ztoc](#soci-ztoc)


### soci create
Creates SOCI index for an image (SOCI Index Manifest v1)

Output of this command is SOCI layers and SOCI index stored in a local directory
> [!NOTE] 
> SOCI layer is named as \<image-layer-digest>.soci.layer
>
> SOCI index is named as \<image-manifest-digest>.soci.index
>
> This command creates SOCI Index Manifest v1 artifacts. For production use, consider using `soci convert` to create SOCI Index Manifest v2 enabled images.


Usage: ```soci create [flags] <image_ref> ```

Flags:

 - ```--span-size``` : Span size that soci index uses to segment layer data. Default is 4MiB
 - ```--min-layer-size``` : Minimum layer size to build zTOC for. Smaller layers won't have zTOC and not lazy pulled. Default is 10MiB
 - ```--optimizations``` : Enable experimental features by name. Usage is `--optimizations opt_name`.
   - `xattr` :  When true, adds DisableXAttrs annotation to SOCI index. This annotation often helps performance at pull time.
     - There is currently a bug using this on an image with volume-mounts in a layer without whiteout directories or xattrs. If in doubt, do not use this on images with volume mounts.

**Example:**
```
soci create public.ecr.aws/soci-workshop-examples/ffmpeg:latest
```

### soci convert
Converts an OCI image to a SOCI-enabled image (SOCI Index Manifest v2)

This command creates a new SOCI-enabled image that packages the original image and SOCI index into a single, strongly-linked artifact. The SOCI-enabled image can be pushed and deployed like any other image, and the SOCI index will move with it across registries.

> [!NOTE]
> SOCI Index Manifest v2 is the recommended approach for production deployments as it provides immutable binding between the image and SOCI index, preventing runtime behavior changes.

Usage: ```soci convert [flags] <source_image_ref> <dest_image_ref> ```

Flags:

 - ```--span-size``` : Span size that soci index uses to segment layer data. Default is 4MiB
 - ```--min-layer-size``` : Minimum layer size to build zTOC for. Smaller layers won't have zTOC and not lazy pulled. Default is 10MiB
 - ```--optimizations``` : Enable experimental features by name. Usage is `--optimizations opt_name`.
   - `xattr` :  When true, adds DisableXAttrs annotation to SOCI index. This annotation often helps performance at pull time.
 - ```--all-platforms``` : Convert all platforms of a multi-platform image
 - ```--platform``` : Convert only the specified platform (e.g., linux/amd64)

**Example:**
```
soci convert public.ecr.aws/soci-workshop-examples/ffmpeg:latest public.ecr.aws/soci-workshop-examples/ffmpeg:latest-soci
```

**Multi-platform example:**
```
soci convert --all-platforms public.ecr.aws/soci-workshop-examples/ffmpeg:latest public.ecr.aws/soci-workshop-examples/ffmpeg:latest-soci
```

### soci push
Push SOCI artifacts to a registry by image reference (SOCI Index Manifest v1 only)

This command pushes SOCI Index Manifest v1 artifacts to the registry. If multiple soci indices exist for the given image, the most recent one will be pushed.

> [!NOTE]
> This command only works with SOCI Index Manifest v1 artifacts created by `soci create`. For SOCI Index Manifest v2, use standard image push commands (e.g., `nerdctl push`) on the SOCI-enabled image created by `soci convert`.

Usage: ```soci push [flags] <image_ref> ```

> After pushing the soci artifacts, they should be available in the registry. 
Soci artifacts will be pushed only
> if they are available in the snapshotter's local content store.

Flags:

- ```--max-concurrent-uploads```: Max concurrent uploads. Default is 3.
- ```--quiet```, ```-q```: Enable quiet mode

**Example:** 
```
soci push public.ecr.aws/soci-workshop-examples/ffmpeg:latest
```

### soci rebuild_db
Use after pulling an image to discover SOCI indices/ztocs or after "```index rm```" 
when using the containerd content store to clear the database of removed zTOCs.

**Example:** 
```
soci rebuild_db
```

### soci index
Manage indices

Sub commands:

-  ```info``` : Get detailed info about an index.

    Usage: ```soci index info <index-digest>```

    **Example:** 
    ```
    soci index info sha256:5c0f5cb700f596d
    ```

- ```list```, ```ls``` : List indices

    Flags:
    - ```--ref``` : Filter indices to those that are associated with a specific image ref
    - ```--quiet```, ```-q```: Only display the index digests
    - ```--platform```, ```-p```: Filter indices to a specific platform

    **Example:** 
    ```
    soci index ls
    ```
- ```remove```, ```rm``` :  Remove an index from local db, and from content store if supported
    Usage: ```soci index rm <digest>```
    Flags:
    - ```--ref``` : Only remove indices that are associated with a specific image ref

    **Example:** 
    ```
    soci index rm sha256:5c0f5cb700f596d
    ```


### soci ztoc
Sub commands:

- ```info``` : Get detailed info about a ztoc
    
    Usage: ```soci ztoc info <digest>```

-  ```list```, ```ls``` : List ztocs

    Flags: 
    - ```--ztoc-digest```:  Filter ztocs by digest
    - ```--image-ref``` : Filter ztocs to those that are associated with a specific image
    - ```--quiet```, ```-q``` : Only display the index digests

    **Example:** 
    ```
    soci ztoc ls
    ```

- ```get-file```: Retrieve a file from a local image layer using a specified ztoc

    Usage: ```soci ztoc get-file <digest> <file>```

    Flags:
    - ```--output```, ```-o``` : The file to write the extracted content. Defaults to stdout

    **Example:** 
    ```
    soci ztoc get-file sha256:5c0f5cb700f596d index.js
    ```


- ```info``` : Get detailed info about a ztoc

    Usage: ```soci ztoc info <digest>```

    **Example:** 
    ```
    soci ztoc info sha256:5c0f5cb700f596d
    ```
