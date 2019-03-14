

# Library of common, shared utility functions.  This file is intended
# to be sourced by other scripts, not called directly.

# Under some contexts these values are not set, make sure they are.
export USER="$(whoami)"
export HOME="$(getent passwd $USER | cut -d : -f 6)"

# These are normally set by cirrus, but can't be for VMs setup by hack/get_ci_vm.sh
# Pick some reasonable defaults
ENVLIB=${ENVLIB:-.bash_profile}
CIRRUS_WORKING_DIR="${CIRRUS_WORKING_DIR:-/var/tmp/go/src/github.com/containers/libpod}"
GOSRC="${GOSRC:-$CIRRUS_WORKING_DIR}"
SCRIPT_BASE=${SCRIPT_BASE:-./contrib/cirrus}
PACKER_BASE=${PACKER_BASE:-./contrib/cirrus/packer}
CIRRUS_BUILD_ID=${CIRRUS_BUILD_ID:-DEADBEEF}  # a human
CIRRUS_BASE_SHA=${CIRRUS_BASE_SHA:-HEAD}
CIRRUS_CHANGE_IN_REPO=${CIRRUS_CHANGE_IN_REPO:-FETCH_HEAD}
TIMESTAMPS_FILEPATH="${TIMESTAMPS_FILEPATH:-/var/tmp/timestamps}"

if ! [[ "$PATH" =~ "/usr/local/bin" ]]
then
    export PATH="$PATH:/usr/local/bin"
fi

# In ci/testing environment, ensure variables are always loaded
if [[ -r "$HOME/$ENVLIB" ]] && [[ -n "$CI" ]]
then
    # Make sure this is always loaded
    source "$HOME/$ENVLIB"
fi

# Pass in a line delimited list of, space delimited name/value pairs
# exit non-zero with helpful error message if any value is empty
req_env_var() {
    echo "$1" | while read NAME VALUE
    do
        if [[ -n "$NAME" ]] && [[ -z "$VALUE" ]]
        then
            echo "Required env. var. \$$NAME is not set"
            exit 9
        fi
    done
}

# Some env. vars may contain secrets.  Display values for known "safe"
# and useful variables.
# ref: https://cirrus-ci.org/guide/writing-tasks/#environment-variables
show_env_vars() {
    # This is almost always multi-line, print it separately
    echo "export CIRRUS_CHANGE_MESSAGE=$CIRRUS_CHANGE_MESSAGE"
    echo "
BUILDTAGS $BUILDTAGS
BUILT_IMAGE_SUFFIX $BUILT_IMAGE_SUFFIX
ROOTLESS_USER $ROOTLESS_USER
ROOTLESS_UID $ROOTLESS_UID
ROOTLESS_GID $ROOTLESS_GID
CI $CI
CIRRUS_CI $CIRRUS_CI
CI_NODE_INDEX $CI_NODE_INDEX
CI_NODE_TOTAL $CI_NODE_TOTAL
CONTINUOUS_INTEGRATION $CONTINUOUS_INTEGRATION
CIRRUS_BASE_BRANCH $CIRRUS_BASE_BRANCH
CIRRUS_BASE_SHA $CIRRUS_BASE_SHA
CIRRUS_BRANCH $CIRRUS_BRANCH
CIRRUS_BUILD_ID $CIRRUS_BUILD_ID
CIRRUS_CHANGE_IN_REPO $CIRRUS_CHANGE_IN_REPO
CIRRUS_CLONE_DEPTH $CIRRUS_CLONE_DEPTH
CIRRUS_DEFAULT_BRANCH $CIRRUS_DEFAULT_BRANCH
CIRRUS_PR $CIRRUS_PR
CIRRUS_TAG $CIRRUS_TAG
CIRRUS_OS $CIRRUS_OS
OS $OS
CIRRUS_TASK_NAME $CIRRUS_TASK_NAME
CIRRUS_TASK_ID $CIRRUS_TASK_ID
CIRRUS_REPO_NAME $CIRRUS_REPO_NAME
CIRRUS_REPO_OWNER $CIRRUS_REPO_OWNER
CIRRUS_REPO_FULL_NAME $CIRRUS_REPO_FULL_NAME
CIRRUS_REPO_CLONE_URL $CIRRUS_REPO_CLONE_URL
CIRRUS_SHELL $CIRRUS_SHELL
CIRRUS_USER_COLLABORATOR $CIRRUS_USER_COLLABORATOR
CIRRUS_USER_PERMISSION $CIRRUS_USER_PERMISSION
CIRRUS_WORKING_DIR $CIRRUS_WORKING_DIR
CIRRUS_HTTP_CACHE_HOST $CIRRUS_HTTP_CACHE_HOST
$(go env)
PACKER_BUILDS $PACKER_BUILDS
    " | while read NAME VALUE
    do
        [[ -z "$NAME" ]] || echo "export $NAME=\"$VALUE\""
    done
    echo ""
    echo "##### $(go version) #####"
    echo ""
}

