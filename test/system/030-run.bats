#!/usr/bin/env bats

load helpers

@test "podman run - basic tests" {
    rand=$(random_string 30)

    # 2019-09 Fedora 31 and rawhide (32) are switching from runc to crun
    # because of cgroups v2; crun emits different error messages.
    # Default to runc:
    err_no_such_cmd="Error: .*: starting container process caused.*exec:.*stat /no/such/command: no such file or directory"
    err_no_exec_dir="Error: .*: starting container process caused.*exec:.* permission denied"

    # ...but check the configured runtime engine, and switch to crun as needed
    run_podman info --format '{{ .Host.OCIRuntime.Path }}'
    if expr "$output" : ".*/crun"; then
        err_no_such_cmd="Error: executable file.* not found in \$PATH: No such file or directory: OCI runtime command not found error"
        err_no_exec_dir="Error: open executable: Operation not permitted: OCI runtime permission denied error"
    fi

    tests="
true              |   0 |
false             |   1 |
sh -c 'exit 32'   |  32 |
echo $rand        |   0 | $rand
/no/such/command  | 127 | $err_no_such_cmd
/etc              | 126 | $err_no_exec_dir
"

    tests_run=0
    while read cmd expected_rc expected_output; do
        if [ "$expected_output" = "''" ]; then expected_output=""; fi

        # THIS IS TRICKY: this is what lets us handle a quoted command.
        # Without this incantation (and the "$@" below), the cmd string
        # gets passed on as individual tokens: eg "sh" "-c" "'exit" "32'"
        # (note unmatched opening and closing single-quotes in the last 2).
        # That results in a bizarre and hard-to-understand failure
        # in the BATS 'run' invocation.
        # This should really be done inside parse_table; I can't find
        # a way to do so.
        eval set "$cmd"

        # FIXME: The </dev/null is a hack, necessary because as of 2019-09
        #        podman-remote has a bug in which it silently slurps up stdin,
        #        including the output of parse_table (i.e. tests to be run).
        run_podman $expected_rc run $IMAGE "$@" </dev/null

        # FIXME: remove conditional once podman-remote issue #4096 is fixed
        if ! is_remote; then
            is "$output" "$expected_output" "podman run $cmd - output"
        fi

        tests_run=$(expr $tests_run + 1)
    done < <(parse_table "$tests")

    # Make sure we ran the expected number of tests! Until 2019-09-24
    # podman-remote was only running one test (the "true" one); all
    # the rest were being silently ignored because of podman-remote
    # bug #4095, in which it slurps up stdin.
    is "$tests_run" "$(grep . <<<$tests | wc -l)" "Ran the full set of tests"
}

@test "podman run - global runtime option" {
    skip_if_remote "runtime flag is not passed over remote"
    run_podman 126 --runtime-flag invalidflag run --rm $IMAGE
    is "$output" ".*invalidflag" "failed when passing undefined flags to the runtime"
}

# 'run --preserve-fds' passes a number of additional file descriptors into the container
@test "podman run --preserve-fds" {
    skip_if_remote "preserve-fds is meaningless over remote"

    content=$(random_string 20)
    echo "$content" > $PODMAN_TMPDIR/tempfile

    run_podman run --rm -i --preserve-fds=2 $IMAGE sh -c "cat <&4" 4<$PODMAN_TMPDIR/tempfile
    is "$output" "$content" "container read input from fd 4"
}

@test "podman run - uidmapping has no /sys/kernel mounts" {
    skip_if_rootless "cannot umount as rootless"
    skip_if_remote "TODO Fix this for remote case"

    run_podman run --rm --uidmap 0:100:10000 $IMAGE mount
    run grep /sys/kernel <(echo "$output")
    is "$output" "" "unwanted /sys/kernel in 'mount' output"

    run_podman run --rm --net host --uidmap 0:100:10000 $IMAGE mount
    run grep /sys/kernel <(echo "$output")
    is "$output" "" "unwanted /sys/kernel in 'mount' output (with --net=host)"
}

# 'run --rm' goes through different code paths and may lose exit status.
# See https://github.com/containers/podman/issues/3795
@test "podman run --rm" {

    run_podman 0 run --rm $IMAGE /bin/true
    run_podman 1 run --rm $IMAGE /bin/false

    # Believe it or not, 'sh -c' resulted in different behavior
    run_podman 0 run --rm $IMAGE sh -c /bin/true
    run_podman 1 run --rm $IMAGE sh -c /bin/false
}

