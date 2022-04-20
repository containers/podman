#!/usr/bin/env bats   -*- bats -*-
#
# Tests generated configurations for systemd.
#

load helpers
load helpers.systemd

SERVICE_NAME="podman_test_$(random_string)"

UNIT_FILE="$UNIT_DIR/$SERVICE_NAME.service"
TEMPLATE_FILE_PREFIX="$UNIT_DIR/$SERVICE_NAME"

function setup() {
    skip_if_remote "systemd tests are meaningless over remote"

    basic_setup
}

function teardown() {
    if [[ -e "$UNIT_FILE" ]]; then
        run systemctl stop "$SERVICE_NAME"
        if [ $status -ne 0 ]; then
            echo "# WARNING: systemctl stop failed in teardown: $output" >&3
        fi

        rm -f "$UNIT_FILE"
        systemctl daemon-reload
    fi
    run_podman rmi -a

    basic_teardown
}

# Helper to start a systemd service running a container
function service_setup() {
    run_podman generate systemd --new $cname
    echo "$output" > "$UNIT_FILE"
    run_podman rm $cname

    systemctl daemon-reload

    # Also test enabling services (see #12438).
    run systemctl enable "$SERVICE_NAME"
    assert $status -eq 0 "Error enabling systemd unit $SERVICE_NAME: $output"

    run systemctl start "$SERVICE_NAME"
    assert $status -eq 0 "Error starting systemd unit $SERVICE_NAME: $output"

    run systemctl status "$SERVICE_NAME"
    assert $status -eq 0 "systemctl status $SERVICE_NAME: $output"
}

# Helper to stop a systemd service running a container
function service_cleanup() {
    local status=$1
    run systemctl stop "$SERVICE_NAME"
    assert $status -eq 0 "Error stopping systemd unit $SERVICE_NAME: $output"

    run systemctl disable "$SERVICE_NAME"
    assert $status -eq 0 "Error disabling systemd unit $SERVICE_NAME: $output"

    rm -f "$UNIT_FILE"
    systemctl daemon-reload
}

# These tests can fail in dev. environment because of SELinux.
# quick fix: chcon -t container_runtime_exec_t ./bin/podman
@test "podman generate - systemd - basic" {
    cname=$(random_string)
    # See #7407 for --pull=always.
    run_podman create --pull=always --name $cname --label "io.containers.autoupdate=registry" $IMAGE \
        sh -c "trap 'echo Received SIGTERM, finishing; exit' SIGTERM; echo WAITING; while :; do sleep 0.1; done"

    # Start systemd service to run this container
    service_setup

    # Give container time to start; make sure output looks top-like
    sleep 2
    run_podman logs $cname
    is "$output" ".*WAITING.*" "running is waiting for signal"

    # Exercise `podman auto-update`.
    # TODO: this will at least run auto-update code but won't perform an update
    #       since the image didn't change.  We need to improve on that and run
    #       an image from a local registry instead.
    run_podman auto-update

    # All good. Stop service, clean up.
    # Also make sure the service is in the `inactive` state (see #11304).
    service_cleanup inactive
}

@test "podman autoupdate local" {
    # Note that the entrypoint may be a JSON string which requires preserving the quotes (see #12477)
    cname=$(random_string)
    run_podman create --name $cname --label "io.containers.autoupdate=local" --entrypoint '["top"]' $IMAGE

    # Start systemd service to run this container
    service_setup

    # Give container time to start; make sure output looks top-like
    sleep 2
    run_podman logs $cname
    is "$output" ".*Load average:.*" "running container 'top'-like output"

    # Save the container id before updating
    run_podman ps --format '{{.ID}}'

    # Run auto-update and check that it restarted the container
    run_podman commit --change "CMD=/bin/bash" $cname $IMAGE
    run_podman auto-update
    is "$output" ".*$SERVICE_NAME.*" "autoupdate local restarted container"

    # All good. Stop service, clean up.
    service_cleanup
}

# These tests can fail in dev. environment because of SELinux.
# quick fix: chcon -t container_runtime_exec_t ./bin/podman
@test "podman generate systemd - envar" {
    cname=$(random_string)
    FOO=value BAR=%s run_podman create --name $cname --env FOO -e BAR --env MYVAR=myval \
        $IMAGE sh -c 'printenv && sleep 100'

    # Start systemd service to run this container
    service_setup

    # Give container time to start; make sure output looks top-like
    sleep 2
    run_podman logs $cname
    is "$output" ".*FOO=value.*" "FOO environment variable set"
    is "$output" ".*BAR=%s.*" "BAR environment variable set"
    is "$output" ".*MYVAR=myval.*" "MYVAL environment variable set"

    # All good. Stop service, clean up.
    service_cleanup
}

