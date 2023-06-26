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
             PODBIN_NAME PRIV_NAME DISTRO_NV DEST_BRANCH

# Verify basic dependencies
for depbin in go rsync unzip sha256sum curl make python3 git
do
    if ! type -P "$depbin" &> /dev/null
    then
        warn "$depbin binary not found in $PATH"
    fi
done

cp hack/podman-registry /bin

# Some test operations & checks require a git "identity"
_gc='git config --file /root/.gitconfig'
$_gc user.email "TMcTestFace@example.com"
$_gc user.name "Testy McTestface"
# Bypass git safety/security checks when operating in a throwaway environment
git config --system --add safe.directory $GOSRC

# Ensure that all lower-level contexts and child-processes have
# ready access to higher level orchestration (e.g Cirrus-CI)
# variables.
echo -e "\n# Begin single-use VM global variables (${BASH_SOURCE[0]})" \
    > "/etc/ci_environment"
(
    while read -r env_var; do
        printf -- "%s=%q\n" "${env_var}" "${!env_var}"
    done <<<"$(passthrough_envars)"
) >> "/etc/ci_environment"

# This is a possible manual maintenance gaff, i.e. forgetting to update a
# *_NAME variable in .cirrus.yml.  check to be sure at least one comparison
# matches the actual OS being run.  Ignore details, such as debian point-release
# number and/or '-aarch64' suffix.
# shellcheck disable=SC2154
grep -q "$DISTRO_NV" <<<"$OS_REL_VER" || \
    grep -q "$OS_REL_VER" <<<"$DISTRO_NV" || \
    grep -q "rawhide" <<<"$DISTRO_NV" || \
    die "Automation spec. '$DISTRO_NV'; actual host '$OS_REL_VER'"

# Only allow this script to execute once
if ((${SETUP_ENVIRONMENT:-0})); then
    # Comes from automation library
    # shellcheck disable=SC2154
    warn "Not executing $SCRIPT_FILENAME again"
    exit 0
fi

cd "${GOSRC}/"

mkdir -p /etc/containers/containers.conf.d

# Defined by lib.sh: Does the host support cgroups v1 or v2? Use runc or crun
# respectively.
# **IMPORTANT**: $OCI_RUNTIME is a fakeout! It is used only in e2e tests.
# For actual podman, as in system tests, we force runtime in containers.conf
case "$CG_FS_TYPE" in
    tmpfs)
        if ((CONTAINER==0)); then
            warn "Forcing testing with runc instead of crun"
            echo "OCI_RUNTIME=runc" >> /etc/ci_environment
            printf "[engine]\nruntime=\"runc\"\n" > /etc/containers/containers.conf.d/90-runtime.conf
        fi
        ;;
    cgroup2fs)
        # Nothing to do: podman defaults to crun
        ;;
    *) die_unknown CG_FS_TYPE
esac

# Force the requested database backend without having to use command-line args
# shellcheck disable=SC2154
printf "[engine]\ndatabase_backend=\"$CI_DESIRED_DATABASE\"\n" > /etc/containers/containers.conf.d/92-db.conf

# For debian envs pre-configure storage driver as overlay.
# See: Discussion here https://github.com/containers/podman/pull/18510#discussion_r1189812306
# for more details.
# TODO: remove this once all CI VM have newer buildah version. (i.e where buildah
# does not defaults to using `vfs` as storage driver)
# shellcheck disable=SC2154
if [[ "$OS_RELEASE_ID" == "debian" ]]; then
    conf=/etc/containers/storage.conf
    if [[ -e $conf ]]; then
        die "FATAL! INTERNAL ERROR! Cannot override $conf"
    fi
    msg "Overriding $conf, setting overlay (was: $buildah_storage)"
    printf '[storage]\ndriver = "overlay"\nrunroot = "/run/containers/storage"\ngraphroot = "/var/lib/containers/storage"\n' >$conf
fi

if ((CONTAINER==0)); then  # Not yet running inside a container
    # Discovered reemergence of BFQ scheduler bug in kernel 5.8.12-200
    # which causes a kernel panic when system is under heavy I/O load.
    # Disable the I/O scheduler (a.k.a. elevator) for all environments,
    # leaving optimization up to underlying storage infrastructure.
    testfs="/"  # mountpoint that experiences the most I/O during testing
    msg "Querying block device owning partition hosting the '$testfs' filesystem"
    # Need --nofsroot b/c btrfs appends subvolume label to `source` name
    testdev=$(findmnt --canonicalize --noheadings --nofsroot \
              --output source --mountpoint $testfs)
    msg "    found partition: '$testdev'"
    testdisk=$(lsblk --noheadings --output pkname --paths $testdev)
    msg "    found block dev: '$testdisk'"
    testsched="/sys/block/$(basename $testdisk)/queue/scheduler"
    if [[ -n "$testdev" ]] && [[ -n "$testdisk" ]] && [[ -e "$testsched" ]]; then
        msg "    Found active I/O scheduler: $(cat $testsched)"
        if [[ ! "$(<$testsched)" =~ \[none\]  ]]; then
            msg "    Disabling elevator for '$testsched'"
            echo "none" > "$testsched"
        else
            msg "    Elevator already disabled"
        fi
    else
        warn "Sys node for elevator doesn't exist: '$testsched'"
    fi
