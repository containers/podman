#!/bin/bash
set -xeuo pipefail
export GOPATH=/go
export PATH=$HOME/gopath/bin:$PATH:$GOPATH/bin
export GOSRC=$GOPATH/src/github.com/containers/buildah

cp -fv /etc/yum.repos.d{.host/*.repo,}

dnf install -y \
  bats \
  btrfs-progs-devel \
  bzip2 \
  device-mapper-devel \
  findutils \
  git \
  glib2-devel \
  glibc-static \
  gnupg \
  golang \
  gpgme-devel \
  libassuan-devel \
  libseccomp-devel \
  libselinux-devel \
  libselinux-static \
  libseccomp-static \
  libselinux-utils \
  make \
  openssl \
  skopeo-containers \
  which


# Install gomega
go get github.com/onsi/gomega/...

# PAPR adds a merge commit, for testing, which fails the
# short-commit-subject validation test, so tell git-validate.sh to only check
# up to, but not including, the merge commit.
export GITVALIDATE_TIP=$(cd $GOSRC; git log -2 --pretty='%H' | tail -n 1)
make -C $GOSRC install.tools runc all validate test-unit test-integration static
env BUILDAH_ISOLATION=chroot make -C $GOSRC test-integration
env BUILDAH_ISOLATION=rootless make -C $GOSRC test-integration
