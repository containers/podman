

# Library of common, shared utility functions.  This file is intended
# to be sourced by other scripts, not called directly.

# BEGIN Global export of all variables
set -a

# Due to differences across platforms and runtime execution environments,
# handling of the (otherwise) default shell setup is non-uniform.  Rather
# than attempt to workaround differences, simply force-load/set required
# items every time this library is utilized.
USER="$(whoami)"
HOME="$(getent passwd $USER | cut -d : -f 6)"
# Some platforms set and make this read-only
[[ -n "$UID" ]] || \
    UID=$(getent passwd $USER | cut -d : -f 3)

# Automation library installed at image-build time,
# defining $AUTOMATION_LIB_PATH in this file.
if [[ -r "/etc/automation_environment" ]]; then
    source /etc/automation_environment
fi
# shellcheck disable=SC2154
if [[ -n "$AUTOMATION_LIB_PATH" ]]; then
        # shellcheck source=/usr/share/automation/lib/common_lib.sh
        source $AUTOMATION_LIB_PATH/common_lib.sh
else
    (
    echo "WARNING: It does not appear that containers/automation was installed."
    echo "         Functionality of most of this library will be negatively impacted"
    echo "         This ${BASH_SOURCE[0]} was loaded by ${BASH_SOURCE[1]}"
    ) > /dev/stderr
fi

# Managed by setup_environment.sh; holds task-specific definitions.
if [[ -r "/etc/ci_environment" ]]; then source /etc/ci_environment; fi

# This is normally set from .cirrus.yml but default is necessary when
# running under hack/get_ci_vm.sh since it cannot infer the value.
DISTRO_NV="${DISTRO_NV:-$OS_REL_VER}"

# Essential default paths, many are overridden when executing under Cirrus-CI
GOPATH="${GOPATH:-/var/tmp/go}"
if type -P go &> /dev/null
then
    # Cirrus-CI caches $GOPATH contents
    export GOCACHE="${GOCACHE:-$GOPATH/cache/go-build}"
    # called processes like `make` and other tools need these vars.
    eval "export $(go env)"

    # Ensure compiled tooling is reachable
    PATH="$PATH:$GOPATH/bin:$HOME/.local/bin"
fi
CIRRUS_WORKING_DIR="${CIRRUS_WORKING_DIR:-$(realpath $(dirname ${BASH_SOURCE[0]})/../../)}"
GOSRC="${GOSRC:-$CIRRUS_WORKING_DIR}"
PATH="$HOME/bin:/usr/local/bin:$PATH"
LD_LIBRARY_PATH="/usr/local/lib${LD_LIBRARY_PATH:+:${LD_LIBRARY_PATH}}"

# Saves typing / in case location ever moves
SCRIPT_BASE=${SCRIPT_BASE:-./contrib/cirrus}

# Downloaded, but not installed packages.
PACKAGE_DOWNLOAD_DIR=/var/cache/download

# Log remote-client system test server output here
PODMAN_SERVER_LOG=$CIRRUS_WORKING_DIR/podman-server.log

# Defaults when not running under CI
export CI="${CI:-false}"
CIRRUS_CI="${CIRRUS_CI:-false}"
CONTINUOUS_INTEGRATION="${CONTINUOUS_INTEGRATION:-false}"
CIRRUS_REPO_NAME=${CIRRUS_REPO_NAME:-podman}
# Cirrus only sets $CIRRUS_BASE_SHA properly for PRs, but $EPOCH_TEST_COMMIT
# needs to be set from this value in order for `make validate` to run properly.
# When running get_ci_vm.sh, most $CIRRUS_xyz variables are empty. Attempt
# to accommodate both branch and get_ci_vm.sh testing by discovering the base
# branch SHA value.
# shellcheck disable=SC2154
if [[ -z "$CIRRUS_BASE_SHA" ]] && [[ -z "$CIRRUS_TAG" ]]
then  # Operating on a branch, or under `get_ci_vm.sh`
    showrun echo "branch or get_ci_vm (CIRRUS_BASE_SHA and CIRRUS_TAG are unset)"
    CIRRUS_BASE_SHA=$(git rev-parse ${UPSTREAM_REMOTE:-origin}/$DEST_BRANCH)
elif [[ -z "$CIRRUS_BASE_SHA" ]]
then  # Operating on a tag
    showrun echo "operating on tag"
    CIRRUS_BASE_SHA=$(git rev-parse HEAD)
fi
# The starting place for linting and code validation
EPOCH_TEST_COMMIT="$CIRRUS_BASE_SHA"

