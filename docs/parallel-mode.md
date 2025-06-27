# Parallel Pull and Unpack Mode

## Introduction

The SOCI Snapshotter's lazy loading technology has proven effective in accelerating container startup times. The technology defers the full image download and unpacking until the container actually needs the files, allowing for near-instant container launches.

However, some users prefer the traditional upfront loading model, where the entire container image is pulled and prepared before the container is started. This is particularly the case for I/O-bound applications that require fast access to the full rootfs.

To support users who favor the upfront loading workflow, we have introduced a new _parallel-pull-unpack_ mode in the SOCI Snapshotter. This mode enables concurrent processing of container image layers, significantly improving the performance of the upfront loading process.

## Functionality
 
The _parallel-pull-unpack_ mode operates as follows:

* Parallel Layer Downloads: When preparing a container image snapshot, the snapshotter initiates concurrent HTTP range GET requests to fetch the individual image layers. Each layer is downloaded in parallel using multiple HTTP connections, with the downloaded data being written to a temporary file on disk rather than buffered in memory.

* Parallel Layer Unpacking: After the layers are downloaded, the snapshotter also launches parallel unpacking operations. Multiple layers can be decompressed and extracted concurrently.

## Use Cases

The _parallel-pull-unpack_ mode is most beneficial in the following scenarios:

* Complex, Multi-Layer Images: Applications that rely on container images with a large number of layers, such as machine learning frameworks or language runtimes, stand to gain the most from the parallel processing capabilities.

* High-Performance Environments: The _parallel-pull-unpack_ mode is optimized for compute-intensive environments with abundant system resources, such as large AWS EC2 instances with 64 or more CPU cores. These systems can effectively leverage the parallelism to maximize throughput and minimize image pull/unpack latency.

## Configuration

The _parallel-pull-unpack_ mode is configured through the following parameters. All default values are set to mimic containerd's default settings, providing a conservative and safe baseline. However, we encourage you to fine-tune these settings to get the best performance for your specific workloads and environment.

* `enable`: Enables or disables the _parallel-pull-unpack_ mode. Default is false.
* `max_concurrent_downloads`: Sets the maximum number of concurrent downloads allowed across all images. Default is -1 (unlimited).
* `max_concurrent_downloads_per_image`: Limits the maximum concurrent downloads per individual image. Default is 3.
* `concurrent_download_chunk_size`: Specifies the size of each download chunk when pulling image layers in parallel. Default is empty (size of layer).
* `max_concurrent_unpacks`: Sets the maximum number of concurrent layer unpacking operations system-wide. Default is -1 (unlimited).
* `max_concurrent_unpacks_per_image`: Sets the limit for concurrent unpacking of layers per image. Default is 1.
* `discard_unpacked_layers`: Controls whether to retain layer blobs after unpacking. Enabling this can reduce disk space usage and speed up pull times. Default is false.
* `decompress_streams`: Allows customizing the decompressor executable used for layer extraction. Default is "unpigz".

### About Decompress Streams

The decompress streams configuration enables the snapshotter to use external compression implementations for decompressing image layers. By default, the snapshotter will use the implementation from containerd's compression library. It is the user's responsibility to ensure any custom external compression implementations are installed on the system. If the implementation is configured but is not installed, then the snapshotter will fail its configuration validation and fail to start.

Note: For security reasons, the snapshotter implements several safeguards when executing external decompression commands:

* **Path validation**: The path to the executable must be an absolute path to a regular file with executable permissions.
* **Direct execution without shell**: The snapshotter executes decompression commands directly using Go's `os/exec` package without invoking a shell. This means:
   - Each command is executed as a single process with the exact arguments specified;
   - Shell operators like `&&`, `||`, `;`, `|`, or `>` are treated as literal arguments, not command separators;
   - Command chaining (e.g., `cmd1 && cmd2`) is not possible as these are passed as literal strings to the executable;
* **No shell expansion**: Shell features like wildcards (`*`), variable substitution (`$VAR`), command substitution (`$(cmd)`), etc. are not processed or expanded when executing decompression commands.

These security measures are in place to prevent command injection attacks. If any of these validations fail, the snapshotter will fail to start.

### Example

Here's a sample configuration that demonstrates how to set up the _parallel-pull-unpack_ mode:

```
[content_store]
  type = "containerd"

[pull_modes.parallel_pull_unpack]
  enable = true
  max_concurrent_downloads = 50
  max_concurrent_downloads_per_image = 10
  concurrent_download_chunk_size = "8mb"
  max_concurrent_unpacks = 20
  max_concurrent_unpacks_per_image = 10
  discard_unpacked_layers = true
  
  [pull_modes.parallel_pull_unpack.decompress_streams."gzip"]
    path = "/usr/bin/unpigz"
    args = ["-d", "-c"]
```

## Performance Tuning

Achieving optimal performance with the _parallel-pull-unpack_ mode requires careful consideration of your system resources and container image characteristics. The following guidelines can help you tune the configuration:

* **CPU Resources**: The parallel processing capabilities are directly influenced by the available CPU cores. For maximum performance, we recommend deploying the snapshotter on compute-optimized hosts with at least 64 CPU cores.

* **Memory Capacity**: Parallel operations, such as concurrent downloads and unpacking, require sufficient memory. We suggest using instances with at least 128 GB of RAM to allow for efficient buffering and decompression.

* **Storage Performance**: Image pulls are I/O-intensive, and the parallel processing can saturate storage throughput. Ensure your storage subsystem (e.g., AWS EBS volumes) is provisioned with high IOPS and throughput to avoid becoming a bottleneck.

* **Image Characteristics**: Images with a large number of layers tend to benefit the most from the parallel processing capabilities. Tune the `max_concurrent_unpacks` and `max_concurrent_unpacks_per_image` settings to match the layer count of your workloads.

* **Layer Sizes**: For images with large individual layers, optimizing the `concurrent_download_chunk_size` and `max_concurrent_downloads` settings can help maximize download performance. For images hosted on AWS ECR, the recommended chunk size is "8mb" or "16mb", and 10-20 concurrent downloads per image. You may further increase the values if you have very large layers.

Please monitor your system's CPU, memory, and storage utilization when running the _parallel-pull-unpack_ mode. Adjust the configuration parameters as needed to find the right balance between performance gains and system stability. We recommend enabling this mode on high-performance infrastructure with ample CPU, memory, and storage resources.

If you have any questions or need further assistance, please don't hesitate to reach out to the SOCI Snapshotter community.
