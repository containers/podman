

# Library of common, shared utility functions.  This file is intended
# to be sourced by other scripts, not called directly.

# Global details persist here
source /etc/environment  # not always loaded under all circumstances

# Automation environment doesn't automatically load for Ubuntu 18
if [[ -r '/usr/share/automation/environment' ]]; then
    source '/usr/share/automation/environment'
fi

# Under some contexts these values are not set, make sure they are.
export USER="$(whoami)"
export HOME="$(getent passwd $USER | cut -d : -f 6)"
[[ -n "$UID" ]] || export UID=$(getent passwd $USER | cut -d : -f 3)
export GID=$(getent passwd $USER | cut -d : -f 4)

# Essential default paths, many are overridden when executing under Cirrus-CI
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
# Saves typing / in case location ever moves
SCRIPT_BASE=${SCRIPT_BASE:-./contrib/cirrus}
PACKER_BASE=${PACKER_BASE:-./contrib/cirrus/packer}
# Important filepaths
SETUP_MARKER_FILEPATH="${SETUP_MARKER_FILEPATH:-/var/tmp/.setup_environment_sh_complete}"
AUTHOR_NICKS_FILEPATH="${CIRRUS_WORKING_DIR}/${SCRIPT_BASE}/git_authors_to_irc_nicks.csv"

# Log remote-client system test varlink output here
export VARLINK_LOG=/var/tmp/varlink.log

cd $GOSRC
if type -P git &> /dev/null && [[ -d "$GOSRC/.git" ]]
then
    CIRRUS_CHANGE_IN_REPO=${CIRRUS_CHANGE_IN_REPO:-$(git show-ref --hash=8 HEAD || date +%s)}
else # pick something unique and obviously not from Cirrus
    CIRRUS_CHANGE_IN_REPO=${CIRRUS_CHANGE_IN_REPO:-unknown_$(date +%s)}
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
PACKER_VER="1.4.2"
# CSV of cache-image names to build (see $PACKER_BASE/libpod_images.json)

# List of cache imaes to build for 'CI:IMG' mode via build_vm_images.sh
# Exists to support manual single-image building in case of emergency
export PACKER_BUILDS="${PACKER_BUILDS:-ubuntu-20,ubuntu-19,fedora-32,fedora-31}"
# Google cloud provides these, we just make copies (see $SCRIPT_BASE/README.md) for use
export UBUNTU_BASE_IMAGE="ubuntu-2004-focal-v20200506"
export PRIOR_UBUNTU_BASE_IMAGE="ubuntu-1910-eoan-v20200211"
# Manually produced base-image names (see $SCRIPT_BASE/README.md)
export FEDORA_BASE_IMAGE="fedora-cloud-base-32-1-6-1588257430"
export PRIOR_FEDORA_BASE_IMAGE="fedora-cloud-base-31-1-9-1588257430"
export BUILT_IMAGE_SUFFIX="${BUILT_IMAGE_SUFFIX:--$CIRRUS_REPO_NAME-${CIRRUS_BUILD_ID}}"
# IN_PODMAN container image
IN_PODMAN_IMAGE="quay.io/libpod/in_podman:$DEST_BRANCH"
# Image for uploading releases
UPLDREL_IMAGE="quay.io/libpod/upldrel:master"

# This is needed under some environments/contexts
SUDO=''
[[ "$UID" -eq 0 ]] || \
    SUDO='sudo -E'

# Avoid getting stuck waiting for user input
export DEBIAN_FRONTEND="noninteractive"
SUDOAPTGET="$SUDO apt-get -qq --yes"
SUDOAPTADD="$SUDO add-apt-repository --yes"
# Regex that finds enabled periodic apt configuration items
PERIODIC_APT_RE='^(APT::Periodic::.+")1"\;'
# Short-cuts for retrying/timeout calls
LILTO="timeout_attempt_delay_command 120s 5 30s"
BIGTO="timeout_attempt_delay_command 300s 5 60s"

