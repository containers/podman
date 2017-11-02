#!/bin/bash
set -xeuo pipefail

export GOPATH=$HOME/gopath
export PATH=$HOME/gopath/bin:$PATH
export GOSRC=$HOME/gopath/src/github.com/projectatomic/libpod

(mkdir -p $GOSRC && cd /code && cp -r . $GOSRC)

dnf install -y \
  bats \
  btrfs-progs-devel \
  bzip2 \
  device-mapper-devel \
  findutils \
  git \
  glib2-devel \
  gnupg \
  golang \
  gpgme-devel \
  libassuan-devel \
  libseccomp-devel \
  libselinux-devel \
  skopeo-containers \
  runc \
  make \
  ostree-devel \
  python \
  which

# PAPR adds a merge commit, for testing, which fails the
# short-commit-subject validation test, so tell git-validate.sh to only check
# up to, but not including, the merge commit.
export GITVALIDATE_TIP=$(cd $GOSRC; git log -2 --pretty='%H' | tail -n 1)
export TAGS="seccomp $($GOSRC/hack/btrfs_tag.sh) $($GOSRC/hack/libdm_tag.sh) $($GOSRC/hack/btrfs_installed_tag.sh) $($GOSRC/hack/ostree_tag.sh) $($GOSRC/hack/selinux_tag.sh)"
make -C $GOSRC binaries install.tools all gofmt localintegration testunit TAGS="${TAGS}"
#make -C $GOSRC lint
