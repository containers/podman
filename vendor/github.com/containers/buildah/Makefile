export GOPROXY=https://proxy.golang.org

SELINUXTAG := $(shell ./selinux_tag.sh)
APPARMORTAG := $(shell hack/apparmor_tag.sh)
STORAGETAGS := $(shell ./btrfs_tag.sh) $(shell ./btrfs_installed_tag.sh) $(shell ./libdm_tag.sh)
SECURITYTAGS ?= seccomp $(SELINUXTAG) $(APPARMORTAG)
TAGS ?= $(SECURITYTAGS) $(STORAGETAGS)
BUILDTAGS += $(TAGS)
PREFIX := /usr/local
BINDIR := $(PREFIX)/bin
BASHINSTALLDIR = $(PREFIX)/share/bash-completion/completions
BUILDFLAGS := -tags "$(BUILDTAGS)"
BUILDAH := buildah

GO := go
GO110 := 1.10
GOVERSION := $(findstring $(GO110),$(shell go version))
# test for go module support
ifeq ($(shell go help mod >/dev/null 2>&1 && echo true), true)
export GO_BUILD=GO111MODULE=on $(GO) build -mod=vendor
export GO_TEST=GO111MODULE=on $(GO) test -mod=vendor
else
export GO_BUILD=$(GO) build
export GO_TEST=$(GO) test
endif

GIT_COMMIT ?= $(if $(shell git rev-parse --short HEAD),$(shell git rev-parse --short HEAD),$(error "git failed"))
SOURCE_DATE_EPOCH ?= $(if $(shell date +%s),$(shell date +%s),$(error "date failed"))
STATIC_STORAGETAGS = "containers_image_openpgp exclude_graphdriver_devicemapper $(STORAGE_TAGS)"

CNI_COMMIT := $(shell sed -n 's;\tgithub.com/containernetworking/cni \([^ \n]*\).*$\;\1;p' go.mod)
#RUNC_COMMIT := $(shell sed -n 's;\tgithub.com/opencontainers/runc \([^ \n]*\).*$\;\1;p' go.mod)
RUNC_COMMIT := v1.0.0-rc8
LIBSECCOMP_COMMIT := release-2.3

