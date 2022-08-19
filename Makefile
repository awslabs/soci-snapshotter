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
UTIL_CFLAGS=-I${CURDIR}/c -L${OUTDIR} -lindexer -lz
UTIL_LDFLAGS=-L${OUTDIR} -lindexer -lz
PKG=github.com/awslabs/soci-snapshotter
VERSION=$(shell git describe --match 'v[0-9]*' --dirty='.m' --always --tags)
REVISION=$(shell git rev-parse HEAD)$(shell if ! git diff --no-ext-diff --quiet --exit-code; then echo .m; fi)
GO_LD_FLAGS=-ldflags '-s -w -X $(PKG)/version.Version=$(VERSION) -X $(PKG)/version.Revision=$(REVISION) $(GO_EXTRA_LDFLAGS)'
SOCI_SNAPSHOTTER_PROJECT_ROOT ?= $(shell pwd)

CMD=soci-snapshotter-grpc soci

CMD_BINARIES=$(addprefix $(OUTDIR)/,$(CMD))

.PHONY: all build check check-ltag check-dco install-check-tools install uninstall clean test test-root test-all integration test-optimize benchmark test-kind test-cri-containerd test-cri-o test-criauth generate validate-generated test-k3s test-k3s-argo-workflow vendor

all: build

build: pre-build $(CMD)

FORCE:

soci-snapshotter-grpc: FORCE
	cd cmd/ ; GO111MODULE=$(GO111MODULE_VALUE) go build -o $(OUTDIR)/$@ $(GO_BUILD_FLAGS) $(GO_LD_FLAGS) -v ./soci-snapshotter-grpc

soci: FORCE
	cd cmd/ ; GO111MODULE=$(GO111MODULE_VALUE) go build -o $(OUTDIR)/$@ $(GO_BUILD_FLAGS) $(GO_LD_FLAGS) -v ./soci

soci_brewer:
	cd cmd; go build -o ${OUTDIR}/$@ ${BUILD_FLAGS} ${LD_FLAGS} ./soci_brewer.go

pre-build:
	rm -rf ${OUTDIR}
	@mkdir -p ${OUTDIR}
	@gcc -c c/indexer.c -o ${OUTDIR}/indexer.o -O3 -Wall -Werror
	@ar rvs ${OUTDIR}/libindexer.a ${OUTDIR}/indexer.o
	@rm -f ${OUTDIR}/indexer.o

install-zlib:
	@wget https://zlib.net/fossils/zlib-1.2.12.tar.gz
	@tar xzvf zlib-1.2.12.tar.gz
	@cd zlib-1.2.12; ./configure; sudo make install
	@rm -rf zlib-1.2.12
	@rm -f zlib-1.2.12.tar.gz

# "check" depends "build". out/libindexer.a seems needed to process cgo directives
check: build check-ltag check-dco
	GO111MODULE=$(GO111MODULE_VALUE) $(shell go env GOPATH)/bin/golangci-lint run
	cd ./cmd ; GO111MODULE=$(GO111MODULE_VALUE) $(shell go env GOPATH)/bin/golangci-lint run

check-ltag:
	$(shell go env GOPATH)/bin/ltag -t ./.headers -check -v || (echo "The files listed above are missing a licence header"; exit 1)

# the very first auto-commit doesn't have a DCO and the first real commit has a slightly different format. Exclude those when doing the check.
check-dco:
	$(shell go env GOPATH)/bin/git-validation -run DCO -range 1628d6eac6cb9383f9538d0bb85de8a007b4f9a3..HEAD

install-check-tools:
	@curl -sfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(shell go env GOPATH)/bin v1.45.2
	go install github.com/kunalkushwaha/ltag@v0.2.3
	go install github.com/vbatts/git-validation@v1.1.0

install:
	@echo "$@"
	@mkdir -p $(CMD_DESTDIR)/bin
	@install $(CMD_BINARIES) $(CMD_DESTDIR)/bin

uninstall:
	@echo "$@"
	@rm -f $(addprefix $(CMD_DESTDIR)/bin/,$(notdir $(CMD_BINARIES)))

clean:
	rm -rf $(OUTDIR)

generate:
	@./script/generated-files/generate.sh update

validate-generated:
	@./script/generated-files/generate.sh validate

vendor:
	@GO111MODULE=$(GO111MODULE_VALUE) go mod tidy
	@cd ./cmd ; GO111MODULE=$(GO111MODULE_VALUE) go mod tidy

test:
	@echo "$@"
	@GO111MODULE=$(GO111MODULE_VALUE) go test -race ./...
	@cd ./cmd/soci ; GO111MODULE=$(GO111MODULE_VALUE) go test -timeout 20m -race ./...

test-root:
	@echo "$@"
	@GO111MODULE=$(GO111MODULE_VALUE) go test -race ./snapshot -test.root

test-all: test-root test

integration:
	@echo "$@"
	@echo "SOCI_SNAPSHOTTER_PROJECT_ROOT=$(SOCI_SNAPSHOTTER_PROJECT_ROOT)"
	@GO111MODULE=$(GO111MODULE_VALUE) SOCI_SNAPSHOTTER_PROJECT_ROOT=$(SOCI_SNAPSHOTTER_PROJECT_ROOT) ENABLE_INTEGRATION_TEST=true go test $(GO_TEST_FLAGS) -v -timeout=0 ./integration

benchmark:
	@./script/benchmark/test.sh
