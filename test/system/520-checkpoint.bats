#!/usr/bin/env bats   -*- bats -*-
#
# test podman checkpoint. Similar in many ways to our pause tests.
#

load helpers

CHECKED_ROOTLESS=
function setup() {
    # FIXME: https://bugs.launchpad.net/ubuntu/+source/linux/+bug/1857257
    # TL;DR they keep fixing it then breaking it again. There's a test we
    # could run to see if it's fixed, but it's way too complicated. Since
    # integration tests also skip checkpoint tests on Ubuntu, do the same here.
    if is_ubuntu; then
        skip "FIXME: checkpointing broken in Ubuntu 2004, 2104, 2110, 2204, ..."
    fi

    # None of these tests work rootless....
    if is_rootless; then
        # ...however, is that a genuine cast-in-stone limitation, or one
        # that can some day be fixed? If one day some PR removes that
        # restriction, fail loudly here, so the developer can enable tests.
        if [[ -n "$CHECKED_ROOTLESS" ]]; then
            run_podman '?' container checkpoint -l
            is "$output" "Error: checkpointing a container requires root" \
               "Confirming that rootless checkpoint doesn't work. If that changed, please reexamine this test file!"
            CHECKED_ROOTLESS=y
        fi
        skip "checkpoint does not work rootless"
    fi

    basic_setup
}

function teardown() {
    run_podman '?' volume rm myvol

    basic_teardown
}

@test "podman checkpoint - basic test" {
    run_podman run -d $IMAGE sh -c 'while :;do cat /proc/uptime; sleep 0.1;done'
    local cid="$output"

    # Wait for container to start emitting output
    wait_for_output '[1-9]\+' $cid

    # Checkpoint, and confirm via inspect
    run_podman container checkpoint $cid
    # FIXME: remove the `.*` prefix after fix packaged for https://github.com/checkpoint-restore/criu/pull/1706
    is "$output" ".*$cid" "podman container checkpoint"

    run_podman container inspect \
               --format '{{.State.Status}}:{{.State.Running}}:{{.State.Paused}}:{{.State.Checkpointed}}' $cid
    is "$output" "exited:false:false:true" "State. Status:Running:Pause:Checkpointed"

    # Plan A was to do something similar to 080-pause.bats: sleep for long
    # enough to cause a gap in the timestamps in the log. But checkpoint
    # doesn't seem to work like that: upon restore, even if we sleep a long
    # time, the newly-started container seems to pick back up close to
    # where it left off. (Maybe it's something about /proc/uptime?)
    # Anyway, scratch Plan A. Plan B is simply to make sure that the
    # restarted container spits something out.
    run_podman logs $cid
    local nlines_before="${#lines[*]}"

    # Restart immediately and confirm state
    run_podman container restore $cid
    is "$output" "$cid" "podman container restore"

    # Note that upon restore, .Checkpointed reverts to false (#12117)
    run_podman container inspect \
               --format '{{.State.Status}}:{{.State.Running}}:{{.State.Paused}}:{{.State.Checkpointed}}' $cid
    is "$output" "running:true:false:false" \
       "State. Status:Running:Pause:Checkpointed"

    # Pause briefly to let restarted container emit some output
    sleep 0.3

    # Get full logs, and make sure something changed
    run_podman logs $cid
    local nlines_after="${#lines[*]}"
    assert $nlines_after -gt $nlines_before \
           "Container failed to output new lines after first restore"

    # Same thing again: test for https://github.com/containers/crun/issues/756
    # in which, after second checkpoint/restore, we lose logs
    run_podman container checkpoint $cid
    run_podman container logs $cid
    nlines_before="${#lines[*]}"
    run_podman container restore $cid

    # Give container time to write new output; then confirm that something
    # was emitted
    sleep 0.3
    run_podman container logs $cid
    nlines_after="${#lines[*]}"
    assert $nlines_after -gt $nlines_before \
           "stdout went away after second restore (crun issue 756)"

    run_podman rm -t 0 -f $cid
}


