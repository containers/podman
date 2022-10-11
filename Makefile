export GOPROXY=https://proxy.golang.org

GO ?= go
COVERAGE_PATH ?= .coverage
DESTDIR ?=
EPOCH_TEST_COMMIT ?= $(shell git merge-base $${DEST_BRANCH:-master} HEAD)
HEAD ?= HEAD
CHANGELOG_BASE ?= HEAD~
CHANGELOG_TARGET ?= HEAD
PROJECT := github.com/containers/podman
GIT_BASE_BRANCH ?= origin/master
GIT_BRANCH ?= $(shell git rev-parse --abbrev-ref HEAD 2>/dev/null)
GIT_BRANCH_CLEAN ?= $(shell echo $(GIT_BRANCH) | sed -e "s/[^[:alnum:]]/-/g")
LIBPOD_INSTANCE := libpod_dev
PREFIX ?= /usr/local
BINDIR ?= ${PREFIX}/bin
LIBEXECDIR ?= ${PREFIX}/libexec
MANDIR ?= ${PREFIX}/share/man
SHAREDIR_CONTAINERS ?= ${PREFIX}/share/containers
ETCDIR ?= /etc
TMPFILESDIR ?= ${PREFIX}/lib/tmpfiles.d
SYSTEMDDIR ?= ${PREFIX}/lib/systemd/system
USERSYSTEMDDIR ?= ${PREFIX}/lib/systemd/user
REMOTETAGS ?= remote exclude_graphdriver_btrfs btrfs_noversion exclude_graphdriver_devicemapper containers_image_openpgp
BUILDTAGS ?= \
	$(shell hack/apparmor_tag.sh) \
	$(shell hack/btrfs_installed_tag.sh) \
	$(shell hack/btrfs_tag.sh) \
	$(shell hack/selinux_tag.sh) \
	$(shell hack/systemd_tag.sh) \
	exclude_graphdriver_devicemapper \
	seccomp
PYTHON ?= $(shell command -v python3 python|head -n1)
PKG_MANAGER ?= $(shell command -v dnf yum|head -n1)
# ~/.local/bin is not in PATH on all systems
PRE_COMMIT = $(shell command -v bin/venv/bin/pre-commit ~/.local/bin/pre-commit pre-commit | head -n1)

SOURCES = $(shell find . -path './.*' -prune -o -name "*.go")

BUILDFLAGS := -mod=vendor $(BUILDFLAGS)

BUILDTAGS_CROSS ?= containers_image_openpgp exclude_graphdriver_btrfs exclude_graphdriver_devicemapper exclude_graphdriver_overlay
CONTAINER_RUNTIME := $(shell command -v podman 2> /dev/null || echo docker)
OCI_RUNTIME ?= ""

BASHINSTALLDIR=${PREFIX}/share/bash-completion/completions
ZSHINSTALLDIR=${PREFIX}/share/zsh/site-functions
FISHINSTALLDIR=${PREFIX}/share/fish/vendor_completions.d

SELINUXOPT ?= $(shell test -x /usr/sbin/selinuxenabled && selinuxenabled && echo -Z)

COMMIT_NO ?= $(shell git rev-parse HEAD 2> /dev/null || true)
GIT_COMMIT ?= $(if $(shell git status --porcelain --untracked-files=no),${COMMIT_NO}-dirty,${COMMIT_NO})
DATE_FMT = %s
ifdef SOURCE_DATE_EPOCH
	BUILD_INFO ?= $(shell date -u -d "@$(SOURCE_DATE_EPOCH)" "+$(DATE_FMT)" 2>/dev/null || date -u -r "$(SOURCE_DATE_EPOCH)" "+$(DATE_FMT)" 2>/dev/null || date -u "+$(DATE_FMT)")
	ISODATE ?= $(shell date -d "@$(SOURCE_DATE_EPOCH)" --iso-8601)
else
	BUILD_INFO ?= $(shell date "+$(DATE_FMT)")
	ISODATE ?= $(shell date --iso-8601)