EXTRALDFLAGS :=
LDFLAGS := -ldflags '-X main.GitCommit=$(GIT_COMMIT) -X main.buildInfo=$(SOURCE_DATE_EPOCH) -X main.cniVersion=$(CNI_COMMIT)' $(EXTRALDFLAGS)
SOURCES=*.go imagebuildah/*.go bind/*.go chroot/*.go cmd/buildah/*.go docker/*.go pkg/blobcache/*.go pkg/cli/*.go pkg/parse/*.go util/*.go

LINTFLAGS ?=

all: buildah imgtype docs

.PHONY: static
static: $(SOURCES)
	$(MAKE) SECURITYTAGS="$(SECURITYTAGS)" STORAGETAGS=$(STATIC_STORAGETAGS) EXTRALDFLAGS='-ldflags "-extldflags '-static'"' BUILDAH=buildah.static binary

.PHONY: binary
binary:  $(SOURCES)
	$(GO_BUILD) $(LDFLAGS) -o $(BUILDAH) $(BUILDFLAGS) ./cmd/buildah

buildah: binary

darwin:
	GOOS=darwin $(GO_BUILD) $(LDFLAGS) -o buildah.darwin -tags "containers_image_openpgp" ./cmd/buildah

imgtype: *.go docker/*.go util/*.go tests/imgtype/imgtype.go
	$(GO_BUILD) $(LDFLAGS) -o imgtype $(BUILDFLAGS) ./tests/imgtype/imgtype.go

.PHONY: clean
clean:
	$(RM) -r buildah imgtype build buildah.static buildah.darwin tests/testreport/testreport
	$(MAKE) -C docs clean

.PHONY: docs
docs: install.tools ## build the docs on the host
	$(MAKE) -C docs

# For vendoring to work right, the checkout directory must be such that our top
# level is at $GOPATH/src/github.com/containers/buildah.
.PHONY: gopath
gopath:
	test $(shell pwd) = $(shell cd ../../../../src/github.com/containers/buildah ; pwd)

codespell:
	codespell -S build,buildah,buildah.spec,imgtype,AUTHORS,bin,vendor,.git,go.sum,CHANGELOG.md,changelog.txt,seccomp.json,.cirrus.yml,"*.xz,*.gz,*.tar,*.tgz,*ico,*.png,*.1,*.5,*.orig,*.rej" -L uint,iff,od

.PHONY: validate
validate: install.tools
	@./tests/validate/whitespace.sh
	@./tests/validate/git-validation.sh

.PHONY: install.tools
install.tools:
	make -C tests/tools

.PHONY: runc
runc: gopath
	rm -rf ../../opencontainers/runc
	git clone https://github.com/opencontainers/runc ../../opencontainers/runc
	cd ../../opencontainers/runc && git checkout $(RUNC_COMMIT) && $(GO) build -tags "$(STORAGETAGS) $(SECURITYTAGS)"
	ln -sf ../../opencontainers/runc/runc

.PHONY: install.libseccomp.sudo
install.libseccomp.sudo: gopath
	rm -rf ../../seccomp/libseccomp
	git clone https://github.com/seccomp/libseccomp ../../seccomp/libseccomp
	cd ../../seccomp/libseccomp && git checkout $(LIBSECCOMP_COMMIT) && ./autogen.sh && ./configure --prefix=/usr && make all && sudo make install

.PHONY: install.cni.sudo
install.cni.sudo: gopath
	rm -rf ../../containernetworking/plugins
	git clone https://github.com/containernetworking/plugins ../../containernetworking/plugins
	cd ../../containernetworking/plugins && ./build_linux.sh && sudo install -D -v -m755 -t /opt/cni/bin/ bin/*

.PHONY: install
install:
	install -D -m0755 buildah $(DESTDIR)/$(BINDIR)/buildah
	$(MAKE) -C docs install

.PHONY: uninstall
uninstall:
	rm -f $(DESTDIR)/$(BINDIR)/buildah
	rm -f $(PREFIX)/share/man/man1/buildah*.1
	rm -f $(DESTDIR)/$(BASHINSTALLDIR)/buildah

.PHONY: install.completions
install.completions:
	install -m 644 -D contrib/completions/bash/buildah $(DESTDIR)/$(BASHINSTALLDIR)/buildah

.PHONY: install.runc
install.runc:
	install -m 755 ../../opencontainers/runc/runc $(DESTDIR)/$(BINDIR)/

.PHONY: test-integration
test-integration: install.tools
	./tests/tools/build/ginkgo $(BUILDFLAGS) -v tests/e2e/.
	cd tests; ./test_runner.sh

tests/testreport/testreport: tests/testreport/testreport.go
	$(GO_BUILD) -ldflags "-linkmode external -extldflags -static" -tags "$(STORAGETAGS) $(SECURITYTAGS)" -o tests/testreport/testreport ./tests/testreport

.PHONY: test-unit
test-unit: tests/testreport/testreport
	$(GO_TEST) -v -tags "$(STORAGETAGS) $(SECURITYTAGS)" -race $(shell $(GO) list ./... | grep -v vendor | grep -v tests | grep -v cmd)
	tmp=$(shell mktemp -d) ; \
	mkdir -p $$tmp/root $$tmp/runroot; \
	$(GO_TEST) -v -tags "$(STORAGETAGS) $(SECURITYTAGS)" ./cmd/buildah -args -root $$tmp/root -runroot $$tmp/runroot -storage-driver vfs -signature-policy $(shell pwd)/tests/policy.json -registries-conf $(shell pwd)/tests/registries.conf

vendor-in-container:
	podman run --privileged --rm --env HOME=/root -v `pwd`:/src -w /src docker.io/library/golang:1.13 make vendor

.PHONY: vendor
vendor:
	GO111MODULE=on $(GO) mod tidy
	GO111MODULE=on $(GO) mod vendor
	GO111MODULE=on $(GO) mod verify

.PHONY: lint
lint: install.tools
	./tests/tools/build/golangci-lint run $(LINTFLAGS)