# Safe env. vars. to transfer from root -> $ROOTLESS_USER  (go env handled separately)
ROOTLESS_ENV_RE='(CIRRUS_.+)|(ROOTLESS_.+)|(.+_IMAGE.*)|(.+_BASE)|(.*DIRPATH)|(.*FILEPATH)|(SOURCE.*)|(DEPEND.*)|(.+_DEPS_.+)|(OS_REL.*)|(.+_ENV_RE)|(TRAVIS)|(CI.+)|(TEST_REMOTE.*)'
# Unsafe env. vars for display
SECRET_ENV_RE='(IRCID)|(ACCOUNT)|(GC[EP]..+)|(SSH)'

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
# Type of filesystem used for cgroups
CG_FS_TYPE="$(stat -f -c %T /sys/fs/cgroup)"

# When building images, the version of automation tooling to install
INSTALL_AUTOMATION_VERSION=1.1.3

# Installed into cache-images, supports overrides
# by user-data in case of breakage or for debugging.
CUSTOM_CLOUD_CONFIG_DEFAULTS="$GOSRC/$PACKER_BASE/cloud-init/$OS_RELEASE_ID/cloud.cfg.d"
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
    echo ">>>>> ${1:-WARNING (but no message given!) in ${FUNCNAME[1]}()}" > /dev/stderr
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

