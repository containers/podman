###
### Makefile Navigation
###
#
# This file is organized based on approximate end-to-end workflow:
#
# 1.  Variables and common definitions are located at the top
#     to make finding them quicker.
# 2.  Main entry-point targets, like "default", "all", and "help"
# 3.  Targets for code formatting and validation
# 4.  Primary build targets, like podman and podman-remote
# 5.  Secondary build targets, shell completions, static and multi-arch.
# 6.  Targets that format and build documentation
# 7.  Testing targets
# 8.  Release and package-building targets
# 9.  Targets that install tools, utilities, binaries and packages
# 10. Uninstall / Cleanup targets
#
###
### Variables & Definitions
###

GO ?= go
GO_LDFLAGS:= $(shell if $(GO) version|grep -q gccgo ; then echo "-gccgoflags"; else echo "-ldflags"; fi)
GOCMD = CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) GOARCH=$(GOARCH) $(GO)
COVERAGE_PATH ?= .coverage
DESTDIR ?=
EPOCH_TEST_COMMIT ?= $(shell git merge-base $${DEST_BRANCH:-main} HEAD)
HEAD ?= HEAD
PROJECT := github.com/containers/podman
GIT_BASE_BRANCH ?= origin/main
LIBPOD_INSTANCE := libpod_dev
PREFIX ?= /usr/local
BINDIR ?= ${PREFIX}/bin
LIBEXECDIR ?= ${PREFIX}/libexec
LIBEXECPODMAN ?= ${LIBEXECDIR}/podman
MANDIR ?= ${PREFIX}/share/man
SHAREDIR_CONTAINERS ?= ${PREFIX}/share/containers
ETCDIR ?= ${PREFIX}/etc
TMPFILESDIR ?= ${PREFIX}/lib/tmpfiles.d
USERTMPFILESDIR ?= ${PREFIX}/share/user-tmpfiles.d
MODULESLOADDIR ?= ${PREFIX}/lib/modules-load.d
SYSTEMDDIR ?= ${PREFIX}/lib/systemd/system
USERSYSTEMDDIR ?= ${PREFIX}/lib/systemd/user
SYSTEMDGENERATORSDIR ?= ${PREFIX}/lib/systemd/system-generators
USERSYSTEMDGENERATORSDIR ?= ${PREFIX}/lib/systemd/user-generators
REMOTETAGS ?= remote exclude_graphdriver_btrfs btrfs_noversion exclude_graphdriver_devicemapper containers_image_openpgp
BUILDTAGS ?= \
	$(shell hack/apparmor_tag.sh) \
	$(shell hack/btrfs_installed_tag.sh) \
	$(shell hack/btrfs_tag.sh) \
	$(shell hack/selinux_tag.sh) \
	$(shell hack/systemd_tag.sh) \
	$(shell hack/libsubid_tag.sh) \
	exclude_graphdriver_devicemapper \
	seccomp
ifeq ($(shell uname -s),FreeBSD)
# Use bash for make's shell function - the default shell on FreeBSD
# has a command builtin is not compatible with the way its used below
SHELL := $(shell command -v bash)
endif
PYTHON ?= $(shell command -v python3 python|head -n1)
PKG_MANAGER ?= $(shell command -v dnf yum|head -n1)
# ~/.local/bin is not in PATH on all systems
PRE_COMMIT = $(shell command -v bin/venv/bin/pre-commit ~/.local/bin/pre-commit pre-commit | head -n1)
ifeq ($(shell uname -s),FreeBSD)
SED=gsed
else
SED=sed
endif

# This isn't what we actually build; it's a superset, used for target
# dependencies. Basically: all *.go and *.c files, except *_test.go,
# and except anything in a dot subdirectory. If any of these files is
# newer than our target (bin/podman{,-remote}), a rebuild is
# triggered.
SOURCES = $(shell find . -path './.*' -prune -o \( \( -name '*.go' -o -name '*.c' \) -a ! -name '*_test.go' \) -print)

BUILDTAGS_CROSS ?= containers_image_openpgp exclude_graphdriver_btrfs exclude_graphdriver_devicemapper exclude_graphdriver_overlay
CONTAINER_RUNTIME := $(shell command -v podman 2> /dev/null || echo docker)
OCI_RUNTIME ?= ""

