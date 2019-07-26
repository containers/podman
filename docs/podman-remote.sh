#!/bin/sh

BREWDIR=$1
mkdir -p $BREWDIR
docs() {
[ -z $1 ] || type="-$1"
for i in $(podman-remote $1 --help | sed -n '/^Available Commands:/,/^Flags:/p'| sed -e '1d;$d' -e '/^$/d' | awk '{print $1}'); do install podman$type-$i.1 $BREWDIR 2>/dev/null || install links/podman$type-$i.1 $BREWDIR; done
}
docs

for cmd in 'container image pod volume'; do docs $cmd; done
