SELINUXTAG := $(shell ./selinux_tag.sh)
STORAGETAGS := $(shell ./btrfs_tag.sh) $(shell ./btrfs_installed_tag.sh) $(shell ./libdm_tag.sh) $(shell ./ostree_tag.sh)
SECURITYTAGS ?= seccomp $(SELINUXTAG)
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
GIT_COMMIT ?= $(if $(shell git rev-parse --short HEAD),$(shell git rev-parse --short HEAD),$(error "git failed"))
BUILD_INFO := $(if $(shell date +%s),$(shell date +%s),$(error "date failed"))
CNI_COMMIT := $(if $(shell sed -e '\,github.com/containernetworking/cni, !d' -e 's,.* ,,g' vendor.conf),$(shell sed -e '\,github.com/containernetworking/cni, !d' -e 's,.* ,,g' vendor.conf),$(error "sed failed"))
STATIC_STORAGETAGS = "containers_image_ostree_stub containers_image_openpgp exclude_graphdriver_devicemapper $(STORAGE_TAGS)"

RUNC_COMMIT := 2c632d1a2de0192c3f18a2542ccb6f30a8719b1f
LIBSECCOMP_COMMIT := release-2.3

EXTRALDFLAGS :=
LDFLAGS := -ldflags '-X main.GitCommit=$(GIT_COMMIT) -X main.buildInfo=$(BUILD_INFO) -X main.cniVersion=$(CNI_COMMIT)' $(EXTRALDFLAGS)
SOURCES=*.go imagebuildah/*.go bind/*.go chroot/*.go cmd/buildah/*.go docker/*.go pkg/blobcache/*.go pkg/cli/*.go pkg/parse/*.go pkg/unshare/*.c pkg/unshare/*.go util/*.go

all: buildah imgtype docs

.PHONY: static
static: $(SOURCES)
	$(MAKE) SECURITYTAGS="$(SECURITYTAGS)" STORAGETAGS=$(STATIC_STORAGETAGS) EXTRALDFLAGS='-ldflags "-extldflags '-static'"' BUILDAH=buildah.static binary

.PHONY: binary
binary:  $(SOURCES)
	$(GO) build $(LDFLAGS) -o $(BUILDAH) $(BUILDFLAGS) ./cmd/buildah

buildah: binary

darwin:
	GOOS=darwin $(GO) build $(LDFLAGS) -o buildah.darwin -tags "containers_image_openpgp" ./cmd/buildah

imgtype: *.go docker/*.go util/*.go tests/imgtype/imgtype.go
	$(GO) build $(LDFLAGS) -o imgtype $(BUILDFLAGS) ./tests/imgtype/imgtype.go

.PHONY: clean
clean:
	$(RM) -r buildah imgtype build buildah.static
	$(MAKE) -C docs clean

.PHONY: docs
docs: ## build the docs on the host
	$(MAKE) -C docs

# For vendoring to work right, the checkout directory must be such that our top
# level is at $GOPATH/src/github.com/containers/buildah.
.PHONY: gopath
gopath:
	test $(shell pwd) = $(shell cd ../../../../src/github.com/containers/buildah ; pwd)

# We use https://github.com/lk4d4/vndr to manage dependencies.
.PHONY: deps
deps: gopath
	env GOPATH=$(shell cd ../../../.. ; pwd) vndr

.PHONY: validate
validate:
	# Run gofmt on version 1.11 and higher
ifneq ($(GO110),$(GOVERSION))
	@./tests/validate/gofmt.sh
endif
	@./tests/validate/whitespace.sh
	@./tests/validate/govet.sh
	@./tests/validate/git-validation.sh
	@./tests/validate/gometalinter.sh . cmd/buildah

.PHONY: install.tools
install.tools:
	$(GO) get -u $(BUILDFLAGS) github.com/cpuguy83/go-md2man
	$(GO) get -u $(BUILDFLAGS) github.com/vbatts/git-validation
	$(GO) get -u $(BUILDFLAGS) github.com/onsi/ginkgo/ginkgo
	$(GO) get -u $(BUILDFLAGS) gopkg.in/alecthomas/gometalinter.v1
	$(GOPATH)/bin/gometalinter.v1 -i

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
	cd ../../containernetworking/plugins && ./build.sh && mkdir -p /opt/cni/bin && sudo install -v -m755 bin/* /opt/cni/bin/

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
test-integration:
	ginkgo -v tests/e2e/.
	cd tests; ./test_runner.sh

tests/testreport/testreport: tests/testreport/testreport.go
	$(GO) build -ldflags "-linkmode external -extldflags -static" -tags "$(STORAGETAGS) $(SECURITYTAGS)" -o tests/testreport/testreport ./tests/testreport

.PHONY: test-unit
test-unit: tests/testreport/testreport
	$(GO) test -v -tags "$(STOAGETAGS) $(SECURITYTAGS)" -race $(shell $(GO) list ./... | grep -v vendor | grep -v tests | grep -v cmd)
	tmp=$(shell mktemp -d) ; \
	mkdir -p $$tmp/root $$tmp/runroot; \
	$(GO) test -v -tags "$(STORAGETAGS) $(SECURITYTAGS)" ./cmd/buildah -args -root $$tmp/root -runroot $$tmp/runroot -storage-driver vfs -signature-policy $(shell pwd)/tests/policy.json -registries-conf $(shell pwd)/tests/registries.conf

.PHONY: .install.vndr
.install.vndr:
	$(GO) get -u github.com/LK4D4/vndr

.PHONY: vendor
vendor: vendor.conf .install.vndr
	$(GOPATH)/bin/vndr \
		-whitelist "github.com/onsi/gomega"
