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
UTIL_CFLAGS=-I${CURDIR}/c -L${OUTDIR} -lz
UTIL_LDFLAGS=-L${OUTDIR} -lz
PKG=github.com/awslabs/soci-snapshotter
VERSION=$(shell git describe --match 'v[0-9]*' --dirty='.m' --always --tags)
REVISION=$(shell git rev-parse HEAD)$(shell if ! git diff --no-ext-diff --quiet --exit-code; then echo .m; fi)
GO_LD_FLAGS=-ldflags '-s -w -X $(PKG)/version.Version=$(VERSION) -X $(PKG)/version.Revision=$(REVISION) $(GO_EXTRA_LDFLAGS)'
SOCI_SNAPSHOTTER_PROJECT_ROOT ?= $(shell pwd)
LTAG_TEMPLATE_FLAG=-t ./.headers
FBS_FILE_PATH=$(CURDIR)/ztoc/fbs/ztoc.fbs
COMMIT=$(shell git rev-parse HEAD)
STARGZ_BINARY?=/usr/local/bin/containerd-stargz-grpc

CMD=soci-snapshotter-grpc soci

CMD_BINARIES=$(addprefix $(OUTDIR)/,$(CMD))

.PHONY: all build check check-ltag check-dco check-lint check-flatc install-check-tools add-ltag install install-cmake install-flatc install-zlib uninstall clean test integration

all: build

build: $(CMD)

FORCE:

soci-snapshotter-grpc: FORCE
	cd cmd/ ; GO111MODULE=$(GO111MODULE_VALUE) go build -o $(OUTDIR)/$@ $(GO_BUILD_FLAGS) $(GO_LD_FLAGS) -v ./soci-snapshotter-grpc

soci: FORCE
	cd cmd/ ; GO111MODULE=$(GO111MODULE_VALUE) go build -o $(OUTDIR)/$@ $(GO_BUILD_FLAGS) $(GO_LD_FLAGS) -v ./soci


install-cmake:
	@wget https://github.com/Kitware/CMake/releases/download/v3.24.1/cmake-3.24.1-Linux-x86_64.sh -O cmake.sh
	@sh cmake.sh --prefix=/usr/local/ --exclude-subdir
	@rm -rf cmake.sh

install-flatc:
	wget https://github.com/google/flatbuffers/archive/refs/tags/v2.0.8.tar.gz -O flatbuffers.tar.gz
	tar xzvf flatbuffers.tar.gz
	cd flatbuffers-2.0.8 && cmake -G "Unix Makefiles" -DCMAKE_BUILD_TYPE=Release && make && make install
	rm -f flatbuffers.tar.gz
	rm -rf flatbuffers-2.0.8

install-zlib:
	@wget https://zlib.net/fossils/zlib-1.2.12.tar.gz
	@tar xzvf zlib-1.2.12.tar.gz
	@cd zlib-1.2.12; ./configure; sudo make install
	@rm -rf zlib-1.2.12
	@rm -f zlib-1.2.12.tar.gz

check: check-ltag check-dco check-lint check-flatc

flatc:
	rm -rf $(CURDIR)/ztoc/fbs/ztoc
	flatc -o $(CURDIR)/ztoc/fbs -g $(FBS_FILE_PATH)

# check if flatbuffers needs to be generated again
check-flatc:
	$(eval TMPDIR := $(shell mktemp -d))
	flatc -o $(TMPDIR) -g $(FBS_FILE_PATH)
	diff -qr $(TMPDIR)/ztoc $(CURDIR)/ztoc/fbs/ztoc || (printf "\n\nThe Ztoc schema seems to be modified. Please run 'make flatc' to re-generate Go files\n\n"; exit 1)
	rm -rf $(TMPDIR)

check-lint: 
	GO111MODULE=$(GO111MODULE_VALUE) $(shell go env GOPATH)/bin/golangci-lint run
	cd ./cmd ; GO111MODULE=$(GO111MODULE_VALUE) $(shell go env GOPATH)/bin/golangci-lint run

check-ltag:
	$(shell go env GOPATH)/bin/ltag $(LTAG_TEMPLATE_FLAG) -check -v || (echo "The files listed above are missing a licence header. Please run make add-ltag"; exit 1)

# the very first auto-commit doesn't have a DCO and the first real commit has a slightly different format. Exclude those when doing the check.
check-dco:
	$(shell go env GOPATH)/bin/git-validation -run DCO -range HEAD~20..HEAD

install-check-tools:
	@curl -sfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(shell go env GOPATH)/bin v1.49.0
	go install github.com/kunalkushwaha/ltag@v0.2.3
	go install github.com/vbatts/git-validation@v1.1.0

install:
	@echo "$@"
	@mkdir -p $(CMD_DESTDIR)/bin
	@install $(CMD_BINARIES) $(CMD_DESTDIR)/bin

uninstall:
	@echo "$@"
	@rm -f $(addprefix $(CMD_DESTDIR)/bin/,$(notdir $(CMD_BINARIES)))

add-ltag:
	$(shell go env GOPATH)/bin/ltag $(LTAG_TEMPLATE_FLAG) -v

clean:
	rm -rf $(OUTDIR)

vendor:
	@GO111MODULE=$(GO111MODULE_VALUE) go mod tidy
	@cd ./cmd ; GO111MODULE=$(GO111MODULE_VALUE) go mod tidy

test:
	@echo "$@"
	@GO111MODULE=$(GO111MODULE_VALUE) go test -race ./...
	@cd ./cmd/soci ; GO111MODULE=$(GO111MODULE_VALUE) go test -timeout 20m -race ./...

integration: 
	@echo "$@"
	@echo "SOCI_SNAPSHOTTER_PROJECT_ROOT=$(SOCI_SNAPSHOTTER_PROJECT_ROOT)"
	@GO111MODULE=$(GO111MODULE_VALUE) SOCI_SNAPSHOTTER_PROJECT_ROOT=$(SOCI_SNAPSHOTTER_PROJECT_ROOT) ENABLE_INTEGRATION_TEST=true go test $(GO_TEST_FLAGS) -v -timeout=0 ./integration

benchmarks:
	@echo "$@"
	@cd benchmark/performanceTest ; GO111MODULE=$(GO111MODULE_VALUE) go build -o ../bin/PerfTests . && sudo ../bin/PerfTests $(COMMIT) ../singleImage.csv 10

benchmarks-comp:
	@echo "$@"
	@cd benchmark/comparisonTest ; GO111MODULE=$(GO111MODULE_VALUE) go build -o ../bin/CompTests . && sudo ../bin/CompTests $(COMMIT) ../singleImage.csv 10

benchmarks-stargz:
	@echo "$@"
	@cd benchmark/stargzTest ; GO111MODULE=$(GO111MODULE_VALUE) go build -o ../bin/StargzTests . && sudo ../bin/StargzTests $(COMMIT) ../singleImage.csv 10 $(STARGZ_BINARY)

benchmarks-parser:
	@echo "$@"
	@cd benchmark/parser ; GO111MODULE=$(GO111MODULE_VALUE) go build -o ../bin/Parser .
