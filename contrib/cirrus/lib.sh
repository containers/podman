

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
CONTINUOUS_INTEGRATION="${CONTINUOUS_INTEGRATION:-false}"
CIRRUS_REPO_NAME=${CIRRUS_REPO_NAME:-libpod}
CIRRUS_BASE_SHA=${CIRRUS_BASE_SHA:-unknown$(date +%s)}  # difficult to reliably discover
CIRRUS_BUILD_ID=${CIRRUS_BUILD_ID:-$RANDOM$(date +%s)}  # must be short and unique
# Vars. for image-building
PACKER_VER="1.3.5"
# CSV of cache-image names to build (see $PACKER_BASE/libpod_images.json)

# Base-images rarely change, define them here so they're out of the way.
PACKER_BUILDS="${PACKER_BUILDS:-ubuntu-18,fedora-30,fedora-29}"
# Google-maintained base-image names
UBUNTU_BASE_IMAGE="ubuntu-1804-bionic-v20181203a"
# Manually produced base-image names (see $SCRIPT_BASE/README.md)
FEDORA_BASE_IMAGE="fedora-cloud-base-30-1-2-1559164849"
PRIOR_FEDORA_BASE_IMAGE="fedora-cloud-base-29-1-2-1559164849"
BUILT_IMAGE_SUFFIX="${BUILT_IMAGE_SUFFIX:--$CIRRUS_REPO_NAME-${CIRRUS_BUILD_ID}}"
# IN_PODMAN container image
IN_PODMAN_IMAGE="quay.io/libpod/in_podman:latest"

# Safe env. vars. to transfer from root -> $ROOTLESS_USER  (go env handled separetly)
ROOTLESS_ENV_RE='(CIRRUS_.+)|(ROOTLESS_.+)|(.+_IMAGE.*)|(.+_BASE)|(.*DIRPATH)|(.*FILEPATH)|(SOURCE.*)|(DEPEND.*)|(.+_DEPS_.+)|(OS_REL.*)|(.+_ENV_RE)|(TRAVIS)|(CI.+)|(TEST_REMOTE.*)'
# Unsafe env. vars for display
SECRET_ENV_RE='(IRCID)|(ACCOUNT)|(^GC[EP]..+)|(SSH)'

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
    echo ""
    echo "##### $(go version) #####"
    echo ""
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
    req_env_var ROOTLESS_USER GOSRC

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
    sudo install -D -m 755 "/tmp/libpod/$SCRIPT_BASE/ooe.sh" /usr/local/bin/ooe.sh
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

install_cni_plugins() {
    echo "Installing CNI Plugins from commit $CNI_COMMIT"
    req_env_var GOPATH CNI_COMMIT
    DEST="$GOPATH/src/github.com/containernetworking/plugins"
    rm -rf "$DEST"
    ooe.sh git clone "https://github.com/containernetworking/plugins.git" "$DEST"
    cd "$DEST"
    ooe.sh git checkout -q "$CNI_COMMIT"
    ooe.sh ./build.sh
    sudo mkdir -p /usr/libexec/cni
    sudo cp bin/* /usr/libexec/cni
}

install_runc_from_git(){
    req_env_var GOPATH OS_RELEASE_ID RUNC_COMMIT
    wd=$(pwd)
    DEST="$GOPATH/src/github.com/opencontainers/runc"
    rm -rf "$DEST"
    ooe.sh git clone https://github.com/opencontainers/runc.git "$DEST"
    cd "$DEST"
    ooe.sh git fetch origin --tags
    ooe.sh git checkout -q "$RUNC_COMMIT"
    if [[ "${OS_RELEASE_ID}" == "ubuntu" ]]
    then
        ooe.sh make static BUILDTAGS="seccomp apparmor"
    else
        ooe.sh make BUILDTAGS="seccomp selinux"
    fi
    sudo install -m 755 runc /usr/bin/runc
    cd $wd
}

install_runc(){
    echo "Installing RunC from commit $RUNC_COMMIT"
    echo "Platform is $OS_RELEASE_ID"
    req_env_var GOPATH RUNC_COMMIT OS_RELEASE_ID
    if [[ "$OS_RELEASE_ID" =~ "ubuntu" ]]; then
        echo "Running make install.libseccomp.sudo for ubuntu"
        if ! [[ -d "/tmp/libpod" ]]
        then
            echo "Expecting a copy of libpod repository in /tmp/libpod"
            exit 5
        fi
        mkdir -p "$GOPATH/src/github.com/containers/"
        # Symlinks don't work with Go
        cp -a /tmp/libpod "$GOPATH/src/github.com/containers/"
        cd "$GOPATH/src/github.com/containers/libpod"
        ooe.sh sudo make install.libseccomp.sudo
    fi
    install_runc_from_git
}

install_buildah() {
    echo "Installing buildah from latest upstream master"
    req_env_var GOPATH
    DEST="$GOPATH/src/github.com/containers/buildah"
    rm -rf "$DEST"
    ooe.sh git clone https://github.com/containers/buildah "$DEST"
    cd "$DEST"
    ooe.sh make
    ooe.sh sudo make install
}

# Requires $GOPATH and $CONMON_COMMIT to be set
install_conmon(){
    echo "Installing conmon from commit $CONMON_COMMIT"
    req_env_var GOPATH CONMON_COMMIT
    DEST="$GOPATH/src/github.com/containers/conmon.git"
    rm -rf "$DEST"
    ooe.sh git clone https://github.com/containers/conmon.git "$DEST"
    cd "$DEST"
    ooe.sh git fetch origin --tags
    ooe.sh git checkout -q "$CONMON_COMMIT"
    ooe.sh make
    sudo install -D -m 755 bin/conmon /usr/libexec/podman/conmon
}

install_criu(){
    echo "Installing CRIU"
    echo "Installing CRIU from commit $CRIU_COMMIT"
    echo "Platform is $OS_RELEASE_ID"
    req_env_var CRIU_COMMIT

    if [[ "$OS_RELEASE_ID" =~ "ubuntu" ]]; then
        ooe.sh sudo -E add-apt-repository -y ppa:criu/ppa
        ooe.sh sudo -E apt-get -qq -y update
        ooe.sh sudo -E apt-get -qq -y install criu
    elif [[ "$OS_RELEASE_ID" =~ "fedora" ]]; then
        echo "Using CRIU from distribution"
    else
        DEST="/tmp/criu"
        rm -rf "$DEST"
        ooe.sh git clone https://github.com/checkpoint-restore/criu.git "$DEST"
        cd $DEST
        ooe.sh git fetch origin --tags
        ooe.sh git checkout -q "$CRIU_COMMIT"
        ooe.sh make
        sudo install -D -m 755  criu/criu /usr/sbin/
    fi
}

install_varlink() {
    echo "Installing varlink from the cheese-factory"
    ooe.sh sudo -H pip3 install varlink
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
    sync
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
    sudo rm -rf /var/cache/apt
    _finalize
}
