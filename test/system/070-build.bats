#!/usr/bin/env bats   -*- bats -*-
#
# Tests for podman build
#

load helpers

@test "podman build - basic test" {
    if is_remote && is_rootless; then
        skip "unreliable with podman-remote and rootless; #2972"
    fi

    rand_filename=$(random_string 20)
    rand_content=$(random_string 50)

    tmpdir=$PODMAN_TMPDIR/build-test
    run mkdir -p $tmpdir || die "Could not mkdir $tmpdir"
    dockerfile=$tmpdir/Dockerfile
    cat >$dockerfile <<EOF
FROM $IMAGE
RUN apk add nginx
RUN echo $rand_content > /$rand_filename
EOF

    run_podman build -t build_test --format=docker $tmpdir
    is "$output" ".*STEP 4: COMMIT" "COMMIT seen in log"

    run_podman run --rm build_test cat /$rand_filename
    is "$output"   "$rand_content"   "reading generated file in image"

    run_podman rmi build_test
}
# vim: filetype=sh
