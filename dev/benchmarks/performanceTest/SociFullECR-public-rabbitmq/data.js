window.BENCHMARK_DATA = {
  "lastUpdate": 1692400671730,
  "repoUrl": "https://github.com/awslabs/soci-snapshotter",
  "entries": {
    "Soci Benchmark": [
      {
        "commit": {
          "author": {
            "email": "arjunry@amazon.com",
            "name": "Arjun Yogidas"
          },
          "committer": {
            "email": "66654647+turan18@users.noreply.github.com",
            "name": "Yasin Turan",
            "username": "turan18"
          },
          "distinct": true,
          "id": "184d1715fe4985936018f8013dd81c54019ae4e4",
          "message": "Add benchmark visualization workflow\n\nThis commit changes the benchmark.yml workflow into\nbenchmark_visualization.yml. The new workflow runs on every code merge,\nit will run the benchmark target and upload the result as\nbenchmark-result-artifact. The results are then converted to the\nappropriate format for visualization using the\nvisualization_data_converter.sh shell script. It then uploads the newly\ngenerated data files to Github-pages using the github-action-benchmark\naction.\n\nSigned-off-by: Arjun Raja Yogidas <arjunry@amazon.com>",
          "timestamp": "2023-08-14T14:24:40-04:00",
          "tree_id": "4d49c9a79b3c29a9a58706a04dc54cbdfcd909e7",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/184d1715fe4985936018f8013dd81c54019ae4e4"
        },
        "date": 1692038786576,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 7.367,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.38,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 8.5195,
            "unit": "Seconds",
            "extra": "P90"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "walster@amazon.com",
            "name": "Kern Walster",
            "username": "Kern--"
          },
          "committer": {
            "email": "66654647+turan18@users.noreply.github.com",
            "name": "Yasin Turan",
            "username": "turan18"
          },
          "distinct": true,
          "id": "8c6880c62279317ca0e66629f70f70ce99babcc5",
          "message": "Retain original cache.Get error in span manager\n\nBefore this change, the span manager would replace the error received\nfrom `m.cache.Get` with a generic `ErrSpanNotAvailable`. The way we use\nthe cache is really just as an abstraction of disk storage for span\ndata so we don't generally expect `m.cache.Get` to throw an error. If it\ndoes, we should keep that context.\n\nSigned-off-by: Kern Walster <walster@amazon.com>",
          "timestamp": "2023-08-16T18:01:03-04:00",
          "tree_id": "83d33914f5d563e4c644cfc16d06871653d1c13c",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/8c6880c62279317ca0e66629f70f70ce99babcc5"
        },
        "date": 1692224210772,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 5.4864999999999995,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 5.609,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 3.528,
            "unit": "Seconds",
            "extra": "P90"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "turyasin@amazon.com",
            "name": "Yasin Turan",
            "username": "turan18"
          },
          "committer": {
            "email": "kern.walster@gmail.com",
            "name": "Kern Walster",
            "username": "Kern--"
          },
          "distinct": true,
          "id": "d290e21d0a705c7cc530221a16dbc959eb3f2941",
          "message": "Deterministically close open span cache file descriptors\n\nThe snapshotter stores fetched spans in a cache either in memory\nor on disk. When reading from the cache on disk we use a Finalizer\nconstruct to close the open file descriptors when the Go garbage\ncollector sees that the fd is no longer being referenced. The issue with\nthis is that we don't have control over when the GC runs (although it's\npossible), and so the process could hold on too open fds for a unknown\namount of time causing a sort of leak. On systems where the snapshotter is\nbounded by a ulimit in the number of open files, this can cause the\nsnapshotter span cache get calls to fail, causing `file.Read` failures for the\nrunning container/process. This change wraps the readers returned by the\ncache in `io.ReadCloser`'s, so we can deterministically close the files\nonce the content has been read from them.\n\nSigned-off-by: Yasin Turan <turyasin@amazon.com>",
          "timestamp": "2023-08-18T16:01:10-07:00",
          "tree_id": "ce88584a4d3302fab09faff35dc07a43f4b0110d",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/d290e21d0a705c7cc530221a16dbc959eb3f2941"
        },
        "date": 1692400670189,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 5.9115,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 5.702,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 6.024,
            "unit": "Seconds",
            "extra": "P90"
          }
        ]
      }
    ]
  }
}