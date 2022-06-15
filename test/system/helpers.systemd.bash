# -*- bash -*-
#
# BATS helpers for systemd-related functionality
#

# podman initializes this if unset, but systemctl doesn't
if [ -z "$XDG_RUNTIME_DIR" ]; then
    if is_rootless; then
        export XDG_RUNTIME_DIR=/run/user/$(id -u)
    fi
fi

# For tests which write systemd unit files
UNIT_DIR="/run/systemd/system"
_DASHUSER=
if is_rootless; then
    UNIT_DIR="${XDG_RUNTIME_DIR}/systemd/user"
    # Why isn't systemd smart enough to figure this out on its own?
    _DASHUSER="--user"
fi

mkdir -p $UNIT_DIR

systemctl() {
    command systemctl $_DASHUSER "$@"
}

journalctl() {
    command journalctl $_DASHUSER "$@"
}

systemd-run() {
    command systemd-run $_DASHUSER "$@";
}
