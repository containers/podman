

# Library of common, shared utility functions.  This file is intended
# to be sourced by other scripts, not called directly.

# Global details persist here
source /etc/environment  # not always loaded under all circumstances

# Under some contexts these values are not set, make sure they are.
USER="$(whoami)"
HOME="$(getent passwd $USER | cut -d : -f 6)"
[[ -n "$UID" ]] || UID=$(getent passwd $USER | cut -d : -f 3)
GID=$(getent passwd $USER | cut -d : -f 4)

# Essential default paths, many are overriden when executing under Cirrus-CI
export GOPATH="${GOPATH:-/var/tmp/go}"
if type -P go &> /dev/null
then
    # required for go 1.12+
    export GOCACHE="${GOCACHE:-$HOME/.cache/go-build}"
    # called processes like `make` and other tools need these vars.
    eval "export $(go env)"

    # Ensure compiled tooling is reachable
    export PATH="$PATH:$GOPATH/bin"
fi
CIRRUS_WORKING_DIR="${CIRRUS_WORKING_DIR:-$GOPATH/src/github.com/containers/libpod}"
export GOSRC="${GOSRC:-$CIRRUS_WORKING_DIR}"
export PATH="$HOME/bin:$GOPATH/bin:/usr/local/bin:$PATH"
export LD_LIBRARY_PATH="/usr/local/lib${LD_LIBRARY_PATH:+:${LD_LIBRARY_PATH}}"
TIMESTAMPS_FILEPATH="${TIMESTAMPS_FILEPATH:-/var/tmp/timestamps}"
SETUP_MARKER_FILEPATH="${SETUP_MARKER_FILEPATH:-/var/tmp/.setup_environment_sh_complete}"
# Saves typing / in case location ever moves
SCRIPT_BASE=${SCRIPT_BASE:-./contrib/cirrus}
PACKER_BASE=${PACKER_BASE:-./contrib/cirrus/packer}

cd $GOSRC
if type -P git &> /dev/null
then
    CIRRUS_CHANGE_IN_REPO=${CIRRUS_CHANGE_IN_REPO:-$(git show-ref --hash=8 HEAD || date +%s)}
else # pick something unique and obviously not from Cirrus
    CIRRUS_CHANGE_IN_REPO=${CIRRUS_CHANGE_IN_REPO:-no_git_$(date +%s)}
fi

# Defaults when not running under CI
export CI="${CI:-false}"
CIRRUS_CI="${CIRRUS_CI:-false}"
DEST_BRANCH="${DEST_BRANCH:-master}"
CONTINUOUS_INTEGRATION="${CONTINUOUS_INTEGRATION:-false}"
CIRRUS_REPO_NAME=${CIRRUS_REPO_NAME:-libpod}
CIRRUS_BASE_SHA=${CIRRUS_BASE_SHA:-unknown$(date +%s)}  # difficult to reliably discover
CIRRUS_BUILD_ID=${CIRRUS_BUILD_ID:-$RANDOM$(date +%s)}  # must be short and unique
# Vars. for image-building
PACKER_VER="1.3.5"
# CSV of cache-image names to build (see $PACKER_BASE/libpod_images.json)

# Base-images rarely change, define them here so they're out of the way.
export PACKER_BUILDS="${PACKER_BUILDS:-ubuntu-18,fedora-30,fedora-29}"
# Google-maintained base-image names
export UBUNTU_BASE_IMAGE="ubuntu-1804-bionic-v20181203a"
# Manually produced base-image names (see $SCRIPT_BASE/README.md)
export FEDORA_BASE_IMAGE="fedora-cloud-base-30-1-2-1559164849"
export PRIOR_FEDORA_BASE_IMAGE="fedora-cloud-base-29-1-2-1559164849"
export BUILT_IMAGE_SUFFIX="${BUILT_IMAGE_SUFFIX:--$CIRRUS_REPO_NAME-${CIRRUS_BUILD_ID}}"
# IN_PODMAN container image
IN_PODMAN_IMAGE="quay.io/libpod/in_podman:latest"

# Avoid getting stuck waiting for user input
export DEBIAN_FRONTEND="noninteractive"
SUDOAPTGET="ooe.sh sudo -E apt-get -qq --yes"
SUDOAPTADD="ooe.sh sudo -E add-apt-repository --yes"
# Short-cuts for retrying/timeout calls
LILTO="timeout_attempt_delay_command 24s 5 30s"
BIGTO="timeout_attempt_delay_command 300s 5 30s"

