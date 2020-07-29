#!/usr/bin/env bash

set -e

# shellcheck source=./contrib/cirrus/lib.sh
source $(dirname $0)/lib.sh

die_unknown() {
    local var_name="$1"
    req_env_vars var_name
    local var_value="${!var_name}"
    die "Unknown/unsupported \$$var_name '$var_value'"
}

msg "************************************************************"
msg "Setting up runtime environment"
msg "************************************************************"
show_env_vars

req_env_vars USER HOME GOSRC SCRIPT_BASE TEST_FLAVOR TEST_ENVIRON \
             PODBIN_NAME PRIV_NAME DISTRO_NV

# Verify basic dependencies
for depbin in go rsync unzip sha256sum curl make python3 git
do
    if ! type -P "$depbin" &> /dev/null
    then
        warn "$depbin binary not found in $PATH"
    fi
done

# This is a possible manual maintenance gaff, check to be sure everything matches.
# shellcheck disable=SC2154
[[ "$DISTRO_NV" == "$OS_REL_VER" ]] || \
    die "Automation spec. '$DISTRO_NV'; actual host '$OS_REL_VER'"

# Only allow this script to execute once
if ((${SETUP_ENVIRONMENT:-0})); then
    # Comes from automation library
    # shellcheck disable=SC2154
    warn "Not executing $SCRIPT_FILENAME again"
    exit 0
fi

cd "${GOSRC}/"

# Defined by lib.sh: Does the host support cgroups v1 or v2
case "$CG_FS_TYPE" in
    tmpfs)
        if ((CONTAINER==0)); then
            warn "Forcing testing with runc instead of crun"
            if [[ "$OS_RELEASE_ID" == "ubuntu" ]]; then
                echo "export OCI_RUNTIME=/usr/lib/cri-o-runc/sbin/runc" >> /etc/environment
            else
                echo "export OCI_RUNTIME=runc" >> /etc/environment
            fi
        fi
        ;;
    cgroup2fs)
        if ((CONTAINER==0)); then
            # This is necessary since we've built/installed from source,
            # which uses runc as the default.
            warn "Forcing testing with crun instead of runc"
            echo "export OCI_RUNTIME=crun" >> /etc/environment
        fi
        ;;
    *) die_unknown CG_FS_TYPE
esac

# Which distribution are we testing on.
case "$OS_RELEASE_ID" in
    ubuntu*) ;;
    fedora*)
        if ((CONTAINER==0)); then  # Not yet running inside a container
            msg "Configuring / Expanding host storage."
            # VM is setup to allow flexibility in testing alternate storage.
            # For general use, simply make use of all available space.
            ooe.sh bash "$SCRIPT_BASE/add_second_partition.sh"

            # All SELinux distros need this for systemd-in-a-container
            msg "Enabling container_manage_cgroup"
            setsebool container_manage_cgroup true
        fi
        ;;
    *) die_unknown OS_RELEASE_ID
esac

# Required to be defined by caller: The environment where primary testing happens
# shellcheck disable=SC2154
case "$TEST_ENVIRON" in
    host)
        if [[ "$OS_RELEASE_ID" == "fedora" ]]; then
            # The e2e tests wrongly guess `--cgroup-manager cgroupfs`
            msg "Forcing CGROUP_MANAGER=systemd"
            _cgm="export CGROUP_MANAGER=systemd"
            echo "$_cgm" >> /etc/environment
            source /etc/environment
        fi
        ;;
    container)
        if ((CONTAINER==0)); then  # not yet inside a container
            msg "Force loading iptables modules"
            # Since CRIU 3.11, uses iptables to lock and unlock
            # the network during checkpoint and restore.  Needs
            # the following two modules loaded on the host.
            modprobe ip6table_nat || :
            modprobe iptable_nat || :
        else
            # The e2e tests wrongly guess `--cgroup-manager systemd`
            msg "Forcing CGROUP_MANAGER=cgroupfs"
            _cgm="export CGROUP_MANAGER=cgroupfs"
            echo "$_cgm" >> /etc/environment
            source /etc/environment
        fi
        ;;
    *) die_unknown TEST_ENVIRON
esac

# Required to be defined by caller: Are we testing as root or a regular user
# shellcheck disable=SC2154
case "$PRIV_NAME" in
    root)
        if [[ "$TEST_ENVIRON" == "container" ]] && ((container)); then
            # There's no practical way to detect userns w/in a container
            # affected/related tests are sensitive to this variable.
            _suns='export SKIP_USERNS=1'
            echo "$_suns" >> /etc/environment
            source /etc/environment
        fi
        ;;
    rootless)
        _ru="export ROOTLESS_USER='${ROOTLESS_USER:-some${RANDOM}dude}'"
        echo "$_ru" >> /etc/environment
        source /etc/environment
        setup_rootless
        ;;
    *) die_unknown PRIV_NAME
esac

# Required to be defined by caller: Are we testing podman or podman-remote client
# shellcheck disable=SC2154
case "$PODBIN_NAME" in
    podman) ;;
    remote) ;;
    *) die_unknown PODBIN_NAME
esac

# Required to be defined by caller: The primary type of testing that will be performed
# shellcheck disable=SC2154
case "$TEST_FLAVOR" in
    ext_svc) ;;
    smoke) ;&
    validate)
        # For some reason, this is also needed for validation
        make .install.pre-commit
        ;;
    automation) ;;
    altbuild)
        # Defined in .cirrus.yml
        # shellcheck disable=SC2154
        if [[ "$ALT_NAME" =~ RPM ]]; then
            bigto dnf install -y glibc-minimal-langpack rpm-build
        fi
        ;&
    docker-py) ;&
    build) make clean ;;
    unit) ;;
    apiv2) ;&  # use next item
    int) ;&
    sys) ;&
    bindings) ;&
    swagger) ;&
    endpoint)
        # Use existing host bits when testing is to happen inside a container
        # since this script will run again in that environment.
        # shellcheck disable=SC2154
        if ((CONTAINER==0)) && [[ "$TEST_ENVIRON" == "host" ]]; then
            remove_packaged_podman_files
            make install PREFIX=/usr ETCDIR=/etc
        fi
        ;;
    vendor) make clean ;;
    release) ;;
    *) die_unknown TEST_FLAVOR
esac

# Must be the very last command.  Establishes successful setup.
echo 'export SETUP_ENVIRONMENT=1' >> /etc/environment
