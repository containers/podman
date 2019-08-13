export GO111MODULE=off

GO ?= go
DESTDIR ?=
EPOCH_TEST_COMMIT ?= bb80586e275fe0d3f47700ec54c9718a28b1e59c
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
BUILDFLAGS ?=
BUILDTAGS ?= \
	$(shell hack/apparmor_tag.sh) \
	$(shell hack/btrfs_installed_tag.sh) \
	$(shell hack/btrfs_tag.sh) \
	$(shell hack/ostree_tag.sh) \
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

BUILDTAGS_CROSS ?= containers_image_openpgp containers_image_ostree_stub exclude_graphdriver_btrfs exclude_graphdriver_devicemapper exclude_graphdriver_overlay
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
	  -X $(LIBPOD).gitCommit=$(GIT_COMMIT) \
	  -X $(LIBPOD).buildInfo=$(BUILD_INFO) \
	  -X $(LIBPOD).installPrefix=$(PREFIX) \
	  -X $(LIBPOD).etcDir=$(ETCDIR)
#Update to LIBSECCOMP_COMMIT should reflect in Dockerfile too.
LIBSECCOMP_COMMIT := release-2.3
# Rarely if ever should integration tests take more than 50min,
# caller may override in special circumstances if needed.
GINKGOTIMEOUT ?= -timeout=90m

RELEASE_VERSION ?= $(shell git fetch --tags && git describe HEAD 2> /dev/null)
RELEASE_NUMBER ?= $(shell echo $(RELEASE_VERSION) | sed 's/-.*//')
RELEASE_DIST ?= $(shell ( source /etc/os-release; echo $$ID ))
RELEASE_DIST_VER ?= $(shell ( source /etc/os-release; echo $$VERSION_ID | cut -d '.' -f 1))
RELEASE_ARCH ?= $(shell go env GOARCH 2> /dev/null)
RELEASE_BASENAME := $(shell basename $(PROJECT))


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
	ln -sfnT "$(CURDIR)" "$(GOPKGDIR)"
	ln -sfnT "$(CURDIR)/vendor/github.com/varlink" "$(FIRST_GOPATH)/src/github.com/varlink"
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
	$(GO_BUILD) -ldflags '$(LDFLAGS)' -tags "$(BUILDTAGS) containers_image_ostree_stub" -o $@ $(PROJECT)/test/checkseccomp

