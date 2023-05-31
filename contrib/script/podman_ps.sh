#!/bin/sh

logins() {
    ps --no-heading -C conmon -o user | sort -u
}

podman_ps_user() {
    name=$1
    shift
    args="$@"
    machinectl shell --quiet $name@ /bin/sh -c "echo User: $name; /usr/bin/podman ps $args"
}

if [ "$EUID" -ne 0 ];then
    echo "The $0 script must be executaed as root"
    exit 1
fi

for name in $(logins); do podman_ps_user $name "$@"; done
