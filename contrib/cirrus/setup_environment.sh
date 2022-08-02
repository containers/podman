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
    ubuntu) ;;
    fedora)
        if ((CONTAINER==0)); then
            # All SELinux distros need this for systemd-in-a-container
            msg "Enabling container_manage_cgroup"
            setsebool container_manage_cgroup true
        fi

        # For release 36 and later, netavark/aardvark is the default
        # networking stack for podman.  All previous releases only have
        # CNI networking available.  Upgrading from one to the other is
        # not supported at this time.  Support execution of the upgrade
        # tests in F36 and later, by disabling Netavark and enabling CNI.
        #
        # OS_RELEASE_VER is defined by automation-library
        # shellcheck disable=SC2154
        if [[ "$OS_RELEASE_VER" -ge 36 ]] && \
           [[ "$TEST_FLAVOR" != "upgrade_test" ]];
        then
            use_netavark
        else # Fedora < 36, or upgrade testing.
            use_cni
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

if [[ -n "$ROOTLESS_USER" ]]; then
    echo "ROOTLESS_USER=$ROOTLESS_USER" >> /etc/ci_environment
    echo "ROOTLESS_UID=$ROOTLESS_UID" >> /etc/ci_environment
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
    ext_svc) ;;
    validate)
        dnf install -y $PACKAGE_DOWNLOAD_DIR/python3*.rpm
        # For some reason, this is also needed for validation
        make install.tools
        make .install.pre-commit
        ;;
    automation) ;;
    altbuild)
        # Defined in .cirrus.yml
        # shellcheck disable=SC2154
        if [[ "$ALT_NAME" =~ RPM ]]; then
            bigto dnf install -y glibc-minimal-langpack go-rpm-macros rpkg rpm-build shadow-utils-subid-devel
        fi
        make install.tools
        ;;
    docker-py)
        remove_packaged_podman_files
        make install.tools
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
        make install.tools
        ;;
    compose_v2)
        make install.tools
        dnf -y remove docker-compose
        curl -SL https://github.com/docker/compose/releases/download/v2.2.3/docker-compose-linux-x86_64 -o /usr/local/bin/docker-compose
        chmod +x /usr/local/bin/docker-compose
        ;& # Continue with next item
    apiv2)
        make install.tools
        msg "Installing previously downloaded/cached packages"
        dnf install -y $PACKAGE_DOWNLOAD_DIR/python3*.rpm
        virtualenv .venv/requests
        source .venv/requests/bin/activate
        pip install --upgrade pip
        pip install --requirement $GOSRC/test/apiv2/python/requirements.txt
        ;&  # continue with next item
    compose)
        make install.tools
        dnf install -y $PACKAGE_DOWNLOAD_DIR/podman-docker*
        ;&  # continue with next item
    int) ;&
    sys) ;&
    upgrade_test) ;&
    bud) ;&
    bindings) ;&
    endpoint)
        make install.tools
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
    machine)
        dnf install -y $PACKAGE_DOWNLOAD_DIR/podman-gvproxy*
        remove_packaged_podman_files
        make install.tools
        make install PREFIX=/usr ETCDIR=/etc
        install_test_configs
        ;;
    gitlab)
        # ***WARNING*** ***WARNING*** ***WARNING*** ***WARNING***
        # This sets up a special ubuntu environment exclusively for
        # running the upstream gitlab-runner unit tests through
        # podman as a drop-in replacement for the Docker daemon.
        # Test and setup information can be found here:
        # https://gitlab.com/gitlab-org/gitlab-runner/-/issues/27270#note_499585550
        #
        # Unless you know what you're doing, and/or are in contact
        # with the upstream gitlab-runner developers/community,
        # please don't make changes willy-nilly to this setup.
        # It's designed to follow upstream gitlab-runner development
        # and alert us if any podman change breaks their foundation.
        #
        # That said, if this task does break in strange ways or requires
        # updates you're unsure of.  Please consult with the upstream
        # community through an issue near the one linked above.  If
        # an extended period of breakage is expected, please un-comment
        # the related `allow_failures: $CI == $CI` line in `.cirrus.yml`.
        # ***WARNING*** ***WARNING*** ***WARNING*** ***WARNING***

        if [[ "$OS_RELEASE_ID" != "ubuntu" ]]; then
            die "This test only runs on Ubuntu due to sheer laziness"
        fi

        remove_packaged_podman_files
        make install PREFIX=/usr ETCDIR=/etc

        msg "Installing docker and containerd"
        # N/B: Tests check/expect `docker info` output, and this `!= podman info`
        ooe.sh dpkg -i \
            $PACKAGE_DOWNLOAD_DIR/containerd.io*.deb \
            $PACKAGE_DOWNLOAD_DIR/docker-ce*.deb

        msg "Disabling docker service and socket activation"
        systemctl stop docker.service docker.socket
        systemctl disable docker.service docker.socket
        rm -rf /run/docker*
        # Guarantee the docker daemon can't be started, even by accident
        rm -vf $(type -P dockerd)

        msg "Recursively chowning source to $ROOTLESS_USER"
        chown -R $ROOTLESS_USER:$ROOTLESS_USER "$GOPATH" "$GOSRC"

        msg "Obtaining necessary gitlab-runner testing bits"
        slug="gitlab.com/gitlab-org/gitlab-runner"
        helper_fqin="registry.gitlab.com/gitlab-org/gitlab-runner/gitlab-runner-helper:x86_64-latest-pwsh"
        ssh="ssh $ROOTLESS_USER@localhost -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no -o CheckHostIP=no env GOPATH=$GOPATH"
        showrun $ssh go get -u github.com/jstemmer/go-junit-report
        showrun $ssh git clone https://$slug $GOPATH/src/$slug
        showrun $ssh make -C $GOPATH/src/$slug development_setup
        showrun $ssh bash -c "'cd $GOPATH/src/$slug && GOPATH=$GOPATH go get .'"

        showrun $ssh podman pull $helper_fqin
        # Tests expect image with this exact name
        showrun $ssh podman tag $helper_fqin \
            docker.io/gitlab/gitlab-runner-helper:x86_64-latest-pwsh
        ;;
    swagger) ;&  # use next item
    consistency)
        make clean
        make install.tools
        ;;
    release) ;;
    *) die_unknown TEST_FLAVOR
esac

# Must be the very last command.  Prevents setup from running twice.
echo 'SETUP_ENVIRONMENT=1' >> /etc/ci_environment
echo -e "\n# End of global variable definitions" \
    >> /etc/ci_environment

msg "Global CI Environment vars.:"
grep -Ev '^#' /etc/ci_environment | sort | indent
