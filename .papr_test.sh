#!/bin/bash
echo $CGROUP_MANAGER
# Install ginkgo
set -xeuo pipefail

export GOPATH=/go
export PATH=$HOME/gopath/bin:$PATH
export GOSRC=$GOPATH/src/github.com/projectatomic/libpod

mkdir -p $GOSRC
mount -o bind /var/tmp/checkout $GOSRC
cd $GOSRC

go get -u github.com/onsi/ginkgo/ginkgo \
    && install -D -m 755 "$GOPATH"/bin/ginkgo /usr/bin/

# Install gomega
go get github.com/onsi/gomega/...

# Install varlink stuff
pip3 install varlink

# PAPR adds a merge commit, for testing, which fails the
# short-commit-subject validation test, so tell git-validate.sh to only check
# up to, but not including, the merge commit.
export GITVALIDATE_TIP=$(cd $GOSRC; git log -2 --pretty='%H' | tail -n 1)
export TAGS="seccomp $($GOSRC/hack/btrfs_tag.sh) $($GOSRC/hack/libdm_tag.sh) $($GOSRC/hack/btrfs_installed_tag.sh) $($GOSRC/hack/ostree_tag.sh) $($GOSRC/hack/selinux_tag.sh)"

cd $GOSRC

# Make sure we have some policy for pulling images
mkdir -p /etc/containers
cp -v test/policy.json /etc/containers/policy.json
cp -v test/redhat_sigstore.yaml /etc/containers/registries.d/registry.access.redhat.com.yaml

# Install cni config
mkdir -p /etc/cni/net.d/
cp -v cni/87-podman-bridge.conflist /etc/cni/net.d/87-podman-bridge.conflist

# Make and install podman
make install.tools TAGS="${TAGS}"
make TAGS="${TAGS}"
make TAGS="${TAGS}" install PREFIX=/usr ETCDIR=/etc
make TAGS="${TAGS}" test-binaries

# Run the ginkgo integration tests
#SKIP_USERNS=1 GOPATH=/go make localintegration
ginkgo -v -cover -flakeAttempts 3 -progress -trace -noColor test/e2e/.
exit 0
