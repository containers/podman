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

    # The 'apk' command can take a long time to fetch files; bump timeout
    PODMAN_TIMEOUT=240 run_podman build -t build_test --format=docker $tmpdir
    is "$output" ".*STEP 4: COMMIT" "COMMIT seen in log"

    run_podman run --rm build_test cat /$rand_filename
    is "$output"   "$rand_content"   "reading generated file in image"

    run_podman rmi -f build_test
}

function teardown() {
    # A timeout or other error in 'build' can leave behind stale images
    # that podman can't even see and which will cascade into subsequent
    # test failures. Try a last-ditch force-rm in cleanup, ignoring errors.
    run_podman '?' rm -a -f
    run_podman '?' rmi -f build_test

    basic_teardown
}

# vim: filetype=sh
