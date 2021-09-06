#!/usr/bin/env bats   -*- bats -*-
#
# Tests generated configurations for systemd.
#

load helpers
load helpers.systemd

SERVICE_NAME="podman_test_$(random_string)"

UNIT_FILE="$UNIT_DIR/$SERVICE_NAME.service"

function setup() {
    skip_if_remote "systemd tests are meaningless over remote"

    basic_setup
}

function teardown() {
    run '?' systemctl stop "$SERVICE_NAME"
    rm -f "$UNIT_FILE"
    systemctl daemon-reload
    run_podman rmi -a

    basic_teardown
}

# Helper to start a systemd service running a container
function service_setup() {
    run_podman generate systemd --new $cname
    echo "$output" > "$UNIT_FILE"
    run_podman rm $cname

    systemctl daemon-reload

    run systemctl start "$SERVICE_NAME"
    if [ $status -ne 0 ]; then
        die "Error starting systemd unit $SERVICE_NAME, output: $output"
    fi

    run systemctl status "$SERVICE_NAME"
    if [ $status -ne 0 ]; then
        die "Non-zero status of systemd unit $SERVICE_NAME, output: $output"
    fi
}

# Helper to stop a systemd service running a container
function service_cleanup() {
    local status=$1
    run systemctl stop "$SERVICE_NAME"
    if [ $status -ne 0 ]; then
        die "Error stopping systemd unit $SERVICE_NAME, output: $output"
    fi

    if [[ -z "$status" ]]; then
        run systemctl is-active "$SERVICE_NAME"
        if [ $status -ne 0 ]; then
            die "Error checking stauts of systemd unit $SERVICE_NAME, output: $output"
        fi
        is "$output" "$status" "$SERVICE_NAME not in expected state"
    fi

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
    cname=$(random_string)
    run_podman create --name $cname --label "io.containers.autoupdate=local" $IMAGE top

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
@test "podman generate systemd - restart policy" {
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
    is "$output" ".*Restart=on-failure.*" "on-failure:xx is parsed correclty"
    is "$output" ".*StartLimitBurst=42.*" "on-failure:xx is parsed correctly"

    run_podman rm -f $cname $cname2 $cname3
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
	is "$output" "$stdenv
LISTEN_PID=1
LISTEN_FDS=1
LISTEN_FDNAMES=listen_fdnames" "LISTEN Environment passed: $context"
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

# vim: filetype=sh
