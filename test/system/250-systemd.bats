#!/usr/bin/env bats   -*- bats -*-
#
# Tests generated configurations for systemd.
#

load helpers

SERVICE_NAME="podman_test_$(random_string)"
UNIT_DIR="/usr/lib/systemd/system"
UNIT_FILE="$UNIT_DIR/$SERVICE_NAME.service"

function setup() {
    skip_if_remote
    skip_if_rootless "systemd tests are root-only for now"

    basic_setup
}

function teardown() {
    rm -f "$UNIT_FILE"
    systemctl daemon-reload
    basic_teardown
}

@test "podman generate - systemd - basic" {
    run_podman create --name keepme --detach busybox:latest top

    run_podman generate systemd --new keepme > "$UNIT_FILE"
    run_podman rm keepme

    systemctl daemon-reload

    run systemctl start "$SERVICE_NAME"
    if [ $status -ne 0 ]; then
        die "Error starting systemd unit $SERVICE_NAME, output: $output"
    fi

    run systemctl status "$SERVICE_NAME"
    if [ $status -ne 0 ]; then
        die "Non-zero status of systemd unit $SERVICE_NAME, output: $output"
    fi

    run systemctl stop "$SERVICE_NAME"
    if [ $status -ne 0 ]; then
        die "Error stopping systemd unit $SERVICE_NAME, output: $output"
    fi
}

# vim: filetype=sh
