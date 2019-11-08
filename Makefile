export GO111MODULE=off
export GOPROXY=https://proxy.golang.org

GO ?= go
DESTDIR ?=
EPOCH_TEST_COMMIT ?= ac73fd3fe5dcbf2647d589f9c9f37fe9531ed663
HEAD ?= HEAD
CHANGELOG_BASE ?= HEAD~
CHANGELOG_TARGET ?= HEAD
PROJECT := github.com/containers/libpod
GIT_BASE_BRANCH ?= origin/master
GIT_BRANCH ?= $(shell git rev-parse --abbrev-ref HEAD 2>/dev/null)
GIT_BRANCH_CLEAN ?= $(shell echo $(GIT_BRANCH) | sed -e "s/[^[:alnum:]]/-/g")
LIBPOD_IMAGE ?= libpod_dev$(if $(GIT_BRANCH_CLEAN),:$(GIT_BRANCH_CLEAN))
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
BUILDFLAGS ?=
BUILDTAGS ?= \
	$(shell hack/apparmor_tag.sh) \
	$(shell hack/btrfs_installed_tag.sh) \
	$(shell hack/btrfs_tag.sh) \
	$(shell hack/selinux_tag.sh) \
	$(shell hack/systemd_tag.sh) \
	exclude_graphdriver_devicemapper \
	seccomp \
	varlink

GO_BUILD=$(GO) build
# Go module support: set `-mod=vendor` to use the vendored sources
ifeq ($(shell go help mod >/dev/null 2>&1 && echo true), true)
	GO_BUILD=GO111MODULE=on $(GO) build -mod=vendor
endif

ifeq (,$(findstring systemd,$(BUILDTAGS)))
$(warning \
	Podman is being compiled without the systemd build tag.\
	Install libsystemd for journald support)
endif

BUILDTAGS_CROSS ?= containers_image_openpgp exclude_graphdriver_btrfs exclude_graphdriver_devicemapper exclude_graphdriver_overlay
ifneq (,$(findstring varlink,$(BUILDTAGS)))
	PODMAN_VARLINK_DEPENDENCIES = cmd/podman/varlink/iopodman.go
endif
CONTAINER_RUNTIME := $(shell command -v podman 2> /dev/null || echo docker)
OCI_RUNTIME ?= ""

BASHINSTALLDIR=${PREFIX}/share/bash-completion/completions
ZSHINSTALLDIR=${PREFIX}/share/zsh/site-functions

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
LIBPOD := ${PROJECT}/libpod
GCFLAGS ?= all=-trimpath=${PWD}
ASMFLAGS ?= all=-trimpath=${PWD}
LDFLAGS_PODMAN ?= $(LDFLAGS) \
	  -X $(LIBPOD)/define.gitCommit=$(GIT_COMMIT) \
	  -X $(LIBPOD)/define.buildInfo=$(BUILD_INFO) \
	  -X $(LIBPOD)/config._installPrefix=$(PREFIX) \
	  -X $(LIBPOD)/config._etcDir=$(ETCDIR)
#Update to LIBSECCOMP_COMMIT should reflect in Dockerfile too.
LIBSECCOMP_COMMIT := release-2.3
# Rarely if ever should integration tests take more than 50min,
# caller may override in special circumstances if needed.
GINKGOTIMEOUT ?= -timeout=90m

RELEASE_VERSION ?= $(shell hack/get_release_info.sh VERSION)
RELEASE_NUMBER ?= $(shell hack/get_release_info.sh NUMBER|sed -e 's/^v\(.*\)/\1/')
RELEASE_DIST ?= $(shell hack/get_release_info.sh DIST)
RELEASE_DIST_VER ?= $(shell hack/get_release_info.sh DIST_VER)
RELEASE_ARCH ?= $(shell hack/get_release_info.sh ARCH)
RELEASE_BASENAME := $(shell hack/get_release_info.sh BASENAME)

# If non-empty, logs all output from varlink during remote system testing
VARLINK_LOG ?=

