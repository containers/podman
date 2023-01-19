#!/usr/bin/env bash

set -e

# shellcheck source=contrib/cirrus/lib.sh
source $(dirname $0)/lib.sh

req_env_vars CIRRUS_WORKING_DIR OS_RELEASE_ID

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
    journal) showrun journalctl -b ;;
    podman) showrun ./bin/podman system info ;;
    packages)
        # These names are common to Fedora and Ubuntu
        PKG_NAMES=(\
                    conmon
                    containernetworking-plugins
                    containers-common
                    criu
                    crun
                    golang
                    podman
                    runc
                    skopeo
                    slirp4netns
        )
        case $OS_RELEASE_ID in
            fedora)
                cat /etc/fedora-release
                PKG_LST_CMD='rpm -q --qf=%{N}-%{V}-%{R}-%{ARCH}\n'
                PKG_NAMES+=(\
                    aardvark-dns
                    container-selinux
                    libseccomp
                    netavark
                    passt
                )
                ;;
            ubuntu)
                cat /etc/issue
                PKG_LST_CMD='dpkg-query --show --showformat=${Package}-${Version}-${Architecture}\n'
                PKG_NAMES+=(\
                    cri-o-runc
                    libseccomp2
                )
                ;;
            *) bad_os_id_ver ;;
        esac
        echo "Kernel: " $(uname -r)
        echo "Cgroups: " $(stat -f -c %T /sys/fs/cgroup)
        # Any not-present packages will be listed as such
        $PKG_LST_CMD "${PKG_NAMES[@]}" | sort -u
        ;;
    time)
        # Assumed to be empty/undefined outside of Cirrus-CI (.cirrus.yml)
        # shellcheck disable=SC2154
        if [[ -r "$STATS_LOGFILE" ]]; then cat "$STATS_LOGFILE"; fi
        ;;
    *) die "Warning, $(basename $0) doesn't know how to handle the parameter '$1'"
esac
