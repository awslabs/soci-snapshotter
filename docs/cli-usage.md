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
- [soci push](#soci-push)
- [soci rebuild_db](#soci-rebuild_db)
- [soci index](#soci-index)
- [soci ztoc](#soci-ztoc)


### soci create
Creates SOCI index for an image

Output of this command is SOCI layers and SOCI index stored in a local directory
> [!NOTE] 
> SOCI layer is named as \<image-layer-digest>.soci.layer
>
> SOCI index is named as \<image-manifest-digest>.soci.index


Usage: ```soci create [flags] <image_ref> ```

Flags:

 - ```--span-size``` : Span size that soci index uses to segment layer data. Default is 4MiB 
 - ```--min-layer-size``` : Minimum layer size to build zTOC for. Smaller layers won't have zTOC and not lazy pulled. Default is 10MiB

**Example:** 
```
soci create public.ecr.aws/soci-workshop-examples/ffmpeg:latest
```

### soci push
Push SOCI artifacts to a registry by image reference.
If multiple soci indices exist for the given image, the most recent one will be pushed.

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
Use this command after image pull so that indices/ztocs can be discovered by commands like "```soci index list```", and after "```index rm```" when using the containerd content store so that deleted orphaned zTOCs will be forgotten

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
    - ```--verbose```, ```-v``` : Display extra debugging messages

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
