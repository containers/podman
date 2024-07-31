

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

# All CI jobs use a local registry
export CI_USE_REGISTRY_CACHE=true

# shellcheck disable=SC2154
if [[ -n "$CIRRUS_PR" ]] && [[ -z "$PR_BASE_SHA" ]]; then
    # shellcheck disable=SC2154
    PR_BASE_SHA=$(git merge-base ${DEST_BRANCH:-main} HEAD)
    export PR_BASE_SHA
fi

# The next three values define regular expressions matching env. vars. necessary
# for all possible testing contexts (rootless, container, etc.).  These values
# are consumed by the passthrough_envars() automation library function.
#
# List of envariables which must be EXACT matches
PASSTHROUGH_ENV_EXACT='CGROUP_MANAGER|DEST_BRANCH|DISTRO_NV|GOCACHE|GOPATH|GOSRC|NETWORK_BACKEND|OCI_RUNTIME|PR_BASE_SHA|ROOTLESS_USER|SCRIPT_BASE|SKIP_USERNS|EC2_INST_TYPE|PODMAN_DB|STORAGE_FS|PODMAN_BATS_LEAK_CHECK'

# List of envariable patterns which must match AT THE BEGINNING of the name.
# Consumed by the passthrough_envars() automation library function.
PASSTHROUGH_ENV_ATSTART='CI|LANG|LC_|STORAGE_OPTIONS_|TEST'

# List of envariable patterns which can match ANYWHERE in the name.
# Consumed by the passthrough_envars() automation library function.
PASSTHROUGH_ENV_ANYWHERE='_NAME|_FQIN'

# Unsafe env. vars for display
SECRET_ENV_RE='ACCOUNT|GC[EP]..|SSH|PASSWORD|SECRET|TOKEN'

# Type of filesystem used for cgroups
CG_FS_TYPE="$(stat -f -c %T /sys/fs/cgroup)"

# Set to 1 in all podman container images
CONTAINER="${CONTAINER:-0}"

# Without this, perl garbles "f39β" command-line args
PERL_UNICODE=A

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
        # Farm tests utilize the rootless user to simulate a "remote" podman instance.
    # Root still needs to own the repo. clone and all things under `$GOPATH`.  The
    # opposite is true for the lower-level podman e2e tests, the rootless user
    # runs them, and therefore needs permissions.
        if [[ $PRIV_NAME = "rootless" ]] && [[ "$TEST_FLAVOR" != "farm"  ]]; then
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

    # use tmpfs to speed up IO
    mount -t tmpfs -o size=75%,mode=0700,uid=$rootless_uid,gid=$rootless_gid none /home/$ROOTLESS_USER

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

    if [[ -n "$ROOTLESS_USER" ]]; then
        showrun echo "conditional setup for ROOTLESS_USER [=$ROOTLESS_USER]"
        # Make all future CI scripts aware of these values
        echo "ROOTLESS_USER=$ROOTLESS_USER" >> /etc/ci_environment
        echo "ROOTLESS_UID=$ROOTLESS_UID" >> /etc/ci_environment
    fi
}

install_test_configs() {
    # Which registries.conf to use. By default we always want the cached one...
    cached="-cached"
    # ...except for podman-machine, where it's antihelpful
    if [[ -n "$1" ]]; then
        if [[ "$1" = "nocache" ]]; then
            cached=""
        else
            die "Internal error: install_test_configs(): unknown arg '$*'"
        fi
    fi

    msg "Installing ./test/registries$cached.conf system-wide."
    # All CI VMs run with a local registry
    install -v -D -m 644 ./test/registries$cached.conf /etc/containers/registries.conf
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

    # delete the podman socket in case it has been created previously.
    # Do so without running podman, lest that invocation initialize unwanted state.
    rm -f /run/podman/podman.sock  /run/user/$(id -u)/podman/podman.sock || true

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
