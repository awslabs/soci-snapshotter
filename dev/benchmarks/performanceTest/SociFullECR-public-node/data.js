window.BENCHMARK_DATA = {
  "lastUpdate": 1692038788584,
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
        "date": 1692038786558,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-node-lazyTaskDuration",
            "value": 0.5555,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-node-localTaskDuration",
            "value": 0.486,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-node-pullTaskDuration",
            "value": 10.3095,
            "unit": "Seconds",
            "extra": "P90"
          }
        ]
      },
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
        "date": 1692038786558,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-node-lazyTaskDuration",
            "value": 0.5555,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-node-localTaskDuration",
            "value": 0.486,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-node-pullTaskDuration",
            "value": 10.3095,
            "unit": "Seconds",
            "extra": "P90"
          }
        ]
      }
    ]
  }
}