@test "podman run --name" {
    randomname=$(random_string 30)

    # Assume that 4 seconds gives us enough time for 3 quick tests (or at
    # least for the 'ps'; the 'container exists' should pass even in the
    # unlikely case that the container exits before we get to them)
    run_podman run -d --name $randomname $IMAGE sleep 4
    cid=$output

    run_podman ps --format '{{.Names}}--{{.ID}}'
    is "$output" "$randomname--${cid:0:12}"

    run_podman container exists $randomname
    run_podman container exists $cid

    # Done with live-container tests; now let's test after container finishes
    run_podman wait $cid

    # Container still exists even after stopping:
    run_podman container exists $randomname
    run_podman container exists $cid

    # ...but not after being removed:
    run_podman rm $cid
    run_podman 1 container exists $randomname
    run_podman 1 container exists $cid
}

@test "podman run --pull" {
    run_podman run --pull=missing $IMAGE true
    is "$output" "" "--pull=missing [present]: no output"

    run_podman run --pull=never $IMAGE true
    is "$output" "" "--pull=never [present]: no output"

    # Now test with a remote image which we don't have present (the 00 tag)
    NONLOCAL_IMAGE="$PODMAN_TEST_IMAGE_REGISTRY/$PODMAN_TEST_IMAGE_USER/$PODMAN_TEST_IMAGE_NAME:00000000"

    run_podman 125 run --pull=never $NONLOCAL_IMAGE true
    is "$output" "Error: unable to find a name and tag match for $NONLOCAL_IMAGE in repotags: no such image" "--pull=never [with image not present]: error"

    run_podman run --pull=missing $NONLOCAL_IMAGE true
    is "$output" "Trying to pull .*" "--pull=missing [with image NOT PRESENT]: fetches"

    run_podman run --pull=missing $NONLOCAL_IMAGE true
    is "$output" "" "--pull=missing [with image PRESENT]: does not re-fetch"

    run_podman run --pull=always $NONLOCAL_IMAGE true
    is "$output" "Trying to pull .*" "--pull=always [with image PRESENT]: re-fetches"

    # Very weird corner case fixed by #7770: 'podman run foo' will run 'myfoo'
    # if it exists, because the string 'foo' appears in 'myfoo'. This test
    # covers that, as well as making sure that our testimage (which is always
    # tagged :YYYYMMDD, never :latest) doesn't match either.
    run_podman tag $IMAGE my${PODMAN_TEST_IMAGE_NAME}:latest
    run_podman 125 run --pull=never $PODMAN_TEST_IMAGE_NAME true
    is "$output" "Error: unable to find a name and tag match for $PODMAN_TEST_IMAGE_NAME in repotags: no such image" \
       "podman run --pull=never with shortname (and implicit :latest)"

    # ...but if we add a :latest tag (without 'my'), it should now work
    run_podman tag $IMAGE ${PODMAN_TEST_IMAGE_NAME}:latest
    run_podman run --pull=never ${PODMAN_TEST_IMAGE_NAME} cat /home/podman/testimage-id
    is "$output" "$PODMAN_TEST_IMAGE_TAG" \
       "podman run --pull=never, with shortname, succeeds if img is present"

    run_podman rm -a
    run_podman rmi $NONLOCAL_IMAGE {my,}${PODMAN_TEST_IMAGE_NAME}:latest
}

# 'run --rmi' deletes the image in the end unless it's used by another container
@test "podman run --rmi" {
    # Name of a nonlocal image. It should be pulled in by the first 'run'
    NONLOCAL_IMAGE="$PODMAN_TEST_IMAGE_REGISTRY/$PODMAN_TEST_IMAGE_USER/$PODMAN_TEST_IMAGE_NAME:00000000"
    run_podman 1 image exists $NONLOCAL_IMAGE

    # Run a container, without --rm; this should block subsequent --rmi
    run_podman run --name keepme $NONLOCAL_IMAGE /bin/true
    run_podman image exists $NONLOCAL_IMAGE

    # Now try running with --rmi : it should succeed, but not remove the image
    run_podman run --rmi --rm $NONLOCAL_IMAGE /bin/true
    run_podman image exists $NONLOCAL_IMAGE

    # Remove the stray container, and run one more time with --rmi.
    run_podman rm keepme
    run_podman run --rmi --rm $NONLOCAL_IMAGE /bin/true
    run_podman 1 image exists $NONLOCAL_IMAGE
}

