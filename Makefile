BUILDDIR ?= $(CURDIR)/build
PACKAGES=$(shell go list ./... | grep -v '/simulation')
COVERAGE ?= coverage.txt

build: go.sum
	@go build -mod=readonly $(BUILD_FLAGS) $(BUILD_TAGS) -o $(BUILDDIR)/cronosd ./cmd/cronosd

test:
	@go test -v -mod=readonly $(PACKAGES) -coverprofile=$(COVERAGE) -covermode=atomic

# release
PACKAGE_NAME:=github.com/crypto-org-chain/cronos
GOLANG_CROSS_VERSION  = v1.16.4
release-dry-run:
	docker run \
		--rm \
		--privileged \
		-e CGO_ENABLED=1 \
		-v /var/run/docker.sock:/var/run/docker.sock \
		-v `pwd`:/go/src/$(PACKAGE_NAME) \
		-v ${GOPATH}/pkg:/go/pkg \
		-w /go/src/$(PACKAGE_NAME) \
		troian/golang-cross:${GOLANG_CROSS_VERSION} \
		--rm-dist --skip-validate --skip-publish

release:
	@if [ ! -f ".release-env" ]; then \
		echo "\033[91m.release-env is required for release\033[0m";\
		exit 1;\
	fi
	docker run \
		--rm \
		--privileged \
		-e CGO_ENABLED=1 \
		--env-file .release-env \
		-v /var/run/docker.sock:/var/run/docker.sock \
		-v `pwd`:/go/src/$(PACKAGE_NAME) \
		-w /go/src/$(PACKAGE_NAME) \
		troian/golang-cross:${GOLANG_CROSS_VERSION} \
		release --rm-dist --skip-validate

.PHONY: release-dry-run release
