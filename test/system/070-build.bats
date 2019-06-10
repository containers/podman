#!/usr/bin/env bats   -*- bats -*-
#
# Tests for podman build
#

load helpers

@test "podman build - basic test" {
    if [[ "$PODMAN" =~ -remote ]]; then
        if [ "$(id -u)" -ne 0 ]; then
            skip "unreliable with podman-remote and rootless; #2972"
        fi
    fi

    rand_filename=$(random_string 20)
    rand_content=$(random_string 50)

    tmpdir=$PODMAN_TMPDIR/build-test
    run mkdir -p $tmpdir || die "Could not mkdir $tmpdir"
    dockerfile=$tmpdir/Dockerfile
    cat >$dockerfile <<EOF
FROM $IMAGE
RUN echo $rand_content > /$rand_filename
EOF

    run_podman build -t build_test --format=docker $tmpdir

    run_podman run --rm build_test cat /$rand_filename
    is "$output"   "$rand_content"   "reading generated file in image"

    run_podman rmi build_test
}

# vim: filetype=sh