# 'run --conmon-pidfile --cid-file' makes sure we don't regress on these flags.
# Both are critical for systemd units.
@test "podman run --conmon-pidfile --cidfile" {
    pidfile=${PODMAN_TMPDIR}/pidfile
    cidfile=${PODMAN_TMPDIR}/cidfile

    cname=$(random_string)
    run_podman run --name $cname \
               --conmon-pidfile=$pidfile \
               --cidfile=$cidfile \
               --detach \
               $IMAGE sleep infinity
    cid="$output"

    is "$(< $cidfile)" "$cid" "contents of cidfile == container ID"

    # Cross-check --conmon-pidfile against 'podman inspect'
    local conmon_pid_from_file=$(< $pidfile)
    run_podman inspect --format '{{.State.ConmonPid}}' $cid
    local conmon_pid_from_inspect="$output"
    is "$conmon_pid_from_file" "$conmon_pid_from_inspect" \
       "Conmon pid in pidfile matches what 'podman inspect' claims"

    # /proc/PID/exe should be a symlink to a conmon executable
    # FIXME: 'echo' and 'ls' are to help debug #7580, a CI flake
    echo "conmon pid = $conmon_pid_from_file"
    ls -l /proc/$conmon_pid_from_file
    is "$(readlink /proc/$conmon_pid_from_file/exe)" ".*/conmon"  \
       "conmon pidfile (= PID $conmon_pid_from_file) points to conmon process"

    # All OK. Kill container.
    run_podman rm -f $cid

    # Podman must not overwrite existing cid file.
    # (overwriting conmon-pidfile is OK, so don't test that)
    run_podman 125 run --cidfile=$cidfile $IMAGE true
    is "$output" "Error: container id file exists. .* delete $cidfile" \
       "podman will not overwrite existing cidfile"
}

@test "podman run docker-archive" {
    skip_if_remote "podman-remote does not support docker-archive (#7116)"

    # Create an image that, when run, outputs a random magic string
    expect=$(random_string 20)
    run_podman run --name myc --entrypoint="[\"/bin/echo\",\"$expect\"]" $IMAGE
    is "$output" "$expect" "podman run --entrypoint echo-randomstring"

    # Save it as a tar archive
    run_podman commit myc myi
    archive=$PODMAN_TMPDIR/archive.tar
    run_podman save myi -o $archive
    is "$output" "" "podman save"

    # Clean up image and container from container storage...
    run_podman rmi myi
    run_podman rm myc

    # ... then confirm we can run from archive. This re-imports the image
    # and runs it, producing our random string as the last line.
    run_podman run docker-archive:$archive
    is "${lines[0]}" "Getting image source signatures" "podman run docker-archive, first line of output"
    is "$output" ".*Copying blob"     "podman run docker-archive"
    is "$output" ".*Copying config"   "podman run docker-archive"
    is "$output" ".*Writing manifest" "podman run docker-archive"
    is "${lines[-1]}" "$expect" "podman run docker-archive: expected random string output"

    # Clean up container as well as re-imported image
    run_podman rm -a
    run_podman rmi myi

    # Repeat the above, with podman-create and podman-start.
    run_podman create docker-archive:$archive
    cid=${lines[-1]}

    run_podman start --attach $cid
    is "$output" "$expect" "'podman run' of 'podman-create docker-archive'"

    # Clean up.
    run_podman rm $cid
    run_podman rmi myi
}

# #6735 : complex interactions with multiple user namespaces
# The initial report has to do with bind mounts, but that particular
# symptom only manifests on a fedora container image -- we have no
# reproducer on alpine. Checking directory ownership is good enough.
@test "podman run : user namespace preserved root ownership" {
    for priv in "" "--privileged"; do
        for user in "--user=0" "--user=100"; do
            for keepid in "" "--userns=keep-id"; do
                opts="$priv $user $keepid"

                for dir in /etc /usr;do
                    run_podman run --rm $opts $IMAGE stat -c '%u:%g:%n' $dir
                    remove_same_dev_warning      # grumble
                    is "$output" "0:0:$dir" "run $opts ($dir)"
                done
            done
        done
    done
}

