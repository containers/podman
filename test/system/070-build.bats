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

@test "podman build - set runtime" {
    skip_if_remote "--runtime flag not supported for remote"
    # Test on the CLI and via containers.conf

    tmpdir=$PODMAN_TMPDIR/build-test
    run mkdir -p $tmpdir
    containerfile=$tmpdir/Containerfile
    cat >$containerfile <<EOF
FROM $IMAGE
RUN echo $rand_content
EOF

    run_podman 125 --runtime=idonotexist build -t build_test $tmpdir
    is "$output" ".*\"idonotexist\" not found.*" "failed when passing invalid OCI runtime via CLI"

    containersconf=$tmpdir/containers.conf
    cat >$containersconf <<EOF
[engine]
runtime="idonotexist"
EOF

    CONTAINERS_CONF="$containersconf" run_podman 125 build -t build_test $tmpdir
    is "$output" ".*\"idonotexist\" not found.*" "failed when passing invalid OCI runtime via containers.conf"
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

    # For overriding with --env-file; using multiple files confirms that
    # the --env-file option is cumulative, not last-one-wins.
    cat >$PODMAN_TMPDIR/env-file1 <<EOF
MYENV3=$s_env3
http_proxy=http-proxy-in-env-file
EOF
    cat >$PODMAN_TMPDIR/env-file2 <<EOF
https_proxy=https-proxy-in-env-file
EOF

    # NOTE: it's important to not create the workdir.
    # Podman will make sure to create a missing workdir
    # if needed. See #9040.
    cat >$tmpdir/Containerfile <<EOF
FROM $IMAGE
LABEL $label_name=$label_value
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


    if is_remote; then
        ENVHOST=""
    else
	ENVHOST="--env-host"
    fi

    # Run without args - should run the above script. Verify its output.
    export MYENV2="$s_env2"
    export MYENV3="env-file-should-override-env-host!"
    run_podman run --rm \
               --env-file=$PODMAN_TMPDIR/env-file1 \
               --env-file=$PODMAN_TMPDIR/env-file2 \
               ${ENVHOST} \
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
              run_podman run --rm \
              --env-file=$PODMAN_TMPDIR/env-file1 \
              --env-file=$PODMAN_TMPDIR/env-file2 \
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

    # Determine buildah version, so we can confirm it gets into Labels
    # Multiple --format options confirm command-line override (last one wins)
    run_podman info --format '{{.Ignore}}' --format '{{ .Host.BuildahVersion }}'
    is "$output" "[1-9][0-9.-]\+" ".Host.BuildahVersion is reasonable"
    buildah_version=$output

    # Confirm that 'podman inspect' shows the expected values
    # FIXME: can we rely on .Env[0] being PATH, and the rest being in order??
    run_podman image inspect build_test

    # (Assert that output is formatted, not a one-line blob: #8011)
    if [[ "${#lines[*]}" -lt 10 ]]; then
        die "Output from 'image inspect' is only ${#lines[*]} lines; see #8011"
    fi

    tests="
Env[1]             | MYENV1=$s_env1
Env[2]             | MYENV2=this-should-be-overridden-by-env-host
Env[3]             | MYENV3=this-should-be-overridden-by-env-file
Env[4]             | MYENV4=this-should-be-overridden-by-cmdline
Cmd[0]             | /bin/mydefaultcmd
Cmd[1]             | $s_echo
WorkingDir         | $workdir
Labels.$label_name | $label_value
Labels.\"io.buildah.version\" | $buildah_version
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

    # FIXME: 'image tree --whatrequires' does not work via remote
    if ! is_remote; then
        run_podman image tree --whatrequires $IMAGE
        is "${lines[-1]}" \
           ".*ID: .* Top Layer of: \\[localhost/build_test:latest\\]" \
           "'image tree --whatrequires' shows our built image"
    fi

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

# #8092 - podman build should not gobble stdin (Fixes: #8066)
@test "podman build - does not gobble stdin that does not belong to it" {
    random1=random1-$(random_string 12)
    random2=random2-$(random_string 15)
    random3=random3-$(random_string 12)

    tmpdir=$PODMAN_TMPDIR/build-test
    mkdir -p $tmpdir
    cat >$tmpdir/Containerfile <<EOF
FROM $IMAGE
RUN echo x${random2}y
EOF

    # This is a little rococo, bear with me please. #8092 fixed a bug
    # in which 'podman build' would slurp up any input in the pipeline.
    # Not a problem in a contrived example such as the one below, but
    # definitely a problem when running commands in a pipeline to bash:
    # all commands after 'podman build' would silently be ignored.
    # In the test below, prior to #8092, the 'sed' would not get
    # any input, and we would never see $random3 in the output.
    # And, we use 'sed' to massage $random3 just on the remote
    # chance that podman itself could pass stdin through.
    results=$(echo $random3 | (
                  echo $random1
                  run_podman build -t build_test $tmpdir
                  sed -e 's/^/a/' -e 's/$/z/'
              ))

    # First simple test: confirm that we see the piped-in string, as
    # massaged by sed. This fails in 287edd4e2, the commit before #8092.
    # We do this before the thorough test (below) because, should it
    # fail, the diagnostic is much clearer and easier to understand.
    is "$results" ".*a${random3}z" "stdin remains after podman-build"

    # More thorough test: verify all the required strings in order.
    # This is unlikely to fail, but it costs us nothing and could
    # catch a regression somewhere else.
    # FIXME: podman-remote output differs from local: #8342 (spurious ^M)
    # FIXME: podman-remote output differs from local: #8343 (extra SHA output)
    remote_extra=""
    if is_remote; then remote_extra=".*";fi
    expect="${random1}
.*
STEP 1: FROM $IMAGE
STEP 2: RUN echo x${random2}y
x${random2}y${remote_extra}
STEP 3: COMMIT build_test${remote_extra}
--> [0-9a-f]\{11\}
[0-9a-f]\{64\}
a${random3}z"

    is "$results" "$expect" "Full output from 'podman build' pipeline"

    run_podman rmi -f build_test
}

@test "podman build --layers test" {
    rand_content=$(random_string 50)
    tmpdir=$PODMAN_TMPDIR/build-test
    run mkdir -p $tmpdir
    containerfile=$tmpdir/Containerfile
    cat >$containerfile <<EOF
FROM $IMAGE
RUN echo $rand_content
EOF

    # Build twice to make sure second time uses cache
    run_podman build -t build_test $tmpdir
    if [[ "$output" =~ "Using cache" ]]; then
        is "$output" "[no instance of 'Using cache']" "no cache used"
    fi

    run_podman build -t build_test $tmpdir
    is "$output" ".*cache" "used cache"

    run_podman build -t build_test --layers=true $tmpdir
    is "$output" ".*cache" "used cache"

    run_podman build -t build_test --layers=false $tmpdir
    if [[ "$output" =~ "Using cache" ]]; then
        is "$output" "[no instance of 'Using cache']" "no cache used"
    fi

    BUILDAH_LAYERS=false run_podman build -t build_test $tmpdir
    if [[ "$output" =~ "Using cache" ]]; then
        is "$output" "[no instance of 'Using cache']" "no cache used"
    fi

    BUILDAH_LAYERS=false run_podman build -t build_test --layers=1 $tmpdir
    is "$output" ".*cache" "used cache"

    BUILDAH_LAYERS=1 run_podman build -t build_test --layers=false $tmpdir
    if [[ "$output" =~ "Using cache" ]]; then
        is "$output" "[no instance of 'Using cache']" "no cache used"
    fi

    run_podman rmi -a --force
}

@test "podman build --logfile test" {
    tmpdir=$PODMAN_TMPDIR/build-test
    mkdir -p $tmpdir
    tmpbuilddir=$tmpdir/build
    mkdir -p $tmpbuilddir
    dockerfile=$tmpbuilddir/Dockerfile
    cat >$dockerfile <<EOF
FROM $IMAGE
EOF

    run_podman build -t build_test --format=docker --logfile=$tmpdir/logfile $tmpbuilddir
    run cat $tmpdir/logfile
    is "$output" ".*STEP 2: COMMIT" "COMMIT seen in log"

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
