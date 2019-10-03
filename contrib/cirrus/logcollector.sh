#!/bin/bash

set -e

source $(dirname $0)/lib.sh

req_env_var CIRRUS_WORKING_DIR OS_RELEASE_ID

# Assume there are other log collection commands to follow - Don't
# let one break another that may be useful, but also keep any
# actual script-problems fatal so they are noticed right away.
showrun() {
    echo '+ '$(printf " %q" "$@")
    set +e
    echo '------------------------------------------------------------'
    "$@"
    local status=$?
    [[ $status -eq 0 ]] || \
        echo "[ rc = $status -- proceeding anyway ]"
    echo '------------------------------------------------------------'
    set -e
}

case $1 in
    audit)
        case $OS_RELEASE_ID in
            ubuntu) showrun cat /var/log/kern.log ;;
            fedora) showrun cat /var/log/audit/audit.log ;;
            *) bad_os_id_ver ;;
        esac
        ;;
    df) showrun df -lhTx tmpfs ;;
    ginkgo) showrun cat $CIRRUS_WORKING_DIR/test/e2e/ginkgo-node-*.log ;;
    journal) showrun journalctl -b ;;
    packages)
        # These names are common to Fedora and Ubuntu
        PKG_NAMES=(\
                    conmon \
                    containernetworking-plugins \
                    containers-common \
                    criu \
                    golang \
                    podman \
                    skopeo \
                    slirp4netns \
        )
        case $OS_RELEASE_ID in
            fedora*)
                PKG_LST_CMD='rpm -q --qf=%{N}-%{V}-%{R}-%{ARCH}\n'
                PKG_NAMES+=(\
                    container-selinux \
                    crun \
                    runc \
                )
                ;;
            ubuntu*)
                PKG_LST_CMD='dpkg-query --show --showformat=${Package}-${Version}-${Architecture}\n'
                PKG_NAMES+=(\
                    cri-o-runc \
                )
                ;;
            *) bad_os_id_ver ;;
        esac
        # Any not-present packages will be listed as such
        $PKG_LST_CMD ${PKG_NAMES[@]} | sort -u
        ;;
    *) die 1 "Warning, $(basename $0) doesn't know how to handle the parameter '$1'"
esac
