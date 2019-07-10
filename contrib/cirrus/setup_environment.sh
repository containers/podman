#!/bin/bash

set -e

source $(dirname $0)/lib.sh

req_env_var USER HOME GOSRC SCRIPT_BASE SETUP_MARKER_FILEPATH

show_env_vars

# Ensure this script only executes successfully once and always logs ending timestamp
[[ ! -e "$SETUP_MARKER_FILEPATH" ]] || exit 0
exithandler() {
    RET=$?
    echo "."
    echo "$(basename $0) exit status: $RET"
    [[ "$RET" -eq "0" ]] && date +%s >> "$SETUP_MARKER_FILEPATH"
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
case "${OS_REL_VER}" in
    ubuntu-18) ;;
    fedora-30) ;;
    fedora-29) ;;
    centos-7)  # Current VM is an image-builder-image no local podman/testing
        echo "No further setup required for VM image building"
        exit 0
        ;;
    *) bad_os_id_ver ;;
esac

# Reload to incorporate any changes from above
source "$SCRIPT_BASE/lib.sh"

install_test_configs

make install.tools

case "$SPECIALMODE" in
    none) ;;  # Do the normal thing
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
        ;;
    in_podman)  # Assumed to be Fedora
        dnf install -y podman
        $SCRIPT_BASE/setup_container_environment.sh
        ;;
    windows) ;&  # for podman-remote building only
    darwin) ;;
    *)
        die 111 "Unsupported \$SPECIAL_MODE: $SPECIALMODE"
esac
