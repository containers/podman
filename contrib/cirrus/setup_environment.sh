#!/usr/bin/env bash

# This script is intended to be executed early by automation before
# performing other substantial operations.  It relies heavily on
# desired setup information being passed in environment variables
# from Cirrus-CI and/or other orchestration tooling.  To that end,
# VM's must always be considered single-purpose, single-use,
# disposable entities. i.e. One setup, one test, then always discarded.

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

# Make sure cni network plugins directory exists
mkdir -p /etc/cni/net.d

# Ensure that all lower-level contexts and child-processes have
# ready access to higher level orchestration (e.g Cirrus-CI)
# variables.
echo -e "\n# Begin single-use VM global variables (${BASH_SOURCE[0]})" \
    > "/etc/ci_environment"
(
    while read -r env_var_val; do
        echo "$env_var_val"
    done <<<"$(passthrough_envars)"
) >> "/etc/ci_environment"

# This is a possible manual maintenance gaff, check to be sure everything matches.
# shellcheck disable=SC2154
[[ "$DISTRO_NV" =~ $OS_REL_VER ]] || \
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
                # Need b/c using cri-o-runc package from OBS
                echo "OCI_RUNTIME=/usr/lib/cri-o-runc/sbin/runc" \
                    >> /etc/ci_environment
            else
                echo "OCI_RUNTIME=runc" >> /etc/ci_environment
            fi
        fi
        ;;
    cgroup2fs)
        if ((CONTAINER==0)); then
            # This is necessary since we've built/installed from source,
            # which uses runc as the default.
            warn "Forcing testing with crun instead of runc"
            echo "OCI_RUNTIME=crun" >> /etc/ci_environment
        fi
        ;;
    *) die_unknown CG_FS_TYPE
esac

if ((CONTAINER==0)); then  # Not yet running inside a container
    # Discovered reemergence of BFQ scheduler bug in kernel 5.8.12-200
    # which causes a kernel panic when system is under heavy I/O load.
    # Previously discovered in F32beta and confirmed fixed. It's been
    # observed in F31 kernels as well.  Deploy workaround for all VMs
    # to ensure a more stable I/O scheduler (elevator).
    echo "mq-deadline" > /sys/block/sda/queue/scheduler
    warn "I/O scheduler: $(cat /sys/block/sda/queue/scheduler)"
fi

# Which distribution are we testing on.
case "$OS_RELEASE_ID" in
    ubuntu) ;;
    fedora)
        if ((CONTAINER==0)); then
            msg "Configuring / Expanding host storage."
            # VM is setup to allow flexibility in testing alternate storage.
            # For general use, simply make use of all available space.
            bash "$SCRIPT_BASE/add_second_partition.sh"
            $SCRIPT_BASE/logcollector.sh df

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
        # The e2e tests wrongly guess `--cgroup-manager` option
        # shellcheck disable=SC2154
        if [[ "$CG_FS_TYPE" == "cgroup2fs" ]] || [[ "$PRIV_NAME" == "root" ]]
        then
            warn "Forcing CGROUP_MANAGER=systemd"
            echo "CGROUP_MANAGER=systemd" >> /etc/ci_environment
        else
            warn "Forcing CGROUP_MANAGER=cgroupfs"
            echo "CGROUP_MANAGER=cgroupfs" >> /etc/ci_environment
        fi
        ;;
    container)
        if ((CONTAINER==0)); then  # not yet inside a container
            warn "Force loading iptables modules"
            # Since CRIU 3.11, uses iptables to lock and unlock
            # the network during checkpoint and restore.  Needs
            # the following two modules loaded on the host.
            modprobe ip6table_nat || :
            modprobe iptable_nat || :
        else
            warn "Forcing CGROUP_MANAGER=cgroupfs"
            echo "CGROUP_MANAGER=cgroupfs" >> /etc/ci_environment

            # There's no practical way to detect userns w/in a container
            # affected/related tests are sensitive to this variable.
            warn "Disabling usernamespace integration testing"
            echo "SKIP_USERNS=1" >> /etc/ci_environment
        fi
        ;;
    *) die_unknown TEST_ENVIRON
esac

# Required to be defined by caller: Are we testing as root or a regular user
case "$PRIV_NAME" in
    root) ;;
    rootless)
        # Needs to exist for setup_rootless()
        ROOTLESS_USER="${ROOTLESS_USER:-some${RANDOM}dude}"
        echo "ROOTLESS_USER=$ROOTLESS_USER" >> /etc/ci_environment
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
    compose)
        dnf install -y $PACKAGE_DOWNLOAD_DIR/podman-docker*
        ;&  # continue with next item
    int) ;&
    sys) ;&
    upgrade_test) ;&
    bud) ;&
    bindings) ;&
    endpoint)
        # Use existing host bits when testing is to happen inside a container
        # since this script will run again in that environment.
        # shellcheck disable=SC2154
        if ((CONTAINER==0)) && [[ "$TEST_ENVIRON" == "host" ]]; then
            remove_packaged_podman_files
            make install PREFIX=/usr ETCDIR=/etc
        fi

        install_test_configs
        ;;
    swagger) ;&  # use next item
    consistency) make clean ;;
    release) ;;
    *) die_unknown TEST_FLAVOR
esac

# Must be the very last command.  Prevents setup from running twice.
echo 'SETUP_ENVIRONMENT=1' >> /etc/ci_environment
echo -e "\n# End of global variable definitions" \
    >> /etc/ci_environment

msg "Global CI Environment vars.:"
grep -Ev '^#' /etc/ci_environment | sort | indent
