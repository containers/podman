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
    timeout --foreground -v --kill=10 $PODMAN_TIMEOUT systemctl $_DASHUSER "$@"
}

journalctl() {
    timeout --foreground -v --kill=10 $PODMAN_TIMEOUT journalctl $_DASHUSER "$@"
}

systemd-run() {
    timeout --foreground -v --kill=10 $PODMAN_TIMEOUT systemd-run $_DASHUSER "$@";
}

install_kube_template() {
    # If running from a podman source directory, build and use the source
    # version of the play-kube-@ unit file
    unit_name="podman-kube@.service"
    unit_file="contrib/systemd/system/${unit_name}"
    if [[ -e ${unit_file}.in ]]; then
        echo "# [Building & using $unit_name from source]" >&3
        # Force regenerating unit file (existing one may have /usr/bin path)
        rm -f $unit_file
        BINDIR=$(dirname $PODMAN) make $unit_file
        cp $unit_file $UNIT_DIR/$unit_name
    fi
}

quadlet_to_service_name() {
    local filename=$(basename -- "$1")
    local extension="${filename##*.}"
    local filename="${filename%.*}"
    local suffix=""

    if [ "$extension" == "volume" ]; then
        suffix="-volume"
    elif [ "$extension" == "network" ]; then
        suffix="-network"
    elif [ "$extension" == "image" ]; then
        suffix="-image"
    fi

    echo "$filename$suffix.service"
}

setup_firewalld_services() {
    unit_names=("podman-firewalld-reload.service" "podman-firewalld-restart.service")

    for unit_name in "${unit_names[@]}"; do
        unit_file="contrib/systemd/system/${unit_name}"

        if [[ -e $unit_file.in ]]; then
            echo "# [Building & using $unit_name from source]" >&3
            # Force regenerating unit file (existing one may have /usr/bin path)
            rm -f "$unit_file"
            BINDIR=$(dirname "$PODMAN") make "$unit_file"
            cp "$unit_file" "$UNIT_DIR/$unit_name"
        fi
    done
}
