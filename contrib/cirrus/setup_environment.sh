#!/bin/bash

set -e

source $(dirname $0)/lib.sh

req_env_var USER HOME GOSRC SCRIPT_BASE SETUP_MARKER_FILEPATH

# Ensure this script only executes successfully once and always logs ending timestamp
if [[ -e "$SETUP_MARKER_FILEPATH" ]]; then
    show_env_vars
    exit 0
fi

exithandler() {
    RET=$?
    echo "."
    echo "$(basename $0) exit status: $RET"
    [[ "$RET" -eq "0" ]] && date +%s >> "$SETUP_MARKER_FILEPATH"
    show_env_vars
    [[ "$RET" -eq "0" ]] || warn "Non-zero exit caused by error ABOVE env. var. display."
}
trap exithandler EXIT

# Verify basic dependencies
for depbin in go rsync unzip sha256sum curl make python3 git
do
    if ! type -P "$depbin" &> /dev/null
    then
        echo "***** WARNING: $depbin binary not found in $PATH *****"
    fi
done

# Sometimes environment setup needs to vary between distros
# Note: This should only be used for environment variables, and temporary workarounds.
#       Anything externally dependent, should be made fixed-in-time by adding to
#       contrib/cirrus/packer/*_setup.sh to be incorporated into VM cache-images
#       (see docs).
cd "${GOSRC}/"
case "${OS_RELEASE_ID}" in
    ubuntu)
        ;;
    fedora)
        # All SELinux distros need this for systemd-in-a-container
        setsebool container_manage_cgroup true

        workaround_bfq_bug

        if [[ "$ADD_SECOND_PARTITION" == "true" ]]; then
            bash "$SCRIPT_BASE/add_second_partition.sh"
        fi

        warn "Forcing systemd cgroup manager"
        X=$(echo "export CGROUP_MANAGER=systemd" | \
            tee -a /etc/environment) && eval "$X" && echo "$X"
        ;;
    centos)  # Current VM is an image-builder-image no local podman/testing
        echo "No further setup required for VM image building"
        exit 0
        ;;
    *) bad_os_id_ver ;;
esac

# Reload to incorporate any changes from above
source "$SCRIPT_BASE/lib.sh"

case "$CG_FS_TYPE" in
    tmpfs)
        warn "Forcing testing with runc instead of crun"
        X=$(echo "export OCI_RUNTIME=/usr/bin/runc" | \
            tee -a /etc/environment) && eval "$X" && echo "$X"
        ;;
    cgroup2fs)
        # This is necessary since we've built/installed from source, which uses runc as the default.
        warn "Forcing testing with crun instead of runc"
        X=$(echo "export OCI_RUNTIME=/usr/bin/crun" | \
            tee -a /etc/environment) && eval "$X" && echo "$X"

        if [[ "$MOD_LIBPOD_CONF" == "true" ]]; then
            warn "Updating runtime setting in repo. copy of libpod.conf"
            sed -i -r -e 's/^runtime = "runc"/runtime = "crun"/' $GOSRC/libpod.conf
            git diff $GOSRC/libpod.conf
        fi

        if [[ "$OS_RELEASE_ID" == "fedora" ]]; then
            warn "Upgrading to the latest crun"
            # Normally not something to do for stable testing
            # but crun is new, and late-breaking fixes may be required
            # on short notice
            dnf update -y crun containers-common
        fi
        ;;
    *)
        die 110 "Unsure how to handle cgroup filesystem type '$CG_FS_TYPE'"
        ;;
esac

# Must execute before possible setup_rootless()
make install.tools

case "$SPECIALMODE" in
    none)
        [[ -n "$CROSS_PLATFORM" ]] || \
            remove_packaged_podman_files
        ;;
    endpoint)
        remove_packaged_podman_files
        ;;
    bindings)
        remove_packaged_podman_files
        ;;
    rootless)
        # Only do this once, even if ROOTLESS_USER (somehow) changes
        if ! grep -q 'ROOTLESS_USER' /etc/environment
        then
            X=$(echo "export ROOTLESS_USER='${ROOTLESS_USER:-some${RANDOM}dude}'" | \
                tee -a /etc/environment) && eval "$X" && echo "$X"
            X=$(echo "export SPECIALMODE='${SPECIALMODE}'" | \
                tee -a /etc/environment) && eval "$X" && echo "$X"
            X=$(echo "export TEST_REMOTE_CLIENT='${TEST_REMOTE_CLIENT}'" | \
                tee -a /etc/environment) && eval "$X" && echo "$X"
            setup_rootless
        fi
        remove_packaged_podman_files
        ;;
    in_podman)  # Assumed to be Fedora
        $SCRIPT_BASE/setup_container_environment.sh
        ;;
    *)
        die 111 "Unsupported \$SPECIALMODE: $SPECIALMODE"
esac

install_test_configs