# Safe env. vars. to transfer from root -> $ROOTLESS_USER  (go env handled separetly)
ROOTLESS_ENV_RE='(CIRRUS_.+)|(ROOTLESS_.+)|(.+_IMAGE.*)|(.+_BASE)|(.*DIRPATH)|(.*FILEPATH)|(SOURCE.*)|(DEPEND.*)|(.+_DEPS_.+)|(OS_REL.*)|(.+_ENV_RE)|(TRAVIS)|(CI.+)|(TEST_REMOTE.*)'
# Unsafe env. vars for display
SECRET_ENV_RE='(IRCID)|(ACCOUNT)|(^GC[EP]..+)|(SSH)'

# Names of systemd units which should never be running
EVIL_UNITS="cron crond atd apt-daily-upgrade apt-daily fstrim motd-news systemd-tmpfiles-clean"

SPECIALMODE="${SPECIALMODE:-none}"
TEST_REMOTE_CLIENT="${TEST_REMOTE_CLIENT:-false}"
export CONTAINER_RUNTIME=${CONTAINER_RUNTIME:-podman}

# When running as root, this may be empty or not, as a user, it MUST be set.
if [[ "$USER" == "root" ]]
then
    ROOTLESS_USER="${ROOTLESS_USER:-}"
else
    ROOTLESS_USER="${ROOTLESS_USER:-$USER}"
fi

# GCE image-name compatible string representation of distribution name
OS_RELEASE_ID="$(source /etc/os-release; echo $ID)"
# GCE image-name compatible string representation of distribution _major_ version
OS_RELEASE_VER="$(source /etc/os-release; echo $VERSION_ID | cut -d '.' -f 1)"
# Combined to ease soe usage
OS_REL_VER="${OS_RELEASE_ID}-${OS_RELEASE_VER}"

# Pass in a list of one or more envariable names; exit non-zero with
# helpful error message if any value is empty
req_env_var() {
    # Provide context. If invoked from function use its name; else script name
    local caller=${FUNCNAME[1]}
    if [[ -n "$caller" ]]; then
        # Indicate that it's a function name
        caller="$caller()"
    else
        # Not called from a function: use script name
        caller=$(basename $0)
    fi

    # Usage check
    [[ -n "$1" ]] || die 1 "FATAL: req_env_var: invoked without arguments"

    # Each input arg is an envariable name, e.g. HOME PATH etc. Expand each.
    # If any is empty, bail out and explain why.
    for i; do
        if [[ -z "${!i}" ]]; then
            die 9 "FATAL: $caller requires \$$i to be non-empty"
        fi
    done
}

item_test() {
    ITEM="$1"
    shift
    TEST_ARGS="$@"
    req_env_var ITEM TEST_ARGS

    if ERR=$(test "$@" 2>&1)
    then
        echo "ok $ITEM"
        return 0
    else
        RET=$?
        echo -n "not ok $ITEM: $TEST_ARGS"
        if [[ -z "$ERR" ]]
        then
            echo ""
        else  # test command itself failed
            echo -n ":"  # space follows :'s in $ERR
            echo "$ERR" | cut -d : -f 4-  # omit filename, line number, and command
        fi
        return $RET
    fi
}

show_env_vars() {
    echo "Showing selection of environment variable definitions:"
    _ENV_VAR_NAMES=$(awk 'BEGIN{for(v in ENVIRON) print v}' | \
        egrep -v "(^PATH$)|(^BASH_FUNC)|(^[[:punct:][:space:]]+)|$SECRET_ENV_RE" | \
        sort -u)
    for _env_var_name in $_ENV_VAR_NAMES
    do
        # Supports older BASH versions
        printf "    ${_env_var_name}=%q\n" "$(printenv $_env_var_name)"
    done
}

die() {
    echo "************************************************"
    echo ">>>>> ${2:-FATAL ERROR (but no message given!) in ${FUNCNAME[1]}()}"
    echo "************************************************"
    exit ${1:-1}
}

warn() {
    echo ">>>>> ${2:-WARNING (but no message given!) in ${FUNCNAME[1]}()}" > /dev/stderr
    echo ${1:-1} > /dev/stdout
}

bad_os_id_ver() {
    echo "Unknown/Unsupported distro. $OS_RELEASE_ID and/or version $OS_RELEASE_VER for $(basename $0)"
    exit 42
}

stub() {
    echo "STUB: Pretending to do $1"
}

