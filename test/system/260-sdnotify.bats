#!/usr/bin/env bats   -*- bats -*-
#
# Tests for systemd sdnotify
#

load helpers

# Shared throughout this module: PID of socat process, and path to its log
_SOCAT_PID=
_SOCAT_LOG=

function setup() {
    skip_if_remote "systemd tests are meaningless over remote"

    # Skip if systemd is not running
    systemctl list-units &>/dev/null || skip "systemd not available"

    # sdnotify fails with runc 1.0.0-3-dev2 on Ubuntu. Let's just
    # assume that we work only with crun, nothing else.
    runtime=$(podman_runtime)
    if [[ "$runtime" != "crun" ]]; then
        skip "this test only works with crun, not $runtime"
    fi

    basic_setup
}

function teardown() {
    unset NOTIFY_SOCKET

    _stop_socat

    basic_teardown
}

###############################################################################
# BEGIN helpers

# Run socat process on a socket, logging to well-known path. Each received
# packet is logged with a newline appended, for ease of parsing the log file.
function _start_socat() {
    _SOCAT_LOG="$PODMAN_TMPDIR/socat.log"

    rm -f $_SOCAT_LOG
    # Execute in subshell so we can close fd3 (which BATS uses).
    # This is a superstitious ritual to try to avoid leaving processes behind,
    # and thus prevent CI hangs.
    (exec socat unix-recvfrom:"$NOTIFY_SOCKET",fork \
          system:"(cat;echo) >> $_SOCAT_LOG" 3>&-) &
    _SOCAT_PID=$!
}

# Stop the socat background process and clean up logs
function _stop_socat() {
    if [[ -n "$_SOCAT_PID" ]]; then
        # Kill all child processes, then the process itself.
        # This is a superstitious incantation to avoid leaving processes behind.
        # The '|| true' is because only f35 leaves behind socat processes;
        # f33 (and perhaps others?) behave nicely. ARGH!
        pkill -P $_SOCAT_PID || true
        kill $_SOCAT_PID
    fi
    _SOCAT_PID=

    if [[ -n "$_SOCAT_LOG" ]]; then
        rm -f $_SOCAT_LOG
    fi
    _SOCAT_LOG=
}

# Check that MAINPID=xxxxx points to a running conmon process
function _assert_mainpid_is_conmon() {
    local mainpid=$(expr "$1" : ".*MAINPID=\([0-9]\+\)")
    test -n "$mainpid" || die "Could not parse '$1' as 'MAINPID=nnnn'"

    test -d /proc/$mainpid || die "sdnotify MAINPID=$mainpid - but /proc/$mainpid does not exist"

    # e.g. /proc/12345/exe -> /usr/bin/conmon
    local mainpid_bin=$(readlink /proc/$mainpid/exe)
    is "$mainpid_bin" ".*/conmon" "sdnotify MAINPID=$mainpid is conmon process"
}

# END   helpers
###############################################################################
# BEGIN tests themselves

@test "sdnotify : ignore" {
    export NOTIFY_SOCKET=$PODMAN_TMPDIR/ignore.sock
    _start_socat

    run_podman 1 run --rm --sdnotify=ignore $IMAGE printenv NOTIFY_SOCKET
    is "$output" "" "\$NOTIFY_SOCKET in container"

    is "$(< $_SOCAT_LOG)" "" "nothing received on socket"
    _stop_socat
}

@test "sdnotify : conmon" {
    export NOTIFY_SOCKET=$PODMAN_TMPDIR/conmon.sock
    _start_socat

    run_podman run -d --name sdnotify_conmon_c \
               --sdnotify=conmon \
               $IMAGE \
               sh -c 'printenv NOTIFY_SOCKET;echo READY;while ! test -f /stop;do sleep 0.1;done'
    cid="$output"
    wait_for_ready $cid

    run_podman container inspect sdnotify_conmon_c --format "{{.State.ConmonPid}}"
    mainPID="$output"

    run_podman logs sdnotify_conmon_c
    is "$output" "READY" "\$NOTIFY_SOCKET in container"

    # The 'echo's help us debug failed runs
    run cat $_SOCAT_LOG
    echo "socat log:"
    echo "$output"

    is "$output" "MAINPID=$mainPID
READY=1" "sdnotify sent MAINPID and READY"

    _assert_mainpid_is_conmon "$output"

    # Done. Stop container, clean up.
    run_podman exec $cid touch /stop
    run_podman wait $cid
    run_podman rm $cid
    _stop_socat
}

# These tests can fail in dev. environment because of SELinux.
# quick fix: chcon -t container_runtime_exec_t ./bin/podman
@test "sdnotify : container" {
    # Sigh... we need to pull a humongous image because it has systemd-notify.
    # (IMPORTANT: fedora:32 and above silently removed systemd-notify; this
    # caused CI to hang. That's why we explicitly require fedora:31)
    # FIXME: is there a smaller image we could use?
    local _FEDORA="$PODMAN_TEST_IMAGE_REGISTRY/$PODMAN_TEST_IMAGE_USER/fedora:31"
    # Pull that image. Retry in case of flakes.
    run_podman pull $_FEDORA || \
        run_podman pull $_FEDORA || \
        run_podman pull $_FEDORA

    export NOTIFY_SOCKET=$PODMAN_TMPDIR/container.sock
    _start_socat

    run_podman run -d --sdnotify=container $_FEDORA \
               sh -c 'printenv NOTIFY_SOCKET;echo READY;systemd-notify --ready;while ! test -f /stop;do sleep 0.1;done'
    cid="$output"
    wait_for_ready $cid

    run_podman logs $cid
    is "${lines[0]}" "/run/notify/notify.sock" "NOTIFY_SOCKET is passed to container"

    # With container, READY=1 isn't necessarily the last message received;
    # just look for it anywhere in received messages
    run cat $_SOCAT_LOG
    # The 'echo's help us debug failed runs
    echo "socat log:"
    echo "$output"

    is "$output" ".*READY=1" "received READY=1 through notify socket"

    _assert_mainpid_is_conmon "$output"

    # Done. Stop container, clean up.
    run_podman exec $cid touch /stop
    run_podman wait $cid
    run_podman rm $cid
    run_podman rmi $_FEDORA
    _stop_socat
}

@test "sdnotify : play kube" {
    # Create the YAMl file
    yaml_source="$PODMAN_TMPDIR/test.yaml"
    cat >$yaml_source <<EOF
apiVersion: v1
kind: Pod
metadata:
  labels:
    app: test
  name: test_pod
spec:
  containers:
  - command:
    - top
    image: $IMAGE
    name: test
    resources: {}
EOF

    # The name of the service container is predictable: the first 12 characters
    # of the hash of the YAML file followed by the "-service" suffix
    yaml_sha=$(sha256sum $yaml_source)
    service_container="${yaml_sha:0:12}-service"


    export NOTIFY_SOCKET=$PODMAN_TMPDIR/conmon.sock
    _start_socat

    run_podman play kube --service-container=true $yaml_source
    run_podman container inspect $service_container --format "{{.State.ConmonPid}}"
    mainPID="$output"
    # The 'echo's help us debug failed runs
    run cat $_SOCAT_LOG
    echo "socat log:"
    echo "$output"

    is "$output" "MAINPID=$mainPID
READY=1" "sdnotify sent MAINPID and READY"

    _stop_socat

    # Clean up pod and pause image
    run_podman play kube --down $PODMAN_TMPDIR/test.yaml
    run_podman version --format "{{.Server.Version}}-{{.Server.Built}}"
    podman rmi -f localhost/podman-pause:$output
}


# vim: filetype=sh