@test "podman checkpoint --export, with volumes" {
    skip_if_remote "Test uses --root/--runroot, which are N/A over remote"

    # Create a root in tempdir. We will run a container here.
    local p_root=${PODMAN_TMPDIR}/testroot/root
    local p_runroot=${PODMAN_TMPDIR}/testroot/runroot
    mkdir -p $p_root $p_runroot

    # To avoid network pull, copy $IMAGE straight to temp root
    local p_opts="--root $p_root --runroot $p_runroot --events-backend file"
    run_podman         save -o $PODMAN_TMPDIR/image.tar $IMAGE
    run_podman $p_opts load -i $PODMAN_TMPDIR/image.tar

    # Create a volume, find unused network port, and create a webserv container
    run_podman $p_opts volume create myvol
    local cname=c_$(random_string 10)
    local host_port=$(random_free_port)
    local server=http://127.0.0.1:$host_port

    run_podman $p_opts run -d --name $cname --volume myvol:/myvol \
               -p $host_port:80 \
               -w /myvol \
               $IMAGE sh -c "/bin/busybox-extras httpd -p 80;echo $cname >cname;echo READY;while :;do cat /proc/uptime >mydate.tmp;mv -f mydate.tmp mydate;sleep 0.1;done"
    local cid="$output"
    _PODMAN_TEST_OPTS="$p_opts" wait_for_ready $cid

    # Confirm that container responds
    run curl --max-time 3 -s $server/cname
    is "$output" "$cname" "curl $server/cname"
    run curl --max-time 3 -s $server/mydate
    local date_oldroot="$output"

    # Checkpoint...
    run_podman $p_opts container checkpoint \
               --ignore-rootfs \
               --export=$PODMAN_TMPDIR/$cname.tar.gz \
               $cname

    # ...confirm that port is now closed
    run curl --max-time 1 -s $server/mydate
    is "$status" "7" "cannot connect to port $host_port while container is down"

    # ...now restore it to our regular root
    run_podman container restore --import=$PODMAN_TMPDIR/$cname.tar.gz
    is "$output" "$cid"

    # Inspect (on regular root). Note that, unlike the basic test above,
    # .State.Checkpointed here is *false*.
    run_podman container inspect \
               --format '{{.State.Status}}:{{.State.Running}}:{{.State.Paused}}:{{.State.Checkpointed}}' $cname
    is "$output" "running:true:false:false" "State. Status:Running:Pause:Checkpointed"

    # Pause a moment to let the restarted container update the timestamp file
    sleep .3
    run curl --max-time 3 -s $server/mydate
    local date_newroot="$output"
    assert "$date_newroot" != "$date_oldroot" \
           "Restored container did not update the timestamp file"

    run_podman exec $cid cat /myvol/cname
    is "$output" "$cname" "volume transferred fine"

    run_podman rm -t 0 -f $cid
    run_podman volume rm -f myvol
}

# FIXME: test --leave-running

@test "podman checkpoint --file-locks" {
    action='flock test.lock sh -c "while [ -e /wait ];do sleep 0.5;done;for i in 1 2 3;do echo \$i;sleep 0.5;done"'
    run_podman run -d $IMAGE sh -c "touch /wait; touch test.lock; echo READY; $action & $action & wait"
    local cid="$output"

    # Wait for container to start emitting output
    wait_for_ready $cid

    # Checkpoint, and confirm via inspect
    run_podman container checkpoint --file-locks $cid
    is "$output" "$cid" "podman container checkpoint"

    run_podman container inspect \
               --format '{{.State.Status}}:{{.State.Running}}:{{.State.Paused}}:{{.State.Checkpointed}}' $cid
    is "$output" "exited:false:false:true" "State. Status:Running:Pause:Checkpointed"

    # Restart immediately and confirm state
    run_podman container restore --file-locks $cid
    is "$output" "$cid" "podman container restore"

    # Signal the container to continue; this is where the 1-2-3s will come from
    run_podman exec $cid rm /wait

    # Wait for the container to stop
    run_podman wait $cid

    run_podman logs $cid
    trim=$(sed -z -e 's/[\r\n]\+//g' <<<"$output")
    is "$trim" "READY123123" "File lock restored"
}
# vim: filetype=sh
