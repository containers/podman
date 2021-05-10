#!/usr/bin/env bats   -*- bats -*-
#
# Tests generated configurations for systemd.
#

load helpers

SERVICE_NAME="podman_test_$(random_string)"

SYSTEMCTL="systemctl"
UNIT_DIR="/usr/lib/systemd/system"
if is_rootless; then
    UNIT_DIR="$HOME/.config/systemd/user"
    mkdir -p $UNIT_DIR

    SYSTEMCTL="$SYSTEMCTL --user"
fi
UNIT_FILE="$UNIT_DIR/$SERVICE_NAME.service"

function setup() {
    skip_if_remote "systemd tests are meaningless over remote"

    basic_setup
}

function teardown() {
    run '?' $SYSTEMCTL stop "$SERVICE_NAME"
    rm -f "$UNIT_FILE"
    $SYSTEMCTL daemon-reload
    run_podman rmi -a

    basic_teardown
}

# Helper to setup xdg runtime for rootless
function xdg_rootless() {
    # podman initializes this if unset, but systemctl doesn't
    if is_rootless; then
        if [ -z "$XDG_RUNTIME_DIR" ]; then
            export XDG_RUNTIME_DIR=/run/user/$(id -u)
        fi
    fi
}

# Helper to start a systemd service running a container
function service_setup() {
    run_podman generate systemd --new $cname
    echo "$output" > "$UNIT_FILE"
    run_podman rm $cname

    $SYSTEMCTL daemon-reload

    run $SYSTEMCTL start "$SERVICE_NAME"
    if [ $status -ne 0 ]; then
        die "Error starting systemd unit $SERVICE_NAME, output: $output"
    fi

    run $SYSTEMCTL status "$SERVICE_NAME"
    if [ $status -ne 0 ]; then
        die "Non-zero status of systemd unit $SERVICE_NAME, output: $output"
    fi
}

# Helper to stop a systemd service running a container
function service_cleanup() {
    run $SYSTEMCTL stop "$SERVICE_NAME"
    if [ $status -ne 0 ]; then
        die "Error stopping systemd unit $SERVICE_NAME, output: $output"
    fi

    rm -f "$UNIT_FILE"
    $SYSTEMCTL daemon-reload
}

# These tests can fail in dev. environment because of SELinux.
# quick fix: chcon -t container_runtime_exec_t ./bin/podman
@test "podman generate - systemd - basic" {
    xdg_rootless

    cname=$(random_string)
    # See #7407 for --pull=always.
    run_podman create --pull=always --name $cname --label "io.containers.autoupdate=registry" $IMAGE top

    # Start systemd service to run this container
    service_setup

    # Give container time to start; make sure output looks top-like
    sleep 2
    run_podman logs $cname
    is "$output" ".*Load average:.*" "running container 'top'-like output"

    # Exercise `podman auto-update`.
    # TODO: this will at least run auto-update code but won't perform an update
    #       since the image didn't change.  We need to improve on that and run
    #       an image from a local registry instead.
    run_podman auto-update

    # All good. Stop service, clean up.
    service_cleanup
}

@test "podman autoupdate local" {
    xdg_rootless

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
    is $output $SERVICE_NAME "autoupdate local restarted container"

    # All good. Stop service, clean up.
    service_cleanup
}

# These tests can fail in dev. environment because of SELinux.
# quick fix: chcon -t container_runtime_exec_t ./bin/podman
@test "podman generate systemd - envar" {
    xdg_rootless

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

# vim: filetype=sh
