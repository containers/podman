export GO111MODULE=off

.PHONY: \
	all \
	binary \
	clean \
	cross \
	default \
	docs \
	gccgo \
	help \
	install.tools \
	local-binary \
	local-cross \
	local-gccgo \
	local-test-integration \
	local-test-unit \
	local-validate \
	lint \
	test \
	test-integration \
	test-unit \
	validate \
	vendor

PACKAGE := github.com/containers/storage
GIT_BRANCH := $(shell git rev-parse --abbrev-ref HEAD 2>/dev/null)
GIT_BRANCH_CLEAN := $(shell echo $(GIT_BRANCH) | sed -e "s/[^[:alnum:]]/-/g")
EPOCH_TEST_COMMIT := 0418ebf59f9e1f564831c0ba9378b7f8e40a1c73
NATIVETAGS :=
AUTOTAGS := $(shell ./hack/btrfs_tag.sh) $(shell ./hack/libdm_tag.sh) $(shell ./hack/ostree_tag.sh)
BUILDFLAGS := -tags "$(AUTOTAGS) $(TAGS)" $(FLAGS)
GO ?= go

GO_BUILD=$(GO) build
# Go module support: set `-mod=vendor` to use the vendored sources
ifeq ($(shell $(GO) help mod >/dev/null 2>&1 && echo true), true)
	GO_BUILD=GO111MODULE=on $(GO) build -mod=vendor
endif

RUNINVM := vagrant/runinvm.sh
FFJSON := tests/tools/build/ffjson

default all: local-binary docs local-validate local-cross local-gccgo test-unit test-integration ## validate all checks, build and cross-build\nbinaries and docs, run tests in a VM

clean: ## remove all built files
	$(RM) -f containers-storage containers-storage.* docs/*.1 docs/*.5

sources := $(wildcard *.go cmd/containers-storage/*.go drivers/*.go drivers/*/*.go pkg/*/*.go pkg/*/*/*.go) layers_ffjson.go images_ffjson.go containers_ffjson.go pkg/archive/archive_ffjson.go

containers-storage: $(sources) ## build using gc on the host
	$(GO_BUILD) -compiler gc $(BUILDFLAGS) ./cmd/containers-storage

layers_ffjson.go: layers.go
	$(RM) $@
	$(FFJSON) layers.go

images_ffjson.go: images.go
	$(RM) $@
	$(FFJSON) images.go

containers_ffjson.go: containers.go
	$(RM) $@
	$(FFJSON) containers.go

pkg/archive/archive_ffjson.go: pkg/archive/archive.go
	$(RM) $@
	$(FFJSON) pkg/archive/archive.go

binary local-binary: containers-storage

local-gccgo: ## build using gccgo on the host
	GCCGO=$(PWD)/hack/gccgo-wrapper.sh $(GO_BUILD) -compiler gccgo $(BUILDFLAGS) -o containers-storage.gccgo ./cmd/containers-storage

local-cross: ## cross build the binaries for arm, darwin, and\nfreebsd
	@for target in linux/amd64 linux/386 linux/arm linux/arm64 linux/ppc64 linux/ppc64le darwin/amd64 windows/amd64 ; do \
		os=`echo $${target} | cut -f1 -d/` ; \
		arch=`echo $${target} | cut -f2 -d/` ; \
		suffix=$${os}.$${arch} ; \
		echo env CGO_ENABLED=0 GOOS=$${os} GOARCH=$${arch} $(GO_BUILD) -compiler gc -tags \"$(NATIVETAGS) $(TAGS)\" $(FLAGS) -o containers-storage.$${suffix} ./cmd/containers-storage ; \
		env CGO_ENABLED=0 GOOS=$${os} GOARCH=$${arch} $(GO_BUILD) -compiler gc -tags "$(NATIVETAGS) $(TAGS)" $(FLAGS) -o containers-storage.$${suffix} ./cmd/containers-storage || exit 1 ; \
	done

cross: ## cross build the binaries for arm, darwin, and\nfreebsd using VMs
	$(RUNINVM) make local-$@

docs: install.tools ## build the docs on the host
	$(MAKE) -C docs docs

gccgo: ## build using gccgo using VMs
	$(RUNINVM) make local-$@

test: local-binary ## build the binaries and run the tests using VMs
	$(RUNINVM) make local-binary local-cross local-test-unit local-test-integration

local-test-unit: local-binary ## run the unit tests on the host (requires\nsuperuser privileges)
	@$(GO) test $(BUILDFLAGS) $(shell $(GO) list ./... | grep -v ^$(PACKAGE)/vendor)

test-unit: local-binary ## run the unit tests using VMs
	$(RUNINVM) make local-$@

local-test-integration: local-binary ## run the integration tests on the host (requires\nsuperuser privileges)
	@cd tests; ./test_runner.bash

test-integration: local-binary ## run the integration tests using VMs
	$(RUNINVM) make local-$@

local-validate: ## validate DCO and gofmt on the host
	@./hack/git-validation.sh
	@./hack/gofmt.sh

validate: ## validate DCO, gofmt, ./pkg/ isolation, golint,\ngo vet and vendor using VMs
	$(RUNINVM) make local-$@

install.tools:
	make -C tests/tools

install.docs: docs
	make -C docs install

install: install.docs

lint: install.tools
	tests/tools/build/golangci-lint run

help: ## this help
	@awk 'BEGIN {FS = ":.*?## "} /^[a-z A-Z_-]+:.*?## / {gsub(" ",",",$$1);gsub("\\\\n",sprintf("\n%22c"," "), $$2);printf "\033[36m%-21s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

vendor:
	export GO111MODULE=on \
		$(GO) mod tidy && \
		$(GO) mod vendor && \
		$(GO) mod verify
