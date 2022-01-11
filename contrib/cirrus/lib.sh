

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

OS_RELEASE_ID="$(source /etc/os-release; echo $ID)"
# GCE image-name compatible string representation of distribution _major_ version
OS_RELEASE_VER="$(source /etc/os-release; echo $VERSION_ID | tr -d '.')"
# Combined to ease some usage
OS_REL_VER="${OS_RELEASE_ID}-${OS_RELEASE_VER}"
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
PODMAN_SERVER_LOG=$CIRRUS_WORKING_DIR/server.log

# Defaults when not running under CI
export CI="${CI:-false}"
CIRRUS_CI="${CIRRUS_CI:-false}"
DEST_BRANCH="${DEST_BRANCH:-main}"
CONTINUOUS_INTEGRATION="${CONTINUOUS_INTEGRATION:-false}"
CIRRUS_REPO_NAME=${CIRRUS_REPO_NAME:-podman}
# Cirrus only sets $CIRRUS_BASE_SHA properly for PRs, but $EPOCH_TEST_COMMIT
# needs to be set from this value in order for `make validate` to run properly.
# When running get_ci_vm.sh, most $CIRRUS_xyz variables are empty. Attempt
# to accomidate both branch and get_ci_vm.sh testing by discovering the base
# branch SHA value.
# shellcheck disable=SC2154
if [[ -z "$CIRRUS_BASE_SHA" ]] && [[ -z "$CIRRUS_TAG" ]]
then  # Operating on a branch, or under `get_ci_vm.sh`
    CIRRUS_BASE_SHA=$(git rev-parse ${UPSTREAM_REMOTE:-origin}/$DEST_BRANCH)
elif [[ -z "$CIRRUS_BASE_SHA" ]]
then  # Operating on a tag
    CIRRUS_BASE_SHA=$(git rev-parse HEAD)
fi
# The starting place for linting and code validation
EPOCH_TEST_COMMIT="$CIRRUS_BASE_SHA"

# Regex defining all CI-related env. vars. necessary for all possible
# testing operations on all platforms and versions.  This is necessary
# to avoid needlessly passing through global/system values across
# contexts, such as host->container or root->rootless user
PASSTHROUGH_ENV_RE='(^CI.*)|(^CIRRUS)|(^DISTRO_NV)|(^GOPATH)|(^GOCACHE)|(^GOSRC)|(^SCRIPT_BASE)|(CGROUP_MANAGER)|(OCI_RUNTIME)|(^TEST.*)|(^PODBIN_NAME)|(^PRIV_NAME)|(^ALT_NAME)|(^ROOTLESS_USER)|(SKIP_USERNS)|(.*_NAME)|(.*_FQIN)'
# Unsafe env. vars for display
SECRET_ENV_RE='(ACCOUNT)|(GC[EP]..+)|(SSH)|(PASSWORD)|(TOKEN)'

# Type of filesystem used for cgroups
CG_FS_TYPE="$(stat -f -c %T /sys/fs/cgroup)"

# Set to 1 in all podman container images
CONTAINER="${CONTAINER:-0}"

# END Global export of all variables
set +a

lilto() { err_retry 8 1000 "" "$@"; }  # just over 4 minutes max
bigto() { err_retry 7 5670 "" "$@"; }  # 12 minutes max

# Print shell-escaped variable=value pairs, one per line, based on
# variable name matching a regex.  This is intended to catch
# variables being passed down from higher layers, like Cirrus-CI.
passthrough_envars(){
    local xchars
    local envname
    local envval
    # Avoid values containing entirely punctuation|control|whitespace
    xchars='[:punct:][:cntrl:][:space:]'
    warn "Will pass env. vars. matching the following regex:
    $PASSTHROUGH_ENV_RE"
    for envname in $(awk 'BEGIN{for(v in ENVIRON) print v}' | \
                         grep -Ev "SETUP_ENVIRONMENT" | \
                         grep -Ev "$SECRET_ENV_RE" | \
                         grep -E "$PASSTHROUGH_ENV_RE"); do

            envval="${!envname}"
            [[ -n $(tr -d "$xchars" <<<"$envval") ]] || continue

            # Properly escape values to prevent injection
            printf -- "$envname=%q\n" "$envval"
    done
}