fi

# Which distribution are we testing on.
case "$OS_RELEASE_ID" in
    debian)
        # FIXME 2023-04-11: workaround for runc regression causing failure
        # in system tests: "skipping device /dev/char/10:200 for systemd"
        # FIXME: please remove this once runc >= 1.2 makes it into debian.
        modprobe tun
        ;;
    fedora)
        if ((CONTAINER==0)); then
            # All SELinux distros need this for systemd-in-a-container
            msg "Enabling container_manage_cgroup"
            setsebool container_manage_cgroup true
        fi
        ;;
    *) die_unknown OS_RELEASE_ID
esac

# Networking: force CNI or Netavark as requested in .cirrus.yml
# (this variable is mandatory).
# shellcheck disable=SC2154
case "$CI_DESIRED_NETWORK" in
    netavark)   use_netavark ;;
    cni)        use_cni ;;
    *)          die_unknown CI_DESIRED_NETWORK ;;
esac

# Database: force SQLite or BoltDB as requested in .cirrus.yml.
# If unset, will default to BoltDB.
# shellcheck disable=SC2154
case "$CI_DESIRED_DATABASE" in
    sqlite)
        warn "Forcing PODMAN_DB=sqlite"
        echo "PODMAN_DB=sqlite" >> /etc/ci_environment
	;;
    boltdb)
        warn "Forcing PODMAN_DB=boltdb"
        echo "PODMAN_DB=boltdb" >> /etc/ci_environment
	;;
    "")
        warn "Using default Podman database"
        ;;
    *)
        die_unknown CI_DESIRED_DATABASE
        ;;
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

            # In F35 the hard-coded default
            # (from containers-common-1-32.fc35.noarch) is 'journald' despite
            # the upstream repository having this line commented-out.
            # Containerized integration tests cannot run with 'journald'
            # as there is no daemon/process there to receive them.
            cconf="/usr/share/containers/containers.conf"
            note="- commented-out by setup_environment.sh"
            if grep -Eq '^log_driver.+journald' "$cconf"; then
                warn "Patching out $cconf journald log_driver"
                sed -r -i -e "s/^log_driver(.*)/# log_driver\1 $note/" "$cconf"
            fi
        fi
        ;;
    *) die_unknown TEST_ENVIRON
esac

# Required to be defined by caller: Are we testing as root or a regular user
case "$PRIV_NAME" in
    root)
        # shellcheck disable=SC2154
        if [[ "$TEST_FLAVOR" = "sys" || "$TEST_FLAVOR" = "apiv2" ]]; then
            # Used in local image-scp testing
            setup_rootless
            echo "PODMAN_ROOTLESS_USER=$ROOTLESS_USER" >> /etc/ci_environment
            echo "PODMAN_ROOTLESS_UID=$ROOTLESS_UID" >> /etc/ci_environment
        fi
        ;;
    rootless)
        # load kernel modules since the rootless user has no permission to do so
        modprobe ip6_tables || :
        modprobe ip6table_nat || :
        setup_rootless
        ;;
    *) die_unknown PRIV_NAME
esac

# shellcheck disable=SC2154
if [[ -n "$ROOTLESS_USER" ]]; then
    echo "ROOTLESS_USER=$ROOTLESS_USER" >> /etc/ci_environment
    echo "ROOTLESS_UID=$ROOTLESS_UID" >> /etc/ci_environment
fi

# FIXME! experimental workaround for #16973, the "lookup cdn03.quay.io" flake.
#
# If you are reading this on or after April 2023:
#   * If we're NOT seeing the cdn03 flake any more, well, someone
#     should probably figure out how to fix systemd-resolved, then
#     remove this workaround.
#
#   * If we're STILL seeing the cdn03 flake, well, this "fix"
#     didn't work and should be removed.
#
# Either way, this block of code should be removed after March 31 2023
# because it creates a system that is not representative of real-world Fedora.
if ((CONTAINER==0)); then
    nsswitch=/etc/authselect/nsswitch.conf
    if [[ -e $nsswitch ]]; then
        if grep -q -E 'hosts:.*resolve' $nsswitch; then
            msg "Disabling systemd-resolved"
            sed -i -e 's/^\(hosts: *\).*/\1files dns myhostname/' $nsswitch
            systemctl stop systemd-resolved
            rm -f /etc/resolv.conf

            # NetworkManager may already be running, or it may not....
            systemctl start NetworkManager
            sleep 1
            systemctl restart NetworkManager

            # ...and it may create resolv.conf upon start/restart, or it
            # may not. Keep restarting until it does. (Yes, I realize
            # this is cargocult thinking. Don't care. Not worth the effort
            # to diagnose and solve properly.)
            retries=10
            while ! test -e /etc/resolv.conf;do
                retries=$((retries - 1))
                if [[ $retries -eq 0 ]]; then
                    die "Timed out waiting for resolv.conf"
                fi
                systemctl restart NetworkManager
                sleep 5
            done
        fi
    fi
