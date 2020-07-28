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
    mkdir -p $tmpdir/subtest || die "Could not mkdir $tmpdir/subtest"

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

@test "podman build - URLs" {
    tmpdir=$PODMAN_TMPDIR/build-test
    mkdir -p $tmpdir

    cat >$tmpdir/Dockerfile <<EOF
FROM $IMAGE
ADD https://github.com/containers/podman/blob/master/README.md /tmp/
EOF
    run_podman build -t add_url $tmpdir
    run_podman run --rm add_url stat /tmp/README.md
    run_podman rmi -f add_url

    # Now test COPY. That should fail.
    sed -i -e 's/ADD/COPY/' $tmpdir/Dockerfile
    run_podman 125 build -t copy_url $tmpdir
    is "$output" ".*error building at STEP .*: source can't be a URL for COPY"
}


@test "podman build - workdir, cmd, env, label" {
    tmpdir=$PODMAN_TMPDIR/build-test
    mkdir -p $tmpdir

    # Random workdir, and multiple random strings to verify command & env
    workdir=/$(random_string 10)
    s_echo=$(random_string 15)
    s_env1=$(random_string 20)
    s_env2=$(random_string 25)
    s_env3=$(random_string 30)

    # Label name: make sure it begins with a letter! jq barfs if you
    # try to ask it for '.foo.<N>xyz', i.e. any string beginning with digit
    label_name=l$(random_string 8)
    label_value=$(random_string 12)

    # Command to run on container startup with no args
    cat >$tmpdir/mycmd <<EOF
#!/bin/sh
pwd
echo "\$1"
echo "\$MYENV1"
echo "\$MYENV2"
echo "\$MYENV3"
EOF

    cat >$tmpdir/Containerfile <<EOF
FROM $IMAGE
LABEL $label_name=$label_value
RUN mkdir $workdir
WORKDIR $workdir
ENV MYENV1=$s_env1
ENV MYENV2 $s_env2
ENV MYENV3 this-should-be-overridden
ADD mycmd /bin/mydefaultcmd
RUN chmod 755 /bin/mydefaultcmd
CMD ["/bin/mydefaultcmd","$s_echo"]
EOF

    # cd to the dir, so we test relative paths (important for podman-remote)
    cd $PODMAN_TMPDIR
    run_podman build -t build_test -f build-test/Containerfile build-test

    # Run without args - should run the above script. Verify its output.
    run_podman run --rm -e MYENV3="$s_env3" build_test
    is "${lines[0]}" "$workdir" "container default command: pwd"
    is "${lines[1]}" "$s_echo"  "container default command: output from echo"
    is "${lines[2]}" "$s_env1"  "container default command: env1"
    is "${lines[3]}" "$s_env2"  "container default command: env2"
    is "${lines[4]}" "$s_env3"  "container default command: env3 (from cmdline)"

    # test that workdir is set for command-line commands also
    run_podman run --rm build_test pwd
    is "$output" "$workdir" "pwd command in container"

    # Confirm that 'podman inspect' shows the expected values
    # FIXME: can we rely on .Env[0] being PATH, and the rest being in order??
    run_podman image inspect build_test
    tests="
Env[1]             | MYENV1=$s_env1
Env[2]             | MYENV2=$s_env2
Env[3]             | MYENV3=this-should-be-overridden
Cmd[0]             | /bin/mydefaultcmd
Cmd[1]             | $s_echo
WorkingDir         | $workdir
Labels.$label_name | $label_value
"

    parse_table "$tests" | while read field expect; do
        actual=$(jq -r ".[0].Config.$field" <<<"$output")
        dprint "# actual=<$actual> expect=<$expect}>"
        is "$actual" "$expect" "jq .Config.$field"
    done

    # Clean up
    run_podman rmi -f build_test
}

@test "podman build - stdin test" {
    if is_remote && is_rootless; then
        skip "unreliable with podman-remote and rootless; #2972"
    fi

    # Random workdir, and multiple random strings to verify command & env
    workdir=/$(random_string 10)
    PODMAN_TIMEOUT=240 run_podman build -t build_test - << EOF
FROM  $IMAGE
RUN mkdir $workdir
WORKDIR $workdir
RUN /bin/echo 'Test'
EOF
    is "$output" ".*STEP 5: COMMIT" "COMMIT seen in log"

    run_podman run --rm build_test pwd
    is "$output" "$workdir" "pwd command in container"

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
