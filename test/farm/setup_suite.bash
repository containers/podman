# -*- bash -*-

bats_require_minimum_version 1.8.0

load helpers

function setup_suite(){
    if [[ -z "$ROOTLESS_USER" ]]; then
        if ! is_rootless; then
            die "Cannot run as root with no \$ROOTLESS_USER defined"
        fi
        export ROOTLESS_USER=$(id -un)
    fi

    sshdir=/home/$ROOTLESS_USER/.ssh
    sshkey=$sshdir/id_rsa
    if [[ ! -e $sshkey ]]; then
        ssh-keygen -t rsa -N "" -f $sshkey
        cat ${sshkey}.pub >> $sshdir/authorized_keys

        # Confirm that ssh localhost works. Since this is probably
        # the first time that we ssh, bypass the host key verification.
        ssh -T -o 'BatchMode yes' -o 'StrictHostKeyChecking no' localhost true
    fi

    # Sigh..... "system connection add" fails if podman is not in $PATH.
    # There does not seem to be any way to tell it to use an explicit path.
    type -P podman || die "No 'podman' in \$PATH"

    export FARMNAME="test-farm-$(random_string 5)"

    # only set up the podman farm before the first test
    run_podman system connection add --identity $sshkey test-node $ROOTLESS_USER@localhost
    run_podman farm create $FARMNAME test-node
}

function teardown_suite(){
    # clear out the farms after the last farm test
    run_podman farm rm --all
}
