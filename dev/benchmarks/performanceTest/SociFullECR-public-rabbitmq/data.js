window.BENCHMARK_DATA = {
  "lastUpdate": 1755104476540,
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
        "date": 1692726132666,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 5.852,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 5.576,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 3.766,
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
        "date": 1692734804454,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 7.945499999999999,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.8095,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 5.613,
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
        "date": 1693251846467,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 7.9855,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.951499999999999,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 5.964,
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
        "date": 1693431938085,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 6.101,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 6.1605,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 4.061,
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
        "date": 1694022251529,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 5.91,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 6.001,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 6.154,
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
        "date": 1694042012513,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 5.5885,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 5.6615,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 4.7355,
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
        "date": 1694053630382,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 5.8545,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 5.913,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 4.37,
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
        "date": 1694125748381,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 5.5145,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 5.4655000000000005,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 10.199,
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
        "date": 1694131835625,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 8.807,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 8.692,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 4.6765,
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
        "date": 1695065081210,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 8.1205,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.744,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 9.41,
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
        "date": 1695318883948,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 5.6025,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 5.737500000000001,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 3.387,
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
        "date": 1695857999254,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 8.5765,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 8.526,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 5.2844999999999995,
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
        "date": 1696278365344,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 8.035499999999999,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 8.041,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 7.6855,
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
        "date": 1696286139708,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 6.159,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 6.297000000000001,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 6.632,
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
        "date": 1696345619105,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 6.0215,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 5.923,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 5.830500000000001,
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
        "date": 1696345642224,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 7.4719999999999995,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.258,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 9.109,
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
          "id": "b5c2e7a05e6df6963f58da810d182245d60931ba",
          "message": "Support xattrs\n\nBefore this change, SOCI stored all PAX header records as linux xattrs.\nPAX header records are a generic key-value pair for TAR files, not\nspecifically linux xattrs. While go does support linux xattrs by\nprefixing them with SCHILY.xattr, since we didn't parse them back to\nlinux xattrs, they did not behave correctly with SOCI. The most likely\nway users would experience this is that file capabilities don't work\nwith SOCI.\n\nThis change keeps all PAX header records in the ztoc format, but parses\nout just the linux xattrs without the prefix when creating the\nfilesystem metadata from a ztoc.\n\nDocker, buildkit, buildah/podman, and kaniko all use the go\ntarHeader.Xattrs to add xattrs which uses the `SCHILY.xattr.` prefix.\nWhile there are technically other ways to encode xattrs (e.g.\n`LIBARCHIVE.xattr.`) it doesn't seem common.\n\nSigned-off-by: Kern Walster <walster@amazon.com>",
          "timestamp": "2023-10-06T09:51:51-07:00",
          "tree_id": "ef8d0d55dc9b97763e485f351de050886fd5ea52",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/b5c2e7a05e6df6963f58da810d182245d60931ba"
        },
        "date": 1696612162030,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 5.8465,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 5.5794999999999995,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 7.1255,
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
          "id": "6fb58b582f4d01c9399387ef606ee23e36fc9e5b",
          "message": "Bump dependencies using scripts/bump-deps.sh\n\nSigned-off-by: GitHub <noreply@github.com>",
          "timestamp": "2023-10-06T09:52:12-07:00",
          "tree_id": "13b31f4d6018116cf13a4da02b84b52a3466a3fa",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/6fb58b582f4d01c9399387ef606ee23e36fc9e5b"
        },
        "date": 1696612332273,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 5.4625,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 5.473,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 5.739,
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
          "id": "104c68c2c6e9c3e10770413862d17f237c9a2600",
          "message": "Add go-fuse logger\n\nsoci-snapshotter-gprc overrides the default logger to be a logrus logger\nwith debug as the write level. If the snapshotter is invoked with\n`--log-level debug`, then logs from e.g. `log.PrinLn` are visibile.\n\ngo-fuse uses this logging method to output a complete transaction of\nfuse operations keeping track of each request/response with a unique id.\nThe ID is only unique within a single server, so trying to parse the\nlogs is ambiguous since you don't know which server a response\ncorresponds to.\n\ngo-fuse recently added support for per-server loggers. This change uses\nthat functionality to add the layer digest to the logs from each fuse\nserver so request/response pairs can be unambiguously matched.\n\nIt also changes the log level of go-fuse logs to trace since there\nreally is no debug reason for enabling them.\n\nSigned-off-by: Kern Walster <walster@amazon.com>",
          "timestamp": "2023-10-16T10:52:17-04:00",
          "tree_id": "e087b5bfa3e6a2ae50fe2ce44e6f5df2a4de7a04",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/104c68c2c6e9c3e10770413862d17f237c9a2600"
        },
        "date": 1697469279496,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 8.1065,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.878,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 7.9305,
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
          "id": "31a8bcc683f7522d146d0505d034558909e53433",
          "message": "Upgrade to go 1.21 to fix `go get` panic when upgrading dependencies.\n\nSigned-off-by: Yasin Turan <turyasin@amazon.com>",
          "timestamp": "2023-10-17T19:58:21-07:00",
          "tree_id": "c051abe9866d955621a3d549b524851ce6a47cf5",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/31a8bcc683f7522d146d0505d034558909e53433"
        },
        "date": 1697599040304,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 5.487,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 5.5125,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 5.6080000000000005,
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
          "id": "68b8fee8acf27bb58b43a13ceefaf666613b980a",
          "message": "Move to github.com/containerd/log\n\nContainerd moved their log package from in tree\ngithub.com/containerd/containerd/log to it's own module at\ngithub.com/containerd/log. The in-tree log is deprecated as of 1.7.7\nwhich is tripping up the linter when we do the containerd update.\n\nThis change moves to the new log package.\n\nSigned-off-by: Kern Walster <walster@amazon.com>",
          "timestamp": "2023-10-23T11:02:24-04:00",
          "tree_id": "c6be5553ef0715f5c024d418c9b892c06595b599",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/68b8fee8acf27bb58b43a13ceefaf666613b980a"
        },
        "date": 1698074710801,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 9.609,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 9.554,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 4.0975,
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
          "id": "114d5ddb8b8db27e5bd588afd30044fa65ed5e00",
          "message": "Add example config TOML\n\nSigned-off-by: David Son <davbson@amazon.com>",
          "timestamp": "2023-10-23T15:46:15-07:00",
          "tree_id": "c550b30a782e928c0abdc7830d6ffeba1aa2fde7",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/114d5ddb8b8db27e5bd588afd30044fa65ed5e00"
        },
        "date": 1698102728778,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 9.8185,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 9.356,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 9.0915,
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
          "id": "d85cdbe880f320f8f56de11d3fa00811b5dde0d3",
          "message": "Bump dependencies using scripts/bump-deps.sh\n\nSigned-off-by: GitHub <noreply@github.com>",
          "timestamp": "2023-10-24T17:22:16-07:00",
          "tree_id": "4d40fa282cc1b9c33c5e321e0bcdc1f4bc6ee12c",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/d85cdbe880f320f8f56de11d3fa00811b5dde0d3"
        },
        "date": 1698194342658,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 6.1665,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 6.52,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 5.52,
            "unit": "Seconds",
            "extra": "P90"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "macedonv@amazon.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "committer": {
            "email": "55906459+austinvazquez@users.noreply.github.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "distinct": true,
          "id": "83aa1d02e0b22e5ff7e0f90ff071080d02ca6a03",
          "message": "Update scripts with minor changes\n\nThis changeset adds consistent naming to shell scripts, adds copyrights\nto benchmark scripts, and changes shebangs to env bash to work in\nenvironments when bash is not under `/bin/`.\n\nSigned-off-by: Austin Vazquez <macedonv@amazon.com>",
          "timestamp": "2023-10-26T08:21:36-07:00",
          "tree_id": "ebc50b2a859f3b208acc286de68f54345b856878",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/83aa1d02e0b22e5ff7e0f90ff071080d02ca6a03"
        },
        "date": 1698334851965,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 5.5575,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 5.9075,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 3.6685,
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
          "id": "fe3fc9339a3fa5a3f602cfcbc21842bb8d3b9028",
          "message": "Add global FUSE failure metric\n\nAdded a global FUSE failure metric that is only ever incremented\nevery time block (5 mins).\n\nSigned-off-by: Yasin Turan <turyasin@amazon.com>",
          "timestamp": "2023-10-26T15:04:42-04:00",
          "tree_id": "da786762df60afd87214a13ca0944c39e4ffb79f",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/fe3fc9339a3fa5a3f602cfcbc21842bb8d3b9028"
        },
        "date": 1698348437334,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 8.216000000000001,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.7335,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 8.667,
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
          "id": "49dc6a77a41175bca0dc74d43389c61edc171803",
          "message": "Add test against referrers API\n\nSigned-off-by: David Son <davbson@amazon.com>",
          "timestamp": "2023-10-27T10:31:47-07:00",
          "tree_id": "e9e58433bb8cd7b8480baf068fcd6a1d950a3d66",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/49dc6a77a41175bca0dc74d43389c61edc171803"
        },
        "date": 1698428814981,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 5.9405,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 6.0504999999999995,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 4.9815,
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
          "id": "5af486cba306bf0b04da5e8b66072a13d259063a",
          "message": "Keep directories when SIGINT sent to daemon\n\nSigned-off-by: David Son <davbson@amazon.com>",
          "timestamp": "2023-10-30T12:37:17-07:00",
          "tree_id": "f9e335adb72eb5b0c9481810a79bf8f486f4f491",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/5af486cba306bf0b04da5e8b66072a13d259063a"
        },
        "date": 1698695680409,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 5.644,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 5.631500000000001,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 6.1245,
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
          "id": "c41c572ff684055a1fa8b954921d12da30fbcdba",
          "message": "Improve benchmark make rules\n\nThis change improves benchmark make rules to reduce duplication, add\ndependencies between targets, and properly mark phony targets.\n\nIt also adds a new `BENCHMARK_FLAGS` option so that we can control how\nthe benchmarks run from make, e.g.\n\n```\nBENCHMARK_FLAGS='-count 10' make benchmarks\n```\n\nSigned-off-by: Kern Walster <walster@amazon.com>",
          "timestamp": "2023-10-31T10:21:35-07:00",
          "tree_id": "53226c875bfe716243a080b9a96688fd4625eb5c",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/c41c572ff684055a1fa8b954921d12da30fbcdba"
        },
        "date": 1698773902612,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 6.314500000000001,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 5.9015,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 8.0365,
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
            "email": "55906459+austinvazquez@users.noreply.github.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "distinct": true,
          "id": "a8df3385dac204a450285f4119542d138c6b04fa",
          "message": "Bump dependencies using scripts/bump-deps.sh\n\nSigned-off-by: GitHub <noreply@github.com>",
          "timestamp": "2023-10-31T10:39:02-07:00",
          "tree_id": "0649b80f86f2131bb27d3ce71190587cae804f7b",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/a8df3385dac204a450285f4119542d138c6b04fa"
        },
        "date": 1698774940718,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 5.968999999999999,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 6.0585,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 4.8755,
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
          "id": "7557b6c3bcf91092c52f38e6f02afdbb38e54a32",
          "message": "Remove most references to ctr\n\nRemoved all references except from \"image rpull\" and \"run\".\n\"run\" is entirely delegated to ctr and thus probably not worth porting.\n\"image rpull\" uses more than just flags and might be a tad more\ndifficult to port over.\n\nSigned-off-by: David Son <davbson@amazon.com>",
          "timestamp": "2023-10-31T14:19:54-07:00",
          "tree_id": "0ee27429a49842b9155c2a9279cbcb0dcdd63dce",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/7557b6c3bcf91092c52f38e6f02afdbb38e54a32"
        },
        "date": 1698788113642,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 5.5575,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 5.589499999999999,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 5.16,
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
          "id": "a8db74e2bc2637ffb89d510efba4025840fd5340",
          "message": "Add release automation on tag push\n\nSigned-off-by: David Son <davbson@amazon.com>",
          "timestamp": "2023-11-01T12:01:01-07:00",
          "tree_id": "8655ecc47d36a7ef6cda31d31423147d485e7ea8",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/a8db74e2bc2637ffb89d510efba4025840fd5340"
        },
        "date": 1698865874353,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 4.2235,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 4.294499999999999,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 2.965,
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
            "email": "kern.walster@gmail.com",
            "name": "Kern Walster",
            "username": "Kern--"
          },
          "distinct": true,
          "id": "c270e9aac507a4c71d558bc22fabf91c1c681e17",
          "message": "Bump github.com/docker/docker in /cmd\n\nBumps [github.com/docker/docker](https://github.com/docker/docker) from 23.0.5+incompatible to 24.0.7+incompatible.\n- [Release notes](https://github.com/docker/docker/releases)\n- [Commits](https://github.com/docker/docker/compare/v23.0.5...v24.0.7)\n\n---\nupdated-dependencies:\n- dependency-name: github.com/docker/docker\n  dependency-type: indirect\n...\n\nSigned-off-by: dependabot[bot] <support@github.com>",
          "timestamp": "2023-11-01T12:00:05-07:00",
          "tree_id": "e51ab831f992c16885e27b4da5e825db9c06c4d6",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/c270e9aac507a4c71d558bc22fabf91c1c681e17"
        },
        "date": 1698865978271,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 4.6015,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 4.7515,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 2.6265,
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
            "email": "55555210+sondavidb@users.noreply.github.com",
            "name": "David Son",
            "username": "sondavidb"
          },
          "distinct": true,
          "id": "3d74f165ab4a75840876f4f70d18d404f303e148",
          "message": "Bump github.com/docker/docker\n\nBumps [github.com/docker/docker](https://github.com/docker/docker) from 23.0.5+incompatible to 24.0.7+incompatible.\n- [Release notes](https://github.com/docker/docker/releases)\n- [Commits](https://github.com/docker/docker/compare/v23.0.5...v24.0.7)\n\n---\nupdated-dependencies:\n- dependency-name: github.com/docker/docker\n  dependency-type: indirect\n...\n\nSigned-off-by: dependabot[bot] <support@github.com>",
          "timestamp": "2023-11-01T13:33:02-07:00",
          "tree_id": "3a3405cb57d34adc9f40b71ca59070ec9a5003e5",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/3d74f165ab4a75840876f4f70d18d404f303e148"
        },
        "date": 1698871919118,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 5.4915,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 5.5045,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 6.1514999999999995,
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
          "id": "ca55a07e2811d7fc0319b349a81990b2343d2bc6",
          "message": "Bump github.com/containerd/containerd\n\nThe previous dependabot commit somehow downgraded containerd.\nThis commit should bring it back to v1.7.8.\n\nSigned-off-by: David Son <davbson@amazon.com>",
          "timestamp": "2023-11-02T10:27:22-07:00",
          "tree_id": "dd6550fe1c88b677d6112409f2def6affc495f03",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/ca55a07e2811d7fc0319b349a81990b2343d2bc6"
        },
        "date": 1698947107321,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 6.0655,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 5.7955000000000005,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 3.7,
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
          "id": "e616fa75dd1a01b9dc55232a7300efce055aa7f5",
          "message": "Add algorithm to default benchmarker index digests\n\nBefore this change, running the benchmarker with default images didn't\nwork as expected because the SOCI index digests were missing the\nalgorithm. This adds the algorithms so the default benchmarks work as\nexpected.\n\nSigned-off-by: Kern Walster <walster@amazon.com>",
          "timestamp": "2023-11-02T16:49:39-07:00",
          "tree_id": "f2f168985065a78b7f96dc2a1add31770a201d34",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/e616fa75dd1a01b9dc55232a7300efce055aa7f5"
        },
        "date": 1698969832184,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 24.971,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 15.734,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.2069999999999999,
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
          "id": "bed91dd46a947040b9bee09fefc6d0dec677940d",
          "message": "Log benchmark errors to stdout\n\nWhen manually running benchmarks with `testing.Benchmark`, the\nnon-configurable output writer is set to discard. This means that if the\nbenchmarker fails, the fatal logs are lost. This change wraps the\nbenchmarker fatal with a direct write to stdout.\n\nSigned-off-by: Kern Walster <walster@amazon.com>",
          "timestamp": "2023-11-03T11:09:52-07:00",
          "tree_id": "10a6120080bd4cd762cc579791d808cbcdc3c23a",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/bed91dd46a947040b9bee09fefc6d0dec677940d"
        },
        "date": 1699035514300,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 10.264,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.664,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 0.944,
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
            "email": "55906459+austinvazquez@users.noreply.github.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "distinct": true,
          "id": "c68b5c4172259d450f02f8e050ecad53b4a6214b",
          "message": "Bump actions/checkout from 3 to 4\n\nBumps [actions/checkout](https://github.com/actions/checkout) from 3 to 4.\n- [Release notes](https://github.com/actions/checkout/releases)\n- [Changelog](https://github.com/actions/checkout/blob/main/CHANGELOG.md)\n- [Commits](https://github.com/actions/checkout/compare/v3...v4)\n\n---\nupdated-dependencies:\n- dependency-name: actions/checkout\n  dependency-type: direct:production\n  update-type: version-update:semver-major\n...\n\nSigned-off-by: dependabot[bot] <support@github.com>",
          "timestamp": "2023-11-03T15:47:20-07:00",
          "tree_id": "93d03e782edc71d916507770f325c055226d124b",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/c68b5c4172259d450f02f8e050ecad53b4a6214b"
        },
        "date": 1699052336258,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 19.592,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 16.39,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 0.9305,
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
          "id": "192b50eb6b16809f604599585a3a2c7004ac3aa9",
          "message": "Bump go-fuse to v2.4.1\n\nUpgrading go-fuse to a tagged revision which contains\nthe bug fix for the sparse file `cp/mv/install` bug on\nimages with coreutils version >= v9.0.\n\nSigned-off-by: Yasin Turan <turyasin@amazon.com>",
          "timestamp": "2023-11-07T15:50:32-05:00",
          "tree_id": "b050ecf4574d99b17b50b607ee525c572b24c535",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/192b50eb6b16809f604599585a3a2c7004ac3aa9"
        },
        "date": 1699391105416,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 15.739,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 9.4855,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 3.178,
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
          "id": "f25e4591adb3524981f2dafb19b196453d7f201a",
          "message": "Allow benchmarks config as json\n\nBenchmarks can currently be passed as a csv file. This additionally adds\njson as an option to make it not order dependent and to allow us to add\nmore complex options in the future (e.g. mounts).\n\nSigned-off-by: Kern Walster <walster@amazon.com>",
          "timestamp": "2023-11-08T11:52:53-08:00",
          "tree_id": "6900591438deee7739ff3aa04b4f6977d1ca789e",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/f25e4591adb3524981f2dafb19b196453d7f201a"
        },
        "date": 1699473744957,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 14.594999999999999,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.275,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.0095,
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
          "id": "bc17f0851c97218562bf7786a940f37904a44758",
          "message": "Remove \"soci run\" from CLI\n\nWith the effort to remove ctr code from CLI, we removed `soci run`,\nas it ran `ctr run` under the hood with no additional functionality.\nIn the testing suite, `soci run` has been replaced in favor of\n`nerdctl run` for similar reasons to above.\n\nSigned-off-by: David Son <davbson@amazon.com>",
          "timestamp": "2023-11-08T12:00:37-08:00",
          "tree_id": "eb0f665e3fc6779665c3c9061751ae4ed663d772",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/bc17f0851c97218562bf7786a940f37904a44758"
        },
        "date": 1699474141773,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 14.813500000000001,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.4475,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 0.899,
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
          "id": "1cafe7c668de7dfeba8c5f573bfc58b5c5b07940",
          "message": "Bump dependencies using scripts/bump-deps.sh\n\nSigned-off-by: GitHub <noreply@github.com>",
          "timestamp": "2023-11-08T11:53:12-08:00",
          "tree_id": "1a5da9f95d6de6f5083a9908da352d04a2f1321b",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/1cafe7c668de7dfeba8c5f573bfc58b5c5b07940"
        },
        "date": 1699474350439,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 25.1805,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 16.795,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 2.3805,
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
          "id": "8dfb7071444c53c3bbdc8c10cbc45eb940186589",
          "message": "Switch release workflow to on.push\n\nBefore this change, the release workflow was running when creating a\nbranch.\n\nSigned-off-by: Kern Walster <walster@amazon.com>",
          "timestamp": "2023-11-14T11:35:24-08:00",
          "tree_id": "26af82720df0e8873c20a4fa461902ad82469d07",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/8dfb7071444c53c3bbdc8c10cbc45eb940186589"
        },
        "date": 1699991116367,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 10.898,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.599,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 0.895,
            "unit": "Seconds",
            "extra": "P90"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "me@tuananh.org",
            "name": "Tuan Anh Tran",
            "username": "tuananh"
          },
          "committer": {
            "email": "55906459+austinvazquez@users.noreply.github.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "distinct": true,
          "id": "3aaee249831a3c45152acfdd05b3cdd1e83187a2",
          "message": "fix: trim unix:// prefix for address flag\n\nSigned-off-by: Tuan Anh Tran <me@tuananh.org>",
          "timestamp": "2023-11-14T20:53:31-08:00",
          "tree_id": "9cf995711952c3013521e9b0d94a55a18b9608bc",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/3aaee249831a3c45152acfdd05b3cdd1e83187a2"
        },
        "date": 1700024498250,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 10.0275,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.3695,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 0.9239999999999999,
            "unit": "Seconds",
            "extra": "P90"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "me@tuananh.org",
            "name": "Tuan Anh Tran",
            "username": "tuananh"
          },
          "committer": {
            "email": "66654647+turan18@users.noreply.github.com",
            "name": "Yasin Turan",
            "username": "turan18"
          },
          "distinct": true,
          "id": "bfd6bb6112413e647db6e5625628de2a842a8a10",
          "message": "feat: export emptyindex error when ztoc empty\n\nSigned-off-by: Tuan Anh Tran <me@tuananh.org>",
          "timestamp": "2023-11-15T14:31:51-05:00",
          "tree_id": "76509bc88bb8da16a7313a0d98683f7ba74713b2",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/bfd6bb6112413e647db6e5625628de2a842a8a10"
        },
        "date": 1700077213985,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 9.8995,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.1935,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 0.7915000000000001,
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
          "id": "f6759462a16208204a8e2da5195efdbf172068d2",
          "message": "Add image-specific options to benchmarker\n\nAll of our existing benchmark images are run with the same set of containerd\noptions which are only configurable at the test level to control things\nlike which snapshotter is used.\n\nThis is a problem for benchmarking GPU workloads, for example, where we need to\npass additional options to mount the GPU in the container which don't\napply to all images in the test.\n\nAdditionally, our benchmarker assumes that the benchmarked images\nrequire no configuration, however this can make experimentation hard in\ncases where a single base-image can be used for multiple use cases\ndepending on environment variables, confiration mounts, etc.\n\nThis change adds the ability to configure image-specific options when\nloading benchmarks from json. The options are not required and if not\npassed, the benchmarker will behave as it did before this change.\n\nThe set of options available in this change are those that were\nnecessary for benchmarking the LLM workloads that I was trying to test.\nThey are not comprehensive, but can be built upon as use cases arise.\n\nSigned-off-by: Kern Walster <walster@amazon.com>",
          "timestamp": "2023-11-15T15:58:08-08:00",
          "tree_id": "c0284443fb1a4262865509a911539609ccf90ce8",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/f6759462a16208204a8e2da5195efdbf172068d2"
        },
        "date": 1700093160436,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 9.709,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.4030000000000005,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 0.778,
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
            "email": "55906459+austinvazquez@users.noreply.github.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "distinct": true,
          "id": "87ab22b4d4f0ad7f069e3a0dd71487529c1b67e3",
          "message": "Bump dependencies using scripts/bump-deps.sh\n\nSigned-off-by: GitHub <noreply@github.com>",
          "timestamp": "2023-11-16T08:56:04-08:00",
          "tree_id": "6bcc78163298c03b8acfcdc5dd7f17ba7e6624ca",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/87ab22b4d4f0ad7f069e3a0dd71487529c1b67e3"
        },
        "date": 1700154383817,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 16.0835,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 8.303999999999998,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.264,
            "unit": "Seconds",
            "extra": "P90"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "macedonv@amazon.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "committer": {
            "email": "66654647+turan18@users.noreply.github.com",
            "name": "Yasin Turan",
            "username": "turan18"
          },
          "distinct": true,
          "id": "e1c66c057b5de0afb278058519ea20844309504a",
          "message": "Redact HTTP query values from logs\n\nHTTP client logs are mostly disabled with the exception of a request\nretry log. The issue observed is that error messages may contain the\nfull HTTP request including the query component which can contain\nsensitive information like credentials or session tokens. To prevent\nleaking sensitive information, this change will redact HTTP query values\nfrom log messages.\n\nSigned-off-by: Austin Vazquez <macedonv@amazon.com>",
          "timestamp": "2023-11-17T11:00:27-05:00",
          "tree_id": "4f20d050b4267fdd5f9eee60638de4d5c7b4d540",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/e1c66c057b5de0afb278058519ea20844309504a"
        },
        "date": 1700237691894,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 22.104,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.7315000000000005,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 2.1065,
            "unit": "Seconds",
            "extra": "P90"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "me@tuananh.org",
            "name": "Tuan Anh Tran",
            "username": "tuananh"
          },
          "committer": {
            "email": "55906459+austinvazquez@users.noreply.github.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "distinct": true,
          "id": "e6dfa2426f39ca083166c8c9bca72610f34bd8ed",
          "message": "fix: remove unnecessary conversion\n\nSigned-off-by: Tuan Anh Tran <me@tuananh.org>",
          "timestamp": "2023-11-17T17:03:12-08:00",
          "tree_id": "ee90030e5a52b5b5c5cc4dc726ed6710040dbd21",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/e6dfa2426f39ca083166c8c9bca72610f34bd8ed"
        },
        "date": 1700269960636,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 10.9035,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.5135000000000005,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 0.9225,
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
          "id": "6dcfca7bc0c70b00a5f23813d468379d6fbc8de5",
          "message": "Support re-authentication on 403\n\nAlthough in most cases 403 responses represent authorization\nissues that generally cannot be resolved by re-authentication,\nsome registries like ECR, will return a 403 on credential\nexpiration. We will attempt to re-authenticate only if the\nresponse body indicates credential expiration.\n\nRef: https://docs.aws.amazon.com/AmazonECR/latest/userguide/common-errors-docker.html#error-403\n\nSigned-off-by: Yasin Turan <turyasin@amazon.com>",
          "timestamp": "2023-11-20T11:16:12-05:00",
          "tree_id": "ea5f244f00095de13836e211f3bf733f3d7157ab",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/6dcfca7bc0c70b00a5f23813d468379d6fbc8de5"
        },
        "date": 1700497546809,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 11.7095,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.7515,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.8094999999999999,
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
          "id": "39aec385976cb8a2e9b7eef754754143fa76c47c",
          "message": "Add unpack stats to benchmarker\n\nSigned-off-by: Kern Walster <walster@amazon.com>",
          "timestamp": "2023-11-22T12:03:11-08:00",
          "tree_id": "8f1cec2ecbe03e4ccf9380e6de5a4cd7da142e82",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/39aec385976cb8a2e9b7eef754754143fa76c47c"
        },
        "date": 1700684010105,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 10.3095,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.284,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 0.876,
            "unit": "Seconds",
            "extra": "P90"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "macedonv@amazon.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "committer": {
            "email": "55906459+austinvazquez@users.noreply.github.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "distinct": true,
          "id": "21ec5445ea5e0908861e60e92cbdcd70d3251c93",
          "message": "Enable build workflow for release branches\n\nSigned-off-by: Austin Vazquez <macedonv@amazon.com>",
          "timestamp": "2023-11-22T13:21:25-08:00",
          "tree_id": "84405cc3b84969b99dcdfb52381154e9711b7823",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/21ec5445ea5e0908861e60e92cbdcd70d3251c93"
        },
        "date": 1700688682539,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 15.5135,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.7010000000000005,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 0.9844999999999999,
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
          "id": "34e03174462b00b48b8ecbbe25d75dc6c53c8d06",
          "message": "Bump dependencies using scripts/bump-deps.sh\n\nSigned-off-by: GitHub <noreply@github.com>",
          "timestamp": "2023-11-29T15:33:47-05:00",
          "tree_id": "53f5afd4f99bf2d3fec7b3e365cce33961ee1454",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/34e03174462b00b48b8ecbbe25d75dc6c53c8d06"
        },
        "date": 1701290573708,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 11.539000000000001,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 8.003,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.082,
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
          "id": "cf25e82c4f729232981c5146890055da518007b3",
          "message": "Store file name as is in metadata DB\n\nRight now, when converting the TOC to metadata entries, we call `path.Clean`\non every file name before writing it to metadata DB. Calling `path.Clean`\non a directory path removes the trailing separator. This isn't directly a problem\nsince we only ever perform TAR header file name validation on file reads, not\ndirectories, since the kernel VFS disallows reads on directories (`EISDIR`).\nCleaning the path, however, also removes the current working directory token\n(`./`) from a path. This means that if a path in a TAR file were prefixed with\n`./`, we would clean the path, removing the `./`, in turn causing our TAR header\nfile name validation check to fail when we attempt to read from the file.\nTo avoid TAR edge cases like this one, we should store TAR names as is in\nour metadata DB.\n\nSigned-off-by: Yasin Turan <turyasin@amazon.com>",
          "timestamp": "2023-12-07T11:41:06-05:00",
          "tree_id": "568172f3946feb88fd9610a67f0f34b4806247a5",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/cf25e82c4f729232981c5146890055da518007b3"
        },
        "date": 1701967870837,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 13.474,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.7265,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.282,
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
          "id": "d43b09491beced4c09273659e8b0676a43994b27",
          "message": "Bump dependencies using scripts/bump-deps.sh\n\nSigned-off-by: GitHub <noreply@github.com>",
          "timestamp": "2023-12-08T13:57:32-05:00",
          "tree_id": "3e6ee8095f6506875e974a78b7a912114b7415e8",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/d43b09491beced4c09273659e8b0676a43994b27"
        },
        "date": 1702062684601,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 19.102,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.4595,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.7530000000000001,
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
          "id": "57b3b667cd2d96bdc34281b0db58bbdefac18a39",
          "message": "Free zTOC from memory\n\nBefore this change, the uncompressed zTOC would stay in memory. This\nwas because when converting the full uncompressed bytes into a struct,\nwe erreneously retained a reference to the original byte array in\nztoc.Checkpoints, because compressionInfo.CheckpointsBytes() returns\na slice of the uncompressed bytes. This change copies the bytes into\na dedicated buffer to free up the full uncompressed byte array.\n\nSigned-off-by: David Son <davbson@amazon.com>",
          "timestamp": "2023-12-08T15:07:17-08:00",
          "tree_id": "24c7d75997cc2ae4f63cb9c5b552f88b6ff9db98",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/57b3b667cd2d96bdc34281b0db58bbdefac18a39"
        },
        "date": 1702077455182,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 14.776,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.4205,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.324,
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
            "email": "55906459+austinvazquez@users.noreply.github.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "distinct": true,
          "id": "8752222b64ecf6173468e3286c5d02071ec1d4b5",
          "message": "Bump actions/setup-go from 4 to 5\n\nBumps [actions/setup-go](https://github.com/actions/setup-go) from 4 to 5.\n- [Release notes](https://github.com/actions/setup-go/releases)\n- [Commits](https://github.com/actions/setup-go/compare/v4...v5)\n\n---\nupdated-dependencies:\n- dependency-name: actions/setup-go\n  dependency-type: direct:production\n  update-type: version-update:semver-major\n...\n\nSigned-off-by: dependabot[bot] <support@github.com>",
          "timestamp": "2023-12-11T07:54:55-08:00",
          "tree_id": "d9af9a69647dbe0ab0c472408f548b6ade53466c",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/8752222b64ecf6173468e3286c5d02071ec1d4b5"
        },
        "date": 1702310605919,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 10.174499999999998,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.4345,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.0165,
            "unit": "Seconds",
            "extra": "P90"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "macedonv@amazon.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "committer": {
            "email": "55906459+austinvazquez@users.noreply.github.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "distinct": true,
          "id": "f8a6d298e741c4871bee930df32a1577c2062562",
          "message": "Adds a newline to pretty print JSON\n\nSigned-off-by: Austin Vazquez <macedonv@amazon.com>",
          "timestamp": "2023-12-11T13:20:10-08:00",
          "tree_id": "c03896464f8e23b94950362b9910a1ad008694bc",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/f8a6d298e741c4871bee930df32a1577c2062562"
        },
        "date": 1702330169293,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 10.879,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.58,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.3555,
            "unit": "Seconds",
            "extra": "P90"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "macedonv@amazon.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "committer": {
            "email": "55906459+austinvazquez@users.noreply.github.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "distinct": true,
          "id": "7044591a8acbe9148cf5e740540d894840210a7f",
          "message": "Fix index info command with containerd content store\n\nPreviously SOCI CLI index info command would fail with context deadline\nexceeded error when the content store was set to containerd. The root\ncause was the default global duration for the app context is zero if not\nset. The result was Go context with an immediate deadline thus resulting\nin the error. The fix is to not set a deadline if the duration is zero.\n\nSigned-off-by: Austin Vazquez <macedonv@amazon.com>",
          "timestamp": "2023-12-11T13:20:32-08:00",
          "tree_id": "12c0ae241bca30706abc3dd2a667691d0d77079a",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/7044591a8acbe9148cf5e740540d894840210a7f"
        },
        "date": 1702330264114,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 9.4495,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 6.849,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.303,
            "unit": "Seconds",
            "extra": "P90"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "macedonv@amazon.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "committer": {
            "email": "55906459+austinvazquez@users.noreply.github.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "distinct": true,
          "id": "5d1022b0def534e457fd0f649b99e3874dc38eaa",
          "message": "Update Go version in CI to v1.20.12\n\nSigned-off-by: Austin Vazquez <macedonv@amazon.com>",
          "timestamp": "2023-12-12T10:34:32-08:00",
          "tree_id": "4253100a6509502208864c5e41005b382f37c038",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/5d1022b0def534e457fd0f649b99e3874dc38eaa"
        },
        "date": 1702406757509,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 10.013,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.3325,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 0.8779999999999999,
            "unit": "Seconds",
            "extra": "P90"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "carlhilt@amazon.com",
            "name": "Carl Hiltbrunner",
            "username": "Subzidion"
          },
          "committer": {
            "email": "55906459+austinvazquez@users.noreply.github.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "distinct": true,
          "id": "462303a7635a3df834e84a9cbcc15d3efa1b62bc",
          "message": "Log successful startup of soci-snapshotter-grpc\n\nSigned-off-by: Carl Hiltbrunner <carlhilt@amazon.com>",
          "timestamp": "2023-12-15T09:36:03-08:00",
          "tree_id": "69b9c908e95cca8c2d02c1375ec74784a915484a",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/462303a7635a3df834e84a9cbcc15d3efa1b62bc"
        },
        "date": 1702662475888,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 15.5915,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.1805,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.6325,
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
          "id": "d40fe8b3d990ffe88af9d9fb20ac020145670af9",
          "message": "Bump dependencies using scripts/bump-deps.sh\n\nSigned-off-by: GitHub <noreply@github.com>",
          "timestamp": "2023-12-15T15:01:56-08:00",
          "tree_id": "1f1eb1257f1687ea7651ed8d81ea91dd22e34d76",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/d40fe8b3d990ffe88af9d9fb20ac020145670af9"
        },
        "date": 1702681870540,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 10.364,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.2940000000000005,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 0.9085,
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
            "email": "55906459+austinvazquez@users.noreply.github.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "distinct": true,
          "id": "b09af65431afdbef231ea932e3fd994d1910e8ff",
          "message": "Bump actions/download-artifact from 3 to 4\n\nBumps [actions/download-artifact](https://github.com/actions/download-artifact) from 3 to 4.\n- [Release notes](https://github.com/actions/download-artifact/releases)\n- [Commits](https://github.com/actions/download-artifact/compare/v3...v4)\n\n---\nupdated-dependencies:\n- dependency-name: actions/download-artifact\n  dependency-type: direct:production\n  update-type: version-update:semver-major\n...\n\nSigned-off-by: dependabot[bot] <support@github.com>",
          "timestamp": "2023-12-18T07:21:08-08:00",
          "tree_id": "c883aeeeef2bc97491303d65ff53a3da1ecbbf43",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/b09af65431afdbef231ea932e3fd994d1910e8ff"
        },
        "date": 1702913494142,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 17.499,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.2364999999999995,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.08,
            "unit": "Seconds",
            "extra": "P90"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "macedonv@amazon.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "committer": {
            "email": "55906459+austinvazquez@users.noreply.github.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "distinct": true,
          "id": "579268175a0fdde7c54ef14da0f57424d27b7dd8",
          "message": "Remove test dependency on docker-compose\n\nSigned-off-by: Austin Vazquez <macedonv@amazon.com>",
          "timestamp": "2023-12-18T07:23:33-08:00",
          "tree_id": "9a83a3257e40a625e3aa8c0cd16f0254772fd1a0",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/579268175a0fdde7c54ef14da0f57424d27b7dd8"
        },
        "date": 1702913523469,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 10.236,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.385,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 0.8879999999999999,
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
          "id": "74be0dae7e05d68428fef0a3b37173c7af51bd7d",
          "message": "Remove \"image rpull\" from CLI\n\nAs an effort to streamline our CLI usage, \"image rpull\" has been removed.\nWhile it technically retains some functionality special to SOCI, it is\nultimately up to the CLI to pass the requisite parameters to the remote\nsnapshotter.\n\nThis also removes the last of our references to ctr.\n\nSigned-off-by: David Son <davbson@amazon.com>",
          "timestamp": "2023-12-20T09:51:05-08:00",
          "tree_id": "43d866002e1b18ba9b4782468a09a9a9e3b3ce54",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/74be0dae7e05d68428fef0a3b37173c7af51bd7d"
        },
        "date": 1703095458788,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 17.3295,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.8325,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.7240000000000002,
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
          "id": "680cbba65e8e81ceac04284698cc32ab583f480e",
          "message": "Add documentation for most TOML variables\n\nSigned-off-by: David Son <davbson@amazon.com>",
          "timestamp": "2023-12-20T13:39:08-08:00",
          "tree_id": "0ebcae466efd28c0a6d3bdada5942101c6fad9ae",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/680cbba65e8e81ceac04284698cc32ab583f480e"
        },
        "date": 1703108945924,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 12.174499999999998,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.091,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.492,
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
          "id": "26cd9bc358cd4f560dda30128f102dc2f60b8de9",
          "message": "Change LogMonitor to look for correct string\n\nSigned-off-by: David Son <davbson@amazon.com>",
          "timestamp": "2023-12-20T13:39:28-08:00",
          "tree_id": "6a65d1f5ca7f9384acaf44253b315e170ab9f9d9",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/26cd9bc358cd4f560dda30128f102dc2f60b8de9"
        },
        "date": 1703109049041,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 13.604,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.3645,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.8055,
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
            "email": "55555210+sondavidb@users.noreply.github.com",
            "name": "David Son",
            "username": "sondavidb"
          },
          "distinct": true,
          "id": "9eeadeb0ddf04558e85e5d652e02bf3ab54c4616",
          "message": "Bump dependencies using scripts/bump-deps.sh\n\nSigned-off-by: GitHub <noreply@github.com>",
          "timestamp": "2023-12-20T15:38:59-08:00",
          "tree_id": "fbaee144c35b51583693af0d63a48f80647140b8",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/9eeadeb0ddf04558e85e5d652e02bf3ab54c4616"
        },
        "date": 1703116253220,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 16.468,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.164,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.4135,
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
          "id": "666ef0467e8111d1e57a43d9273ac8bcc770edca",
          "message": "Unified HTTP client\n\nRight now, the snapshotter maintains a single HTTP client for fetching\nSOCI artifacts and `n` clients for every layer in an image (used to fetch\nspans/layers). Every client maintains its own credential cache, meaning\nwe have to re-authenticate an extra `n` times every time we need to fetch/\nrefresh credentials. This change unifies client creation at a global level\n(a single global retryable client) and authentication at an image level,\nwhere we we create a new `AuthClient` for every image. The AuthClient is\nresponsible for authenticating with registries and sending the request\nout via it's inner retryable HTTP client. This effectively reduces the\namount of round trips we make to registries/authorization servers,\nreducing the risk of network failures.\n\nThis change also fixes a bug in our blob/http fetcher where we always\ncache the base blob URL as the redirected/\"real\" URL, even if the\nblob existed in a nested storage backend.\n\nSigned-off-by: Yasin Turan <turyasin@amazon.com>",
          "timestamp": "2023-12-21T19:09:05-05:00",
          "tree_id": "d9440d0145a0b7530fc3c228316c60f6c79d151e",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/666ef0467e8111d1e57a43d9273ac8bcc770edca"
        },
        "date": 1703204323107,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 15.847999999999999,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.2989999999999995,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 2.9905,
            "unit": "Seconds",
            "extra": "P90"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "anhtt109@vpbank.com.vn",
            "name": "Tuan Anh Tran"
          },
          "committer": {
            "email": "55555210+sondavidb@users.noreply.github.com",
            "name": "David Son",
            "username": "sondavidb"
          },
          "distinct": true,
          "id": "91ce7a8fcb74deb01a950b891b7c8ad5d0dcb2e1",
          "message": "fix: fix: strip path in release tar\n\nSigned-off-by: Tuan Anh Tran <anhtt109@vpbank.com.vn>",
          "timestamp": "2023-12-24T23:39:32-08:00",
          "tree_id": "b588108a93af66d065e4a1fc252b7b9c93e4f093",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/91ce7a8fcb74deb01a950b891b7c8ad5d0dcb2e1"
        },
        "date": 1703490543882,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 14.1555,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.211,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.3815,
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
          "id": "00b3501b309d13c8e00e8c244c55476943cdfbb6",
          "message": "Bump dependencies using scripts/bump-deps.sh\n\nSigned-off-by: GitHub <noreply@github.com>",
          "timestamp": "2023-12-28T12:27:50-05:00",
          "tree_id": "65e67be48e29664c95e19c5fcfebb922ad3925aa",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/00b3501b309d13c8e00e8c244c55476943cdfbb6"
        },
        "date": 1703785033144,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 14.604,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.215,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.6575,
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
          "id": "4d0cf944bc61d727a9a6c4b905caef4992451c93",
          "message": "Bump dependencies using scripts/bump-deps.sh\n\nSigned-off-by: GitHub <noreply@github.com>",
          "timestamp": "2024-01-03T10:08:58-08:00",
          "tree_id": "af4f98884c8ee9d8b0bc62d11e3794c34b84dc4c",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/4d0cf944bc61d727a9a6c4b905caef4992451c93"
        },
        "date": 1704305972440,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 14.601500000000001,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.202500000000001,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.6625,
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
          "id": "f925c9d9c1e8ca02fc6059467331d8d0dd2bede4",
          "message": "Add ztoc generation benchmark\n\nWe have benchmarks for pulling and running images with OverlayFS vs\nSOCI, but we don't have any benchmarking for how long it takes to build\nztocs. This adds a benchmark for building ztocs to give us directional\ninformation about how the changes we make affect build times.\n\nSigned-off-by: Kern Walster <walster@amazon.com>",
          "timestamp": "2024-01-05T14:45:29-08:00",
          "tree_id": "a93c448548cb6f6cc723e903be03c9548cd332ae",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/f925c9d9c1e8ca02fc6059467331d8d0dd2bede4"
        },
        "date": 1704495123909,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 8.6075,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.4075,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 0.851,
            "unit": "Seconds",
            "extra": "P90"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "macedonv@amazon.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "committer": {
            "email": "55906459+austinvazquez@users.noreply.github.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "distinct": true,
          "id": "88bcf75788de454dd03137b65ed29c7e662e0333",
          "message": "Add verification step to release automation\n\nThis change includes enhancements to the release automation workflow.\nThe primary focus is adding a release artifact verification script to\nthe automation to validate release artifact contents and checksums.\n\nOther minor changes include reordering of release automation workflow\njobs and declaration of job environment variables to resolve warnings.\n\nSigned-off-by: Austin Vazquez <macedonv@amazon.com>",
          "timestamp": "2024-01-05T17:53:13-06:00",
          "tree_id": "a5c0a4662d4f3a3e6f689665fc4889f0b8866402",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/88bcf75788de454dd03137b65ed29c7e662e0333"
        },
        "date": 1704499237878,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 8.840499999999999,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.333,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 0.8694999999999999,
            "unit": "Seconds",
            "extra": "P90"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "macedonv@amazon.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "committer": {
            "email": "55906459+austinvazquez@users.noreply.github.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "distinct": true,
          "id": "44ab5b734c6141b1edc72a3b9cc0df9dd840919d",
          "message": "Fix containerd socket address when using containerd as content store\n\nPreviously the CLI and snapshotter service would use\n'/run/containerd/containerd.sock' as the containerd socket when using\ncontainerd as content store. This resulted in errors for users not using\nthe default install path for containerd. This change allows for pass\nthrough of `--address` flag to content store and configuration of\ncontaienrd socket in SOCI config.toml.\n\nSigned-off-by: Austin Vazquez <macedonv@amazon.com>",
          "timestamp": "2024-01-09T09:12:19-06:00",
          "tree_id": "83043e1d13d0c3d40a0d4c04f18fb2c65e7668eb",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/44ab5b734c6141b1edc72a3b9cc0df9dd840919d"
        },
        "date": 1704813569902,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 14.075499999999998,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.0875,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.6835,
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
          "id": "a8d99b93c13c9f96e771ce36cdf395bf80f7a88d",
          "message": "Revert Credential function signature\n\nCommit 666ef04 introduced the notion of an `AuthClient`. We create new\n`AuthClient`'s for every unique image reference since credentials can be\nscoped to specific images/repositories. This also included mirrors,\nmeaning mirror credentials are completely independent of the host\ncredentials. For the most part this is correct, but the CRI\nimplementation in `containerd` does not do this, instead they try to use\nthe same CRI/kubelet credentials for every endpoint (host and mirrors),\nunless there are host credentials directly provided in the config. We should\nadopt this same policy as-well. This means we'll have to re-introduce the\n`host` argument in our `Credential` type, so that when we attempt to get\ncredentials through our CRI implementation for a mirror, we try to use\nthe credentials for the base host first. If that fails, we can try other\ncredential providers using the `host` argument as our index.\n\nThis also prevents us from blindly sending the credentials we have\ncached in our CRI implementation for a specific image reference when the\nauthorizer asks for a set of credentials for a host.\n\neg: If we make a request for image ref `host.io/namespace/repo:latest`\nand we somehow get a 401 response from some other host `differenthost.io`,\nwe shouldn't send the credentials for `host.io/namespace/repo:latest`.\n\nSigned-off-by: Yasin Turan <turyasin@amazon.com>",
          "timestamp": "2024-01-09T10:13:03-05:00",
          "tree_id": "5f22db046f18ff2a283a58d4d26e652fe5dd2515",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/a8d99b93c13c9f96e771ce36cdf395bf80f7a88d"
        },
        "date": 1704813608903,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 8.868500000000001,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.8065,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.2315,
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
          "id": "95c139e65a4bdfca058cec324403afad0f8fcd58",
          "message": "Bump dependencies using scripts/bump-deps.sh\n\nSigned-off-by: GitHub <noreply@github.com>",
          "timestamp": "2024-01-10T14:07:37-05:00",
          "tree_id": "24b0d034d0bf46740134b1584469109e254b4365",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/95c139e65a4bdfca058cec324403afad0f8fcd58"
        },
        "date": 1704914090860,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 9.5045,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.2645,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 0.8385,
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
          "id": "2b56440396ffcb8403e82b1f57d528704a5b2084",
          "message": "Eagerly resolve local blobs\n\nFor local snapshots, we previously encountered a bug where the\nsize of a layer was resported as zero in its descriptor. This caused the\nFetch function to attempt to resolve it as a manifest, which would then\ncause the local snapshot to defer to container runtime, causing\ncontainerd to fetch all remaining layers ahead of time.\nThis change resolves the local blob earlier to populate\nthe size field, avoiding this issue.\n\nSigned-off-by: David Son <davbson@amazon.com>",
          "timestamp": "2024-01-10T12:31:36-08:00",
          "tree_id": "b595ef98cc1cd0b41af9e7e8c5c817ea505a36c5",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/2b56440396ffcb8403e82b1f57d528704a5b2084"
        },
        "date": 1704919124191,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 8.9985,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.185499999999999,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.3755000000000002,
            "unit": "Seconds",
            "extra": "P90"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "macedonv@amazon.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "committer": {
            "email": "55555210+sondavidb@users.noreply.github.com",
            "name": "David Son",
            "username": "sondavidb"
          },
          "distinct": true,
          "id": "47839a01b06ce50c6f5415e4c13ecfc6d75b0f72",
          "message": "Run build workflow on change\n\nSigned-off-by: Austin Vazquez <macedonv@amazon.com>",
          "timestamp": "2024-01-11T10:41:00-08:00",
          "tree_id": "5e1c5f3e0862812a770eea6e538f1f7adb34f34e",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/47839a01b06ce50c6f5415e4c13ecfc6d75b0f72"
        },
        "date": 1704998851176,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 8.4725,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.226,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 0.848,
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
          "id": "21181edae6b06ce5511bf72eb8204c853fee3ae1",
          "message": "Update containerd versions in workflows\n\nv1.6.19 -> v1.6.26\nv1.7.0 -> v1.7.11\n\nSigned-off-by: David Son <davbson@amazon.com>",
          "timestamp": "2024-01-11T13:44:06-08:00",
          "tree_id": "71870bc042f89c39d6228997516958c1d25c44dd",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/21181edae6b06ce5511bf72eb8204c853fee3ae1"
        },
        "date": 1705010053124,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 14.849,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 8.0535,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.558,
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
          "id": "9edd90fdd030672ec7383a20ecc91f3da304fa24",
          "message": "Update Go version in workflows\n\nv1.20.12 -> v1.20.13\n\nSigned-off-by: David Son <davbson@amazon.com>",
          "timestamp": "2024-01-11T15:35:34-08:00",
          "tree_id": "04631c82256e8abfe3d6f1addb9b702fd8f802dd",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/9edd90fdd030672ec7383a20ecc91f3da304fa24"
        },
        "date": 1705016722077,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 12.3395,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.3469999999999995,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.5310000000000001,
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
          "id": "adaef47049c7a81ec4461cbe72a31508edd09728",
          "message": "Add +x to scripts/verify-release-artifacts.sh\n\nSigned-off-by: David Son <davbson@amazon.com>",
          "timestamp": "2024-01-12T15:25:53-08:00",
          "tree_id": "7d53fe978bcc9641639fd2f9acacee421c3a1433",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/adaef47049c7a81ec4461cbe72a31508edd09728"
        },
        "date": 1705102382603,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 10.762,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.3165,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.351,
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
            "email": "55906459+austinvazquez@users.noreply.github.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "distinct": true,
          "id": "c1152e9549df07609e6d31c0016e954bcbf71191",
          "message": "Bump dependencies using scripts/bump-deps.sh\n\nSigned-off-by: GitHub <noreply@github.com>",
          "timestamp": "2024-01-16T12:41:16-06:00",
          "tree_id": "14b917355e6147b7fa7335f43aa0a2e4b36d35f6",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/c1152e9549df07609e6d31c0016e954bcbf71191"
        },
        "date": 1705431021860,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 16.942,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.2415,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.072,
            "unit": "Seconds",
            "extra": "P90"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "davanum@gmail.com",
            "name": "Davanum Srinivas",
            "username": "dims"
          },
          "committer": {
            "email": "55906459+austinvazquez@users.noreply.github.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "distinct": true,
          "id": "ece7c95acd735292d78b52f7daa6219396cc5d1b",
          "message": "Call fs.Unmount only if needed\n\nSigned-off-by: Davanum Srinivas <davanum@gmail.com>",
          "timestamp": "2024-01-16T14:03:44-06:00",
          "tree_id": "56769120b2b7ccae7718208b767d4081bc65a910",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/ece7c95acd735292d78b52f7daa6219396cc5d1b"
        },
        "date": 1705435974288,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 9.798,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.6145,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.3285,
            "unit": "Seconds",
            "extra": "P90"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "davanum@gmail.com",
            "name": "Davanum Srinivas",
            "username": "dims"
          },
          "committer": {
            "email": "66654647+turan18@users.noreply.github.com",
            "name": "Yasin Turan",
            "username": "turan18"
          },
          "distinct": true,
          "id": "335515f746f50c964ed48159257e1aeba04805b6",
          "message": "Leave Debug breadcrumbs for snapshotter functions called\n\nSigned-off-by: Davanum Srinivas <davanum@gmail.com>",
          "timestamp": "2024-01-17T16:05:55-05:00",
          "tree_id": "d14ab6a942093823a58e7c3a4b59a32a491cbfac",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/335515f746f50c964ed48159257e1aeba04805b6"
        },
        "date": 1705526061548,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 11.106,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.575,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.5819999999999999,
            "unit": "Seconds",
            "extra": "P90"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "macedonv@amazon.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "committer": {
            "email": "55906459+austinvazquez@users.noreply.github.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "distinct": true,
          "id": "9731bb29b37b0fb4e5b2c4f366f05eb38b4464da",
          "message": "Disable release note generation in release automation\n\nAs part of the v0.5.0 release, automation generated release notes which\nincluded every commit in history. Instead the desired effect was to only\ninclude the diff since the last release. Disabling for now until another\nsolution is found.\n\nSigned-off-by: Austin Vazquez <macedonv@amazon.com>",
          "timestamp": "2024-01-22T09:06:06-06:00",
          "tree_id": "200c2b6be8e694e5f1f894c12225f557033b6610",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/9731bb29b37b0fb4e5b2c4f366f05eb38b4464da"
        },
        "date": 1705936387352,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 10.8555,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.807,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.335,
            "unit": "Seconds",
            "extra": "P90"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "akihiro.suda.cz@hco.ntt.co.jp",
            "name": "Akihiro Suda",
            "username": "AkihiroSuda"
          },
          "committer": {
            "email": "55906459+austinvazquez@users.noreply.github.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "distinct": true,
          "id": "8d2b306690f9484a21c29963c6c56315580258fa",
          "message": "fs/source: drop indirect dependency on k8s.io/cri-api\n\nnerdctl imports `soci-snapshotter/fs/source` but does not want to import\nk8s.io/cri-api\n\nSee containerd/nerdctl PR 2761\n\nSigned-off-by: Akihiro Suda <akihiro.suda.cz@hco.ntt.co.jp>",
          "timestamp": "2024-01-23T09:44:19-06:00",
          "tree_id": "17c65e4452569ff175e57513fd42ad066fb400f6",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/8d2b306690f9484a21c29963c6c56315580258fa"
        },
        "date": 1706025061065,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 9.352,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.4765,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 0.857,
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
            "email": "55555210+sondavidb@users.noreply.github.com",
            "name": "David Son",
            "username": "sondavidb"
          },
          "distinct": true,
          "id": "b81ba3ba594a5857e9e1d3e63ce09c6fb7ef1f58",
          "message": "Bump dependencies using scripts/bump-deps.sh\n\nSigned-off-by: GitHub <noreply@github.com>",
          "timestamp": "2024-01-23T10:29:09-08:00",
          "tree_id": "ad5bce5caa577b6818c311c80ea34d5a9e149280",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/b81ba3ba594a5857e9e1d3e63ce09c6fb7ef1f58"
        },
        "date": 1706035007103,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 8.867,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.1825,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.367,
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
          "id": "db7df3ab2a840bc927417a84448263885b3e21ff",
          "message": "Fix file descriptor leak\n\nSigned-off-by: David Son <davbson@amazon.com>",
          "timestamp": "2024-01-23T11:07:22-08:00",
          "tree_id": "4bb28524fbd14033535eaa41d6d75e165576cd76",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/db7df3ab2a840bc927417a84448263885b3e21ff"
        },
        "date": 1706037435918,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 17.1625,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.574,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 2.128,
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
            "email": "55906459+austinvazquez@users.noreply.github.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "distinct": true,
          "id": "ee0693e125229dc60bbd052e9808a10636d675c0",
          "message": "Bump dependencies using scripts/bump-deps.sh\n\nSigned-off-by: GitHub <noreply@github.com>",
          "timestamp": "2024-02-01T22:07:32-06:00",
          "tree_id": "dcd7e913a34d996e60ad9e2c7d1b4dd14f0a0888",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/ee0693e125229dc60bbd052e9808a10636d675c0"
        },
        "date": 1706847397935,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 10.5735,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.176500000000001,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.337,
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
          "id": "04a83aaa56243152a0c14828a12cb810997ea6d8",
          "message": "Modify artifact verification script\n\nAllowed artifact verification to be called without having to cd into\nthe release directory\n\nSigned-off-by: David Son <davbson@amazon.com>",
          "timestamp": "2024-02-02T15:24:48-08:00",
          "tree_id": "cd20bdccb2420224875483d34a685875ac332b02",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/04a83aaa56243152a0c14828a12cb810997ea6d8"
        },
        "date": 1706916724092,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 11.8705,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.373,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 0.8039999999999999,
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
            "email": "55906459+austinvazquez@users.noreply.github.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "distinct": true,
          "id": "f5333b01e5ca34fa8221f79a8a116e014e51ce08",
          "message": "Bump dependencies using scripts/bump-deps.sh\n\nSigned-off-by: GitHub <noreply@github.com>",
          "timestamp": "2024-02-06T15:20:58-06:00",
          "tree_id": "5cd3e2bb37e7ecaf2ad60e8f666db15103cf01ef",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/f5333b01e5ca34fa8221f79a8a116e014e51ce08"
        },
        "date": 1707255078570,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 14.424,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.246,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.4395,
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
            "email": "55555210+sondavidb@users.noreply.github.com",
            "name": "David Son",
            "username": "sondavidb"
          },
          "distinct": true,
          "id": "159e1a49352a822cea37b96def89166ce9b84cb7",
          "message": "Bump peter-evans/create-pull-request from 5 to 6\n\nBumps [peter-evans/create-pull-request](https://github.com/peter-evans/create-pull-request) from 5 to 6.\n- [Release notes](https://github.com/peter-evans/create-pull-request/releases)\n- [Commits](https://github.com/peter-evans/create-pull-request/compare/v5...v6)\n\n---\nupdated-dependencies:\n- dependency-name: peter-evans/create-pull-request\n  dependency-type: direct:production\n  update-type: version-update:semver-major\n...\n\nSigned-off-by: dependabot[bot] <support@github.com>",
          "timestamp": "2024-02-06T13:30:17-08:00",
          "tree_id": "0fc4616039af9921c6b677f414c4096387c49552",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/159e1a49352a822cea37b96def89166ce9b84cb7"
        },
        "date": 1707255411212,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 8.573,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.1114999999999995,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.23,
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
          "id": "0065fb0e834ee5c81cf60fe36f0b072f1c0393d0",
          "message": "Remove testing from release workflow\n\nTesting on tag push is somewhat redundant since we will not be pushing\nout new versions unless the previous commit has passing tests, and\nmanual testing on ARM must be done before the release tag is pushed, so\nthis just saves us some time.\n\nSigned-off-by: David Son <davbson@amazon.com>",
          "timestamp": "2024-02-06T16:48:02-08:00",
          "tree_id": "08e3a29b7fa38bc62549842128b9e0e32865a5c6",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/0065fb0e834ee5c81cf60fe36f0b072f1c0393d0"
        },
        "date": 1707267321163,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 9.015,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.284,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 0.73,
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
            "email": "55906459+austinvazquez@users.noreply.github.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "distinct": true,
          "id": "2e3df4a92415ff02ccc76ed9ceb1c25ef9b5c75f",
          "message": "Bump dependencies using scripts/bump-deps.sh\n\nSigned-off-by: github-actions[bot] <41898282+github-actions[bot]@users.noreply.github.com>",
          "timestamp": "2024-02-13T19:04:09-06:00",
          "tree_id": "3cb855e8c7f8f83aa00a0f752555989f1105e642",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/2e3df4a92415ff02ccc76ed9ceb1c25ef9b5c75f"
        },
        "date": 1707873409277,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 16.849,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.264,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.9075000000000002,
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
            "email": "55555210+sondavidb@users.noreply.github.com",
            "name": "David Son",
            "username": "sondavidb"
          },
          "distinct": true,
          "id": "e3b7acbff3356dc69c89a77dad5708546600b164",
          "message": "Bump dependencies using scripts/bump-deps.sh\n\nSigned-off-by: github-actions[bot] <41898282+github-actions[bot]@users.noreply.github.com>",
          "timestamp": "2024-02-20T13:02:22-08:00",
          "tree_id": "2ce6f05629dd20766d7a434efbf12fcc1e332ef0",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/e3b7acbff3356dc69c89a77dad5708546600b164"
        },
        "date": 1708463610222,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 17.887,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.5755,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.596,
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
          "id": "ae21d2640e9e409f73afa2c932a2965fbddb4994",
          "message": "Remove unparsed references from zTOC\n\nOur current zTOC structure uses more memory than needed. Specifically,\nthe Checkpoints and FileMetadata arrays only get called once, yet they\ncannot be freed because the SpanManager retains a reference to them,\ndespite never needing either past their initial calls. This is an\ninherent design flaw with our zTOC APIs, and this fix is a temporary\nworkaround to increase performance. Once we can flesh out the zTOC\nAPI, this solution can be much more elegant, or even unneeded.\n\nSigned-off-by: David Son <davbson@amazon.com>",
          "timestamp": "2024-02-26T15:28:08-08:00",
          "tree_id": "c1f0537042b441dfe3464ae0b4b8af168c972d9a",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/ae21d2640e9e409f73afa2c932a2965fbddb4994"
        },
        "date": 1708990555665,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 12.1785,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.6645,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.062,
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
          "id": "3cd8cd448a09b4f9d69213da3075b3d045fd6a73",
          "message": "Ensure bbolt KV pairs are inserted in key sorted order\n\nSigned-off-by: Yasin Turan <turyasin@amazon.com>",
          "timestamp": "2024-02-27T10:04:12-05:00",
          "tree_id": "8e93e925895b910d945fcfa0c0090ae2aeb0c7f6",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/3cd8cd448a09b4f9d69213da3075b3d045fd6a73"
        },
        "date": 1709046696595,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 8.981,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.723,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.3275,
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
          "id": "ba1058799622817963cbb694f0d9a3e5d3c2d9d5",
          "message": "Add concurrency limits\n\nSigned-off-by: David Son <davbson@amazon.com>",
          "timestamp": "2024-02-27T09:50:22-08:00",
          "tree_id": "eecb6b370deb0b58ff7cad4e5b019713c4d27469",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/ba1058799622817963cbb694f0d9a3e5d3c2d9d5"
        },
        "date": 1709056645654,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 8.913499999999999,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.436999999999999,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 0.8345,
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
            "email": "55906459+austinvazquez@users.noreply.github.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "distinct": true,
          "id": "acbe54dc00b7c8ac2d8bbe4a84b139c64add4ec5",
          "message": "Bump dependencies using scripts/bump-deps.sh\n\nSigned-off-by: github-actions[bot] <41898282+github-actions[bot]@users.noreply.github.com>",
          "timestamp": "2024-02-29T06:28:14-08:00",
          "tree_id": "2781bd169aeb9e0dcbe033099a195b0398f05081",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/acbe54dc00b7c8ac2d8bbe4a84b139c64add4ec5"
        },
        "date": 1709217484017,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 15.113,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.611,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.6835,
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
          "id": "0b85085ef6a20e1ef9aa95029794e72aaf1e0b2c",
          "message": "Fix tar archive generation\n\nOur release scripts erreneously created tar archives with the ./ prefix.\nThis change removes this bug.\n\nSigned-off-by: David Son <davbson@amazon.com>",
          "timestamp": "2024-03-01T14:23:07-08:00",
          "tree_id": "86b1942bb2a942779de7dfea65884819aa42e16e",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/0b85085ef6a20e1ef9aa95029794e72aaf1e0b2c"
        },
        "date": 1709332176466,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 8.751999999999999,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.446999999999999,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 0.7285,
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
          "id": "9bc4984587d46b8a8c4043858e564027ebed38ff",
          "message": "Temporarily modify DCO check\n\nWe erreneously allowed a non-signed commit into main, so our pre-build\nscript would fail as long as this commit was put into the checker. This\nchange temporarily only checks from every commit after this commit, and\nshould be reverted once it falls out of the original scope.\n\nSigned-off-by: David Son <davbson@amazon.com>",
          "timestamp": "2024-03-04T10:27:59-08:00",
          "tree_id": "de6942af406f7d20d624da1f3d6898b59d611af2",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/9bc4984587d46b8a8c4043858e564027ebed38ff"
        },
        "date": 1709577524697,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 13.549,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.1855,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.6935,
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
          "id": "bb526c30097754683e91abb7836252b1bc3663f4",
          "message": "Update RUNC_VERSION in Dockerfile\n\nThis addresses CVE-2024-21626 in our testing suite. Note that our\nbinaries do not depend on runc, and thus are unaffected.\n\nSigned-off-by: David Son <davbson@amazon.com>",
          "timestamp": "2024-03-04T11:25:22-08:00",
          "tree_id": "ca222f98a34c20558e258320bbd84fe178756ae1",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/bb526c30097754683e91abb7836252b1bc3663f4"
        },
        "date": 1709580962516,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 9.649000000000001,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.72,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.6645,
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
            "email": "55555210+sondavidb@users.noreply.github.com",
            "name": "David Son",
            "username": "sondavidb"
          },
          "distinct": true,
          "id": "1f3e47e7d4b741aa86b15099fe4cc15df91455f1",
          "message": "Bump dependencies using scripts/bump-deps.sh\n\nSigned-off-by: github-actions[bot] <41898282+github-actions[bot]@users.noreply.github.com>",
          "timestamp": "2024-03-05T10:24:05-08:00",
          "tree_id": "b0dc0873b1dd45ded729ae59756ee32b3a8a5b44",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/1f3e47e7d4b741aa86b15099fe4cc15df91455f1"
        },
        "date": 1709663473396,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 8.858,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.523,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 0.671,
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
          "id": "1d4d288d008d5f9b67f7179e25e2b6f22e8d1365",
          "message": "Update zot image tag to v2.0.1\n\nThe v2.0.0-rc6 tag got removed, so using the latest stable version\nas of the time of this commit.\n\nSigned-off-by: David Son <davbson@amazon.com>",
          "timestamp": "2024-03-07T11:02:45-08:00",
          "tree_id": "f0d00c483ca3a8d05c9f116856f02f010cdd3ebf",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/1d4d288d008d5f9b67f7179e25e2b6f22e8d1365"
        },
        "date": 1709838772156,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 14.8995,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 8.1005,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.7505,
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
            "email": "55906459+austinvazquez@users.noreply.github.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "distinct": true,
          "id": "2ed2689b48ac9f49d17bbf8aa0bb9260fd185090",
          "message": "Bump softprops/action-gh-release from 1 to 2\n\nBumps [softprops/action-gh-release](https://github.com/softprops/action-gh-release) from 1 to 2.\n- [Release notes](https://github.com/softprops/action-gh-release/releases)\n- [Changelog](https://github.com/softprops/action-gh-release/blob/master/CHANGELOG.md)\n- [Commits](https://github.com/softprops/action-gh-release/compare/v1...v2)\n\n---\nupdated-dependencies:\n- dependency-name: softprops/action-gh-release\n  dependency-type: direct:production\n  update-type: version-update:semver-major\n...\n\nSigned-off-by: dependabot[bot] <support@github.com>",
          "timestamp": "2024-03-14T07:58:25-07:00",
          "tree_id": "62bf71d96cbcc32a786cf3492dc58177e83d2195",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/2ed2689b48ac9f49d17bbf8aa0bb9260fd185090"
        },
        "date": 1710428990529,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 14.754,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.576499999999999,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.8315,
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
            "email": "55906459+austinvazquez@users.noreply.github.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "distinct": true,
          "id": "27d6b4064e76871646fe021258556de436b669ae",
          "message": "Update Golang to v1.21.7\n\nDockerfile + workflows — 1.20.13 -> 1.21.7\ngo.mod — 1.20 -> 1.21\ngolangci-lint — 1.53.3 -> 1.56.2\n\nSigned-off-by: David Son <davbson@amazon.com>",
          "timestamp": "2024-03-15T10:42:37-07:00",
          "tree_id": "4e2567a4e86cd5fe989120713488e14e88401ddc",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/27d6b4064e76871646fe021258556de436b669ae"
        },
        "date": 1710525044895,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 8.734,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.4295,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 0.7865,
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
            "email": "55906459+austinvazquez@users.noreply.github.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "distinct": true,
          "id": "0da1238f9ca1aaa021ae3de092f4cebba793b3a4",
          "message": "Bump dependencies using scripts/bump-deps.sh\n\nSigned-off-by: Austin Vazquez <macedonv@amazon.com>",
          "timestamp": "2024-03-15T11:06:39-07:00",
          "tree_id": "f8153603f5e3d38bac36e2045bbd61e6de623f10",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/0da1238f9ca1aaa021ae3de092f4cebba793b3a4"
        },
        "date": 1710526510710,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 10.3995,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.428,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.6615,
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
            "email": "55906459+austinvazquez@users.noreply.github.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "distinct": true,
          "id": "7af8577ac627f991d561f94d476ea4529a02626a",
          "message": "Bump google.golang.org/protobuf from 1.32.0 to 1.33.0\n\nBumps google.golang.org/protobuf from 1.32.0 to 1.33.0.\n\n---\nupdated-dependencies:\n- dependency-name: google.golang.org/protobuf\n  dependency-type: indirect\n...\n\nSigned-off-by: Austin Vazquez <macedonv@amazon.com>",
          "timestamp": "2024-03-15T11:43:40-07:00",
          "tree_id": "27e8c2fffa72d90e7badc3246e1b6fc8b2547045",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/7af8577ac627f991d561f94d476ea4529a02626a"
        },
        "date": 1710528658173,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 8.497499999999999,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.4665,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.0725,
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
            "email": "55906459+austinvazquez@users.noreply.github.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "distinct": true,
          "id": "f3eb99da462a140d0b4a581791bfefcc21eec0df",
          "message": "Bump github.com/docker/cli\n\nBumps [github.com/docker/cli](https://github.com/docker/cli) from 25.0.4+incompatible to 25.0.5+incompatible.\n- [Commits](https://github.com/docker/cli/compare/v25.0.4...v25.0.5)\n\n---\nupdated-dependencies:\n- dependency-name: github.com/docker/cli\n  dependency-type: direct:production\n  update-type: version-update:semver-patch\n...\n\nSigned-off-by: Austin Vazquez <macedonv@amazon.com>",
          "timestamp": "2024-03-20T18:17:31-07:00",
          "tree_id": "006b0f56f20e0bb39b9541a5da4fad9db6e6b279",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/f3eb99da462a140d0b4a581791bfefcc21eec0df"
        },
        "date": 1710984372766,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 11.157,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.541499999999999,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.454,
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
            "email": "55906459+austinvazquez@users.noreply.github.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "distinct": true,
          "id": "b881ab5d4f9c88b32226efe94d78a37ec6c95da2",
          "message": "Bump github.com/docker/docker\n\nBumps [github.com/docker/docker](https://github.com/docker/docker) from 24.0.7+incompatible to 25.0.5+incompatible.\n- [Release notes](https://github.com/docker/docker/releases)\n- [Commits](https://github.com/docker/docker/compare/v24.0.7...v25.0.5)\n\n---\nupdated-dependencies:\n- dependency-name: github.com/docker/docker\n  dependency-type: indirect\n...\n\nSigned-off-by: Austin Vazquez <macedonv@amazon.com>",
          "timestamp": "2024-03-21T08:53:59-07:00",
          "tree_id": "bc9f83f6c45ea6b35372c19ff03d2bd17588bf3b",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/b881ab5d4f9c88b32226efe94d78a37ec6c95da2"
        },
        "date": 1711036903912,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 9.169,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 8.1465,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.2445,
            "unit": "Seconds",
            "extra": "P90"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "macedonv@amazon.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "committer": {
            "email": "55906459+austinvazquez@users.noreply.github.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "distinct": true,
          "id": "fa59739309c7f823196aa54427742071736860cd",
          "message": "Add make tidy and make vendor targets\n\nThis change reworks the vendor make target to vendor dependencies for\nusers looking for stronger build reproducibility. This change adds a new\ntidy make target for the previous behavior to install dependencies in\nthe local Go module cache and ensures go.mod file matches the source\ncode in the module.\n\nSigned-off-by: Austin Vazquez <macedonv@amazon.com>",
          "timestamp": "2024-03-21T10:56:16-07:00",
          "tree_id": "8692edf3bb8f29c79640492dc1568a7096b0a05a",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/fa59739309c7f823196aa54427742071736860cd"
        },
        "date": 1711044225113,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 8.9565,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.34,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.536,
            "unit": "Seconds",
            "extra": "P90"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "macedonv@amazon.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "committer": {
            "email": "55906459+austinvazquez@users.noreply.github.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "distinct": true,
          "id": "d85499605b926c3bb94b125e76715d745387546a",
          "message": "Fix max concurrent uploads on SOCI push\n\nThis change fixes the SOCI push command to set the maximum number of\nconcurrent copy tasks to the value passed via the CLI flag. Before this\nchange, the concurrency limit would ignore the value set via CLI and\ndefault to 3 copy tasks.\n\nSigned-off-by: Austin Vazquez <macedonv@amazon.com>",
          "timestamp": "2024-03-21T11:01:23-07:00",
          "tree_id": "7f3eb3701e32effdfc4aada2cbfc559a592c0d8c",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/d85499605b926c3bb94b125e76715d745387546a"
        },
        "date": 1711044524580,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 8.3615,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.2684999999999995,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.4715,
            "unit": "Seconds",
            "extra": "P90"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "macedonv@amazon.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "committer": {
            "email": "55906459+austinvazquez@users.noreply.github.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "distinct": true,
          "id": "c9d49c352b6fce9fd7b364bee7c3025cdb31d168",
          "message": "Update go.work example for Go 1.21\n\nSigned-off-by: Austin Vazquez <macedonv@amazon.com>",
          "timestamp": "2024-03-21T11:12:21-07:00",
          "tree_id": "9fc6b83957ef422d45155c507310f9d9ae83316f",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/c9d49c352b6fce9fd7b364bee7c3025cdb31d168"
        },
        "date": 1711045163260,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 8.587,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.32,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 0.846,
            "unit": "Seconds",
            "extra": "P90"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "arjunry@amazon.com",
            "name": "Arjun Raja Yogidas",
            "username": "coderbirju"
          },
          "committer": {
            "email": "arjunry@amazon.com",
            "name": "Arjun",
            "username": "coderbirju"
          },
          "distinct": true,
          "id": "f0a1fe4c4dae7a940c7932fdaf86ee03c5a66395",
          "message": "Update go version in toolchain\n\nSigned-off-by: Arjun Raja <arjunry@amazon.com>",
          "timestamp": "2024-03-21T13:06:56-07:00",
          "tree_id": "b0bafdfd4df2535864b5c3a051d6f4579a92dfdb",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/f0a1fe4c4dae7a940c7932fdaf86ee03c5a66395"
        },
        "date": 1711052037402,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 8.834499999999998,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.77,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 0.6285000000000001,
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
          "id": "715ac46f5741ac246435b31556aca7f53177e70a",
          "message": "Fix xattr optimization\n\nThis change updates the xattr optimizaion by:\n1) Properly handling opaque directories\n2) Changing the CLI flag to `--optimizations xattr`\n3) Changing the label to `disable-xattrs`\n\nSigned-off-by: Kern Walster <walster@amazon.com>",
          "timestamp": "2024-03-22T12:06:48-07:00",
          "tree_id": "c4d79f627ad2b72a0189771bde246f6b5aa04b20",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/715ac46f5741ac246435b31556aca7f53177e70a"
        },
        "date": 1711134804054,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 8.979,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.165,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 0.9039999999999999,
            "unit": "Seconds",
            "extra": "P90"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "macedonv@amazon.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "committer": {
            "email": "55906459+austinvazquez@users.noreply.github.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "distinct": true,
          "id": "9d6a4202b4831daa193ccd08ae125ba216b09780",
          "message": "Add shellcheck to lint\n\nSigned-off-by: Austin Vazquez <macedonv@amazon.com>",
          "timestamp": "2024-03-25T15:10:56-07:00",
          "tree_id": "251be8188e5ba5ee274b7279eefcf25c49ba8b34",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/9d6a4202b4831daa193ccd08ae125ba216b09780"
        },
        "date": 1711405137432,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 10.5325,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.404,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 0.7165,
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
            "email": "55906459+austinvazquez@users.noreply.github.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "distinct": true,
          "id": "1997f8731ed7c4ab068c2dbb9f7eec62b3ef4584",
          "message": "Fix scripts/check-dco.sh CI failure\n\nSigned-off-by: David Son <davbson@amazon.com>",
          "timestamp": "2024-03-26T08:46:43-07:00",
          "tree_id": "9583df4662cb0b06923db3a9692a62a2cf92d1f5",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/1997f8731ed7c4ab068c2dbb9f7eec62b3ef4584"
        },
        "date": 1711468454596,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 10.033000000000001,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.243,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.6975,
            "unit": "Seconds",
            "extra": "P90"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "macedonv@amazon.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "committer": {
            "email": "55906459+austinvazquez@users.noreply.github.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "distinct": true,
          "id": "eb5189203bfaf5e30b8f11e406aa1f5200ae536b",
          "message": "Revert changes to bump-deps script and ignore SC2046\n\nThis change reverts the changes to quote the output of go list in\nbump-deps script to prevent word splitting. This has caused a regression\nin the dependency update automation.\n\nSigned-off-by: Austin Vazquez <macedonv@amazon.com>",
          "timestamp": "2024-03-26T09:52:54-07:00",
          "tree_id": "d436ad45e83860e126d3f7a043098ae713002625",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/eb5189203bfaf5e30b8f11e406aa1f5200ae536b"
        },
        "date": 1711472631844,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 15.199,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.2545,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.7915,
            "unit": "Seconds",
            "extra": "P90"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "macedonv@amazon.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "committer": {
            "email": "55906459+austinvazquez@users.noreply.github.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "distinct": true,
          "id": "7c8c4584838784e4f93b6e7d0145246097e181b5",
          "message": "Remove release directory on make clean\n\nSigned-off-by: Austin Vazquez <macedonv@amazon.com>",
          "timestamp": "2024-03-26T10:33:18-07:00",
          "tree_id": "a8b1ab186b474f5f222bcae965c4129509585d5e",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/7c8c4584838784e4f93b6e7d0145246097e181b5"
        },
        "date": 1711474777475,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 8.419,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.434,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 0.744,
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
            "email": "55555210+sondavidb@users.noreply.github.com",
            "name": "David Son",
            "username": "sondavidb"
          },
          "distinct": true,
          "id": "a9f14c218b3ce1813b932ffcff928c69d6381342",
          "message": "Bump dependencies using scripts/bump-deps.sh\n\nSigned-off-by: github-actions[bot] <41898282+github-actions[bot]@users.noreply.github.com>",
          "timestamp": "2024-03-26T10:34:23-07:00",
          "tree_id": "14b797d28c8fe8b69b5d573d7d81a48867edf2e5",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/a9f14c218b3ce1813b932ffcff928c69d6381342"
        },
        "date": 1711475067349,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 12.522,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.9945,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.6364999999999998,
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
            "email": "wh_henry@hotmail.com",
            "name": "Henry Wang",
            "username": "henry118"
          },
          "distinct": true,
          "id": "059e9e86b69e1c545504d4a89c6a0de3c732c51e",
          "message": "Allow make clean to be called from anywhere\n\nSigned-off-by: David Son <davbson@amazon.com>",
          "timestamp": "2024-03-27T10:02:40-07:00",
          "tree_id": "1478bc1064835067109e718df40d2823a16f5d4d",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/059e9e86b69e1c545504d4a89c6a0de3c732c51e"
        },
        "date": 1711559342423,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 8.5725,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.2595,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 0.7785,
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
            "email": "wh_henry@hotmail.com",
            "name": "Henry Wang",
            "username": "henry118"
          },
          "distinct": true,
          "id": "41bbfbb384eaad104bb8b7393d0b8e2cf5e60dc1",
          "message": "Add debug logging to metadata DB initialization\n\nSigned-off-by: David Son <davbson@amazon.com>",
          "timestamp": "2024-03-27T10:03:22-07:00",
          "tree_id": "6189f24d7ce6a0b6b80c80df461fdcf72f85f4ba",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/41bbfbb384eaad104bb8b7393d0b8e2cf5e60dc1"
        },
        "date": 1711559504675,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 9.526,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.162,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.759,
            "unit": "Seconds",
            "extra": "P90"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "arjunry@amazon.com",
            "name": "Arjun Raja Yogidas",
            "username": "coderbirju"
          },
          "committer": {
            "email": "wh_henry@hotmail.com",
            "name": "Henry Wang",
            "username": "henry118"
          },
          "distinct": true,
          "id": "f78090a12c74ec4a98c9172d519a8811dbf8d97c",
          "message": "Update containerd version in build.yml\n\nSigned-off-by: Arjun Raja <arjunry@amazon.com>",
          "timestamp": "2024-03-27T12:14:52-07:00",
          "tree_id": "3560b08a303f2a8042018ef622e1d09ac7591182",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/f78090a12c74ec4a98c9172d519a8811dbf8d97c"
        },
        "date": 1711567481892,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 14.7835,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.3505,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.6444999999999999,
            "unit": "Seconds",
            "extra": "P90"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "macedonv@amazon.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "committer": {
            "email": "wh_henry@hotmail.com",
            "name": "Henry Wang",
            "username": "henry118"
          },
          "distinct": true,
          "id": "e8ffbc8f7f93e3313f30af2650ff9ba251c666ba",
          "message": "Update registry test dependency to registry:3.0.0-alpha.1\n\nThis change updates the test registry version to v3.0.0-alpha.1 for\nregistry CVE updates.\n\nSigned-off-by: Austin Vazquez <macedonv@amazon.com>",
          "timestamp": "2024-03-27T15:44:43-07:00",
          "tree_id": "688fa07f8e7e7da10eb202e3d755ad0eb6842c63",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/e8ffbc8f7f93e3313f30af2650ff9ba251c666ba"
        },
        "date": 1711579931499,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 9.801,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.318,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.4705,
            "unit": "Seconds",
            "extra": "P90"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "macedonv@amazon.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "committer": {
            "email": "55555210+sondavidb@users.noreply.github.com",
            "name": "David Son",
            "username": "sondavidb"
          },
          "distinct": true,
          "id": "373be7f0fa8cb9ba631c55a45fb8c61eacbdecc8",
          "message": "Run shellcheck from container in CI\n\nThis change runs shellcheck from container in CI instead of installing\nthe binary to the system and running from the lint bash script.\n\nSigned-off-by: Austin Vazquez <macedonv@amazon.com>",
          "timestamp": "2024-03-27T15:52:20-07:00",
          "tree_id": "745cfd2435e374186ae326719449c18e8d0c4f65",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/373be7f0fa8cb9ba631c55a45fb8c61eacbdecc8"
        },
        "date": 1711580554219,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 8.989,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.513999999999999,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.4115,
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
            "email": "55906459+austinvazquez@users.noreply.github.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "distinct": true,
          "id": "a85ea6c84d5d0cf369a707f3800eaffeb64fa8f9",
          "message": "Fine-tune shell scripts\n\nAdded some modularity in our installation scripts by moving versions\ninto their own variable. Also added an integrity check for cmake, as\nwell as TODO comments for the other scripts.\n\nSigned-off-by: David Son <davbson@amazon.com>",
          "timestamp": "2024-03-28T16:25:57-07:00",
          "tree_id": "74832e057642560ffcdc7bdf5ef78b324e543ae2",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/a85ea6c84d5d0cf369a707f3800eaffeb64fa8f9"
        },
        "date": 1711668842690,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 13.8635,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 8.04,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.375,
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
            "email": "wh_henry@hotmail.com",
            "name": "Henry Wang",
            "username": "henry118"
          },
          "distinct": true,
          "id": "cc90b9b2f645972aef36230323366d108ee06dd0",
          "message": "Move golangci-lint to GH Actions\n\nSigned-off-by: David Son <davbson@amazon.com>",
          "timestamp": "2024-03-28T21:49:28-07:00",
          "tree_id": "2bc14fd50356c6a67e009b092bdd30e260ca9033",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/cc90b9b2f645972aef36230323366d108ee06dd0"
        },
        "date": 1711688230695,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 8.2045,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.209,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.241,
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
            "email": "55906459+austinvazquez@users.noreply.github.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "distinct": true,
          "id": "2a70d12d833f8e93f62dea30d400bac1e2d7810d",
          "message": "Disable xattrs by default\n\nChange --optimizations xattr to be default behavior, and add a new flag\nto disable this annotation when creating a SOCI index.\n\nThis change also eliminated the need for the optimizations structure in\nthe CLI.\n\nSigned-off-by: David Son <davbson@amazon.com>",
          "timestamp": "2024-03-30T08:04:50-07:00",
          "tree_id": "d37fbdcd7d94d32f6aecd11f9503f65173093511",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/2a70d12d833f8e93f62dea30d400bac1e2d7810d"
        },
        "date": 1711811478349,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 8.469999999999999,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.397,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 0.895,
            "unit": "Seconds",
            "extra": "P90"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "macedonv@amazon.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "committer": {
            "email": "55906459+austinvazquez@users.noreply.github.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "distinct": true,
          "id": "1e42eeb6f8e1ddcc9888b86c74a5601d2517d473",
          "message": "Add git secrets scanning to CI\n\nThis change adds a GitHub Actions job to validate git secrets are not\nsubmitted to version control.\n\nSigned-off-by: Austin Vazquez <macedonv@amazon.com>",
          "timestamp": "2024-04-01T09:57:08-07:00",
          "tree_id": "25fb2506384f170ab73131b8930693280d54803d",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/1e42eeb6f8e1ddcc9888b86c74a5601d2517d473"
        },
        "date": 1711991106528,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 16.9285,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.240500000000001,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.8159999999999998,
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
            "email": "55555210+sondavidb@users.noreply.github.com",
            "name": "David Son",
            "username": "sondavidb"
          },
          "distinct": true,
          "id": "4d6b50fc8aae3bee70551c77dd457ae7779087cf",
          "message": "Bump dependencies using scripts/bump-deps.sh\n\nSigned-off-by: github-actions[bot] <41898282+github-actions[bot]@users.noreply.github.com>",
          "timestamp": "2024-04-02T10:17:55-07:00",
          "tree_id": "2b938e2e13c8e5a2c052ab5dbeda92294734080f",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/4d6b50fc8aae3bee70551c77dd457ae7779087cf"
        },
        "date": 1712078884326,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 10.823,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.5355,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 0.9730000000000001,
            "unit": "Seconds",
            "extra": "P90"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "macedonv@amazon.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "committer": {
            "email": "55906459+austinvazquez@users.noreply.github.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "distinct": true,
          "id": "d0e78441b1a03e88fe5d74fd875c8df5968a0b68",
          "message": "Pull container images from ECR Public instead of Docker Hub\n\nSigned-off-by: Austin Vazquez <macedonv@amazon.com>",
          "timestamp": "2024-04-02T11:33:06-07:00",
          "tree_id": "58f7791573dcd0457e1df33050bdc490f42370c3",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/d0e78441b1a03e88fe5d74fd875c8df5968a0b68"
        },
        "date": 1712083298947,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 10.5555,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.752000000000001,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.7454999999999998,
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
          "id": "ab85d3dee2d5471b864e1b0a54ed4a187db2f22f",
          "message": "Check connection only when image isn't fully cached\n\nTaken from stargz:\nhttps://github.com/containerd/stargz-snapshotter/pull/1584/files\n\nSigned-off-by: David Son <davbson@amazon.com>",
          "timestamp": "2024-04-03T11:23:32-07:00",
          "tree_id": "62f928928bddce2ec80c9a4229561d7de3d61a1d",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/ab85d3dee2d5471b864e1b0a54ed4a187db2f22f"
        },
        "date": 1712169063616,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 9.554,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.172499999999999,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 0.9285,
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
          "id": "a04f8abec2ea6ebcbde26eccf02ec9ad8065f023",
          "message": "Hardcode cmake.sh expected shasum\n\nSigned-off-by: David Son <davbson@amazon.com>",
          "timestamp": "2024-04-03T11:26:19-07:00",
          "tree_id": "b498629b2f5c4a2df7be946a12035bc897196c3a",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/a04f8abec2ea6ebcbde26eccf02ec9ad8065f023"
        },
        "date": 1712169218023,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 9.339,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 8.214,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 0.8069999999999999,
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
            "email": "arjunry@amazon.com",
            "name": "Arjun",
            "username": "coderbirju"
          },
          "distinct": true,
          "id": "4547e50a7c9961d0cf1c2d534a85d1a7cd7efdc3",
          "message": "Add prebuild workflow to release branches\n\nSigned-off-by: David Son <davbson@amazon.com>",
          "timestamp": "2024-04-03T14:46:10-07:00",
          "tree_id": "e1ba618e874be0ba09baf06cfaf8f1fda196c160",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/4547e50a7c9961d0cf1c2d534a85d1a7cd7efdc3"
        },
        "date": 1712181279805,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 13.633,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.3495,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.7545000000000002,
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
          "id": "dbdef0840b1d5fa609273829f438361d5a5d9f70",
          "message": "Address yamllint findings\n\nSigned-off-by: David Son <davbson@amazon.com>",
          "timestamp": "2024-04-09T10:19:41-07:00",
          "tree_id": "8bbd5cc118003574234ff8da2e180a67f5dedb9a",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/dbdef0840b1d5fa609273829f438361d5a5d9f70"
        },
        "date": 1712683719150,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 12.9605,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.2455,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.051,
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
            "email": "55906459+austinvazquez@users.noreply.github.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "distinct": true,
          "id": "cf3f3d9874b22a3b9efd0c777bb56f6914ceef96",
          "message": "Bump dependencies using scripts/bump-deps.sh\n\nSigned-off-by: github-actions[bot] <41898282+github-actions[bot]@users.noreply.github.com>",
          "timestamp": "2024-04-09T10:21:35-07:00",
          "tree_id": "e44acdbaae6e2509c279c065889253edb565bb33",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/cf3f3d9874b22a3b9efd0c777bb56f6914ceef96"
        },
        "date": 1712683840463,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 9.2655,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.414,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.5434999999999999,
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
          "id": "afec192851a0783ed504d49f06997aa57c7d01c4",
          "message": "Check first header in each layer\n\nIn #1147, we cherry-picked a commit from stargz where we don't make\nregistry calls if layers are fully pulled. This works fine for stargz,\nbut for SOCI, we skip reading the first header for each layer, as we\ndon't need it. This caused a bug where our fetched size never matched\nour expected size, so this condition was never met.\n\nThis commit fixes this by reading the initially skipped header. This\ncommit also checks this header to ensure that it is not malformed for\nany reason.\n\nSigned-off-by: David Son <davbson@amazon.com>",
          "timestamp": "2024-04-11T15:29:53-07:00",
          "tree_id": "cf0bbd527eb9906bbf8fc3692e94f198c09180c8",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/afec192851a0783ed504d49f06997aa57c7d01c4"
        },
        "date": 1712875106682,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 9.3595,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.6305,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.6475,
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
            "email": "arjunry@amazon.com",
            "name": "Arjun",
            "username": "coderbirju"
          },
          "distinct": true,
          "id": "779edee7a038c13875945f4c6db56c575d0c0266",
          "message": "Disable TestNetworkRetry on ARM machines\n\nThis test currently hangs on ARM machines, so disabling till we can fix\nthis.\n\nSigned-off-by: David Son <davbson@amazon.com>",
          "timestamp": "2024-04-11T15:33:37-07:00",
          "tree_id": "5bea95a4da1476f3ef79c504292734e370aff90c",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/779edee7a038c13875945f4c6db56c575d0c0266"
        },
        "date": 1712875338419,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 10.105,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 8.173,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.7315,
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
            "email": "55555210+sondavidb@users.noreply.github.com",
            "name": "David Son",
            "username": "sondavidb"
          },
          "distinct": true,
          "id": "39548da712e7eda5be77a6c879457b7b68572f45",
          "message": "Bump dependencies using scripts/bump-deps.sh\n\nSigned-off-by: github-actions[bot] <41898282+github-actions[bot]@users.noreply.github.com>",
          "timestamp": "2024-04-16T15:04:48-07:00",
          "tree_id": "979976bc7130e37127b1b02c8632e361b4e9e08b",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/39548da712e7eda5be77a6c879457b7b68572f45"
        },
        "date": 1713305582829,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 12.7335,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.4195,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.1535,
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
          "id": "8a3b3617a0dff68c7728b99369e458e360d48c53",
          "message": "Fix binary download directory in release workflow\n\nSigned-off-by: David Son <davbson@amazon.com>",
          "timestamp": "2024-04-16T15:40:18-07:00",
          "tree_id": "21856b2491d6aa882134c9063eb7b4ca8edcd54c",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/8a3b3617a0dff68c7728b99369e458e360d48c53"
        },
        "date": 1713307730511,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 8.7905,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.2545,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 0.9435,
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
          "id": "dfcd5e5fd22bf0e78fb6b666b522613964a3f933",
          "message": "Add release workflow testing\n\nSigned-off-by: David Son <davbson@amazon.com>",
          "timestamp": "2024-04-22T16:21:42-07:00",
          "tree_id": "0f02d28390e2daba3e7d8eca6000583f8154af0c",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/dfcd5e5fd22bf0e78fb6b666b522613964a3f933"
        },
        "date": 1713828529409,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 10.9865,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.7620000000000005,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 0.835,
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
            "email": "55906459+austinvazquez@users.noreply.github.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "distinct": true,
          "id": "711afec16b4e3fe2eda8738978f3ea93bae83806",
          "message": "Bump dependencies using scripts/bump-deps.sh\n\nSigned-off-by: github-actions[bot] <41898282+github-actions[bot]@users.noreply.github.com>",
          "timestamp": "2024-04-24T16:35:10-07:00",
          "tree_id": "30e2441478f7ebd91f8ec0934b28acd38ddcf204",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/711afec16b4e3fe2eda8738978f3ea93bae83806"
        },
        "date": 1714002186762,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 12.113,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.379,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 0.904,
            "unit": "Seconds",
            "extra": "P90"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "haddscot@amazon.com",
            "name": "Scott Haddlesey",
            "username": "haddscot"
          },
          "committer": {
            "email": "160976906+haddscot@users.noreply.github.com",
            "name": "haddscot",
            "username": "haddscot"
          },
          "distinct": true,
          "id": "a9952c0206f197e6479cea8722de7785b9f7b8d3",
          "message": "remove duplicate logging on integ tests, add info for where log came from\n\nSigned-off-by: Scott Haddlesey <haddscot@amazon.com>",
          "timestamp": "2024-04-26T10:53:07-07:00",
          "tree_id": "fd68ec48b398cd3fcce8bf5d42cced1e80fe05ad",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/a9952c0206f197e6479cea8722de7785b9f7b8d3"
        },
        "date": 1714154499318,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 9.658999999999999,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.2455,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.653,
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
            "email": "55555210+sondavidb@users.noreply.github.com",
            "name": "David Son",
            "username": "sondavidb"
          },
          "distinct": true,
          "id": "94bccb2c9554c5feb39890a9da3733db31bd091a",
          "message": "Bump golangci/golangci-lint-action from 4 to 5\n\nBumps [golangci/golangci-lint-action](https://github.com/golangci/golangci-lint-action) from 4 to 5.\n- [Release notes](https://github.com/golangci/golangci-lint-action/releases)\n- [Commits](https://github.com/golangci/golangci-lint-action/compare/v4...v5)\n\n---\nupdated-dependencies:\n- dependency-name: golangci/golangci-lint-action\n  dependency-type: direct:production\n  update-type: version-update:semver-major\n...\n\nSigned-off-by: dependabot[bot] <support@github.com>",
          "timestamp": "2024-04-26T13:57:56-07:00",
          "tree_id": "ea3bb442e57b4405e4985114cf2a80d755c8666c",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/94bccb2c9554c5feb39890a9da3733db31bd091a"
        },
        "date": 1714165551288,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 9.303999999999998,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.484500000000001,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 0.873,
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
            "email": "55906459+austinvazquez@users.noreply.github.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "distinct": true,
          "id": "01840577363d68f36060bf7079fea055a35dcc6c",
          "message": "Bump dependencies using scripts/bump-deps.sh\n\nSigned-off-by: github-actions[bot] <41898282+github-actions[bot]@users.noreply.github.com>",
          "timestamp": "2024-04-30T15:24:40-07:00",
          "tree_id": "3316b094723f3cd70fa4d13fe960ccb46bcbd0c4",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/01840577363d68f36060bf7079fea055a35dcc6c"
        },
        "date": 1714516410016,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 10.2945,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.08,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.7665,
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
            "email": "kern.walster@gmail.com",
            "name": "Kern Walster",
            "username": "Kern--"
          },
          "distinct": true,
          "id": "3fd12f4337230326e8df20a3c759e1e6c11c8f18",
          "message": "Fix scripts/install-dep.sh cmake shasum for ARM64\n\nSigned-off-by: David Son <davbson@amazon.com>",
          "timestamp": "2024-05-01T16:15:53-07:00",
          "tree_id": "4eb5459d4c775dbed1737cf76906b750c851d6a3",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/3fd12f4337230326e8df20a3c759e1e6c11c8f18"
        },
        "date": 1714605817521,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 9.030000000000001,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.324999999999999,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 0.92,
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
          "id": "bf4c69d854d8e10be4a555658010277139d6ff12",
          "message": "Use public ECR zot image\n\nPer Amazon best security practices, we switched to using a version of\nproject zot hosted on public ECR instead of ghcr.io.\n\nSigned-off-by: David Son <davbson@amazon.com>",
          "timestamp": "2024-05-01T22:56:23-07:00",
          "tree_id": "6cb04fd54212aad678777ed085074bd04cd86181",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/bf4c69d854d8e10be4a555658010277139d6ff12"
        },
        "date": 1714629858527,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 11.11,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.4879999999999995,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 0.9610000000000001,
            "unit": "Seconds",
            "extra": "P90"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "macedonv@amazon.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "committer": {
            "email": "55555210+sondavidb@users.noreply.github.com",
            "name": "David Son",
            "username": "sondavidb"
          },
          "distinct": true,
          "id": "2cb31a4f8277a31a29d4bb3c441231647a2b5177",
          "message": "Add timeouts to testing in CI\n\nSigned-off-by: Austin Vazquez <macedonv@amazon.com>",
          "timestamp": "2024-05-06T15:44:11-04:00",
          "tree_id": "5a4913faa4197550211a17c39274852418e41297",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/2cb31a4f8277a31a29d4bb3c441231647a2b5177"
        },
        "date": 1715025210175,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 16.2315,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.1905,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.218,
            "unit": "Seconds",
            "extra": "P90"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "haddscot@amazon.com",
            "name": "haddscot",
            "username": "haddscot"
          },
          "committer": {
            "email": "160976906+haddscot@users.noreply.github.com",
            "name": "haddscot",
            "username": "haddscot"
          },
          "distinct": true,
          "id": "21b8effe32e05e174e685a24773334c1e19a7b0d",
          "message": "Remove benchmarker CSV input in favor of JSON #946\n\nSigned-off-by: Scott Haddlesey <haddscot@amazon.com>",
          "timestamp": "2024-05-07T10:32:33-07:00",
          "tree_id": "c6c0a6567c8150bdd82e4c9780fd87e6e55ef69a",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/21b8effe32e05e174e685a24773334c1e19a7b0d"
        },
        "date": 1715103675569,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 10.95,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.3435,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.5805,
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
            "email": "55555210+sondavidb@users.noreply.github.com",
            "name": "David Son",
            "username": "sondavidb"
          },
          "distinct": true,
          "id": "69583117e3bde0833c0d949d38911e3455ee7072",
          "message": "Bump dependencies using scripts/bump-deps.sh\n\nSigned-off-by: github-actions[bot] <41898282+github-actions[bot]@users.noreply.github.com>",
          "timestamp": "2024-05-07T11:33:35-07:00",
          "tree_id": "0ce0be9d55853b77a6668dff8f0f4e2441769a27",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/69583117e3bde0833c0d949d38911e3455ee7072"
        },
        "date": 1715107274548,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 8.6035,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.2645,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 0.8779999999999999,
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
            "email": "55555210+sondavidb@users.noreply.github.com",
            "name": "David Son",
            "username": "sondavidb"
          },
          "distinct": true,
          "id": "941a2072aaaddc44087ae9b6ba857a5acb4cb0d1",
          "message": "Bump golangci/golangci-lint-action from 5 to 6\n\nBumps [golangci/golangci-lint-action](https://github.com/golangci/golangci-lint-action) from 5 to 6.\n- [Release notes](https://github.com/golangci/golangci-lint-action/releases)\n- [Commits](https://github.com/golangci/golangci-lint-action/compare/v5...v6)\n\n---\nupdated-dependencies:\n- dependency-name: golangci/golangci-lint-action\n  dependency-type: direct:production\n  update-type: version-update:semver-major\n...\n\nSigned-off-by: dependabot[bot] <support@github.com>",
          "timestamp": "2024-05-08T07:29:30-07:00",
          "tree_id": "aae317f3df714e4e0c9a987d7c79d08151666089",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/941a2072aaaddc44087ae9b6ba857a5acb4cb0d1"
        },
        "date": 1715179112619,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 12.1355,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.4045000000000005,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.669,
            "unit": "Seconds",
            "extra": "P90"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "macedonv@amazon.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "committer": {
            "email": "55906459+austinvazquez@users.noreply.github.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "distinct": true,
          "id": "f2e945397fe6bf9f1f1440dd66c3ca9a6d1d27c5",
          "message": "Update to Go 1.21.10 in CI\n\nSigned-off-by: Austin Vazquez <macedonv@amazon.com>",
          "timestamp": "2024-05-08T08:03:41-07:00",
          "tree_id": "3ffc261330e72110219cbe6139d3721dbbd172dc",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/f2e945397fe6bf9f1f1440dd66c3ca9a6d1d27c5"
        },
        "date": 1715181169682,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 10.353,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.2485,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.7760000000000002,
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
            "email": "55555210+sondavidb@users.noreply.github.com",
            "name": "David Son",
            "username": "sondavidb"
          },
          "distinct": true,
          "id": "3a5819f9fe8f54d842d53f59eef68bb07689d4f3",
          "message": "Fix network retry integration test\n\nSigned-off-by: Yasin Turan <turyasin@amazon.com>\nSigned-off-by: David Son <davbson@amazon.com>",
          "timestamp": "2024-05-09T14:45:51-07:00",
          "tree_id": "657dbe3975d3d76207604288dc937c82f7322766",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/3a5819f9fe8f54d842d53f59eef68bb07689d4f3"
        },
        "date": 1715291707839,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 14.04,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.561,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.8014999999999999,
            "unit": "Seconds",
            "extra": "P90"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "macedonv@amazon.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "committer": {
            "email": "55555210+sondavidb@users.noreply.github.com",
            "name": "David Son",
            "username": "sondavidb"
          },
          "distinct": true,
          "id": "c098211421153be89d153688210c69cfb9aaf08f",
          "message": "Change containerd-snapshotter-base to alpine based\n\nThis was done to use a smaller base image which makes us less prone to\nsecurity issues.\n\nAdditionally, this commit switches to using raw image URLs instead of\ninserting in the version via a variable, so that dependabot can track\nnew versions.\n\nThe Dockerfile line that pulls the registry  was moved up to allow\nproper tagging when building locally instead of with Docker Compose.\n\nSigned-off-by: Austin Vazquez <macedonv@amazon.com>",
          "timestamp": "2024-05-10T15:08:59-07:00",
          "tree_id": "194608fc3eb730d98fba6678ce5162e3758723fa",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/c098211421153be89d153688210c69cfb9aaf08f"
        },
        "date": 1715379613526,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 11.374,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.4305,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.6715,
            "unit": "Seconds",
            "extra": "P90"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "macedonv@amazon.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "committer": {
            "email": "55906459+austinvazquez@users.noreply.github.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "distinct": true,
          "id": "3e86314f49f7f00289cec39f43ea0668c281a7f8",
          "message": "Fix digest formatting in soci index remove error\n\nSigned-off-by: Austin Vazquez <macedonv@amazon.com>",
          "timestamp": "2024-05-10T20:50:53-07:00",
          "tree_id": "20f025754f01a6dd581a26494bed3eb00ecad15d",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/3e86314f49f7f00289cec39f43ea0668c281a7f8"
        },
        "date": 1715400003251,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 9.5475,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.232,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 2.7965,
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
            "email": "55906459+austinvazquez@users.noreply.github.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "distinct": true,
          "id": "7a5056d16a5b1ad0845ba5ef58c77ef4108b2f60",
          "message": "Add manual arch option in artifact verification\n\nSigned-off-by: David Son <davbson@amazon.com>",
          "timestamp": "2024-05-11T04:41:29-07:00",
          "tree_id": "1abdad6cf4206ccc3e44d5ac73edc00aeca4ba9d",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/7a5056d16a5b1ad0845ba5ef58c77ef4108b2f60"
        },
        "date": 1715428136135,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 8.915,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.327,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 0.8765000000000001,
            "unit": "Seconds",
            "extra": "P90"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "steven.davidovitz@dominodatalab.com",
            "name": "Steven Davidovitz",
            "username": "steved"
          },
          "committer": {
            "email": "55906459+austinvazquez@users.noreply.github.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "distinct": true,
          "id": "071a9111ec94afedd38f4deac7c960b803710059",
          "message": "Trigger Docker auth on ECR token expiry\n\nAn expired token passed to ECR will return a 403, which lacks a\nWww-Authenticate header required to trigger the docker authorizer. This\nmeant that credential helpers like amazon-ecr-credential-helper would\nnot refresh the token. This change adds the proper header to fix this\nbehavior.\n\nSigned-off-by: David Son <davbson@amazon.com>",
          "timestamp": "2024-05-11T04:42:39-07:00",
          "tree_id": "5084428f0af5e11612e6aaacf4d58e3bb790124b",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/071a9111ec94afedd38f4deac7c960b803710059"
        },
        "date": 1715428283022,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 8.89,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.186,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 2.7675,
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
          "id": "292499059e1bc84bef2fa227b7c56361752cb7cf",
          "message": "Regenerate changed flatbuffer definitions\n\nSigned-off-by: David Son <davbson@amazon.com>",
          "timestamp": "2024-05-13T09:49:29-07:00",
          "tree_id": "955bfde5fbd2a27cea3d41d2cb196fbc40242545",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/292499059e1bc84bef2fa227b7c56361752cb7cf"
        },
        "date": 1715619405304,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 8.2595,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.317,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 0.7575000000000001,
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
          "id": "1142aec4a8b18a763e2e76284cfaa48a36012fc3",
          "message": "Add testing suite cleanup to Makefile\n\nSigned-off-by: David Son <davbson@amazon.com>",
          "timestamp": "2024-05-13T10:29:54-07:00",
          "tree_id": "a90a804d63109c6a585bb2b873d51a8c3a5cc1d9",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/1142aec4a8b18a763e2e76284cfaa48a36012fc3"
        },
        "date": 1715621989752,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 9.585,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.763,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.5525,
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
            "email": "55906459+austinvazquez@users.noreply.github.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "distinct": true,
          "id": "d25a2ea1b3565f7220e0bc33bb40a4d1fed80484",
          "message": "Bump dependencies using scripts/bump-deps.sh\n\nSigned-off-by: github-actions[bot] <41898282+github-actions[bot]@users.noreply.github.com>",
          "timestamp": "2024-05-14T09:21:38-07:00",
          "tree_id": "f4b2e93335536150a8bc8f7c9ec5b15ad5ec9e9f",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/d25a2ea1b3565f7220e0bc33bb40a4d1fed80484"
        },
        "date": 1715704123602,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 8.681000000000001,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.2075,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 0.7709999999999999,
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
            "email": "55555210+sondavidb@users.noreply.github.com",
            "name": "David Son",
            "username": "sondavidb"
          },
          "distinct": true,
          "id": "0fc1d822ecca3bf0ae09f1c0b794410f1e7eebbc",
          "message": "Bump dependencies using scripts/bump-deps.sh\n\nSigned-off-by: github-actions[bot] <41898282+github-actions[bot]@users.noreply.github.com>",
          "timestamp": "2024-05-22T11:32:29-07:00",
          "tree_id": "a79d502e57f41ad326499446597f864cf0ab3b68",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/0fc1d822ecca3bf0ae09f1c0b794410f1e7eebbc"
        },
        "date": 1716403268888,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 9.859,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.3405000000000005,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.666,
            "unit": "Seconds",
            "extra": "P90"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "macedonv@amazon.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "committer": {
            "email": "55906459+austinvazquez@users.noreply.github.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "distinct": true,
          "id": "d0a67cf8230d2b9a4ffd136a770f200de6057aae",
          "message": "Add dependabot configuration to ignore Go minor updates\n\nThis change adds dependabot configuration to ignore Go major and minor\nupdates. This is to keep the container image aligned with the version\nused by the release pipeline. A major update should not occur.\n\nSigned-off-by: Austin Vazquez <macedonv@amazon.com>",
          "timestamp": "2024-05-23T07:43:10-07:00",
          "tree_id": "92d6967acaec2093eed70bfbdf112667bd1d09fc",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/d0a67cf8230d2b9a4ffd136a770f200de6057aae"
        },
        "date": 1716475798349,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 8.3845,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.2844999999999995,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 0.669,
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
            "email": "55555210+sondavidb@users.noreply.github.com",
            "name": "David Son",
            "username": "sondavidb"
          },
          "distinct": true,
          "id": "81b170fdf88c19efa3c35c431abd3cfd511c6903",
          "message": "Bump docker/library/golang from 1.21.9-alpine to 1.21.10-alpine\n\nBumps docker/library/golang from 1.21.9-alpine to 1.21.10-alpine.\n\n---\nupdated-dependencies:\n- dependency-name: docker/library/golang\n  dependency-type: direct:production\n  update-type: version-update:semver-patch\n...\n\nSigned-off-by: dependabot[bot] <support@github.com>",
          "timestamp": "2024-05-23T12:59:49-07:00",
          "tree_id": "b6767dc2f1661eb0fbc004fb61b6cd99f2f5c7b0",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/81b170fdf88c19efa3c35c431abd3cfd511c6903"
        },
        "date": 1716494939499,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 13.098500000000001,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.5175,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.069,
            "unit": "Seconds",
            "extra": "P90"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "macedonv@amazon.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "committer": {
            "email": "55906459+austinvazquez@users.noreply.github.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "distinct": true,
          "id": "310a88e7a7d9ef8fcc6f8c835edb2edb71a87642",
          "message": "Add workflow to update getting started guide\n\nThis change adds an automated workflow to open a pull request to update the\nversion in the getting started guide when a release is made.\n\nSigned-off-by: Austin Vazquez <macedonv@amazon.com>",
          "timestamp": "2024-05-31T10:44:25-07:00",
          "tree_id": "efa4a8b4b08318e0a9c1163f145b8dcf9d959567",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/310a88e7a7d9ef8fcc6f8c835edb2edb71a87642"
        },
        "date": 1717177976779,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 8.74,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.2075,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.717,
            "unit": "Seconds",
            "extra": "P90"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "arjunry@amazon.com",
            "name": "Arjun Raja Yogidas",
            "username": "coderbirju"
          },
          "committer": {
            "email": "arjunry@amazon.com",
            "name": "Arjun",
            "username": "coderbirju"
          },
          "distinct": true,
          "id": "98f2fb42d88484398bdd125a8d20f0468ec1088b",
          "message": "fix fuse test issues\n\nSigned-off-by: Arjun Raja Yogidas <arjunry@amazon.com>",
          "timestamp": "2024-06-03T09:46:21-07:00",
          "tree_id": "6f15ffd5c51460d47dd7392e6d46da399996dd4f",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/98f2fb42d88484398bdd125a8d20f0468ec1088b"
        },
        "date": 1717433690796,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 13.592500000000001,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.593,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.52,
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
          "id": "f7858b69111e1f68e8d94e0f178c7a548744d3e9",
          "message": "Enforce lease when building SOCI index\n\nPreviously, in our CLI create command, we would create zTOCs, push them\nto the content store, then label the index to refer to the zTOCs.\nThis created a problem where a user using the containerd content store\nmight have their zTOCs deleted by the containerd garbage collector\nbefore it could be labeled. While a lease was used, it was only done\nduring the latter step, so it did not prevent zTOCs from being deleted\nbefore being labeled.\n\nThis commit fixes this by using a lease on the entire process via a new\ncommand, BuildAndWriteIndex, which is called by \"soci create\".\n\nSigned-off-by: David Son <davbson@amazon.com>",
          "timestamp": "2024-06-03T10:20:46-07:00",
          "tree_id": "136b9859783114dea8ef1bc95a56341b442447c7",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/f7858b69111e1f68e8d94e0f178c7a548744d3e9"
        },
        "date": 1717435686282,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 8.1675,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.2225,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.213,
            "unit": "Seconds",
            "extra": "P90"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "sbora@amazon.com",
            "name": "Swagat Bora",
            "username": "swagatbora90"
          },
          "committer": {
            "email": "swagatbora90@gmail.com",
            "name": "Swagat Bora",
            "username": "swagatbora90"
          },
          "distinct": true,
          "id": "c538952debec5e42451b5accf04dc77ac60a7558",
          "message": "Update Go version to 1.21.11 in CI\n\nSigned-off-by: Swagat Bora <sbora@amazon.com>",
          "timestamp": "2024-06-06T12:22:31-07:00",
          "tree_id": "ed9d4cd916a88d0186a88301f963f8a42e6eb3e8",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/c538952debec5e42451b5accf04dc77ac60a7558"
        },
        "date": 1717702347172,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 14.4875,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 8.0785,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.5145,
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
            "email": "swagatbora90@gmail.com",
            "name": "Swagat Bora",
            "username": "swagatbora90"
          },
          "distinct": true,
          "id": "95695e23c08cf33b64bc150e0281ac3fe36b9d92",
          "message": "Bump dependencies using scripts/bump-deps.sh\n\nSigned-off-by: github-actions[bot] <41898282+github-actions[bot]@users.noreply.github.com>",
          "timestamp": "2024-06-06T12:22:50-07:00",
          "tree_id": "6a552dc884a1eb51f3fb3483fa55094b908f6d45",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/95695e23c08cf33b64bc150e0281ac3fe36b9d92"
        },
        "date": 1717702387545,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 13.2135,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.409,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.6844999999999999,
            "unit": "Seconds",
            "extra": "P90"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "macedonv@amazon.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "committer": {
            "email": "kern.walster@gmail.com",
            "name": "Kern Walster",
            "username": "Kern--"
          },
          "distinct": true,
          "id": "99fc201a71114eafcc9b0c8a126a968d319b1138",
          "message": "Update SOCI rebuild DB CLI help text and doc\n\nRemoves rpull command reference from help text which support has been\nremoved. Aligns help text in CLI and documentation to match.\n\nSigned-off-by: Austin Vazquez <macedonv@amazon.com>",
          "timestamp": "2024-06-06T14:33:12-07:00",
          "tree_id": "dc0af7b575d7c4750408be1136b6510929d84ff9",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/99fc201a71114eafcc9b0c8a126a968d319b1138"
        },
        "date": 1717710092928,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 8.5345,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.539,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.375,
            "unit": "Seconds",
            "extra": "P90"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "macedonv@amazon.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "committer": {
            "email": "kern.walster@gmail.com",
            "name": "Kern Walster",
            "username": "Kern--"
          },
          "distinct": true,
          "id": "184d564b9003d0fca62b6e8cc331bb5451a06bc3",
          "message": "Add workflow to label pull requests\n\nThis change adds a GitHub Actions workflow to label PR's based on their\nchanges. The following labels will be applied to pull requests:\n\n- `documentation` - updates to `**/*.md`\n- `go` - updates to `**/*.go`\n- `testing` - updates to `**/*_test.go`\n- `github_actions` - updates to `.github/`\n- `benchmarking` - updates to `benchmark/`\n- `dependencies` - updates to `go.(mod|sum)`\n\nSigned-off-by: Austin Vazquez <macedonv@amazon.com>",
          "timestamp": "2024-06-12T16:32:52-07:00",
          "tree_id": "f719eade37bc161b09ccdd52d683701e38a5901d",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/184d564b9003d0fca62b6e8cc331bb5451a06bc3"
        },
        "date": 1718235696924,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 11.056000000000001,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.4405,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.7055000000000002,
            "unit": "Seconds",
            "extra": "P90"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "macedonv@amazon.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "committer": {
            "email": "55906459+austinvazquez@users.noreply.github.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "distinct": true,
          "id": "1a00a81fed3422845e4773311c5bb0267bcbc86e",
          "message": "Fix bug in update getting started guide workflow\n\nThis change fixes a bug in the update getting started guide workflow\nwhen assertions are disabled. This change also fixes a bug where\n'--verbose' executed git diff command in interactive mode.\n\nSigned-off-by: Austin Vazquez <macedonv@amazon.com>",
          "timestamp": "2024-06-13T10:06:10-07:00",
          "tree_id": "5f773d856156000cdb485960a4f85d2c2a89b153",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/1a00a81fed3422845e4773311c5bb0267bcbc86e"
        },
        "date": 1718298904511,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 13.457999999999998,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.4505,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.2175,
            "unit": "Seconds",
            "extra": "P90"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "macedonv@amazon.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "committer": {
            "email": "55906459+austinvazquez@users.noreply.github.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "distinct": true,
          "id": "847f01a0ea0a43bc090e72daa3f887ef4c51394f",
          "message": "Fix label workflow configuration\n\nSigned-off-by: Austin Vazquez <macedonv@amazon.com>",
          "timestamp": "2024-06-14T11:13:07-07:00",
          "tree_id": "7a0e82f4fadb59fd71da78915205bac198da9565",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/847f01a0ea0a43bc090e72daa3f887ef4c51394f"
        },
        "date": 1718389235114,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 8.541,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.4655000000000005,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 0.705,
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
            "email": "55555210+sondavidb@users.noreply.github.com",
            "name": "David Son",
            "username": "sondavidb"
          },
          "distinct": true,
          "id": "e1ceff5a0bdf951a6fc80dcf9e7866e38dc1d96b",
          "message": "Bump dependencies using scripts/bump-deps.sh\n\nSigned-off-by: github-actions[bot] <41898282+github-actions[bot]@users.noreply.github.com>",
          "timestamp": "2024-06-18T12:47:45-07:00",
          "tree_id": "d49063f8cbf96642d4f7fa999ed05b5b8a92cdc9",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/e1ceff5a0bdf951a6fc80dcf9e7866e38dc1d96b"
        },
        "date": 1718740527096,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 10.5825,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.3395,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.428,
            "unit": "Seconds",
            "extra": "P90"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "shubhum@amazon.com",
            "name": "Shubharanshu Mahapatra",
            "username": "Shubhranshu153"
          },
          "committer": {
            "email": "55555210+sondavidb@users.noreply.github.com",
            "name": "David Son",
            "username": "sondavidb"
          },
          "distinct": true,
          "id": "07296f9741ff540f226125dfa795270c2f9900a7",
          "message": "Fix daemon startup latency issue by providing an option to skip snapshotter supported flag\n\nSigned-off-by: Shubharanshu Mahapatra <shubhum@amazon.com>",
          "timestamp": "2024-06-19T15:48:43-07:00",
          "tree_id": "3b0ccddec373fb6aec21c3e932067bbfe0bb32a9",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/07296f9741ff540f226125dfa795270c2f9900a7"
        },
        "date": 1718837744971,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 8.911999999999999,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.257,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.2525,
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
            "email": "swagatbora90@gmail.com",
            "name": "Swagat Bora",
            "username": "swagatbora90"
          },
          "distinct": true,
          "id": "145a4382bad97921bb72b08a8376d6276b58afbc",
          "message": "Auto-update packages in Dockerfile\n\nSigned-off-by: David Son <davbson@amazon.com>",
          "timestamp": "2024-06-21T08:56:25-07:00",
          "tree_id": "53fd21ddcb40740f3af3c8d794af8a8e7170981d",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/145a4382bad97921bb72b08a8376d6276b58afbc"
        },
        "date": 1718985860516,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 10.695,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.5445,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.2734999999999999,
            "unit": "Seconds",
            "extra": "P90"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "shubhum@amazon.com",
            "name": "Shubharanshu Mahapatra",
            "username": "Shubhranshu153"
          },
          "committer": {
            "email": "47723536+Shubhranshu153@users.noreply.github.com",
            "name": "Subhransu",
            "username": "Shubhranshu153"
          },
          "distinct": true,
          "id": "a857d2b83d3c86e70392ae658e918816c747d5d8",
          "message": "Bump version of containerd to 1.7.18 for latest and 1.6.33 for LTS\n\nSigned-off-by: Shubharanshu Mahapatra <shubhum@amazon.com>",
          "timestamp": "2024-06-24T09:18:25-07:00",
          "tree_id": "a2d8d2413c0a74defbfb63af7c7eec8e8f416f20",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/a857d2b83d3c86e70392ae658e918816c747d5d8"
        },
        "date": 1719246386012,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 12.967500000000001,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.4405,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.5615,
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
            "email": "55906459+austinvazquez@users.noreply.github.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "distinct": true,
          "id": "a6719c59c314db23612386b28bfa8115f140ffad",
          "message": "Bump dependencies using scripts/bump-deps.sh\n\nSigned-off-by: github-actions[bot] <41898282+github-actions[bot]@users.noreply.github.com>",
          "timestamp": "2024-06-26T20:31:18-07:00",
          "tree_id": "7c8cc38c7a59993c0d3a294d8988563526d96a73",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/a6719c59c314db23612386b28bfa8115f140ffad"
        },
        "date": 1719459657031,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 13.529499999999999,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.336,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.4655,
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
            "email": "55555210+sondavidb@users.noreply.github.com",
            "name": "David Son",
            "username": "sondavidb"
          },
          "distinct": true,
          "id": "3510a9663bc0b18c72bcf6692fc1233776648330",
          "message": "Bump dependencies using scripts/bump-deps.sh\n\nSigned-off-by: github-actions[bot] <41898282+github-actions[bot]@users.noreply.github.com>",
          "timestamp": "2024-07-02T13:42:06-07:00",
          "tree_id": "8d220b202fa652cb9437d9789f9623cf88ce431c",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/3510a9663bc0b18c72bcf6692fc1233776648330"
        },
        "date": 1719953486184,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 12.5195,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.4835,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.193,
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
          "id": "0e1e73bac251896a7658a03913771cb73b8997b4",
          "message": "Remove ioutils.CountWriter\n\nWe had 2 implementations to keep track of how long an io.Reader is:\nioutils.CountWriter and ioutils.PositionTrackerReader.\n\nioutils.CountWriter implements io.Writer, but it just discards the bytes\nand keeps track of the number of bytes written. The caller connects it\nto an existing io.Reader by creating an io.TeeReader which writes to it\nwhenever the original reader is read.\n\nioutils.PositionTrackerReader is a simpler interface that wraps an\nexisting io.Reader and proxies Read requests while updating it's\ninternal position.\n\nThis change removes CountWriter in favor of the simpler and better\ntested PositionTrackerReader.\n\nSigned-off-by: Kern Walster <walster@amazon.com>",
          "timestamp": "2024-07-02T15:32:27-07:00",
          "tree_id": "eb7c01ca1df7bf5b18b45e2aa826444089ec43ef",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/0e1e73bac251896a7658a03913771cb73b8997b4"
        },
        "date": 1719960010675,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 10.2355,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.709,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.472,
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
          "id": "2f82461d214d2bc30843af32c52b7883b304db60",
          "message": "Allow insecure localhost connections\n\nBefore this change, the `docker.MatchLocalhost` function was applied to\nhosts retrieved from labels on snapshots, but not in the artifact\nfetcher. This meant that data could be lazily loaded from an insecure\nlocalhost, but we couldn't fetch the SOCI index/ztocs from an insecure\nlocalhost. This change adds the matcher to the artifact fetcher so that\nimages can be lazily loaded from an insecure localhost.\n\nSigned-off-by: Kern Walster <walster@amazon.com>",
          "timestamp": "2024-07-02T15:32:40-07:00",
          "tree_id": "7907d519c02959f738f4bfb449a3249cc69e6075",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/2f82461d214d2bc30843af32c52b7883b304db60"
        },
        "date": 1719960232054,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 14.867,
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
            "value": 1.8525,
            "unit": "Seconds",
            "extra": "P90"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "macedonv@amazon.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "committer": {
            "email": "kern.walster@gmail.com",
            "name": "Kern Walster",
            "username": "Kern--"
          },
          "distinct": true,
          "id": "9aaf67e8cca2fd24e91dffea3b8e1b9c09842c84",
          "message": "Update to Go 1.21.12 in CI\n\nSigned-off-by: Austin Vazquez <macedonv@amazon.com>",
          "timestamp": "2024-07-05T09:40:29-07:00",
          "tree_id": "b653a4fb59d11ffb1cb2729873b1b7d65c32b9c7",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/9aaf67e8cca2fd24e91dffea3b8e1b9c09842c84"
        },
        "date": 1720198171382,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 11.027000000000001,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.327999999999999,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.0775000000000001,
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
          "id": "0d06c8814527d128499f08422e892e4ca0c83b24",
          "message": "Update deprecated packages\n\ngithub.com/containerd/containerd/platforms ->\ngithub.com/containerd/platforms\n\ngithub.com/containerd/containerd/reference/docker ->\ngithub.com/distribution/reference\n\nSigned-off-by: David Son <davbson@amazon.com>",
          "timestamp": "2024-07-09T13:32:51-07:00",
          "tree_id": "70f6ffe8aefa1cbe89d4f6ca651875a0632ea819",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/0d06c8814527d128499f08422e892e4ca0c83b24"
        },
        "date": 1720557669171,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 12.026,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 8.037,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.0575,
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
            "email": "55555210+sondavidb@users.noreply.github.com",
            "name": "David Son",
            "username": "sondavidb"
          },
          "distinct": true,
          "id": "0399e9a07784ea35e3daa1b457b32e8c840afe7f",
          "message": "Bump dependencies using scripts/bump-deps.sh\n\nSigned-off-by: github-actions[bot] <41898282+github-actions[bot]@users.noreply.github.com>",
          "timestamp": "2024-07-09T14:21:18-07:00",
          "tree_id": "4365c03fb5eb05b6e128339e61f88ee2800d4300",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/0399e9a07784ea35e3daa1b457b32e8c840afe7f"
        },
        "date": 1720560682641,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 14.469999999999999,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.8345,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.4705,
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
            "email": "55906459+austinvazquez@users.noreply.github.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "distinct": true,
          "id": "702560159c04ec049c5e120d02f641ae5f9586bc",
          "message": "Test against containerd 2.0 in CI\n\nSigned-off-by: Kern Walster <walster@amazon.com>",
          "timestamp": "2024-07-09T19:40:08-07:00",
          "tree_id": "da394ac0e4950aa9b7dc1de3efdf85a7278ff26c",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/702560159c04ec049c5e120d02f641ae5f9586bc"
        },
        "date": 1720579749124,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 13.2995,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 7.394500000000001,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.5445,
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
            "email": "arjunry@amazon.com",
            "name": "Arjun",
            "username": "coderbirju"
          },
          "distinct": true,
          "id": "9eef2c0ff55f7e52d9b2f47364dce56eaef63e9e",
          "message": "Use rabbitmqImage in FUSE failure test\n\nOn ARM instances, pinnedRabbitmqImage does not work for this test due to\nsome issues with xattr/whiteout dir detection. This is either a bug in\nthe image or our code, but to get around CI for now we will just use a\nregular rabbitmq image.\n\nSigned-off-by: David Son <davbson@amazon.com>",
          "timestamp": "2024-07-16T08:54:51-07:00",
          "tree_id": "17760429481a83641d9808691159a22144684d68",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/9eef2c0ff55f7e52d9b2f47364dce56eaef63e9e"
        },
        "date": 1721145865804,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 15.810500000000001,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 8.567,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.733,
            "unit": "Seconds",
            "extra": "P90"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "arjunry@amazon.com",
            "name": "Arjun Raja Yogidas",
            "username": "coderbirju"
          },
          "committer": {
            "email": "arjunry@amazon.com",
            "name": "Arjun",
            "username": "coderbirju"
          },
          "distinct": true,
          "id": "436e476c69c767b0a1f265e8944477d2a2d9c00b",
          "message": "add codebuild integration to build workflow\n\nSigned-off-by: Arjun Raja Yogidas <arjunry@amazon.com>",
          "timestamp": "2024-07-16T12:03:18-07:00",
          "tree_id": "ace3ed6080666d6eaef94c0ad96008e1c6bd971f",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/436e476c69c767b0a1f265e8944477d2a2d9c00b"
        },
        "date": 1721157125376,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 10.0835,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 8.848500000000001,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.274,
            "unit": "Seconds",
            "extra": "P90"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "macedonv@amazon.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "committer": {
            "email": "arjunry@amazon.com",
            "name": "Arjun",
            "username": "coderbirju"
          },
          "distinct": true,
          "id": "64f0e37a75b63cac1e163442b21c772de3706687",
          "message": "Add back containerd 2.0 testing to CI\n\nSigned-off-by: Austin Vazquez <macedonv@amazon.com>",
          "timestamp": "2024-07-17T08:58:22-07:00",
          "tree_id": "38d338f4d2c3f4a61684b59c91acd04bbfc3096e",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/64f0e37a75b63cac1e163442b21c772de3706687"
        },
        "date": 1721232463193,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 11.01,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 9.065999999999999,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.293,
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
          "id": "a641e0de43b1e5c2939fc36f9c8727933d2da8de",
          "message": "Revert \"Disable xattrs by default\"\n\nThis reverts commit 2a70d12d833f8e93f62dea30d400bac1e2d7810d and some\nextra reverts related to it.\n\nSigned-off-by: David Son <davbson@amazon.com>",
          "timestamp": "2024-07-17T14:52:37-07:00",
          "tree_id": "f158afe3c3e171afc1f9d4f85e3b74d23386dc16",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/a641e0de43b1e5c2939fc36f9c8727933d2da8de"
        },
        "date": 1721253610649,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 10.2595,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 9.0915,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 0.8240000000000001,
            "unit": "Seconds",
            "extra": "P90"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "alvajus@amazon.com",
            "name": "Justin Alvarez",
            "username": "pendo324"
          },
          "committer": {
            "email": "pendo324@users.noreply.github.com",
            "name": "Justin",
            "username": "pendo324"
          },
          "distinct": true,
          "id": "914f9a03a6757210daba3add12b20b0d6c145142",
          "message": "fix errdefs deprecation warning\n\nSigned-off-by: Justin Alvarez <alvajus@amazon.com>",
          "timestamp": "2024-07-18T14:27:49-04:00",
          "tree_id": "35d732a0e79fc2caabee730cc47f16cbe42c2b6d",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/914f9a03a6757210daba3add12b20b0d6c145142"
        },
        "date": 1721327842127,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 12.8095,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 9.222,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.2675,
            "unit": "Seconds",
            "extra": "P90"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "alvajus@amazon.com",
            "name": "Justin Alvarez",
            "username": "pendo324"
          },
          "committer": {
            "email": "pendo324@users.noreply.github.com",
            "name": "Justin",
            "username": "pendo324"
          },
          "distinct": true,
          "id": "327fbed1a5376493080ec758e9eb83a0390225f8",
          "message": "Bump containerd 1.6 and 1.7 versions to latest\n\nSigned-off-by: Justin Alvarez <alvajus@amazon.com>",
          "timestamp": "2024-07-18T16:23:57-04:00",
          "tree_id": "b8b3ec207c858cb2394c78381825dce1370636d1",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/327fbed1a5376493080ec758e9eb83a0390225f8"
        },
        "date": 1721334780019,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 11.838999999999999,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 8.760000000000002,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.4135,
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
          "id": "9b19a99205c05056a5e2e8785c8a3822f57e53e0",
          "message": "Add integration test for offline content\n\nSigned-off-by: David Son <davbson@amazon.com>",
          "timestamp": "2024-07-19T14:59:52-07:00",
          "tree_id": "b48a2bdf7bf80d6d66cba4ae4dc2d2364c64e3b0",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/9b19a99205c05056a5e2e8785c8a3822f57e53e0"
        },
        "date": 1721426833380,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 9.899999999999999,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 8.5145,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 0.7065,
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
          "id": "846397c346c81fc03b8eab44236c0dfe7f22419b",
          "message": "Add integration test for no image index\n\nSigned-off-by: David Son <davbson@amazon.com>",
          "timestamp": "2024-07-19T14:59:41-07:00",
          "tree_id": "6541eaf5b9b1dee3e1b59f711d9e9b288aa55760",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/846397c346c81fc03b8eab44236c0dfe7f22419b"
        },
        "date": 1721426843383,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 9.785499999999999,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 8.725,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 0.6475,
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
            "email": "55906459+austinvazquez@users.noreply.github.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "distinct": true,
          "id": "237fc956b8366e49927c84fcfee9a2defbb8f53c",
          "message": "Bump dependencies using scripts/bump-deps.sh\n\nSigned-off-by: github-actions[bot] <41898282+github-actions[bot]@users.noreply.github.com>",
          "timestamp": "2024-07-30T22:37:12-07:00",
          "tree_id": "361b2e9618eb42c60ad1cbfadf1f7fab4bdbeb77",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/237fc956b8366e49927c84fcfee9a2defbb8f53c"
        },
        "date": 1722404772433,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 15.1675,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 8.6815,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.165,
            "unit": "Seconds",
            "extra": "P90"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "macedonv@amazon.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "committer": {
            "email": "55906459+austinvazquez@users.noreply.github.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "distinct": true,
          "id": "d0fc67eb4aaf7870afac4a607b053a718da8a9af",
          "message": "Add review dependencies workflow to CI\n\nThis change adds a new CI workflow for reviewing added Go dependencies.\nThis include license and vulnerability checks.\n\nSigned-off-by: Austin Vazquez <macedonv@amazon.com>",
          "timestamp": "2024-08-02T10:49:15-07:00",
          "tree_id": "9a2a2437d50a3648e6c52b6c78aa0adb5be0cc7e",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/d0fc67eb4aaf7870afac4a607b053a718da8a9af"
        },
        "date": 1722621567957,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 14.594000000000001,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 8.6905,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.146,
            "unit": "Seconds",
            "extra": "P90"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "alvajus@amazon.com",
            "name": "Justin Alvarez",
            "username": "pendo324"
          },
          "committer": {
            "email": "pendo324@users.noreply.github.com",
            "name": "Justin",
            "username": "pendo324"
          },
          "distinct": true,
          "id": "49fc2dedccbc4e6fd31b325f0a49e76353a3021c",
          "message": "fix architecture check in verify-release-artifacts\n\nSigned-off-by: Justin Alvarez <alvajus@amazon.com>",
          "timestamp": "2024-08-05T16:22:46-04:00",
          "tree_id": "deb5661c10e6a22421742b7bf55faedbf8ca8a15",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/49fc2dedccbc4e6fd31b325f0a49e76353a3021c"
        },
        "date": 1722890063313,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 14.84,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 8.847999999999999,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.6005000000000003,
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
          "id": "9a5d7266be448e071de8b7618c7a8256bf31aa39",
          "message": "Bump dependencies using scripts/bump-deps.sh\n\nSigned-off-by: github-actions[bot] <41898282+github-actions[bot]@users.noreply.github.com>",
          "timestamp": "2024-08-06T14:52:50Z",
          "tree_id": "4a8b6ff22113f89b68c7b0942614fb9986154308",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/9a5d7266be448e071de8b7618c7a8256bf31aa39"
        },
        "date": 1722956661100,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 15.795000000000002,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 8.8565,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.876,
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
            "email": "55906459+austinvazquez@users.noreply.github.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "distinct": true,
          "id": "bae6ba541385466e3fb0de2f3117ad2585258468",
          "message": "Bump docker/library/golang from 1.21.12-alpine to 1.21.13-alpine\n\nBumps docker/library/golang from 1.21.12-alpine to 1.21.13-alpine.\n\n---\nupdated-dependencies:\n- dependency-name: docker/library/golang\n  dependency-type: direct:production\n  update-type: version-update:semver-patch\n...\n\nSigned-off-by: dependabot[bot] <support@github.com>",
          "timestamp": "2024-08-08T07:25:43-07:00",
          "tree_id": "e40a508814fa04e5956cf8c5d1e080975e5cbfe0",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/bae6ba541385466e3fb0de2f3117ad2585258468"
        },
        "date": 1723127823373,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 16.130000000000003,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 8.6645,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 2.2365,
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
            "email": "55906459+austinvazquez@users.noreply.github.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "distinct": true,
          "id": "0a4347677028f58706272e72a03365f38c1f88c2",
          "message": "Bump dependencies using scripts/bump-deps.sh\n\nSigned-off-by: github-actions[bot] <41898282+github-actions[bot]@users.noreply.github.com>",
          "timestamp": "2024-08-13T20:01:17-07:00",
          "tree_id": "8bf170b68c767c78e2cb87481ec2a5aba4fb5728",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/0a4347677028f58706272e72a03365f38c1f88c2"
        },
        "date": 1723605078180,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 18.711,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 8.6715,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.7615,
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
            "email": "55555210+sondavidb@users.noreply.github.com",
            "name": "David Son",
            "username": "sondavidb"
          },
          "distinct": true,
          "id": "a0a267164ae3b8eb33b60eaf72258424c473d303",
          "message": "Bump dependencies using scripts/bump-deps.sh\n\nSigned-off-by: github-actions[bot] <41898282+github-actions[bot]@users.noreply.github.com>",
          "timestamp": "2024-08-21T11:52:04-07:00",
          "tree_id": "6c9dc70cf5468283f0768dc8852e7cbc5800f91c",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/a0a267164ae3b8eb33b60eaf72258424c473d303"
        },
        "date": 1724267004678,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 17.5355,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 9.0465,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.2685,
            "unit": "Seconds",
            "extra": "P90"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "macedonv@amazon.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "committer": {
            "email": "55906459+austinvazquez@users.noreply.github.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "distinct": true,
          "id": "e8636bcff9141fa387ce3c30c6b1ebb7cf6d6551",
          "message": "Update toolchain min version to Go 1.22\n\nSigned-off-by: Austin Vazquez <macedonv@amazon.com>",
          "timestamp": "2024-08-27T15:41:16-07:00",
          "tree_id": "af2e6f05feeae857e36fe4d2c3d608dade571c46",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/e8636bcff9141fa387ce3c30c6b1ebb7cf6d6551"
        },
        "date": 1724799007720,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 12.5135,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 8.842,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 0.8065,
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
            "email": "arjunry@amazon.com",
            "name": "Arjun",
            "username": "coderbirju"
          },
          "distinct": true,
          "id": "0aeabca1e8ac34a1e260d736eecb0a336159db3c",
          "message": "Bump dependencies using scripts/bump-deps.sh\n\nSigned-off-by: github-actions[bot] <41898282+github-actions[bot]@users.noreply.github.com>\nSigned-off-by: Austin Vazquez <macedonv@amazon.com>",
          "timestamp": "2024-08-28T08:58:41-07:00",
          "tree_id": "1fcfa6bdb72c468b816086ebd247cd00254431ed",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/0aeabca1e8ac34a1e260d736eecb0a336159db3c"
        },
        "date": 1724861324126,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 12.4445,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 9.156500000000001,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.776,
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
            "email": "55555210+sondavidb@users.noreply.github.com",
            "name": "David Son",
            "username": "sondavidb"
          },
          "distinct": true,
          "id": "3bc2ce5a10c1b143d39b9e557a5eb08f39eb3d73",
          "message": "Bump dependencies using scripts/bump-deps.sh\n\nSigned-off-by: github-actions[bot] <41898282+github-actions[bot]@users.noreply.github.com>",
          "timestamp": "2024-09-04T16:10:51-07:00",
          "tree_id": "e81b43f0b1d8922f52aec6c44bdf62db1c825cd3",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/3bc2ce5a10c1b143d39b9e557a5eb08f39eb3d73"
        },
        "date": 1725492112644,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 11.7925,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 9.004,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.3715,
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
            "email": "55906459+austinvazquez@users.noreply.github.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "distinct": true,
          "id": "d61fec4ad62ef060d3950d8e2af9a489767ec8bb",
          "message": "Bump peter-evans/create-pull-request from 6 to 7\n\nBumps [peter-evans/create-pull-request](https://github.com/peter-evans/create-pull-request) from 6 to 7.\n- [Release notes](https://github.com/peter-evans/create-pull-request/releases)\n- [Commits](https://github.com/peter-evans/create-pull-request/compare/v6...v7)\n\n---\nupdated-dependencies:\n- dependency-name: peter-evans/create-pull-request\n  dependency-type: direct:production\n  update-type: version-update:semver-major\n...\n\nSigned-off-by: dependabot[bot] <support@github.com>",
          "timestamp": "2024-09-04T19:23:13-07:00",
          "tree_id": "e045984da49d2ab3d989c8d0208d2b70b5e05833",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/d61fec4ad62ef060d3950d8e2af9a489767ec8bb"
        },
        "date": 1725503531368,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 10.29,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 8.889,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 0.7855,
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
            "email": "55906459+austinvazquez@users.noreply.github.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "distinct": true,
          "id": "51f88f689c9c3ca5e1ad208282dcfec1ca3d2085",
          "message": "Bump docker/library/golang from 1.22.6-alpine to 1.22.7-alpine\n\nBumps docker/library/golang from 1.22.6-alpine to 1.22.7-alpine.\n\n---\nupdated-dependencies:\n- dependency-name: docker/library/golang\n  dependency-type: direct:production\n  update-type: version-update:semver-patch\n...\n\nSigned-off-by: dependabot[bot] <support@github.com>",
          "timestamp": "2024-09-09T08:52:23-07:00",
          "tree_id": "e2ba8e27afc8d7a3326c41396d28fe8643ce76f9",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/51f88f689c9c3ca5e1ad208282dcfec1ca3d2085"
        },
        "date": 1725897693250,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 14.131,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 8.722000000000001,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.1055000000000001,
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
          "id": "871bec22b92b8f343d3546f76c76826fa526aab1",
          "message": "Add test coverage to workflows\n\nThis change adds code coverage to our workflows and the necessary\nMakefile targets to support this.\n\nSigned-off-by: David Son <davbson@amazon.com>",
          "timestamp": "2024-09-11T19:49:27-07:00",
          "tree_id": "e46d04ff2bba5287f537efe72aabaa078d9b6d50",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/871bec22b92b8f343d3546f76c76826fa526aab1"
        },
        "date": 1726109910659,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 14.424999999999999,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 8.896999999999998,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.8079999999999998,
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
            "email": "55555210+sondavidb@users.noreply.github.com",
            "name": "David Son",
            "username": "sondavidb"
          },
          "distinct": true,
          "id": "375a699e959947890aadf164e84a9d45b81c547b",
          "message": "Bump dependencies using scripts/bump-deps.sh\n\nSigned-off-by: github-actions[bot] <41898282+github-actions[bot]@users.noreply.github.com>",
          "timestamp": "2024-09-13T11:15:49-07:00",
          "tree_id": "88288fe8587afb59b1e3e328089050c57e55f599",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/375a699e959947890aadf164e84a9d45b81c547b"
        },
        "date": 1726251964327,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 17.245,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 8.786,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.8595,
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
            "email": "kern.walster@gmail.com",
            "name": "Kern Walster",
            "username": "Kern--"
          },
          "distinct": true,
          "id": "0a3b4b452b1a9536b05d29d8e87f15125c999b75",
          "message": "Remove obsolete \"version\" definition in tests\n\nPer warning messages, the \"version\" definition in Docker compose files\nis obsolete, so removing it.\n\nSigned-off-by: David Son <davbson@amazon.com>",
          "timestamp": "2024-10-01T14:41:00-07:00",
          "tree_id": "03d38b4ea519cf3738e54f35c540dc9acaf2b73e",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/0a3b4b452b1a9536b05d29d8e87f15125c999b75"
        },
        "date": 1727819408344,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 12.8855,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 8.6345,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.099,
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
            "email": "55555210+sondavidb@users.noreply.github.com",
            "name": "David Son",
            "username": "sondavidb"
          },
          "distinct": true,
          "id": "1c9035236e32e077f326249942153bdc7fe15767",
          "message": "Bump dependencies using scripts/bump-deps.sh\n\nSigned-off-by: github-actions[bot] <41898282+github-actions[bot]@users.noreply.github.com>",
          "timestamp": "2024-10-02T21:15:45-07:00",
          "tree_id": "cd00de0a5681b39ead3fa2e1327e755042d192da",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/1c9035236e32e077f326249942153bdc7fe15767"
        },
        "date": 1727929449066,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 13.5765,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 8.7935,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.1925,
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
          "id": "212fe220f061413eb9f1a86556057128b25f4cab",
          "message": "Use AL2023 for integ test container\n\nThis change moves our integ test container from Alpine to AL2023. The\nmotivation is to be able to test features of a bigger OS, such as\ntesting systemd socket activation.\n\nSigned-off-by: Kern Walster <walster@amazon.com>",
          "timestamp": "2024-10-04T13:20:50-07:00",
          "tree_id": "0965d1a8323185f82fc32c731d4ae10b8c28d302",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/212fe220f061413eb9f1a86556057128b25f4cab"
        },
        "date": 1728073838321,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 13.768500000000001,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 8.593499999999999,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.4929999999999999,
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
          "id": "97bb872478e308e22f93c01c7a77befa29a2dd6e",
          "message": "Move TestSnapshotterStartup to startup_test.go\n\nThis change consolidates snapshotter startup tests into a single file\ninstead of putting it in pull_test.go\n\nSigned-off-by: Kern Walster <walster@amazon.com>",
          "timestamp": "2024-10-07T12:43:29-07:00",
          "tree_id": "6c2508d1ccb170f4ec45e089ff7a79e1f12c0a5b",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/97bb872478e308e22f93c01c7a77befa29a2dd6e"
        },
        "date": 1728330750100,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 10.652000000000001,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 8.885,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 0.921,
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
            "email": "55906459+austinvazquez@users.noreply.github.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "distinct": true,
          "id": "cd3f1f4e70d191fb54a18c850dad9760ccd0a6ac",
          "message": "Bump dependencies using scripts/bump-deps.sh\n\nSigned-off-by: github-actions[bot] <41898282+github-actions[bot]@users.noreply.github.com>",
          "timestamp": "2024-10-14T15:36:23-07:00",
          "tree_id": "e909fccbefc3d99ac91c00b3cb14f1b4ae94f204",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/cd3f1f4e70d191fb54a18c850dad9760ccd0a6ac"
        },
        "date": 1728945932990,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 11.358,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 8.688500000000001,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.3535,
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
          "id": "eb67eda19ed156fea94286f1bf970c26c0172d1a",
          "message": "Remove VerifiableReader\n\nVerifiableReader was code that SOCI inherited from Stargz, but didn't\nuse. With estargz images, the TOC is embedded in the layer in the remote\nstore. When setting up a reader, you can optionally pass a digest of the\nTOC which the stargz-snaphostter will verify against the actual TOC in\nthe remote registry.\n\nSOCI stores TOCs in zTOCs which have integrity rooted in the SOCI index.\nTherefore, you get equivalent verification by specifing a SOCI index\ndigest when pulling an image. By the time we're setting up a reader,\nwe've already verified the ztocs and their layer mapping which means\nthere is nothing to verify here.\n\nInitially we initially just made Verify a no-op and bypassed it.\nThis change cleans up that unneeded code entirely.\n\nSigned-off-by: Kern Walster <walster@amazon.com>",
          "timestamp": "2024-10-18T10:51:54-07:00",
          "tree_id": "89fa3583353a3ddabd9e6f944d72354f4d21c889",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/eb67eda19ed156fea94286f1bf970c26c0172d1a"
        },
        "date": 1729274398789,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 10.8405,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 8.7745,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 0.974,
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
            "email": "55906459+austinvazquez@users.noreply.github.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "distinct": true,
          "id": "30217c2f91e7ad2451240654b03b9acbe37da9ed",
          "message": "Bump dependencies using scripts/bump-deps.sh\n\nSigned-off-by: github-actions[bot] <41898282+github-actions[bot]@users.noreply.github.com>",
          "timestamp": "2024-10-27T17:33:49-07:00",
          "tree_id": "1e0f9cd9d4e5b9bea7239374dde12fa5d7b2879f",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/30217c2f91e7ad2451240654b03b9acbe37da9ed"
        },
        "date": 1730076113423,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 14.917000000000002,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 8.7975,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.0205,
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
          "id": "cd0bd7c4c4baa26534ddab623b41225cb6eba53b",
          "message": "Update containerd version in workflows\n\nSigned-off-by: David Son <davbson@amazon.com>",
          "timestamp": "2024-10-31T10:42:54-07:00",
          "tree_id": "3c96434625107a7826b77f939b31d06c866875e3",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/cd0bd7c4c4baa26534ddab623b41225cb6eba53b"
        },
        "date": 1730397075030,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 11.222,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 8.840499999999999,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.026,
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
          "id": "c3c64a20d57b90567774a257221cffeaab19b332",
          "message": "Update to Go 1.22.8 in CI\n\nSigned-off-by: David Son <davbson@amazon.com>",
          "timestamp": "2024-10-31T10:43:35-07:00",
          "tree_id": "524399b7fc4b9be9f30264a1e9c1217d7beb1f3b",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/c3c64a20d57b90567774a257221cffeaab19b332"
        },
        "date": 1730397138984,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 10.6665,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 8.942499999999999,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 0.9189999999999999,
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
          "id": "f8309afec3c56f7b3e58d283730b2c95cf5032b8",
          "message": "Add support for idmapped layers\n\nThis commit adds ID mapping functionality in SOCI. ID mapping is enabled\nif the correct labels are passed through.\n\nTo avoid having containerd handle the ID mapping, we must declare in the\ncontainerd config file that the snapshotter supports ID mapping.\n\nNote that usage of this feature requires proxy plugins to have\ncapabilities, which is only supported in containerd v1.7.23 onwards.\n\nSigned-off-by: David Son <davbson@amazon.com>",
          "timestamp": "2024-11-04T08:22:35-08:00",
          "tree_id": "6fc5e3be05a54d830ef34ae684246f058b942a6f",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/f8309afec3c56f7b3e58d283730b2c95cf5032b8"
        },
        "date": 1730737907719,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 13.493,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 8.914000000000001,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.495,
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
          "id": "92910f8efc392b777c32bc9ea9832ab4e037e826",
          "message": "Add EKS docs to automated version updating\n\nSigned-off-by: David Son <davbson@amazon.com>",
          "timestamp": "2024-11-04T10:27:41-08:00",
          "tree_id": "5575e6ecacbdba54727485ad64ba70874c292381",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/92910f8efc392b777c32bc9ea9832ab4e037e826"
        },
        "date": 1730745522469,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 17.327,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 8.8845,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.5915,
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
          "id": "4aeb0c1235dd0c3bcdfd16e84266c4f0adabed7b",
          "message": "Update golangci-lint to 1.61.0\n\nSigned-off-by: David Son <davbson@amazon.com>",
          "timestamp": "2024-11-07T12:59:08-08:00",
          "tree_id": "b0898ddb54fdea0cf400ed70b20b8f8bd584f810",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/4aeb0c1235dd0c3bcdfd16e84266c4f0adabed7b"
        },
        "date": 1731013707832,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 10.745000000000001,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 9.263,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 0.8925,
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
          "id": "09b075f6ffb89e367ce08e1b88a3518c7fceeab5",
          "message": "Ignore benchmark dir for unit/integration tests\n\nSigned-off-by: David Son <davbson@amazon.com>",
          "timestamp": "2024-11-07T13:30:15-08:00",
          "tree_id": "b343b8524703a7a6ddb11bf9ac563f2bf16d48d6",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/09b075f6ffb89e367ce08e1b88a3518c7fceeab5"
        },
        "date": 1731015502313,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 12.336,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 8.6795,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.609,
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
            "email": "55906459+austinvazquez@users.noreply.github.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "distinct": true,
          "id": "dd67cb8d5651951240f0840a11f8fabe357a8303",
          "message": "Bump dependencies using scripts/bump-deps.sh\n\nSigned-off-by: github-actions[bot] <41898282+github-actions[bot]@users.noreply.github.com>",
          "timestamp": "2024-11-07T21:47:27-08:00",
          "tree_id": "6c5a18afe491e9184653f5f864c5abd65037f9ff",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/dd67cb8d5651951240f0840a11f8fabe357a8303"
        },
        "date": 1731045395345,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 11.673,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 8.5975,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.7635,
            "unit": "Seconds",
            "extra": "P90"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "macedonv@amazon.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "committer": {
            "email": "55906459+austinvazquez@users.noreply.github.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "distinct": true,
          "id": "8fa766d430c709a5e4300a5e86f962775ea80415",
          "message": "Update runc v1.2.1\n\nSigned-off-by: Austin Vazquez <macedonv@amazon.com>",
          "timestamp": "2024-11-18T10:15:50-07:00",
          "tree_id": "b44d6ef9b440a624abb384e7abd94f8d767f42bb",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/8fa766d430c709a5e4300a5e86f962775ea80415"
        },
        "date": 1731950654385,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 11.298,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 8.9285,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.286,
            "unit": "Seconds",
            "extra": "P90"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "macedonv@amazon.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "committer": {
            "email": "55906459+austinvazquez@users.noreply.github.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "distinct": true,
          "id": "ffa83d2fdd050edc724c733739268b0189ff30a8",
          "message": "Update containerd v2.0.0 in test matrix\n\nSigned-off-by: Austin Vazquez <macedonv@amazon.com>",
          "timestamp": "2024-11-19T14:43:59-07:00",
          "tree_id": "671167d72a84990667046bb7e656527a2fa7578e",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/ffa83d2fdd050edc724c733739268b0189ff30a8"
        },
        "date": 1732053173096,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 9.924,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 8.9085,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 0.8185,
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
            "email": "55906459+austinvazquez@users.noreply.github.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "distinct": true,
          "id": "82e58929920881709051151536bb489541db9140",
          "message": "Bump dependencies using scripts/bump-deps.sh\n\nSigned-off-by: github-actions[bot] <41898282+github-actions[bot]@users.noreply.github.com>",
          "timestamp": "2024-11-21T09:08:52-08:00",
          "tree_id": "d1f26b217914f483bcc7f9c03b0cd4016966736c",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/82e58929920881709051151536bb489541db9140"
        },
        "date": 1732209508033,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 12.2425,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 9.135,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.4144999999999999,
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
            "email": "55906459+austinvazquez@users.noreply.github.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "distinct": true,
          "id": "388bb3b775cbf16eca62e348b599522a3c408da0",
          "message": "Bump dependencies using scripts/bump-deps.sh\n\nSigned-off-by: github-actions[bot] <41898282+github-actions[bot]@users.noreply.github.com>",
          "timestamp": "2024-11-27T21:23:57-08:00",
          "tree_id": "878cf84075693ec1b791a181eeb503d65c658a41",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/388bb3b775cbf16eca62e348b599522a3c408da0"
        },
        "date": 1732772029017,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 12.7395,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 8.7665,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.0150000000000001,
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
          "id": "3b10bf683e4ba09633c86c4256b3ce4da117382e",
          "message": "Truncate benchmark logs on re-run\n\nRerunning benchmarks did not truncate the benchmark log file before this\nchange. New line would overwrite old lines, but you had to scan\ntimestamps to figure out where exactly the new run was at. It also\nprevented `tail -f` to get real-time updates.\n\nThis change truncates the benchmark file on each execution.\n\nSigned-off-by: Kern Walster <walster@amazon.com>",
          "timestamp": "2024-12-10T09:40:07-08:00",
          "tree_id": "4ee23512f39676552a7d94e9aa8ab6c0d6983655",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/3b10bf683e4ba09633c86c4256b3ce4da117382e"
        },
        "date": 1733852975921,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 15.970500000000001,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 8.7425,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.605,
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
            "email": "55555210+sondavidb@users.noreply.github.com",
            "name": "David Son",
            "username": "sondavidb"
          },
          "distinct": true,
          "id": "0a554a183c99b2d77dec026f71d603eaaad0a9fa",
          "message": "Bump dependencies using scripts/bump-deps.sh\n\nSigned-off-by: github-actions[bot] <41898282+github-actions[bot]@users.noreply.github.com>",
          "timestamp": "2024-12-12T09:49:43-08:00",
          "tree_id": "6e479f1c4b371d82aef2298c1d6075ac3fd98fec",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/0a554a183c99b2d77dec026f71d603eaaad0a9fa"
        },
        "date": 1734026329432,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 14.5275,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 9.630500000000001,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.1825,
            "unit": "Seconds",
            "extra": "P90"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "arjunry@amazon.com",
            "name": "Arjun Raja Yogidas",
            "username": "coderbirju"
          },
          "committer": {
            "email": "arjunry@amazon.com",
            "name": "Arjun",
            "username": "coderbirju"
          },
          "distinct": true,
          "id": "81dececfa3e82e367b28eeda36ab997a4e4421bf",
          "message": "fix: Update go mod to fix CVE-2024-45338\n\nSigned-off-by: Arjun Raja Yogidas <arjunry@amazon.com>",
          "timestamp": "2024-12-18T19:27:10-08:00",
          "tree_id": "0ee105ef04e439051d024ec30e16d1ddce574e35",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/81dececfa3e82e367b28eeda36ab997a4e4421bf"
        },
        "date": 1734579401117,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 12.904,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 8.631499999999999,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.5335,
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
            "email": "arjunry@amazon.com",
            "name": "Arjun",
            "username": "coderbirju"
          },
          "distinct": true,
          "id": "ac157d85f29f5fa81b8cd9a44d018db6ccd8f849",
          "message": "Bump dependencies using scripts/bump-deps.sh\n\nSigned-off-by: github-actions[bot] <41898282+github-actions[bot]@users.noreply.github.com>",
          "timestamp": "2024-12-20T11:25:22-08:00",
          "tree_id": "e01fc213e6da5a98175d3246b90b3d7e3a5024af",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/ac157d85f29f5fa81b8cd9a44d018db6ccd8f849"
        },
        "date": 1734723225879,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 11.001000000000001,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 9.131499999999999,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.5514999999999999,
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
          "id": "aa306e4bf5657ce4a0aca4e0c64ff6828523d362",
          "message": "Create metrics socket dir\n\nBefore this change, if the metrics socket was a unix socket, SOCI would\nnot create the parent dir before trying to bind. This exposed an\nimplicit dependency on bind order if you tried to put the metrics\naddress next to the main socket which we broke when we introduced\nsystemd socket activation.\n\nThis change ensures that we create the parent dir before binding the\nmetrics socket.\n\nSigned-off-by: Kern Walster <walster@amazon.com>",
          "timestamp": "2024-12-20T13:48:59-08:00",
          "tree_id": "3f3fdf0c018de5d17b0da0cc1e4955a27c712566",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/aa306e4bf5657ce4a0aca4e0c64ff6828523d362"
        },
        "date": 1734731849499,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 12.511,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 8.5155,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.2185,
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
            "email": "55555210+sondavidb@users.noreply.github.com",
            "name": "David Son",
            "username": "sondavidb"
          },
          "distinct": true,
          "id": "ed29d70ae6e0e7a882d90345692e3e019828fa2b",
          "message": "Bump dependencies using scripts/bump-deps.sh\n\nSigned-off-by: github-actions[bot] <41898282+github-actions[bot]@users.noreply.github.com>",
          "timestamp": "2025-01-02T19:12:23-08:00",
          "tree_id": "1c5e4768041610a4aa486e86ca3fa39be6e9fab6",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/ed29d70ae6e0e7a882d90345692e3e019828fa2b"
        },
        "date": 1735874507270,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 12.142,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 8.788499999999999,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.022,
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
          "id": "0a7f14681a24904fcdbf66b773f424fccd9b2899",
          "message": "Bump dependencies using scripts/bump-deps.sh\n\nSigned-off-by: github-actions[bot] <41898282+github-actions[bot]@users.noreply.github.com>",
          "timestamp": "2025-01-10T07:27:52-08:00",
          "tree_id": "868e41fff072ba33fdc6862d1b66ddf392f8aaa5",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/0a7f14681a24904fcdbf66b773f424fccd9b2899"
        },
        "date": 1736523461390,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 13.1295,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 9.0705,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.4025,
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
            "email": "55906459+austinvazquez@users.noreply.github.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "distinct": true,
          "id": "1b2442118aac93685d5a6f95ca6e49101a0e6f96",
          "message": "Bump dependencies using scripts/bump-deps.sh\n\nSigned-off-by: github-actions[bot] <41898282+github-actions[bot]@users.noreply.github.com>",
          "timestamp": "2025-01-15T11:13:43-06:00",
          "tree_id": "ed17c90070bfa100a2370bb18d00f7d2d8dc3f0f",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/1b2442118aac93685d5a6f95ca6e49101a0e6f96"
        },
        "date": 1736961718156,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 11.015,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 8.843499999999999,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.9874999999999998,
            "unit": "Seconds",
            "extra": "P90"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "alvajus@amazon.com",
            "name": "Justin Alvarez",
            "username": "pendo324"
          },
          "committer": {
            "email": "kern.walster@gmail.com",
            "name": "Kern Walster",
            "username": "Kern--"
          },
          "distinct": true,
          "id": "5f3847cb9ae490478b2ee05ef14af73d48d896c5",
          "message": "Allow setting version and revision\n\nSigned-off-by: Justin Alvarez <alvajus@amazon.com>",
          "timestamp": "2025-01-15T14:58:16-08:00",
          "tree_id": "7cce33c3aaf5e3a0876284cf113a632c7b56001c",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/5f3847cb9ae490478b2ee05ef14af73d48d896c5"
        },
        "date": 1736982403903,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 9.997499999999999,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 8.7355,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.4365,
            "unit": "Seconds",
            "extra": "P90"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "macedonv@amazon.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "committer": {
            "email": "55906459+austinvazquez@users.noreply.github.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "distinct": true,
          "id": "4cfe4cc3e5c576737d20baaf95b6f45874b690d8",
          "message": "Update Go 1.22.10 in CI\n\nSigned-off-by: Austin Vazquez <macedonv@amazon.com>",
          "timestamp": "2025-01-15T17:07:10-06:00",
          "tree_id": "61da8f8612ea0ffe414518dc2e55c08c8eefe6d6",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/4cfe4cc3e5c576737d20baaf95b6f45874b690d8"
        },
        "date": 1736982895988,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 10.062999999999999,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 8.643,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.268,
            "unit": "Seconds",
            "extra": "P90"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "macedonv@amazon.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "committer": {
            "email": "55906459+austinvazquez@users.noreply.github.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "distinct": true,
          "id": "b8102223c08b72a0870953300a8364dd9afb9ad6",
          "message": "Update containerd 1.7.25, 2.0.2 in CI\n\nSigned-off-by: Austin Vazquez <macedonv@amazon.com>",
          "timestamp": "2025-01-22T10:18:12-06:00",
          "tree_id": "579a1ba090f01f540be0ee21b8eecdd03f0ca994",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/b8102223c08b72a0870953300a8364dd9afb9ad6"
        },
        "date": 1737563227068,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 13.288499999999999,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 9.414,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.46,
            "unit": "Seconds",
            "extra": "P90"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "macedonv@amazon.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "committer": {
            "email": "55906459+austinvazquez@users.noreply.github.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "distinct": true,
          "id": "4691e977191148fe01f6e5de35b1e01a5e87ded1",
          "message": "Update Go 1.22.11 in CI\n\nSigned-off-by: Austin Vazquez <macedonv@amazon.com>",
          "timestamp": "2025-01-22T12:07:28-06:00",
          "tree_id": "c89fc862fec0bf838a0398b9713b6a46d71c529f",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/4691e977191148fe01f6e5de35b1e01a5e87ded1"
        },
        "date": 1737569768844,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 11.611,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 8.9785,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.254,
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
            "email": "55906459+austinvazquez@users.noreply.github.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "distinct": true,
          "id": "986864c6532b8b391ac64c9cef029a6c1a78dd58",
          "message": "Move io abstractions to ioutils\n\nSOCI has a couple of extensions to combine various `io` interfaces. This\nPR moves these under `ioutils` instead of defining them in the packages\nwhere they were first needed to encourage reuse.\n\nSigned-off-by: Kern Walster <walster@amazon.com>",
          "timestamp": "2025-01-24T11:21:29-06:00",
          "tree_id": "073280ae78771d5c5cea0d798348cdded3b05ca7",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/986864c6532b8b391ac64c9cef029a6c1a78dd58"
        },
        "date": 1737739795530,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 10.778500000000001,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 9.076,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.462,
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
            "email": "55906459+austinvazquez@users.noreply.github.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "distinct": true,
          "id": "3b846abeb4da3cf417227041f8bf0ffc9e06a3f7",
          "message": "Remove allow_no_verification option\n\nallow_no_verification was an option that SOCI inherited from Stargz that\nis not used by SOCI. It wasn't well documented and was not connected to\nanything.\n\nSigned-off-by: Kern Walster <walster@amazon.com>",
          "timestamp": "2025-01-24T11:22:04-06:00",
          "tree_id": "a560fb987b66203b7a944bbda97fedc09dc99504",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/3b846abeb4da3cf417227041f8bf0ffc9e06a3f7"
        },
        "date": 1737739859837,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 11.787,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 9.339500000000001,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.743,
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
            "email": "55555210+sondavidb@users.noreply.github.com",
            "name": "David Son",
            "username": "sondavidb"
          },
          "distinct": true,
          "id": "8d03ee610d4aeefbc002b1e6f232a956543ba2d6",
          "message": "Bump dependencies using scripts/bump-deps.sh\n\nSigned-off-by: github-actions[bot] <41898282+github-actions[bot]@users.noreply.github.com>",
          "timestamp": "2025-02-03T16:11:31-08:00",
          "tree_id": "e5293cf5f74199cec6ccb8d724f4c0a264f5142a",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/8d03ee610d4aeefbc002b1e6f232a956543ba2d6"
        },
        "date": 1738628535494,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 17.8035,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 8.5395,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.867,
            "unit": "Seconds",
            "extra": "P90"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "macedonv@amazon.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "committer": {
            "email": "55906459+austinvazquez@users.noreply.github.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "distinct": true,
          "id": "033739819b123aaec1761c793aa8f71a4e9b5ff7",
          "message": "Update runc 1.2.5 in CI\n\nSigned-off-by: Austin Vazquez <macedonv@amazon.com>",
          "timestamp": "2025-03-12T22:32:10-05:00",
          "tree_id": "d8dd83646f49f2e7c4d4aadb54ff57d7387f6654",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/033739819b123aaec1761c793aa8f71a4e9b5ff7"
        },
        "date": 1741837231645,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 14.125,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 8.782499999999999,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.158,
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
            "email": "55906459+austinvazquez@users.noreply.github.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "distinct": true,
          "id": "f397d107aeb778b9bfdb7def5dfdd1c47609c59d",
          "message": "Properly refresh expired tokens\n\nAs issue 8735 in containerd has been fixed, we can remove this\nworkaround to force credential refresh.\n\nFunnily enough, this logic was actually broken by another bugfix in\ncontainerd (issue 9274), so we're conveniently killing two birds with\none stone here :)\n\nSigned-off-by: David Son <davbson@amazon.com>",
          "timestamp": "2025-03-12T22:34:13-05:00",
          "tree_id": "e2e5cd1b23a4ce2114f8d24c5a8e16aceccaff80",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/f397d107aeb778b9bfdb7def5dfdd1c47609c59d"
        },
        "date": 1741837305982,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 10.027000000000001,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 8.5835,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 0.978,
            "unit": "Seconds",
            "extra": "P90"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "macedonv@amazon.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "committer": {
            "email": "55555210+sondavidb@users.noreply.github.com",
            "name": "David Son",
            "username": "sondavidb"
          },
          "distinct": true,
          "id": "7c69e4e7a275ffbb19199eff03b0e370f9d9170c",
          "message": "Update golangci-lint to v1.63.4\n\nSigned-off-by: Austin Vazquez <macedonv@amazon.com>",
          "timestamp": "2025-03-13T10:48:29-07:00",
          "tree_id": "e8b5b335dc8521fd01e1657cef6d1eeda905bc49",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/7c69e4e7a275ffbb19199eff03b0e370f9d9170c"
        },
        "date": 1741888658925,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 10.370999999999999,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 9,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.03,
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
            "email": "55906459+austinvazquez@users.noreply.github.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "distinct": true,
          "id": "81b03e600f7adc16b993b594b0dfb1b98b0c3c6c",
          "message": "Bump dependencies using scripts/bump-deps.sh\n\nSigned-off-by: github-actions[bot] <41898282+github-actions[bot]@users.noreply.github.com>\nSigned-off-by: Austin Vazquez <macedonv@amazon.com>",
          "timestamp": "2025-03-13T13:52:02-05:00",
          "tree_id": "f4fb32bdf0536fd65d58bc418818f8a0d79a6677",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/81b03e600f7adc16b993b594b0dfb1b98b0c3c6c"
        },
        "date": 1741892406885,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 9.806999999999999,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 8.803,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.2475,
            "unit": "Seconds",
            "extra": "P90"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "macedonv@amazon.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "committer": {
            "email": "55906459+austinvazquez@users.noreply.github.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "distinct": true,
          "id": "ebc8e5718967fc6aa5a435ec9ce2fa5a1375d5cd",
          "message": "Bump golang.org/x/net from 0.33.0 to 0.37.0\n\nSigned-off-by: Austin Vazquez <macedonv@amazon.com>",
          "timestamp": "2025-03-13T15:41:47-05:00",
          "tree_id": "af8cdceb25cc401535791f3b677c74c0603f4a74",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/ebc8e5718967fc6aa5a435ec9ce2fa5a1375d5cd"
        },
        "date": 1741899006304,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 10.453,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 9.3925,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.2455,
            "unit": "Seconds",
            "extra": "P90"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "shubhum@amazon.com",
            "name": "Shubharanshu Mahapatra",
            "username": "Shubhranshu153"
          },
          "committer": {
            "email": "47723536+Shubhranshu153@users.noreply.github.com",
            "name": "Subhransu",
            "username": "Shubhranshu153"
          },
          "distinct": true,
          "id": "71581c2687880181a5de40fdaae9bcc3f6a6e244",
          "message": "fix: fuse file seek\n\nFind file for some containers were failing while launching containers with SOCI\n\nWe checked this was introduced via the go-fuse update to remove seek in dirstreams from 2.5.1 to 2.7.2\nin the commit hanwen/go-fuse@e885cea,\n\nThis was subsequently fixed via re-introducing the seek functionality in the pr: hanwen/go-fuse@d6170d0\n\nSigned-off-by: Shubharanshu Mahapatra <shubhum@amazon.com>",
          "timestamp": "2025-03-13T13:43:26-07:00",
          "tree_id": "e1cb87797312d0e7ad44875206560f3ed9a6c547",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/71581c2687880181a5de40fdaae9bcc3f6a6e244"
        },
        "date": 1741899086679,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 10.442,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 9.1965,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.058,
            "unit": "Seconds",
            "extra": "P90"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "macedonv@amazon.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "committer": {
            "email": "55555210+sondavidb@users.noreply.github.com",
            "name": "David Son",
            "username": "sondavidb"
          },
          "distinct": true,
          "id": "01fec775333eb637e9c6ee3af4dfd29ecdf2dfa7",
          "message": "Bump golang.org/x/oauth2 from 0.21.0 to 0.28.0\n\nSigned-off-by: Austin Vazquez <macedonv@amazon.com>",
          "timestamp": "2025-03-13T15:26:49-07:00",
          "tree_id": "66f3fe248e0c08fbc2f3f951616638589b563199",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/01fec775333eb637e9c6ee3af4dfd29ecdf2dfa7"
        },
        "date": 1741905317427,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 10.119,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 8.7195,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.042,
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
            "email": "55906459+austinvazquez@users.noreply.github.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "distinct": true,
          "id": "2a0b49fe1111fe3e574256a1aef24767e40a4f5a",
          "message": "Bump dependencies using scripts/bump-deps.sh\n\nSigned-off-by: github-actions[bot] <41898282+github-actions[bot]@users.noreply.github.com>",
          "timestamp": "2025-03-17T17:15:23-05:00",
          "tree_id": "e647076d7efeecea2c8bcebc3a23639381ebd845",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/2a0b49fe1111fe3e574256a1aef24767e40a4f5a"
        },
        "date": 1742250271145,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 14.9225,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 8.705,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.4205,
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
          "id": "170d5232609c0094570fea754b05ce3283f5fefb",
          "message": "Add metric for failing to prepare local snapshot\n\nSigned-off-by: David Son <davbson@amazon.com>",
          "timestamp": "2025-03-21T11:48:41-07:00",
          "tree_id": "34cb9e8e4691c3d3110a6cfb6d8802ea6a4d9419",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/170d5232609c0094570fea754b05ce3283f5fefb"
        },
        "date": 1742583383392,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 10.6445,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 9.349499999999999,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.221,
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
          "id": "fadf416a4d5120e340686e298b14688270a8551c",
          "message": "Update runners to ubuntu 22.04\n\nGitHub is removing ubuntu 20.04 hosted runners on April 15. This bumps\nall of our runners to 22.04\n\nThe most significant change for us is that ubuntu 22.04 uses libc 2.34\nwhich will be used in our release artifacts and may not be compatible\nwith older systems (e.g. AL2). For affected customers, we also ship a\nstatically linked binary, so this should have minimal impact.\n\nSigned-off-by: Kern Walster <walster@amazon.com>",
          "timestamp": "2025-04-01T13:32:27-07:00",
          "tree_id": "674935e6091d7e7cdcb9169519e901abfabef9a7",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/fadf416a4d5120e340686e298b14688270a8551c"
        },
        "date": 1743540097630,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 14.246500000000001,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 9.463000000000001,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.5655000000000001,
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
          "id": "84ad7cb1d6840950acc6e04c390b08a401e0e4bb",
          "message": "Remove ORAS Resolve calls in MountLocal\n\nThis PR has two parts:\n\nPreviously in #1025, we fixed an issue where MountLocal would fail as\nthe artifact fetcher would make a call to the remote store, but since we\npassed it an ORAS Repository, the Resolve call would assume it was given\na ref to a manifest, which will never be the case in this call stack.\n\nBy passing the artifact fetcher a BlobStore instead in this callstack,\nwe can remove redundant logic. However, the ORAS resolve call will\nalways try to resolve with a HEAD request, which some repos do not\nsupport. This is not a problem for the remote snapshot codepath, as it\nresolves manually via the RoundTripper. These two codepaths should\nreally be using the same logic but currently are not, so by removing the\nORAS resolve codepath in MountLocal, we can also squash this bug.\n\nSigned-off-by: David Son <davbson@amazon.com>",
          "timestamp": "2025-04-07T16:49:37-07:00",
          "tree_id": "dbf490d1b7a4a8c150d075cbd61fa248ed65cecf",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/84ad7cb1d6840950acc6e04c390b08a401e0e4bb"
        },
        "date": 1744070296219,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 17.592,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 9.474499999999999,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.3505,
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
          "id": "404f9b95ad9d99aaa08766f5334db48ee1513698",
          "message": "Attach SOCI index GC to Image\n\nBefore this change, SOCI indexes were given a root garbage collection\nlabel when using the containerd content store. This meant that in order\nto remove SOCI indexes, customers would have to know to delete the\ncontent in containerd (e.g. with ctr).\n\nThis change adds an option to the `Build` API to skip the GC root label\nand instead allow the caller to manage GC labels. As an example, it\nupdates the CLI's `create` command to set the SOCI index as a content\nref for the image. When using the contaienrd content store, the SOCI\nindex will be removed if/when the image gets removed.\n\nSigned-off-by: Kern Walster <walster@amazon.com>",
          "timestamp": "2025-04-08T09:19:44-07:00",
          "tree_id": "bf3c8d383c385d14bf26b1daac086ded739b1101",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/404f9b95ad9d99aaa08766f5334db48ee1513698"
        },
        "date": 1744129761355,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 13.0915,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 9.264,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.1059999999999999,
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
          "id": "fc8257f8f1a706a4f7aa84655865ddef0e917edc",
          "message": "Always use namespace from request context\n\nBefore this change, any calls (mostly to the containerd content store)\nwould use the namespace specified in the config. This means that\nmaking changes to the content store within one namespace would still be\naccessible when using another namespace. This was done because the\ncontext would have a namespace attached in the gRPC header, but not in\nthe context struct, so if we didn't explicitly attach a namespace to the\ncontext, any calls to the containerd content store would fail.\n\nThis change adds middleware to the gRPC to explicitly attach the context\nfrom the header to the context, so that it can be passed to store calls.\n\nThis change also removes the config option to specify a specific\nnamespace, which is okay since the CLI was never tied to this and still\nrequires a namespace to be used, and the correct namespace to use is the\none attached to the containerd context anyway.\n\nSigned-off-by: David Son <davbson@amazon.com>",
          "timestamp": "2025-04-08T11:13:31-07:00",
          "tree_id": "4bd9b3625d4fc1a1cbaa55c556e06498c9f18fab",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/fc8257f8f1a706a4f7aa84655865ddef0e917edc"
        },
        "date": 1744136687790,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 16.979,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 9.3415,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 2.1345,
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
            "email": "55906459+austinvazquez@users.noreply.github.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "distinct": true,
          "id": "745588355d864c4bf96d81af8de07aeeeba3b847",
          "message": "Bump dependencies using scripts/bump-deps.sh\n\nSigned-off-by: github-actions[bot] <41898282+github-actions[bot]@users.noreply.github.com>",
          "timestamp": "2025-04-09T15:19:48-05:00",
          "tree_id": "9ffa273d3695feb6e01eb9ddc6bcfb219696d717",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/745588355d864c4bf96d81af8de07aeeeba3b847"
        },
        "date": 1744230501694,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 11.942,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 9.407,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 5.9505,
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
          "id": "15370c6fd7af921f6ca9daeb6ef14621592823e1",
          "message": "Refactor SOCI Init logic\n\nThis change moves most of the SOCI init logic out of `SociContext.Init`\ninto `fs.fetchSociIndex`. This is to reduce the number of args we need\nto sent to `SociContext.Init`.\n\nThere are 2 logic changes here:\n\n1. Remove the cached error lock on `SociContext`. `sync.Once` guarantees\n   that no call to `Do` will return until exactly 1 executes the\n   function. Therefore, there was never any concurrent access as the\n   write to the cached error will always happen before any read.\n2. The SOCI index digest is parsed rather than casted to a\n   digest.Digest.\n\nSigned-off-by: Kern Walster <walster@amazon.com>",
          "timestamp": "2025-04-10T09:05:21-07:00",
          "tree_id": "531b7ab2204af7e525a810a457b1d26ca2d2bd13",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/15370c6fd7af921f6ca9daeb6ef14621592823e1"
        },
        "date": 1744301618280,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 12.606,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 9.474499999999999,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.2325,
            "unit": "Seconds",
            "extra": "P90"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "macedonv@amazon.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "committer": {
            "email": "55906459+austinvazquez@users.noreply.github.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "distinct": true,
          "id": "d484678111e08ef4de7b439805982074d264c007",
          "message": "Dockerfile: update container registry to v3.0.0\n\nSigned-off-by: Austin Vazquez <macedonv@amazon.com>",
          "timestamp": "2025-04-15T18:35:35-05:00",
          "tree_id": "c83663dd3a4a0dfc2b13fc14e160b4d7296fc314",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/d484678111e08ef4de7b439805982074d264c007"
        },
        "date": 1744760632688,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 10.981,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 9.4735,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 0.9565,
            "unit": "Seconds",
            "extra": "P90"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "macedonv@amazon.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "committer": {
            "email": "55906459+austinvazquez@users.noreply.github.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "distinct": true,
          "id": "2b3b87767712d503fe72462f35bb948dbe37601b",
          "message": "Update containerd {1.6.38,1.7.27,2.0.4} in CI\n\nSigned-off-by: Austin Vazquez <macedonv@amazon.com>",
          "timestamp": "2025-04-15T18:36:22-05:00",
          "tree_id": "c7db9aac2ace44e9d66f80f93c4f409c5e9c5cac",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/2b3b87767712d503fe72462f35bb948dbe37601b"
        },
        "date": 1744760648240,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 17.3565,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 9.935,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.088,
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
          "id": "4544842f4b54548facccdfa94f358f6fc6bf3518",
          "message": "Disable systemd tests on x86 codebuild\n\nsystemd tests fail on codebuild x86 runners due to some issue with\ncgroups v2. For now, disable the tests (they should still pass on ARM)\nso we're not blocking PRs while we investigate.\n\nSigned-off-by: Kern Walster <walster@amazon.com>",
          "timestamp": "2025-04-16T21:28:12-07:00",
          "tree_id": "574ba06596a078fed9eb3d5aaa2569c41ff5ba35",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/4544842f4b54548facccdfa94f358f6fc6bf3518"
        },
        "date": 1744864608664,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 13.4195,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 9.4515,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.13,
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
          "id": "575576db28b6b50c313e17a77dcdc3dda8a0a2dc",
          "message": "Update golang.org/x/net to v0.39.0\n\nThis is a manual dependency bump to patch CVE GO-2025-3595, since our\nautomated script doesn't auto-bump indirect dependencies.\n\nhttps://pkg.go.dev/vuln/GO-2025-3595\n\nSigned-off-by: David Son <davbson@amazon.com>",
          "timestamp": "2025-04-17T10:25:52-07:00",
          "tree_id": "f33f8b2d723a67910167ab265347621f85cf23ff",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/575576db28b6b50c313e17a77dcdc3dda8a0a2dc"
        },
        "date": 1744911313345,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 11.469,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 9.098,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.3735,
            "unit": "Seconds",
            "extra": "P90"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "macedonv@amazon.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "committer": {
            "email": "55906459+austinvazquez@users.noreply.github.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "distinct": true,
          "id": "f1dc793496e7fcda14974c9ef9fae36df78044cc",
          "message": "Dockerfile: add igzip and rapidgzip to development container\n\nSigned-off-by: Austin Vazquez <macedonv@amazon.com>",
          "timestamp": "2025-04-18T17:51:06-05:00",
          "tree_id": "6195d2cf98b1bb70618932924e92ef424f5a9899",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/f1dc793496e7fcda14974c9ef9fae36df78044cc"
        },
        "date": 1745017120542,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 11.3865,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 9.475999999999999,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.1195,
            "unit": "Seconds",
            "extra": "P90"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "macedonv@amazon.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "committer": {
            "email": "55906459+austinvazquez@users.noreply.github.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "distinct": true,
          "id": "8674b00fbbc98df538c41f7820ed2d85a34c173d",
          "message": "Dockerfile: build rapidgzip on ARM\n\nSigned-off-by: Austin Vazquez <macedonv@amazon.com>",
          "timestamp": "2025-04-21T11:07:08-07:00",
          "tree_id": "3c93603f85fff35798dfe6f2d9ba08e56fa79c91",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/8674b00fbbc98df538c41f7820ed2d85a34c173d"
        },
        "date": 1745259301748,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 11.282,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 9.3385,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.2885,
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
          "id": "e155859d63cadcb6dbf02ad09cb70c6eea17185f",
          "message": "Use functional args for config in integ tests\n\nSigned-off-by: Kern Walster <walster@amazon.com>",
          "timestamp": "2025-04-21T12:46:57-07:00",
          "tree_id": "632985df0688565cad6cc358ee34caaf07dc2aec",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/e155859d63cadcb6dbf02ad09cb70c6eea17185f"
        },
        "date": 1745265276309,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 11.402000000000001,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 9.7455,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.2955,
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
          "id": "69e9063ef54c93575c154caf37f8bae650a19027",
          "message": "Add error to unexpected read failures\n\nSigned-off-by: Kern Walster <walster@amazon.com>",
          "timestamp": "2025-04-22T19:53:16-07:00",
          "tree_id": "52981c0462bce01d822945a21a9aa5c06bb46929",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/69e9063ef54c93575c154caf37f8bae650a19027"
        },
        "date": 1745377324573,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 14.9925,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 9.3205,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.1265,
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
            "email": "55555210+sondavidb@users.noreply.github.com",
            "name": "David Son",
            "username": "sondavidb"
          },
          "distinct": true,
          "id": "60475f67947df43dcb075e93b41fd9b4b5393906",
          "message": "Bump dependencies using scripts/bump-deps.sh\n\nSigned-off-by: github-actions[bot] <41898282+github-actions[bot]@users.noreply.github.com>",
          "timestamp": "2025-04-23T09:18:04-07:00",
          "tree_id": "cd3fdbd4a52073891e5ffcf230683dcada2fe49c",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/60475f67947df43dcb075e93b41fd9b4b5393906"
        },
        "date": 1745425602366,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 12.832,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 9.328,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.404,
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
          "id": "739c04f167b63b334e6cdae4486cabe8b17f69a7",
          "message": "Pass cache options to spancache.Get\n\nBefore this change, the span manager did not pass cache opts to the span\ncache when getting keys, but it did pass options when putting keys. This\nchange makes them consistent.\n\nThis is problematic with the Direct option because writes are directly\nto files, but reads use a file descriptor cache. This leads to the\nfollowing issue:\n\n1. The background fetcher fetches a span, but does not decompress it\n2. The background fetcher writes the compressed span to the span cache\n3. A later read loads the compressed data from the span cache. This\n   caches the file descriptor\n4. The span is decompressed\n5. The span is rewritten to the same cache key by opening a temp file,\n   writing the decompressed span data, then renaming it to the original\n   name. NOTE: This is a different file descriptor from step 3!\n6. The span is marked decompressed\n7. Another read hits in the cache and get's back the fd to the (now\n   deleted) compressed file from step 3.\n8. SOCI returns compressed data or get's an unxpected EOF because there\n   isn't as much compressed data as uncompressed data.\n\nThe solution here is to pass cache options. Each cache Get opens a new\nfile descriptor, so the caching issue is bypassed.\n\nSigned-off-by: Kern Walster <walster@amazon.com>",
          "timestamp": "2025-04-25T11:06:26-07:00",
          "tree_id": "bb6e930d3a227a6c9e9b80f7fe543672fe37fa9c",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/739c04f167b63b334e6cdae4486cabe8b17f69a7"
        },
        "date": 1745604941480,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 12.11,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 9.2595,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.6355,
            "unit": "Seconds",
            "extra": "P90"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "macedonv@amazon.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "committer": {
            "email": "55906459+austinvazquez@users.noreply.github.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "distinct": true,
          "id": "d72b9cc31885f9f6192adceeea73bb8c6e3d07b2",
          "message": "ci: add containerd-2.1.0-rc.0 to integration test matrix\n\nSigned-off-by: Austin Vazquez <macedonv@amazon.com>",
          "timestamp": "2025-04-30T08:18:51-07:00",
          "tree_id": "2a278881ca3a38b28bb8c43544c45f57e59bec29",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/d72b9cc31885f9f6192adceeea73bb8c6e3d07b2"
        },
        "date": 1746026914713,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 13.8725,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 9.334,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 2.2245,
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
          "id": "963edca6d4a103f15c04cfbb77ae3e17ee17f4cf",
          "message": "Add SOCI conversion\n\nThis change adds a command to convert an image to a SOCI-enabled image which\ncontains strong references to both the image and SOCI index in a\nlogically single, deployable artifact.\n\nSigned-off-by: Kern Walster <walster@amazon.com>",
          "timestamp": "2025-04-30T15:20:03-07:00",
          "tree_id": "3e7f4b53bb17cf40556545332fb8370edda97667",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/963edca6d4a103f15c04cfbb77ae3e17ee17f4cf"
        },
        "date": 1746052117606,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 15.8005,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 9.14,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.9129999999999998,
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
          "id": "528b46ae132735ab96dd2e6b3d1ba6c2adbe83c1",
          "message": "Fix config mediatype in Convert\n\nConverting an image to a soci-enabled image also converts docker v2\nimages to oci v1 images. Before this change, we didn't change the\nimage config's mediatype meaning the image would claim it is an OCI v1\nimage while the config would claim it's a Docker v2 image. This caused\nsome registries to interpret this as a non-image artifact. This change\nsets the config media type to be a OCI v1 config.\n\nSigned-off-by: Kern Walster <walster@amazon.com>",
          "timestamp": "2025-05-01T13:48:08-07:00",
          "tree_id": "90cecfb6aca42624a0fdcd3657e9e994ddf89933",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/528b46ae132735ab96dd2e6b3d1ba6c2adbe83c1"
        },
        "date": 1746132998034,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 12.022,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 9.411999999999999,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.0745,
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
          "id": "82530d44ee2f477a9bbd2b821d6bfcb7711b5c16",
          "message": "Support finding SOCI indexes in image annotations\n\nThis change adds a new mechanism to discover SOCI indexes without the\nOCI referrers API via annotations on the image manifest. The new\nmechanism is called SOCI v2 while the original is called SOCI v1.\n\nBy default, SOCI v1 is disabled and SOCI v2 is enabled.\n\nSigned-off-by: Kern Walster <walster@amazon.com>",
          "timestamp": "2025-05-02T14:12:37-07:00",
          "tree_id": "c632f197ff6ca7ad554eb0d8642dc59ce2ad26bc",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/82530d44ee2f477a9bbd2b821d6bfcb7711b5c16"
        },
        "date": 1746220869663,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 12.474499999999999,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 9.869,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.322,
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
          "id": "2618de545e9cf5fbc1e4056540eb6743e14e0ace",
          "message": "Run benchmark workflows on change\n\nSigned-off-by: David Son <davbson@amazon.com>",
          "timestamp": "2025-05-05T10:20:28-07:00",
          "tree_id": "7162068db80b9c20bbf33f10132c4a24fbd2a2fb",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/2618de545e9cf5fbc1e4056540eb6743e14e0ace"
        },
        "date": 1746466099229,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 14.7515,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 9.526499999999999,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.0899999999999999,
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
          "id": "869f8b30841c6836f426c187ab88cadc4de8b25f",
          "message": "Increase CI Integration test timeout\n\nSigned-off-by: Kern Walster <walster@amazon.com>",
          "timestamp": "2025-05-06T11:40:25-07:00",
          "tree_id": "5dcb9340697e5e5132b17fc576a6409cb46cf920",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/869f8b30841c6836f426c187ab88cadc4de8b25f"
        },
        "date": 1746557277957,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 11.931000000000001,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 9.318,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.412,
            "unit": "Seconds",
            "extra": "P90"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "macedonv@amazon.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "committer": {
            "email": "55906459+austinvazquez@users.noreply.github.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "distinct": true,
          "id": "6f51b4b32ed6740b11d8de9cc1c978f075dcda7b",
          "message": "ci: integration test containerd@v2.1.0-rc.1\n\nSigned-off-by: Austin Vazquez <macedonv@amazon.com>",
          "timestamp": "2025-05-07T09:26:27-07:00",
          "tree_id": "90e0f818e499d4eedb711216dabee344c310ab8c",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/6f51b4b32ed6740b11d8de9cc1c978f075dcda7b"
        },
        "date": 1746635647528,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 11.1935,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 9.261500000000002,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.714,
            "unit": "Seconds",
            "extra": "P90"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "retornam@users.noreply.github.com",
            "name": "retornam",
            "username": "retornam"
          },
          "committer": {
            "email": "55906459+austinvazquez@users.noreply.github.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "distinct": true,
          "id": "4b87d89f2f2965160ee5edd34d24ad962469cc93",
          "message": "Updates to Dockerfile\n\ncontainerd to 1.7\n\nAccording to [1] the 1.6 branch of containerd is in log term testing\nuntil July 23, 2025, they suggest upgrading to 1.7 which will be\nin long term testing until March 10, 2026.\n\n[1]https://github.com/containerd/containerd/blob/main/RELEASES.md\n\nUpgrade runc to 1.3.0\n\nAccording to [1] opencontainers/runc\n\n\"runc follows a 6-month minor version release schedule, with the\n aim of releases happening at the end of April and October each\nyear.\"\n\nStarting with 1.3.0 which was released Apr 30th, 2025.\n\nThis PR upgrades the image so we can follow their release cadence.\n\n[1] https://github.com/opencontainers/runc/blob/main/RELEASES.md\n\nClear dnf cache after install\n\nAdd the dnf clean command to clear the DNF cache, deletie temporary\nfiles, metadata, and cached packages.\n\nI also reformatted the Dockerfile to be follow recommended formatting\nguidelines\n\nUpgrade nerdctl to 1.7.7\n\nA new version of nerdctl has been released[1] with\nseveral fixes and is compatible with containerd versions\nv1.6 and v1.7\n\n[1] https://github.com/containerd/nerdctl/releases/tag/v1.7.7\n\nSigned-off-by: Raymond Etornam <retornam@users.noreply.github.com>",
          "timestamp": "2025-05-07T11:25:33-07:00",
          "tree_id": "99ef4706499a483af9d6d66273fecb35484a85c1",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/4b87d89f2f2965160ee5edd34d24ad962469cc93"
        },
        "date": 1746642795620,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 10.720500000000001,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 9.2005,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.4195,
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
            "email": "55906459+austinvazquez@users.noreply.github.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "distinct": true,
          "id": "1499076f097d7334dbcb395f60c222a90268530d",
          "message": "Bump dependencies using scripts/bump-deps.sh\n\nSigned-off-by: github-actions[bot] <41898282+github-actions[bot]@users.noreply.github.com>",
          "timestamp": "2025-05-08T08:10:29-07:00",
          "tree_id": "9fa74cd853ecafbc21d2b782dfbde0c0f35fbc55",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/1499076f097d7334dbcb395f60c222a90268530d"
        },
        "date": 1746717594089,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 11.998,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 9.3135,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.6,
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
          "id": "65562008c43fb5231ac104cf038aecc4dd6d30ca",
          "message": "Add FetchSociArtifacts test\n\nThis adds tests to verify that we correctly reject SOCI indexes if\nthey are somehow modified by the registry. We have always done this, but\nthis verifies it via a test that's run continuously.\n\nSigned-off-by: Kern Walster <walster@amazon.com>",
          "timestamp": "2025-05-08T09:48:28-07:00",
          "tree_id": "e36b363026c7d8f974aaebe80e0c93abc4bf4b92",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/65562008c43fb5231ac104cf038aecc4dd6d30ca"
        },
        "date": 1746723456929,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 14.2225,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 9.5115,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.5605000000000002,
            "unit": "Seconds",
            "extra": "P90"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "macedonv@amazon.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "committer": {
            "email": "55906459+austinvazquez@users.noreply.github.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "distinct": true,
          "id": "9266a7cf122b1315e24dcf8f438137fc1d1976fe",
          "message": "ci: update containerd@v2.1.0 integration testing\n\nSigned-off-by: Austin Vazquez <macedonv@amazon.com>",
          "timestamp": "2025-05-13T07:28:54-07:00",
          "tree_id": "8677bfa4ee1c13bbda7e86ffc08fcff104764c79",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/9266a7cf122b1315e24dcf8f438137fc1d1976fe"
        },
        "date": 1747147109766,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 11.120999999999999,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 9.046,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 2.2239999999999998,
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
            "email": "55906459+austinvazquez@users.noreply.github.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "distinct": true,
          "id": "9fae5811513832d244c90bbf7dcbdd7b52e10e64",
          "message": "Bump dependencies using scripts/bump-deps.sh\n\nSigned-off-by: github-actions[bot] <41898282+github-actions[bot]@users.noreply.github.com>",
          "timestamp": "2025-05-13T15:36:24-07:00",
          "tree_id": "1d0c61c2b60667072c0c4ce7930b4e42dd004de4",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/9fae5811513832d244c90bbf7dcbdd7b52e10e64"
        },
        "date": 1747176402900,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 15.5125,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 9.49,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.8885,
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
          "id": "926f17e7c6a55c61c3bbc53616a04619144de3ee",
          "message": "Add test for dangling SOCI index v2 reference\n\nIf an image manifest contains a reference to a SOCI index, but the SOCI\nindex doesn't exist in the registry, the snapshotter should try to use\nthat SOCI index, but then fallback to non-lazy loading when it can't\nfind the SOCI index. This adds a test to verify that.\n\nSigned-off-by: Kern Walster <walster@amazon.com>",
          "timestamp": "2025-05-15T08:57:13-07:00",
          "tree_id": "b525ce2c91ccb91ed70e1bad94e38c4b7bcf0518",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/926f17e7c6a55c61c3bbc53616a04619144de3ee"
        },
        "date": 1747325154085,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 13.1325,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 9.197500000000002,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.674,
            "unit": "Seconds",
            "extra": "P90"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "macedonv@amazon.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "committer": {
            "email": "55906459+austinvazquez@users.noreply.github.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "distinct": true,
          "id": "ca55b7fe5f35682e24c69da51ce93b584b3c6359",
          "message": "Add SOCI CLI root flag\n\nThis change adds --root flag for the soci command and refactors common\ncode with the snapshotter grpc for similar logic.\n\nSigned-off-by: Austin Vazquez <macedonv@amazon.com>",
          "timestamp": "2025-05-19T09:57:40-07:00",
          "tree_id": "4a26569726033740b5195af7f609bb7ceffdd774",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/ca55b7fe5f35682e24c69da51ce93b584b3c6359"
        },
        "date": 1747674421963,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 15.288,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 9.881,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.0905,
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
          "id": "f2d8312d0d12fdc40d5964b3b8dd203968c9a02d",
          "message": "Add test for pulling an image with the wrong index\n\nThis change adds a test that if you manage to pull and image with a SOCI\nindex that was built for another image, the SOCI snapshotter will use\nthe SOCI index, but the image will not be lazy loaded.\n\nSigned-off-by: Kern Walster <walster@amazon.com>",
          "timestamp": "2025-05-21T14:09:48-07:00",
          "tree_id": "58b5a81defe8c0835fa1d4374d3a7f350de554a4",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/f2d8312d0d12fdc40d5964b3b8dd203968c9a02d"
        },
        "date": 1747862288416,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 11.048,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 9.107500000000002,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 0.9675,
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
          "id": "16d71c28b17e959def9c226111f41df1b5e9bb3a",
          "message": "Test invalid conversions\n\nIf a SOCI index or image manifest is removed from a converted image, the\nimage should be invalid. This change adds a test to confirm this.\n\nThis effectively tests that we are actually putting the SOCI index into\nthe converted Image Index.\n\nSigned-off-by: Kern Walster <walster@amazon.com>",
          "timestamp": "2025-05-21T14:21:52-07:00",
          "tree_id": "32133fe99e972c0c55692283735f359c3d170ec2",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/16d71c28b17e959def9c226111f41df1b5e9bb3a"
        },
        "date": 1747863021949,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 12.461500000000001,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 9.901499999999999,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.147,
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
          "id": "8bbfe951bbb411798ee85dbd908544df4a1619a8",
          "message": "Add test for span arg validation\n\nThis adds a test that a user cannot specify a min-layer-size or\nspan-size that is negative or larger than what can fit into a signed\n64-bit integer.\n\nSigned-off-by: Kern Walster <walster@amazon.com>",
          "timestamp": "2025-05-21T14:22:17-07:00",
          "tree_id": "c8707347eb25b8b26d06222e27f816c6c8a92d79",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/8bbfe951bbb411798ee85dbd908544df4a1619a8"
        },
        "date": 1747863021864,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 11.658000000000001,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 9.265,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 0.821,
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
          "id": "f69ce011022e04482a91f9ffce7d012bb4b620bc",
          "message": "Resolve descriptor before unpacking\n\nIn #1493, we introduced a fix where OPAS Resolve calls would fail when\npulling images from Public ECR using crictl by implementing our own\nlogic to populate the descriptor size. In doing this we removed\n\"redundant\" logic in fs/fs.go, but this removal actually broke the same\nuse case using the ORAS store. (containerd content store doesn't check\nthe expected descriptor size if it is zero, so it passed in this use\ncase.) This is because we would resolve the descriptor in Fetch, but\nthen when storing we would use the unresolved descriptor, which ORAS\ndoes not have a special ignore case for if the descriptor size is zero,\nso it would fail to ingest and error out.\n\nThis change re-adds this logic back.\n\nSigned-off-by: David Son <davbson@amazon.com>",
          "timestamp": "2025-05-23T15:56:38-07:00",
          "tree_id": "f886c22c07444e62db46b18320d1d8c285d77d19",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/f69ce011022e04482a91f9ffce7d012bb4b620bc"
        },
        "date": 1748041551632,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 11.445,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 9.788,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 0.9444999999999999,
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
          "id": "fb0570f8d239d60e9bcec994a059dd24d5beb1ac",
          "message": "Add ARM release workflow\n\nSigned-off-by: David Son <davbson@amazon.com>",
          "timestamp": "2025-05-28T15:28:04-07:00",
          "tree_id": "3828926c8edc490f924ab9f2da9b74a6b35b3c5b",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/fb0570f8d239d60e9bcec994a059dd24d5beb1ac"
        },
        "date": 1748471773177,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 11.475999999999999,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 9.244,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.3305,
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
          "id": "f53f34731338305393662163675d30dfe87fcc92",
          "message": "Fix TestOverlayView in userns\n\nTestOverlayView assumes that it is not run in a user namespace or that\n/tmp is a tmpfs. If it is run in a user namespace with a non-tmpfs /tmp,\nthen the mount contains an additional `userxattr` mount option. This\nchange checks if we expect that option and asserts on it as necessary.\n\nSigned-off-by: Kern Walster <walster@amazon.com>",
          "timestamp": "2025-05-28T15:35:06-07:00",
          "tree_id": "4cb81574241a33221a9c43a801c42789110a780a",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/f53f34731338305393662163675d30dfe87fcc92"
        },
        "date": 1748472178681,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 10.763,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 9.177,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.3525,
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
          "id": "fa7d4ca92b444e71b4cd54c757e9165552838eed",
          "message": "Use default platform in convert\n\nBefore this change, the SOCI cli defaulted to converting all platforms.\nThis is inconsistent with nerdctl's pull, push, and convert functions.\nIt also leads to weird issues like this not working:\n\n```\nnerdctl pull docker.io/library/busybox:latest\nsoci convert docker.io/library/busybox:latest other:latest-soci\n```\n\nthe solution is either to pull with `--all-platforms` or convert with\n`--platform`. The latter had an additional bug that convert touched all\nmanifests, even if you didn't convert them.\n\nThis change makes SOCI default to single platform conversions and fixes\nthe manifests bug. SOCI convert now behaves like nerdctl\npull/convert/push.\n\nSigned-off-by: Kern Walster <walster@amazon.com>",
          "timestamp": "2025-05-29T10:13:03-07:00",
          "tree_id": "5d9b8231688dd86e2e15b93e8556893a63e20b75",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/fa7d4ca92b444e71b4cd54c757e9165552838eed"
        },
        "date": 1748539366669,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 14.558,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 10.05,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.214,
            "unit": "Seconds",
            "extra": "P90"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "macedonv@amazon.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "committer": {
            "email": "55906459+austinvazquez@users.noreply.github.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "distinct": true,
          "id": "32dcd22b868132b63b974d6c346c84ae6eb50193",
          "message": "Upgrade to github.com/pelletier/go-toml/v2\n\nSigned-off-by: Austin Vazquez <macedonv@amazon.com>",
          "timestamp": "2025-06-02T14:50:16-07:00",
          "tree_id": "2cdc09fcf7074d65bc4a0280ea198cec631d27cc",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/32dcd22b868132b63b974d6c346c84ae6eb50193"
        },
        "date": 1748901653049,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 15.466,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 9.217500000000001,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.7999999999999998,
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
            "email": "55555210+sondavidb@users.noreply.github.com",
            "name": "David Son",
            "username": "sondavidb"
          },
          "distinct": true,
          "id": "e0be12595269793172abfb80394f4eaebe1941eb",
          "message": "Bump dependencies using scripts/bump-deps.sh\n\nSigned-off-by: github-actions[bot] <41898282+github-actions[bot]@users.noreply.github.com>",
          "timestamp": "2025-06-03T14:12:44-07:00",
          "tree_id": "be93d052b723cf47add8de9d78d5df425e0ad9a7",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/e0be12595269793172abfb80394f4eaebe1941eb"
        },
        "date": 1748985693165,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 13.325,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 9.469999999999999,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.814,
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
          "id": "3d62be8fe92673e2c465e68dd4a5985c5dd5da8d",
          "message": "Update SOCI index manifest V2 db references\n\nWhen we convert images to SOCI index manifest v2, we first create SOCI\nindexes, then add the annotations to the image manifests. In the process\nof creating the SOCI indexes, we create an artifacts DB reference to the\nSOCI index using the original manifest and image ids. This change goes\nback after everything is done and updates the artifacts DB so the index\nis associated with the new manifest and image.\n\nSigned-off-by: Kern Walster <walster@amazon.com>",
          "timestamp": "2025-06-05T13:52:02-07:00",
          "tree_id": "1a79bd97ab12bbf3d7a1e6556abf8f1e05b29ee6",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/3d62be8fe92673e2c465e68dd4a5985c5dd5da8d"
        },
        "date": 1749157255893,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 13.092500000000001,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 9.844999999999999,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.0955,
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
          "id": "d96cd2d58828d1d5ebe41a97d9ab4e421e9cac30",
          "message": "Prevent pushing SOCI index manifest v2\n\nSOCI index manifest v2 does not have a subject field and instead relies\non a connection to an image via an image index. Before this change, you\ncould use `soci push` to push a SOCI index manifest v2 without the rest\nof the image which is unusable without the image index. This change\ndetects SOCI index manifest v2 and skips the push.\n\nSigned-off-by: Kern Walster <walster@amazon.com>",
          "timestamp": "2025-06-09T12:51:20-07:00",
          "tree_id": "89a210f1fc0d59a0459a3f2b1dc62caf32103954",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/d96cd2d58828d1d5ebe41a97d9ab4e421e9cac30"
        },
        "date": 1749499257894,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 13.285,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 9.479,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.8365,
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
            "email": "55555210+sondavidb@users.noreply.github.com",
            "name": "David Son",
            "username": "sondavidb"
          },
          "distinct": true,
          "id": "4f5969182edd24feb9c5ef8f91e7309b982cb414",
          "message": "Bump dependencies using scripts/bump-deps.sh\n\nSigned-off-by: github-actions[bot] <41898282+github-actions[bot]@users.noreply.github.com>",
          "timestamp": "2025-06-11T10:57:45-07:00",
          "tree_id": "85dfc000ddbaccada29fd2d8c4285de494d38678",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/4f5969182edd24feb9c5ef8f91e7309b982cb414"
        },
        "date": 1749665359999,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 16.133000000000003,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 9.5215,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.2985,
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
          "id": "0a23f72a606f1105002c3a52ecbac4658722dff1",
          "message": "Add pull_modes to example config\n\nSigned-off-by: Kern Walster <walster@amazon.com>",
          "timestamp": "2025-06-11T13:26:20-07:00",
          "tree_id": "802320761ff9769e32c89c792369ae02243d4f07",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/0a23f72a606f1105002c3a52ecbac4658722dff1"
        },
        "date": 1749674051953,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 10.959,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 9.18,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.3465,
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
          "id": "edb6dcfebbc00731896935d1701c5177a3e7069b",
          "message": "Add debug documentation for pull mode\n\nThis change adds information to debug how the SOCI snapshotter pulls an\nimage.\n\nSigned-off-by: Kern Walster <walster@amazon.com>",
          "timestamp": "2025-06-13T16:20:58-07:00",
          "tree_id": "00fc359dc03c99912c8e3e3d995fb7c661c8bc69",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/edb6dcfebbc00731896935d1701c5177a3e7069b"
        },
        "date": 1749857341063,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 11.6525,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 9.184999999999999,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.2955,
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
          "id": "63e5cf953dd1e5c680d81af21e2702ad84db3314",
          "message": "Fix incorrect type check for soci ztoc info\n\nWe used to check the artifact's MediaType to determine if the artifact\nbeing passed to the command was a SOCI index or a zTOC, but the\nMediaType will always be the same regardless of the artifact type. This\nfixes the check to look for the artifact type instead of the MediaType.\n\nSigned-off-by: David Son <davbson@amazon.com>",
          "timestamp": "2025-06-23T13:26:24-07:00",
          "tree_id": "06584353512fe588ae9d3a06e00e0134b941ac2c",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/63e5cf953dd1e5c680d81af21e2702ad84db3314"
        },
        "date": 1750710867869,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 11.4735,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 9.7405,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 0.9455,
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
          "id": "f08749521a52434456a9affe32848f0bb5778bf4",
          "message": "Update go in CI to 1.24.4\n\nSigned-off-by: Kern Walster <walster@amazon.com>",
          "timestamp": "2025-06-26T11:30:16-07:00",
          "tree_id": "9bd1d5377cd85c857f71f1d85b33d67017092b1f",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/f08749521a52434456a9affe32848f0bb5778bf4"
        },
        "date": 1750963209206,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 14.382000000000001,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 9.943999999999999,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.651,
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
            "email": "55555210+sondavidb@users.noreply.github.com",
            "name": "David Son",
            "username": "sondavidb"
          },
          "distinct": true,
          "id": "506e318fcc768b1140b6cbd223baf0091bb2d0ee",
          "message": "Bump golangci/golangci-lint-action from 6 to 7\n\nBumps [golangci/golangci-lint-action](https://github.com/golangci/golangci-lint-action) from 6 to 7.\n- [Release notes](https://github.com/golangci/golangci-lint-action/releases)\n- [Commits](https://github.com/golangci/golangci-lint-action/compare/v6...v7)\n\n---\nupdated-dependencies:\n- dependency-name: golangci/golangci-lint-action\n  dependency-type: direct:production\n  update-type: version-update:semver-major\n...\n\nSigned-off-by: dependabot[bot] <support@github.com>",
          "timestamp": "2025-06-26T12:50:44-07:00",
          "tree_id": "d18d896c5ad491651868d9f6b9ffa45ef955f9aa",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/506e318fcc768b1140b6cbd223baf0091bb2d0ee"
        },
        "date": 1750967973784,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 11.8335,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 9.281500000000001,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.127,
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
          "id": "472f4e5b563910064e4d040e62f49cacfef9c134",
          "message": "Wait for containerd in systemd tests\n\nSigned-off-by: Kern Walster <walster@amazon.com>",
          "timestamp": "2025-06-26T13:41:29-07:00",
          "tree_id": "b9a7433e4abc59ad139644acbc2afa270c74e199",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/472f4e5b563910064e4d040e62f49cacfef9c134"
        },
        "date": 1750971014308,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 14.5215,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 9.5195,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.1445,
            "unit": "Seconds",
            "extra": "P90"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "henwang@amazon.com",
            "name": "Henry Wang",
            "username": "henry118"
          },
          "committer": {
            "email": "55555210+sondavidb@users.noreply.github.com",
            "name": "David Son",
            "username": "sondavidb"
          },
          "distinct": true,
          "id": "4a78fc699fb41ca6401ac9b457b11bde4014b257",
          "message": "refactor parallel pull mode configs\n\nSigned-off-by: Henry Wang <henwang@amazon.com>",
          "timestamp": "2025-06-27T11:20:02-07:00",
          "tree_id": "2593c5bc40b4cfabc52bbd6d4ae070eaa51fd393",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/4a78fc699fb41ca6401ac9b457b11bde4014b257"
        },
        "date": 1751048911162,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 13.7515,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 9.8245,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 3.7395,
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
            "email": "55555210+sondavidb@users.noreply.github.com",
            "name": "David Son",
            "username": "sondavidb"
          },
          "distinct": true,
          "id": "296f71fd15a03ec2859a76a2eb0081999ac0a85e",
          "message": "Bump dependencies using scripts/bump-deps.sh\n\nSigned-off-by: github-actions[bot] <41898282+github-actions[bot]@users.noreply.github.com>",
          "timestamp": "2025-06-27T13:18:42-07:00",
          "tree_id": "2760b2e82225f0474344325b85cea49d7af73e8f",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/296f71fd15a03ec2859a76a2eb0081999ac0a85e"
        },
        "date": 1751056028641,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 11.533999999999999,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 9.634,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 0.956,
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
            "email": "55555210+sondavidb@users.noreply.github.com",
            "name": "David Son",
            "username": "sondavidb"
          },
          "distinct": true,
          "id": "57fc72d626cee0a6ae328c02dce4c91f50309fef",
          "message": "Bump golangci/golangci-lint-action from 7 to 8\n\nBumps [golangci/golangci-lint-action](https://github.com/golangci/golangci-lint-action) from 7 to 8.\n- [Release notes](https://github.com/golangci/golangci-lint-action/releases)\n- [Commits](https://github.com/golangci/golangci-lint-action/compare/v7...v8)\n\n---\nupdated-dependencies:\n- dependency-name: golangci/golangci-lint-action\n  dependency-version: '8'\n  dependency-type: direct:production\n  update-type: version-update:semver-major\n...\n\nSigned-off-by: dependabot[bot] <support@github.com>",
          "timestamp": "2025-06-30T11:38:16-07:00",
          "tree_id": "2efcb095becdfc2ac9e68804a33887a0d76634cb",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/57fc72d626cee0a6ae328c02dce4c91f50309fef"
        },
        "date": 1751309242990,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 11.8485,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 9.910499999999999,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.825,
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
          "id": "cf5bfbaa3727b032c993a99d99933fbdde6b773a",
          "message": "Upgrade benchmark scripts to 1.24.4\n\nSigned-off-by: David Son <davbson@amazon.com>",
          "timestamp": "2025-06-30T11:38:25-07:00",
          "tree_id": "3bd68878cb0d502c17b9c4953cc085c9618019b0",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/cf5bfbaa3727b032c993a99d99933fbdde6b773a"
        },
        "date": 1751309302716,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 12.533999999999999,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 9.5775,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.574,
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
          "id": "53b432fd5886653c3cea800bc01ba073f5041aa5",
          "message": "Fix snapshotter startup when config is missing\n\nIf the default config file is missing, the snapshotter should start up\nand use the default parsed config file. If the user explicitly passes a\nconfig file with `--config` that's different than the default, then that\nsnapshotter will not start if it is missing.\n\nSigned-off-by: Kern Walster <walster@amazon.com>",
          "timestamp": "2025-07-01T10:41:02-07:00",
          "tree_id": "0fc95c28b6a6287b387098c1e2e88f00bfc86072",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/53b432fd5886653c3cea800bc01ba073f5041aa5"
        },
        "date": 1751392138798,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 10.9995,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 9.322,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 0.935,
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
          "id": "8b2bf80a852b68e7bdc24fcb5d35772566376ada",
          "message": "Remove image if rebase or initial fetch fails\n\nIf an image is unable to be claimed, the associated image pull with this\nrequest will fail, so it would make sense to also remove the rest of the\njobs associated with this image.\n\nAdditionally, if the initial authorization fetch fails, we should also\nremove the image from unpackJobs to avoid blocking future image pulls.\n\nSigned-off-by: David Son <davbson@amazon.com>",
          "timestamp": "2025-07-09T15:57:56-07:00",
          "tree_id": "5e695f026412da4546dcde51c6f53a237a06c413",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/8b2bf80a852b68e7bdc24fcb5d35772566376ada"
        },
        "date": 1752102432692,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 14.123000000000001,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 9.3215,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 4.2885,
            "unit": "Seconds",
            "extra": "P90"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "sbora@amazon.com",
            "name": "Swagat Bora",
            "username": "swagatbora90"
          },
          "committer": {
            "email": "swagatbora90@gmail.com",
            "name": "Swagat Bora",
            "username": "swagatbora90"
          },
          "distinct": true,
          "id": "8d8d1200b8eba2daf31a95ce8585a44a553d91cc",
          "message": "handle failures while commiting duplicate layer ztocs\n\nSigned-off-by: Swagat Bora <sbora@amazon.com>",
          "timestamp": "2025-07-14T14:01:43-07:00",
          "tree_id": "2faeda2ade27fc8e9196a8b17f8dc0988618ba0e",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/8d8d1200b8eba2daf31a95ce8585a44a553d91cc"
        },
        "date": 1752527459515,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 13.7695,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 9.368500000000001,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.529,
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
          "id": "73dd46eed7a77f5f5b454eaeb1c227fb5efab320",
          "message": "Hard-fail on config parsing errors\n\nSigned-off-by: David Son <davbson@amazon.com>",
          "timestamp": "2025-07-21T11:27:20-07:00",
          "tree_id": "8001452945121a6ee88368ec2794c0b4079fd070",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/73dd46eed7a77f5f5b454eaeb1c227fb5efab320"
        },
        "date": 1753122973921,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 12.1725,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 10.0405,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 4.1445,
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
          "id": "790a445854406da5b174b267104be43a01bd37ec",
          "message": "Only instantiate parallel pull structs if enabled\n\nSigned-off-by: David Son <davbson@amazon.com>",
          "timestamp": "2025-07-22T14:54:15-07:00",
          "tree_id": "554d7fb68d9252d04c2f4f941376a403be1d0df7",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/790a445854406da5b174b267104be43a01bd37ec"
        },
        "date": 1753221851782,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 14.34,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 10.275,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.087,
            "unit": "Seconds",
            "extra": "P90"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "erezz@amazon.com",
            "name": "Erez Zarum",
            "username": "erezzarum"
          },
          "committer": {
            "email": "55555210+sondavidb@users.noreply.github.com",
            "name": "David Son",
            "username": "sondavidb"
          },
          "distinct": true,
          "id": "45def2dcf87781de75634f5e2744b73fdb2dd8aa",
          "message": "script to setup SOCI parallel mode\n\nSigned-off-by: Erez Zarum <erezz@amazon.com>",
          "timestamp": "2025-07-23T13:14:55-07:00",
          "tree_id": "c240bf5ca72c2e69e5c4f7ab0045d374ee924da9",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/45def2dcf87781de75634f5e2744b73fdb2dd8aa"
        },
        "date": 1753302274671,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 14.631,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 9.7125,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.0725,
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
            "email": "55555210+sondavidb@users.noreply.github.com",
            "name": "David Son",
            "username": "sondavidb"
          },
          "distinct": true,
          "id": "56f893d87b5eb52d04206992c3edcef1fc4aa182",
          "message": "Bump dependencies using scripts/bump-deps.sh\n\nSigned-off-by: github-actions[bot] <41898282+github-actions[bot]@users.noreply.github.com>",
          "timestamp": "2025-07-29T15:27:37-07:00",
          "tree_id": "47dbb4aed0309102e0bec378ea50678bd81416f2",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/56f893d87b5eb52d04206992c3edcef1fc4aa182"
        },
        "date": 1753828580975,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 11.857500000000002,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 9.637,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.627,
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
          "id": "f698d94a448f4dd4152bc8695ce846e68bab30ad",
          "message": "Move the snapshotter to urfave\n\nBefore this change, the SOCI CLI used urfave CLI, but the snapshotter\nused go's flag package. This moved the snapshotter to urfave as well.\n\nSigned-off-by: Kern Walster <walster@amazon.com>",
          "timestamp": "2025-07-30T10:21:35-07:00",
          "tree_id": "9dfb8fe5cd1914660894cc025edca15a69533493",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/f698d94a448f4dd4152bc8695ce846e68bab30ad"
        },
        "date": 1753896571429,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 12.1705,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 10.2165,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 0.9219999999999999,
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
          "id": "70d0a37970ebd201c0226a8e177d27a92de2c260",
          "message": "Add integration test for config paths\n\nSigned-off-by: David Son <davbson@amazon.com>",
          "timestamp": "2025-07-30T15:22:36-07:00",
          "tree_id": "326218b1f1e13434acd1d19eec62eda24cc7ceaa",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/70d0a37970ebd201c0226a8e177d27a92de2c260"
        },
        "date": 1753914633494,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 11.276,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 9.485,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.0325,
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
          "id": "f94b989ab5fdd473e7afac45151f3d229a035285",
          "message": "Use common IsErrAlreadyExists for Push errors\n\ncontainerd and ORAS both return a var named ErrAlreadyExists when\npushing to their respective local content stores. This leads to some\npretty cumbersome error checking cases. This change should make it a\nlittle easier to work with.\n\nThis also changes the push behavior to return an error if content\nalready exists. This is to match the ORAS behavior for if content\nalready exists and relies on the caller to catch this error.\n\nSigned-off-by: David Son <davbson@amazon.com>",
          "timestamp": "2025-07-30T15:22:22-07:00",
          "tree_id": "313b3feed38c71823f6b96326015526e0b361d0f",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/f94b989ab5fdd473e7afac45151f3d229a035285"
        },
        "date": 1753914643329,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 12.0445,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 9.466,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.4409999999999998,
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
            "email": "55555210+sondavidb@users.noreply.github.com",
            "name": "David Son",
            "username": "sondavidb"
          },
          "distinct": true,
          "id": "6d62f0b50968dc378576edd6b82eaa876b2f4e24",
          "message": "Bump dependencies using scripts/bump-deps.sh\n\nSigned-off-by: github-actions[bot] <41898282+github-actions[bot]@users.noreply.github.com>",
          "timestamp": "2025-08-05T11:13:32-07:00",
          "tree_id": "c5f852b9ffb81b8eeaa5e4ce432332ec8f8ff790",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/6d62f0b50968dc378576edd6b82eaa876b2f4e24"
        },
        "date": 1754418234334,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 12.998000000000001,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 9.745000000000001,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.5975000000000001,
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
            "email": "kern.walster@gmail.com",
            "name": "Kern Walster",
            "username": "Kern--"
          },
          "distinct": true,
          "id": "3008f273c357b986dbd571fdd709d7d4b6921881",
          "message": "Use -q flag when pulling with nerdctl in tests\n\nThis should hopefully greatly reduce the number of log lines we get.\n\nSigned-off-by: David Son <davbson@amazon.com>",
          "timestamp": "2025-08-05T14:50:23-07:00",
          "tree_id": "60f1ba9189bb5598a12ec25147fc8095872585d8",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/3008f273c357b986dbd571fdd709d7d4b6921881"
        },
        "date": 1754431132898,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 11.315000000000001,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 9.4635,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.58,
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
            "email": "kern.walster@gmail.com",
            "name": "Kern Walster",
            "username": "Kern--"
          },
          "distinct": true,
          "id": "a998022a174bc2ddc6c258c6d880f663c9664b2a",
          "message": "Update dependabot.yml and remove custom workflow\n\nSince June 25 2024, dependabot has added multi-directory support for\ndependencies. As the existing bump-deps.yml was meant as a workaround\nfor this, we can safely remove it and rely solely on dependabot instead.\n\nSigned-off-by: David Son <davbson@amazon.com>",
          "timestamp": "2025-08-05T14:49:51-07:00",
          "tree_id": "ad979db0e6230edc269950a46486bbfe9aa8fdce",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/a998022a174bc2ddc6c258c6d880f663c9664b2a"
        },
        "date": 1754431140213,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 11.905,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 9.6065,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.634,
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
            "email": "kern.walster@gmail.com",
            "name": "Kern Walster",
            "username": "Kern--"
          },
          "distinct": true,
          "id": "5570f817ea39b5c4d73f8fce3efaa80cbb1f2074",
          "message": "Enable copyloopvar rule in Go linter\n\nSigned-off-by: David Son <davbson@amazon.com>",
          "timestamp": "2025-08-05T14:50:46-07:00",
          "tree_id": "cb554bad5677b5e696d3b197f813b2e6d03f55c2",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/5570f817ea39b5c4d73f8fce3efaa80cbb1f2074"
        },
        "date": 1754431164594,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 11.376,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 9.541,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.763,
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
            "email": "kern.walster@gmail.com",
            "name": "Kern Walster",
            "username": "Kern--"
          },
          "distinct": true,
          "id": "bdd630e2f63811782606ddfce57cf4cab783663b",
          "message": "Move to math/rand/v2 and remove containerd/seed\n\nGo 1.22 introduced math/rand/v2, which introduced breaking changes from\nthe math/rand library. Notably, it removed ambiguity on who seeds the\nRNG used for global random number calls. By moving away from this\nlibrary we can remove our usage of containerd/seed, which has been\ndeprecated anyway.\n\nThis process included moving away from ThreadsafeRandom in our testutil\npackage. The idea made sense (we want to seed the RNG but make it\nthreadsafe) but in practice it never really worked properly since the\norder the tests run in aren't really deterministic, and if it's not\ndeterministic then there's no reason to seed it to begin with. By\nleveraging the new rand.Source and ensuring we create a new Random\nvariable for every instance we need it, we can achieve the actual end\ngoal of deterministic randomness in our testing suite.\n\nPart of this change also included ensuring all of our calls to TestRand\nare tied to a TestRand object, which removes the ambiguity present in\nsome of our other functions (some would just use the global rand while\nothers created a new ThreadsafeRandom in place) and allows the caller\nto control the amount of randomness they wish.\n\nWith that said, I did make a best effort to ensure that we are always\ncreating new TestRand calls within single-threaded operations, but it's\npossible that I missed a couple cases.\n\nThere's still a couple of considerations even after this commit:\n\n1. We still use the global RNG in two places: jitter() which is used for\n   our HTTP backoff strategy, and generateUniqueString(), which is used\n   for our parallel pull mode. I can't imagine either is particularly\n   impactful, but it's still worth noting that we are not making that\n   deterministic within our tests.\n2. If we consume any libraries that still use math/rand, we might lose\n   reproducible behavior as Go 1.20 has math/rand calls be\n   nondeterministic unless it is seeded, which we always did.\n\nIn spite of these caveats, this is hopefully still a net positive :)\n\nSigned-off-by: David Son <davbson@amazon.com>",
          "timestamp": "2025-08-05T15:42:08-07:00",
          "tree_id": "ade9bc937fb757f8ca36d7022acadba2021302d2",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/bdd630e2f63811782606ddfce57cf4cab783663b"
        },
        "date": 1754434252552,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 11.527000000000001,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 9.455,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.612,
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
          "id": "45a548c076b82d883b887e87a014af0f8fe2871a",
          "message": "Reduce integration test log noise\n\nThis change reduces the integration test log volume by only printing\nlogs when a test fails. Even when the tests are run with `go test -v`,\nthe logs from docker compose, soci, and all the shell commands will be\nsurpressed unless the test fails.\n\nSigned-off-by: Kern Walster <walster@amazon.com>",
          "timestamp": "2025-08-06T14:56:36-07:00",
          "tree_id": "691aaaa3d1bc0cc95f666d57848cb4bedc1bc403",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/45a548c076b82d883b887e87a014af0f8fe2871a"
        },
        "date": 1754517918302,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 14.053,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 9.386,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 1.1135000000000002,
            "unit": "Seconds",
            "extra": "P90"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "austin.vazquez.dev@gmail.com",
            "name": "Austin Vazquez",
            "username": "austinvazquez"
          },
          "committer": {
            "email": "kern.walster@gmail.com",
            "name": "Kern Walster",
            "username": "Kern--"
          },
          "distinct": true,
          "id": "c8bc2dcc8b596075be5769c3800e3c2ecdd86166",
          "message": "Migrate soci to urfave/cli/v2\n\nSigned-off-by: Austin Vazquez <austin.vazquez.dev@gmail.com>\nCo-authored-by: Kern Walster <walster@amazon.com>",
          "timestamp": "2025-08-07T16:01:26-07:00",
          "tree_id": "f25d6126f42a6754e6858482a642ebde71b091aa",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/c8bc2dcc8b596075be5769c3800e3c2ecdd86166"
        },
        "date": 1754608165504,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 11.7005,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 9.6735,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 0.9875,
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
          "id": "6036fdc455e1f778d79b99eae1064d677f923aca",
          "message": "Revert \"Update dependabot.yml and remove custom workflow\"\n\nThis reverts commit a998022a174bc2ddc6c258c6d880f663c9664b2a.\n\nUnfortunately dependabot does not recursively run go mod tidy for nested\ngomodules (e.g.: https://github.com/awslabs/soci-snapshotter/pull/1673)\n\nThere is currently an issue open for this, once it is resolved we can\nprobably un-revert this.\n\nIssue: https://github.com/dependabot/dependabot-core/issues/11046\n\nSigned-off-by: David Son <davbson@amazon.com>",
          "timestamp": "2025-08-13T12:53:08-04:00",
          "tree_id": "39f37dd784bce63bccf61c42e303550420d02bc3",
          "url": "https://github.com/awslabs/soci-snapshotter/commit/6036fdc455e1f778d79b99eae1064d677f923aca"
        },
        "date": 1755104469476,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "SociFullECR-public-rabbitmq-lazyTaskDuration",
            "value": 11.747499999999999,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-localTaskDuration",
            "value": 9.982,
            "unit": "Seconds",
            "extra": "P90"
          },
          {
            "name": "SociFullECR-public-rabbitmq-pullTaskDuration",
            "value": 0.8855,
            "unit": "Seconds",
            "extra": "P90"
          }
        ]
      }
    ]
  }
}