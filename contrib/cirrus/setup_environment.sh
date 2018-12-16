#!/bin/bash

set -e
source $(dirname $0)/lib.sh

req_env_var "
USER $USER
HOME $HOME
ENVLIB $ENVLIB
SCRIPT_BASE $SCRIPT_BASE
CIRRUS_BUILD_ID $CIRRUS_BUILD_ID"

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
        "export BUILT_IMAGE_SUFFIX=\"-$CIRRUS_REPO_NAME-${CIRRUS_CHANGE_IN_REPO:0:8}\"" \
        "export GOPATH=\"/var/tmp/go\"" \
        'export PATH="$HOME/bin:$GOPATH/bin:/usr/local/bin:$PATH"' \
        'export LD_LIBRARY_PATH="/usr/local/lib${LD_LIBRARY_PATH:+:${LD_LIBRARY_PATH}}"'
    do
        # Make permanent in later shells, and set in current shell
        X=$(echo "$envstr" | tee -a "$HOME/$ENVLIB") && eval "$X" && echo "$X"
    done

    # Some setup needs to vary between distros
    case "${OS_RELEASE_ID}-${OS_RELEASE_VER}" in
        ubuntu-18)
            # Always install runc on Ubuntu
            install_runc_from_git
            envstr='export BUILDTAGS="seccomp $($GOSRC/hack/btrfs_tag.sh) $($GOSRC/hack/btrfs_installed_tag.sh) $($GOSRC/hack/ostree_tag.sh) varlink exclude_graphdriver_devicemapper"'
            ;;
        fedora-29) ;&  # Continue to the next item
        fedora-28)
            RUNC="https://kojipkgs.fedoraproject.org/packages/runc/1.0.0/55.dev.git578fe65.fc${OS_RELEASE_VER}/x86_64/runc-1.0.0-55.dev.git578fe65.fc${OS_RELEASE_VER}.x86_64.rpm"
            echo ">>>>> OVERRIDING RUNC WITH $RUNC <<<<<"
            dnf -y install "$RUNC"
            ;&  # Continue to the next item
        centos-7) ;&
        rhel-7)
            envstr='unset BUILDTAGS'  # Use default from Makefile
            ;;
        *) bad_os_id_ver ;;
    esac
    X=$(echo "$envstr" | tee -a "$HOME/$ENVLIB") && eval "$X" && echo "$X"

    # Do the same for golang env. vars
    go env | while read envline
    do
        X=$(echo "export $envline" | tee -a "$HOME/$ENVLIB") && eval "$X" && echo "$X"
    done

    cd "${GOSRC}/"
    source "$SCRIPT_BASE/lib.sh"

    # Only testing-VMs need deps installed
    [[ -n "$PACKER_BUILDS" ]] || install_testing_dependencies  # must exist in $GOPATH
fi
