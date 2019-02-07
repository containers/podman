GO ?= go
DESTDIR ?= /
EPOCH_TEST_COMMIT ?= 4406e1cfeed18fe89c0ad4e20a3c3b2f4b9ffcae
HEAD ?= HEAD
CHANGELOG_BASE ?= HEAD~
CHANGELOG_TARGET ?= HEAD
PROJECT := github.com/containers/libpod
GIT_BASE_BRANCH ?= origin/master
GIT_BRANCH ?= $(shell git rev-parse --abbrev-ref HEAD 2>/dev/null)
GIT_BRANCH_CLEAN ?= $(shell echo $(GIT_BRANCH) | sed -e "s/[^[:alnum:]]/-/g")
LIBPOD_IMAGE ?= libpod_dev$(if $(GIT_BRANCH_CLEAN),:$(GIT_BRANCH_CLEAN))
LIBPOD_INSTANCE := libpod_dev
PREFIX ?= ${DESTDIR}/usr/local
BINDIR ?= ${PREFIX}/bin
LIBEXECDIR ?= ${PREFIX}/libexec
MANDIR ?= ${PREFIX}/share/man
SHAREDIR_CONTAINERS ?= ${PREFIX}/share/containers
ETCDIR ?= ${DESTDIR}/etc
ETCDIR_LIBPOD ?= ${ETCDIR}/crio
TMPFILESDIR ?= ${PREFIX}/lib/tmpfiles.d
SYSTEMDDIR ?= ${PREFIX}/lib/systemd/system
BUILDTAGS ?= seccomp $(shell hack/btrfs_tag.sh) $(shell hack/btrfs_installed_tag.sh) $(shell hack/ostree_tag.sh) $(shell hack/selinux_tag.sh) $(shell hack/apparmor_tag.sh) varlink exclude_graphdriver_devicemapper
BUILDTAGS_CROSS ?= containers_image_openpgp containers_image_ostree_stub exclude_graphdriver_btrfs exclude_graphdriver_devicemapper exclude_graphdriver_overlay
ifneq (,$(findstring varlink,$(BUILDTAGS)))
	PODMAN_VARLINK_DEPENDENCIES = cmd/podman/varlink/iopodman.go
endif
CONTAINER_RUNTIME := $(shell command -v podman 2> /dev/null || echo docker)
OCI_RUNTIME ?= ""

BASHINSTALLDIR=${PREFIX}/share/bash-completion/completions
OCIUMOUNTINSTALLDIR=$(PREFIX)/share/oci-umount/oci-umount.d

SELINUXOPT ?= $(shell test -x /usr/sbin/selinuxenabled && selinuxenabled && echo -Z)
PACKAGES ?= $(shell $(GO) list -tags "${BUILDTAGS}" ./... | grep -v github.com/containers/libpod/vendor | grep -v e2e | grep -v system )

COMMIT_NO ?= $(shell git rev-parse HEAD 2> /dev/null || true)
GIT_COMMIT ?= $(if $(shell git status --porcelain --untracked-files=no),"${COMMIT_NO}-dirty","${COMMIT_NO}")
BUILD_INFO ?= $(shell date +%s)
LIBPOD := ${PROJECT}/libpod
LDFLAGS_PODMAN ?= $(LDFLAGS) -X $(LIBPOD).gitCommit=$(GIT_COMMIT) -X $(LIBPOD).buildInfo=$(BUILD_INFO)
ISODATE ?= $(shell date --iso-8601)
#Update to LIBSECCOMP_COMMIT should reflect in Dockerfile too.
LIBSECCOMP_COMMIT := release-2.3

# Rarely if ever should integration tests take more than 50min,
# caller may override in special circumstances if needed.
GINKGOTIMEOUT ?= -timeout=50m

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
	ln -sf "$(CURDIR)" "$(GOPKGBASEDIR)"
endif
	touch $@

lint: .gopathok varlink_generate ## Execute the source code linter
	@echo "checking lint"
	@./.tool/lint

gofmt: ## Verify the source code gofmt
	find . -name '*.go' ! -path './vendor/*' -exec gofmt -s -w {} \+
	git diff --exit-code

