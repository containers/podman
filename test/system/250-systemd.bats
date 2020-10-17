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
    basic_teardown
}

# This test can fail in dev. environment because of SELinux.
# quick fix: chcon -t container_runtime_exec_t ./bin/podman
@test "podman generate - systemd - basic" {
    # podman initializes this if unset, but systemctl doesn't
    if is_rootless; then
        if [ -z "$XDG_RUNTIME_DIR" ]; then
            export XDG_RUNTIME_DIR=/run/user/$(id -u)
        fi
    fi

    cname=$(random_string)
    # See #7407 for --pull=always.
    run_podman create --pull=always --name $cname --label "io.containers.autoupdate=image" $IMAGE top

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
    run $SYSTEMCTL stop "$SERVICE_NAME"
    if [ $status -ne 0 ]; then
        die "Error stopping systemd unit $SERVICE_NAME, output: $output"
    fi

    rm -f "$UNIT_FILE"
    $SYSTEMCTL daemon-reload
}

# vim: filetype=sh