test/goecho/goecho: .gopathok $(wildcard test/goecho/*.go)
	$(GO_BUILD) -ldflags '$(LDFLAGS)' -o $@ $(PROJECT)/test/goecho

podman: .gopathok $(PODMAN_VARLINK_DEPENDENCIES) ## Build with podman
	$(GO_BUILD) $(BUILDFLAGS) -gcflags '$(GCFLAGS)' -asmflags '$(ASMFLAGS)' -ldflags '$(LDFLAGS_PODMAN)' -tags "$(BUILDTAGS)" -o bin/$@ $(PROJECT)/cmd/podman

podman-remote: .gopathok $(PODMAN_VARLINK_DEPENDENCIES) ## Build with podman on remote environment
	$(GO_BUILD) $(BUILDFLAGS) -gcflags '$(GCFLAGS)' -asmflags '$(ASMFLAGS)' -ldflags '$(LDFLAGS_PODMAN)' -tags "$(BUILDTAGS) remoteclient" -o bin/$@ $(PROJECT)/cmd/podman

podman-remote-darwin: .gopathok $(PODMAN_VARLINK_DEPENDENCIES) ## Build with podman on remote OSX environment
	CGO_ENABLED=0 GOOS=darwin $(GO_BUILD) -gcflags '$(GCFLAGS)' -asmflags '$(ASMFLAGS)' -ldflags '$(LDFLAGS_PODMAN)' -tags "remoteclient containers_image_openpgp exclude_graphdriver_devicemapper" -o bin/$@ $(PROJECT)/cmd/podman

podman-remote-windows: .gopathok $(PODMAN_VARLINK_DEPENDENCIES) ## Build with podman for a remote windows environment
	CGO_ENABLED=0 GOOS=windows $(GO_BUILD) -gcflags '$(GCFLAGS)' -asmflags '$(ASMFLAGS)' -ldflags '$(LDFLAGS_PODMAN)' -tags "remoteclient containers_image_openpgp exclude_graphdriver_devicemapper" -o bin/$@.exe $(PROJECT)/cmd/podman

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
		podman*.zip \
		podman*.tar.gz \
		bin \
		build \
		docs/remote \
		test/checkseccomp/checkseccomp \
		test/goecho/goecho \
		test/testdata/redis-image \
		cmd/podman/varlink/iopodman.go \
		libpod/container_ffjson.go \
		libpod/pod_ffjson.go \
		libpod/container_easyjson.go \
		libpod/pod_easyjson.go \
		$(MANPAGES) ||:
	find . -name \*~ -delete
	find . -name \#\* -delete

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
		--skipPackage test/e2e,pkg/apparmor \
		--cover \
		--covermode atomic \
		--tags "$(BUILDTAGS)" \
		--succinct
	$(MAKE) -C contrib/cirrus/packer test
	./contrib/cirrus/lib.sh.t
	./contrib/cirrus/cirrus_yaml_test.py

ginkgo:
	ginkgo -v -tags "$(BUILDTAGS)" $(GINKGOTIMEOUT) -cover -flakeAttempts 3 -progress -trace -noColor -nodes 3 -debug test/e2e/.

ginkgo-remote:
	ginkgo -v -tags "$(BUILDTAGS) remoteclient" $(GINKGOTIMEOUT) -cover -flakeAttempts 3 -progress -trace -noColor test/e2e/.

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
		./bin/podman varlink --timeout=0 $$PODMAN_VARLINK_ADDRESS &>/dev/null & \
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

# Zip archives are supported on all platforms + allows embedding metadata
podman.zip: binaries docs
	$(eval TMPDIR := $(shell mktemp -d -p '' $@_XXXX))
	test -n "$(TMPDIR)"
	$(MAKE) install "DESTDIR=$(TMPDIR)" "PREFIX=$(TMPDIR)/usr"
	# Encoded RELEASE_INFO format depended upon by CI tooling
	# X-RELEASE-INFO format depended upon by CI tooling
	cd "$(TMPDIR)" && echo \
		"X-RELEASE-INFO: $(RELEASE_BASENAME) $(RELEASE_VERSION) $(RELEASE_DIST) $(RELEASE_DIST_VER) $(RELEASE_ARCH)" | \
		zip --recurse-paths --archive-comment "$(CURDIR)/$@" "./"
	-rm -rf "$(TMPDIR)"

podman-remote-%.zip: podman-remote-%
	# Don't label darwin/windows cros-compiles with local distribution & version
	echo "X-RELEASE-INFO: podman-remote $(RELEASE_VERSION) $* cc $(RELEASE_ARCH)" | \
        zip --archive-comment "$(CURDIR)/$@" ./bin/$<*

install.catatonit:
	./hack/install_catatonit.sh

test-binaries: test/checkseccomp/checkseccomp test/goecho/goecho install.catatonit

MANPAGES_MD ?= $(wildcard docs/*.md pkg/*/docs/*.md)
MANPAGES ?= $(MANPAGES_MD:%.md=%)

$(MANPAGES): %: %.md .gopathok
	@sed -e 's/\((podman.*\.md)\)//' -e 's/\[\(podman.*\)\]/\1/' $<  | $(GOMD2MAN) -in /dev/stdin -out $@

docs: $(MANPAGES) ## Generate documentation

install-podman-remote-docs: docs
	@(cd docs; ./podman-remote.sh ./remote)


brew-pkg: install-podman-remote-docs podman-remote-darwin
	@mkdir -p ./brew
	@cp ./bin/podman-remote-darwin ./brew/podman
	@cp -r ./docs/remote ./brew/docs/
	@cp docs/podman-remote.1 ./brew/docs/podman.1
	@cp docs/podman-remote.conf.5 ./brew/docs/podman-remote.conf.5
	@sed -i 's/podman\\*-remote/podman/g' ./brew/docs/podman.1
	@sed -i 's/Podman\\*-remote/Podman\ for\ Mac/g' ./brew/docs/podman.1
	@sed -i 's/podman\.conf/podman\-remote\.conf/g' ./brew/docs/podman.1
	@sed -i 's/A\ remote\ CLI\ for\ Podman\:\ //g' ./brew/docs/podman.1
	tar -czvf podman-${RELEASE_NUMBER}.tar.gz ./brew
	@rm -rf ./brew

docker-docs: docs
	(cd docs; ./dckrman.sh *.1)

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
	install ${SELINUXOPT} -m 644 $(filter %.1,$(MANPAGES)) -t $(DESTDIR)$(MANDIR)/man1
	install ${SELINUXOPT} -m 644 $(filter %.5,$(MANPAGES)) -t $(DESTDIR)$(MANDIR)/man5
	install ${SELINUXOPT} -m 644 docs/links/*1 -t $(DESTDIR)$(MANDIR)/man1

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
	install ${SELINUXOPT} -m 644 docs/docker*.1 -t $(DESTDIR)$(MANDIR)/man1

install.systemd:
	install ${SELINUXOPT} -m 755 -d ${DESTDIR}${SYSTEMDDIR} ${DESTDIR}${TMPFILESDIR}
	install ${SELINUXOPT} -m 644 contrib/varlink/io.podman.socket ${DESTDIR}${SYSTEMDDIR}/io.podman.socket
	install ${SELINUXOPT} -m 644 contrib/varlink/io.podman.service ${DESTDIR}${SYSTEMDDIR}/io.podman.service
	install ${SELINUXOPT} -m 644 contrib/varlink/podman.conf ${DESTDIR}${TMPFILESDIR}/podman.conf

uninstall:
	for i in $(filter %.1,$(MANPAGES)); do \
		rm -f $(DESTDIR)$(MANDIR)/man1/$$(basename $${i}); \
	done; \
	for i in $(filter %.5,$(MANPAGES)); do \
		rm -f $(DESTDIR)$(MANDIR)/man5/$$(basename $${i}); \
	done

.PHONY: .gitvalidation
.gitvalidation: .gopathok
	GIT_CHECK_EXCLUDE="./vendor" $(GOBIN)/git-validation -v -run DCO,short-subject,dangling-whitespace -range $(EPOCH_TEST_COMMIT)..$(HEAD)

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

.install.ostree: .gopathok
	if ! pkg-config ostree-1 2> /dev/null ; then \
		git clone https://github.com/ostreedev/ostree $(FIRST_GOPATH)/src/github.com/ostreedev/ostree ; \
		cd $(FIRST_GOPATH)src/github.com/ostreedev/ostree ; \
		./autogen.sh --prefix=/usr/local; \
		make all install; \
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

validate: gofmt .gitvalidation validate.completions

build-all-new-commits:
	# Validate that all the commits build on top of $(GIT_BASE_BRANCH)
	git rebase $(GIT_BASE_BRANCH) -x make

build-no-cgo:
	env BUILDTAGS="containers_image_openpgp containers_image_ostree_stub exclude_graphdriver_btrfs exclude_graphdriver_devicemapper exclude_disk_quota" CGO_ENABLED=0 $(MAKE)

vendor:
	export GO111MODULE=on \
		$(GO) mod tidy && \
		$(GO) mod vendor && \
		$(GO) mod verify

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
