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

# Regression from v1.5.0. This test passes fine in v1.5.0, fails in 1.6
@test "podman build - cache (#3920)" {
    if is_remote && is_rootless; then
        skip "unreliable with podman-remote and rootless; #2972"
    fi

    # Make an empty test directory, with a subdirectory used for tar
    tmpdir=$PODMAN_TMPDIR/build-test
    run mkdir -p $tmpdir/subtest || die "Could not mkdir $tmpdir/subtest"

    echo "This is the ORIGINAL file" > $tmpdir/subtest/myfile1
    run tar -C $tmpdir -cJf $tmpdir/myfile.tar.xz subtest

    cat >$tmpdir/Dockerfile <<EOF
FROM $IMAGE
ADD myfile.tar.xz /
EOF

    # One of: ADD myfile /myfile or COPY . .
    run_podman build  -t build_test -f $tmpdir/Dockerfile $tmpdir
    is "$output" ".*STEP 3: COMMIT" "COMMIT seen in log"
    if [[ "$output" =~ "Using cache" ]]; then
        is "$output" "[no instance of 'Using cache']" "no cache used"
    fi
    iid=${lines[-1]}

    run_podman run --rm build_test cat /subtest/myfile1
    is "$output"   "This is the ORIGINAL file" "file contents, first time"

    # Step 2: Recreate the tarfile, with new content. Rerun podman build.
    echo "This is a NEW file" >| $tmpdir/subtest/myfile2
    run tar -C $tmpdir -cJf $tmpdir/myfile.tar.xz subtest

    run_podman build -t build_test -f $tmpdir/Dockerfile $tmpdir
    is "$output" ".*STEP 3: COMMIT" "COMMIT seen in log"

    # Since the tarfile is modified, podman SHOULD NOT use a cached layer.
    if [[ "$output" =~ "Using cache" ]]; then
        is "$output" "[no instance of 'Using cache']" "no cache used"
    fi

    # Pre-buildah-1906, this fails with ENOENT because the tarfile was cached
    run_podman run --rm build_test cat /subtest/myfile2
    is "$output"   "This is a NEW file" "file contents, second time"

    run_podman rmi -f build_test $iid
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