# Unset environment variables not needed for testing purposes
clean_env() {
    req_env_var "
        UNSET_ENV_VARS $UNSET_ENV_VARS
    "
    echo "Unsetting $(echo $UNSET_ENV_VARS | wc -w) environment variables"
    unset -v UNSET_ENV_VARS $UNSET_ENV_VARS || true  # don't fail on read-only
}

die() {
    req_env_var "
        1 $1
        2 $2
    "
    echo "$2"
    exit $1
}

# Return a GCE image-name compatible string representation of distribution name
os_release_id() {
    eval "$(egrep -m 1 '^ID=' /etc/os-release | tr -d \' | tr -d \")"
    echo "$ID"
}

# Return a GCE image-name compatible string representation of distribution major version
os_release_ver() {
    eval "$(egrep -m 1 '^VERSION_ID=' /etc/os-release | tr -d \' | tr -d \")"
    echo "$VERSION_ID" | cut -d '.' -f 1
}

bad_os_id_ver() {
    echo "Unknown/Unsupported distro. $OS_RELEASE_ID and/or version $OS_RELEASE_VER for $ARGS"
    exit 42
}

run_rootless() {
    if [[ -z "$ROOTLESS_USER" ]] && [[ -z "$ROOTLESS_UID" ]] && [[ -z "$ROOTLESS_GID" ]]
    then
        return 1
    else
        return 0
    fi
}

stub() {
    echo "STUB: Pretending to do $1"
}

ircmsg() {
    req_env_var "
        CIRRUS_TASK_ID $CIRRUS_TASK_ID
        @ $@
    "
    # Sometimes setup_environment.sh didn't run
    SCRIPT="$(dirname $0)/podbot.py"
    NICK="podbot_$CIRRUS_TASK_ID"
    NICK="${NICK:0:15}"  # Any longer will break things
    set +e
    $SCRIPT $NICK $@
    echo "Ignoring exit($?)"
    set -e
}

record_timestamp() {
    set +x  # sometimes it's turned on
    req_env_var "TIMESTAMPS_FILEPATH $TIMESTAMPS_FILEPATH"
    echo "."  # cirrus webui strips blank-lines
    STAMPMSG="The $1 time at the tone will be:"
    echo -e "$STAMPMSG\t$(date --iso-8601=seconds)" | \
        tee -a $TIMESTAMPS_FILEPATH
    echo -e "BLEEEEEEEEEEP!\n."
}

setup_rootless() {
    req_env_var "
        ROOTLESS_USER $ROOTLESS_USER
        ROOTLESS_UID $ROOTLESS_UID
        ROOTLESS_GID $ROOTLESS_GID
        GOSRC $GOSRC
        ENVLIB $ENVLIB
    "
    echo "creating $ROOTLESS_UID:$ROOTLESS_GID $ROOTLESS_USER user"
    groupadd -g $ROOTLESS_GID $ROOTLESS_USER
    useradd -g $ROOTLESS_GID -u $ROOTLESS_UID --no-user-group --create-home $ROOTLESS_USER
    chown -R $ROOTLESS_UID:$ROOTLESS_GID "$GOSRC"

    echo "creating ssh keypair for $USER"
    ssh-keygen -P "" -f $HOME/.ssh/id_rsa

    echo "Allowing ssh key for $ROOTLESS_USER"
    (umask 077 && mkdir "/home/$ROOTLESS_USER/.ssh")
    chown -R $ROOTLESS_UID:$ROOTLESS_GID "/home/$ROOTLESS_USER/.ssh"
    install -o $ROOTLESS_UID -g $ROOTLESS_GID -m 0600 \
        "$HOME/.ssh/id_rsa.pub" "/home/$ROOTLESS_USER/.ssh/authorized_keys"

    echo "Setting permissions on automation files"
    chmod 666 "$TIMESTAMPS_FILEPATH"

    echo "Copying $HOME/$ENVLIB"
    install -o $ROOTLESS_UID -g $ROOTLESS_GID -m 0700 \
        "$HOME/$ENVLIB" "/home/$ROOTLESS_USER/$ENVLIB"

    echo "Configuring user's go environment variables"
    su --login --command 'go env' $ROOTLESS_USER | \
        while read envline
        do
            X=$(echo "export $envline" | tee -a "/home/$ROOTLESS_USER/$ENVLIB") && echo "$X"
        done
}

# Helper/wrapper script to only show stderr/stdout on non-zero exit
install_ooe() {
    req_env_var "SCRIPT_BASE $SCRIPT_BASE"
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
    req_env_var "
        GOPATH $GOPATH
        CNI_COMMIT $CNI_COMMIT
    "
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
    wd=$(pwd)
    DEST="$GOPATH/src/github.com/opencontainers/runc"
    rm -rf "$DEST"
    ooe.sh git clone https://github.com/opencontainers/runc.git "$DEST"
    cd "$DEST"
    ooe.sh git fetch origin --tags
    ooe.sh git checkout -q "$RUNC_COMMIT"
    ooe.sh make static BUILDTAGS="seccomp apparmor selinux"
    sudo install -m 755 runc /usr/bin/runc
    cd $wd
}

