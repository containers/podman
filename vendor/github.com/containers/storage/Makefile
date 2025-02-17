.PHONY: \
	all \
	binary \
	clean \
	codespell \
	containers-storage \
	cross \
	default \
	docs \
	gccgo \
	help \
	install \
	install.docs \
	install.tools \
	lint \
	local-binary \
	local-cross \
	local-gccgo \
	local-test \
	local-test-integration \
	local-test-unit \
	local-validate \
	test-integration \
	test-unit \
	validate \
	vendor \
	vendor-in-container

NATIVETAGS :=
AUTOTAGS := $(shell ./hack/btrfs_tag.sh) $(shell ./hack/libsubid_tag.sh)
BUILDFLAGS := -tags "$(AUTOTAGS) $(TAGS)" $(FLAGS)
GO ?= go
TESTFLAGS := $(shell $(GO) test -race $(BUILDFLAGS) ./pkg/stringutils 2>&1 > /dev/null && echo -race)

# N/B: This value is managed by Renovate, manual changes are
# possible, as long as they don't disturb the formatting
# (i.e. DO NOT ADD A 'v' prefix!)
GOLANGCI_LINT_VERSION := 1.64.5

default all: local-binary docs local-validate local-cross ## validate all checks, build and cross-build\nbinaries and docs

clean: ## remove all built files
	$(RM) -f containers-storage containers-storage.* docs/*.1 docs/*.5

containers-storage: ## build using gc on the host
	$(GO) build -compiler gc $(BUILDFLAGS) ./cmd/containers-storage

codespell:
	codespell

binary local-binary: containers-storage

local-gccgo gccgo: ## build using gccgo on the host
	GCCGO=$(PWD)/hack/gccgo-wrapper.sh $(GO) build -compiler gccgo $(BUILDFLAGS) -o containers-storage.gccgo ./cmd/containers-storage

local-cross cross: ## cross build the binaries for arm, darwin, and freebsd
	@for target in linux/amd64 linux/386 linux/arm linux/arm64 linux/ppc64 linux/ppc64le linux/riscv64 linux/s390x linux/mips linux/mipsle linux/mips64 linux/mips64le darwin/amd64 windows/amd64 freebsd/amd64 freebsd/arm64 ; do \
		os=`echo $${target} | cut -f1 -d/` ; \
		arch=`echo $${target} | cut -f2 -d/` ; \
		suffix=$${os}.$${arch} ; \
		echo env CGO_ENABLED=0 GOOS=$${os} GOARCH=$${arch} $(GO) build -compiler gc -tags \"$(NATIVETAGS) $(TAGS)\" $(FLAGS) ./... ; \
		env CGO_ENABLED=0 GOOS=$${os} GOARCH=$${arch} $(GO) build -compiler gc -tags "$(NATIVETAGS) $(TAGS)" $(FLAGS) ./... || exit 1 ; \
		echo env CGO_ENABLED=0 GOOS=$${os} GOARCH=$${arch} $(GO) build -compiler gc -tags \"$(NATIVETAGS) $(TAGS)\" $(FLAGS) -o containers-storage.$${suffix} ./cmd/containers-storage ; \
		env CGO_ENABLED=0 GOOS=$${os} GOARCH=$${arch} $(GO) build -compiler gc -tags "$(NATIVETAGS) $(TAGS)" $(FLAGS) -o containers-storage.$${suffix} ./cmd/containers-storage || exit 1 ; \
	done

docs: install.tools ## build the docs on the host
	$(MAKE) -C docs docs

local-test: local-binary local-test-unit local-test-integration ## build the binaries and run the tests

local-test-unit test-unit: local-binary ## run the unit tests on the host (requires\nsuperuser privileges)
	@$(GO) test -count 1 $(BUILDFLAGS) $(TESTFLAGS) ./...

local-test-integration test-integration: local-binary ## run the integration tests on the host (requires\nsuperuser privileges)
	@cd tests; ./test_runner.bash

local-validate validate: install.tools ## validate DCO on the host
	@./hack/git-validation.sh

install.tools:
	$(MAKE) -C tests/tools GOLANGCI_LINT_VERSION=$(GOLANGCI_LINT_VERSION)

install.docs: docs
	$(MAKE) -C docs install

install: install.docs

lint: install.tools
	tests/tools/build/golangci-lint run --build-tags="$(AUTOTAGS) $(TAGS)"

help: ## this help
	@awk 'BEGIN {FS = ":.*?## "} /^[a-z A-Z_-]+:.*?## / {gsub(" ",",",$$1);gsub("\\\\n",sprintf("\n%22c"," "), $$2);printf "\033[36m%-21s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

vendor-in-container:
	podman run --privileged --rm --env HOME=/root -v `pwd`:/src -w /src golang make vendor

vendor:
	$(GO) mod tidy
	$(GO) mod vendor
	$(GO) mod verify