# #6829 : add username to /etc/passwd inside container if --userns=keep-id
@test "podman run : add username to /etc/passwd if --userns=keep-id" {
    # Default: always run as root
    run_podman run --rm $IMAGE id -un
    is "$output" "root" "id -un on regular container"

    # This would always work on root, but is new behavior on rootless: #6829
    # adds a user entry to /etc/passwd
    whoami=$(id -un)
    run_podman run --rm --userns=keep-id $IMAGE id -un
    is "$output" "$whoami" "username on container with keep-id"

    # Setting user should also set $HOME (#8013).
    # Test setup below runs three cases: one with an existing home dir
    # and two without (one without any volume mounts, one with a misspelled
    # username). In every case, initial cwd should be /home/podman because
    # that's the container-defined WORKDIR. In the case of an existing
    # home dir, $HOME and ~ (passwd entry) will be /home/user; otherwise
    # they should be /home/podman.
    if is_rootless; then
        tests="
                |  /home/podman /home/podman /home/podman    | no vol mount
/home/x$whoami  |  /home/podman /home/podman /home/podman    | bad vol mount
/home/$whoami   |  /home/podman /home/$whoami /home/$whoami  | vol mount
"
        while read vol expect name; do
            opts=
            if [[ "$vol" != "''" ]]; then
                opts="-v $vol"
            fi
            run_podman run --rm $opts --userns=keep-id \
                   $IMAGE sh -c 'echo $(pwd;printenv HOME;echo ~)'
            is "$output" "$expect" "run with --userns=keep-id and $name sets \$HOME"
        done < <(parse_table "$tests")

        # Clean up volumes
        run_podman volume rm -a
    fi

    # --privileged should make no difference
    run_podman run --rm --privileged --userns=keep-id $IMAGE id -un
    remove_same_dev_warning      # grumble
    is "$output" "$(id -un)" "username on container with keep-id"

    # ...but explicitly setting --user should override keep-id
    run_podman run --rm --privileged --userns=keep-id --user=0 $IMAGE id -un
    remove_same_dev_warning      # grumble
    is "$output" "root" "--user=0 overrides keep-id"
}

# #6991 : /etc/passwd is modifiable
@test "podman run : --userns=keep-id: passwd file is modifiable" {
    run_podman run -d --userns=keep-id --cap-add=dac_override $IMAGE sh -c 'while ! test -e /tmp/stop; do sleep 0.1; done'
    cid="$output"

    # Assign a UID that is (a) not in our image /etc/passwd and (b) not
    # the same as that of the user running the test script; this guarantees
    # that the added passwd entry will be what we expect.
    #
    # For GID, we have to use one that already exists in the container. And
    # unfortunately, 'adduser' requires a string name. We use 999:ping
    local uid=4242
    if [[ $uid == $(id -u) ]]; then
        uid=4343
    fi

    gecos="$(random_string 6) $(random_string 8)"
    run_podman exec --user root $cid adduser -u $uid -G ping -D -g "$gecos" -s /bin/sh newuser3
    is "$output" "" "output from adduser"
    run_podman exec $cid tail -1 /etc/passwd
    is "$output" "newuser3:x:$uid:999:$gecos:/home/newuser3:/bin/sh" \
       "newuser3 added to /etc/passwd in container"

    run_podman exec $cid touch /tmp/stop
    run_podman wait $cid
}

