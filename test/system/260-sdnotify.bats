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

    # Reset socat logfile to empty
    rm -f $_SOCAT_LOG
    touch $_SOCAT_LOG
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

    run_podman create --rm --sdnotify=ignore $IMAGE printenv NOTIFY_SOCKET
    cid="$output"

    run_podman container inspect $cid --format "{{.Config.SdNotifyMode}} {{.Config.SdNotifySocket}}"
    is "$output" "ignore " "NOTIFY_SOCKET is not set with 'ignore' mode"

    run_podman 1 start --attach $cid
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

    run_podman container inspect $cid --format "{{.Config.SdNotifyMode}} {{.Config.SdNotifySocket}}"
    is "$output" "conmon $NOTIFY_SOCKET"

    run_podman container inspect sdnotify_conmon_c --format "{{.State.ConmonPid}}"
    mainPID="$output"

    run_podman logs sdnotify_conmon_c
    is "$output" "READY" "\$NOTIFY_SOCKET in container"

    # The 'echo's help us debug failed runs
    wait_for_file $_SOCAT_LOG
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
               sh -c 'printenv NOTIFY_SOCKET; echo READY; while ! test -f /stop;do sleep 0.1;done;systemd-notify --ready'
    cid="$output"
    wait_for_ready $cid

    run_podman container inspect $cid --format "{{.Config.SdNotifyMode}} {{.Config.SdNotifySocket}}"
    is "$output" "container $NOTIFY_SOCKET"

    run_podman logs $cid
    is "${lines[0]}" "/run/notify/notify.sock" "NOTIFY_SOCKET is passed to container"

    run_podman container inspect $cid --format "{{.State.ConmonPid}}"
    mainPID="$output"
    # With container, READY=1 isn't necessarily the last message received;
    # just look for it anywhere in received messages
    run cat $_SOCAT_LOG
    # The 'echo's help us debug failed runs
    echo "socat log:"
    echo "$output"

    is "$output" "MAINPID=$mainPID" "Container is not ready yet, so we only know the main PID"

    # Done. Stop container, clean up.
    run_podman exec $cid touch /stop
    run_podman wait $cid

    wait_for_file $_SOCAT_LOG
    run cat $_SOCAT_LOG
    echo "socat log:"
    echo "$output"
    is "$output" "MAINPID=$mainPID
READY=1"

    run_podman rm $cid
    run_podman rmi $_FEDORA
    _stop_socat
}

@test "sdnotify : play kube - no policies" {
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

    # Make sure the containers have the correct policy.
    run_podman container inspect test_pod-test $service_container --format "{{.Config.SdNotifyMode}}"
    is "$output" "ignore
ignore"

    run_podman container inspect $service_container --format "{{.State.ConmonPid}}"
    mainPID="$output"
    wait_for_file $_SOCAT_LOG
    # The 'echo's help us debug failed runs
    run cat $_SOCAT_LOG
    echo "socat log:"
    echo "$output"

    is "$output" "MAINPID=$mainPID
READY=1" "sdnotify sent MAINPID and READY"

    _stop_socat

    # Clean up pod and pause image
    run_podman play kube --down $PODMAN_TMPDIR/test.yaml
    run_podman rmi $(pause_image)
}

@test "sdnotify : play kube - with policies" {
    # Sigh... we need to pull a humongous image because it has systemd-notify.
    # (IMPORTANT: fedora:32 and above silently removed systemd-notify; this
    # caused CI to hang. That's why we explicitly require fedora:31)
    # FIXME: is there a smaller image we could use?
    local _FEDORA="$PODMAN_TEST_IMAGE_REGISTRY/$PODMAN_TEST_IMAGE_USER/fedora:31"
    # Pull that image. Retry in case of flakes.
    run_podman pull $_FEDORA || \
        run_podman pull $_FEDORA || \
        run_podman pull $_FEDORA

    # Create the YAMl file
    yaml_source="$PODMAN_TMPDIR/test.yaml"
    cat >$yaml_source <<EOF
apiVersion: v1
kind: Pod
metadata:
  labels:
    app: test
  name: test_pod
  annotations:
    io.containers.sdnotify:   "container"
    io.containers.sdnotify/b: "conmon"
spec:
  containers:
  - command:
    - /bin/sh
    - -c
    - 'printenv NOTIFY_SOCKET; echo READY; while ! test -f /stop;do sleep 0.1;done;systemd-notify --ready'
    image: $_FEDORA
    name: a
  - command:
    - /bin/sh
    - -c
    - 'echo READY; top'
    image: $IMAGE
    name: b
EOF
    container_a="test_pod-a"
    container_b="test_pod-b"

    # The name of the service container is predictable: the first 12 characters
    # of the hash of the YAML file followed by the "-service" suffix
    yaml_sha=$(sha256sum $yaml_source)
    service_container="${yaml_sha:0:12}-service"

    export NOTIFY_SOCKET=$PODMAN_TMPDIR/conmon.sock
    _start_socat

    # Run `play kube` in the background as it will wait for all containers to
    # send the READY=1 message.
    timeout --foreground -v --kill=10 60 \
        $PODMAN play kube --service-container=true $yaml_source &>/dev/null &

    # Wait for both containers to be running
    for i in $(seq 1 20); do
        run_podman "?" container wait $container_a $container_b --condition="running"
        if [[ $status == 0 ]]; then
            break
        fi
        sleep 0.5
        # Just for debugging
        run_podman ps -a
    done
    if [[ $status != 0 ]]; then
        die "container $container_a and/or $container_b did not start"
    fi

    # Make sure the containers have the correct policy
    run_podman container inspect $container_a $container_b $service_container --format "{{.Config.SdNotifyMode}}"
    is "$output" "container
conmon
ignore"

    is "$(< $_SOCAT_LOG)" "" "nothing received on socket"

    # Make sure the container received a "proxy" socket and is not using the
    # one of `kube play`
    run_podman container inspect $container_a --format "{{.Config.SdNotifySocket}}"
    assert "$output" != $NOTIFY_SOCKET

    run_podman logs $container_a
    is "${lines[0]}" "/run/notify/notify.sock" "NOTIFY_SOCKET is passed to container"

    # Instruct the container to send the READY
    run_podman exec $container_a /bin/touch /stop

    run_podman container inspect $service_container --format "{{.State.ConmonPid}}"
    main_pid="$output"

    run_podman container wait $container_a
    wait_for_file $_SOCAT_LOG
    # The 'echo's help us debug failed runs
    run cat $_SOCAT_LOG
    echo "socat log:"
    echo "$output"

    is "$output" "MAINPID=$main_pid
READY=1" "sdnotify sent MAINPID and READY"

    _stop_socat

    # Clean up pod and pause image
    run_podman play kube --down $yaml_source
    run_podman rmi $_FEDORA $(pause_image)
}

# vim: filetype=sh
