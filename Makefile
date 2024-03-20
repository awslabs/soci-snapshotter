#   Copyright The containerd Authors.

#   Licensed under the Apache License, Version 2.0 (the "License");
#   you may not use this file except in compliance with the License.
#   You may obtain a copy of the License at

#       http://www.apache.org/licenses/LICENSE-2.0

#   Unless required by applicable law or agreed to in writing, software
#   distributed under the License is distributed on an "AS IS" BASIS,
#   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
#   See the License for the specific language governing permissions and
#   limitations under the License.


# Base path used to install.
CMD_DESTDIR ?= /usr/local
GO111MODULE_VALUE=auto
OUTDIR ?= $(CURDIR)/out
PKG=github.com/awslabs/soci-snapshotter
VERSION=$(shell git describe --match 'v[0-9]*' --dirty='.m' --always --tags)
REVISION=$(shell git rev-parse HEAD)$(shell if ! git diff --no-ext-diff --quiet --exit-code; then echo .m; fi)

GO_BUILDTAGS ?=
ifneq ($(STATIC),)
	GO_BUILDTAGS += osusergo netgo static_build
endif
GO_TAGS=$(if $(GO_BUILDTAGS),-tags "$(strip $(GO_BUILDTAGS))",)

GO_LD_FLAGS=-ldflags '-s -w -X $(PKG)/version.Version=$(VERSION) -X $(PKG)/version.Revision=$(REVISION) $(GO_EXTRA_LDFLAGS)
ifneq ($(STATIC),)
	GO_LD_FLAGS += -extldflags "-static"
endif
GO_LD_FLAGS+='

SOCI_SNAPSHOTTER_PROJECT_ROOT ?= $(shell pwd)
LTAG_TEMPLATE_FLAG=-t ./.headers
FBS_FILE_PATH=$(CURDIR)/ztoc/fbs/ztoc.fbs
FBS_FILE_PATH_COMPRESSION=$(CURDIR)/ztoc/compression/fbs/zinfo.fbs
COMMIT=$(shell git rev-parse HEAD)
STARGZ_BINARY?=/usr/local/bin/containerd-stargz-grpc

CMD=soci-snapshotter-grpc soci

CMD_BINARIES=$(addprefix $(OUTDIR)/,$(CMD))

GO_BENCHMARK_TESTS?=.

.PHONY: all build check add-ltag install uninstall tidy vendor clean \
	test integration release benchmarks build-benchmarks \
	benchmarks-perf-test benchmarks-comparison-test

all: build

build: $(CMD)

FORCE:

soci-snapshotter-grpc: FORCE
	cd cmd/ ; GO111MODULE=$(GO111MODULE_VALUE) go build -o $(OUTDIR)/$@ $(GO_BUILD_FLAGS) $(GO_LD_FLAGS) $(GO_TAGS) ./soci-snapshotter-grpc

soci: FORCE
	cd cmd/ ; GO111MODULE=$(GO111MODULE_VALUE) go build -o $(OUTDIR)/$@ $(GO_BUILD_FLAGS) $(GO_LD_FLAGS) $(GO_TAGS) ./soci

check:
	cd scripts/ ; ./check-all.sh

flatc:
	rm -rf $(CURDIR)/ztoc/fbs/ztoc
	flatc -o $(CURDIR)/ztoc/fbs -g $(FBS_FILE_PATH)
	rm -rf $(CURDIR)/ztoc/compression/fbs/zinfo
	flatc -o $(CURDIR)/ztoc/compression/fbs -g $(FBS_FILE_PATH_COMPRESSION)

install:
	@echo "$@"
	@mkdir -p $(CMD_DESTDIR)/bin
	@install $(CMD_BINARIES) $(CMD_DESTDIR)/bin

uninstall:
	@echo "$@"
	@rm -f $(addprefix $(CMD_DESTDIR)/bin/,$(notdir $(CMD_BINARIES)))

clean:
	rm -rf $(OUTDIR)

tidy:
	@GO111MODULE=$(GO111MODULE_VALUE) go mod tidy
	@cd ./cmd ; GO111MODULE=$(GO111MODULE_VALUE) go mod tidy

vendor:
	@GO111MODULE=$(GO111MODULE_VALUE) go mod vendor
	@cd ./cmd ; GO111MODULE=$(GO111MODULE_VALUE) go mod vendor

test:
	@echo "$@"
	@GO111MODULE=$(GO111MODULE_VALUE) go test $(GO_TEST_FLAGS) $(GO_LD_FLAGS) -race ./...


integration: build
	@echo "$@"
	@echo "SOCI_SNAPSHOTTER_PROJECT_ROOT=$(SOCI_SNAPSHOTTER_PROJECT_ROOT)"
	@GO111MODULE=$(GO111MODULE_VALUE) SOCI_SNAPSHOTTER_PROJECT_ROOT=$(SOCI_SNAPSHOTTER_PROJECT_ROOT) ENABLE_INTEGRATION_TEST=true go test $(GO_TEST_FLAGS) -v -timeout=0 ./integration

release:
	@echo "$@"
	@$(SOCI_SNAPSHOTTER_PROJECT_ROOT)/scripts/create-releases.sh $(RELEASE_TAG)

go-benchmarks:
    # -run matches TestXXX type functions. Setting it to ^$ ensures non-benchmark tests are not run
	go test -run=^$$ -bench=$(GO_BENCHMARK_TESTS) -benchmem $(GO_BENCHMARK_FLAGS) ./...

benchmarks: benchmarks-perf-test benchmarks-comparison-test

build-benchmarks: benchmark/bin/PerfTests benchmark/bin/CompTests

benchmark/bin/PerfTests: FORCE
	GO111MODULE=$(GO111MODULE_VALUE) go build -o $@ ./benchmark/performanceTest

benchmark/bin/CompTests: FORCE
	GO111MODULE=$(GO111MODULE_VALUE) go build -o $@ ./benchmark/comparisonTest

benchmarks-perf-test: benchmark/bin/PerfTests
	@echo "$@"
	@cd benchmark/performanceTest ; sudo ../bin/PerfTests -show-commit $(BENCHMARK_FLAGS)

benchmarks-comparison-test: benchmark/bin/CompTests
	@echo "$@"
	@cd benchmark/comparisonTest ; sudo ../bin/CompTests $(BENCHMARK_FLAGS)

benchmarks-stargz:
	@echo "$@"
	@cd benchmark/stargzTest ; GO111MODULE=$(GO111MODULE_VALUE) go build -o ../bin/StargzTests . && sudo ../bin/StargzTests $(COMMIT) ../singleImage.csv 10 $(STARGZ_BINARY)

benchmarks-parser:
	@echo "$@"
	@cd benchmark/parser ; GO111MODULE=$(GO111MODULE_VALUE) go build -o ../bin/Parser .
