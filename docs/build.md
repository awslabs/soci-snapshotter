# Build SOCI from source

This document is helpful if you plan to contribute to the project (thanks!) or
want to use the latest version of either `soci-snapshotter-grpc` or `soci` CLI 
in the main branch.

This document includes the following sections:

<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->

- [Dependencies](#dependencies)
- [Build SOCI](#build-soci)
- [Test SOCI](#test-soci)
- [(Optional) Contribute your change](#optional-contribute-your-change)
- [Development tooling](#development-tooling)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

## Dependencies

The project binaries have the following dependencies. Please follow the links or commands
to install them on your machine:

> **Note**
> We only mention the direct dependencies of the project. Some dependencies may
> have their own dependencies (e.g., containerd depends on runc/cni). Please refer
> to their doc for a complete installation guide (mainly containerd).

- **[go](https://go.dev/doc/install) >= 1.22** - required to build the project;
to confirm please check with `go version`.
- **[containerd](https://github.com/containerd/containerd/blob/main/docs/getting-started.md) >= 1.4** -
required to run the SOCI snapshotter; to confirm please check with `sudo containerd --version`.
- **fuse** - used for mounting without root access (`sudo yum install fuse`).
- **zlib** - used for decompression and ztoc creation; Both the CLI and the SOCI snapshotter build zlib statically
(`sudo yum install zlib-devel zlib-static`).
- **gcc** - used for compiling C code, gzip's zinfo implementation (`sudo yum install gcc`).
- **[flatc](https://github.com/google/flatbuffers)** - used for compiling zTOC
flatbuffer file and generating corresponding Go code.

For fuse/zlib/gcc, they can be installed by your Linux package manager (e.g., `yum` or `apt-get`).

For flatc, you can download and install a [release](https://github.com/google/flatbuffers/releases)
into your `/usr/local` (or other `$PATH`) directory. For example:

```shell
wget -c https://github.com/google/flatbuffers/releases/download/v23.3.3/Linux.flatc.binary.g++-10.zip
sudo unzip Linux.flatc.binary.g++-10.zip -d /usr/local
rm Linux.flatc.binary.g++-10.zip
```

## Build SOCI

First you need `git` to clone the repository (if you intend to contribute, you
can fork the repository and clone your own fork):

```shell
git clone https://github.com/awslabs/soci-snapshotter.git
cd soci-snapshotter
```

SOCI uses `make` as the build tool. Assuming you're in the root directory
of the repository, you can build the CLI and the snapshotter by running:

```shell
make
```

This builds the project binaries into the `./out` directory. You can install them
to a `PATH` directory (`/usr/local/bin`) with:

```shell
sudo make install
# check to make sure the SOCI CLI can be found in PATH
sudo soci --help
# check to make sure the SOCI snapshotter can be found in PATH
sudo soci-snapshotter-grpc --help
```

When changing the zTOC flatbuffer definition, you need to regenerate the generated
code package with:

> It's rare to make such a change, especially delete a field which is a breaking
> change and discouraged by flatbuffers.

```shell
make flatc
```

## Test SOCI

We have unit tests and integration tests as part of our automated CI, as well as
benchmark tests that can be used to test the performance of the SOCI snapshotter. You
can run these tests using the following `Makefile` targets:

- `make test`: run all unit tests.
- `make integration`: run all integration tests.

### Benchmark SOCI
We now have a benchmark framework available at [SOCI Benchmarking](/docs/benchmark.md)


To speed up develop-test cycle, you can run individual test(s) by utilizing `go test`'s
`-run` flag. For example, suppose you only want to run a test named `TestFooBar`, you can:

```shell
# 1. if TestFooBar is a unit test
GO_TEST_FLAGS="-run TestFooBar" make test

# 2. if TestFooBar is an integration test
GO_TEST_FLAGS="-run TestFooBar" make integration
```

## (Optional) Contribute your change

If you intend to contribute your change, you need to validate your changes pass
all unit/integration tests. (i.e., `make test` and `make integration` pass).

Meanwhile, there are a few steps you should follow to ensure your change is ready
for review:

1. If you added any new files, make sure they contain the SOCI license header. We
provide a script (`./scripts/add-ltag.sh`) that can do this.

2. Make sure your change is well-formatted and you've run `gofmt`.

3. Make sure your commit is signed (`git commit -s`).

4. As a final step, run `make check` to verify your change passes these checks.

> **Note**
> `make check` requires some checking tools (`golangci`, `ltag`,
> `git-validation`). We provide a script (`./scripts/install-check-tools.sh`) to
> help install all these checking tools.

Once you pass all the tests and checks. You're ready to make your PR!

## Development tooling

This repository contains two go modules, one in the root directory and the other in [`cmd`](../cmd). To describe this arrangement to tools like `gopls` (and, by extension, vscode), you need a `go.work` file listing the module locations. An example such file is included in this repository as `go.work.example` which you could rename to `go.work` to use as-is.
