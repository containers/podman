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
               sh -c 'printenv NOTIFY_SOCKET;echo READY;sleep 999'
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
    run_podman rm -f -t0 $cid
    _stop_socat
}

# These tests can fail in dev. environment because of SELinux.
# quick fix: chcon -t container_runtime_exec_t ./bin/podman
@test "sdnotify : container" {
    _prefetch $SYSTEMD_IMAGE

    export NOTIFY_SOCKET=$PODMAN_TMPDIR/container.sock
    _start_socat

    run_podman run -d --sdnotify=container $SYSTEMD_IMAGE \
               sh -c 'trap "touch /stop" SIGUSR1;printenv NOTIFY_SOCKET; echo READY; while ! test -f /stop;do sleep 0.1;done;systemd-notify --ready'
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

    # Done. Tell container to stop itself, and clean up
    run_podman kill -s USR1 $cid
    run_podman wait $cid

    wait_for_file $_SOCAT_LOG
    run cat $_SOCAT_LOG
    echo "socat log:"
    echo "$output"
    is "$output" "MAINPID=$mainPID
READY=1"

    run_podman rm $cid
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
  restartPolicy: "Never"
  containers:
  - command:
    - /bin/sh
    - -c
    - 'while :; do if test -e /rain/tears; then exit 0; fi; sleep 1; done'
    image: $IMAGE
    name: test
    resources: {}
    volumeMounts:
    - mountPath: /rain:z
      name: test-mountdir
  volumes:
  - hostPath:
      path: $PODMAN_TMPDIR
      type: Directory
    name: test-mountdir
EOF

    # The name of the service container is predictable: the first 12 characters
    # of the hash of the YAML file followed by the "-service" suffix
    yaml_sha=$(sha256sum $yaml_source)
    service_container="${yaml_sha:0:12}-service"

    export NOTIFY_SOCKET=$PODMAN_TMPDIR/conmon.sock
    _start_socat
    wait_for_file $_SOCAT_LOG

    run_podman play kube --service-container=true --log-driver journald $yaml_source

    # The service container is the main PID since no container has a custom
    # sdnotify policy.
    run_podman container inspect $service_container --format "{{.State.ConmonPid}}"
    main_pid="$output"

    # Tell pod to finish, then wait for all containers to stop
    touch $PODMAN_TMPDIR/tears
    run_podman container wait $service_container test_pod-test

    # Make sure the containers have the correct policy.
    run_podman container inspect test_pod-test $service_container --format "{{.Config.SdNotifyMode}}"
    is "$output" "ignore
ignore"

    # The 'echo's help us debug failed runs
    run cat $_SOCAT_LOG
    echo "socat log:"
    echo "$output"

    # The "with policies" test below checks the MAINPID.
    is "$output" "MAINPID=$main_pid
READY=1" "sdnotify sent MAINPID and READY"

    _stop_socat

    # Clean up pod and pause image
    run_podman play kube --down $PODMAN_TMPDIR/test.yaml
    run_podman rmi $(pause_image)
}

@test "sdnotify : play kube - with policies" {
    skip_if_journald_unavailable

    _prefetch $SYSTEMD_IMAGE

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
  restartPolicy: "Never"
  containers:
  - command:
    - /bin/sh
    - -c
    - 'printenv NOTIFY_SOCKET; while ! test -f /stop;do sleep 0.1;done'
    image: $SYSTEMD_IMAGE
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
        $PODMAN play kube --service-container=true --log-driver journald $yaml_source &>/dev/null &

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

    # Send the READY message.  Doing it in an exec session helps debug
    # potential issues.
    run_podman exec --env NOTIFY_SOCKET="/run/notify/notify.sock" $container_a /usr/bin/systemd-notify --ready

    # Instruct the container to stop.
    # Run detached as the `exec` session races with the cleanup process
    # of the exiting container (see #10825).
    run_podman exec -d $container_a /bin/touch /stop

    run_podman container wait $container_a
    run_podman container inspect $container_a --format "{{.State.ExitCode}}"
    is "$output" "0" "container exited cleanly after sending READY message"
    wait_for_file $_SOCAT_LOG
    # The 'echo's help us debug failed runs
    run cat $_SOCAT_LOG
    echo "socat log:"
    echo "$output"

    is "$output" "MAINPID=.*
READY=1" "sdnotify sent MAINPID and READY"

    # Make sure that Podman is the service's MainPID
    main_pid=$(awk -F= '{print $2}' <<< ${lines[0]})
    is "$(</proc/$main_pid/comm)" "podman" "podman is the service mainPID"
    _stop_socat

    # Clean up pod and pause image
    run_podman play kube --down $yaml_source
    run_podman rmi $(pause_image)
}

function generate_exit_code_yaml {
    local fname=$1
    local cmd1=$2
    local cmd2=$3
    local sdnotify_policy=$4
    echo "
apiVersion: v1
kind: Pod
metadata:
  labels:
    app: test
  name: test_pod
  annotations:
    io.containers.sdnotify: "$sdnotify_policy"
spec:
  restartPolicy: Never
  containers:
    - name: ctr1
      image: $IMAGE
      command:
      - $cmd1
    - name: ctr2
      image: $IMAGE
      command:
      - $cmd2
" > $fname
}

@test "podman kube play - exit-code propagation" {
    fname=$PODMAN_TMPDIR/$(random_string).yaml

    # Create a test matrix with the following arguments:
    # exit-code propagation | ctr1 command | ctr2 command | service-container exit code
    exit_tests="
all  | true  | true  | 0
all  | true  | false | 0
all  | false | false | 137
any  | true  | true  | 0
any  | false | true  | 137
any  | false | false | 137
none | true  | true  | 0
none | true  | false | 0
none | false | false | 0
"

    # I am sorry, this is a long test as we need to test the upper matrix
    # twice. The first run is using the default sdnotify policy of "ignore".
    # In this case, the service container serves as the main PID of the service
    # to have a minimal resource footprint.  The second run is using the
    # "conmon" sdnotify policy in which case Podman needs to serve as the main
    # PID to act as an sdnotify proxy; there Podman will wait for the service
    # container to exit and reflects its exit code.
    while read exit_code_prop cmd1 cmd2 exit_code; do
        for sdnotify_policy in ignore conmon; do
            generate_exit_code_yaml $fname $cmd1 $cmd2 $sdnotify_policy
            yaml_sha=$(sha256sum $fname)
            service_container="${yaml_sha:0:12}-service"
            podman_exit=$exit_code
            if [[ $sdnotify_policy == "ignore" ]];then
                 podman_exit=0
            fi
            run_podman $podman_exit kube play --service-exit-code-propagation="$exit_code_prop" --service-container $fname
            run_podman container inspect --format '{{.KubeExitCodePropagation}}' $service_container
            is "$output" "$exit_code_prop" "service container has the expected policy set in its annotations"
            run_podman wait $service_container
            is "$output" "$exit_code" "service container reflects expected exit code $exit_code (policy: $policy, cmd1: $cmd1, cmd2: $cmd2)"
            run_podman kube down $fname
        done
    done < <(parse_table "$exit_tests")

    # A final smoke test to make sure bogus policies lead to an error
    run_podman 125 kube play --service-exit-code-propagation=bogus --service-container $fname
    is "$output" "Error: unsupported exit-code propagation \"bogus\"" "error on unsupported exit-code propagation"

    run_podman rmi $(pause_image)
}
# vim: filetype=sh
