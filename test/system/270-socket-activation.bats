#!/usr/bin/env bats   -*- bats -*-
#
# Tests podman system service under systemd socket activation
#

load helpers
load helpers.systemd

SERVICE_NAME="podman_test_$(random_string)"

SERVICE_SOCK_ADDR="/run/podman/podman.sock"
if is_rootless; then
    SERVICE_SOCK_ADDR="$XDG_RUNTIME_DIR/podman/podman.sock"
fi

SERVICE_FILE="$UNIT_DIR/$SERVICE_NAME.service"
SOCKET_FILE="$UNIT_DIR/$SERVICE_NAME.socket"


function setup() {
    skip_if_remote "systemd tests are meaningless over remote"

    basic_setup

    cat > $SERVICE_FILE <<EOF
[Unit]
Description=Podman API Service
Requires=podman.socket
After=podman.socket
Documentation=man:podman-system-service(1)
StartLimitIntervalSec=0

[Service]
Type=exec
KillMode=process
Environment=LOGGING="--log-level=info"
ExecStart=$PODMAN $LOGGING system service -t 2
EOF
    cat > $SOCKET_FILE <<EOF
[Unit]
Description=Podman API Socket
Documentation=man:podman-system-service(1)

[Socket]
ListenStream=%t/podman/podman.sock
SocketMode=0660

[Install]
WantedBy=sockets.target
EOF

    # ensure pause die before each test runs
    if is_rootless; then
        local pause_pid="$XDG_RUNTIME_DIR/libpod/tmp/pause.pid"
        if [ -f $pause_pid ]; then
            kill -9 $(cat $pause_pid) 2> /dev/null
            rm -f $pause_pid
        fi
    fi
    systemctl start "$SERVICE_NAME.socket"
}

function teardown() {
    systemctl stop "$SERVICE_NAME.socket"
    rm -f "$SERVICE_FILE" "$SOCKET_FILE"
    systemctl daemon-reload
    basic_teardown
}

@test "podman system service - socket activation - no container" {
    run curl -s --max-time 3 --unix-socket $SERVICE_SOCK_ADDR http://podman/libpod/_ping
    is "$output" "OK" "podman service responses normally"
}

@test "podman system service - socket activation - exist container " {
    run_podman run $IMAGE sleep 90
    run curl -s --max-time 3 --unix-socket $SERVICE_SOCK_ADDR http://podman/libpod/_ping
    is "$output" "OK" "podman service responses normally"
}

@test "podman system service - socket activation - kill rootless pause " {
    if ! is_rootless; then
        skip "root podman no need pause process"
    fi
    run_podman run $IMAGE sleep 90
    local pause_pid="$XDG_RUNTIME_DIR/libpod/tmp/pause.pid"
    if [ -f $pause_pid ]; then
        kill -9 $(cat $pause_pid) 2> /dev/null
    fi
    run curl -s --max-time 3 --unix-socket $SERVICE_SOCK_ADDR http://podman/libpod/_ping
    is "$output" "OK" "podman service responses normally"
}

# vim: filetype=sh
