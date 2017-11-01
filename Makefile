GO ?= go
EPOCH_TEST_COMMIT ?= 1cc5a27
PROJECT := github.com/kubernetes-incubator/cri-o
GIT_BRANCH := $(shell git rev-parse --abbrev-ref HEAD 2>/dev/null)
GIT_BRANCH_CLEAN := $(shell echo $(GIT_BRANCH) | sed -e "s/[^[:alnum:]]/-/g")
CRIO_IMAGE := crio_dev$(if $(GIT_BRANCH_CLEAN),:$(GIT_BRANCH_CLEAN))
CRIO_INSTANCE := crio_dev
PREFIX ?= ${DESTDIR}/usr/local
BINDIR ?= ${PREFIX}/bin
LIBEXECDIR ?= ${PREFIX}/libexec
MANDIR ?= ${PREFIX}/share/man
ETCDIR ?= ${DESTDIR}/etc
ETCDIR_CRIO ?= ${ETCDIR}/crio
BUILDTAGS ?= seccomp $(shell hack/btrfs_tag.sh) $(shell hack/libdm_tag.sh) $(shell hack/btrfs_installed_tag.sh) $(shell hack/ostree_tag.sh) $(shell hack/selinux_tag.sh)

BASHINSTALLDIR=${PREFIX}/share/bash-completion/completions
OCIUMOUNTINSTALLDIR=$(PREFIX)/share/oci-umount/oci-umount.d

SELINUXOPT ?= $(shell test -x /usr/sbin/selinuxenabled && selinuxenabled && echo -Z)
PACKAGES ?= $(shell go list -tags "${BUILDTAGS}" ./... | grep -v github.com/kubernetes-incubator/cri-o/vendor)

COMMIT_NO := $(shell git rev-parse HEAD 2> /dev/null || true)
GIT_COMMIT := $(if $(shell git status --porcelain --untracked-files=no),"${COMMIT_NO}-dirty","${COMMIT_NO}")
BUILD_INFO := $(shell date +%s)

KPOD_VERSION := ${shell cat ./KPOD_VERSION}

# If GOPATH not specified, use one in the local directory
ifeq ($(GOPATH),)
export GOPATH := $(CURDIR)/_output
unexport GOBIN
endif
GOPKGDIR := $(GOPATH)/src/$(PROJECT)
GOPKGBASEDIR := $(shell dirname "$(GOPKGDIR)")

# Update VPATH so make finds .gopathok
VPATH := $(VPATH):$(GOPATH)
SHRINKFLAGS := -s -w
BASE_LDFLAGS := ${SHRINKFLAGS} -X main.gitCommit=${GIT_COMMIT} -X main.buildInfo=${BUILD_INFO}
KPOD_LDFLAGS := -X main.kpodVersion=${KPOD_VERSION}
LDFLAGS := -ldflags '${BASE_LDFLAGS}'
LDFLAGS_KPOD := -ldflags '${BASE_LDFLAGS} ${KPOD_LDFLAGS}'

all: binaries crio.conf docs

default: help

help:
	@echo "Usage: make <target>"
	@echo
	@echo " * 'install' - Install binaries to system locations"
	@echo " * 'binaries' - Build crio, conmon, pause, crioctl and kpod"
	@echo " * 'integration' - Execute integration tests"
	@echo " * 'clean' - Clean artifacts"
	@echo " * 'lint' - Execute the source code linter"
	@echo " * 'gofmt' - Verify the source code gofmt"

.gopathok:
ifeq ("$(wildcard $(GOPKGDIR))","")
	mkdir -p "$(GOPKGBASEDIR)"
	ln -s "$(CURDIR)" "$(GOPKGBASEDIR)"
endif
	touch "$(GOPATH)/.gopathok"

lint: .gopathok
	@echo "checking lint"
	@./.tool/lint

gofmt:
	@./hack/verify-gofmt.sh

conmon:
	$(MAKE) -C $@

pause:
	$(MAKE) -C $@