setup_rootless() {
    req_env_vars GOPATH GOSRC SECRET_ENV_RE

    ROOTLESS_USER="${ROOTLESS_USER:-some${RANDOM}dude}"

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
    rootless_uid=$[RANDOM+1000]
    rootless_gid=$[RANDOM+1000]
    msg "creating $rootless_uid:$rootless_gid $ROOTLESS_USER user"
    groupadd -g $rootless_gid $ROOTLESS_USER
    useradd -g $rootless_gid -u $rootless_uid --no-user-group --create-home $ROOTLESS_USER

    # We also set up rootless user for image-scp tests (running as root)
    if [[ $PRIV_NAME = "rootless" ]]; then
        chown -R $ROOTLESS_USER:$ROOTLESS_USER "$GOPATH" "$GOSRC"
    fi
    echo "$ROOTLESS_USER ALL=(root) NOPASSWD: ALL" > /etc/sudoers.d/ci-rootless

    mkdir -p "$HOME/.ssh" "/home/$ROOTLESS_USER/.ssh"

    msg "Creating ssh key pairs"
    [[ -r "$HOME/.ssh/id_rsa" ]] || \
        ssh-keygen -t rsa -P "" -f "$HOME/.ssh/id_rsa"
    ssh-keygen -t ed25519 -P "" -f "/home/$ROOTLESS_USER/.ssh/id_ed25519"
    ssh-keygen -t rsa -P "" -f "/home/$ROOTLESS_USER/.ssh/id_rsa"

    msg "Setup authorized_keys"
    cat $HOME/.ssh/*.pub /home/$ROOTLESS_USER/.ssh/*.pub >> $HOME/.ssh/authorized_keys
    cat $HOME/.ssh/*.pub /home/$ROOTLESS_USER/.ssh/*.pub >> /home/$ROOTLESS_USER/.ssh/authorized_keys

    msg "Ensure the ssh daemon is up and running within 5 minutes"
    systemctl start sshd
    lilto systemctl is-active sshd

    msg "Configure ssh file permissions"
    chmod -R 700 "$HOME/.ssh"
    chmod -R 700 "/home/$ROOTLESS_USER/.ssh"
    chown -R $ROOTLESS_USER:$ROOTLESS_USER "/home/$ROOTLESS_USER/.ssh"

    msg "   setup known_hosts for $USER"
    ssh -q root@localhost \
        -o UserKnownHostsFile=/root/.ssh/known_hosts \
        -o UpdateHostKeys=yes \
        -o StrictHostKeyChecking=no \
        -o CheckHostIP=no \
        true

    msg "   setup known_hosts for $ROOTLESS_USER"
    su $ROOTLESS_USER -c "ssh -q $ROOTLESS_USER@localhost \
        -o UserKnownHostsFile=/home/$ROOTLESS_USER/.ssh/known_hosts \
        -o UpdateHostKeys=yes \
        -o StrictHostKeyChecking=no \
        -o CheckHostIP=no \
        true"
}

install_test_configs() {
    echo "Installing cni config, policy and registry config"
    req_env_vars GOSRC SCRIPT_BASE
    cd $GOSRC || exit 1
    install -v -D -m 644 ./cni/87-podman-bridge.conflist /etc/cni/net.d/
    # This config must always sort last in the list of networks (podman picks first one
    # as the default).  This config prevents allocation of network address space used
    # by default in google cloud.  https://cloud.google.com/vpc/docs/vpc#ip-ranges
    install -v -D -m 644 $SCRIPT_BASE/99-do-not-use-google-subnets.conflist /etc/cni/net.d/

    install -v -D -m 644 ./test/registries.conf /etc/containers/
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
    for unit in io.podman.service io.podman.socket
    do
        for state in enabled active
        do
            if systemctl --quiet is-$state $unit
            then
                echo "Warning: $unit found $state prior to packaged-file removal"
                systemctl --quiet disable $unit || true
                systemctl --quiet stop $unit || true
            fi
        done
    done

    if [[ "$OS_RELEASE_ID" =~ "ubuntu" ]]
    then
        LISTING_CMD="dpkg-query -L podman"
    else
        LISTING_CMD="rpm -ql podman"
    fi

    # yum/dnf/dpkg may list system directories, only remove files
    $LISTING_CMD | while read fullpath
    do
        # Sub-directories may contain unrelated/valuable stuff
        if [[ -d "$fullpath" ]]; then continue; fi
        ooe.sh rm -vf "$fullpath"
    done

    # Be super extra sure and careful vs performant and completely safe
    sync && echo 3 > /proc/sys/vm/drop_caches || true
}