# If GOPATH not specified, use one in the local directory
ifeq ($(GOPATH),)
export GOPATH := $(CURDIR)/_output
unexport GOBIN
endif
FIRST_GOPATH := $(firstword $(subst :, ,$(GOPATH)))
GOPKGDIR := $(FIRST_GOPATH)/src/$(PROJECT)
GOPKGBASEDIR ?= $(shell dirname "$(GOPKGDIR)")

GOBIN := $(shell $(GO) env GOBIN)
ifeq ($(GOBIN),)
GOBIN := $(FIRST_GOPATH)/bin
endif

GOMD2MAN ?= $(shell command -v go-md2man || echo '$(GOBIN)/go-md2man')

BOX="fedora_atomic"

CROSS_BUILD_TARGETS := \
	bin/podman.cross.darwin.amd64 \
	bin/podman.cross.linux.amd64

all: binaries docs

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

help:
	@python -c "$$PRINT_HELP_PYSCRIPT" < $(MAKEFILE_LIST)

.gopathok:
ifeq ("$(wildcard $(GOPKGDIR))","")
	mkdir -p "$(GOPKGBASEDIR)"
	ln -sfn "$(CURDIR)" "$(GOPKGDIR)"
	ln -sfn "$(CURDIR)/vendor/github.com/varlink" "$(FIRST_GOPATH)/src/github.com/varlink"
endif
	touch $@

lint: .gopathok varlink_generate ## Execute the source code linter
	@echo "checking lint"
	@./.tool/lint

golangci-lint: .gopathok varlink_generate .install.golangci-lint
	$(GOBIN)/golangci-lint run --tests=false

gofmt: ## Verify the source code gofmt
	find . -name '*.go' ! -path './vendor/*' -exec gofmt -s -w {} \+
	git diff --exit-code

