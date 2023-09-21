window.BENCHMARK_DATA = {
  "lastUpdate": 1695318882897,
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
        "date": 1692038787541,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-tensorflow_gpu-lazyTaskDuration",
            "value": 2.8515,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-tensorflow_gpu-localTaskDuration",
            "value": 2.4050000000000002,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-tensorflow_gpu-pullTaskDuration",
            "value": 167.96699999999998,
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
        "date": 1692038787541,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-tensorflow_gpu-lazyTaskDuration",
            "value": 2.8515,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-tensorflow_gpu-localTaskDuration",
            "value": 2.4050000000000002,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-tensorflow_gpu-pullTaskDuration",
            "value": 167.96699999999998,
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
        "date": 1692224211229,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-tensorflow_gpu-lazyTaskDuration",
            "value": 2.216,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-tensorflow_gpu-localTaskDuration",
            "value": 1.918,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-tensorflow_gpu-pullTaskDuration",
            "value": 76.06,
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
        "date": 1692400667024,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-tensorflow_gpu-lazyTaskDuration",
            "value": 2.5595,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-tensorflow_gpu-localTaskDuration",
            "value": 2.053,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-tensorflow_gpu-pullTaskDuration",
            "value": 75.50399999999999,
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
          "id": "670edd50e7640c86af4e64120ac9b68da9914ffd",
          "message": "Update check_regreesion.sh file\n\nThis commit updates the regression check script to skip the initial\nvalue in all BenchmarkTimes array of the benchmark results json file to\ncalculate a new p90. We use this new p90 to identify regression, this\nchange was made to combat the skewed p90 metrics we were seeing due to\nthe slow starts of the benchmark pull times which were affecting the\noverall regression comparison. Skipping the first value allows us to\nhave a more uniform comparison, remove github environment noise and we'd\nbe able to identify true regression in our code.\n\nSigned-off-by: Arjun Raja Yogidas <arjunry@amazon.com>",
          "timestamp": "2023-08-22T13:27:16-04:00",
          "tree_id": "7e0bf34018e2cf82ac9a943db20cbeb7c4d2ece5",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/670edd50e7640c86af4e64120ac9b68da9914ffd"
        },
        "date": 1692726131329,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-tensorflow_gpu-lazyTaskDuration",
            "value": 2.4825,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-tensorflow_gpu-localTaskDuration",
            "value": 1.9645000000000001,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-tensorflow_gpu-pullTaskDuration",
            "value": 75.44749999999999,
            "unit": "Seconds",
            "extra": "P90"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "manujgrover71@gmail.com",
            "name": "Manuj Grover",
            "username": "manujgrover71"
          },
          "committer": {
            "email": "66654647+turan18@users.noreply.github.com",
            "name": "Yasin Turan",
            "username": "turan18"
          },
          "distinct": true,
          "id": "5a40aff504535a863e0655de76a77b058184cafc",
          "message": "Using standard error package instead of go-multierror for multierror error.\n\nSigned-off-by: Manuj Grover <manujgrover71@gmail.com>",
          "timestamp": "2023-08-22T15:49:08-04:00",
          "tree_id": "b3c836a33b9dbca10ef7d1afb88e4e343a05e9b3",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/5a40aff504535a863e0655de76a77b058184cafc"
        },
        "date": 1692734803224,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-tensorflow_gpu-lazyTaskDuration",
            "value": 3.978,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-tensorflow_gpu-localTaskDuration",
            "value": 2.542,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-tensorflow_gpu-pullTaskDuration",
            "value": 91.2995,
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
            "email": "55555210+sondavidb@users.noreply.github.com",
            "name": "David Son",
            "username": "sondavidb"
          },
          "distinct": true,
          "id": "ec1e62326578318ace5048c487b7e155a068dd4b",
          "message": "Remove remote snapshot key from local mount logs\n\nIn an attempt to make it more clear when we aren't lazily loading\nlayers, we emit logs with a \"remote-snapshot-prepared\":\"false\" context.\nThis often happens when a layer doesn't have a ztoc.\n\nWhen we skipped lazy loading, we emitted a log saying that we skipped,\nbut we also emitted a confusing (and incorrect) message like:\n\n```\n{\"msg\":\"local snapshot successfully prepared\",\"remote-snapshot-prepared\":\"true\"}\n```\n\nThis could say \"remote-snapshot-prepared\":\"false\", but this change is\nmore clear because there is exactly 1 log line that contains the key per\nimage layer. By inspecting the logs, you can count how many layers\nweren't lazily loaded by counting the number of log lines with\n\"remote-snapshot-prepared\":\"false\".\n\nSigned-off-by: Kern Walster <walster@amazon.com>",
          "timestamp": "2023-08-28T12:15:51-07:00",
          "tree_id": "4e6fafff7a0e2e95bdf435130130c01496d85d8a",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/ec1e62326578318ace5048c487b7e155a068dd4b"
        },
        "date": 1693251850857,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-tensorflow_gpu-lazyTaskDuration",
            "value": 3.3810000000000002,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-tensorflow_gpu-localTaskDuration",
            "value": 3.105,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-tensorflow_gpu-pullTaskDuration",
            "value": 265.3935,
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
          "id": "7cc04aff01bce06f3b1759cf1eecf4cebdeeb2cb",
          "message": "Fix integration test failures on non-x86\n\nThere were several issues that caused integration tests to fail\non non-x86. The first one was our dependency GHCR registry image which\nis only supported on amd64. The GHCR registry was needed when we supported artifact\nmanifests, but those have been seemingly removed from the OCI 1.1 spec proposal,\nso we no longer need it. We also had 2 tests that relied on a pinned amd64\nvariant of rabbitmq. Those have been replaced with a pinned multi-arch index.\n\nSigned-off-by: Yasin Turan <turyasin@amazon.com>",
          "timestamp": "2023-08-30T16:26:31-05:00",
          "tree_id": "455f1564db6ccfe97230840e05d49071744b1f72",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/7cc04aff01bce06f3b1759cf1eecf4cebdeeb2cb"
        },
        "date": 1693431941929,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-tensorflow_gpu-lazyTaskDuration",
            "value": 3.683,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-tensorflow_gpu-localTaskDuration",
            "value": 2.2824999999999998,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-tensorflow_gpu-pullTaskDuration",
            "value": 130.6205,
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
            "email": "kern.walster@gmail.com",
            "name": "Kern Walster",
            "username": "Kern--"
          },
          "distinct": true,
          "id": "a1ec4dab790da07e9e29f94293a1f7338b38068d",
          "message": "Allow manually tirggering bump-deps on forks\n\nBy default, bump-deps doesn't run on forks since forks will generally\nget updates by rebasing on awslabs/soci-snapshotter. When making\nchanges to dump-deps.sh, however, we generally want to test on a fork\nbefore merging the change upstream. Before this change, users had to\ntemporarily enable the cron workflow on their fork to manually test,\nthen re-disable it before submitting a PR. With this change, you can\nmanually run bump-deps on a fork without any changes.\n\nSigned-off-by: Kern Walster <walster@amazon.com>",
          "timestamp": "2023-09-06T12:27:11-05:00",
          "tree_id": "4eb7da66d75b52564f688942e5d8dc2134fa8abe",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/a1ec4dab790da07e9e29f94293a1f7338b38068d"
        },
        "date": 1694022252699,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-tensorflow_gpu-lazyTaskDuration",
            "value": 3.2145,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-tensorflow_gpu-localTaskDuration",
            "value": 2.144,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-tensorflow_gpu-pullTaskDuration",
            "value": 96.932,
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
            "email": "kern.walster@gmail.com",
            "name": "Kern Walster",
            "username": "Kern--"
          },
          "distinct": true,
          "id": "53fb9d9960c929faf0b69260e79567e5c5323f02",
          "message": "Add TOC entry validation on first read\n\nThis change validates that when a file is read, the TOC entry is checked\nto be sure that it matches the TAR header in the image layer.\n\nSigned-off-by: Kern Walster <walster@amazon.com>",
          "timestamp": "2023-09-06T17:49:28-05:00",
          "tree_id": "ed569c30379a2e50121f3e34afe125c1852afc2e",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/53fb9d9960c929faf0b69260e79567e5c5323f02"
        },
        "date": 1694042013982,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-tensorflow_gpu-lazyTaskDuration",
            "value": 3.2085,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-tensorflow_gpu-localTaskDuration",
            "value": 2.0505,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-tensorflow_gpu-pullTaskDuration",
            "value": 216.9985,
            "unit": "Seconds",
            "extra": "P90"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "49699333+dependabot[bot]@users.noreply.github.com",
            "name": "dependabot[bot]",
            "username": "dependabot[bot]"
          },
          "committer": {
            "email": "66654647+turan18@users.noreply.github.com",
            "name": "Yasin Turan",
            "username": "turan18"
          },
          "distinct": true,
          "id": "e464dda76e10cbd842ffadfbe9c0eb0bed9e9548",
          "message": "Bump actions/checkout from 3 to 4\n\nBumps [actions/checkout](https://github.com/actions/checkout) from 3 to 4.\n- [Release notes](https://github.com/actions/checkout/releases)\n- [Changelog](https://github.com/actions/checkout/blob/main/CHANGELOG.md)\n- [Commits](https://github.com/actions/checkout/compare/v3...v4)\n\n---\nupdated-dependencies:\n- dependency-name: actions/checkout\n  dependency-type: direct:production\n  update-type: version-update:semver-major\n...\n\nSigned-off-by: dependabot[bot] <support@github.com>",
          "timestamp": "2023-09-06T22:10:21-04:00",
          "tree_id": "438e921396cef07dbb949a6b9406e949f3c93ca2",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/e464dda76e10cbd842ffadfbe9c0eb0bed9e9548"
        },
        "date": 1694053630364,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-tensorflow_gpu-lazyTaskDuration",
            "value": 3.4735,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-tensorflow_gpu-localTaskDuration",
            "value": 2.221,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-tensorflow_gpu-pullTaskDuration",
            "value": 95.4175,
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
            "email": "kern.walster@gmail.com",
            "name": "Kern Walster",
            "username": "Kern--"
          },
          "distinct": true,
          "id": "fe6994c8f582f2983b11898a41c98e3328f4bbb8",
          "message": "Skip grpc in bump-deps.sh\n\nThe latest version of grpc is not compatible with the version containerd\nis using. Skip automatic updates.\n\nThis change also stops using the `-u` flag in `go get`. The flag causes\n`go get` to update transitive depdendencies which means that grpc was\ngetting update whenever we tried to update containerd.\n\nSigned-off-by: Kern Walster <walster@amazon.com>",
          "timestamp": "2023-09-07T15:06:01-07:00",
          "tree_id": "9b6ac4ac3a67490f0c31dcc673a4475db8690fe3",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/fe6994c8f582f2983b11898a41c98e3328f4bbb8"
        },
        "date": 1694125753080,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-tensorflow_gpu-lazyTaskDuration",
            "value": 2.216,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-tensorflow_gpu-localTaskDuration",
            "value": 1.9675,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-tensorflow_gpu-pullTaskDuration",
            "value": 181.7045,
            "unit": "Seconds",
            "extra": "P90"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "no-reply@github.com",
            "name": "GitHub",
            "username": "invalid-email-address"
          },
          "committer": {
            "email": "kern.walster@gmail.com",
            "name": "Kern Walster",
            "username": "Kern--"
          },
          "distinct": true,
          "id": "1e0057135d22a2bd91567200be8a73ead99bb63e",
          "message": "Bump dependencies using scripts/bump-deps.sh\n\nSigned-off-by: GitHub <noreply@github.com>",
          "timestamp": "2023-09-07T16:49:08-07:00",
          "tree_id": "1116478adde69f9e2fb42f43ce5bc7f04587b49b",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/1e0057135d22a2bd91567200be8a73ead99bb63e"
        },
        "date": 1694131831413,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-tensorflow_gpu-lazyTaskDuration",
            "value": 4.5845,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-tensorflow_gpu-localTaskDuration",
            "value": 2.7874999999999996,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-tensorflow_gpu-pullTaskDuration",
            "value": 86.1515,
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
            "email": "kern.walster@gmail.com",
            "name": "Kern Walster",
            "username": "Kern--"
          },
          "distinct": true,
          "id": "b06aad0ad53dcd7a0c08e92b56a6237bad969e8f",
          "message": "Set fuse.Attr.Blocks to # of 512-byte blocks\n\nBefore this change, SOCI set fuse.Attr.Blocks to the number of\nblockSize-byte blocks instead of the expected number of 512-byte blocks.\nThis caused the files to appear sparse and uncovered a bug in go-fuse.\nOnce the go-fuse bug is fixed, there shouldn't be any functional\ndifference, but it causes unnecessary lseeks which we can eliminate.\n\nSigned-off-by: Kern Walster <walster@amazon.com>",
          "timestamp": "2023-09-18T11:57:23-07:00",
          "tree_id": "ec128ba9f372c53384d67b818f2ad248ddec6512",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/b06aad0ad53dcd7a0c08e92b56a6237bad969e8f"
        },
        "date": 1695065086631,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-tensorflow_gpu-lazyTaskDuration",
            "value": 4.1265,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-tensorflow_gpu-localTaskDuration",
            "value": 2.8129999999999997,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-tensorflow_gpu-pullTaskDuration",
            "value": 249.3415,
            "unit": "Seconds",
            "extra": "P90"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "no-reply@github.com",
            "name": "GitHub",
            "username": "invalid-email-address"
          },
          "committer": {
            "email": "66654647+turan18@users.noreply.github.com",
            "name": "Yasin Turan",
            "username": "turan18"
          },
          "distinct": true,
          "id": "eebd84e352a64731a92412447c6dbfcd198328cd",
          "message": "Bump dependencies using scripts/bump-deps.sh\n\nSigned-off-by: GitHub <noreply@github.com>",
          "timestamp": "2023-09-21T13:38:39-04:00",
          "tree_id": "2bab2f344cc75b1bc12b09777499a4ef32944930",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/eebd84e352a64731a92412447c6dbfcd198328cd"
        },
        "date": 1695318881115,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-tensorflow_gpu-lazyTaskDuration",
            "value": 2.298,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-tensorflow_gpu-localTaskDuration",
            "value": 1.9905,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-tensorflow_gpu-pullTaskDuration",
            "value": 76.5535,
            "unit": "Seconds",
            "extra": "P90"
          }
        ]
      }
    ]
  }
}