# This covers all possible human & CI workflow parallel & serial combinations
# where at least one caller must definitively discover if within a commit range
# there is at least one release tag not having any '-' characters (return 0)
# or otherwise (return non-0).
is_release() {
    unset RELVER
    local ret
    req_env_var CIRRUS_CHANGE_IN_REPO
    if [[ -n "$CIRRUS_TAG" ]]; then
        RELVER="$CIRRUS_TAG"
    elif [[ ! "$CIRRUS_BASE_SHA" =~ "unknown" ]]
    then
        # Normally not possible for this to be empty, except when unittesting.
        req_env_var CIRRUS_BASE_SHA
        local range="${CIRRUS_BASE_SHA}..${CIRRUS_CHANGE_IN_REPO}"
        if echo "${range}$CIRRUS_TAG" | grep -iq 'unknown'; then
            die 11 "is_release() unusable range ${range} or tag $CIRRUS_TAG"
        fi

        if type -P git &> /dev/null
        then
            git fetch --all --tags &> /dev/null|| \
                die 12 "is_release() failed to fetch tags"
            RELVER=$(git log --pretty='format:%d' $range | \
                     grep '(tag:' | sed -r -e 's/\s+[(]tag:\s+(v[0-9].*)[)]/\1/' | \
                     sort -uV | tail -1)
            ret=$?
        else
            warn -1 "Git command not found while checking for release"
            ret="-1"
        fi
        [[ "$ret" -eq "0" ]] || \
            die 13 "is_release() failed to parse tags"
    else  # Not testing a PR, but neither CIRRUS_BASE_SHA or CIRRUS_TAG are set
        return 1
    fi
    if [[ -n "$RELVER" ]]; then
        echo "Found \$RELVER $RELVER"
        if echo "$RELVER" | grep -q '-'; then
            return 2  # development tag
        else
            return 0
        fi
    else
        return 1  # not a release
    fi
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
    # transferred to the test-user.
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

# Grab a newer version of git from software collections
# https://www.softwarecollections.org/en/
# and use it with a wrapper
install_scl_git() {
    echo "Installing SoftwareCollections updated 'git' version."
    ooe.sh $SUDO yum -y install rh-git29
    cat << "EOF" | $SUDO tee /usr/bin/git
#!/bin/bash

scl enable rh-git29 -- git $@
EOF
    $SUDO chmod 755 /usr/bin/git
}

install_test_configs() {
    echo "Installing cni config, policy and registry config"
    req_env_var GOSRC SCRIPT_BASE
    cd $GOSRC
    install -v -D -m 644 ./cni/87-podman-bridge.conflist /etc/cni/net.d/
    # This config must always sort last in the list of networks (podman picks first one
    # as the default).  This config prevents allocation of network address space used
    # by default in google cloud.  https://cloud.google.com/vpc/docs/vpc#ip-ranges
    install -v -D -m 644 $SCRIPT_BASE/99-do-not-use-google-subnets.conflist /etc/cni/net.d/
    install -v -D -m 644 ./test/policy.json /etc/containers/
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
    req_env_var OS_RELEASE_ID

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
        LISTING_CMD="$SUDO dpkg-query -L podman"
    else
        LISTING_CMD='$SUDO rpm -ql podman'
    fi

    # yum/dnf/dpkg may list system directories, only remove files
    $LISTING_CMD | while read fullpath
    do
        # Sub-directories may contain unrelated/valuable stuff
        if [[ -d "$fullpath" ]]; then continue; fi
        ooe.sh $SUDO rm -vf "$fullpath"
    done

    # Be super extra sure and careful vs performant and completely safe
    sync && echo 3 > /proc/sys/vm/drop_caches
}

canonicalize_image_names() {
    req_env_var IMGNAMES
    echo "Adding all current base images to \$IMGNAMES for timestamp update"
    export IMGNAMES="\
$IMGNAMES
$UBUNTU_BASE_IMAGE
$PRIOR_UBUNTU_BASE_IMAGE
$FEDORA_BASE_IMAGE
$PRIOR_FEDORA_BASE_IMAGE
"
}

systemd_banish() {
    $GOSRC/$PACKER_BASE/systemd_banish.sh
}

# This can be removed when the kernel bug fix is included in Fedora
workaround_bfq_bug() {
    if [[ "$OS_RELEASE_ID" == "fedora" ]] && [[ $OS_RELEASE_VER -le 32 ]]; then
        warn "Switching io scheduler to 'deadline' to avoid RHBZ 1767539"
        warn "aka https://bugzilla.kernel.org/show_bug.cgi?id=205447"
        echo "mq-deadline" | sudo tee /sys/block/sda/queue/scheduler > /dev/null
        echo -n "IO Scheduler set to: "
        $SUDO cat /sys/block/sda/queue/scheduler
    fi
}

# Warning: DO NOT USE.
# This is called by other functions as the very last step during the VM Image build
# process.  It's purpose is to "reset" the image, so all the first-boot operations
# happen at test runtime (like generating new ssh host keys, resizing partitions, etc.)
_finalize() {
    set +e  # Don't fail at the very end
    if [[ -d "$CUSTOM_CLOUD_CONFIG_DEFAULTS" ]]
    then
        echo "Installing custom cloud-init defaults"
        $SUDO cp -v "$CUSTOM_CLOUD_CONFIG_DEFAULTS"/* /etc/cloud/cloud.cfg.d/
    else
        echo "Could not find any files in $CUSTOM_CLOUD_CONFIG_DEFAULTS"
    fi
    echo "Re-initializing so next boot does 'first-boot' setup again."
    cd /
    $SUDO rm -rf /var/lib/cloud/instanc*
    $SUDO rm -rf /root/.ssh/*
    $SUDO rm -rf /etc/ssh/*key*
    $SUDO rm -rf /etc/ssh/moduli
    $SUDO rm -rf /home/*
    $SUDO rm -rf /tmp/*
    $SUDO rm -rf /tmp/.??*
    $SUDO sync
    $SUDO fstrim -av
}

# Called during VM Image setup, not intended for general use.
rh_finalize() {
    set +e  # Don't fail at the very end
    echo "Resetting to fresh-state for usage as cloud-image."
    PKG=$(type -P dnf || type -P yum || echo "")
    $SUDO $PKG clean all
    $SUDO rm -rf /var/cache/{yum,dnf}
    $SUDO rm -f /etc/udev/rules.d/*-persistent-*.rules
    $SUDO touch /.unconfigured  # force firstboot to run
    _finalize
}

# Called during VM Image setup, not intended for general use.
ubuntu_finalize() {
    set +e  # Don't fail at the very end
    echo "Resetting to fresh-state for usage as cloud-image."
    $LILTO $SUDOAPTGET autoremove
    $SUDO rm -rf /var/cache/apt
    _finalize
}