test/bin2img/bin2img: .gopathok $(wildcard test/bin2img/*.go)
	$(GO) build -ldflags '$(LDFLAGS)' -tags "$(BUILDTAGS) containers_image_ostree_stub" -o $@ $(PROJECT)/test/bin2img

test/copyimg/copyimg: .gopathok $(wildcard test/copyimg/*.go)
	$(GO) build -ldflags '$(LDFLAGS)' -tags "$(BUILDTAGS) containers_image_ostree_stub" -o $@ $(PROJECT)/test/copyimg

test/checkseccomp/checkseccomp: .gopathok $(wildcard test/checkseccomp/*.go)
	$(GO) build -ldflags '$(LDFLAGS)' -tags "$(BUILDTAGS) containers_image_ostree_stub" -o $@ $(PROJECT)/test/checkseccomp

test/goecho/goecho: .gopathok $(wildcard test/goecho/*.go)
	$(GO) build -ldflags '$(LDFLAGS)' -o $@ $(PROJECT)/test/goecho

podman: .gopathok $(PODMAN_VARLINK_DEPENDENCIES) ## Build with podman
	$(GO) build -ldflags '$(LDFLAGS_PODMAN)' -tags "$(BUILDTAGS)" -o bin/$@ $(PROJECT)/cmd/podman

podman-remote: .gopathok $(PODMAN_VARLINK_DEPENDENCIES) ## Build with podman on remote environment
	$(GO) build -ldflags '$(LDFLAGS_PODMAN)' -tags "$(BUILDTAGS) remoteclient" -o bin/$@ $(PROJECT)/cmd/podman

podman-remote-darwin: .gopathok $(PODMAN_VARLINK_DEPENDENCIES) ## Build with podman on remote OSX environment
	GOOS=darwin $(GO) build -ldflags '$(LDFLAGS_PODMAN)' -tags "remoteclient containers_image_openpgp exclude_graphdriver_devicemapper" -o bin/$@ $(PROJECT)/cmd/podman

local-cross: $(CROSS_BUILD_TARGETS) ## Cross local compilation

bin/podman.cross.%: .gopathok
	TARGET="$*"; \
	GOOS="$${TARGET%%.*}" \
	GOARCH="$${TARGET##*.}" \
	$(GO) build -ldflags '$(LDFLAGS_PODMAN)' -tags '$(BUILDTAGS_CROSS)' -o "$@" $(PROJECT)/cmd/podman

clean: ## Clean artifacts
	rm -rf \
		.gopathok \
		_output \
		bin \
		build \
		test/bin2img/bin2img \
		test/checkseccomp/checkseccomp \
		test/copyimg/copyimg \
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
	$(GO) test -tags "$(BUILDTAGS)" -cover $(PACKAGES)
	$(MAKE) -C contrib/cirrus/packer test

ginkgo:
	ginkgo -v -tags "$(BUILDTAGS)" $(GINKGOTIMEOUT) -cover -flakeAttempts 3 -progress -trace -noColor test/e2e/.

ginkgo-remote:
	ginkgo -v -tags "$(BUILDTAGS) remoteclient" $(GINKGOTIMEOUT) -cover -flakeAttempts 3 -progress -trace -noColor test/e2e/.

localintegration: varlink_generate test-binaries ginkgo ginkgo-remote

localsystem: .install.ginkgo
	ginkgo -v -noColor test/system/

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

test-binaries: test/bin2img/bin2img test/copyimg/copyimg test/checkseccomp/checkseccomp test/goecho/goecho install.catatonit

MANPAGES_MD ?= $(wildcard docs/*.md pkg/*/docs/*.md)
MANPAGES ?= $(MANPAGES_MD:%.md=%)

$(MANPAGES): %: %.md .gopathok
	@sed -e 's/\((podman.*\.md)\)//' -e 's/\[\(podman.*\)\]/\1/' $<  | $(GOMD2MAN) -in /dev/stdin -out $@

docs: $(MANPAGES) ## Generate documentation

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

install: .gopathok install.bin install.man install.cni install.systemd  ## Install binaries to system locations

install.bin:
	install ${SELINUXOPT} -d -m 755 $(BINDIR)
	install ${SELINUXOPT} -m 755 bin/podman $(BINDIR)/podman
	test -z "${SELINUXOPT}" || chcon --verbose --reference=$(BINDIR)/podman bin/podman

install.man: docs
	install ${SELINUXOPT} -d -m 755 $(MANDIR)/man1
	install ${SELINUXOPT} -d -m 755 $(MANDIR)/man5
	install ${SELINUXOPT} -m 644 $(filter %.1,$(MANPAGES)) -t $(MANDIR)/man1
	install ${SELINUXOPT} -m 644 $(filter %.5,$(MANPAGES)) -t $(MANDIR)/man5
	install ${SELINUXOPT} -m 644 docs/links/*1 -t $(MANDIR)/man1

install.config:
	install ${SELINUXOPT} -d -m 755 $(SHAREDIR_CONTAINERS) $(ETCDIR_LIBPOD) $(OCIUMOUNTINSTALLDIR)
	install ${SELINUXOPT} -m 644 libpod.conf $(SHAREDIR_CONTAINERS)/libpod.conf
	install ${SELINUXOPT} -m 644 seccomp.json $(ETCDIR_LIBPOD)/seccomp.json
	install ${SELINUXOPT} -m 644 crio-umount.conf $(OCIUMOUNTINSTALLDIR)/crio-umount.conf

install.completions:
	install ${SELINUXOPT} -d -m 755 ${BASHINSTALLDIR}
	install ${SELINUXOPT} -m 644 completions/bash/podman ${BASHINSTALLDIR}

install.cni:
	install ${SELINUXOPT} -d -m 755 ${ETCDIR}/cni/net.d/
	install ${SELINUXOPT} -m 644 cni/87-podman-bridge.conflist ${ETCDIR}/cni/net.d/87-podman-bridge.conflist

install.docker: docker-docs
	install ${SELINUXOPT} -d -m 755 $(BINDIR) $(MANDIR)/man1
	install ${SELINUXOPT} -m 755 docker $(BINDIR)/docker
	install ${SELINUXOPT} -m 644 docs/docker*.1 -t $(MANDIR)/man1

install.systemd:
	install ${SELINUXOPT} -m 755 -d ${SYSTEMDDIR} ${TMPFILESDIR}
	install ${SELINUXOPT} -m 644 contrib/varlink/io.podman.socket ${SYSTEMDDIR}/io.podman.socket
	install ${SELINUXOPT} -m 644 contrib/varlink/io.podman.service ${SYSTEMDDIR}/io.podman.service
	install ${SELINUXOPT} -m 644 contrib/varlink/podman.conf ${TMPFILESDIR}/podman.conf

uninstall:
	for i in $(filter %.1,$(MANPAGES)); do \
		rm -f $(MANDIR)/man1/$$(basename $${i}); \
	done; \
	for i in $(filter %.5,$(MANPAGES)); do \
		rm -f $(MANDIR)/man5/$$(basename $${i}); \
	done

.PHONY: .gitvalidation
.gitvalidation: .gopathok
	GIT_CHECK_EXCLUDE="./vendor" $(GOBIN)/git-validation -v -run DCO,short-subject,dangling-whitespace -range $(EPOCH_TEST_COMMIT)..$(HEAD)

.PHONY: install.tools

install.tools: .install.gitvalidation .install.gometalinter .install.md2man .install.ginkgo ## Install needed tools

.install.vndr: .gopathok
	$(GO) get github.com/LK4D4/vndr

.install.ginkgo: .gopathok
	if [ ! -x "$(GOBIN)/ginkgo" ]; then \
		$(GO) build -o ${GOPATH}/bin/ginkgo ./vendor/github.com/onsi/ginkgo/ginkgo ; \
	fi

.install.gitvalidation: .gopathok
	if [ ! -x "$(GOBIN)/git-validation" ]; then \
		$(GO) get -u github.com/vbatts/git-validation; \
	fi

.install.gometalinter: .gopathok
	if [ ! -x "$(GOBIN)/gometalinter" ]; then \
		$(GO) get -u github.com/alecthomas/gometalinter; \
		cd $(FIRST_GOPATH)/src/github.com/alecthomas/gometalinter; \
		git checkout e8d801238da6f0dfd14078d68f9b53fa50a7eeb5; \
		$(GO) install github.com/alecthomas/gometalinter; \
		$(GOBIN)/gometalinter --install; \
	fi

.install.md2man: .gopathok
	if [ ! -x "$(GOBIN)/go-md2man" ]; then \
		   $(GO) get -u github.com/cpuguy83/go-md2man; \
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
	$(GO) generate ./cmd/podman/varlink/...

API.md: cmd/podman/varlink/io.podman.varlink
	$(GO) generate ./docs/...

validate.completions: completions/bash/podman
	. completions/bash/podman

validate: gofmt .gitvalidation validate.completions

build-all-new-commits:
	# Validate that all the commits build on top of $(GIT_BASE_BRANCH)
	git rebase $(GIT_BASE_BRANCH) -x make

vendor:
	vndr -whitelist "github.com/varlink/go"  \
	     -whitelist "github.com/onsi/ginkgo" \
	     -whitelist "github.com/onsi/gomega"

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
	lint \
	pause \
	uninstall \
	shell \
	changelog \
	validate \
	install.libseccomp.sudo \
	vendor