install_runc(){
    OS_RELEASE_ID=$(os_release_id)
    echo "Installing RunC from commit $RUNC_COMMIT"
    echo "Platform is $OS_RELEASE_ID"
    req_env_var "
        GOPATH $GOPATH
        RUNC_COMMIT $RUNC_COMMIT
        OS_RELEASE_ID $OS_RELEASE_ID
    "
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
    req_env_var "GOPATH $GOPATH"
    DEST="$GOPATH/src/github.com/containers/buildah"
    rm -rf "$DEST"
    ooe.sh git clone https://github.com/containers/buildah "$DEST"
    cd "$DEST"
    ooe.sh make
    ooe.sh sudo make install
}

# Requires $GOPATH and $CRIO_COMMIT to be set
install_conmon(){
    echo "Installing conmon from commit $CRIO_COMMIT"
    req_env_var "
        GOPATH $GOPATH
        CRIO_COMMIT $CRIO_COMMIT
    "
    DEST="$GOPATH/src/github.com/kubernetes-sigs/cri-o.git"
    rm -rf "$DEST"
    ooe.sh git clone https://github.com/kubernetes-sigs/cri-o.git "$DEST"
    cd "$DEST"
    ooe.sh git fetch origin --tags
    ooe.sh git checkout -q "$CRIO_COMMIT"
    ooe.sh make
    sudo install -D -m 755 bin/conmon /usr/libexec/podman/conmon
}

install_criu(){
    OS_RELEASE_ID=$(os_release_id)
    OS_RELEASE_VER=$(os_release_ver)
    echo "Installing CRIU"
    echo "Installing CRIU from commit $CRIU_COMMIT"
    echo "Platform is $OS_RELEASE_ID"
    req_env_var "
    CRIU_COMMIT $CRIU_COMMIT
    "

    if [[ "$OS_RELEASE_ID" =~ "ubuntu" ]]; then
        ooe.sh sudo -E add-apt-repository -y ppa:criu/ppa
        ooe.sh sudo -E apt-get -qq -y update
        ooe.sh sudo -E apt-get -qq -y install criu
    elif [[ ( "$OS_RELEASE_ID" =~ "centos" || "$OS_RELEASE_ID" =~ "rhel" ) && "$OS_RELEASE_VER" =~ "7"* ]]; then
        echo "Configuring Repositories for latest CRIU"
        ooe.sh sudo tee /etc/yum.repos.d/adrian-criu-el7.repo <<EOF
[adrian-criu-el7]
name=Copr repo for criu-el7 owned by adrian
baseurl=https://copr-be.cloud.fedoraproject.org/results/adrian/criu-el7/epel-7-$basearch/
type=rpm-md
skip_if_unavailable=True
gpgcheck=1
gpgkey=https://copr-be.cloud.fedoraproject.org/results/adrian/criu-el7/pubkey.gpg
repo_gpgcheck=0
enabled=1
enabled_metadata=1
EOF
        ooe.sh sudo yum -y install criu
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

install_packer_copied_files(){
    # Install cni config, policy and registry config
    sudo install -D -m 755 /tmp/libpod/cni/87-podman-bridge.conflist \
                           /etc/cni/net.d/87-podman-bridge.conflist
    sudo install -D -m 755 /tmp/libpod/test/policy.json \
                           /etc/containers/policy.json
    sudo install -D -m 755 /tmp/libpod/test/redhat_sigstore.yaml \
                           /etc/containers/registries.d/registry.access.redhat.com.yaml
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
    sudo rm -rf /var/lib/cloud/instance?
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
    [[ -z "$PKG" ]] || sudo $PKG clean all  # not on atomic
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

rhel_exit_handler() {
    set +ex
    req_env_var "
        GOPATH $GOPATH
        RHSMCMD $RHSMCMD
    "
    cd /
    sudo rm -rf "$RHSMCMD"
    sudo rm -rf "$GOPATH"
    sudo subscription-manager remove --all
    sudo subscription-manager unregister
    sudo subscription-manager clean
}

rhsm_enable() {
    req_env_var "
        RHSM_COMMAND $RHSM_COMMAND
    "
    export GOPATH="$(mktemp -d)"
    export RHSMCMD="$(mktemp)"
    trap "rhel_exit_handler" EXIT
    # Avoid logging sensitive details
    echo "$RHSM_COMMAND" > "$RHSMCMD"
    ooe.sh sudo bash "$RHSMCMD"
    sudo rm -rf "$RHSMCMD"
}
