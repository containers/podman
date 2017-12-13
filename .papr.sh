#!/bin/bash
set -xeuo pipefail

DIST=$(cat /etc/redhat-release  | awk '{print $1}')
IMAGE=registry.fedoraproject.org/fedora:26
PACKAGER=dnf
if [[ ${DIST} != "Fedora" ]]; then
    PACKAGER=yum
    IMAGE=registry.centos.org/centos/centos:7
fi

if test -z "${INSIDE_CONTAINER:-}"; then
    if [ -f /run/ostree-booted ]; then

        # by default, the root LV on AH is only 3G, but we need a
        # bit more for our tests
        lvresize -r -L +4G atomicos/root

        if [ ! -e /var/tmp/ostree-unlock-ovl.* ]; then
            ostree admin unlock
        fi
    fi
    # Restarting docker helps with permissions related to above.
    systemctl restart docker

    # somewhat mimic the spec conditional
    source /etc/os-release
    if [ "$ID" == fedora ]; then
      PYTHON=python3
    else
      PYTHON=python
    fi
    docker run --rm \
               --privileged \
               -v $PWD:/go/src/github.com/projectatomic/libpod \
               -v /etc/yum.repos.d:/etc/yum.repos.d.host:ro \
               -v /usr:/host/usr \
               -v /etc:/host/etc \
               -v /host:/host/var \
               --workdir /go/src/github.com/projectatomic/libpod \
               -e INSIDE_CONTAINER=1 \
               -e PYTHON=$PYTHON \
               ${IMAGE} /go/src/github.com/projectatomic/libpod/.papr.sh
    systemd-detect-virt
    ./test/test_runner.sh
    exit 0
fi

export GOPATH=/go
export PATH=$HOME/gopath/bin:$PATH
export GOSRC=/$GOPATH/src/github.com/projectatomic/libpod

${PACKAGER} install -y \
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
              which\
              golang-github-cpuguy83-go-md2man


# PAPR adds a merge commit, for testing, which fails the
# short-commit-subject validation test, so tell git-validate.sh to only check
# up to, but not including, the merge commit.
export GITVALIDATE_TIP=$(cd $GOSRC; git log -2 --pretty='%H' | tail -n 1)
export TAGS="seccomp $($GOSRC/hack/btrfs_tag.sh) $($GOSRC/hack/libdm_tag.sh) $($GOSRC/hack/btrfs_installed_tag.sh) $($GOSRC/hack/ostree_tag.sh) $($GOSRC/hack/selinux_tag.sh)"

make gofmt TAGS="${TAGS}"
make testunit TAGS="${TAGS}"
make install.tools TAGS="${TAGS}"

# Only check lint and gitvalidation on more recent
# distros with updated git and tooling
if [[ ${PACKAGER} != "yum" ]]; then
    HEAD=$GITVALIDATE_TIP make -C $GOSRC .gitvalidation TAGS="${TAGS}"
    make lint
fi

make TAGS="${TAGS}"
make TAGS="${TAGS}" install PREFIX=/host/usr
make TAGS="${TAGS}" test-binaries