fi

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
    validate)
        dnf install -y $PACKAGE_DOWNLOAD_DIR/python3*.rpm
        # For some reason, this is also needed for validation
        make .install.pre-commit .install.gitvalidation
        ;;
    altbuild)
        # Defined in .cirrus.yml
        # shellcheck disable=SC2154
        if [[ "$ALT_NAME" =~ RPM ]]; then
            bigto dnf install -y glibc-minimal-langpack go-rpm-macros rpkg rpm-build shadow-utils-subid-devel
        fi
        ;;
    docker-py)
        remove_packaged_podman_files
        make install PREFIX=/usr ETCDIR=/etc

        msg "Installing previously downloaded/cached packages"
        dnf install -y $PACKAGE_DOWNLOAD_DIR/python3*.rpm
        virtualenv .venv/docker-py
        source .venv/docker-py/bin/activate
        pip install --upgrade pip
        pip install --requirement $GOSRC/test/python/requirements.txt
        ;;
    build) make clean ;;
    unit)
        make .install.ginkgo
        ;;
    compose_v2)
        dnf -y remove docker-compose
        # TODO: Either move this "install" into CI VM image build scripts
        # since runtime-installs can be fragile.  Or, configure renovate
        # to manage the version number since nobody will ever realize to
        # update it here, on their own.
        curl -SL https://github.com/docker/compose/releases/download/v2.2.3/docker-compose-linux-x86_64 -o /usr/local/bin/docker-compose
        chmod +x /usr/local/bin/docker-compose
        ;& # Continue with next item
    apiv2)
        msg "Installing previously downloaded/cached packages"
        dnf install -y $PACKAGE_DOWNLOAD_DIR/python3*.rpm
        virtualenv .venv/requests
        source .venv/requests/bin/activate
        pip install --upgrade pip
        pip install --requirement $GOSRC/test/apiv2/python/requirements.txt
        ;&  # continue with next item
    int)
        make .install.ginkgo
        ;&
    sys) ;&
    upgrade_test) ;&
    bud) ;&
    bindings) ;&
    endpoint)
        # Use existing host bits when testing is to happen inside a container
        # since this script will run again in that environment.
        # shellcheck disable=SC2154
        if [[ "$TEST_ENVIRON" =~ host ]]; then
            if ((CONTAINER)); then
                die "Refusing to config. host-test in container";
            fi
            remove_packaged_podman_files
            make install PREFIX=/usr ETCDIR=/etc
        elif [[ "$TEST_ENVIRON" == "container" ]]; then
            if ((CONTAINER)); then
                remove_packaged_podman_files
                make install PREFIX=/usr ETCDIR=/etc
            fi
        else
            die "Invalid value for \$TEST_ENVIRON=$TEST_ENVIRON"
        fi

        install_test_configs
        ;;
    minikube)
        dnf install -y $PACKAGE_DOWNLOAD_DIR/minikube-latest*
        remove_packaged_podman_files
        make install.tools
        make install PREFIX=/usr ETCDIR=/etc
        minikube config set driver podman
        install_test_configs
        ;;
    machine)
        dnf install -y podman-gvproxy*
        remove_packaged_podman_files
        make install PREFIX=/usr ETCDIR=/etc
        install_test_configs
        ;;
    swagger)
        make .install.swagger
        ;;
    release) ;;
    *) die_unknown TEST_FLAVOR
esac

# See ./contrib/cirrus/CIModes.md.
# Vars defined by cirrus-ci
# shellcheck disable=SC2154
if [[ ! "$OS_RELEASE_ID" =~ "debian" ]] && \
   [[ "$CIRRUS_CHANGE_TITLE" =~ CI:NEXT ]]
then
    # shellcheck disable=SC2154
    if [[ "$CIRRUS_PR_DRAFT" != "true" ]]; then
        die "Magic 'CI:NEXT' string can only be used on DRAFT PRs"
    fi

    showrun dnf copr enable rhcontainerbot/podman-next -y

    # DNF ignores repos that don't exist.  For example, updates-testing is not
    # enabled on Fedora N-1 CI VMs.  Don't updated everything, isolate just the
    # podman-next COPR updates.
    showrun dnf update -y \
      "--enablerepo=copr:copr.fedorainfracloud.org:rhcontainerbot:podman-next" \
      "--disablerepo=copr:copr.fedorainfracloud.org:sbrivio:passt" \
      "--disablerepo=fedora*" "--disablerepo=updates*"
fi

# Must be the very last command.  Prevents setup from running twice.
echo 'SETUP_ENVIRONMENT=1' >> /etc/ci_environment
echo -e "\n# End of global variable definitions" \
    >> /etc/ci_environment

msg "Global CI Environment vars.:"
grep -Ev '^#' /etc/ci_environment | sort | indent
