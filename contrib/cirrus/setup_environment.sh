#!/bin/bash

set -e

source $(dirname $0)/lib.sh

req_env_var USER HOME ENVLIB SCRIPT_BASE CIRRUS_BUILD_ID

[[ "$SHELL" =~ "bash" ]] || chsh -s /bin/bash

cd "$CIRRUS_WORKING_DIR"  # for clarity of initial conditions

# Verify basic dependencies
for depbin in go rsync unzip sha256sum curl make python3 git
do
    if ! type -P "$depbin" &> /dev/null
    then
        echo "***** WARNING: $depbin binary not found in $PATH *****"
    fi
done

# Setup env. vars common to all tasks/scripts/platforms and
# ensure they return for every following script execution.
MARK="# Added by $0, manual changes will be lost."
touch "$HOME/$ENVLIB"
if ! grep -q "$MARK" "$HOME/$ENVLIB"
then
    cp "$HOME/$ENVLIB" "$HOME/${ENVLIB}_original"
    # N/B: Single-quote items evaluated every time, double-quotes only once (right now).
    for envstr in \
        "$MARK" \
        "export EPOCH_TEST_COMMIT=\"$CIRRUS_BASE_SHA\"" \
        "export HEAD=\"$CIRRUS_CHANGE_IN_REPO\"" \
        "export TRAVIS=\"1\"" \
        "export GOSRC=\"$CIRRUS_WORKING_DIR\"" \
        "export OS_RELEASE_ID=\"$(os_release_id)\"" \
        "export OS_RELEASE_VER=\"$(os_release_ver)\"" \
        "export OS_REL_VER=\"$(os_release_id)-$(os_release_ver)\"" \
        "export TEST_REMOTE_CLIENT=\"$TEST_REMOTE_CLIENT\"" \
        "export GOPATH=\"/var/tmp/go\"" \
        'export PATH="$HOME/bin:$GOPATH/bin:/usr/local/bin:$PATH"' \
        'export LD_LIBRARY_PATH="/usr/local/lib${LD_LIBRARY_PATH:+:${LD_LIBRARY_PATH}}"'
    do
        # Make permanent in later shells, and set in current shell
        X=$(echo "$envstr" | tee -a "$HOME/$ENVLIB") && eval "$X" && echo "$X"
    done

    # Some environment setup needs to vary between distros
    # Note: This should only be used for environment variables, and minor details.
    #       Anything that could vary from one run to the next, should go into
    #       contrib/cirrus/packer/*_setup.sh and be incorporated into VM cache-images
    #       (see docs)
    case "${OS_RELEASE_ID}-${OS_RELEASE_VER}" in
        ubuntu-18) ;;
        fedora-29) ;;
        fedora-28) ;;
        centos-7) ;;
        rhel-7) ;;
        *) bad_os_id_ver ;;
    esac

    cd "${GOSRC}/"
    # Reload to incorporate any changes from above
    source "$SCRIPT_BASE/lib.sh"

    case "$SPECIALMODE" in
        rootless)
            X=$(echo "export ROOTLESS_USER='some${RANDOM}dude'" | \
                tee -a "$HOME/$ENVLIB") && eval "$X" && echo "$X"
            setup_rootless
            ;;
        in_podman)  # Assumed to be Fedora
            dnf install -y podman buildah
            $SCRIPT_BASE/setup_container_environment.sh
            ;;
    esac
fi

show_env_vars
