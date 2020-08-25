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
        err_no_such_cmd="Error: executable file not found in \$PATH: No such file or directory: OCI runtime command not found error"
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
# See https://github.com/containers/libpod/issues/3795
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
    skip_if_remote "podman-remote does not emit 'Trying to pull' msgs"

    run_podman run --pull=missing $IMAGE true
    is "$output" "" "--pull=missing [present]: no output"

    run_podman run --pull=never $IMAGE true
    is "$output" "" "--pull=never [present]: no output"

    # Now test with busybox, which we don't have present
    run_podman 125 run --pull=never busybox true
    is "$output" "Error: unable to find a name and tag match for busybox in repotags: no such image" "--pull=never [busybox/missing]: error"

    run_podman run --pull=missing busybox true
    is "$output" "Trying to pull .*" "--pull=missing [busybox/missing]: fetches"

    run_podman run --pull=always busybox true
    is "$output" "Trying to pull .*" "--pull=always [busybox/present]: fetches"

    run_podman rm -a
    run_podman rmi busybox
}

# 'run --rmi' deletes the image in the end unless it's used by another container
@test "podman run --rmi" {
    skip_if_remote

    # Name of a nonlocal image. It should be pulled in by the first 'run'
    NONLOCAL_IMAGE=busybox
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

    conmon_pid=$(< $pidfile)
    is "$(readlink /proc/$conmon_pid/exe)" ".*/conmon"  \
       "conmon pidfile (= PID $conmon_pid) points to conmon process"

    # All OK. Kill container.
    run_podman rm -f $cid

    # Podman must not overwrite existing cid file.
    # (overwriting conmon-pidfile is OK, so don't test that)
    run_podman 125 run --cidfile=$cidfile $IMAGE true
    is "$output" "Error: container id file exists. .* delete $cidfile" \
       "podman will not overwrite existing cidfile"
}

@test "podman run docker-archive" {
    skip_if_remote "FIXME: pending #7116"

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
    skip_if_remote "FIXME: pending #7195"

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
    skip_if_remote "FIXME: pending #7195"

    # Default: always run as root
    run_podman run --rm $IMAGE id -un
    is "$output" "root" "id -un on regular container"

    # This would always work on root, but is new behavior on rootless: #6829
    # adds a user entry to /etc/passwd
    run_podman run --rm --userns=keep-id $IMAGE id -un
    is "$output" "$(id -un)" "username on container with keep-id"

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
    skip_if_remote "FIXME: pending #7195"

    run_podman run -d --userns=keep-id $IMAGE sh -c 'while ! test -e /stop; do sleep 0.1; done'
    cid="$output"

    gecos="$(random_string 6) $(random_string 8)"
    run_podman exec --user root $cid adduser -u 4242 -G ping -D -g "$gecos" -s /bin/sh newuser3
    is "$output" "" "output from adduser"
    run_podman exec $cid tail -1 /etc/passwd
    is "$output" "newuser3:x:4242:999:$gecos:/home/newuser3:/bin/sh" \
       "newuser3 added to /etc/passwd in container"

    run_podman exec $cid touch /stop
    run_podman wait $cid
}

# vim: filetype=sh
