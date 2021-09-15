export GOPROXY=https://proxy.golang.org

APPARMORTAG := $(shell hack/apparmor_tag.sh)
STORAGETAGS := $(shell ./btrfs_tag.sh) $(shell ./btrfs_installed_tag.sh) $(shell ./libdm_tag.sh) $(shell ./hack/libsubid_tag.sh)
SECURITYTAGS ?= seccomp $(APPARMORTAG)
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

EXTRA_LDFLAGS ?=
BUILDAH_LDFLAGS := -ldflags '-X main.GitCommit=$(GIT_COMMIT) -X main.buildInfo=$(SOURCE_DATE_EPOCH) -X main.cniVersion=$(CNI_COMMIT) $(EXTRA_LDFLAGS)'
SOURCES=*.go imagebuildah/*.go bind/*.go chroot/*.go copier/*.go docker/*.go manifests/*.go pkg/blobcache/*.go pkg/chrootuser/*.go pkg/cli/*.go pkg/completion/*.go pkg/formats/*.go pkg/overlay/*.go pkg/parse/*.go pkg/rusage/*.go pkg/sshagent/*.go pkg/umask/*.go pkg/util/*.go util/*.go

LINTFLAGS ?=

ifeq ($(DEBUG), 1)
  override GOGCFLAGS += -N -l
endif

#   make all DEBUG=1
#     Note: Uses the -N -l go compiler options to disable compiler optimizations
#           and inlining. Using these build options allows you to subsequently
#           use source debugging tools like delve.
all: bin/buildah bin/imgtype bin/copy docs

# Update nix/nixpkgs.json its latest stable commit
.PHONY: nixpkgs
nixpkgs:
	@nix run \
		-f channel:nixos-20.09 nix-prefetch-git \
		-c nix-prefetch-git \
		--no-deepClone \
		https://github.com/nixos/nixpkgs refs/heads/nixos-20.09 > nix/nixpkgs.json

# Build statically linked binary
.PHONY: static
static:
	@nix build -f nix/
	mkdir -p ./bin
	cp -rfp ./result/bin/* ./bin/

bin/buildah: $(SOURCES) cmd/buildah/*.go
	$(GO_BUILD) $(BUILDAH_LDFLAGS) -gcflags "$(GOGCFLAGS)" -o $@ $(BUILDFLAGS) ./cmd/buildah

.PHONY: buildah
buildah: bin/buildah

LINUX_CROSS_TARGETS = $(addprefix bin/buildah.,$(subst /,.,$(shell $(GO) tool dist list | grep ^linux/)))
DARWIN_CROSS_TARGETS = $(addprefix bin/buildah.,$(subst /,.,$(shell $(GO) tool dist list | grep ^darwin/)))
WINDOWS_CROSS_TARGETS = $(addsuffix .exe,$(addprefix bin/buildah.,$(subst /,.,$(shell $(GO) tool dist list | grep ^windows/))))
.PHONY: cross
cross: $(LINUX_CROSS_TARGETS) $(DARWIN_CROSS_TARGETS) $(WINDOWS_CROSS_TARGETS)

bin/buildah.%:
	mkdir -p ./bin
	GOOS=$(word 2,$(subst ., ,$@)) GOARCH=$(word 3,$(subst ., ,$@)) $(GO_BUILD) $(BUILDAH_LDFLAGS) -o $@ -tags "containers_image_openpgp" ./cmd/buildah

bin/imgtype: $(SOURCES) tests/imgtype/imgtype.go
	$(GO_BUILD) $(BUILDAH_LDFLAGS) -o $@ $(BUILDFLAGS) ./tests/imgtype/imgtype.go

bin/copy: $(SOURCES) tests/copy/copy.go
	$(GO_BUILD) $(BUILDAH_LDFLAGS) -o $@ $(BUILDFLAGS) ./tests/copy/copy.go

.PHONY: clean
clean:
	$(RM) -r bin tests/testreport/testreport
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
	codespell -S Makefile,build,buildah,buildah.spec,imgtype,copy,AUTHORS,bin,vendor,.git,go.sum,CHANGELOG.md,changelog.txt,seccomp.json,.cirrus.yml,"*.xz,*.gz,*.tar,*.tgz,*ico,*.png,*.1,*.5,*.orig,*.rej" -L uint,iff,od,ERRO -w

.PHONY: validate
validate: install.tools
	./tests/validate/whitespace.sh
	./tests/validate/git-validation.sh
	./hack/xref-helpmsgs-manpages
	./tests/validate/pr-should-include-tests
	./tests/validate/buildahimages-are-sane

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
	install -D -m0755 bin/buildah $(DESTDIR)/$(BINDIR)/buildah
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

.PHONY: test-conformance
test-conformance:
	$(GO_TEST) -v -tags "$(STORAGETAGS) $(SECURITYTAGS)" -cover -timeout 20m ./tests/conformance

.PHONY: test-integration
test-integration: install.tools
	./tests/tools/build/ginkgo $(BUILDFLAGS) -v tests/e2e/.
	cd tests; ./test_runner.sh

tests/testreport/testreport: tests/testreport/testreport.go
	$(GO_BUILD) -ldflags "-linkmode external -extldflags -static" -tags "$(STORAGETAGS) $(SECURITYTAGS)" -o tests/testreport/testreport ./tests/testreport/testreport.go

.PHONY: test-unit
test-unit: tests/testreport/testreport
	$(GO_TEST) -v -tags "$(STORAGETAGS) $(SECURITYTAGS)" -cover -race $(shell $(GO) list ./... | grep -v vendor | grep -v tests | grep -v cmd) -timeout 45m
	tmp=$(shell mktemp -d) ; \
	mkdir -p $$tmp/root $$tmp/runroot; \
	$(GO_TEST) -v -tags "$(STORAGETAGS) $(SECURITYTAGS)" -cover -race ./cmd/buildah -args --root $$tmp/root --runroot $$tmp/runroot --storage-driver vfs --signature-policy $(shell pwd)/tests/policy.json --registries-conf $(shell pwd)/tests/registries.conf

vendor-in-container:
	podman run --privileged --rm --env HOME=/root -v `pwd`:/src -w /src docker.io/library/golang:1.16 make vendor

.PHONY: vendor
vendor:
	GO111MODULE=on $(GO) mod tidy
	GO111MODULE=on $(GO) mod vendor
	GO111MODULE=on $(GO) mod verify

.PHONY: lint
lint: install.tools
	./tests/tools/build/golangci-lint run $(LINTFLAGS)
