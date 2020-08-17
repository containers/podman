#!/usr/bin/env bash

set -e

source $(dirname $0)/lib.sh

req_env_var CIRRUS_WORKING_DIR OS_RELEASE_ID RCLI

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
    podman) showrun ./bin/podman system info ;;
    varlink)
       if [[ "$RCLI" == "true" ]]
       then
          echo "(Trailing 100 lines of $VARLINK_LOG)"
          showrun tail -100 $VARLINK_LOG
       else
          die 0 "\$RCLI is not 'true': $RCLI"
       fi
       ;;
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
                cat /etc/fedora-release
                PKG_LST_CMD='rpm -q --qf=%{N}-%{V}-%{R}-%{ARCH}\n'
                PKG_NAMES+=(\
                    container-selinux \
                    crun \
                    libseccomp \
                    runc \
                )
                ;;
            ubuntu*)
                cat /etc/issue
                PKG_LST_CMD='dpkg-query --show --showformat=${Package}-${Version}-${Architecture}\n'
                PKG_NAMES+=(\
                    cri-o-runc \
                    libseccomp2 \
                )
                ;;
            *) bad_os_id_ver ;;
        esac
        echo "Kernel: " $(uname -r)
        echo "Cgroups: " $(stat -f -c %T /sys/fs/cgroup)
        # Any not-present packages will be listed as such
        $PKG_LST_CMD ${PKG_NAMES[@]} | sort -u
        ;;
    *) die 1 "Warning, $(basename $0) doesn't know how to handle the parameter '$1'"
esac
