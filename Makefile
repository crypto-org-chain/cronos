BUILDDIR ?= $(CURDIR)/build
PACKAGES=$(shell go list ./... | grep -v '/simulation')
COVERAGE ?= coverage.txt

build: go.sum
	@go build -mod=readonly $(BUILD_FLAGS) $(BUILD_TAGS) -o $(BUILDDIR)/cronosd ./cmd/cronosd

test:
	@go test -v -mod=readonly $(PACKAGES) -coverprofile=$(COVERAGE) -covermode=atomic