endif
LIBPOD := ${PROJECT}/v3/libpod
GCFLAGS ?= all=-trimpath=${PWD}
ASMFLAGS ?= all=-trimpath=${PWD}
LDFLAGS_PODMAN ?= \
	  -X $(LIBPOD)/define.gitCommit=$(GIT_COMMIT) \
	  -X $(LIBPOD)/define.buildInfo=$(BUILD_INFO) \
	  -X $(LIBPOD)/config._installPrefix=$(PREFIX) \
	  -X $(LIBPOD)/config._etcDir=$(ETCDIR) \
	  $(EXTRA_LDFLAGS)
#Update to LIBSECCOMP_COMMIT should reflect in Dockerfile too.
LIBSECCOMP_COMMIT := v2.3.3
# Rarely if ever should integration tests take more than 50min,
# caller may override in special circumstances if needed.
GINKGOTIMEOUT ?= -timeout=90m

RELEASE_VERSION ?= $(shell hack/get_release_info.sh VERSION)
RELEASE_NUMBER ?= $(shell hack/get_release_info.sh NUMBER|sed -e 's/^v\(.*\)/\1/')
RELEASE_DIST ?= $(shell hack/get_release_info.sh DIST)
RELEASE_DIST_VER ?= $(shell hack/get_release_info.sh DIST_VER)
RELEASE_ARCH ?= $(shell hack/get_release_info.sh ARCH)
RELEASE_BASENAME := $(shell hack/get_release_info.sh BASENAME)

# If non-empty, logs all output from server during remote system testing
PODMAN_SERVER_LOG ?=

# If GOPATH not specified, use one in the local directory
ifeq ($(GOPATH),)
export GOPATH := $(HOME)/go
unexport GOBIN
endif
FIRST_GOPATH := $(firstword $(subst :, ,$(GOPATH)))
GOPKGDIR := $(FIRST_GOPATH)/src/$(PROJECT)
GOPKGBASEDIR ?= $(shell dirname "$(GOPKGDIR)")

GOBIN := $(shell $(GO) env GOBIN)
ifeq ($(GOBIN),)
GOBIN := $(FIRST_GOPATH)/bin
endif

export PATH := $(PATH):$(GOBIN)

GOMD2MAN ?= $(shell command -v go-md2man || echo '$(GOBIN)/go-md2man')

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
	bin/podman.cross.linux.mips64le

.PHONY: all
all: binaries docs

.PHONY: default
default: help

define PRINT_HELP_PYSCRIPT
import re, sys

print("Usage: make <target>")
cmds = {}
for line in sys.stdin:
	match = re.match(r'^([a-zA-Z_-]+):.*?## (.*)$$', line)
	if match:
	  target, help = match.groups()
	  cmds.update({target: help})
for cmd in sorted(cmds):
		print(" * '%s' - %s" % (cmd, cmds[cmd]))
endef
export PRINT_HELP_PYSCRIPT

# Dereference variable $(1), return value if non-empty, otherwise raise an error.
err_if_empty = $(if $(strip $($(1))),$(strip $($(1))),$(error Required variable $(1) value is undefined, whitespace, or empty))

.PHONY: help
ifneq (, ${PYTHON})
help:
	@$(PYTHON) -c "$$PRINT_HELP_PYSCRIPT" < $(MAKEFILE_LIST)
else
help:
	$(error python required for 'make help', executable not found)
endif

.gopathok:
ifeq ("$(wildcard $(GOPKGDIR))","")
	mkdir -p "$(GOPKGBASEDIR)"
	ln -sfn "$(CURDIR)" "$(GOPKGDIR)"
endif
	touch $@

.PHONY: lint
lint: golangci-lint
	@echo "Linting vs commit '$(call err_if_empty,EPOCH_TEST_COMMIT)'"
ifeq ($(PRE_COMMIT),)
	@echo "FATAL: pre-commit was not found, make .install.pre-commit to installing it." >&2
	@exit 2
endif
	$(PRE_COMMIT) run -a

