#!/usr/bin/env bats   -*- bats -*-
#
# tests for podman system connection
#

load helpers
load helpers.network

# This will be set if we start a local service
_SERVICE_PID=

function setup() {
    if ! is_remote; then
        skip "only applicable when running remote"
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

    # Aaaaargh! When running as root, 'system service' creates a tmpfs
    # mount on $root/overlay. This in turn causes cleanup to fail.
    mount \
        | grep $PODMAN_TMPDIR \
        | awk '{print $3}' \
        | xargs -l1 --no-run-if-empty umount

    # Remove all system connections
    run_podman system connection rm --all

    basic_teardown
}

# Helper function: invokes $PODMAN (which is podman-remote) _without_ --url opt
#
# Needed because, in CI, PODMAN="/path/to/podman-remote --url /path/to/socket"
# which of course overrides podman's detection and use of a connection.
function _run_podman_remote() {
    PODMAN=${PODMAN%%--url*} run_podman "$@"
}

# Very basic test, does not actually connect at any time
@test "podman system connection - basic add / ls / remove" {
    run_podman system connection ls
    is "$output" "Name        URI         Identity    Default" \
       "system connection ls: no connections"

    c1="c1_$(random_string 15)"
    c2="c2_$(random_string 15)"

    run_podman system connection add $c1 tcp://localhost:12345
    run_podman context create --docker "host=tcp://localhost:54321" $c2
    run_podman system connection ls
    is "$output" \
       ".*$c1[ ]\+tcp://localhost:12345[ ]\+true
$c2[ ]\+tcp://localhost:54321[ ]\+false" \
       "system connection ls"
    run_podman system connection ls -q
    is "$(echo $(sort <<<$output))" \
       "$c1 $c2" \
       "system connection ls -q should show two names"
    run_podman context ls -q
    is "$(echo $(sort <<<$output))" \
       "$c1 $c2" \
       "context ls -q should show two names"
    run_podman context use $c2
    run_podman system connection ls
    is "$output" \
       ".*$c1[ ]\+tcp://localhost:12345[ ]\+false
$c2[ ]\+tcp://localhost:54321[ ]\+true" \
       "system connection ls"

    # Remove default connection; the remaining one should still not be default
    run_podman system connection rm $c2
    run_podman context ls
    is "$output" ".*$c1[ ]\+tcp://localhost:12345[ ]\+false" \
       "system connection ls (after removing default connection)"

    run_podman context rm $c1
}

# Test tcp socket; requires starting a local server
@test "podman system connection - tcp" {
    # Start server
    _SERVICE_PORT=$(random_free_port 63000-64999)

    # Add the connection, and run podman info *before* starting the service.
    # This should fail.
    run_podman system connection add myconnect tcp://localhost:$_SERVICE_PORT
    # IMPORTANT NOTE: in CI, podman-remote is tested by setting PODMAN
    # to "podman-remote --url sdfsdf". This of course overrides the default
    # podman-remote action. Our solution: strip off the "--url xyz" part
    # when invoking podman.
    _run_podman_remote 125 info
    is "$output" \
       "Cannot connect to Podman. Please verify.*dial tcp.*connection refused" \
       "podman info, without active service"

    # Start service. Now podman info should work fine. The %%-remote*
    # converts "podman-remote --opts" to just "podman", which is what
    # we need for the server.
    ${PODMAN%%-remote*} --root ${PODMAN_TMPDIR}/root \
                        --runroot ${PODMAN_TMPDIR}/runroot \
                        system service -t 99 tcp://localhost:$_SERVICE_PORT &
    _SERVICE_PID=$!
    # Wait for the port and the podman-service to be ready.
    wait_for_port localhost $_SERVICE_PORT
    local timeout=10
    while [[ $timeout -gt 1 ]]; do
        _run_podman_remote '?' info --format '{{.Host.RemoteSocket.Path}}'
        if [[ $status == 0 ]]; then
            break
        fi
        sleep 1
        let timeout=$timeout-1
    done
    is "$output" "tcp://localhost:$_SERVICE_PORT" \
       "podman info works, and talks to the correct server"

    _run_podman_remote info --format '{{.Store.GraphRoot}}'
    is "$output" "${PODMAN_TMPDIR}/root" \
       "podman info, talks to the right service"

    # Add another connection; make sure it does not get set as default
    _run_podman_remote system connection add fakeconnect tcp://localhost:$(( _SERVICE_PORT + 1))
    _run_podman_remote info --format '{{.Store.GraphRoot}}'
    # (Don't bother checking output; we just care about exit status)

    # Stop server. Use 'run' to avoid failing on nonzero exit status
    run kill $_SERVICE_PID
    run wait $_SERVICE_PID
    _SERVICE_PID=

    run_podman system connection rm fakeconnect
    run_podman system connection rm myconnect
}

# If we have ssh access to localhost (unlikely in CI), test that.
@test "podman system connection - ssh" {
    # system connection only really works if we have an agent
    run ssh-add -l
    test "$status"      -eq 0 || skip "Not running under ssh-agent"
    test "${#lines[@]}" -ge 1 || skip "ssh agent has no identities"

    # Can we actually ssh to localhost?
    rand=$(random_string 20)
    echo $rand >$PODMAN_TMPDIR/testfile
    run ssh -q -o BatchMode=yes \
        -o UserKnownHostsFile=/dev/null \
        -o StrictHostKeyChecking=no \
        -o CheckHostIP=no \
        localhost \
        cat $PODMAN_TMPDIR/testfile
    test "$status" -eq 0 || skip "cannot ssh to localhost"
    is "$output" "$rand" "weird! ssh worked, but could not cat local file"

    # OK, ssh works.
    # Create a new connection, over ssh, but using existing socket file
    # (Remember, we're already podman-remote, there's a service running)
    run_podman info --format '{{.Host.RemoteSocket.Path}}'
    local socketpath="$output"
    run_podman system connection add --socket-path "$socketpath" \
               mysshcon ssh://localhost
    is "$output" "" "output from system connection add"

    # debug logs will confirm that we use ssh connection
    _run_podman_remote --log-level=debug info --format '{{.Host.RemoteSocket.Path}}'
    is "$output" ".*msg=\"SSH Agent Key .*" "we are truly using ssh"

    # Clean up
    run_podman system connection rm mysshconn
}

# vim: filetype=sh