# The 'sort' below is crucial: without it, 'make docs' behaves differently
# on the first run than on subsequent ones, because the generated .md
MANPAGES_SOURCE_DIR = docs/source/markdown
MANPAGES_MD_IN ?= $(wildcard $(MANPAGES_SOURCE_DIR)/*.md.in)
MANPAGES_MD_GENERATED ?= $(MANPAGES_MD_IN:%.md.in=%.md)
MANPAGES_MD ?= $(sort $(wildcard $(MANPAGES_SOURCE_DIR)/*.md) $(MANPAGES_MD_GENERATED))
MANPAGES ?= $(MANPAGES_MD:%.md=%)
MANPAGES_DEST ?= $(subst markdown,man, $(subst source,build,$(MANPAGES)))

BASHINSTALLDIR=${PREFIX}/share/bash-completion/completions
ZSHINSTALLDIR=${PREFIX}/share/zsh/site-functions
FISHINSTALLDIR=${PREFIX}/share/fish/vendor_completions.d

SELINUXOPT ?= $(shell test -x /usr/sbin/selinuxenabled && selinuxenabled && echo -Z)

COMMIT_NO ?= $(shell git rev-parse HEAD 2> /dev/null || true)
GIT_COMMIT ?= $(if $(shell git status --porcelain --untracked-files=no),$(call err_if_empty,COMMIT_NO)-dirty,$(COMMIT_NO))
DATE_FMT = %s
ifdef SOURCE_DATE_EPOCH
	BUILD_INFO ?= $(shell date -u -d "@$(call err_if_empty,SOURCE_DATE_EPOCH)" "+$(DATE_FMT)" 2>/dev/null || date -u -r "$(SOURCE_DATE_EPOCH)" "+$(DATE_FMT)" 2>/dev/null || date -u "+$(DATE_FMT)")
else
	BUILD_INFO ?= $(shell date "+$(DATE_FMT)")
endif
LIBPOD := ${PROJECT}/v4/libpod
GOFLAGS ?= -trimpath
LDFLAGS_PODMAN ?= \
	$(if $(GIT_COMMIT),-X $(LIBPOD)/define.gitCommit=$(GIT_COMMIT),) \
	$(if $(BUILD_INFO),-X $(LIBPOD)/define.buildInfo=$(BUILD_INFO),) \
	-X $(LIBPOD)/config._installPrefix=$(PREFIX) \
	-X $(LIBPOD)/config._etcDir=$(ETCDIR) \
	-X $(PROJECT)/v4/pkg/systemd/quadlet._binDir=$(BINDIR) \
	-X github.com/containers/common/pkg/config.additionalHelperBinariesDir=$(HELPER_BINARIES_DIR)\
	$(EXTRA_LDFLAGS)
LDFLAGS_PODMAN_STATIC ?= \
	$(LDFLAGS_PODMAN) \
	-extldflags=-static
#Update to LIBSECCOMP_COMMIT should reflect in Dockerfile too.
LIBSECCOMP_COMMIT := v2.3.3
# Rarely if ever should integration tests take more than 50min,
# caller may override in special circumstances if needed.
GINKGOTIMEOUT ?= -timeout=90m
# By default, run test/e2e
GINKGOWHAT ?= test/e2e/.
# By default, run tests in parallel across 3 nodes.
GINKGONODES ?= 3
GINKGO ?= ./test/tools/build/ginkgo

# Conditional required to produce empty-output if binary not built yet.
RELEASE_VERSION = $(shell if test -x test/version/version; then test/version/version; fi)
RELEASE_NUMBER = $(shell echo "$(call err_if_empty,RELEASE_VERSION)" | sed -e 's/^v\(.*\)/\1/')

# If non-empty, logs all output from server during remote system testing
PODMAN_SERVER_LOG ?=

# Ensure GOBIN is not set so the default (`go env GOPATH`/bin) is used.
override undefine GOBIN
# This must never include the 'hack' directory
export PATH := $(shell $(GO) env GOPATH)/bin:$(PATH)

GOMD2MAN ?= $(shell command -v go-md2man || echo './test/tools/build/go-md2man')

CROSS_BUILD_TARGETS := \
	bin/podman.cross.linux.amd64 \
	bin/podman.cross.linux.ppc64le \
	bin/podman.cross.linux.arm \
	bin/podman.cross.linux.arm64 \
	bin/podman.cross.linux.386 \
	bin/podman.cross.linux.s390x \
	bin/podman.cross.linux.mips \
	bin/podman.cross.linux.mipsle \
	bin/podman.cross.linux.mips64 \
	bin/podman.cross.linux.mips64le \
	bin/podman.cross.freebsd.amd64 \
	bin/podman.cross.freebsd.arm64

# Dereference variable $(1), return value if non-empty, otherwise raise an error.
err_if_empty = $(if $(strip $($(1))),$(strip $($(1))),$(error Required variable $(1) value is undefined, whitespace, or empty))

# Podman does not work w/o CGO_ENABLED, except in some very specific cases.
# Windows and Mac (both podman-remote client only) require CGO_ENABLED=0.
CGO_ENABLED ?= 1
# Default to the native OS type and architecture unless otherwise specified
NATIVE_GOOS := $(shell env -u GOOS $(GO) env GOOS)
GOOS ?= $(call err_if_empty,NATIVE_GOOS)
# Default to the native architecture type
NATIVE_GOARCH := $(shell env -u GOARCH $(GO) env GOARCH)
GOARCH ?= $(NATIVE_GOARCH)
ifeq ($(call err_if_empty,GOOS),windows)
BINSFX := .exe
SRCBINDIR := bin/windows
CGO_ENABLED := 0
else ifeq ($(GOOS),darwin)
BINSFX :=
SRCBINDIR := bin/darwin
CGO_ENABLED := 0
else
BINSFX := -remote
SRCBINDIR := bin
endif
# Necessary for nested-$(MAKE) calls and docs/remote-docs.sh
export GOOS GOARCH CGO_ENABLED BINSFX SRCBINDIR

# Need to use CGO for mDNS resolution, but cross builds need CGO disabled
# See https://github.com/golang/go/issues/12524 for details
DARWIN_GCO := 0
ifeq ($(call err_if_empty,NATIVE_GOOS),darwin)
ifdef HOMEBREW_PREFIX
	DARWIN_GCO := 1
endif
endif

# win-sshproxy is checked out manually to keep from pulling in gvisor and it's transitive
# dependencies. This is only used for the Windows installer task (podman.msi), which must
# include this lightweight helper binary.
#
GV_GITURL=https://github.com/containers/gvisor-tap-vsock.git
GV_SHA=aab0ac9367fc5142f5857c36ac2352bcb3c60ab7

###
### Primary entry-point targets
###

.PHONY: default
default: all

.PHONY: all
all: binaries docs

.PHONY: binaries
ifeq ($(shell uname -s),FreeBSD)
binaries: podman podman-remote ## Build podman and podman-remote binaries
else
binaries: podman podman-remote rootlessport quadlet ## Build podman, podman-remote and rootlessport binaries quadlet
endif

# Extract text following double-# for targets, as their description for
# the `help` target.  Otherwise These simple-substitutions are resolved
# at reference-time (due to `=` and not `=:`).
_HLP_TGTS_RX = '^[[:print:]]+:.*?\#\# .*$$'
_HLP_TGTS_CMD = grep -E $(_HLP_TGTS_RX) $(MAKEFILE_LIST)
_HLP_TGTS_LEN = $(shell $(call err_if_empty,_HLP_TGTS_CMD) | cut -d : -f 1 | wc -L)
_HLPFMT = "%-$(call err_if_empty,_HLP_TGTS_LEN)s %s\n"
.PHONY: help
help: ## (Default) Print listing of key targets with their descriptions
	@printf $(_HLPFMT) "Target:" "Description:"
	@printf $(_HLPFMT) "--------------" "--------------------"
	@$(_HLP_TGTS_CMD) | sort | \
		awk 'BEGIN {FS = ":(.*)?## "}; \
			{printf $(_HLPFMT), $$1, $$2}'

###
### Linting/Formatting/Code Validation targets
###

.PHONY: .gitvalidation
.gitvalidation:
	@echo "Validating vs commit '$(call err_if_empty,EPOCH_TEST_COMMIT)'"
	GIT_CHECK_EXCLUDE="./vendor:./test/tools/vendor:docs/make.bat:test/buildah-bud/buildah-tests.diff" ./test/tools/build/git-validation -run DCO,short-subject,dangling-whitespace -range $(EPOCH_TEST_COMMIT)..$(HEAD)

.PHONY: lint
lint: golangci-lint
	@echo "Linting vs commit '$(call err_if_empty,EPOCH_TEST_COMMIT)'"
ifeq ($(PRE_COMMIT),)
	@echo "FATAL: pre-commit was not found, make .install.pre-commit to installing it." >&2
	@exit 2
endif
	$(PRE_COMMIT) run -a

.PHONY: golangci-lint
golangci-lint: .install.golangci-lint
	hack/golangci-lint.sh run

.PHONY: test/checkseccomp/checkseccomp
test/checkseccomp/checkseccomp: $(wildcard test/checkseccomp/*.go)
	$(GOCMD) build $(BUILDFLAGS) $(GO_LDFLAGS) '$(LDFLAGS_PODMAN)' -tags "$(BUILDTAGS)" -o $@ ./test/checkseccomp

.PHONY: test/testvol/testvol
test/testvol/testvol: $(wildcard test/testvol/*.go)
	$(GOCMD) build -o $@ ./test/testvol

.PHONY: volume-plugin-test-img
volume-plugin-test-img:
	./bin/podman build --network none -t quay.io/libpod/volume-plugin-test-img:$$(date +%Y%m%d) -f ./test/testvol/Containerfile .

.PHONY: test/goecho/goecho
test/goecho/goecho: $(wildcard test/goecho/*.go)
	$(GOCMD) build $(BUILDFLAGS) $(GO_LDFLAGS) '$(LDFLAGS_PODMAN)' -o $@ ./test/goecho

# The ./test/version/version binary is executed in other make steps
# so we have to make sure the version binary is built for NATIVE_GOARCH.
test/version/version: version/version.go
	GOARCH=$(NATIVE_GOARCH) $(GO) build -o $@ ./test/version/

.PHONY: codespell
codespell:
	codespell -S bin,vendor,.git,go.sum,.cirrus.yml,"RELEASE_NOTES.md,*.xz,*.gz,*.ps1,*.tar,swagger.yaml,*.tgz,bin2img,*ico,*.png,*.1,*.5,copyimg,*.orig,apidoc.go" -L te,clos,ans,pullrequest,uint,iff,od,seeked,splitted,marge,erro,hist,ether,specif -w

.PHONY: validate
validate: lint .gitvalidation validate.completions man-page-check swagger-check tests-included tests-expect-exit pr-removes-fixed-skips

.PHONY: build-all-new-commits
build-all-new-commits:
	# Validate that all the commits build on top of $(GIT_BASE_BRANCH)
	git rebase $(call err_if_empty,GIT_BASE_BRANCH) -x "$(MAKE)"

.PHONY: vendor
vendor:
	$(GO) mod tidy -compat=1.18
	$(GO) mod vendor
	$(GO) mod verify


# We define *-in-container targets for the following make targets. This allow the targets to be run in a container.
# Note that the PODMANCMD can also be overridden to allow a different container CLI to be used on systems where podman is not already available.
IN_CONTAINER_TARGETS = vendor validate
PODMANCMD ?= podman
IN_CONTAINER = $(patsubst %,%-in-container,$(IN_CONTAINER_TARGETS))

.PHONY: $(IN_CONTAINER)
$(IN_CONTAINER): %-in-container:
	$(PODMANCMD) run --rm --env HOME=/root \
		-v $(CURDIR):/src -w /src \
		--security-opt label=disable \
		docker.io/library/golang:1.18 \
		make $(*)


###
### Primary binary-build targets
###

# Make sure to warn in case we're building without the systemd buildtag.
bin/podman: $(SOURCES) go.mod go.sum
ifeq (,$(findstring systemd,$(BUILDTAGS)))
	@echo "Podman is being compiled without the systemd build tag. \
		Install libsystemd on Ubuntu or systemd-devel on rpm based \
		distro for journald support."
endif
	$(GOCMD) build \
		$(BUILDFLAGS) \
		$(GO_LDFLAGS) '$(LDFLAGS_PODMAN)' \
		-tags "$(BUILDTAGS)" \
		-o $@ ./cmd/podman

# Disambiguate Linux vs Darwin/Windows platform binaries under distinct "bin" dirs
$(SRCBINDIR):
	mkdir -p $(SRCBINDIR)

# '|' is to ignore SRCBINDIR mtime; see: info make 'Types of Prerequisites'
$(SRCBINDIR)/podman$(BINSFX): $(SOURCES) go.mod go.sum | $(SRCBINDIR)
	$(GOCMD) build \
		$(BUILDFLAGS) \
		$(GO_LDFLAGS) '$(LDFLAGS_PODMAN)' \
		-tags "${REMOTETAGS}" \
		-o $@ ./cmd/podman

$(SRCBINDIR)/podman-remote-static-linux_amd64 $(SRCBINDIR)/podman-remote-static-linux_arm64: $(SRCBINDIR)/podman-remote-static-linux_%: $(SRCBINDIR) $(SOURCES) go.mod go.sum
	CGO_ENABLED=0 \
	GOOS=linux \
	GOARCH=$* \
	$(GO) build \
		$(BUILDFLAGS) \
		$(GO_LDFLAGS) '$(LDFLAGS_PODMAN_STATIC)' \
		-tags "${REMOTETAGS}" \
		-o $@ ./cmd/podman

.PHONY: podman
podman: bin/podman

# This will map to the right thing on Linux, Windows, and Mac.
.PHONY: podman-remote
podman-remote: $(SRCBINDIR)/podman$(BINSFX)

$(SRCBINDIR)/quadlet: $(SOURCES) go.mod go.sum
	$(GOCMD) build \
		$(BUILDFLAGS) \
		$(GO_LDFLAGS) '$(LDFLAGS_PODMAN)' \
		-tags "${BUILDTAGS}" \
		-o $@ ./cmd/quadlet

.PHONY: quadlet
quadlet: bin/quadlet

PHONY: podman-remote-static-linux_amd64 podman-remote-static-linux_arm64
podman-remote-static-linux_amd64: $(SRCBINDIR)/podman-remote-static-linux_amd64
podman-remote-static-linux_arm64: $(SRCBINDIR)/podman-remote-static-linux_arm64

.PHONY: podman-winpath
podman-winpath: $(SOURCES) go.mod go.sum
	CGO_ENABLED=0 \
		GOOS=windows \
		$(GO) build \
		$(BUILDFLAGS) \
		-ldflags -H=windowsgui \
		-o bin/windows/winpath.exe \
		./cmd/winpath

.PHONY: podman-mac-helper
podman-mac-helper: ## Build podman-mac-helper for macOS
	CGO_ENABLED=0 \
		GOOS=darwin \
		GOARCH=$(GOARCH) \
		$(GO) build \
		$(BUILDFLAGS) \
		-o bin/darwin/podman-mac-helper \
		./cmd/podman-mac-helper

bin/rootlessport: $(SOURCES) go.mod go.sum
	CGO_ENABLED=$(CGO_ENABLED) \
		$(GO) build \
		$(BUILDFLAGS) \
		-o $@ ./cmd/rootlessport

.PHONY: rootlessport
rootlessport: bin/rootlessport

.PHONY: podman-remote-experimental
podman-remote-experimental: $(SRCBINDIR)/experimental/podman$(BINSFX)
$(SRCBINDIR)/experimental/podman$(BINSFX): $(SOURCES) go.mod go.sum | $(SRCBINDIR)
	$(GOCMD) build \
		$(BUILDFLAGS) \
		$(GO_LDFLAGS) '$(LDFLAGS_PODMAN)' \
		-tags "${REMOTETAGS} experimental" \
		-o $@ ./cmd/podman

###
### Secondary binary-build targets
###

.PHONY: generate-bindings
generate-bindings:
ifneq ($(GOOS),darwin)
	$(GOCMD) generate ./pkg/bindings/... ;
endif

# DO NOT USE: use local-cross instead
bin/podman.cross.%:
	TARGET="$*"; \
	GOOS="$${TARGET%%.*}"; \
	GOARCH="$${TARGET##*.}"; \
	CGO_ENABLED=0 \
		$(GO) build \
		$(BUILDFLAGS) \
		$(GO_LDFLAGS) '$(LDFLAGS_PODMAN)' \
		-tags '$(BUILDTAGS_CROSS)' \
		-o "$@" ./cmd/podman

.PHONY: local-cross
local-cross: $(CROSS_BUILD_TARGETS) ## Cross compile podman binary for multiple architectures

.PHONY: cross
cross: local-cross

.PHONY: build-no-cgo
build-no-cgo:
	BUILDTAGS="containers_image_openpgp exclude_graphdriver_btrfs \
		exclude_graphdriver_devicemapper exclude_disk_quota" \
	CGO_ENABLED=0 \
	$(MAKE) all

.PHONY: completions
completions: podman podman-remote
	# key = shell, value = completion filename
	declare -A outfiles=([bash]=%s [zsh]=_%s [fish]=%s.fish [powershell]=%s.ps1);\
	for shell in $${!outfiles[*]}; do \
	    for remote in "" "-remote"; do \
		podman="podman$$remote"; \
		outfile=$$(printf "completions/$$shell/$${outfiles[$$shell]}" $$podman); \
		./bin/$$podman completion $$shell >| $$outfile; \
	    done;\
	done

###
### Documentation targets
###

pkg/api/swagger.yaml:
	make -C pkg/api

$(MANPAGES_MD_GENERATED): %.md: %.md.in $(MANPAGES_SOURCE_DIR)/options/*.md
	hack/markdown-preprocess

$(MANPAGES): %: %.md .install.md2man docdir

# This does a bunch of filtering needed for man pages:
#  1. Strip markdown link targets like '[podman(1)](podman.1.md)'
#     to just '[podman(1)]', because man pages have no link mechanism;
#  2. Then remove the brackets: '[podman(1)]' -> 'podman(1)';
#  3. Then do the same for all other markdown links,
#     like '[cgroups(7)](https://.....)'  -> just 'cgroups(7)';
#  4. Remove HTML-ish stuff like '<sup>..</sup>' and '<a>..</a>'
#  5. Replace "\" (backslash) at EOL with two spaces (no idea why)
	@$(SED) -e 's/\((podman[^)]*\.md\(#.*\)\?)\)//g'    \
	       -e 's/\[\(podman[^]]*\)\]/\1/g'              \
	       -e 's/\[\([^]]*\)](http[^)]\+)/\1/g'         \
	       -e 's;<\(/\)\?\(a\|a\s\+[^>]*\|sup\)>;;g'    \
	       -e 's/\\$$/  /g' $<                         |\
	$(GOMD2MAN) -out $(subst source/markdown,build/man,$@)

.PHONY: docdir
docdir:
	mkdir -p docs/build/man

.PHONY: docs
docs: $(MANPAGES) ## Generate documentation

# docs/remote-docs.sh requires a locally executable 'podman-remote' binary
# in addition to the target-architecture binary (if different). That's
# what the NATIVE_GOOS make does in the first line.
podman-remote-%-docs: podman-remote
	$(MAKE) clean-binaries podman-remote GOOS=$(NATIVE_GOOS) GOARCH=$(NATIVE_GOARCH)
	$(eval GOOS := $*)
	$(MAKE) docs $(MANPAGES)
	rm -rf docs/build/remote
	mkdir -p docs/build/remote
	ln -sf $(CURDIR)/docs/source/markdown/links docs/build/man/
	docs/remote-docs.sh \
		$(GOOS) \
		docs/build/remote/$* \
		$(if $(findstring windows,$*),docs/source/markdown,docs/build/man)

.PHONY: man-page-check
man-page-check: bin/podman
	hack/man-page-checker
	hack/xref-helpmsgs-manpages

.PHONY: swagger-check
swagger-check:
	hack/swagger-check

.PHONY: swagger
swagger: pkg/api/swagger.yaml

.PHONY: docker-docs
docker-docs: docs
	(cd docs; ./dckrman.sh ./build/man/*.1)

# Workaround vim syntax highlighting bug: "

###
### Utility and Testing targets
###

.PHONY: validate.completions
validate.completions: SHELL:=/usr/bin/env bash # Set shell to bash for this target
validate.completions:
	# Check if the files can be loaded by the shell
	. completions/bash/podman
	if [ -x /bin/zsh ]; then /bin/zsh completions/zsh/_podman; fi
	if [ -x /bin/fish ]; then /bin/fish completions/fish/podman.fish; fi

# Note: Assumes test/python/requirements.txt is installed & available
.PHONY: run-docker-py-tests
run-docker-py-tests:
	touch test/__init__.py
	env CONTAINERS_CONF=$(CURDIR)/test/apiv2/containers.conf pytest --disable-warnings test/python/docker/
	rm -f test/__init__.py

.PHONY: localunit
localunit: test/goecho/goecho test/version/version
	rm -rf ${COVERAGE_PATH} && mkdir -p ${COVERAGE_PATH}
	UNIT=1 $(GINKGO) \
		-r \
		$(TESTFLAGS) \
		--skipPackage test/e2e,pkg/apparmor,pkg/bindings,hack,pkg/machine/e2e \
		--cover \
		--covermode atomic \
		--coverprofile coverprofile \
		--outputdir ${COVERAGE_PATH} \
		--tags "$(BUILDTAGS)" \
		--succinct
	$(GO) tool cover -html=${COVERAGE_PATH}/coverprofile -o ${COVERAGE_PATH}/coverage.html
	$(GO) tool cover -func=${COVERAGE_PATH}/coverprofile > ${COVERAGE_PATH}/functions
	cat ${COVERAGE_PATH}/functions | sed -n 's/\(total:\).*\([0-9][0-9].[0-9]\)/\1 \2/p'

.PHONY: test
test: localunit localintegration remoteintegration localsystem remotesystem  ## Run unit, integration, and system tests.

.PHONY: ginkgo-run
ginkgo-run: .install.ginkgo
	ACK_GINKGO_RC=true $(GINKGO) version
	ACK_GINKGO_RC=true $(GINKGO) -v $(TESTFLAGS) -tags "$(TAGS) remote" $(GINKGOTIMEOUT) -cover -flakeAttempts 3 -progress -trace -noColor -nodes $(GINKGONODES) -debug $(GINKGOWHAT) $(HACK)

.PHONY: ginkgo
ginkgo:
	$(MAKE) ginkgo-run TAGS="$(BUILDTAGS)" HACK=hack/.

.PHONY: ginkgo-remote
ginkgo-remote:
	$(MAKE) ginkgo-run TAGS="$(REMOTETAGS) remote_testing" HACK=

.PHONY: testbindings
testbindings: .install.ginkgo
	ACK_GINKGO_RC=true $(GINKGO) -v $(TESTFLAGS) -tags "$(TAGS) remote" $(GINKGOTIMEOUT) -progress -trace -noColor -debug -timeout 30m  -v -r ./pkg/bindings/test

.PHONY: localintegration
localintegration: test-binaries ginkgo

.PHONY: remoteintegration
remoteintegration: test-binaries ginkgo-remote

.PHONY: localmachine
localmachine: test-binaries .install.ginkgo
	$(MAKE) ginkgo-run GINKGONODES=1 GINKGOWHAT=pkg/machine/e2e/. HACK=

.PHONY: localbenchmarks
localbenchmarks: install.tools test-binaries
	PATH=$(PATH):$(shell pwd)/hack ACK_GINKGO_RC=true $(GINKGO) \
		      -focus "Podman Benchmark Suite" \
		      -tags "$(BUILDTAGS) benchmarks" -noColor \
		      -noisySkippings=false -noisyPendings=false \
		      test/e2e/.

.PHONY: localsystem
localsystem:
	# Wipe existing config, database, and cache: start with clean slate.
	$(RM) -rf ${HOME}/.local/share/containers ${HOME}/.config/containers
	if timeout -v 1 true; then PODMAN=$(CURDIR)/bin/podman QUADLET=$(CURDIR)/bin/quadlet bats test/system/; else echo "Skipping $@: 'timeout -v' unavailable'"; fi

.PHONY: remotesystem
remotesystem:
	# Wipe existing config, database, and cache: start with clean slate.
	$(RM) -rf ${HOME}/.local/share/containers ${HOME}/.config/containers
	# Start podman server using tmp socket; loop-wait for it;
	# test podman-remote; kill server, clean up tmp socket file.
	# podman server spews copious unhelpful output; ignore it.
	rc=0;\
	if timeout -v 1 true; then \
		SOCK_FILE=$(shell mktemp --dry-run --tmpdir podman_tmp_XXXX);\
		export PODMAN_SOCKET=unix:$$SOCK_FILE; \
		./bin/podman system service --timeout=0 $$PODMAN_SOCKET > $(if $(PODMAN_SERVER_LOG),$(PODMAN_SERVER_LOG),/dev/null) 2>&1 & \
		retry=5;\
		while [ $$retry -ge 0 ]; do\
			echo Waiting for server...;\
			sleep 1;\
			./bin/podman-remote --url $$PODMAN_SOCKET info >/dev/null 2>&1 && break;\
			retry=$$(expr $$retry - 1);\
		done;\
		if [ $$retry -lt 0 ]; then\
			echo "Error: ./bin/podman system service did not come up on $$SOCK_FILE" >&2;\
			exit 1;\
		fi;\
		env PODMAN="$(CURDIR)/bin/podman-remote --url $$PODMAN_SOCKET" bats test/system/ ;\
		rc=$$?;\
		kill %1;\
		rm -f $$SOCK_FILE;\
	else \
		echo "Skipping $@: 'timeout -v' unavailable'";\
	fi;\
	exit $$rc

.PHONY: localapiv2-bash
localapiv2-bash:
	env PODMAN=./bin/podman stdbuf -o0 -e0 ./test/apiv2/test-apiv2

.PHONY: localapiv2-python
localapiv2-python:
	env CONTAINERS_CONF=$(CURDIR)/test/apiv2/containers.conf PODMAN=./bin/podman \
		pytest --verbose --disable-warnings ./test/apiv2/python
	touch test/__init__.py
	env CONTAINERS_CONF=$(CURDIR)/test/apiv2/containers.conf PODMAN=./bin/podman \
		pytest --verbose --disable-warnings ./test/python/docker
	rm -f test/__init__.py

# Order is important running python tests first causes the bash tests
# to fail, see 12-imagesMore.  FIXME order of tests should not matter
.PHONY: localapiv2
localapiv2: localapiv2-bash localapiv2-python

.PHONY: remoteapiv2
remoteapiv2:
	true

.PHONY: system.test-binary
system.test-binary: .install.ginkgo
	$(GO) test -c ./test/system

.PHONY: test-binaries
test-binaries: test/checkseccomp/checkseccomp test/goecho/goecho install.catatonit test/version/version
	@echo "Canonical source version: $(call err_if_empty,RELEASE_VERSION)"

.PHONY: tests-included
tests-included:
	contrib/cirrus/pr-should-include-tests

.PHONY: tests-expect-exit
tests-expect-exit:
	@if egrep --line-number 'Expect.*ExitCode' test/e2e/*.go | egrep -v ', ".*"\)'; then \
		echo "^^^ Unhelpful use of Expect(ExitCode())"; \
		echo "   Please use '.Should(Exit(...))' pattern instead."; \
		echo "   If that's not possible, please add an annotation (description) to your assertion:"; \
		echo "        Expect(...).To(..., \"Friendly explanation of this check\")"; \
		exit 1; \
	fi

.PHONY: pr-removes-fixed-skips
pr-removes-fixed-skips:
	contrib/cirrus/pr-removes-fixed-skips

###
### Release/Packaging targets
###

.PHONY: podman-release
podman-release: podman-release-$(GOARCH).tar.gz  # Build all Linux binaries for $GOARCH, docs., and installation tree, into a tarball.

# The following two targets are nuanced and complex:
# Cross-building the podman-remote documentation requires a functional
# native architecture executable.  However `make` only deals with
# files/timestamps, it doesn't understand if an existing binary will
# function on the system or not.  This makes building cross-platform
# releases incredibly accident-prone and fragile.  The only practical
# way to deal with this, is via multiple conditional (nested) `make`
# calls along with careful manipulation of `$GOOS` and `$GOARCH`.

podman-release-%.tar.gz: test/version/version
	$(eval TMPDIR := $(shell mktemp -d podman_tmp_XXXX))
	$(eval SUBDIR := podman-v$(call err_if_empty,RELEASE_NUMBER))
	$(eval _DSTARGS := "DESTDIR=$(TMPDIR)/$(SUBDIR)" "PREFIX=/usr")
	$(eval GOARCH := $*)
	mkdir -p "$(call err_if_empty,TMPDIR)/$(SUBDIR)"
	$(MAKE) GOOS=$(GOOS) GOARCH=$(NATIVE_GOARCH) \
		clean-binaries docs podman-remote-$(GOOS)-docs
	if [[ "$(GOARCH)" != "$(NATIVE_GOARCH)" ]]; then \
		$(MAKE) CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) \
			BUILDTAGS="$(BUILDTAGS_CROSS)" clean-binaries binaries; \
	else \
		$(MAKE) GOOS=$(GOOS) GOARCH=$(GOARCH) binaries; \
	fi
	$(MAKE) $(_DSTARGS) install.bin install.remote install.man install.systemd
	tar -czvf $@ --xattrs -C "$(TMPDIR)" "./$(SUBDIR)"
	if [[ "$(GOARCH)" != "$(NATIVE_GOARCH)" ]]; then $(MAKE) clean-binaries; fi
	-rm -rf "$(TMPDIR)"

podman-remote-release-%.zip: test/version/version ## Build podman-remote for %=$GOOS_$GOARCH, and docs. into an installation zip.
	$(eval TMPDIR := $(shell mktemp -d podman_tmp_XXXX))
	$(eval SUBDIR := podman-$(call err_if_empty,RELEASE_NUMBER))
	$(eval _DSTARGS := "DESTDIR=$(TMPDIR)/$(SUBDIR)" "PREFIX=/usr")
	$(eval GOOS := $(firstword $(subst _, ,$*)))
	$(eval GOARCH := $(lastword $(subst _, ,$*)))
	$(eval _GOPLAT := GOOS=$(call err_if_empty,GOOS) GOARCH=$(call err_if_empty,GOARCH))
	mkdir -p "$(call err_if_empty,TMPDIR)/$(SUBDIR)"
	$(MAKE) GOOS=$(GOOS) GOARCH=$(GOARCH) \
		clean-binaries podman-remote-$(GOOS)-docs
	if [[ "$(GOARCH)" != "$(NATIVE_GOARCH)" ]]; then \
		$(MAKE) CGO_ENABLED=0 $(GOPLAT) BUILDTAGS="$(BUILDTAGS_CROSS)" \
			clean-binaries podman-remote; \
	else \
		$(MAKE) $(GOPLAT) podman-remote; \
	fi
	if [[ "$(GOOS)" == "windows" ]]; then \
		$(MAKE) $(GOPLAT) TMPDIR="" win-gvproxy; \
	fi
	if [[ "$(GOOS)" == "darwin" ]]; then \
		$(MAKE) $(GOPLAT) podman-mac-helper;\
	fi
	cp -r ./docs/build/remote/$(GOOS) "$(TMPDIR)/$(SUBDIR)/docs/"
	cp ./contrib/remote/containers.conf "$(TMPDIR)/$(SUBDIR)/"
	$(MAKE) $(GOPLAT) $(_DSTARGS) SELINUXOPT="" install.remote
	cd "$(TMPDIR)" && \
		zip --recurse-paths "$(CURDIR)/$@" "./$(SUBDIR)"
	if [[ "$(GOARCH)" != "$(NATIVE_GOARCH)" ]]; then $(MAKE) clean-binaries; fi
	-rm -rf "$(TMPDIR)"

podman.msi: test/version/version  ## Build podman-remote, package for installation on Windows
	$(MAKE) podman-v$(call err_if_empty,RELEASE_NUMBER).msi
	cp podman-v$(call err_if_empty,RELEASE_NUMBER).msi podman.msi

podman-v%.msi: test/version/version
# Passing explicitly OS and ARCH, because ARM is not supported by wixl https://gitlab.gnome.org/GNOME/msitools/-/blob/master/tools/wixl/builder.vala#L3
	$(MAKE) GOOS=windows GOARCH=amd64 podman-remote-windows-docs
	$(MAKE) GOOS=windows GOARCH=amd64 clean-binaries podman-remote podman-winpath win-gvproxy
	$(eval DOCFILE := docs/build/remote/windows)
	find $(DOCFILE) -print | \
		wixl-heat --var var.ManSourceDir --component-group ManFiles \
		--directory-ref INSTALLDIR --prefix $(DOCFILE)/ > \
			$(DOCFILE)/pages.wsx
	wixl -D VERSION=$(call err_if_empty,RELEASE_VERSION) -D ManSourceDir=$(DOCFILE) \
		-o $@ contrib/msi/podman.wxs $(DOCFILE)/pages.wsx --arch x64

# Checks out and builds win-sshproxy helper. See comment on GV_GITURL declaration
.PHONY: win-gvproxy
win-gvproxy: test/version/version
	rm -rf tmp-gv; mkdir tmp-gv
	(cd tmp-gv; \
         git init; \
         git remote add origin $(GV_GITURL); \
         git fetch --depth 1 origin $(GV_SHA); \
         git checkout FETCH_HEAD; make win-gvproxy win-sshproxy)
	mkdir -p bin/windows/
	cp tmp-gv/bin/win-sshproxy.exe bin/windows/
	cp tmp-gv/bin/gvproxy.exe bin/windows/
	rm -rf tmp-gv

.PHONY: package
package:  ## Build rpm packages
	rpkg local

###
### Installation targets
###

# Remember that rpms install exec to /usr/bin/podman while a `make install`
# installs them to /usr/local/bin/podman which is likely before. Always use
# a full path to test installed podman or you risk to call another executable.
.PHONY: package-install
package-install: package  ## Install rpm packages
	sudo $(call err_if_empty,PKG_MANAGER) -y install ${HOME}/rpmbuild/RPMS/*/*.rpm
	/usr/bin/podman version
	/usr/bin/podman info  # will catch a broken conmon

.PHONY: install
install: install.bin install.remote install.man install.systemd  ## Install binaries to system locations

.PHONY: install.catatonit
install.catatonit:
	./hack/install_catatonit.sh

.PHONY: install.remote
install.remote:
	install ${SELINUXOPT} -d -m 755 $(DESTDIR)$(BINDIR)
	install ${SELINUXOPT} -m 755 $(SRCBINDIR)/podman$(BINSFX) \
		$(DESTDIR)$(BINDIR)/podman$(BINSFX)
	test "${GOOS}" != "windows" || \
		install -m 755 $(SRCBINDIR)/win-sshproxy.exe $(DESTDIR)$(BINDIR)
	test "${GOOS}" != "windows" || \
		install -m 755 $(SRCBINDIR)/gvproxy.exe $(DESTDIR)$(BINDIR)
	test "${GOOS}" != "darwin" || \
		install -m 755 $(SRCBINDIR)/podman-mac-helper $(DESTDIR)$(BINDIR)
	test -z "${SELINUXOPT}" || \
		chcon --verbose --reference=$(DESTDIR)$(BINDIR)/podman-remote \
		bin/podman-remote

.PHONY: install.bin
install.bin:
	install ${SELINUXOPT} -d -m 755 $(DESTDIR)$(BINDIR)
	install ${SELINUXOPT} -m 755 bin/podman $(DESTDIR)$(BINDIR)/podman
	test -z "${SELINUXOPT}" || chcon --verbose --reference=$(DESTDIR)$(BINDIR)/podman bin/podman
	install ${SELINUXOPT} -d -m 755 $(DESTDIR)$(LIBEXECPODMAN)
ifneq ($(shell uname -s),FreeBSD)
	install ${SELINUXOPT} -m 755 bin/rootlessport $(DESTDIR)$(LIBEXECPODMAN)/rootlessport
	test -z "${SELINUXOPT}" || chcon --verbose --reference=$(DESTDIR)$(LIBEXECPODMAN)/rootlessport bin/rootlessport
	install ${SELINUXOPT} -m 755 bin/quadlet $(DESTDIR)$(LIBEXECPODMAN)/quadlet
	install ${SELINUXOPT} -d -m 755 $(DESTDIR)$(SYSTEMDGENERATORSDIR)
	ln -sfr $(DESTDIR)$(LIBEXECPODMAN)/quadlet $(DESTDIR)$(SYSTEMDGENERATORSDIR)/podman-system-generator
	install ${SELINUXOPT} -d -m 755 $(DESTDIR)$(USERSYSTEMDGENERATORSDIR)
	ln -sfr $(DESTDIR)$(LIBEXECPODMAN)/quadlet $(DESTDIR)$(USERSYSTEMDGENERATORSDIR)/podman-user-generator
	install ${SELINUXOPT} -m 755 -d ${DESTDIR}${TMPFILESDIR}
	install ${SELINUXOPT} -m 644 contrib/tmpfile/podman.conf ${DESTDIR}${TMPFILESDIR}/podman.conf
endif

.PHONY: install.modules-load
install.modules-load: # This should only be used by distros which might use iptables-legacy, this is not needed on RHEL
	install ${SELINUXOPT} -m 755 -d ${DESTDIR}${MODULESLOADDIR}
	install ${SELINUXOPT} -m 644 contrib/modules-load.d/podman-iptables.conf ${DESTDIR}${MODULESLOADDIR}/podman-iptables.conf

.PHONY: install.man
install.man:
	install ${SELINUXOPT} -d -m 755 $(DESTDIR)$(MANDIR)/man1
	install ${SELINUXOPT} -m 644 $(filter %.1,$(MANPAGES_DEST)) $(DESTDIR)$(MANDIR)/man1
	install ${SELINUXOPT} -m 644 docs/source/markdown/links/*1 $(DESTDIR)$(MANDIR)/man1

.PHONY: install.completions
install.completions:
	install ${SELINUXOPT} -d -m 755 ${DESTDIR}${BASHINSTALLDIR}
	install ${SELINUXOPT} -m 644 completions/bash/podman ${DESTDIR}${BASHINSTALLDIR}
	install ${SELINUXOPT} -m 644 completions/bash/podman-remote ${DESTDIR}${BASHINSTALLDIR}
	install ${SELINUXOPT} -d -m 755 ${DESTDIR}${ZSHINSTALLDIR}
	install ${SELINUXOPT} -m 644 completions/zsh/_podman ${DESTDIR}${ZSHINSTALLDIR}
	install ${SELINUXOPT} -m 644 completions/zsh/_podman-remote ${DESTDIR}${ZSHINSTALLDIR}
	install ${SELINUXOPT} -d -m 755 ${DESTDIR}${FISHINSTALLDIR}
	install ${SELINUXOPT} -m 644 completions/fish/podman.fish ${DESTDIR}${FISHINSTALLDIR}
	install ${SELINUXOPT} -m 644 completions/fish/podman-remote.fish ${DESTDIR}${FISHINSTALLDIR}
	# There is no common location for powershell files so do not install them. Users have to source the file from their powershell profile.

.PHONY: install.docker
install.docker:
	install ${SELINUXOPT} -d -m 755 $(DESTDIR)$(BINDIR)
	install ${SELINUXOPT} -m 755 docker $(DESTDIR)$(BINDIR)/docker
	install ${SELINUXOPT} -m 755 -d ${DESTDIR}${SYSTEMDDIR}  ${DESTDIR}${USERSYSTEMDDIR} ${DESTDIR}${TMPFILESDIR} ${DESTDIR}${USERTMPFILESDIR}
	install ${SELINUXOPT} -m 644 contrib/systemd/system/podman-docker.conf -t ${DESTDIR}${TMPFILESDIR}
	install ${SELINUXOPT} -m 644 contrib/systemd/system/podman-docker.conf -t ${DESTDIR}${USERTMPFILESDIR}

.PHONY: install.docker-docs
install.docker-docs:
	install ${SELINUXOPT} -d -m 755 $(DESTDIR)$(MANDIR)/man1
	install ${SELINUXOPT} -m 644 docs/build/man/docker*.1 -t $(DESTDIR)$(MANDIR)/man1
	install ${SELINUXOPT} -d -m 755 $(DESTDIR)$(MANDIR)/man5
	install ${SELINUXOPT} -m 644 docs/build/man/docker*.5 -t $(DESTDIR)$(MANDIR)/man5

.PHONY: install.docker-full
install.docker-full: install.docker install.docker-docs

.PHONY: install.systemd
ifneq (,$(findstring systemd,$(BUILDTAGS)))
PODMAN_UNIT_FILES = contrib/systemd/auto-update/podman-auto-update.service \
		    contrib/systemd/system/podman.service \
		    contrib/systemd/system/podman-restart.service \
		    contrib/systemd/system/podman-kube@.service \
		    contrib/systemd/system/podman-clean-transient.service

%.service: %.service.in
	sed -e 's;@@PODMAN@@;$(BINDIR)/podman;g' $< >$@.tmp.$$ \
		&& mv -f $@.tmp.$$ $@

install.systemd: $(PODMAN_UNIT_FILES)
	install ${SELINUXOPT} -m 755 -d ${DESTDIR}${SYSTEMDDIR}  ${DESTDIR}${USERSYSTEMDDIR}
	# User services
	install ${SELINUXOPT} -m 644 contrib/systemd/auto-update/podman-auto-update.service ${DESTDIR}${USERSYSTEMDDIR}/podman-auto-update.service
	install ${SELINUXOPT} -m 644 contrib/systemd/auto-update/podman-auto-update.timer ${DESTDIR}${USERSYSTEMDDIR}/podman-auto-update.timer
	install ${SELINUXOPT} -m 644 contrib/systemd/system/podman.socket ${DESTDIR}${USERSYSTEMDDIR}/podman.socket
	install ${SELINUXOPT} -m 644 contrib/systemd/system/podman.service ${DESTDIR}${USERSYSTEMDDIR}/podman.service
	install ${SELINUXOPT} -m 644 contrib/systemd/system/podman-restart.service ${DESTDIR}${USERSYSTEMDDIR}/podman-restart.service
	install ${SELINUXOPT} -m 644 contrib/systemd/system/podman-kube@.service ${DESTDIR}${USERSYSTEMDDIR}/podman-kube@.service
	# System services
	install ${SELINUXOPT} -m 644 contrib/systemd/auto-update/podman-auto-update.service ${DESTDIR}${SYSTEMDDIR}/podman-auto-update.service
	install ${SELINUXOPT} -m 644 contrib/systemd/auto-update/podman-auto-update.timer ${DESTDIR}${SYSTEMDDIR}/podman-auto-update.timer
	install ${SELINUXOPT} -m 644 contrib/systemd/system/podman.socket ${DESTDIR}${SYSTEMDDIR}/podman.socket
	install ${SELINUXOPT} -m 644 contrib/systemd/system/podman.service ${DESTDIR}${SYSTEMDDIR}/podman.service
	install ${SELINUXOPT} -m 644 contrib/systemd/system/podman-restart.service ${DESTDIR}${SYSTEMDDIR}/podman-restart.service
	install ${SELINUXOPT} -m 644 contrib/systemd/system/podman-kube@.service ${DESTDIR}${SYSTEMDDIR}/podman-kube@.service
	install ${SELINUXOPT} -m 644 contrib/systemd/system/podman-clean-transient.service ${DESTDIR}${SYSTEMDDIR}/podman-clean-transient.service
	rm -f $(PODMAN_UNIT_FILES)
else
install.systemd:
endif

.PHONY: install.tools
install.tools: .install.golangci-lint ## Install needed tools
	$(MAKE) -C test/tools

.PHONY: .install.goimports
.install.goimports:
	$(MAKE) -C test/tools build/goimports

.PHONY: .install.ginkgo
.install.ginkgo:
	$(MAKE) -C test/tools build/ginkgo

.PHONY: .install.gitvalidation
.install.gitvalidation:
	$(MAKE) -C test/tools build/git-validation

.PHONY: .install.golangci-lint
.install.golangci-lint:
	VERSION=1.50.1 ./hack/install_golangci.sh

.PHONY: .install.swagger
.install.swagger:
	env VERSION=0.30.3 \
		BINDIR=$(BINDIR) \
		GOOS=$(GOOS) \
		GOARCH=$(GOARCH) \
		./hack/install_swagger.sh

.PHONY: .install.md2man
.install.md2man:
	if [ ! -x "$(GOMD2MAN)" ]; then \
		$(MAKE) -C test/tools build/go-md2man ; \
	fi

.PHONY: .install.pre-commit
.install.pre-commit:
	if [ -z "$(PRE_COMMIT)" ]; then \
		$(PYTHON) -m pip install --user pre-commit; \
	fi

.PHONY: uninstall
uninstall:
	for i in $(filter %.1,$(MANPAGES_DEST)); do \
		rm -f $(DESTDIR)$(MANDIR)/man1/$$(basename $${i}); \
	done; \
	for i in $(filter %.5,$(MANPAGES_DEST)); do \
		rm -f $(DESTDIR)$(MANDIR)/man5/$$(basename $${i}); \
	done
	# Remove podman and remote bin
	rm -f $(DESTDIR)$(BINDIR)/podman
	rm -f $(DESTDIR)$(BINDIR)/podman-remote
	# Remove related config files
	rm -f ${DESTDIR}${ETCDIR}/cni/net.d/87-podman-bridge.conflist
	rm -f ${DESTDIR}${TMPFILESDIR}/podman.conf
	rm -f ${DESTDIR}${SYSTEMDDIR}/io.podman.socket
	rm -f ${DESTDIR}${USERSYSTEMDDIR}/io.podman.socket
	rm -f ${DESTDIR}${SYSTEMDDIR}/io.podman.service
	rm -f ${DESTDIR}${SYSTEMDDIR}/podman.service
	rm -f ${DESTDIR}${SYSTEMDDIR}/podman.socket
	rm -f ${DESTDIR}${USERSYSTEMDDIR}/podman.socket
	rm -f ${DESTDIR}${USERSYSTEMDDIR}/podman.service

.PHONY: clean-binaries
clean-binaries: ## Remove platform/architecture specific binary files
	rm -rf \
		bin

.PHONY: clean
clean: clean-binaries ## Clean all make artifacts
	rm -rf \
		_output \
		$(wildcard podman-*.msi) \
		$(wildcard podman-remote*.zip) \
		$(wildcard podman_tmp_*) \
		$(wildcard podman*.tar.gz) \
		build \
		test/checkseccomp/checkseccomp \
		test/goecho/goecho \
		test/version/version \
		test/__init__.py \
		test/testdata/redis-image \
		libpod/container_ffjson.go \
		libpod/pod_ffjson.go \
		libpod/container_easyjson.go \
		libpod/pod_easyjson.go \
		docs/build \
		.venv
	make -C docs clean