# Regex defining all CI-related env. vars. necessary for all possible
# testing operations on all platforms and versions.  This is necessary
# to avoid needlessly passing through global/system values across
# contexts, such as host->container or root->rootless user
#
# List of envariables which must be EXACT matches
PASSTHROUGH_ENV_EXACT='CGROUP_MANAGER|DEST_BRANCH|DISTRO_NV|GOCACHE|GOPATH|GOSRC|NETWORK_BACKEND|OCI_RUNTIME|ROOTLESS_USER|SCRIPT_BASE|SKIP_USERNS|EC2_INST_TYPE|PODMAN_DB'

# List of envariable patterns which must match AT THE BEGINNING of the name.
PASSTHROUGH_ENV_ATSTART='CI|LANG|LC_|TEST'

# List of envariable patterns which can match ANYWHERE in the name
PASSTHROUGH_ENV_ANYWHERE='_NAME|_FQIN'

# Unsafe env. vars for display
SECRET_ENV_RE='ACCOUNT|GC[EP]..|SSH|PASSWORD|SECRET|TOKEN'

# Type of filesystem used for cgroups
CG_FS_TYPE="$(stat -f -c %T /sys/fs/cgroup)"

# Set to 1 in all podman container images
CONTAINER="${CONTAINER:-0}"

# END Global export of all variables
set +a

lilto() { err_retry 8 1000 "" "$@"; }  # just over 4 minutes max
bigto() { err_retry 7 5670 "" "$@"; }  # 12 minutes max