# For #7754: json-file was equating to 'none'
@test "podman run --log-driver" {
    # '-' means that LogPath will be blank and there's no easy way to test
    tests="
none      | -
journald  | -
k8s-file  | y
json-file | f
"
    while read driver do_check; do
        msg=$(random_string 15)
        run_podman run --name myctr --log-driver $driver $IMAGE echo $msg

        # Simple output check
        # Special case: 'json-file' emits a warning, the rest do not
        # ...but with podman-remote the warning is on the server only
        if [[ $do_check == 'f' ]] && ! is_remote; then      # 'f' for 'fallback'
            is "${lines[0]}" ".* level=error msg=\"json-file logging specified but not supported. Choosing k8s-file logging instead\"" \
               "Fallback warning emitted"
            is "${lines[1]}" "$msg" "basic output sanity check (driver=$driver)"
        else
            is "$output" "$msg" "basic output sanity check (driver=$driver)"
        fi

        # Simply confirm that podman preserved our argument as-is
        run_podman inspect --format '{{.HostConfig.LogConfig.Type}}' myctr
        is "$output" "$driver" "podman inspect: driver"

        # If LogPath is non-null, check that it exists and has a valid log
        run_podman inspect --format '{{.LogPath}}' myctr
        if [[ $do_check != '-' ]]; then
            is "$output" "/.*" "LogPath (driver=$driver)"
            if ! test -e "$output"; then
                die "LogPath (driver=$driver) does not exist: $output"
            fi
            # eg 2020-09-23T13:34:58.644824420-06:00 stdout F 7aiYtvrqFGJWpak
            is "$(< $output)" "[0-9T:.+-]\+ stdout F $msg" \
               "LogPath contents (driver=$driver)"
        else
            is "$output" "" "LogPath (driver=$driver)"
        fi
        run_podman rm myctr
    done < <(parse_table "$tests")

    # Invalid log-driver argument
    run_podman 125 run --log-driver=InvalidDriver $IMAGE true
    is "$output" "Error: error running container create option: invalid log driver: invalid argument" \
       "--log-driver InvalidDriver"
}

@test "podman run --log-driver journald" {
    skip_if_remote "We cannot read journalctl over remote."

    msg=$(random_string 20)
    pidfile="${PODMAN_TMPDIR}/$(random_string 20)"

    run_podman run --name myctr --log-driver journald --conmon-pidfile $pidfile $IMAGE echo $msg

    journalctl --output cat  _PID=$(cat $pidfile)
    is "$output" "$msg" "check that journalctl output equals the container output"

    run_podman rm myctr
}

@test "podman run --tz" {
    # This file will always have a constant reference timestamp
    local testfile=/home/podman/testimage-id

    run_podman run --rm $IMAGE date -r $testfile
    is "$output" "Sun Sep 13 12:26:40 UTC 2020" "podman run with no TZ"

    run_podman run --rm --tz=MST7MDT $IMAGE date -r $testfile
    is "$output" "Sun Sep 13 06:26:40 MDT 2020" "podman run with --tz=MST7MDT"

    # --tz=local pays attention to /etc/localtime, not $TZ. We set TZ anyway,
    # to make sure podman ignores it; and, because this test is locale-
    # dependent, we pick an obscure zone (+1245) that is unlikely to
    # collide with any of our testing environments.
    #
    # To get a reference timestamp we run 'date' locally; note the explicit
    # strftime() format. We can't use --iso=seconds because GNU date adds
    # a colon to the TZ offset (eg -07:00) whereas alpine does not (-0700).
    run date --date=@1600000000 +%Y-%m-%dT%H:%M:%S%z
    expect="$output"
    TZ=Pacific/Chatham run_podman run --rm --tz=local $IMAGE date -Iseconds -r $testfile
    is "$output" "$expect" "podman run with --tz=local, matches host"
}

# run with --runtime should preserve the named runtime
@test "podman run : full path to --runtime is preserved" {
    skip_if_cgroupsv1
    skip_if_remote
    run_podman run -d --runtime '/usr/bin/crun' $IMAGE sleep 60
    cid="$output"

    run_podman inspect --format '{{.OCIRuntime}}' $cid
    is "$output" "/usr/bin/crun"

    run_podman kill $cid
}

# Regression test for issue #8082
@test "podman run : look up correct image name" {
	# Create a 2nd tag for the local image.
	local name="localhost/foo/bar"
	run_podman tag $IMAGE $name

	# Create a container with the 2nd tag and make sure that it's being
	# used.  #8082 always inaccurately used the 1st tag.
	run_podman create $name
	cid="$output"

	run_podman inspect --format "{{.ImageName}}" $cid
	is "$output" "$name"

	# Clean up.
	run_podman rm $cid
	run_podman untag $IMAGE $name
}

# vim: filetype=sh