timeout_attempt_delay_command() {
    TIMEOUT=$1
    ATTEMPTS=$2
    DELAY=$3
    shift 3
    STDOUTERR=$(mktemp -p '' $(basename $0)_XXXXX)
    req_env_var ATTEMPTS DELAY
    echo "Retrying $ATTEMPTS times with a $DELAY delay, and $TIMEOUT timeout for command: $@"
    for (( COUNT=1 ; COUNT <= $ATTEMPTS ; COUNT++ ))
    do
        echo "##### (attempt #$COUNT)" &>> "$STDOUTERR"
        if timeout --foreground $TIMEOUT "$@" &>> "$STDOUTERR"
        then
            echo "##### (success after #$COUNT attempts)" &>> "$STDOUTERR"
            break
        else
            echo "##### (failed with exit: $?)" &>> "$STDOUTERR"
            sleep $DELAY
        fi
    done
    cat "$STDOUTERR"
    rm -f "$STDOUTERR"
    if (( COUNT > $ATTEMPTS ))
    then
        echo "##### (exceeded $ATTEMPTS attempts)"
        exit 125
    fi
}

ircmsg() {
    req_env_var CIRRUS_TASK_ID IRCID
    [[ -n "$*" ]] || die 9 "ircmsg() invoked without message text argument"
    # Sometimes setup_environment.sh didn't run
    SCRIPT="$(dirname $0)/podbot.py"
    NICK="podbot_$CIRRUS_TASK_ID"
    NICK="${NICK:0:15}"  # Any longer will break things
    set +e
    $SCRIPT $NICK $@
    echo "Ignoring exit($?)"
    set -e
}

setup_rootless() {
    req_env_var ROOTLESS_USER GOSRC SECRET_ENV_RE ROOTLESS_ENV_RE

    # Only do this once
    if passwd --status $ROOTLESS_USER
    then
        echo "Updating $ROOTLESS_USER user permissions on possibly changed libpod code"
        chown -R $ROOTLESS_USER:$ROOTLESS_USER "$GOSRC"
        return 0
    fi

    cd $GOSRC
    # Guarantee independence from specific values
    ROOTLESS_UID=$[RANDOM+1000]
    ROOTLESS_GID=$[RANDOM+1000]
    echo "creating $ROOTLESS_UID:$ROOTLESS_GID $ROOTLESS_USER user"
    groupadd -g $ROOTLESS_GID $ROOTLESS_USER
    useradd -g $ROOTLESS_GID -u $ROOTLESS_UID --no-user-group --create-home $ROOTLESS_USER
    chown -R $ROOTLESS_USER:$ROOTLESS_USER "$GOSRC"

    echo "creating ssh keypair for $USER"
    [[ -r "$HOME/.ssh/id_rsa" ]] || \
        ssh-keygen -P "" -f "$HOME/.ssh/id_rsa"

    echo "Allowing ssh key for $ROOTLESS_USER"
    (umask 077 && mkdir "/home/$ROOTLESS_USER/.ssh")
    chown -R $ROOTLESS_USER:$ROOTLESS_USER "/home/$ROOTLESS_USER/.ssh"
    install -o $ROOTLESS_USER -g $ROOTLESS_USER -m 0600 \
        "$HOME/.ssh/id_rsa.pub" "/home/$ROOTLESS_USER/.ssh/authorized_keys"
    # Makes debugging easier
    cat /root/.ssh/authorized_keys >> "/home/$ROOTLESS_USER/.ssh/authorized_keys"

    echo "Configuring subuid and subgid"
    grep -q "${ROOTLESS_USER}" /etc/subuid || \
        echo "${ROOTLESS_USER}:$[ROOTLESS_UID * 100]:65536" | \
            tee -a /etc/subuid >> /etc/subgid

    # Env. vars set by Cirrus and setup_environment.sh must be explicitly
    # transfered to the test-user.
    echo "Configuring rootless user's environment variables:"
    echo "# Added by $GOSRC/$SCRIPT_PATH/lib.sh setup_rootless()"
    _ENV_VAR_NAMES=$(awk 'BEGIN{for(v in ENVIRON) print v}' | \
        egrep -v "(^PATH$)|(^BASH_FUNC)|(^[[:punct:][:space:]]+)|$SECRET_ENV_RE" | \
        egrep "$ROOTLESS_ENV_RE" | \
        sort -u)
    for _env_var_name in $_ENV_VAR_NAMES
    do
        # Works with older versions of bash
        printf "${_env_var_name}=%q\n" "$(printenv $_env_var_name)" >> "/home/$ROOTLESS_USER/.bashrc"
    done

    echo "Ensure the systems ssh process is up and running within 5 minutes"
    systemctl start sshd
    NOW=$(date +%s)
    TIMEOUT=$(date --date '+5 minutes' +%s)
    while [[ "$(date +%s)" -lt "$TIMEOUT" ]]
    do
        if timeout --foreground -k 1s 1s \
            ssh $ROOTLESS_USER@localhost \
            -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no -o CheckHostIP=no \
            true
        then
            break
        else
            sleep 2s
        fi
    done
    [[ "$(date +%s)" -lt "$TIMEOUT" ]] || \
        die 11 "Timeout exceeded waiting for localhost ssh capability"
}