test/bin2img/bin2img: .gopathok $(wildcard test/bin2img/*.go)
	$(GO) build $(LDFLAGS) -tags "$(BUILDTAGS) containers_image_ostree_stub" -o $@ $(PROJECT)/test/bin2img

test/copyimg/copyimg: .gopathok $(wildcard test/copyimg/*.go)
	$(GO) build $(LDFLAGS) -tags "$(BUILDTAGS) containers_image_ostree_stub" -o $@ $(PROJECT)/test/copyimg

test/checkseccomp/checkseccomp: .gopathok $(wildcard test/checkseccomp/*.go)
	$(GO) build $(LDFLAGS) -tags "$(BUILDTAGS) containers_image_ostree_stub" -o $@ $(PROJECT)/test/checkseccomp

crio: .gopathok $(shell hack/find-godeps.sh $(GOPKGDIR) cmd/crio $(PROJECT))
	$(GO) build $(LDFLAGS) -tags "$(BUILDTAGS) containers_image_ostree_stub" -o bin/$@ $(PROJECT)/cmd/crio

crioctl: .gopathok $(shell hack/find-godeps.sh $(GOPKGDIR) cmd/crioctl $(PROJECT))
	$(GO) build $(LDFLAGS) -tags "$(BUILDTAGS) containers_image_ostree_stub" -o bin/$@ $(PROJECT)/cmd/crioctl

kpod: .gopathok $(shell hack/find-godeps.sh $(GOPKGDIR) cmd/kpod $(PROJECT))
	$(GO) build $(LDFLAGS_KPOD) -tags "$(BUILDTAGS)" -o bin/$@ $(PROJECT)/cmd/kpod

crio.conf: crio
	./bin/crio --config="" config --default > crio.conf

clean:
ifneq ($(GOPATH),)
	rm -f "$(GOPATH)/.gopathok"
endif
	rm -rf _output
	rm -f docs/*.1 docs/*.5 docs/*.8
	rm -fr test/testdata/redis-image
	find . -name \*~ -delete
	find . -name \#\* -delete
	rm -f bin/crioctl bin/crio bin/kpod
	make -C conmon clean
	make -C pause clean
	rm -f test/bin2img/bin2img
	rm -f test/copyimg/copyimg
	rm -f test/checkseccomp/checkseccomp

crioimage:
	docker build -t ${CRIO_IMAGE} .

dbuild: crioimage
	docker run --name=${CRIO_INSTANCE} --privileged ${CRIO_IMAGE} -v ${PWD}:/go/src/${PROJECT} --rm make binaries

integration: crioimage
	docker run -e STORAGE_OPTIONS="--storage-driver=vfs" -e TESTFLAGS -e TRAVIS -t --privileged --rm -v ${CURDIR}:/go/src/${PROJECT} ${CRIO_IMAGE} make localintegration

testunit:
	$(GO) test -tags "$(BUILDTAGS)" -cover $(PACKAGES)

localintegration: clean binaries test-binaries
	./test/test_runner.sh ${TESTFLAGS}

binaries: crio conmon pause kpod crioctl
test-binaries: test/bin2img/bin2img test/copyimg/copyimg test/checkseccomp/checkseccomp

MANPAGES_MD := $(wildcard docs/*.md)
MANPAGES    := $(MANPAGES_MD:%.md=%)

docs/%.1: docs/%.1.md .gopathok
	(go-md2man -in $< -out $@.tmp && touch $@.tmp && mv $@.tmp $@) || ($(GOPATH)/bin/go-md2man -in $< -out $@.tmp && touch $@.tmp && mv $@.tmp $@)

docs/%.5: docs/%.5.md .gopathok
	(go-md2man -in $< -out $@.tmp && touch $@.tmp && mv $@.tmp $@) || ($(GOPATH)/bin/go-md2man -in $< -out $@.tmp && touch $@.tmp && mv $@.tmp $@)

docs/%.8: docs/%.8.md .gopathok
	(go-md2man -in $< -out $@.tmp && touch $@.tmp && mv $@.tmp $@) || ($(GOPATH)/bin/go-md2man -in $< -out $@.tmp && touch $@.tmp && mv $@.tmp $@)

docs: $(MANPAGES)

install: .gopathok install.bin install.man

install.bin:
	install ${SELINUXOPT} -D -m 755 bin/crio $(BINDIR)/crio
	install ${SELINUXOPT} -D -m 755 bin/crioctl $(BINDIR)/crioctl
	install ${SELINUXOPT} -D -m 755 bin/kpod $(BINDIR)/kpod
	install ${SELINUXOPT} -D -m 755 bin/conmon $(LIBEXECDIR)/crio/conmon
	install ${SELINUXOPT} -D -m 755 bin/pause $(LIBEXECDIR)/crio/pause

install.man:
	install ${SELINUXOPT} -d -m 755 $(MANDIR)/man1
	install ${SELINUXOPT} -d -m 755 $(MANDIR)/man5
	install ${SELINUXOPT} -d -m 755 $(MANDIR)/man8
	install ${SELINUXOPT} -m 644 $(filter %.1,$(MANPAGES)) -t $(MANDIR)/man1
	install ${SELINUXOPT} -m 644 $(filter %.5,$(MANPAGES)) -t $(MANDIR)/man5
	install ${SELINUXOPT} -m 644 $(filter %.8,$(MANPAGES)) -t $(MANDIR)/man8

install.config:
	install ${SELINUXOPT} -D -m 644 crio.conf $(ETCDIR_CRIO)/crio.conf
	install ${SELINUXOPT} -D -m 644 seccomp.json $(ETCDIR_CRIO)/seccomp.json
	install ${SELINUXOPT} -D -m 644 crio-umount.conf $(OCIUMOUNTINSTALLDIR)/crio-umount.conf

install.completions:
	install ${SELINUXOPT} -d -m 755 ${BASHINSTALLDIR}
	install ${SELINUXOPT} -m 644 -D completions/bash/kpod ${BASHINSTALLDIR}

install.systemd:
	install ${SELINUXOPT} -D -m 644 contrib/systemd/crio.service $(PREFIX)/lib/systemd/system/crio.service
	ln -sf crio.service $(PREFIX)/lib/systemd/system/cri-o.service
	install ${SELINUXOPT} -D -m 644 contrib/systemd/crio-shutdown.service $(PREFIX)/lib/systemd/system/crio-shutdown.service

uninstall:
	rm -f $(BINDIR)/crio
	rm -f $(BINDIR)/crioctl
	rm -f $(LIBEXECDIR)/crio/conmon
	rm -f $(LIBEXECDIR)/crio/pause
	for i in $(filter %.1,$(MANPAGES)); do \
		rm -f $(MANDIR)/man8/$$(basename $${i}); \
	done
	for i in $(filter %.5,$(MANPAGES)); do \
		rm -f $(MANDIR)/man5/$$(basename $${i}); \
	done
	for i in $(filter %.8,$(MANPAGES)); do \
		rm -f $(MANDIR)/man8/$$(basename $${i}); \
	done

.PHONY: .gitvalidation
# When this is running in travis, it will only check the travis commit range
.gitvalidation: .gopathok
ifeq ($(TRAVIS),true)
	GIT_CHECK_EXCLUDE="./vendor" $(GOPATH)/bin/git-validation -q -run DCO,short-subject,dangling-whitespace
else
	GIT_CHECK_EXCLUDE="./vendor" $(GOPATH)/bin/git-validation -v -run DCO,short-subject,dangling-whitespace -range $(EPOCH_TEST_COMMIT)..HEAD
endif

.PHONY: install.tools

install.tools: .install.gitvalidation .install.gometalinter .install.md2man

.install.gitvalidation: .gopathok
	if [ ! -x "$(GOPATH)/bin/git-validation" ]; then \
		go get -u github.com/vbatts/git-validation; \
	fi

.install.gometalinter: .gopathok
	if [ ! -x "$(GOPATH)/bin/gometalinter" ]; then \
		go get -u github.com/alecthomas/gometalinter; \
		cd $(GOPATH)/src/github.com/alecthomas/gometalinter; \
		git checkout 23261fa046586808612c61da7a81d75a658e0814; \
		go install github.com/alecthomas/gometalinter; \
		$(GOPATH)/bin/gometalinter --install; \
	fi

.install.md2man: .gopathok
	if [ ! -x "$(GOPATH)/bin/go-md2man" ]; then \
		go get -u github.com/cpuguy83/go-md2man; \
	fi

.install.ostree: .gopathok
	if ! pkg-config ostree-1 2> /dev/null ; then \
		git clone https://github.com/ostreedev/ostree $(GOPATH)/src/github.com/ostreedev/ostree ; \
		cd $(GOPATH)/src/github.com/ostreedev/ostree ; \
		./autogen.sh --prefix=/usr/local; \
		make all install; \
	fi

.PHONY: \
	binaries \
	clean \
	conmon \
	default \
	docs \
	gofmt \
	help \
	install \
	lint \
	pause \
	uninstall