.PHONY: golangci-lint
golangci-lint: .gopathok .install.golangci-lint
	hack/golangci-lint.sh run

.PHONY: gofmt
gofmt: ## Verify the source code gofmt
	find . -name '*.go' -type f \
		-not \( \
			-name '.golangci.yml' -o \
			-name 'Makefile' -o \
			-path './vendor/*' -prune -o \
			-path './contrib/*' -prune \
		\) -exec gofmt -d -e -s -w {} \+
	git diff --exit-code

.PHONY: test/checkseccomp/checkseccomp
test/checkseccomp/checkseccomp: .gopathok $(wildcard test/checkseccomp/*.go)
	$(GO) build $(BUILDFLAGS) -ldflags '$(LDFLAGS_PODMAN)' -tags "$(BUILDTAGS)" -o $@ ./test/checkseccomp

.PHONY: test/testvol/testvol
test/testvol/testvol: .gopathok $(wildcard test/testvol/*.go)
	$(GO) build $(BUILDFLAGS) -ldflags '$(LDFLAGS_PODMAN)' -o $@ ./test/testvol

.PHONY: volume-plugin-test-image
volume-plugin-test-img:
	podman build -t quay.io/libpod/volume-plugin-test-img -f Containerfile-testvol .

.PHONY: test/goecho/goecho
test/goecho/goecho: .gopathok $(wildcard test/goecho/*.go)
	$(GO) build $(BUILDFLAGS) -ldflags '$(LDFLAGS_PODMAN)' -o $@ ./test/goecho


.PHONY: bin/podman
bin/podman: .gopathok $(SOURCES) go.mod go.sum ## Build with podman
# Make sure to warn in case we're building without the systemd buildtag.
ifeq (,$(findstring systemd,$(BUILDTAGS)))
	@echo "Podman is being compiled without the systemd build tag. Install libsystemd on \
		Ubuntu or systemd-devel on rpm based distro for journald support."
endif
	$(GO) build $(BUILDFLAGS) -gcflags '$(GCFLAGS)' -asmflags '$(ASMFLAGS)' -ldflags '$(LDFLAGS_PODMAN)' -tags "$(BUILDTAGS)" -o $@ ./cmd/podman

.PHONY: podman
podman: bin/podman

.PHONY: bin/podman-remote
bin/podman-remote: .gopathok $(SOURCES) go.mod go.sum ## Build with podman on remote environment
	$(GO) build $(BUILDFLAGS) -gcflags '$(GCFLAGS)' -asmflags '$(ASMFLAGS)' -ldflags '$(LDFLAGS_PODMAN)' -tags "${REMOTETAGS}" -o $@ ./cmd/podman

.PHONY: bin/podman-remote-static
podman-remote-static: bin/podman-remote-static
	CGO_ENABLED=0 $(GO) build $(BUILDFLAGS) -gcflags '$(GCFLAGS)' -asmflags '$(ASMFLAGS)' -ldflags '$(LDFLAGS_PODMAN_STATIC)' -tags "${REMOTETAGS}" -o bin/podman-remote-static ./cmd/podman

.PHONY: podman-remote
podman-remote: bin/podman-remote

.PHONY: podman.msi
podman.msi: podman-remote podman-remote-windows install-podman-remote-windows-docs ## Will always rebuild exe as there is no podman-remote-windows.exe target to verify timestamp
	$(eval DOCFILE := docs/build/remote/windows)
	find $(DOCFILE) -print \
	|wixl-heat --var var.ManSourceDir --component-group ManFiles --directory-ref INSTALLDIR --prefix $(DOCFILE)/ >$(DOCFILE)/pages.wsx
	wixl -D VERSION=$(RELEASE_NUMBER) -D ManSourceDir=$(DOCFILE) -o podman-v$(RELEASE_NUMBER).msi contrib/msi/podman.wxs $(DOCFILE)/pages.wsx

podman-remote-%: .gopathok ## Build podman for a specific GOOS
	$(eval BINSFX := $(shell test "$*" != "windows" || echo ".exe"))
	CGO_ENABLED=0 GOOS=$* $(GO) build $(BUILDFLAGS) -gcflags '$(GCFLAGS)' -asmflags '$(ASMFLAGS)' -ldflags '$(LDFLAGS_PODMAN)' -tags "${REMOTETAGS}" -o bin/$@$(BINSFX) ./cmd/podman

local-cross: $(CROSS_BUILD_TARGETS) ## Cross local compilation

bin/podman.cross.%: .gopathok
	TARGET="$*"; \
	GOOS="$${TARGET%%.*}" \
	GOARCH="$${TARGET##*.}" \
	CGO_ENABLED=0 $(GO) build $(BUILDFLAGS) -gcflags '$(GCFLAGS)' -asmflags '$(ASMFLAGS)' -ldflags '$(LDFLAGS_PODMAN)' -tags '$(BUILDTAGS_CROSS)' -o "$@" ./cmd/podman

# Update nix/nixpkgs.json its latest stable commit
.PHONY: nixpkgs
nixpkgs:
	@nix run -f channel:nixos-20.09 nix-prefetch-git -c nix-prefetch-git \
		--no-deepClone https://github.com/nixos/nixpkgs > nix/nixpkgs.json

# Build statically linked binary
.PHONY: static
static:
	@nix build -f nix/
	mkdir -p ./bin
	cp -rfp ./result/bin/* ./bin/

.PHONY: run-docker-py-tests
run-docker-py-tests:
	$(eval testLogs=$(shell mktemp))
	./bin/podman run --rm --security-opt label=disable --privileged -v $(testLogs):/testLogs --net=host -e DOCKER_HOST=tcp://localhost:8080 $(DOCKERPY_IMAGE) sh -c "pytest $(DOCKERPY_TEST) "

pkg/api/swagger.yaml: .gopathok release.txt
	make -C pkg/api

.PHONY: swagger
swagger: pkg/api/swagger.yaml

.PHONY: clean
clean: ## Clean artifacts
	rm -rf \
		.gopathok \
		_output \
		release.txt \
		$(wildcard podman-remote*.zip) \
		$(wildcard podman*.tar.gz) \
		bin \
		build \
		test/checkseccomp/checkseccomp \
		test/goecho/goecho \
		test/testdata/redis-image \
		libpod/container_ffjson.go \
		libpod/pod_ffjson.go \
		libpod/container_easyjson.go \
		libpod/pod_easyjson.go \
		.install.goimports \
		docs/build
	make -C docs clean

.PHONY: localunit
localunit: test/goecho/goecho
	hack/check_root.sh make localunit
	rm -rf ${COVERAGE_PATH} && mkdir -p ${COVERAGE_PATH}
	$(GOBIN)/ginkgo \
		-r \
		$(TESTFLAGS) \
		--skipPackage test/e2e,pkg/apparmor,pkg/bindings,hack \
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

.PHONY: ginkgo
ginkgo:
	$(GOBIN)/ginkgo -v $(TESTFLAGS) -tags "$(BUILDTAGS)" $(GINKGOTIMEOUT) -cover -flakeAttempts 3 -progress -trace -noColor -nodes 3 -debug test/e2e/. hack/.

.PHONY: ginkgo-remote
ginkgo-remote:
	$(GOBIN)/ginkgo -v $(TESTFLAGS) -tags "$(REMOTETAGS)" $(GINKGOTIMEOUT) -cover -flakeAttempts 3 -progress -trace -noColor test/e2e/.

.PHONY: localintegration
localintegration: test-binaries ginkgo

.PHONY: remoteintegration
remoteintegration: test-binaries ginkgo-remote

.PHONY: localsystem
localsystem:
	# Wipe existing config, database, and cache: start with clean slate.
	$(RM) -rf ${HOME}/.local/share/containers ${HOME}/.config/containers
	if timeout -v 1 true; then PODMAN=$(shell pwd)/bin/podman bats test/system/; else echo "Skipping $@: 'timeout -v' unavailable'"; fi

.PHONY: remotesystem
remotesystem:
	# Wipe existing config, database, and cache: start with clean slate.
	$(RM) -rf ${HOME}/.local/share/containers ${HOME}/.config/containers
	# Start podman server using tmp socket; loop-wait for it;
	# test podman-remote; kill server, clean up tmp socket file.
	# podman server spews copious unhelpful output; ignore it.
	rc=0;\
	if timeout -v 1 true; then \
		SOCK_FILE=$(shell mktemp --dry-run --tmpdir podman.XXXXXX);\
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
		env PODMAN="$(shell pwd)/bin/podman-remote --url $$PODMAN_SOCKET" bats test/system/ ;\
		rc=$$?;\
		kill %1;\
		rm -f $$SOCK_FILE;\
	else \
		echo "Skipping $@: 'timeout -v' unavailable'";\
	fi;\
	exit $$rc

.PHONY: localapiv2
localapiv2:
	env PODMAN=./bin/podman ./test/apiv2/test-apiv2
	env PODMAN=./bin/podman ${PYTHON} -m unittest discover -v ./test/apiv2/rest_api/
	env PODMAN=./bin/podman ${PYTHON} -m unittest discover -v ./test/python/docker

.PHONY: remoteapiv2
remoteapiv2:
	true

.PHONY: system.test-binary
system.test-binary: .install.ginkgo
	$(GO) test -c ./test/system

.PHONY: binaries
binaries: podman podman-remote ## Build podman

.PHONY: install.catatonit
install.catatonit:
	./hack/install_catatonit.sh

.PHONY: test-binaries
test-binaries: test/checkseccomp/checkseccomp test/goecho/goecho install.catatonit

MANPAGES_MD ?= $(wildcard docs/source/markdown/*.md pkg/*/docs/*.md)
MANPAGES ?= $(MANPAGES_MD:%.md=%)
MANPAGES_DEST ?= $(subst markdown,man, $(subst source,build,$(MANPAGES)))

$(MANPAGES): %: %.md .install.md2man docdir
	@sed -e 's/\((podman.*\.md)\)//' -e 's/\[\(podman.*\)\]/\1/' $<  | $(GOMD2MAN) -in /dev/stdin -out $(subst source/markdown,build/man,$@)

.PHONY: docs
docdir:
	mkdir -p docs/build/man

.PHONY: docs
docs: $(MANPAGES) ## Generate documentation

install-podman-remote-%-docs: podman-remote docs $(MANPAGES)
	rm -rf docs/build/remote
	mkdir -p docs/build/remote
	ln -sf $(shell pwd)/docs/source/markdown/links docs/build/man/
	docs/remote-docs.sh $* docs/build/remote/$* $(if $(findstring windows,$*),docs/source/markdown,docs/build/man)

.PHONY: man-page-check
man-page-check:
	hack/man-page-checker
	hack/xref-helpmsgs-manpages

.PHONY: swagger-check
swagger-check:
	hack/swagger-check

.PHONY: codespell
codespell:
	codespell -S bin,vendor,.git,go.sum,changelog.txt,.cirrus.yml,"RELEASE_NOTES.md,*.xz,*.gz,*.tar,*.tgz,bin2img,*ico,*.png,*.1,*.5,copyimg,*.orig,apidoc.go" -L uint,iff,od,seeked,splitted,marge,ERRO,hist -w

# When publishing releases include critical build-time details
.PHONY: release.txt
release.txt:
	# X-RELEASE-INFO format depended upon by automated tooling
	echo -n "X-RELEASE-INFO:" > "$@"
	for field in "$(RELEASE_BASENAME)" "$(RELEASE_VERSION)" \
			 "$(RELEASE_DIST)" "$(RELEASE_DIST_VER)" "$(RELEASE_ARCH)"; do \
		echo -n " $$field"; done >> "$@"
	echo "" >> "$@"

podman-release.tar.gz: binaries docs release.txt
	$(eval TMPDIR := $(shell mktemp -d -p '' podman_XXXX))
	$(eval SUBDIR := podman-v$(RELEASE_NUMBER))
	mkdir -p "$(TMPDIR)/$(SUBDIR)"
	$(MAKE) install.bin install.man install.cni install.systemd "DESTDIR=$(TMPDIR)/$(SUBDIR)" "PREFIX=/usr"
	# release.txt location and content depended upon by automated tooling
	cp release.txt "$(TMPDIR)/"
	tar -czvf $@ --xattrs -C "$(TMPDIR)" "./release.txt" "./$(SUBDIR)"
	-rm -rf "$(TMPDIR)"

# Must call make in-line: Dependency-spec. w/ wild-card.
podman-remote-release-%.zip:
	$(MAKE) podman-remote-$* install-podman-remote-$*-docs release.txt \
		RELEASE_BASENAME=$(shell hack/get_release_info.sh REMOTENAME) \
		RELEASE_DIST=$* RELEASE_DIST_VER="-"
	$(eval TMPDIR := $(shell mktemp -d -p '' $podman_remote_XXXX))
	$(eval SUBDIR := podman-$(RELEASE_VERSION))
	$(eval BINSFX := $(shell test "$*" != "windows" || echo ".exe"))
	mkdir -p "$(TMPDIR)/$(SUBDIR)"
	# release.txt location and content depended upon by automated tooling
	cp release.txt "$(TMPDIR)/"
	cp ./bin/podman-remote-$*$(BINSFX) "$(TMPDIR)/$(SUBDIR)/podman$(BINSFX)"
	cp -r ./docs/build/remote/$* "$(TMPDIR)/$(SUBDIR)/docs/"
	cp ./contrib/remote/containers.conf "$(TMPDIR)/$(SUBDIR)/"
	cd "$(TMPDIR)/$(SUBDIR)" && \
		zip --recurse-paths "$(CURDIR)/$@" "./release.txt" "./"
	-rm -rf "$(TMPDIR)"

.PHONY: podman-release
podman-release:
	rm -f release.txt
	$(MAKE) podman-release.tar.gz

.PHONY: podman-remote-%-release
podman-remote-%-release:
	rm -f release.txt
	$(MAKE) podman-remote-release-$*.zip

.PHONY: generate-bindings
generate-bindings:
ifneq ($(shell uname -s), Darwin)
	GO111MODULE=off $(GO) generate ./pkg/bindings/... ;
endif

.PHONY: docker-docs
docker-docs: docs
	(cd docs; ./dckrman.sh ./build/man/*.1)

.PHONY: changelog
changelog: ## Generate changelog
	@echo "Creating changelog from $(CHANGELOG_BASE) to $(CHANGELOG_TARGET)"
	$(eval TMPFILE := $(shell mktemp))
	$(shell cat changelog.txt > $(TMPFILE))
	$(shell echo "- Changelog for $(CHANGELOG_TARGET) ($(ISODATE)):" > changelog.txt)
	$(shell git log --no-merges --format="  * %s" $(CHANGELOG_BASE)..$(CHANGELOG_TARGET) >> changelog.txt)
	$(shell echo "" >> changelog.txt)
	$(shell cat $(TMPFILE) >> changelog.txt)
	$(shell rm $(TMPFILE))

.PHONY: install
install: .gopathok install.bin install.remote install.man install.cni install.systemd  ## Install binaries to system locations

.PHONY: install.remote-nobuild
install.remote-nobuild:
	install ${SELINUXOPT} -d -m 755 $(DESTDIR)$(BINDIR)
	install ${SELINUXOPT} -m 755 bin/podman-remote $(DESTDIR)$(BINDIR)/podman-remote
	test -z "${SELINUXOPT}" || chcon --verbose --reference=$(DESTDIR)$(BINDIR)/podman-remote bin/podman-remote

.PHONY: install.remote
install.remote: podman-remote install.remote-nobuild

.PHONY: install.bin-nobuild
install.bin-nobuild:
	install ${SELINUXOPT} -d -m 755 $(DESTDIR)$(BINDIR)
	install ${SELINUXOPT} -m 755 bin/podman $(DESTDIR)$(BINDIR)/podman
	test -z "${SELINUXOPT}" || chcon --verbose --reference=$(DESTDIR)$(BINDIR)/podman bin/podman
	install ${SELINUXOPT} -m 755 -d ${DESTDIR}${TMPFILESDIR}
	install ${SELINUXOPT} -m 644 contrib/tmpfile/podman.conf ${DESTDIR}${TMPFILESDIR}/podman.conf

.PHONY: install.bin
install.bin: podman install.bin-nobuild

.PHONY: install.man-nobuild
install.man-nobuild:
	install ${SELINUXOPT} -d -m 755 $(DESTDIR)$(MANDIR)/man1
	install ${SELINUXOPT} -d -m 755 $(DESTDIR)$(MANDIR)/man5
	install ${SELINUXOPT} -m 644 $(filter %.1,$(MANPAGES_DEST)) -t $(DESTDIR)$(MANDIR)/man1
	install ${SELINUXOPT} -m 644 $(filter %.5,$(MANPAGES_DEST)) -t $(DESTDIR)$(MANDIR)/man5
	install ${SELINUXOPT} -m 644 docs/source/markdown/links/*1 -t $(DESTDIR)$(MANDIR)/man1

.PHONY: install.man
install.man: docs install.man-nobuild

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

.PHONY: install.cni
install.cni:
	install ${SELINUXOPT} -d -m 755 ${DESTDIR}${ETCDIR}/cni/net.d/
	install ${SELINUXOPT} -m 644 cni/87-podman-bridge.conflist ${DESTDIR}${ETCDIR}/cni/net.d/87-podman-bridge.conflist

.PHONY: install.docker
install.docker: docker-docs
	install ${SELINUXOPT} -d -m 755 $(DESTDIR)$(BINDIR) $(DESTDIR)$(MANDIR)/man1
	install ${SELINUXOPT} -m 755 docker $(DESTDIR)$(BINDIR)/docker
	install ${SELINUXOPT} -m 644 docs/build/man/docker*.1 -t $(DESTDIR)$(MANDIR)/man1
	install ${SELINUXOPT} -m 755 -d ${DESTDIR}${SYSTEMDDIR}  ${DESTDIR}${USERSYSTEMDDIR} ${DESTDIR}${TMPFILESDIR}
	install ${SELINUXOPT} -m 644 contrib/systemd/system/podman-docker.conf -t ${DESTDIR}${TMPFILESDIR}

.PHONY: install.systemd
install.systemd:
	install ${SELINUXOPT} -m 755 -d ${DESTDIR}${SYSTEMDDIR}  ${DESTDIR}${USERSYSTEMDDIR}
	# User services
	install ${SELINUXOPT} -m 644 contrib/systemd/auto-update/podman-auto-update.service ${DESTDIR}${USERSYSTEMDDIR}/podman-auto-update.service
	install ${SELINUXOPT} -m 644 contrib/systemd/auto-update/podman-auto-update.timer ${DESTDIR}${USERSYSTEMDDIR}/podman-auto-update.timer
	install ${SELINUXOPT} -m 644 contrib/systemd/user/podman.socket ${DESTDIR}${USERSYSTEMDDIR}/podman.socket
	install ${SELINUXOPT} -m 644 contrib/systemd/user/podman.service ${DESTDIR}${USERSYSTEMDDIR}/podman.service
	# System services
	install ${SELINUXOPT} -m 644 contrib/systemd/auto-update/podman-auto-update.service ${DESTDIR}${SYSTEMDDIR}/podman-auto-update.service
	install ${SELINUXOPT} -m 644 contrib/systemd/auto-update/podman-auto-update.timer ${DESTDIR}${SYSTEMDDIR}/podman-auto-update.timer
	install ${SELINUXOPT} -m 644 contrib/systemd/system/podman.socket ${DESTDIR}${SYSTEMDDIR}/podman.socket
	install ${SELINUXOPT} -m 644 contrib/systemd/system/podman.service ${DESTDIR}${SYSTEMDDIR}/podman.service

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

.PHONY: install.tools
install.tools: .install.goimports .install.md2man .install.ginkgo .install.golangci-lint .install.bats ## Install needed tools

define go-get
	env GO111MODULE=on \
		$(GO) get ${1}
endef

.install.goimports: .gopathok
	if [ ! -x "$(GOBIN)/goimports" ]; then \
		$(call go-get,golang.org/x/tools/cmd/goimports); \
	fi
	touch .install.goimports

.PHONY: .install.ginkgo
.install.ginkgo: .gopathok
	if [ ! -x "$(GOBIN)/ginkgo" ]; then \
		$(GO) install $(BUILDFLAGS) ./vendor/github.com/onsi/ginkgo/ginkgo ; \
	fi

.PHONY: .install.golangci-lint
.install.golangci-lint: .gopathok
	VERSION=1.36.0 GOBIN=$(GOBIN) sh ./hack/install_golangci.sh

.PHONY: .install.bats
.install.bats: .gopathok
	VERSION=v1.1.0 ./hack/install_bats.sh

.PHONY: .install.pre-commit
.install.pre-commit:
	if [ -z "$(PRE_COMMIT)" ]; then \
		python3 -m pip install --user pre-commit; \
	fi

.PHONY: .install.md2man
.install.md2man: .gopathok
	if [ ! -x "$(GOMD2MAN)" ]; then \
		$(call go-get,github.com/cpuguy83/go-md2man); \
	fi

# $BUILD_TAGS variable is used in hack/golangci-lint.sh
.PHONY: install.libseccomp.sudo
install.libseccomp.sudo:
	rm -rf ../../seccomp/libseccomp
	git clone https://github.com/seccomp/libseccomp ../../seccomp/libseccomp
	cd ../../seccomp/libseccomp && git checkout --detach $(LIBSECCOMP_COMMIT) && ./autogen.sh && ./configure --prefix=/usr && make all && make install


.PHONY: validate.completions
validate.completions: SHELL:=/usr/bin/env bash # Set shell to bash for this target
validate.completions:
	# Check if the files can be loaded by the shell
	. completions/bash/podman
	if [ -x /bin/zsh ]; then /bin/zsh completions/zsh/_podman; fi
	if [ -x /bin/fish ]; then /bin/fish completions/fish/podman.fish; fi

.PHONY: validate
validate: gofmt lint validate.completions man-page-check swagger-check

.PHONY: build-all-new-commits
build-all-new-commits:
	# Validate that all the commits build on top of $(GIT_BASE_BRANCH)
	git rebase $(GIT_BASE_BRANCH) -x make

.PHONY: build-no-cgo
build-no-cgo:
	env BUILDTAGS="containers_image_openpgp exclude_graphdriver_btrfs exclude_graphdriver_devicemapper exclude_disk_quota" CGO_ENABLED=0 $(MAKE)

.PHONY: vendor
vendor:
	GO111MODULE=on $(GO) mod tidy
	GO111MODULE=on $(GO) mod vendor
	GO111MODULE=on $(GO) mod verify

.PHONY: vendor-in-container
vendor-in-container:
	podman run --privileged --rm --env HOME=/root -v `pwd`:/src -w /src docker.io/library/golang:1.13 make vendor

.PHONY: package
package:  ## Build rpm packages
	## TODO(ssbarnea): make version number predictable, it should not change
	## on each execution, producing duplicates.
	rm -rf build/* *.src.rpm ~/rpmbuild/RPMS/*
	./contrib/build_rpm.sh

# Remember that rpms install exec to /usr/bin/podman while a `make install`
# installs them to /usr/local/bin/podman which is likely before. Always use
# a full path to test installed podman or you risk to call another executable.
.PHONY: package-install
package-install: package  ## Install rpm packages
	sudo ${PKG_MANAGER} -y install ${HOME}/rpmbuild/RPMS/*/*.rpm
	/usr/bin/podman version
	/usr/bin/podman info  # will catch a broken conmon
