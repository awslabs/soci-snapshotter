# Benchmarking SOCI snapshotter

This document walks through how to run the SOCI snapshotter benchmarks, including running the benchmarks on default and custom workloads.

There are two types of benchmarks:

```Performance benchmarks```: The performance benchmark focuses on measuring various metrics using the soci snapshotter.

```Comparison benchmarks```: Runs benchmark tests with soci snapshotter and then the same tests with overlayFS snapshotter. We can use these results to compare the difference in performance between the two.

- [Prerequisites](#prerequisites)
- [Running benchmarks on default workloads](#running-benchmarks-on-default-workloads)
- [Running benchmarks on custom workloads](#running-benchmarks-on-custom-workloads)
- [Benchmark binaries cli flags](#benchmark-binaries-have-different-cli-flags)
- [JSON file format for custom workloads](#json-file-format-for-custom-workloads)
- [Default workloads](#default-workloads)
- [Benchmark results format](#benchmark-results)

## Prerequisites

Follow the [Getting started guide](/docs/getting-started.md) and complete setting up the project.


## Running benchmarks on default workloads

There a set of [8 workloads](#default-custom-workloads) that are included in the benchmark binaries that can readily be benchmarked without any additional setup.
These workloads are hosted on a public ECR repository and hence do not need any credentials.

```make benchmarks```  - Runs both the Performance and Comparison benchmark tests on the workloads 5 times

```make benchmarks-perf-test``` - Runs only the Performance benchmark tests on the workloads 5 times

## Running benchmarks on custom workloads

Custom workloads can also be benchmarked with SOCI.

In order to run the benchmarks on custom workloads the custom container image needs to have its soci indices generated and pushed to a contianer registry as described in the [Getting started docs](/docs/getting-started.md)

Generate benchmark binaries:
``` make build-benchmarks``` will generate benchmark binaries for performance testing and comparison testing against overlayFS. The binaries will be available in the ```/benchmark/bin``` folder.

### Benchmark binaries have different cli flags:

| Flag | Description | Required / Optional |
|----------|----------|----------|
| ```-f``` | File path to a json file containing details of multiple images to be tested | Optional
| ```-count``` | Specify number of times the benchmarker needs to run  | Optional
| ```-show-commit``` | Tag the latest commit hash to the benchmark results | Optional

We can now run benchmarks on custom workloads using the ```-f``` flag to specify the file path to a json file containing details of the workloads.

### JSON file format for custom workloads

Ensure that the file being used with the ```-f``` flag follows the following format

```json
{
    "short_name": "<Test_name>",
    "image_ref": "<Container_Image_ref>",
    "ready_line": "<Ready_line>",
    "soci_index_digest": "<Soci_Index_Manifest_Digest>"
}
```

Example :

```json
{
    "short_name": "ffmpeg",
    "image_ref": "public.ecr.aws/soci-workshop-examples/ffmpeg:latest",
    "ready_line": "Hello World",
    "soci_index_digest": "ef63578971ebd8fc700c74c96f81dafab4f3875e9117ef3c5eb7446e169d91cb"
}
```

### Default workloads

| Name             | ECR Repository/Tag  | Description               | Size       |
|------------------|----------------------|---------------------------|------------|
|   ffmpeg    |  public.ecr.aws/soci-workshop-examples/ffmpeg:latest  |   A minimalist Docker image converting a video file format using ffmpeg. | Medium(~600MB) |
| tensor_flow_gpu  | public.ecr.aws/soci-workshop-examples/tensorflow_gpu:latest | Software library for machine learning and artificial intelligence with nvidia CUDA drivers installed  | Large (> 1GB)      |
| tensor_flow     | public.ecr.aws/soci-workshop-examples/tensorflow:latest  | Software library for machine learning and artificial intelligence   | Medium  (~600MB)    |
| NodeJs  | public.ecr.aws/soci-workshop-examples/node:latest      | Back-end JavaScript runtime environment  | Large (~1GB)     |
| busybox    | public.ecr.aws/soci-workshop-examples/busybox:latest  | Unix utilities suite.   | Small (~2MB)     |
| MongoDb  | public.ecr.aws/soci-workshop-examples/mongo:latest     | MongoDB is a source-available cross-platform document-oriented database program.  | Medium (~700MB)     |
| RabbitMQ     | public.ecr.aws/soci-workshop-examples/rabbitmq:latest  | RabbitMQ is an open-source message-broker software   | Small (~100MB)     |
| Redis  | public.ecr.aws/soci-workshop-examples/redis:latest     | Redis is an in-memory data structure store, used as a distributed, in-memory key–value database, cache and message broker, with optional durability.  | Small (~50MB)     |

### Benchmark results
The benchmark tests generate results of various metrics, the results also provide statistics like mean, standard deviation, p25,p50, p75 and p90 (90th percentile) alongside the min and max value calculated. All measured times are in seconds.

Results are available in the ```/benchmark/<test_type>/output ```folder in the following format
```shell
{
 "commit": "commit_hash",
 "benchmarkTests": [
  {
   "testName": "Image-name",
   "numberOfTests": 1,
   "fullRunStats": {
    "BenchmarkTimes": [
     39.098384883
    ],
    "stdDev": 0,
    "mean": 39.098384883,
    "min": 39.098384883,
    "pct25": 39.098384883,
    "pct50": 39.098384883,
    "pct75": 39.098384883,
    "pct90": 39.098384883,
    "max": 39.098384883
   },
   "pullStats": {
    "BenchmarkTimes": [
     38.801
    ],
    "stdDev": 0,
    "mean": 38.801,
    "min": 38.801,
    "pct25": 38.801,
    "pct50": 38.801,
    "pct75": 38.801,
    "pct90": 38.801,
    "max": 38.801
   },
   "lazyTaskStats": {
    "BenchmarkTimes": [
     0.009
    ],
    "stdDev": 0,
    "mean": 0.009,
    "min": 0.009,
    "pct25": 0.009,
    "pct50": 0.009,
    "pct75": 0.009,
    "pct90": 0.009,
    "max": 0.009
   },
   "localTaskStats": {
    "BenchmarkTimes": [
     0.009
    ],
    "stdDev": 0,
    "mean": 0.009,
    "min": 0.009,
    "pct25": 0.009,
    "pct50": 0.009,
    "pct75": 0.009,
    "pct90": 0.009,
    "max": 0.009
   }
  }
 ]
}
```
