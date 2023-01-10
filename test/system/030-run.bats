#!/usr/bin/env bats

load helpers
load helpers.network

@test "podman run - basic tests" {
    rand=$(random_string 30)

    err_no_such_cmd="Error:.*/no/such/command.*[Nn]o such file or directory"
    err_no_exec_dir="Error:.*exec.*permission denied"

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
        run_podman $expected_rc run $IMAGE "$@"
        is "$output" "$expected_output" "podman run $cmd - output"

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

@test "podman run --memory=0 runtime option" {
    run_podman run --memory=0 --rm $IMAGE echo hello
    if is_rootless && ! is_cgroupsv2; then
        is "${lines[0]}" "Resource limits are not supported and ignored on cgroups V1 rootless systems" "--memory is not supported"
        is "${lines[1]}" "hello" "--memory is ignored"
    else
        is "$output" "hello" "failed to run when --memory is set to 0"
    fi
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
    skip_if_cgroupsv1 "FIXME: #15025: run --uidmap fails on cgroups v1"
    skip_if_rootless "cannot umount as rootless"
    skip_if_remote "TODO Fix this for remote case"

    run_podman run --rm --uidmap 0:100:10000 $IMAGE mount
    assert "$output" !~ /sys/kernel "unwanted /sys/kernel in 'mount' output"

    run_podman run --rm --net host --uidmap 0:100:10000 $IMAGE mount
    assert "$output" !~ /sys/kernel \
           "unwanted /sys/kernel in 'mount' output (with --net=host)"
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
    NONLOCAL_IMAGE="$PODMAN_NONLOCAL_IMAGE_FQN"

    run_podman 125 run --pull=never $NONLOCAL_IMAGE true
    is "$output" "Error: $NONLOCAL_IMAGE: image not known" "--pull=never [with image not present]: error"

    run_podman run --pull=missing $NONLOCAL_IMAGE true
    is "$output" "Trying to pull .*" "--pull=missing [with image NOT PRESENT]: fetches"

    run_podman run --pull=missing $NONLOCAL_IMAGE true
    is "$output" "" "--pull=missing [with image PRESENT]: does not re-fetch"

    run_podman run --pull=always $NONLOCAL_IMAGE true
    is "$output" "Trying to pull .*" "--pull=always [with image PRESENT]: re-fetches"

    # NOTE: older version of podman would match "foo" against "myfoo". That
    # behaviour was changed with introduction of `containers/common/libimage`
    # which will only match at repository boundaries (/).
    run_podman 125 run --pull=never my$PODMAN_TEST_IMAGE_NAME true
    is "$output" "Error: my$PODMAN_TEST_IMAGE_NAME: image not known" \
       "podman run --pull=never with shortname (and implicit :latest)"

    # ...but if we add a :latest tag (without 'my'), it should now work
    run_podman tag $IMAGE ${PODMAN_TEST_IMAGE_NAME}:latest
    run_podman run --pull=never ${PODMAN_TEST_IMAGE_NAME} cat /home/podman/testimage-id
    is "$output" "$PODMAN_TEST_IMAGE_TAG" \
       "podman run --pull=never, with shortname, succeeds if img is present"

    run_podman rm -a
    run_podman rmi $NONLOCAL_IMAGE ${PODMAN_TEST_IMAGE_NAME}:latest
}

# 'run --rmi' deletes the image in the end unless it's used by another container
@test "podman run --rmi" {
    # Name of a nonlocal image. It should be pulled in by the first 'run'
    NONLOCAL_IMAGE="$PODMAN_NONLOCAL_IMAGE_FQN"
    run_podman 1 image exists $NONLOCAL_IMAGE

    # Run a container, without --rm; this should block subsequent --rmi
    run_podman run --name /keepme $NONLOCAL_IMAGE /bin/true
    run_podman image exists $NONLOCAL_IMAGE

    # Now try running with --rmi : it should succeed, but not remove the image
    run_podman run --rmi --rm $NONLOCAL_IMAGE /bin/true
    run_podman image exists $NONLOCAL_IMAGE

    # Remove the stray container, and run one more time with --rmi.
    run_podman rm /keepme
    run_podman run --rmi --rm $NONLOCAL_IMAGE /bin/true
    run_podman 1 image exists $NONLOCAL_IMAGE
}

# 'run --conmon-pidfile --cid-file' makes sure we don't regress on these flags.
# Both are critical for systemd units.
@test "podman run --conmon-pidfile --cidfile" {
    pidfile=${PODMAN_TMPDIR}/pidfile
    cidfile=${PODMAN_TMPDIR}/cidfile

    # Write random content to the cidfile to make sure its content is truncated
    # on write.
    echo "$(random_string 120)" > $cidfile

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
    if [[ -e $cidfile ]]; then
        die "cidfile $cidfile should be removed along with container"
    fi
}

@test "podman run docker-archive" {
    skip_if_remote "podman-remote does not support docker-archive"

    # Create an image that, when run, outputs a random magic string
    expect=$(random_string 20)
    run_podman run --name myc --entrypoint="[\"/bin/echo\",\"$expect\"]" $IMAGE
    is "$output" "$expect" "podman run --entrypoint echo-randomstring"

    # Save it as a tar archive
    run_podman commit myc myi
    archive=$PODMAN_TMPDIR/archive.tar
    run_podman save --quiet myi -o $archive
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
    keep="--userns=keep-id"
    is_rootless || keep=""
    for priv in "" "--privileged"; do
        for user in "--user=0" "--user=100"; do
            for keepid in "" ${keep}; do
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
    skip_if_not_rootless "--userns=keep-id only works in rootless mode"
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
    skip_if_not_rootless "--userns=keep-id only works in rootless mode"
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
        is "$output" "$msg" "basic output sanity check (driver=$driver)"

        # Simply confirm that podman preserved our argument as-is
        run_podman inspect --format '{{.HostConfig.LogConfig.Type}}' myctr
        is "$output" "$driver" "podman inspect: driver"

        # If LogPath is non-null, check that it exists and has a valid log
        run_podman inspect --format '{{.HostConfig.LogConfig.Path}}' myctr
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

        if [[ $driver != 'none' ]]; then
            if [[ $driver = 'journald' ]] && journald_unavailable; then
                # Cannot perform check
                :
            else
                run_podman logs myctr
                is "$output" "$msg" "podman logs, with driver '$driver'"
            fi
        else
            run_podman 125 logs myctr
            if ! is_remote; then
                is "$output" ".*this container is using the 'none' log driver, cannot read logs.*" \
                   "podman logs, with driver 'none', should fail with error"
            fi
        fi
        run_podman rm myctr
    done < <(parse_table "$tests")

    # Invalid log-driver argument
    run_podman 125 run --log-driver=InvalidDriver $IMAGE true
    is "$output" "Error: running container create option: invalid log driver: invalid argument" \
       "--log-driver InvalidDriver"
}

@test "podman run --log-driver journald" {
    skip_if_remote "We cannot read journalctl over remote."

    # We can't use journald on RHEL as rootless, either: rhbz#1895105
    skip_if_journald_unavailable

    msg=$(random_string 20)
    pidfile="${PODMAN_TMPDIR}/$(random_string 20)"

    # Multiple --log-driver options to confirm that last one wins
    run_podman run --name myctr --log-driver=none --log-driver journald \
               --conmon-pidfile $pidfile $IMAGE echo $msg

    journalctl --output cat  _PID=$(cat $pidfile)
    is "$output" "$msg" "check that journalctl output equals the container output"

    run_podman rm myctr
}

@test "podman run --tz" {
    # This file will always have a constant reference timestamp
    local testfile=/home/podman/testimage-id

    run_podman run --rm $IMAGE date -r $testfile
    is "$output" "Sun Sep 13 12:26:40 UTC 2020" "podman run with no TZ"

    # Multiple --tz options; confirm that the last one wins
    run_podman run --rm --tz=US/Eastern --tz=Iceland --tz=MST7MDT \
               $IMAGE date -r $testfile
    is "$output" "Sun Sep 13 06:26:40 MDT 2020" "podman run with --tz=MST7MDT"

    # --tz=local pays attention to /etc/localtime, not $TZ. We set TZ anyway,
    # to make sure podman ignores it; and, because this test is locale-
    # dependent, we pick an obscure zone (+1245) that is unlikely to
    # collide with any of our testing environments.
    #
    # To get a reference timestamp we run 'date' locally. This requires
    # that GNU date output matches that of alpine; this seems to be true
    # as of testimage:20220615.
    run date --date=@1600000000 --iso=seconds
    expect="$output"
    TZ=Pacific/Chatham run_podman run --rm --tz=local $IMAGE date -Iseconds -r $testfile
    is "$output" "$expect" "podman run with --tz=local, matches host"
}

# run with --runtime should preserve the named runtime
@test "podman run : full path to --runtime is preserved" {
    skip_if_remote "podman-remote does not support --runtime option"

    # Get configured runtime
    run_podman info --format '{{.Host.OCIRuntime.Path}}'
    runtime="$output"

    # Assumes that /var/tmp is not mounted noexec; this is usually safe
    new_runtime="/var/tmp/myruntime$(random_string 12)"
    cp --preserve $runtime $new_runtime

    run_podman run -d --runtime "$new_runtime" $IMAGE sleep 60
    cid="$output"

    run_podman inspect --format '{{.OCIRuntime}}' $cid
    is "$output" "$new_runtime" "podman inspect shows configured runtime"
    run_podman kill $cid
    run_podman wait $cid
    run_podman rm $cid
    rm -f $new_runtime
}

@test "podman --noout run should print output" {
    run_podman --noout run -d --name test $IMAGE echo hi
    is "$output" "" "output should be empty"
    run_podman wait test
    run_podman --noout rm test
    is "$output" "" "output should be empty"
}

@test "podman --noout create should print output" {
    run_podman --noout create --name test $IMAGE echo hi
    is "$output" "" "output should be empty"
    run_podman --noout rm test
    is "$output" "" "output should be empty"
}

# Regression test for issue #8082
@test "podman run : look up correct image name" {
    # Create a 2nd tag for the local image. Force to lower case, and apply it.
    local newtag="localhost/$(random_string 10)/$(random_string 8)"
    newtag=${newtag,,}
    run_podman tag $IMAGE $newtag

    # Create a container with the 2nd tag and make sure that it's being
    # used.  #8082 always inaccurately used the 1st tag.
    run_podman create $newtag
    cid="$output"

    run_podman inspect --format "{{.ImageName}}" $cid
    is "$output" "$newtag:latest" \
       "container .ImageName is the container-create name"

    # Same thing, but now with a :tag, and making sure it works with --name
    newtag2="${newtag}:$(random_string 6|tr A-Z a-z)"
    run_podman tag $IMAGE $newtag2

    cname="$(random_string 14|tr A-Z a-z)"
    run_podman create --name $cname $newtag2
    run_podman inspect --format "{{.ImageName}}" $cname
    is "$output" "$newtag2" \
       "container .ImageName is the container-create name, with :tag"

    # Clean up.
    run_podman rm $cid $cname
    run_podman untag $IMAGE $newtag $newtag2
}

# Regression test for issue #8558
@test "podman run on untagged image: make sure that image metadata is set" {
    run_podman inspect $IMAGE --format "{{.ID}}"
    imageID="$output"

    # prior to #8623 `podman run` would error out on untagged images with:
    # Error: both RootfsImageName and RootfsImageID must be set if either is set: invalid argument
    run_podman untag $IMAGE

    run_podman run --rm $randomname $imageID true
    run_podman tag $imageID $IMAGE
}

@test "podman inspect includes image data" {
    randomname=$(random_string 30)

    run_podman inspect $IMAGE --format "{{.ID}} {{.Digest}}"
    expected="$IMAGE $output"

    run_podman run --name $randomname $IMAGE true
    run_podman container inspect $randomname --format "{{.ImageName}} {{.Image}} {{.ImageDigest}}"
    is "$output" "$expected"
    run_podman rm -f -t0 $randomname
}

@test "Verify /run/.containerenv exist" {
    # Nonprivileged container: file exists, but must be empty
    run_podman run --rm $IMAGE stat -c '%s' /run/.containerenv
    is "$output" "0" "file size of /run/.containerenv, nonprivileged"

    # Prep work: get ID of image; make a cont. name; determine if we're rootless
    run_podman inspect --format '{{.ID}}' $IMAGE
    local iid="$output"

    random_cname=c$(random_string 15 | tr A-Z a-z)
    local rootless=0
    if is_rootless; then
        rootless=1
    fi

    run_podman run --privileged --rm --name $random_cname $IMAGE \
               sh -c '. /run/.containerenv; echo $engine; echo $name; echo $image; echo $id; echo $imageid; echo $rootless'

    # FIXME: on some CI systems, 'run --privileged' emits a spurious
    # warning line about dup devices. Ignore it.
    remove_same_dev_warning

    is "${lines[0]}" "podman-.*"      'containerenv : $engine'
    is "${lines[1]}" "$random_cname"  'containerenv : $name'
    is "${lines[2]}" "$IMAGE"         'containerenv : $image'
    is "${lines[3]}" "[0-9a-f]\{64\}" 'containerenv : $id'
    is "${lines[4]}" "$iid"           'containerenv : $imageid'
    is "${lines[5]}" "$rootless"      'containerenv : $rootless'
}

@test "podman run with --net=host and --port prints warning" {
    rand=$(random_string 10)

    run_podman run --rm -p 8080 --net=host $IMAGE echo $rand
    is "${lines[0]}" \
       "Port mappings have been discarded as one of the Host, Container, Pod, and None network modes are in use" \
       "Warning is emitted before container output"
    is "${lines[1]}" "$rand" "Container runs successfully despite warning"
}

@test "podman run - check workdir" {
    # Workdirs specified via the CLI are not created on the root FS.
    run_podman 126 run --rm --workdir /i/do/not/exist $IMAGE pwd
    # Note: remote error prepends an attach error.
    is "$output" "Error: .*workdir \"/i/do/not/exist\" does not exist on container.*"

    testdir=$PODMAN_TMPDIR/volume
    mkdir -p $testdir
    randomcontent=$(random_string 10)
    echo "$randomcontent" > $testdir/content

    # Workdir does not exist on the image but is volume mounted.
    run_podman run --rm --workdir /IamNotOnTheImage -v $testdir:/IamNotOnTheImage:Z $IMAGE cat content
    is "$output" "$randomcontent" "cat random content"

    # Workdir does not exist on the image but is created by the runtime as it's
    # a subdir of a volume.
    run_podman run --rm --workdir /IamNotOntheImage -v $testdir/content:/IamNotOntheImage/foo:Z $IMAGE cat foo
    is "$output" "$randomcontent" "cat random content"

    # Make sure that running on a read-only rootfs works (#9230).
    if ! is_rootless && ! is_remote; then
        # image mount is hard to test as a rootless user
        # and does not work remotely
        run_podman image mount $IMAGE
        romount="$output"

        randomname=$(random_string 30)
        # FIXME FIXME FIXME: Remove :O once (if) #14504 is fixed!
        run_podman run --name=$randomname --rootfs $romount:O echo "Hello world"
        is "$output" "Hello world"

	run_podman container inspect $randomname --format "{{.ImageDigest}}"
	is "$output" "" "Empty image digest for --rootfs container"

	run_podman rm -f -t0 $randomname
        run_podman image unmount $IMAGE
    fi
}

# https://github.com/containers/podman/issues/9096
# podman exec may truncate stdout/stderr; actually a bug in conmon:
# https://github.com/containers/conmon/issues/236
@test "podman run - does not truncate or hang with big output" {
    # Size, in bytes, to dd and to expect in return
    char_count=700000

    # Container name; primarily needed when running podman-remote
    cname=mybigdatacontainer

    # This is one of those cases where BATS is not the best test framework.
    # We can't do any output redirection, because 'run' overrides it so
    # as to preserve $output. We can't _not_ do redirection, because BATS
    # doesn't like NULs in $output (and neither would humans who might
    # have to read them in an error log).
    # Workaround: write to a log file, and don't attach stdout.
    run_podman run --name $cname --attach stderr --log-driver k8s-file \
               $IMAGE dd if=/dev/zero count=$char_count bs=1
    is "${lines[0]}" "$char_count+0 records in"  "dd: number of records in"
    is "${lines[1]}" "$char_count+0 records out" "dd: number of records out"

    # We don't have many tests for '-l'. This is as good a place as any
    if ! is_remote; then
        cname=-l
    fi

    # Now find that log file, and count the NULs in it.
    # The log file is of the form '<timestamp> <P|F> <data>', where P|F
    # is Partial/Full; I think that's called "kubernetes log format"?
    run_podman inspect $cname --format '{{.HostConfig.LogConfig.Path}}'
    logfile="$output"

    count_zero=$(tr -cd '\0' <$logfile | wc -c)
    is "$count_zero" "$char_count" "count of NULL characters in log"

    # Clean up
    run_podman rm $cname
}

@test "podman run - do not set empty HOME" {
    # Regression test for #9378.
    run_podman run --rm --user 100 $IMAGE printenv
    is "$output" ".*HOME=/.*"
}

@test "podman run --timeout - basic test" {
    cid=timeouttest
    t0=$SECONDS
    run_podman 255 run --name $cid --timeout 10 $IMAGE sleep 60
    t1=$SECONDS
    # Confirm that container is stopped. Podman-remote unfortunately
    # cannot tell the difference between "stopped" and "exited", and
    # spits them out interchangeably, so we need to recognize either.
    run_podman inspect --format '{{.State.Status}} {{.State.ExitCode}}' $cid
    is "$output" "\\(stopped\|exited\\) \-1" \
       "Status and exit code of stopped container"

    # This operation should take
    # exactly 10 seconds. Give it some leeway.
    delta_t=$(( $t1 - $t0 ))
    assert "$delta_t" -gt  8 "podman stop: ran too quickly!"
    assert "$delta_t" -le 14 "podman stop: took too long"

    run_podman rm $cid
}

@test "podman run no /etc/mtab " {
    tmpdir=$PODMAN_TMPDIR/build-test
    mkdir -p $tmpdir

    cat >$tmpdir/Dockerfile <<EOF
FROM $IMAGE
RUN rm /etc/mtab
EOF
    expected="'/etc/mtab' -> '/proc/mounts'"
    run_podman build -t nomtab $tmpdir
    run_podman run --rm nomtab stat -c %N /etc/mtab
    is "$output" "$expected" "/etc/mtab should be created"

    run_podman rmi nomtab
}

@test "podman run --hostuser tests" {
    skip_if_not_rootless "test whether hostuser is successfully added"
    user=$(id -un)
    run_podman 1 run --rm $IMAGE grep $user /etc/passwd
    run_podman run --hostuser=$user --rm $IMAGE grep $user /etc/passwd
    user=$(id -u)
    run_podman run --hostuser=$user --rm $IMAGE grep $user /etc/passwd
    run_podman run --hostuser=$user --user $user --rm $IMAGE grep $user /etc/passwd
    user=bogus
    run_podman 126 run --hostuser=$user --rm $IMAGE grep $user /etc/passwd
}

@test "podman run --device-cgroup-rule tests" {
    skip_if_rootless "cannot add devices in rootless mode"

    run_podman run --device-cgroup-rule="b 7:* rmw" --rm $IMAGE
    run_podman run --device-cgroup-rule="c 7:* rmw" --rm $IMAGE
    run_podman run --device-cgroup-rule="a 7:1 rmw" --rm $IMAGE
    run_podman run --device-cgroup-rule="a 7 rmw" --rm $IMAGE
    run_podman 125 run --device-cgroup-rule="b 7:* rmX" --rm $IMAGE
    is "$output" "Error: invalid device access in device-access-add: X"
    run_podman 125 run --device-cgroup-rule="b 7:2" --rm $IMAGE
    is "$output" 'Error: invalid device cgroup rule requires type, major:Minor, and access rules: "b 7:2"'
    run_podman 125 run --device-cgroup-rule="x 7:* rmw" --rm $IMAGE
    is "$output" "Error: invalid device type in device-access-add: x"
    run_podman 125 run --device-cgroup-rule="a a:* rmw" --rm $IMAGE
    is "$output" "Error: strconv.ParseUint: parsing \"a\": invalid syntax"
}

@test "podman run closes stdin" {
    random_1=$(random_string 25)
    run_podman run -i --rm $IMAGE cat <<<"$random_1"
    is "$output" "$random_1" "output matches STDIN"
}

@test "podman run defaultenv" {
    run_podman run --rm $IMAGE printenv
    assert "$output" =~ "TERM=xterm" "env includes TERM"
    assert "$output" =~ "container=podman" "env includes container=podman"

    run_podman run --unsetenv=TERM --rm $IMAGE printenv
    assert "$output" =~ "container=podman" "env includes container=podman"
    assert "$output" != "TERM" "unwanted TERM environment variable despite --unsetenv=TERM"

    run_podman run --unsetenv-all --rm $IMAGE /bin/printenv
    for v in TERM container PATH; do
        assert "$output" !~ "$v" "variable present despite --unsetenv-all"
    done

    run_podman run --unsetenv-all --env TERM=abc --rm $IMAGE /bin/printenv
    assert "$output" =~ "TERM=abc" \
           "missing TERM environment variable despite TERM being set on commandline"
}

@test "podman run - no /etc/hosts" {
    if [[ -z "$container" ]]; then
        skip "Test is too dangerous to run in a non-container environment"
    fi
    skip_if_rootless "cannot move /etc/hosts file as a rootless user"

    local hosts_tmp=/etc/hosts.RENAME-ME-BACK-TO-JUST-HOSTS
    if [[ -e $hosts_tmp ]]; then
        die "Internal error: leftover backup hosts file: $hosts_tmp"
    fi
    mv /etc/hosts $hosts_tmp
    run_podman '?' run --rm --add-host "foo.com:1.2.3.4" $IMAGE cat "/etc/hosts"
    mv $hosts_tmp /etc/hosts
    assert "$status" = 0 \
           "podman run without /etc/hosts file should work"
    assert "$output" =~ "^1\.2\.3\.4[[:blank:]]foo\.com.*" \
           "users can add hosts even without /etc/hosts"
}

# rhbz#1854566 : $IMAGE has incorrect permission 555 on the root '/' filesystem
@test "podman run image with filesystem permission" {
    # make sure the IMAGE image have permissiong of 555 like filesystem RPM expects
    run_podman run --rm $IMAGE stat -c %a /
    is "$output" "555" "directory permissions on /"
}

# rhbz#1763007 : the --log-opt for podman run does not work as expected
@test "podman run with log-opt option" {
    # Pseudorandom size of the form N.NNN. The '| 1' handles '0.NNN' or 'N.NN0',
    # which podman displays as 'NNN kB' or 'N.NN MB' respectively.
    size=$(printf "%d.%03d" $(($RANDOM % 10 | 1)) $(($RANDOM % 100 | 1)))
    run_podman run -d --rm --log-opt max-size=${size}m $IMAGE sleep 5
    cid=$output
    run_podman inspect --format "{{ .HostConfig.LogConfig.Size }}" $cid
    is "$output" "${size}MB"
    run_podman rm -t 0 -f $cid
}

@test "podman run --kernel-memory warning" {
    # Not sure what situations this fails in, but want to make sure warning shows.
    run_podman '?' run --rm --kernel-memory 100 $IMAGE false
    is "$output" ".*The --kernel-memory flag is no longer supported. This flag is a noop." "warn on use of --kernel-memory"

}

# rhbz#1902979 : podman run fails to update /etc/hosts when --uidmap is provided
@test "podman run update /etc/hosts" {
    skip_if_cgroupsv1 "FIXME: #15025: run --uidmap fails on cgroups v1"
    HOST=$(random_string 25)
    run_podman run --uidmap 0:10001:10002 --rm --hostname ${HOST} $IMAGE grep ${HOST} /etc/hosts
    is "${lines[0]}" ".*${HOST}.*"
}

@test "podman run doesn't override oom-score-adj" {
    current_oom_score_adj=$(cat /proc/self/oom_score_adj)
    run_podman run --rm $IMAGE cat /proc/self/oom_score_adj
    is "$output" "$current_oom_score_adj" "different oom_score_adj in the container"
}

# CVE-2022-1227 : podman top joins container mount NS and uses nsenter from image
@test "podman top does not use nsenter from image" {
    keepid="--userns=keep-id"
    is_rootless || keepid=""

    tmpdir=$PODMAN_TMPDIR/build-test
    mkdir -p $tmpdir
    tmpbuilddir=$tmpdir/build
    mkdir -p $tmpbuilddir
    dockerfile=$tmpbuilddir/Dockerfile
    cat >$dockerfile <<EOF
FROM $IMAGE
RUN rm /usr/bin/nsenter; \
echo -e "#!/bin/sh\nfalse" >> /usr/bin/nsenter; \
chmod +x /usr/bin/nsenter
EOF

    test_image="cve_2022_1227_test"
    run_podman build -t $test_image $tmpbuilddir
    run_podman run -d ${keepid} $test_image top
    ctr="$output"
    run_podman top $ctr huser,user
    run_podman rm -f -t0 $ctr
    run_podman rmi $test_image
}

@test "podman create --security-opt" {
    run_podman create --security-opt no-new-privileges=true $IMAGE
    run_podman rm $output
    run_podman create --security-opt no-new-privileges:true $IMAGE
    run_podman rm $output
    run_podman create --security-opt no-new-privileges=false $IMAGE
    run_podman rm $output
    run_podman create --security-opt no-new-privileges $IMAGE
    run_podman rm $output
}

@test "podman run --device-read-bps" {
    skip_if_rootless "cannot use this flag in rootless mode"
    # this test is a triple check on blkio flags since they seem to sneak by the tests
    if is_cgroupsv2; then
        run_podman run -dt --device-read-bps=/dev/zero:1M $IMAGE top
        run_podman exec -it $output cat /sys/fs/cgroup/io.max
        is "$output" ".*1:5 rbps=1048576 wbps=max riops=max wiops=max" "throttle devices passed successfully.*"
    else
        run_podman run -dt --device-read-bps=/dev/zero:1M $IMAGE top
        run_podman exec -it $output cat /sys/fs/cgroup/blkio/blkio.throttle.read_bps_device
        is "$output" ".*1:5 1048576" "throttle devices passed successfully.*"
    fi
    run_podman container rm -fa
}

@test "podman run failed --rm " {
    port=$(random_free_port)

    # Run two containers with the same port bindings. The second must fail
    run_podman     run -p $port:80 --rm -d --name c_ok           $IMAGE top
    run_podman 126 run -p $port:80      -d --name c_fail_no_rm   $IMAGE top
    run_podman 126 run -p $port:80 --rm -d --name c_fail_with_rm $IMAGE top
    # Prior to #15060, the third container would still show up in ps -a
    run_podman ps -a --sort names --format '{{.Image}}--{{.Names}}'
    is "$output" "$IMAGE--c_fail_no_rm
$IMAGE--c_ok" \
       "podman ps -a shows running & failed containers, but not failed-with-rm"

    run_podman container rm -f -t 0 c_ok c_fail_no_rm
}

@test "podman run --attach stdin prints container ID" {
    ctr_name="container-$(random_string 5)"
    run_podman run --name $ctr_name --attach stdin $IMAGE echo hello
    run_output=$output
    run_podman inspect --format "{{.Id}}" $ctr_name
    ctr_id=$output
    is "$run_output" "$ctr_id" "Did not find container ID in the output"
    run_podman rm $ctr_name
}

# 15895: --privileged + --systemd = hide /dev/ttyNN
@test "podman run --privileged as root with systemd will not mount /dev/tty" {
    skip_if_rootless "this test only makes sense as root"

    # First, confirm that we _have_ /dev/ttyNN devices on the host.
    # ('skip' would be nicer in some sense... but could hide a regression.
    # Fedora, RHEL, Debian, Ubuntu, Gentoo, all have /dev/ttyN, so if
    # this ever triggers, it means a real problem we should know about.)
    vt_tty_devices_count=$(find /dev -regex '/dev/tty[0-9].*' | wc -w)
    assert "$vt_tty_devices_count" != "0" \
           "Expected at least one /dev/ttyN device on host"

    # Ok now confirm that without --systemd, podman exposes ttyNN devices
    run_podman run --rm -d --privileged $IMAGE ./pause
    cid="$output"

    run_podman exec $cid sh -c "find /dev -regex '/dev/tty[0-9].*' | wc -w"
    assert "$output" = "$vt_tty_devices_count" \
           "ls /dev/tty* without systemd; should have lots of ttyN devices"
    run_podman stop -t 0 $cid

    # Actual test for 15895: with --systemd, no ttyN devices are passed through
    run_podman run --rm -d --privileged --systemd=always $IMAGE ./pause
    cid="$output"

    run_podman exec $cid sh -c "find /dev -regex '/dev/tty[0-9].*' | wc -w"
    assert "$output" = "0" \
           "ls /dev/tty[0-9] with --systemd=always: should have no ttyN devices"

    run_podman stop -t 0 $cid
}

# 16925: --privileged + --systemd = share non-virtual-terminal TTYs
@test "podman run --privileged as root with systemd mounts non-vt /dev/tty devices" {
    skip_if_rootless "this test only makes sense as root"

    # First, confirm that we _have_ non-virtual terminal /dev/tty* devices on
    # the host.
    non_vt_tty_devices_count=$(find /dev -regex '/dev/tty[^0-9].*' | wc -w)
    if [ "$non_vt_tty_devices_count" -eq 0 ]; then
        skip "The server does not have non-vt TTY devices"
    fi

    # Verify that all the non-vt TTY devices got mounted in the container
    run_podman run --rm -d --privileged --systemd=always $IMAGE ./pause
    cid="$output"
    run_podman '?' exec $cid find /dev -regex '/dev/tty[^0-9].*'
    assert "$status" = 0 \
           "No non-virtual-terminal TTY devices got mounted in the container"
    assert "$(echo "$output" | wc -w)" = "$non_vt_tty_devices_count" \
           "Some non-virtual-terminal TTY devices are missing in the container"
    run_podman stop -t 0 $cid
}

@test "podman run read-only from containers.conf" {
    containersconf=$PODMAN_TMPDIR/containers.conf
    cat >$containersconf <<EOF
[containers]
read_only=true
EOF

    CONTAINERS_CONF="$containersconf" run_podman 1 run --rm $IMAGE touch /testro
    CONTAINERS_CONF="$containersconf" run_podman run --rm --read-only=false $IMAGE touch /testrw
    CONTAINERS_CONF="$containersconf" run_podman run --rm $IMAGE touch /tmp/testrw
    CONTAINERS_CONF="$containersconf" run_podman 1 run --rm --read-only-tmpfs=false $IMAGE touch /tmp/testro
}

@test "podman run bad --name" {
    randomname=$(random_string 30)
    run_podman 125 create --name "$randomname/bad" $IMAGE
    run_podman create --name "/$randomname" $IMAGE
    run_podman ps -a --filter name="^/$randomname$" --format '{{ .Names }}'
    is $output "$randomname" "Should be able to find container by name"
    run_podman rm "/$randomname"
    run_podman 125 create --name "$randomname/" $IMAGE
}

@test "podman run --net=host --cgroupns=host with read only cgroupfs" {
    # verify that the last /sys/fs/cgroup mount is read-only
    run_podman run --net=host --cgroupns=host --rm $IMAGE sh -c "grep ' / /sys/fs/cgroup ' /proc/self/mountinfo | tail -n 1 | grep '/sys/fs/cgroup ro'"
}

# vim: filetype=sh
