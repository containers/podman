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
        case $OS_RELEASE_ID in
            fedora*)
                PKG_LST_CMD='rpm -q --qf=%{N}-%{V}-%{R}-%{ARCH}\n'
                PKG_NAMES=(\
                    container-selinux \
                    containernetworking-plugins \
                    containers-common \
                    criu \
                    golang \
                    podman \
                    slirp4netns \
                )
                if [[ "$OS_RELEASE_VER" -lt "31" ]]; then
                    PKG_NAMES+=(runc)
                else
                    PKG_NAMES+=(crun)
                fi
                ;;
            ubuntu*)
                PKG_LST_CMD='dpkg-query --show --showformat=${Package}-${Version}-${Architecture}\n'
                PKG_NAMES=(\
                    containernetworking-plugins \
                    containers-common \
                    cri-o-runc \
                    criu \
                    golang \
                    libvarlink \
                    podman \
                    skopeo \
                    slirp4netns \
                )
                ;;
            *) bad_os_id_ver ;;
        esac
        $PKG_LST_CMD ${PKG_NAMES[@]} | sort -u
        ;;
    *) die 1 "Warning, $(basename $0) doesn't know how to handle the parameter '$1'"
esac