# Regression test for #11438
@test "podman generate systemd - restart policy & timeouts" {
    cname=$(random_string)
    run_podman create --restart=always --name $cname $IMAGE
    run_podman generate systemd --new $cname
    is "$output" ".*Restart=always.*" "Use container's restart policy if set"
    run_podman generate systemd --new --restart-policy=on-failure $cname
    is "$output" ".*Restart=on-failure.*" "Override container's restart policy"

    cname2=$(random_string)
    run_podman create --restart=unless-stopped --name $cname2 $IMAGE
    run_podman generate systemd --new $cname2
    is "$output" ".*Restart=always.*" "unless-stopped translated to always"

    cname3=$(random_string)
    run_podman create --restart=on-failure:42 --name $cname3 $IMAGE
    run_podman generate systemd --new $cname3
    is "$output" ".*Restart=on-failure.*" "on-failure:xx is parsed correctly"
    is "$output" ".*StartLimitBurst=42.*" "on-failure:xx is parsed correctly"

    run_podman rm -t 0 -f $cname $cname2 $cname3
}

function set_listen_env() {
    export LISTEN_PID="100" LISTEN_FDS="1" LISTEN_FDNAMES="listen_fdnames"
}

function unset_listen_env() {
    unset LISTEN_PID LISTEN_FDS LISTEN_FDNAMES
}

function check_listen_env() {
    local stdenv="$1"
    local context="$2"
    if is_remote; then
	is "$output" "$stdenv" "LISTEN Environment did not pass: $context"
    else
	out=$(for o in $output; do echo $o; done| sort)
	std=$(echo "$stdenv
LISTEN_PID=1
LISTEN_FDS=1
LISTEN_FDNAMES=listen_fdnames" | sort)
       echo "<$out>"
       echo "<$std>"
       is "$out" "$std" "LISTEN Environment passed: $context"
    fi
}

@test "podman pass LISTEN environment " {
    # Note that `--hostname=host1` makes sure that all containers have the same
    # environment.
    run_podman run --hostname=host1 --rm $IMAGE printenv
    stdenv=$output

    # podman run
    set_listen_env
    run_podman run --hostname=host1 --rm $IMAGE printenv
    unset_listen_env
    check_listen_env "$stdenv" "podman run"

    # podman start
    run_podman create --hostname=host1 --rm $IMAGE printenv
    cid="$output"
    set_listen_env
    run_podman start --attach $cid
    unset_listen_env
    check_listen_env "$stdenv" "podman start"
}

@test "podman generate - systemd template" {
    cname=$(random_string)
    run_podman create --name $cname $IMAGE top

    run_podman generate systemd --template -n $cname
    echo "$output" > "$TEMPLATE_FILE_PREFIX@.service"
    run_podman rm -f $cname

    systemctl daemon-reload

    INSTANCE="$SERVICE_NAME@1.service"
    run systemctl start "$INSTANCE"
    assert $status -eq 0 "Error starting systemd unit $INSTANCE: $output"

    run systemctl status "$INSTANCE"
    assert $status -eq 0 "systemctl status $INSTANCE: $output"

    run systemctl stop "$INSTANCE"
    assert $status -eq 0 "Error stopping systemd unit $INSTANCE: $output"

    rm -f "$TEMPLATE_FILE_PREFIX@.service"
    systemctl daemon-reload
}

@test "podman generate - systemd template no support for pod" {
    cname=$(random_string)
    podname=$(random_string)
    run_podman pod create --name $podname
    run_podman run --pod $podname -dt --name $cname $IMAGE top

    run_podman 125 generate systemd --new --template -n $podname
    is "$output" ".*--template is not supported for pods.*" "Error message contains 'not supported'"

    run_podman rm -f $cname
    run_podman pod rm -f $podname
}

@test "podman generate - systemd template only used on --new" {
    cname=$(random_string)
    run_podman create --name $cname $IMAGE top
    run_podman 125 generate systemd --new=false --template -n $cname
    is "$output" ".*--template cannot be set" "Error message should be '--template requires --new'"
}

@test "podman --cgroup=cgroupfs doesn't show systemd warning" {
    DBUS_SESSION_BUS_ADDRESS= run_podman --log-level warning --cgroup-manager=cgroupfs info -f ''
    is "$output" "" "output should be empty"
}

@test "podman --systemd sets container_uuid" {
    run_podman run --systemd=always --name test $IMAGE printenv container_uuid
    container_uuid=$output
    run_podman inspect test --format '{{ .ID }}'
    is "${container_uuid}" "${output:0:32}" "UUID should be first 32 chars of Container id"
}

# https://github.com/containers/podman/issues/13153
@test "podman rootless-netns slirp4netns process should be in different cgroup" {
    is_rootless || skip "only meaningful for rootless"

    cname=$(random_string)
    local netname=testnet-$(random_string 10)

    # create network and container with network
    run_podman network create $netname
    run_podman create --name $cname --network $netname $IMAGE top

    # run container in systemd unit
    service_setup

    # run second container with network
    cname2=$(random_string)
    run_podman run -d --name $cname2 --network $netname $IMAGE top

    # stop systemd container
    service_cleanup

    # now check that the rootless netns slirp4netns process is still alive and working
    run_podman unshare --rootless-netns ip addr
    is "$output" ".*tap0.*" "slirp4netns interface exists in the netns"
    run_podman exec $cname2 nslookup google.com

    run_podman rm -f -t0 $cname2
    run_podman network rm -f $netname
}

# vim: filetype=sh