setup_rootless() {
    req_env_vars GOPATH GOSRC SECRET_ENV_RE

    ROOTLESS_USER="${ROOTLESS_USER:-some${RANDOM}dude}"
    ROOTLESS_UID=""

    local rootless_uid
    local rootless_gid
    local env_var_val
    local akfilepath
    local sshcmd

    # Only do this once; established by setup_environment.sh
    # shellcheck disable=SC2154
    if passwd --status $ROOTLESS_USER
    then
        if [[ $PRIV_NAME = "rootless" ]]; then
            msg "Updating $ROOTLESS_USER user permissions on possibly changed libpod code"
            chown -R $ROOTLESS_USER:$ROOTLESS_USER "$GOPATH" "$GOSRC"
            return 0
        fi
    fi
    msg "************************************************************"
    msg "Setting up rootless user '$ROOTLESS_USER'"
    msg "************************************************************"
    cd $GOSRC || exit 1
    # Guarantee independence from specific values
    rootless_uid=$((1500 + RANDOM % 5000))
    ROOTLESS_UID=$rootless_uid
    rootless_gid=$((1500 + RANDOM % 5000))
    msg "creating $rootless_uid:$rootless_gid $ROOTLESS_USER user"
    showrun groupadd -g $rootless_gid $ROOTLESS_USER
    showrun useradd -g $rootless_gid -u $rootless_uid --no-user-group --create-home $ROOTLESS_USER

    echo "$ROOTLESS_USER ALL=(root) NOPASSWD: ALL" > /etc/sudoers.d/ci-rootless

    mkdir -p "$HOME/.ssh" "/home/$ROOTLESS_USER/.ssh"

    msg "Creating ssh key pairs"
    [[ -r "$HOME/.ssh/id_rsa" ]] || \
        ssh-keygen -t rsa -P "" -f "$HOME/.ssh/id_rsa"
    showrun ssh-keygen -t ed25519 -P "" -f "/home/$ROOTLESS_USER/.ssh/id_ed25519"
    showrun ssh-keygen -t rsa -P "" -f "/home/$ROOTLESS_USER/.ssh/id_rsa"

    msg "Set up authorized_keys"
    cat $HOME/.ssh/*.pub /home/$ROOTLESS_USER/.ssh/*.pub >> $HOME/.ssh/authorized_keys
    cat $HOME/.ssh/*.pub /home/$ROOTLESS_USER/.ssh/*.pub >> /home/$ROOTLESS_USER/.ssh/authorized_keys

    msg "Configure ssh file permissions"
    chmod -R 700 "$HOME/.ssh"
    chmod -R 700 "/home/$ROOTLESS_USER/.ssh"
    chown -R $ROOTLESS_USER:$ROOTLESS_USER "/home/$ROOTLESS_USER/.ssh"

    # N/B: We're clobbering the known_hosts here on purpose.  There should
    # never be any non-localhost connections made from tests (using strict-mode).
    # If there are, it's either a security problem or a broken test, both of which
    # we want to lead to test failures.
    msg "   set up known_hosts for $USER"
    ssh-keyscan localhost > /root/.ssh/known_hosts
    msg "   set up known_hosts for $ROOTLESS_USER"
    # Maintain access-permission consistency with all other .ssh files.
    install -Z -m 700 -o $ROOTLESS_USER -g $ROOTLESS_USER \
        /root/.ssh/known_hosts /home/$ROOTLESS_USER/.ssh/known_hosts
}

install_test_configs() {
    msg "Installing ./test/registries.conf system-wide."
    install -v -D -m 644 ./test/registries.conf /etc/containers/
}

use_cni() {
    req_env_vars OS_RELEASE_ID PACKAGE_DOWNLOAD_DIR SCRIPT_BASE
    # Defined by common automation library
    # shellcheck disable=SC2154
    if [[ "$OS_RELEASE_ID" =~ "debian" ]]; then
        # Supporting it involves swapping the rpm & dnf commands below
        die "Testing debian w/ CNI networking currently not supported"
    fi

    msg "Forcing NETWORK_BACKEND=cni for all subsequent environments."
    echo "NETWORK_BACKEND=cni" >> /etc/ci_environment
    export NETWORK_BACKEND=cni
    # While it's possible a user may want both installed, for CNI CI testing
    # purposes we only care about backward-compatibility, not forward.
    # If both CNI & netavark are present, in some situations where --root
    # is used it's possible for podman to pick the "wrong" networking stack.
    msg "Force-removing netavark and aardvark-dns"
    # Other packages depend on nv/av, but we're testing with podman
    # binaries built from source, so it's safe to ignore these deps.
    #
    # Do not fail when netavark and aardvark-dns are not installed.
    for pkg in aardvark-dns netavark
    do
        [ -z "$(rpm -qa | grep $pkg)" ] && echo "$pkg not installed" || rpm -e --nodeps $pkg
    done
    msg "Installing default CNI configuration"
    showrun dnf install -y $PACKAGE_DOWNLOAD_DIR/podman-plugins*
    cd $GOSRC || exit 1
    rm -rvf /etc/cni/net.d
    mkdir -p /etc/cni/net.d
    showrun install -v -D -m 644 ./cni/87-podman-bridge.conflist \
        /etc/cni/net.d/
    # This config must always sort last in the list of networks (podman picks
    # first one as the default).  This config prevents allocation of network
    # address space used by default in google cloud.
    # https://cloud.google.com/vpc/docs/vpc#ip-ranges
    showrun install -v -D -m 644 $SCRIPT_BASE/99-do-not-use-google-subnets.conflist \
        /etc/cni/net.d/
}

use_netavark() {
    req_env_vars OS_RELEASE_ID PRIOR_FEDORA_NAME DISTRO_NV
    local magickind repokind
    msg "Unsetting NETWORK_BACKEND for all subsequent environments."
    echo "export -n NETWORK_BACKEND" >> /etc/ci_environment
    echo "unset NETWORK_BACKEND" >> /etc/ci_environment
    export -n NETWORK_BACKEND
    unset NETWORK_BACKEND
    msg "Removing any/all CNI configuration"
    showrun rm -rvf /etc/cni/net.d/*
    # N/B: The CNI packages are still installed and available. This is
    # on purpose, since CI needs to verify the selection mechanisms are
    # functional when both are available.
}

# Remove all files provided by the distro version of podman.
# All VM cache-images used for testing include the distro podman because (1) it's
# required for podman-in-podman testing and (2) it somewhat simplifies the task
# of pulling in necessary prerequisites packages as the set can change over time.
# For general CI testing however, calling this function makes sure the system
# can only run the compiled source version.
remove_packaged_podman_files() {
    echo "Removing packaged podman files to prevent conflicts with source build and testing."
    req_env_vars OS_RELEASE_ID

    # If any binaries are resident they could cause unexpected pollution
    for unit in podman.socket podman-auto-update.timer
    do
        for state in enabled active
        do
            if systemctl --quiet is-$state $unit
            then
                echo "Warning: $unit found $state prior to packaged-file removal"
                showrun systemctl --quiet disable $unit || true
                showrun systemctl --quiet stop $unit || true
            fi
        done
    done

    # OS_RELEASE_ID is defined by automation-library
    # shellcheck disable=SC2154
    if [[ "$OS_RELEASE_ID" =~ "debian" ]]
    then
        LISTING_CMD="dpkg-query -L podman"
    else
        LISTING_CMD="rpm -ql podman"
    fi

    # delete the podman socket in case it has been created previously
    rm -f $(podman info --format "{{.Host.RemoteSocket.Path}}")

    # yum/dnf/dpkg may list system directories, only remove files
    $LISTING_CMD | while read fullpath
    do
        # Sub-directories may contain unrelated/valuable stuff
        if [[ -d "$fullpath" ]]; then continue; fi
        showrun ooe.sh rm -vf "$fullpath"
    done

    # Be super extra sure and careful vs performant and completely safe
    sync && echo 3 > /proc/sys/vm/drop_caches || true
}

showrun echo "finished"
