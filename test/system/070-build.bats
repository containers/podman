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
    is "$output" ".*COMMIT" "COMMIT seen in log"

    run_podman run --rm build_test cat /$rand_filename
    is "$output"   "$rand_content"   "reading generated file in image"

    run_podman rmi -f build_test
}

@test "podman buildx - basic test" {
    rand_filename=$(random_string 20)
    rand_content=$(random_string 50)

    tmpdir=$PODMAN_TMPDIR/build-test
    mkdir -p $tmpdir
    dockerfile=$tmpdir/Dockerfile
    cat >$dockerfile <<EOF
FROM $IMAGE
RUN echo $rand_content > /$rand_filename
VOLUME /a/b/c
VOLUME ['/etc/foo', '/etc/bar']
EOF

    run_podman buildx build --load -t build_test --format=docker $tmpdir
    is "$output" ".*COMMIT" "COMMIT seen in log"

    run_podman run --rm build_test cat /$rand_filename
    is "$output"   "$rand_content"   "reading generated file in image"

    # Make sure the volumes are created at surprising yet Docker-compatible
    # destinations (see bugzilla.redhat.com/show_bug.cgi?id=2014149).
    run_podman run --rm build_test find /[ /etc/bar\] -print
    is "$output" "/\[
/\[/etc
/\[/etc/foo,
/etc/bar]" "weird VOLUME gets converted to directories with brackets and comma"

    # Now confirm that each volume got a unique device ID
    run_podman run --rm build_test stat -c '%D' / /a /a/b /a/b/c /\[ /\[/etc /\[/etc/foo, /etc /etc/bar\]
    # First, the non-volumes should all be the same...
    is "${lines[0]}" "${lines[1]}" "devnum( / ) = devnum( /a )"
    is "${lines[0]}" "${lines[2]}" "devnum( / ) = devnum( /a/b )"
    is "${lines[0]}" "${lines[4]}" "devnum( / ) = devnum( /[ )"
    is "${lines[0]}" "${lines[5]}" "devnum( / ) = devnum( /[etc )"
    is "${lines[0]}" "${lines[7]}" "devnum( / ) = devnum( /etc )"
    is "${lines[6]}" "${lines[8]}" "devnum( /[etc/foo, ) = devnum( /etc/bar] )"
    # ...then, each volume should be different
    if [[ "${lines[0]}" = "${lines[3]}" ]]; then
        die "devnum( / ) (${lines[0]}) = devnum( volume0 ) (${lines[3]}) -- they should differ"
    fi
    if [[ "${lines[0]}" = "${lines[6]}" ]]; then
        die "devnum( / ) (${lines[0]}) = devnum( volume1 ) (${lines[6]}) -- they should differ"
    fi
    # FIXME: is this expected? I thought /a/b/c and /[etc/foo, would differ
    is "${lines[3]}" "${lines[6]}" "devnum( volume0 ) = devnum( volume1 )"

    run_podman rmi -f build_test
}

@test "podman build test -f -" {
    rand_filename=$(random_string 20)
    rand_content=$(random_string 50)

    tmpdir=$PODMAN_TMPDIR/build-test
    mkdir -p $tmpdir
    containerfile=$PODMAN_TMPDIR/Containerfile
    cat >$containerfile <<EOF
FROM $IMAGE
RUN echo $rand_content > /$rand_filename
EOF

    run_podman build -t build_test -f - --format=docker $tmpdir < $containerfile
    is "$output" ".*COMMIT" "COMMIT seen in log"

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

    run_podman 1 --runtime-flag invalidflag build -t build_test $tmpdir
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
    is "$output" ".*COMMIT" "COMMIT seen in log"
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
    is "$output" ".*COMMIT" "COMMIT seen in log"

    # Since the tarfile is modified, podman SHOULD NOT use a cached layer.
    if [[ "$output" =~ "Using cache" ]]; then
        is "$output" "[no instance of 'Using cache']" "no cache used"
    fi

    # Pre-buildah-1906, this fails with ENOENT because the tarfile was cached
    run_podman run --rm build_test cat /subtest/myfile2
    is "$output"   "This is a NEW file" "file contents, second time"

    run_podman rmi -f build_test $iid
}

@test "podman build test -f ./relative" {
    rand_filename=$(random_string 20)
    rand_content=$(random_string 50)

    tmpdir=$PODMAN_TMPDIR/build-test
    mkdir -p $tmpdir
    mkdir -p $PODMAN_TMPDIR/reldir

    containerfile=$PODMAN_TMPDIR/reldir/Containerfile
    cat >$containerfile <<EOF
FROM $IMAGE
RUN echo $rand_content > /$rand_filename
EOF

    cd $PODMAN_TMPDIR
    run_podman build -t build_test -f ./reldir/Containerfile --format=docker $tmpdir
    is "$output" ".*COMMIT" "COMMIT seen in log"

    run_podman run --rm build_test cat /$rand_filename
    is "$output"   "$rand_content"   "reading generated file in image"

    run_podman rmi -f build_test
}

@test "podman build - URLs" {
    tmpdir=$PODMAN_TMPDIR/build-test
    mkdir -p $tmpdir

    cat >$tmpdir/Dockerfile <<EOF
FROM $IMAGE
ADD https://github.com/containers/podman/blob/main/README.md /tmp/
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

    # #8679: Create a secrets directory, and mount it in the container
    # (can only test locally; podman-remote has no --default-mounts-file opt)
    MOUNTS_CONF=
    secret_contents="ceci nest pas un secret"
    CAT_SECRET="echo $secret_contents"
    if ! is_remote; then
        mkdir $tmpdir/secrets
        echo  $tmpdir/secrets:/run/secrets > $tmpdir/mounts.conf

        secret_filename=secretfile-$(random_string 20)
        secret_contents=shhh-$(random_string 30)-shhh
        echo $secret_contents >$tmpdir/secrets/$secret_filename

        MOUNTS_CONF=--default-mounts-file=$tmpdir/mounts.conf
        CAT_SECRET="cat /run/secrets/$secret_filename"
    fi

    # For --dns-search: a domain that is unlikely to exist
    local nosuchdomain=nx$(random_string 10).net

    # Command to run on container startup with no args
    cat >$tmpdir/mycmd <<EOF
#!/bin/sh
PATH=/usr/bin:/bin
pwd
echo "\$1"
printenv | grep MYENV | sort | sed -e 's/^MYENV.=//'
$CAT_SECRET
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

    # Build args: one explicit (foo=bar), one implicit (foo)
    local arg_implicit_value=implicit_$(random_string 15)
    local arg_explicit_value=explicit_$(random_string 15)

    # NOTE: it's important to not create the workdir.
    # Podman will make sure to create a missing workdir
    # if needed. See #9040.
    cat >$tmpdir/Containerfile <<EOF
FROM $IMAGE
ARG arg_explicit
ARG arg_implicit
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

RUN $CAT_SECRET

RUN echo explicit-build-arg=\$arg_explicit
RUN echo implicit-build-arg=\$arg_implicit

CMD ["/bin/mydefaultcmd","$s_echo"]
RUN cat /etc/resolv.conf
EOF

    # The goal is to test that a missing value will be inherited from
    # environment - but that can't work with remote, so for simplicity
    # just make it explicit in that case too.
    local build_arg_implicit="--build-arg arg_implicit"
    if is_remote; then
        build_arg_implicit+="=$arg_implicit_value"
    fi

    # cd to the dir, so we test relative paths (important for podman-remote)
    cd $PODMAN_TMPDIR
    export arg_explicit="THIS SHOULD BE OVERRIDDEN BY COMMAND LINE!"
    export arg_implicit=${arg_implicit_value}
    run_podman ${MOUNTS_CONF} build \
               --build-arg arg_explicit=${arg_explicit_value} \
               $build_arg_implicit \
               --dns-search $nosuchdomain \
               -t build_test -f build-test/Containerfile build-test
    local iid="${lines[-1]}"

    if [[ $output =~ missing.*build.argument ]]; then
        die "podman did not see the given --build-arg(s)"
    fi

    # Make sure 'podman build' had the secret mounted
    is "$output" ".*$secret_contents.*" "podman build has /run/secrets mounted"

    # --build-arg should be set, both via 'foo=bar' and via just 'foo' ($foo)
    is "$output" ".*explicit-build-arg=${arg_explicit_value}" \
       "--build-arg arg_explicit=explicit-value works"
    is "$output" ".*implicit-build-arg=${arg_implicit_value}" \
       "--build-arg arg_implicit works (inheriting from environment)"
    is "$output" ".*search $nosuchdomain" \
       "--dns-search added to /etc/resolv.conf"

    if is_remote; then
        ENVHOST=""
    else
	ENVHOST="--env-host"
    fi

    # Run without args - should run the above script. Verify its output.
    export MYENV2="$s_env2"
    export MYENV3="env-file-should-override-env-host!"
    run_podman ${MOUNTS_CONF} run --rm \
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

    is "${lines[6]}" "$secret_contents" \
       "Contents of /run/secrets/$secret_filename in container"

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
"
    # FIXME: 2021-02-24: Fixed in buildah #3036; re-enable this once podman
    #        vendors in a newer buildah!
    # Labels.\"io.buildah.version\" | $buildah_version

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
    is "${lines[4]}" ".* ID: [0-9a-f]\{12\} Size: .* Top Layer of: \[$IMAGE]" \
       "image tree: first layer line"
    is "${lines[-1]}"  ".* ID: [0-9a-f]\{12\} Size: .* Top Layer of: \[localhost/build_test:latest]" \
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

@test "podman build - COPY with ignore" {
    local tmpdir=$PODMAN_TMPDIR/build-test-$(random_string 10)
    mkdir -p $tmpdir/subdir{1,2}

    # Create a bunch of files. Declare this as an array to avoid duplication
    # because we iterate over that list below, checking for each file.
    # A leading "-" indicates that the file SHOULD NOT exist in the built image
    #
    # Weird side effect of Buildah 3486, relating to subdirectories and
    # wildcard patterns. See that PR for details, it's way too confusing
    # to explain in a comment.
    local -a files=(
        -test1 -test1.txt
         test2  test2.txt
          subdir1/sub1  subdir1/sub1.txt
         -subdir1/sub2 -subdir1/sub2.txt
          subdir1/sub3  subdir1/sub3.txt
         -subdir2/sub1 -subdir2/sub1.txt
         -subdir2/sub2 -subdir2/sub2.txt
         -subdir2/sub3 -subdir2/sub3.txt
         this-file-does-not-match-anything-in-ignore-file
         comment
    )
    for f in ${files[@]}; do
        # The magic '##-' strips off the '-' prefix
        echo "$f" > $tmpdir/${f##-}
    done

    # Directory that doesn't exist in the image; COPY should create it
    local newdir=/newdir-$(random_string 12)
    cat >$tmpdir/Containerfile <<EOF
FROM $IMAGE
COPY ./ $newdir/
EOF

    # Run twice: first with a custom --ignorefile, then with a default one.
    # This ordering is deliberate: if we were to run with .dockerignore
    # first, and forget to rm it, and then run with --ignorefile, _and_
    # there was a bug in podman where --ignorefile was a NOP (eg #9570),
    # the test might pass because of the existence of .dockerfile.
    for ignorefile in ignoreme-$(random_string 5) .dockerignore; do
        # Patterns to ignore. Mostly copied from buildah/tests/bud/dockerignore
        cat >$tmpdir/$ignorefile <<EOF
# comment
test*
!test2*
subdir1
subdir2
!*/sub1*
!subdir1/sub3*
EOF

        # Build an image. For .dockerignore
        local -a ignoreflag
	unset ignoreflag
        if [[ $ignorefile != ".dockerignore" ]]; then
            ignoreflag="--ignorefile $tmpdir/$ignorefile"
        fi
        run_podman build -t build_test ${ignoreflag} $tmpdir

        # Delete the ignore file! Otherwise, in the next iteration of the loop,
        # we could end up with an existing .dockerignore that invisibly
        # takes precedence over --ignorefile
        rm -f $tmpdir/$ignorefile

        # It would be much more readable, and probably safer, to iterate
        # over each file, running 'podman run ... ls -l $f'. But each podman run
        # takes a second or so, and we are mindful of each second.
        run_podman run --rm build_test find $newdir -type f
        for f in ${files[@]}; do
            if [[ $f =~ ^- ]]; then
                f=${f##-}
                if [[ $output =~ $f ]]; then
                    die "File '$f' found in image; it should have been ignored via $ignorefile"
                fi
            else
                is "$output" ".*$newdir/$f" \
                   "File '$f' should exist in container (no match in $ignorefile)"
            fi
        done

        # Clean up
        run_podman rmi -f build_test
    done
}

# Regression test for #9867
# Make sure that if you exclude everything in context dir, that
# the Containerfile/Dockerfile in the context dir are used
@test "podman build with ignore '*'" {
    local tmpdir=$PODMAN_TMPDIR/build-test-$(random_string 10)
    mkdir -p $tmpdir

    cat >$tmpdir/Containerfile <<EOF
FROM scratch
EOF

cat >$tmpdir/.dockerignore <<EOF
*
EOF

    run_podman build -t build_test $tmpdir

    # Rename Containerfile to Dockerfile
    mv $tmpdir/Containerfile $tmpdir/Dockerfile

    run_podman build -t build_test $tmpdir

    # Rename Dockerfile to foofile
    mv $tmpdir/Dockerfile $tmpdir/foofile

    run_podman 125 build -t build_test $tmpdir
    is "$output" ".*Dockerfile: no such file or directory"

    run_podman build -t build_test -f $tmpdir/foofile $tmpdir

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
    is "$output" ".*COMMIT" "COMMIT seen in log"
    is "$output" ".*STEP .*: RUN /bin/echo $random_echo"

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
STEP 1/2: FROM $IMAGE
STEP 2/2: RUN echo x${random2}y
x${random2}y${remote_extra}
COMMIT build_test${remote_extra}
--> [0-9a-f]\{11\}
Successfully tagged localhost/build_test:latest
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

# Caveat lector: this test was mostly copy-pasted from buildah in #9275.
# It's not entirely clear what it's testing, or if the 'mount' section is
# necessary.
@test "build with copy-from referencing the base image" {
  target=derived
  target_mt=derived-mt
  tmpdir=$PODMAN_TMPDIR/build-test
  mkdir -p $tmpdir

  containerfile1=$tmpdir/Containerfile1
  cat >$containerfile1 <<EOF
FROM $IMAGE AS build
RUN rm -f /etc/issue
USER 1001
COPY --from=$IMAGE /etc/issue /test/
EOF

  containerfile2=$tmpdir/Containerfile2
  cat >$containerfile2 <<EOF
FROM $IMAGE AS test
RUN rm -f /etc/alpine-release
FROM quay.io/libpod/alpine AS final
COPY --from=$IMAGE /etc/alpine-release /test/
EOF

  # Before the build, $IMAGE's base image should not be present
  local base_image=quay.io/libpod/alpine:latest
  run_podman 1 image exists $base_image

  run_podman build --jobs 1 -t ${target} -f ${containerfile2} ${tmpdir}
  run_podman build --no-cache --jobs 4 -t ${target_mt} -f ${containerfile2} ${tmpdir}

  # After the build, the base image should exist
  run_podman image exists $base_image

  # (can only test locally; podman-remote has no image mount command)
  # (can also only test as root; mounting under rootless podman is too hard)
  # We perform the test as a conditional, not a 'skip', because there's
  # value in testing the above 'build' commands even remote & rootless.
  if ! is_remote && ! is_rootless; then
    run_podman image mount ${target}
    root_single_job=$output

    run_podman image mount ${target_mt}
    root_multi_job=$output

    # Check that both the version with --jobs 1 and --jobs=N have the same number of files
    nfiles_single=$(find $root_single_job -type f | wc -l)
    nfiles_multi=$(find $root_multi_job -type f | wc -l)
    run_podman image umount ${target_mt}
    run_podman image umount ${target}

    is "$nfiles_single" "$nfiles_multi" \
       "Number of files (--jobs=1) == (--jobs=4)"

    # Make sure the number is reasonable
    test "$nfiles_single" -gt 50
  fi

  # Clean up
  run_podman rmi ${target_mt} ${target} ${base_image}
  run_podman image prune -f
}

@test "podman build --pull-never" {
    local tmpdir=$PODMAN_TMPDIR/build-test
    mkdir -p $tmpdir

    # First, confirm that --pull-never is a NOP if image exists locally
    local random_string=$(random_string 15)

    cat >$tmpdir/Containerfile <<EOF
FROM $IMAGE
RUN echo $random_string
EOF

    run_podman build -t build_test --pull-never $tmpdir
    is "$output" ".*$random_string" "pull-never is OK if image already exists"
    run_podman rmi build_test

    # Now try an image that does not exist locally nor remotely
    cat >$tmpdir/Containerfile <<EOF
FROM quay.io/libpod/nosuchimage:nosuchtag
RUN echo $random_string
EOF

    run_podman 125 build -t build_test --pull-never $tmpdir
    is "$output" \
       ".*Error: error creating build container: quay.io/libpod/nosuchimage:nosuchtag: image not known" \
       "--pull-never fails with expected error message"
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
    is "$output" ".*COMMIT" "COMMIT seen in log"

    run_podman rmi -f build_test
}

@test "podman build check_label" {
    skip_if_no_selinux
    tmpdir=$PODMAN_TMPDIR/build-test
    mkdir -p $tmpdir
    tmpbuilddir=$tmpdir/build
    mkdir -p $tmpbuilddir
    dockerfile=$tmpbuilddir/Dockerfile
    cat >$dockerfile <<EOF
FROM $IMAGE
RUN cat /proc/self/attr/current
EOF

    run_podman build -t build_test --security-opt label=level:s0:c3,c4 --format=docker $tmpbuilddir
    is "$output" ".*s0:c3,c4COMMIT" "label setting level"

    run_podman rmi -f build_test
}

@test "podman build check_seccomp_ulimits" {
    tmpdir=$PODMAN_TMPDIR/build-test
    mkdir -p $tmpdir
    tmpbuilddir=$tmpdir/build
    mkdir -p $tmpbuilddir
    dockerfile=$tmpbuilddir/Dockerfile
    cat >$dockerfile <<EOF
FROM $IMAGE
RUN grep Seccomp: /proc/self/status |awk '{ print \$1\$2 }'
RUN grep "Max open files" /proc/self/limits |awk '{ print \$4":"\$5 }'
EOF

    run_podman build --ulimit nofile=101:102 -t build_test $tmpbuilddir
    is "$output" ".*Seccomp:2" "setting seccomp"
    is "$output" ".*101:102" "setting ulimits"
    run_podman rmi -f build_test

    run_podman build -t build_test --security-opt seccomp=unconfined $tmpbuilddir
    is "$output" ".*Seccomp:0" "setting seccomp"
    run_podman rmi -f build_test
}

@test "podman build --authfile bogus test" {
    run_podman 125 build --authfile=/tmp/bogus - <<< "from scratch"
    is "$output" ".*/tmp/bogus: no such file or directory"
}

@test "podman build COPY hardlinks " {
    tmpdir=$PODMAN_TMPDIR/build-test
    subdir=$tmpdir/subdir
    subsubdir=$subdir/subsubdir
    mkdir -p $subsubdir

    dockerfile=$tmpdir/Dockerfile
    cat >$dockerfile <<EOF
FROM $IMAGE
COPY . /test
EOF
    ln $dockerfile $tmpdir/hardlink1
    ln $dockerfile $subdir/hardlink2
    ln $dockerfile $subsubdir/hardlink3

    run_podman build -t build_test $tmpdir
    run_podman run --rm build_test stat -c '%i' /test/Dockerfile
    dinode=$output
    run_podman run --rm build_test stat -c '%i' /test/hardlink1
    is "$output"   "$dinode"   "COPY hardlinks work"
    run_podman run --rm build_test stat -c '%i' /test/subdir/hardlink2
    is "$output"   "$dinode"   "COPY hardlinks work"
    run_podman run --rm build_test stat -c '%i' /test/subdir/subsubdir/hardlink3
    is "$output"   "$dinode"   "COPY hardlinks work"

    run_podman rmi -f build_test
}

@test "podman build -f test" {
    tmpdir=$PODMAN_TMPDIR/build-test
    subdir=$tmpdir/subdir
    mkdir -p $subdir

    containerfile1=$tmpdir/Containerfile1
    cat >$containerfile1 <<EOF
FROM scratch
copy . /tmp
EOF
    containerfile2=$PODMAN_TMPDIR/Containerfile2
    cat >$containerfile2 <<EOF
FROM $IMAGE
EOF
    run_podman build -t build_test -f Containerfile1 $tmpdir
    run_podman 125 build -t build_test -f Containerfile2 $tmpdir
    is "$output" ".*Containerfile2: no such file or directory" "Containerfile2 should not exist"
    run_podman build -t build_test -f $containerfile1 $tmpdir
    run_podman build -t build_test -f $containerfile2 $tmpdir
    run_podman build -t build_test -f $containerfile1
    run_podman build -t build_test -f $containerfile2
    run_podman build -t build_test -f $containerfile1 -f $containerfile2 $tmpdir
    is "$output" ".*$IMAGE" "Containerfile2 is also passed to server"
    run_podman rmi -f build_test
}

@test "podman build .dockerignore failure test" {
    tmpdir=$PODMAN_TMPDIR/build-test
    subdir=$tmpdir/subdir
    mkdir -p $subdir

    cat >$tmpdir/.dockerignore <<EOF
*
subdir
!*/sub1*
EOF
    cat >$tmpdir/Containerfile <<EOF
FROM $IMAGE
COPY ./ ./
COPY subdir ./
EOF
    run_podman 125 build -t build_test $tmpdir
    is "$output" ".*Error: error building at STEP \"COPY subdir ./\"" ".dockerignore was ignored"
}

@test "podman build .containerignore and .dockerignore test" {
    tmpdir=$PODMAN_TMPDIR/build-test
    mkdir -p $tmpdir
    touch $tmpdir/test1 $tmpdir/test2
    cat >$tmpdir/.containerignore <<EOF
test2*
EOF
    cat >$tmpdir/.dockerignore <<EOF
test1*
EOF
    cat >$tmpdir/Containerfile <<EOF
FROM $IMAGE
COPY ./ /tmp/test/
RUN ls /tmp/test/
EOF
    run_podman build -t build_test $tmpdir
    is "$output" ".*test1" "test1 should exists in the final image"
}

@test "podman build build context ownership" {
    tmpdir=$PODMAN_TMPDIR/build-test
    subdir=$tmpdir/subdir
    mkdir -p $subdir

    touch $tmpdir/empty-file.txt
    if is_remote && ! is_rootless ; then
	# TODO: set this file's owner to a UID:GID that will not be mapped
	# in the context where the remote server is running, which generally
	# requires us to be root (or running with more mapped IDs) on the
	# client, but not root (or running with fewer mapped IDs) on the
	# remote server
	# 4294967292:4294967292 (0xfffffffc:0xfffffffc) isn't that, but
	# it will catch errors where a remote server doesn't apply the right
	# default as it copies content into the container
        chown 4294967292:4294967292 $tmpdir/empty-file.txt
    fi
    cat >$tmpdir/Dockerfile <<EOF
FROM $IMAGE
COPY empty-file.txt .
RUN echo 0:0 | tee expected.txt
RUN stat -c "%u:%g" empty-file.txt | tee actual.txt
RUN cmp expected.txt actual.txt
EOF
    run_podman build -t build_test $tmpdir
}

@test "podman build build context is a symlink to a directory" {
    tmpdir=$PODMAN_TMPDIR/build-test
    mkdir -p $tmpdir/target
    ln -s target $tmpdir/link
    echo FROM $IMAGE > $tmpdir/link/Dockerfile
    echo RUN echo hello >> $tmpdir/link/Dockerfile
    run_podman build -t build_test $tmpdir/link
}

@test "podman build --volumes-from conflict" {
    rand_content=$(random_string 50)

    tmpdir=$PODMAN_TMPDIR/build-test
    mkdir -p $tmpdir
    dockerfile=$tmpdir/Dockerfile
    cat >$dockerfile <<EOF
FROM $IMAGE
VOLUME /vol
EOF

    run_podman build -t build_test $tmpdir
    is "$output" ".*COMMIT" "COMMIT seen in log"

    run_podman run -d --name test_ctr build_test  top
    run_podman run --rm --volumes-from test_ctr $IMAGE  echo $rand_content
    is "$output"   "$rand_content"   "No error should be thrown about volume in use"

    run_podman rmi -f build_test
}

function teardown() {
    # A timeout or other error in 'build' can leave behind stale images
    # that podman can't even see and which will cascade into subsequent
    # test failures. Try a last-ditch force-rm in cleanup, ignoring errors.
    run_podman '?' rm -t 0 -a -f
    run_podman '?' rmi -f build_test

    # Many of the tests above leave interim layers behind. Clean them up.
    run_podman '?' image prune -f

    basic_teardown
}

# vim: filetype=sh
