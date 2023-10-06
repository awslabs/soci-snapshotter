window.BENCHMARK_DATA = {
  "lastUpdate": 1696345646357,
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
        "date": 1692224210408,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-node-lazyTaskDuration",
            "value": 0.4575,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-node-localTaskDuration",
            "value": 0.4325,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-node-pullTaskDuration",
            "value": 29.456,
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
        "date": 1692400664253,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-node-lazyTaskDuration",
            "value": 0.4465,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-node-localTaskDuration",
            "value": 0.4125,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-node-pullTaskDuration",
            "value": 42.1225,
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
        "date": 1692726129741,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-node-lazyTaskDuration",
            "value": 0.5005,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-node-localTaskDuration",
            "value": 0.428,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-node-pullTaskDuration",
            "value": 14.485,
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
        "date": 1692734801466,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-node-lazyTaskDuration",
            "value": 0.583,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-node-localTaskDuration",
            "value": 0.5285,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-node-pullTaskDuration",
            "value": 11.931999999999999,
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
        "date": 1693251846537,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-node-lazyTaskDuration",
            "value": 0.5660000000000001,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-node-localTaskDuration",
            "value": 0.5595,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-node-pullTaskDuration",
            "value": 48.379999999999995,
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
        "date": 1693431937338,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-node-lazyTaskDuration",
            "value": 0.4555,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-node-localTaskDuration",
            "value": 0.409,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-node-pullTaskDuration",
            "value": 9.3865,
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
        "date": 1694022252906,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-node-lazyTaskDuration",
            "value": 0.4985,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-node-localTaskDuration",
            "value": 0.4285,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-node-pullTaskDuration",
            "value": 9.563500000000001,
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
        "date": 1694042009888,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-node-lazyTaskDuration",
            "value": 0.449,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-node-localTaskDuration",
            "value": 0.40349999999999997,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-node-pullTaskDuration",
            "value": 38.9975,
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
        "date": 1694053633898,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-node-lazyTaskDuration",
            "value": 0.5395000000000001,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-node-localTaskDuration",
            "value": 0.4115,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-node-pullTaskDuration",
            "value": 9.9,
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
        "date": 1694125752718,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-node-lazyTaskDuration",
            "value": 0.454,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-node-localTaskDuration",
            "value": 0.399,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-node-pullTaskDuration",
            "value": 52.8975,
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
        "date": 1694131842896,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-node-lazyTaskDuration",
            "value": 0.5525,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-node-localTaskDuration",
            "value": 0.5800000000000001,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-node-pullTaskDuration",
            "value": 54.16,
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
        "date": 1695065088745,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-node-lazyTaskDuration",
            "value": 0.532,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-node-localTaskDuration",
            "value": 0.5045,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-node-pullTaskDuration",
            "value": 56.4965,
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
        "date": 1695318880345,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-node-lazyTaskDuration",
            "value": 0.521,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-node-localTaskDuration",
            "value": 0.406,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-node-pullTaskDuration",
            "value": 9.179499999999999,
            "unit": "Seconds",
            "extra": "P90"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "xiainx@gmail.com",
            "name": "Iain Macdonald",
            "username": "iain-macdonald"
          },
          "committer": {
            "email": "66654647+turan18@users.noreply.github.com",
            "name": "Yasin Turan",
            "username": "turan18"
          },
          "distinct": true,
          "id": "af3111d99f24e2653312682495e52dd91b2fa688",
          "message": "Protect access to node.ents and node.entsCached with a mutex in fs/layer/node.go\n\nSigned-off-by: Iain Macdonald <xiainx@gmail.com>",
          "timestamp": "2023-09-27T19:19:07-04:00",
          "tree_id": "5387197f5deb6917dde49a3c1fc182316b6c9c3a",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/af3111d99f24e2653312682495e52dd91b2fa688"
        },
        "date": 1695858001290,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-node-lazyTaskDuration",
            "value": 0.609,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-node-localTaskDuration",
            "value": 0.5435000000000001,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-node-pullTaskDuration",
            "value": 58.789,
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
          "id": "52bd24d685711b33c3893a3a3b6540f97153c8e4",
          "message": "Fix integrations tests with cgroupv2\n\nWith cgroupv2, dind doesn't work out of the box because the inner docker\nprocess is in the (containerized) root cgroup so it can't create the\ninner container's cgroups because doing so would make the inner docker a\nprocess on an interior cgroup node. cgroupv2 only allows processes on\nthe leaf nodes.\n\nThe solution is to move docker to a child cgroup (called init) so that\nthe container can exist as a sibling.\n\nSigned-off-by: Kern Walster <walster@amazon.com>",
          "timestamp": "2023-10-02T13:02:00-07:00",
          "tree_id": "e4e5cc9c570a3d592ace5c45948ce84f831df589",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/52bd24d685711b33c3893a3a3b6540f97153c8e4"
        },
        "date": 1696278364888,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-node-lazyTaskDuration",
            "value": 0.5955,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-node-localTaskDuration",
            "value": 0.5135000000000001,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-node-pullTaskDuration",
            "value": 26.238,
            "unit": "Seconds",
            "extra": "P90"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "davbson@amazon.com",
            "name": "David Son",
            "username": "sondavidb"
          },
          "committer": {
            "email": "55555210+sondavidb@users.noreply.github.com",
            "name": "David Son",
            "username": "sondavidb"
          },
          "distinct": true,
          "id": "3dff0ad97b658b008fe9f0b98605b2a60f4a5839",
          "message": "Add extra comments on deprecated functions\n\nSigned-off-by: David Son <davbson@amazon.com>",
          "timestamp": "2023-10-02T15:15:32-07:00",
          "tree_id": "d754687fc7ca5d44b2fd2a9b5003812f7023cb6c",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/3dff0ad97b658b008fe9f0b98605b2a60f4a5839"
        },
        "date": 1696286135484,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-node-lazyTaskDuration",
            "value": 0.429,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-node-localTaskDuration",
            "value": 0.4045,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-node-pullTaskDuration",
            "value": 18.4525,
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
            "email": "66654647+turan18@users.noreply.github.com",
            "name": "Yasin Turan",
            "username": "turan18"
          },
          "distinct": true,
          "id": "342eff60b4eee144fe7b92a125a79a301e53725a",
          "message": "Upgrade go-fuse to address LSEEK bug\n\nUpgrade go-fuse to commit fc2c4d3, as it contains the\nfix for the LSEEK bug that caused cp/mv/install on sparse\nfiles to hang.\n\nSigned-off-by: Yasin Turan <turyasin@amazon.com>",
          "timestamp": "2023-10-03T10:45:56-04:00",
          "tree_id": "b5d2b3bbc1496359609d8f6a1eb5a0a3de2ad572",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/342eff60b4eee144fe7b92a125a79a301e53725a"
        },
        "date": 1696345618572,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-node-lazyTaskDuration",
            "value": 0.4425,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-node-localTaskDuration",
            "value": 0.399,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-node-pullTaskDuration",
            "value": 45.6285,
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
            "email": "66654647+turan18@users.noreply.github.com",
            "name": "Yasin Turan",
            "username": "turan18"
          },
          "distinct": true,
          "id": "f72ea9ceea656c4b1e0c1c9745550cf732c26cb2",
          "message": "Add support for CRI v1 API\n\nSigned-off-by: Yasin Turan <turyasin@amazon.com>",
          "timestamp": "2023-10-03T10:46:20-04:00",
          "tree_id": "65f15d1f31e3edf0aee01a339adb75ea246601ad",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/f72ea9ceea656c4b1e0c1c9745550cf732c26cb2"
        },
        "date": 1696345641835,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-node-lazyTaskDuration",
            "value": 0.544,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-node-localTaskDuration",
            "value": 0.5105,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-node-pullTaskDuration",
            "value": 11.8925,
            "unit": "Seconds",
            "extra": "P90"
          }
        ]
      }
    ]
  }
}