# Helper/wrapper script to only show stderr/stdout on non-zero exit
install_ooe() {
    req_env_var SCRIPT_BASE
    echo "Installing script to mask stdout/stderr unless non-zero exit."
    sudo install -D -m 755 "$GOSRC/$SCRIPT_BASE/ooe.sh" /usr/local/bin/ooe.sh
}

# Grab a newer version of git from software collections
# https://www.softwarecollections.org/en/
# and use it with a wrapper
install_scl_git() {
    echo "Installing SoftwareCollections updated 'git' version."
    ooe.sh sudo yum -y install rh-git29
    cat << "EOF" | sudo tee /usr/bin/git
#!/bin/bash

scl enable rh-git29 -- git $@
EOF
    sudo chmod 755 /usr/bin/git
}

install_test_configs(){
    echo "Installing cni config, policy and registry config"
    req_env_var GOSRC
    sudo install -D -m 755 $GOSRC/cni/87-podman-bridge.conflist \
                           /etc/cni/net.d/87-podman-bridge.conflist
    sudo install -D -m 755 $GOSRC/test/policy.json \
                           /etc/containers/policy.json
    sudo install -D -m 755 $GOSRC/test/registries.conf \
                           /etc/containers/registries.conf
}

# Remove all files (except conmon, for now) provided by the distro version of podman.
# Except conmon, for now as it's expected to eventually be  packaged separately.
# All VM cache-images used for testing include the distro podman because (1) it's
# required for podman-in-podman testing and (2) it somewhat simplifies the task
# of pulling in necessary prerequisites packages as the set can change over time.
# For general CI testing however, calling this function makes sure the system
# can only run the compiled source version.
remove_packaged_podman_files(){
    echo "Removing packaged podman files to prevent conflicts with source build and testing."
    req_env_var OS_RELEASE_ID
    if [[ "$OS_RELEASE_ID" =~ "ubuntu" ]]
    then
        LISTING_CMD="sudo -E dpkg-query -L podman"
    else
        LISTING_CMD='sudo rpm -ql podman'
    fi

    # yum/dnf/dpkg may list system directories, only remove files
    $LISTING_CMD | while read fullpath
    do
        # TODO: This can go away when conmon gets it's own package
        if [[ -d "$fullpath" ]] || [[ $(basename "$fullpath") == "conmon" ]] ; then continue; fi
        ooe.sh sudo rm -vf "$fullpath"
    done
}

systemd_banish(){
    echo "Disabling periodic services that could destabilize testing:"
    set +e  # Not all of these exist on every platform
    for unit in $EVIL_UNITS
    do
        echo "Banishing $unit (ignoring errors)"
        (
            sudo systemctl stop $unit
            sudo systemctl disable $unit
            sudo systemctl disable $unit.timer
            sudo systemctl mask $unit
            sudo systemctl mask $unit.timer
        ) &> /dev/null
    done
    set -e
}

_finalize(){
    set +e  # Don't fail at the very end
    set +e  # make errors non-fatal
    echo "Removing leftover giblets from cloud-init"
    cd /
    sudo rm -rf /var/lib/cloud/instanc*
    sudo rm -rf /root/.ssh/*
    sudo rm -rf /home/*
    sudo rm -rf /tmp/*
    sudo rm -rf /tmp/.??*
    sudo sync
    sudo fstrim -av
}

rh_finalize(){
    set +e  # Don't fail at the very end
    # Allow root ssh-logins
    if [[ -r /etc/cloud/cloud.cfg ]]
    then
        sudo sed -re 's/^disable_root:.*/disable_root: 0/g' -i /etc/cloud/cloud.cfg
    fi
    echo "Resetting to fresh-state for usage as cloud-image."
    PKG=$(type -P dnf || type -P yum || echo "")
    sudo $PKG clean all
    sudo rm -rf /var/cache/{yum,dnf}
    sudo rm -f /etc/udev/rules.d/*-persistent-*.rules
    sudo touch /.unconfigured  # force firstboot to run
    _finalize
}

ubuntu_finalize(){
    set +e  # Don't fail at the very end
    echo "Resetting to fresh-state for usage as cloud-image."
    $LILTO $SUDOAPTGET autoremove
    sudo rm -rf /var/cache/apt
    _finalize
}
