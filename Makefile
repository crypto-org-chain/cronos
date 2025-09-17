BUILDDIR ?= $(CURDIR)/build
PACKAGES=$(shell go list ./... | grep -v '/simulation')
COVERAGE ?= coverage.txt

GOPATH ?= $(shell $(GO) env GOPATH)
BINDIR ?= ~/go/bin
NETWORK ?= mainnet
LEDGER_ENABLED ?= true
PROJECT_NAME = $(shell git remote get-url origin | xargs basename -s .git)

TESTNET_FLAGS ?=

VERSION := $(shell echo $(shell git describe --tags 2>/dev/null ) | sed 's/^v//')
COMMIT := $(shell git log -1 --format='%H')
DOCKER := $(shell which docker)

UNAME_S := $(shell uname -s)

GOLANGCI_VERSION := "2.1.6"

# process build tags
build_tags = netgo objstore pebbledb
ifeq ($(NETWORK),mainnet)
    build_tags += mainnet
else ifeq ($(NETWORK),testnet)
    build_tags += testnet
endif

ifeq ($(LEDGER_ENABLED),true)
    ifeq ($(OS),Windows_NT)
        GCCEXE = $(shell where gcc.exe 2> NUL)
        ifeq ($(GCCEXE),)
            $(error gcc.exe not installed for ledger support, please install or set LEDGER_ENABLED=false)
        else
            build_tags += ledger
        endif
    else
        ifeq ($(UNAME_S),OpenBSD)
            $(warning OpenBSD detected, disabling ledger support (https://github.com/cosmos/cosmos-sdk/issues/1988))
        else
            GCC = $(shell command -v gcc 2> /dev/null)
            ifeq ($(GCC),)
                $(error gcc not installed for ledger support, please install or set LEDGER_ENABLED=false)
            else
                build_tags += ledger
            endif
        endif
    endif
endif

ifeq ($(shell uname -s),Darwin)
    GEN_BINDING_FLAGS := --system x86_64-darwin
else
    ifeq ($(shell uname -s),Linux)
        GEN_BINDING_FLAGS := --system x86_64-linux
    endif
endif

# DB backend selection
ifeq (cleveldb,$(findstring cleveldb,$(COSMOS_BUILD_OPTIONS)))
  BUILD_TAGS += gcc
endif
ifeq (badgerdb,$(findstring badgerdb,$(COSMOS_BUILD_OPTIONS)))
  BUILD_TAGS += badgerdb
endif
# handle rocksdb
ifeq (rocksdb,$(findstring rocksdb,$(COSMOS_BUILD_OPTIONS)))
  CGO_ENABLED=1
  BUILD_TAGS += rocksdb grocksdb_clean_link
endif
# handle boltdb
ifeq (boltdb,$(findstring boltdb,$(COSMOS_BUILD_OPTIONS)))
  BUILD_TAGS += boltdb
endif

# nativebyteorder mode will panic on big endian machines
BUILD_TAGS += nativebyteorder

ifeq (,$(findstring nostrip,$(COSMOS_BUILD_OPTIONS)))
  ldflags += -w -s
endif
ldflags += $(LDFLAGS)
ldflags := $(strip $(ldflags))

build_tags += $(BUILD_TAGS)
build_tags := $(strip $(build_tags))

whitespace := $(subst ,, )
comma := ,
build_tags_comma_sep := $(subst $(whitespace),$(comma),$(build_tags))

# process linker flags
ldflags += -X github.com/cosmos/cosmos-sdk/version.Name=cronos \
	-X github.com/cosmos/cosmos-sdk/version.AppName=cronosd \
	-X github.com/cosmos/cosmos-sdk/version.Version=$(VERSION) \
	-X github.com/cosmos/cosmos-sdk/version.Commit=$(COMMIT) \
	-X github.com/cosmos/cosmos-sdk/version.BuildTags=$(build_tags_comma_sep)

BUILD_FLAGS := -tags "$(build_tags)" -ldflags '$(ldflags)'
# check for nostrip option
ifeq (,$(findstring nostrip,$(COSMOS_BUILD_OPTIONS)))
  BUILD_FLAGS += -trimpath
endif

all: build
build: check-network print-ledger go.sum
	@go build -mod=readonly $(BUILD_FLAGS) -o $(BUILDDIR)/cronosd ./cmd/cronosd

install: check-network print-ledger go.sum
	@go install -mod=readonly $(BUILD_FLAGS) ./cmd/cronosd

test: test-memiavl test-store
	@go test -tags=objstore -v -mod=readonly $(PACKAGES) -coverprofile=$(COVERAGE) -covermode=atomic

test-memiavl:
	@cd memiavl; go test -tags=objstore -v -mod=readonly ./... -coverprofile=$(COVERAGE) -covermode=atomic;

test-store:
	@cd store; go test -tags=objstore -v -mod=readonly ./... -coverprofile=$(COVERAGE) -covermode=atomic;

test-versiondb:
	@cd versiondb; go test -tags=objstore,rocksdb -v -mod=readonly ./... -coverprofile=$(COVERAGE) -covermode=atomic;

.PHONY: clean build install test test-memiavl test-store test-versiondb

clean:
	rm -rf $(BUILDDIR)/

###############################################################################
###                                Linting                                  ###
###############################################################################

lint-install:
	@echo "--> Installing golangci-lint $(GOLANGCI_VERSION)"
	@nix profile install -f ./nix golangci-lint

lint:
	go mod verify
	golangci-lint run --output.text.path stdout --path-prefix=./

lint-fix:
	golangci-lint run --fix --issues-exit-code=0 --path-prefix=./

lint-py:
	flake8 --show-source --count --statistics \
          --format="::error file=%(path)s,line=%(row)d,col=%(col)d::%(path)s:%(row)d:%(col)d: %(code)s %(text)s" \

lint-nix:
	find . -name "*.nix" ! -path './integration_tests/contracts/*' ! -path "./contracts/*" | xargs nixfmt -c

lint-nix-fix:
	find . -name "*.nix" ! -path './integration_tests/contracts/*' ! -path "./contracts/*" | xargs nixfmt

.PHONY: lint-install lint lint-fix lint-py lint-nix lint-nix-fix

###############################################################################
###                                Releasing                                ###
###############################################################################

release-dry-run:
	./scripts/release.sh

.PHONY: release-dry-run

###############################################################################
###                                Sim Test                                 ###
###############################################################################

SIMAPP = github.com/crypto-org-chain/cronos/v2/app

# Install the runsim binary with a temporary workaround of entering an outside
# directory as the "go get" command ignores the -mod option and will polute the
# go.{mod, sum} files.
#
# ref: https://github.com/golang/go/issues/30515
runsim: $(BINDIR)/runsim
$(BINDIR)/runsim:
	@echo "Installing runsim..."
	@(cd /tmp && go install github.com/cosmos/tools/cmd/runsim@v1.0.0)

test-sim-nondeterminism:
	@echo "Running non-determinism test..."
	@go test -tags=objstore -mod=readonly $(SIMAPP) -run TestAppStateDeterminism -Enabled=true \
		-NumBlocks=100 -BlockSize=200 -Commit=true -Period=0 -v -timeout 24h

test-sim-random-genesis-fast:
	@echo "Running random genesis simulation..."
	@go test -tags=objstore -mod=readonly $(SIMAPP) -run TestFullAppSimulation \
		-Enabled=true -NumBlocks=100 -BlockSize=200 -Commit=true -Seed=99 -Period=5 -v -timeout 24h

test-sim-import-export: export GOFLAGS=-tags=objstore
test-sim-import-export: runsim
	@echo "Running application import/export simulation. This may take several minutes..."
	@$(BINDIR)/runsim -Jobs=4 -SimAppPkg=$(SIMAPP) -ExitOnFail 50 5 TestAppImportExport

test-sim-after-import: export GOFLAGS=-tags=objstore
test-sim-after-import: runsim
	@echo "Running application simulation-after-import. This may take several minutes..."
	@$(BINDIR)/runsim -Jobs=4 -SimAppPkg=$(SIMAPP) -ExitOnFail 50 5 TestAppSimulationAfterImport

test-sim-custom-genesis-multi-seed: export GOFLAGS=-tags=objstore
test-sim-custom-genesis-multi-seed: runsim
	@echo "Running multi-seed custom genesis simulation..."
	@echo "By default, ${HOME}/.cronosd/config/genesis.json will be used."
	@$(BINDIR)/runsim -Genesis=${HOME}/.cronosd/config/genesis.json -SimAppPkg=$(SIMAPP) -ExitOnFail 400 5 TestFullAppSimulation

test-sim-multi-seed-long: export GOFLAGS=-tags=objstore
test-sim-multi-seed-long: runsim
	@echo "Running long multi-seed application simulation. This may take awhile!"
	@$(BINDIR)/runsim -Jobs=4 -SimAppPkg=$(SIMAPP) -ExitOnFail 500 50 TestFullAppSimulation

test-sim-multi-seed-short: export GOFLAGS=-tags=objstore
test-sim-multi-seed-short: runsim
	@echo "Running short multi-seed application simulation. This may take awhile!"
	@$(BINDIR)/runsim -Jobs=4 -SimAppPkg=$(SIMAPP) -ExitOnFail 50 10 TestFullAppSimulation

test-sim-benchmark-invariants:
	@echo "Running simulation invariant benchmarks..."
	@go test -tags=objstore -mod=readonly $(SIMAPP) -benchmem -bench=BenchmarkInvariants -run=^$ \
	-Enabled=true -NumBlocks=1000 -BlockSize=200 \
	-Period=1 -Commit=true -Seed=57 -v -timeout 24h

.PHONY: \
test-sim-nondeterminism \
test-sim-custom-genesis-fast \
test-sim-import-export \
test-sim-after-import \
test-sim-custom-genesis-multi-seed \
test-sim-multi-seed-short \
test-sim-multi-seed-long \
test-sim-benchmark-invariants

SIM_NUM_BLOCKS ?= 500
SIM_BLOCK_SIZE ?= 200
SIM_COMMIT ?= true

test-sim-benchmark:
	@echo "Running application benchmark for numBlocks=$(SIM_NUM_BLOCKS), blockSize=$(SIM_BLOCK_SIZE). This may take awhile!"
	@go test -tags=objstore -mod=readonly -benchmem -run=^$$ $(SIMAPP) -bench ^BenchmarkFullAppSimulation$$  \
		-Enabled=true -NumBlocks=$(SIM_NUM_BLOCKS) -BlockSize=$(SIM_BLOCK_SIZE) -Commit=$(SIM_COMMIT) -timeout 24h

test-sim-profile:
	@echo "Running application benchmark for numBlocks=$(SIM_NUM_BLOCKS), blockSize=$(SIM_BLOCK_SIZE). This may take awhile!"
	@go test -tags=objstore -mod=readonly -benchmem -run=^$$ $(SIMAPP) -bench ^BenchmarkFullAppSimulation$$ \
		-Enabled=true -NumBlocks=$(SIM_NUM_BLOCKS) -BlockSize=$(SIM_BLOCK_SIZE) -Commit=$(SIM_COMMIT) -timeout 24h -cpuprofile cpu.out -memprofile mem.out

.PHONY: test-sim-profile test-sim-benchmark

###############################################################################
###                                Integration Test                         ###
###############################################################################

# possible values:
# - all: run all integration tests
# - unmarked: run integration tests that are not marked
# - marker1,marker2: markers separated by comma, run integration tests that are marked with any of the markers
TESTS_TO_RUN ?= all

run-integration-tests:
	@make gen-bindings-contracts
	@./scripts/run-integration-tests

.PHONY: run-integration-tests

###############################################################################
###                                Utility                                  ###
###############################################################################

test-cronos-contracts:
	@git submodule update --init --recursive
	@nix-shell ./contracts/shell.nix --pure --run ./scripts/test-cronos-contracts

gen-cronos-contracts:
	@git submodule update --init --recursive
	@nix-shell ./contracts/shell.nix --pure --run ./scripts/gen-cronos-contracts

gen-bindings-contracts:
	@nix-shell ./nix/gen-binding-shell.nix $(GEN_BINDING_FLAGS) --pure --run ./scripts/gen-bindings-contracts

.PHONY: gen-cronos-contracts gen-bindings-contracts test-cronos-contracts

check-network:
ifeq ($(NETWORK),mainnet)
else ifeq ($(NETWORK),testnet)
else
	@echo "Unrecognized network: ${NETWORK}"
endif
	@echo "building network: ${NETWORK}"

print-ledger:
ifeq ($(LEDGER_ENABLED),true)
	@echo "building with ledger support"
endif

###############################################################################
###                                Protobuf                                 ###
###############################################################################

HTTPS_GIT := https://github.com/crypto-org-chain/cronos.git
protoVer=0.14.0
protoImageName=ghcr.io/cosmos/proto-builder:$(protoVer)
protoImageCi=$(DOCKER) run --rm -v $(CURDIR):/workspace --workdir /workspace --user root $(protoImageName)
protoImage=$(DOCKER) run --rm -v $(CURDIR):/workspace --workdir /workspace $(protoImageName)

# ------
# NOTE: If you are experiencing problems running these commands, try deleting
#       the docker images and execute the desired command again.
#
proto-all: proto-format proto-lint proto-gen

proto-gen-ci:
	@echo "Generating Protobuf files"
	$(protoImageCi) sh ./scripts/protocgen.sh

proto-gen:
	@echo "Generating Protobuf files"
	$(protoImage) sh ./scripts/protocgen.sh

proto-lint:
	@echo "Linting Protobuf files"
	@$(protoImage) buf lint ./proto --error-format=json

proto-swagger-gen:
	@echo "Generating Protobuf Swagger"
	$(protoImage) sh ./scripts/protoc-swagger-gen.sh

proto-format:
	@echo "Formatting Protobuf files"
	@$(protoImage) find ./ -not -path "./third_party/*" -name "*.proto" -exec clang-format -i {} \;

proto-check-breaking:
	@echo "Checking Protobuf files for breaking changes"
	@$(protoImage) buf breaking --against $(HTTPS_GIT)#branch=main


.PHONY: proto-all proto-gen proto-format proto-lint proto-check-breaking

vulncheck: $(BUILDDIR)/
	GOBIN=$(BUILDDIR) go install golang.org/x/vuln/cmd/govulncheck@latest
	$(BUILDDIR)/govulncheck ./...