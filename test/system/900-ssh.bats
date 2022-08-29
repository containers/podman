#!/usr/bin/env bats
#
# Simplest set of podman tests. If any of these fail, we have serious problems.
#

load helpers

# Override standard setup! We don't yet trust podman-images or podman-rm
function setup() {
    if ! is_remote; then
        skip "only applicable on podman-remote"
    fi

    basic_setup
}

function teardown() {
    if ! is_remote; then
        return
    fi

    # In case test function failed to clean up
    if [[ -n $_SERVICE_PID ]]; then
        run kill $_SERVICE_PID
    fi

    # see test/system/272-system-connection.bats for why this is needed
    mount \
        | grep $PODMAN_TMPDIR \
        | awk '{print $3}' \
        | xargs -l1 --no-run-if-empty umount

    run_podman system connection rm --all

    basic_teardown
}

function _run_podman_remote() {
    PODMAN=${PODMAN%%--url*} run_podman "$@"
}

@test "podman --ssh test" {
    skip_if_no_ssh "cannot run these tests without an ssh binary"
    # Start server
    _SERVICE_PORT=$(random_free_port 63000-64999)

    ${PODMAN%%-remote*} --root ${PODMAN_TMPDIR}/root \
                        --runroot ${PODMAN_TMPDIR}/runroot \
                        system service -t 99 tcp://localhost:$_SERVICE_PORT &
    _SERVICE_PID=$!
    wait_for_port localhost $_SERVICE_PORT

    notme=${PODMAN_ROOTLESS_USER}

    uid=$(id -u $notme)

    run_podman 125 --ssh=native system connection add testing ssh://$notme@localhost:22/run/user/$uid/podman/podman.sock
    is "$output" "Error: exit status 255"

    # need to figure out how to podman remote test with the new ssh
}
