#!/bin/bash

set -e

SCRIPTDIR=$(realpath $(dirname "$0"))
REPODIR=$(realpath $SCRIPTDIR/../)
PODMAN_REMOTE=$(type -P podman-remote || echo "$REPODIR/bin/podman-remote")
DESTDIR="$1"

die() {
    echo "${1:-No error message specified}" > /dev/stderr
    exit 1
}

subcommand_list() {
    if $PODMAN_REMOTE $1 --help &> /dev/null
    then
        $PODMAN_REMOTE $1 --help | \
            sed -n '/^Available Commands:/,/^Flags:/p' | \
            sed -e '1d;$d' -e '/^$/d' | awk '{print $1}'
    # else ignore subcommand w/o sub-subcommand
    fi
}

install_remote_docs() {
    local cmd
    for cmd in $(subcommand_list $1)
    do
        install -v ./podman-$cmd.1 "$DESTDIR" 2>/dev/null || \
            install -v $SCRIPTDIR/links/podman-$cmd.1 "$DESTDIR"
    done
}

[[ -n "$DESTDIR" ]] || \
    die "Must specify path to install remote docs as first argument"

[[ -d "$DESTDIR" ]] || \
    die "Not a directory: $1"

[[ -x "$PODMAN_REMOTE" ]] || \
    die "The podman-remote executable could not be found: $PODMAN_REMOTE"

cd "$REPODIR/docs"

for subcommand in "" $(subcommand_list)
do
    install_remote_docs $subcommand
done
