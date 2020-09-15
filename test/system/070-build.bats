#!/usr/bin/env bats   -*- bats -*-
# shellcheck disable=SC2096
#
# Tests for podman build
#

load helpers

@test "podman build - basic test" {
    rand_filename=$(random_string 20)
    rand_content=$(random_string 50)

    tmpdir=$PODMAN_TMPDIR/build-test
    mkdir -p $tmpdir
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

@test "podman build - global runtime flags test" {
    skip_if_remote "--runtime-flag flag not supported for remote"

    rand_content=$(random_string 50)

    tmpdir=$PODMAN_TMPDIR/build-test
    run mkdir -p $tmpdir
    containerfile=$tmpdir/Containerfile
    cat >$containerfile <<EOF
FROM $IMAGE
RUN echo $rand_content
EOF

    run_podman 125 --runtime-flag invalidflag build -t build_test $tmpdir
    is "$output" ".*invalidflag" "failed when passing undefined flags to the runtime"
}

# Regression from v1.5.0. This test passes fine in v1.5.0, fails in 1.6
@test "podman build - cache (#3920)" {
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
    s_env4=$(random_string 40)

    # Label name: make sure it begins with a letter! jq barfs if you
    # try to ask it for '.foo.<N>xyz', i.e. any string beginning with digit
    label_name=l$(random_string 8)
    label_value=$(random_string 12)

    # Command to run on container startup with no args
    cat >$tmpdir/mycmd <<EOF
#!/bin/sh
PATH=/usr/bin:/bin
pwd
echo "\$1"
printenv | grep MYENV | sort | sed -e 's/^MYENV.=//'
EOF

    # For overriding with --env-file
    cat >$PODMAN_TMPDIR/env-file <<EOF
MYENV3=$s_env3
http_proxy=http-proxy-in-env-file
https_proxy=https-proxy-in-env-file
EOF

    cat >$tmpdir/Containerfile <<EOF
FROM $IMAGE
LABEL $label_name=$label_value
RUN mkdir $workdir
WORKDIR $workdir

# Test for #7094 - chowning of invalid symlinks
RUN mkdir -p /a/b/c
RUN ln -s /no/such/nonesuch /a/b/c/badsymlink
RUN ln -s /bin/mydefaultcmd /a/b/c/goodsymlink
RUN touch /a/b/c/myfile
RUN chown -h 1:2 /a/b/c/badsymlink /a/b/c/goodsymlink && chown -h 4:5 /a/b/c/myfile
VOLUME /a/b/c

# Test for environment passing and override
ENV MYENV1=$s_env1
ENV MYENV2 this-should-be-overridden-by-env-host
ENV MYENV3 this-should-be-overridden-by-env-file
ENV MYENV4 this-should-be-overridden-by-cmdline
ENV http_proxy http-proxy-in-image
ENV ftp_proxy  ftp-proxy-in-image
ADD mycmd /bin/mydefaultcmd
RUN chmod 755 /bin/mydefaultcmd
RUN chown 2:3 /bin/mydefaultcmd
CMD ["/bin/mydefaultcmd","$s_echo"]
EOF

    # cd to the dir, so we test relative paths (important for podman-remote)
    cd $PODMAN_TMPDIR
    run_podman build -t build_test -f build-test/Containerfile build-test
    local iid="${lines[-1]}"

    # Run without args - should run the above script. Verify its output.
    export MYENV2="$s_env2"
    export MYENV3="env-file-should-override-env-host!"
    run_podman run --rm \
               --env-file=$PODMAN_TMPDIR/env-file \
               --env-host \
               -e MYENV4="$s_env4" \
               build_test
    is "${lines[0]}" "$workdir" "container default command: pwd"
    is "${lines[1]}" "$s_echo"  "container default command: output from echo"

    is "${lines[2]}" "$s_env1"  "container default command: env1"

    if is_remote; then
        is "${lines[3]}" "this-should-be-overridden-by-env-host" "podman-remote does not send local environment"
    else
        is "${lines[3]}" "$s_env2" "container default command: env2"
    fi

    is "${lines[4]}" "$s_env3"  "container default command: env3 (from envfile)"
    is "${lines[5]}" "$s_env4"  "container default command: env4 (from cmdline)"

    # Proxies - environment should override container, but not env-file
    http_proxy=http-proxy-from-env  ftp_proxy=ftp-proxy-from-env \
              run_podman run --rm --env-file=$PODMAN_TMPDIR/env-file \
              build_test \
              printenv http_proxy https_proxy ftp_proxy
    is "${lines[0]}" "http-proxy-in-env-file"  "env-file overrides env"
    is "${lines[1]}" "https-proxy-in-env-file" "env-file sets proxy var"

    if is_remote; then
        is "${lines[2]}" "ftp-proxy-in-image" "podman-remote does not send local environment"
    else
        is "${lines[2]}" "ftp-proxy-from-env" "ftp-proxy is passed through"
    fi

    # test that workdir is set for command-line commands also
    run_podman run --rm build_test pwd
    is "$output" "$workdir" "pwd command in container"

    # Confirm that 'podman inspect' shows the expected values
    # FIXME: can we rely on .Env[0] being PATH, and the rest being in order??
    run_podman image inspect build_test
    tests="
Env[1]             | MYENV1=$s_env1
Env[2]             | MYENV2=this-should-be-overridden-by-env-host
Env[3]             | MYENV3=this-should-be-overridden-by-env-file
Env[4]             | MYENV4=this-should-be-overridden-by-cmdline
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

    # Bad symlink in volume. Prior to #7094, well, we wouldn't actually
    # get here because any 'podman run' on a volume that had symlinks,
    # be they dangling or valid, would barf with
    #    Error: chown <mountpath>/_data/symlink: ENOENT
    run_podman run --rm build_test stat -c'%u:%g:%N' /a/b/c/badsymlink
    is "$output" "1:2:'/a/b/c/badsymlink' -> '/no/such/nonesuch'" \
       "bad symlink to nonexistent file is chowned and preserved"

    run_podman run --rm build_test stat -c'%u:%g:%N' /a/b/c/goodsymlink
    is "$output" "1:2:'/a/b/c/goodsymlink' -> '/bin/mydefaultcmd'" \
       "good symlink to existing file is chowned and preserved"

    run_podman run --rm build_test stat -c'%u:%g' /bin/mydefaultcmd
    is "$output" "2:3" "target of symlink is not chowned"

    run_podman run --rm build_test stat -c'%u:%g:%N' /a/b/c/myfile
    is "$output" "4:5:/a/b/c/myfile" "file in volume is chowned"

    # Hey, as long as we have an image with lots of layers, let's
    # confirm that 'image tree' works as expected
    run_podman image tree build_test
    is "${lines[0]}" "Image ID: ${iid:0:12}" \
       "image tree: first line"
    is "${lines[1]}" "Tags:     \[localhost/build_test:latest]" \
       "image tree: second line"
    is "${lines[2]}" "Size:     [0-9.]\+[kM]B" \
       "image tree: third line"
    is "${lines[3]}" "Image Layers" \
       "image tree: fourth line"
    is "${lines[4]}"  "...  ID: [0-9a-f]\{12\} Size: .* Top Layer of: \[$IMAGE]" \
       "image tree: first layer line"
    is "${lines[-1]}" "...  ID: [0-9a-f]\{12\} Size: .* Top Layer of: \[localhost/build_test:latest]" \
       "image tree: last layer line"

    # Clean up
    run_podman rmi -f build_test
}

@test "podman build - stdin test" {
    # Random workdir, and random string to verify build output
    workdir=/$(random_string 10)
    random_echo=$(random_string 15)
    PODMAN_TIMEOUT=240 run_podman build -t build_test - << EOF
FROM  $IMAGE
RUN mkdir $workdir
WORKDIR $workdir
RUN /bin/echo $random_echo
EOF
    is "$output" ".*STEP 5: COMMIT" "COMMIT seen in log"
    is "$output" ".*STEP .: RUN /bin/echo $random_echo"

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