test/checkseccomp/checkseccomp: .gopathok $(wildcard test/checkseccomp/*.go)
	$(GO_BUILD) -ldflags '$(LDFLAGS)' -tags "$(BUILDTAGS)" -o $@ $(PROJECT)/test/checkseccomp

test/goecho/goecho: .gopathok $(wildcard test/goecho/*.go)
	$(GO_BUILD) -ldflags '$(LDFLAGS)' -o $@ $(PROJECT)/test/goecho

podman: .gopathok $(PODMAN_VARLINK_DEPENDENCIES) ## Build with podman
	$(GO_BUILD) $(BUILDFLAGS) -gcflags '$(GCFLAGS)' -asmflags '$(ASMFLAGS)' -ldflags '$(LDFLAGS_PODMAN)' -tags "$(BUILDTAGS)" -o bin/$@ $(PROJECT)/cmd/podman

podman-remote: .gopathok $(PODMAN_VARLINK_DEPENDENCIES) ## Build with podman on remote environment
	$(GO_BUILD) $(BUILDFLAGS) -gcflags '$(GCFLAGS)' -asmflags '$(ASMFLAGS)' -ldflags '$(LDFLAGS_PODMAN)' -tags "$(BUILDTAGS) remoteclient" -o bin/$@ $(PROJECT)/cmd/podman

.PHONY: podman.msi
podman.msi: podman-remote podman-remote-windows install-podman-remote-windows-docs ## Will always rebuild exe as there is no podman-remote-windows.exe target to verify timestamp
	$(eval DOCFILE := docs/build/remote/windows)
	find $(DOCFILE) -print \
	|wixl-heat --var var.ManSourceDir --component-group ManFiles --directory-ref INSTALLDIR --prefix $(DOCFILE)/ >$(DOCFILE)/pages.wsx
	wixl -D VERSION=$(RELEASE_NUMBER) -D ManSourceDir=$(DOCFILE) -o podman-v$(RELEASE_NUMBER).msi contrib/msi/podman.wxs $(DOCFILE)/pages.wsx

podman-remote-%: .gopathok $(PODMAN_VARLINK_DEPENDENCIES) ## Build podman for a specific GOOS
	$(eval BINSFX := $(shell test "$*" != "windows" || echo ".exe"))
	CGO_ENABLED=0 GOOS=$* $(GO_BUILD) -gcflags '$(GCFLAGS)' -asmflags '$(ASMFLAGS)' -ldflags '$(LDFLAGS_PODMAN)' -tags "remoteclient containers_image_openpgp exclude_graphdriver_devicemapper" -o bin/$@$(BINSFX) $(PROJECT)/cmd/podman

local-cross: $(CROSS_BUILD_TARGETS) ## Cross local compilation

bin/podman.cross.%: .gopathok
	TARGET="$*"; \
	GOOS="$${TARGET%%.*}" \
	GOARCH="$${TARGET##*.}" \
	$(GO_BUILD) -gcflags '$(GCFLAGS)' -asmflags '$(ASMFLAGS)' -ldflags '$(LDFLAGS_PODMAN)' -tags '$(BUILDTAGS_CROSS)' -o "$@" $(PROJECT)/cmd/podman

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
		cmd/podman/varlink/iopodman.go \
		libpod/container_ffjson.go \
		libpod/pod_ffjson.go \
		libpod/container_easyjson.go \
		libpod/pod_easyjson.go \
		docs/build

libpodimage: ## Build the libpod image
	${CONTAINER_RUNTIME} build -t ${LIBPOD_IMAGE} .

dbuild: libpodimage
	${CONTAINER_RUNTIME} run --name=${LIBPOD_INSTANCE} --privileged -v ${PWD}:/go/src/${PROJECT} --rm ${LIBPOD_IMAGE} make all

dbuild-podman-remote: libpodimage
	${CONTAINER_RUNTIME} run --name=${LIBPOD_INSTANCE} --privileged -v ${PWD}:/go/src/${PROJECT} --rm ${LIBPOD_IMAGE} go build -ldflags '$(LDFLAGS_PODMAN)' -tags "$(BUILDTAGS) remoteclient" -o bin/podman-remote $(PROJECT)/cmd/podman

dbuild-podman-remote-darwin: libpodimage
	${CONTAINER_RUNTIME} run --name=${LIBPOD_INSTANCE} --privileged -v ${PWD}:/go/src/${PROJECT} --rm ${LIBPOD_IMAGE} env GOOS=darwin go build -ldflags '$(LDFLAGS_PODMAN)' -tags "remoteclient containers_image_openpgp exclude_graphdriver_devicemapper" -o bin/podman-remote-darwin $(PROJECT)/cmd/podman

test: libpodimage ## Run tests on built image
	${CONTAINER_RUNTIME} run -e STORAGE_OPTIONS="--storage-driver=vfs" -e TESTFLAGS -e OCI_RUNTIME -e CGROUP_MANAGER=cgroupfs -e TRAVIS -t --privileged --rm -v ${CURDIR}:/go/src/${PROJECT} ${LIBPOD_IMAGE} make clean all localunit install.catatonit localintegration

integration: libpodimage ## Execute integration tests
	${CONTAINER_RUNTIME} run -e STORAGE_OPTIONS="--storage-driver=vfs" -e TESTFLAGS -e OCI_RUNTIME -e CGROUP_MANAGER=cgroupfs -e TRAVIS -t --privileged --rm -v ${CURDIR}:/go/src/${PROJECT} ${LIBPOD_IMAGE} make clean all install.catatonit localintegration

integration.fedora:
	DIST=Fedora sh .papr_prepare.sh

integration.centos:
	DIST=CentOS sh .papr_prepare.sh

shell: libpodimage ## Run the built image and attach a shell
	${CONTAINER_RUNTIME} run -e STORAGE_OPTIONS="--storage-driver=vfs" -e CGROUP_MANAGER=cgroupfs -e TESTFLAGS -e OCI_RUNTIME -e TRAVIS -it --privileged --rm -v ${CURDIR}:/go/src/${PROJECT} ${LIBPOD_IMAGE} sh

testunit: libpodimage ## Run unittest on the built image
	${CONTAINER_RUNTIME} run -e STORAGE_OPTIONS="--storage-driver=vfs" -e TESTFLAGS -e CGROUP_MANAGER=cgroupfs -e OCI_RUNTIME -e TRAVIS -t --privileged --rm -v ${CURDIR}:/go/src/${PROJECT} ${LIBPOD_IMAGE} make localunit

localunit: test/goecho/goecho varlink_generate
	ginkgo \
		-r \
		--skipPackage test/e2e,pkg/apparmor,test/endpoint \
		--cover \
		--covermode atomic \
		--tags "$(BUILDTAGS)" \
		--succinct

ginkgo:
	ginkgo -v -tags "$(BUILDTAGS)" $(GINKGOTIMEOUT) -cover -flakeAttempts 3 -progress -trace -noColor -nodes 3 -debug test/e2e/.

ginkgo-remote:
	ginkgo -v -tags "$(BUILDTAGS) remoteclient" $(GINKGOTIMEOUT) -cover -flakeAttempts 3 -progress -trace -noColor test/e2e/.

endpoint:
	ginkgo -v -tags "$(BUILDTAGS)" $(GINKGOTIMEOUT) -cover -flakeAttempts 3 -progress -trace -noColor -debug test/endpoint/.

localintegration: varlink_generate test-binaries ginkgo

remoteintegration: varlink_generate test-binaries ginkgo-remote

localsystem:
	# Wipe existing config, database, and cache: start with clean slate.
	$(RM) -rf ${HOME}/.local/share/containers ${HOME}/.config/containers
	if timeout -v 1 true; then PODMAN=./bin/podman bats test/system/; else echo "Skipping $@: 'timeout -v' unavailable'"; fi

remotesystem:
	# Wipe existing config, database, and cache: start with clean slate.
	$(RM) -rf ${HOME}/.local/share/containers ${HOME}/.config/containers
	# Start varlink server using tmp socket; loop-wait for it;
	# test podman-remote; kill server, clean up tmp socket file.
	# varlink server spews copious unhelpful output; ignore it.
	rc=0;\
	if timeout -v 1 true; then \
		SOCK_FILE=$(shell mktemp --dry-run --tmpdir io.podman.XXXXXX);\
		export PODMAN_VARLINK_ADDRESS=unix:$$SOCK_FILE; \
		./bin/podman varlink --timeout=0 $$PODMAN_VARLINK_ADDRESS &> $(if $(VARLINK_LOG),$(VARLINK_LOG),/dev/null) & \
		retry=5;\
		while [[ $$retry -ge 0 ]]; do\
			echo Waiting for varlink server...;\
			sleep 1;\
			./bin/podman-remote info &>/dev/null && break;\
			retry=$$(expr $$retry - 1);\
		done;\
		env PODMAN=./bin/podman-remote bats test/system/ ;\
		rc=$$?;\
		kill %1;\
		rm -f $$SOCK_FILE;\
	else \
		echo "Skipping $@: 'timeout -v' unavailable'";\
	fi;\
	exit $$rc

system.test-binary: .install.ginkgo
	$(GO) test -c ./test/system

perftest:  ## Build perf tests
	$ cd contrib/perftest;go build

run-perftest: perftest ## Build and run perf tests
	$ contrib/perftest/perftest

vagrant-check:
	BOX=$(BOX) sh ./vagrant.sh

binaries: varlink_generate podman podman-remote  ## Build podman

install.catatonit:
	./hack/install_catatonit.sh

test-binaries: test/checkseccomp/checkseccomp test/goecho/goecho install.catatonit

MANPAGES_MD ?= $(wildcard docs/source/markdown/*.md pkg/*/docs/*.md)
MANPAGES ?= $(MANPAGES_MD:%.md=%)
MANPAGES_DEST ?= $(subst markdown,man, $(subst source,build,$(MANPAGES)))

$(MANPAGES): %: %.md .gopathok
	@sed -e 's/\((podman.*\.md)\)//' -e 's/\[\(podman.*\)\]/\1/' $<  | $(GOMD2MAN) -in /dev/stdin -out $(subst source/markdown,build/man,$@)

docdir:
	mkdir -p docs/build/man

docs: docdir $(MANPAGES) ## Generate documentation

install-podman-remote-%-docs: podman-remote docs $(MANPAGES)
	rm -rf docs/build/remote
	mkdir -p docs/build/remote
	ln -sf $(shell pwd)/docs/source/markdown/links docs/build/man/
	docs/remote-docs.sh $* docs/build/remote/$* $(if $(findstring windows,$*),docs/source/markdown,docs/build/man)

man-page-check:
	hack/man-page-checker

# When publishing releases include critical build-time details
.PHONY: release.txt
release.txt:
	# X-RELEASE-INFO format depended upon by automated tooling
	echo -n "X-RELEASE-INFO:" > "$@"
	for field in "$(RELEASE_BASENAME)" "$(RELEASE_VERSION)" \
		         "$(RELEASE_DIST)" "$(RELEASE_DIST_VER)" "$(RELEASE_ARCH)"; do \
		echo -n " $$field"; done >> "$@"
	echo "" >> "$@"

podman-v$(RELEASE_NUMBER).tar.gz: binaries docs release.txt
	$(eval TMPDIR := $(shell mktemp -d -p '' podman_XXXX))
	$(eval SUBDIR := podman-v$(RELEASE_NUMBER))
	mkdir -p "$(TMPDIR)/$(SUBDIR)"
	$(MAKE) install.bin install.man install.cni install.systemd "DESTDIR=$(TMPDIR)/$(SUBDIR)" "PREFIX=/usr"
	# release.txt location and content depended upon by automated tooling
	cp release.txt "$(TMPDIR)/"
	tar -czvf $@ --xattrs -C "$(TMPDIR)" "./release.txt" "./$(SUBDIR)"
	-rm -rf "$(TMPDIR)"

# Must call make in-line: Dependency-spec. w/ wild-card also consumes variable value.
podman-remote-v$(RELEASE_NUMBER)-%.zip:
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
	cd "$(TMPDIR)" && \
		zip --recurse-paths "$(CURDIR)/$@" "./release.txt" "./"
	-rm -rf "$(TMPDIR)"

.PHONY: podman-release
podman-release:
	rm -f release.txt
	$(MAKE) podman-v$(RELEASE_NUMBER).tar.gz

.PHONY: podman-remote-%-release
podman-remote-%-release:
	rm -f release.txt
	$(MAKE) podman-remote-v$(RELEASE_NUMBER)-$*.zip

docker-docs: docs
	(cd docs; ./dckrman.sh ./build/man/*.1)

changelog: ## Generate changelog
	@echo "Creating changelog from $(CHANGELOG_BASE) to $(CHANGELOG_TARGET)"
	$(eval TMPFILE := $(shell mktemp))
	$(shell cat changelog.txt > $(TMPFILE))
	$(shell echo "- Changelog for $(CHANGELOG_TARGET) ($(ISODATE)):" > changelog.txt)
	$(shell git log --no-merges --format="  * %s" $(CHANGELOG_BASE)..$(CHANGELOG_TARGET) >> changelog.txt)
	$(shell echo "" >> changelog.txt)
	$(shell cat $(TMPFILE) >> changelog.txt)
	$(shell rm $(TMPFILE))

install: .gopathok install.bin install.remote install.man install.cni install.systemd  ## Install binaries to system locations

install.remote: podman-remote
	install ${SELINUXOPT} -d -m 755 $(DESTDIR)$(BINDIR)
	install ${SELINUXOPT} -m 755 bin/podman-remote $(DESTDIR)$(BINDIR)/podman-remote
	test -z "${SELINUXOPT}" || chcon --verbose --reference=$(DESTDIR)$(BINDIR)/podman bin/podman-remote

install.bin: podman
	install ${SELINUXOPT} -d -m 755 $(DESTDIR)$(BINDIR)
	install ${SELINUXOPT} -m 755 bin/podman $(DESTDIR)$(BINDIR)/podman
	test -z "${SELINUXOPT}" || chcon --verbose --reference=$(DESTDIR)$(BINDIR)/podman bin/podman

install.man: docs
	install ${SELINUXOPT} -d -m 755 $(DESTDIR)$(MANDIR)/man1
	install ${SELINUXOPT} -d -m 755 $(DESTDIR)$(MANDIR)/man5
	install ${SELINUXOPT} -m 644 $(filter %.1,$(MANPAGES_DEST)) -t $(DESTDIR)$(MANDIR)/man1
	install ${SELINUXOPT} -m 644 $(filter %.5,$(MANPAGES_DEST)) -t $(DESTDIR)$(MANDIR)/man5
	install ${SELINUXOPT} -m 644 docs/source/markdown/links/*1 -t $(DESTDIR)$(MANDIR)/man1

install.config:
	install ${SELINUXOPT} -d -m 755 $(DESTDIR)$(SHAREDIR_CONTAINERS)
	install ${SELINUXOPT} -m 644 libpod.conf $(DESTDIR)$(SHAREDIR_CONTAINERS)/libpod.conf
	install ${SELINUXOPT} -m 644 seccomp.json $(DESTDIR)$(SHAREDIR_CONTAINERS)/seccomp.json

install.completions:
	install ${SELINUXOPT} -d -m 755 ${DESTDIR}${BASHINSTALLDIR}
	install ${SELINUXOPT} -m 644 completions/bash/podman ${DESTDIR}${BASHINSTALLDIR}
	install ${SELINUXOPT} -d -m 755 ${DESTDIR}${ZSHINSTALLDIR}
	install ${SELINUXOPT} -m 644 completions/zsh/_podman ${DESTDIR}${ZSHINSTALLDIR}

install.cni:
	install ${SELINUXOPT} -d -m 755 ${DESTDIR}${ETCDIR}/cni/net.d/
	install ${SELINUXOPT} -m 644 cni/87-podman-bridge.conflist ${DESTDIR}${ETCDIR}/cni/net.d/87-podman-bridge.conflist

install.docker: docker-docs
	install ${SELINUXOPT} -d -m 755 $(DESTDIR)$(BINDIR) $(DESTDIR)$(MANDIR)/man1
	install ${SELINUXOPT} -m 755 docker $(DESTDIR)$(BINDIR)/docker
	install ${SELINUXOPT} -m 644 docs/build/man/docker*.1 -t $(DESTDIR)$(MANDIR)/man1

install.systemd:
	install ${SELINUXOPT} -m 755 -d ${DESTDIR}${SYSTEMDDIR}  ${DESTDIR}${USERSYSTEMDDIR} ${DESTDIR}${TMPFILESDIR}
	install ${SELINUXOPT} -m 644 contrib/varlink/io.podman.socket ${DESTDIR}${SYSTEMDDIR}/io.podman.socket
	install ${SELINUXOPT} -m 644 contrib/varlink/io.podman.socket ${DESTDIR}${USERSYSTEMDDIR}/io.podman.socket
	install ${SELINUXOPT} -m 644 contrib/varlink/io.podman.service ${DESTDIR}${SYSTEMDDIR}/io.podman.service
	install ${SELINUXOPT} -d ${DESTDIR}${USERSYSTEMDDIR}
	# User units are ordered differently, we can't make the *system* multi-user.target depend on a user unit.
	# For user units the default.target that's the default is fine.
	sed -e 's,^WantedBy=.*,WantedBy=default.target,' < contrib/varlink/io.podman.service > ${DESTDIR}${USERSYSTEMDDIR}/io.podman.service
	install ${SELINUXOPT} -m 644 contrib/varlink/podman.conf ${DESTDIR}${TMPFILESDIR}/podman.conf

uninstall:
	for i in $(filter %.1,$(MANPAGES_DEST)); do \
		rm -f $(DESTDIR)$(MANDIR)/man1/$$(basename $${i}); \
	done; \
	for i in $(filter %.5,$(MANPAGES_DEST)); do \
		rm -f $(DESTDIR)$(MANDIR)/man5/$$(basename $${i}); \
	done

.PHONY: .gitvalidation
.gitvalidation: .gopathok
	GIT_CHECK_EXCLUDE="./vendor:docs/make.bat" $(GOBIN)/git-validation -v -run DCO,short-subject,dangling-whitespace -range $(EPOCH_TEST_COMMIT)..$(HEAD)

.PHONY: install.tools
install.tools: .install.gitvalidation .install.gometalinter .install.md2man .install.ginkgo .install.golangci-lint ## Install needed tools

define go-get
	env GO111MODULE=off \
		$(GO) get -u ${1}
endef

.install.ginkgo: .gopathok
	if [ ! -x "$(GOBIN)/ginkgo" ]; then \
		$(GO_BUILD) -o ${GOPATH}/bin/ginkgo ./vendor/github.com/onsi/ginkgo/ginkgo ; \
	fi

.install.gitvalidation: .gopathok
	if [ ! -x "$(GOBIN)/git-validation" ]; then \
		$(call go-get,github.com/vbatts/git-validation); \
	fi

.install.gometalinter: .gopathok
	if [ ! -x "$(GOBIN)/gometalinter" ]; then \
		$(call go-get,github.com/alecthomas/gometalinter); \
		cd $(FIRST_GOPATH)/src/github.com/alecthomas/gometalinter; \
		git checkout e8d801238da6f0dfd14078d68f9b53fa50a7eeb5; \
		$(GO) install github.com/alecthomas/gometalinter; \
		$(GOBIN)/gometalinter --install; \
	fi

.install.golangci-lint: .gopathok
	if [ ! -x "$(GOBIN)/golangci-lint" ]; then \
		curl -sfL https://install.goreleaser.com/github.com/golangci/golangci-lint.sh | sh -s -- -b $(GOBIN)/ v1.17.1; \
	fi

.install.md2man: .gopathok
	if [ ! -x "$(GOBIN)/go-md2man" ]; then \
		   $(call go-get,github.com/cpuguy83/go-md2man); \
	fi

varlink_generate: .gopathok cmd/podman/varlink/iopodman.go ## Generate varlink
varlink_api_generate: .gopathok API.md

.PHONY: install.libseccomp.sudo
install.libseccomp.sudo:
	rm -rf ../../seccomp/libseccomp
	git clone https://github.com/seccomp/libseccomp ../../seccomp/libseccomp
	cd ../../seccomp/libseccomp && git checkout $(LIBSECCOMP_COMMIT) && ./autogen.sh && ./configure --prefix=/usr && make all && make install


cmd/podman/varlink/iopodman.go: cmd/podman/varlink/io.podman.varlink
	GO111MODULE=off $(GO) generate ./cmd/podman/varlink/...

API.md: cmd/podman/varlink/io.podman.varlink
	$(GO) generate ./docs/...

validate.completions: completions/bash/podman
	. completions/bash/podman
	if [ -x /bin/zsh ]; then /bin/zsh completions/zsh/_podman; fi

validate: gofmt .gitvalidation validate.completions golangci-lint man-page-check

build-all-new-commits:
	# Validate that all the commits build on top of $(GIT_BASE_BRANCH)
	git rebase $(GIT_BASE_BRANCH) -x make

build-no-cgo:
	env BUILDTAGS="containers_image_openpgp exclude_graphdriver_btrfs exclude_graphdriver_devicemapper exclude_disk_quota" CGO_ENABLED=0 $(MAKE)

vendor:
	export GO111MODULE=on \
		$(GO) mod tidy && \
		$(GO) mod vendor && \
		$(GO) mod verify

vendor-in-container:
	podman run --privileged --rm --env HOME=/root -v `pwd`:/src -w /src docker.io/library/golang:1.13 make vendor

.PHONY: \
	.gopathok \
	binaries \
	clean \
	validate.completions \
	default \
	docs \
	gofmt \
	help \
	install \
	golangci-lint \
	lint \
	pause \
	uninstall \
	shell \
	changelog \
	validate \
	install.libseccomp.sudo \
	vendor
