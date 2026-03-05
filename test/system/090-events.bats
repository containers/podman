#!/usr/bin/env bats   -*- bats -*-
#
# tests for podman events functionality
#

load helpers

@test "events with a filter by label" {
    cname=test-$(random_string 30 | tr A-Z a-z)
    labelname=$(random_string 10)
    labelvalue=$(random_string 15)

    run_podman run --label $labelname=$labelvalue --name $cname --rm $IMAGE ls

    expect=".* container start [0-9a-f]\+ (image=$IMAGE, name=$cname,.* ${labelname}=${labelvalue}"
    run_podman events --filter type=container -f container=$cname --filter label=${labelname}=${labelvalue} --filter event=start --stream=false
    is "$output" "$expect" "filtering by container name and label"

    # Same thing, but without the container-name filter
    run_podman system events -f type=container --filter label=${labelname}=${labelvalue} --filter event=start --stream=false
    is "$output" "$expect" "filtering just by label"

    # Now filter just by container name, no label
    run_podman events --filter type=container --filter container=$cname --filter event=start --stream=false
    is "$output" "$expect" "filtering just by container"
}

@test "truncate events" {
    cname=test-$(random_string 30 | tr A-Z a-z)

    run_podman run -d --name=$cname --rm $IMAGE echo hi
    id="$output"

    run_podman events --filter container=$cname --filter event=start --stream=false
    is "$output" ".* $id " "filtering by container name full id"

    truncID=${id:0:12}
    run_podman events --filter container=$cname --filter event=start --stream=false --no-trunc=false
    is "$output" ".* $truncID " "filtering by container name trunc id"

    # --no-trunc does not affect --format; we always get the full ID
    run_podman events --filter container=$cname --filter event=died --stream=false --format='{{.ID}}--{{.Image}}' --no-trunc=false
    assert "$output" = "${id}--${IMAGE}"
}

@test "image events" {
    skip_if_remote "remote does not support --events-backend"
    pushedDir=$PODMAN_TMPDIR/dir
    mkdir -p $pushedDir

    tarball=$PODMAN_TMPDIR/ball.tar

    run_podman image inspect --format "{{.ID}}" $IMAGE
    imageID="$output"

    t0=$(date --iso-8601=seconds)
    tag=registry.com/$(random_string 10 | tr A-Z a-z)

    # Force using the file backend since the journal backend is eating events
    # (see containers/podman/pull/10219#issuecomment-842325032).
    run_podman --events-backend=file push $IMAGE dir:$pushedDir
    run_podman --events-backend=file save $IMAGE -o $tarball
    run_podman --events-backend=file load -i $tarball
    run_podman --events-backend=file pull docker-archive:$tarball
    run_podman --events-backend=file tag $IMAGE $tag
    run_podman --events-backend=file untag $IMAGE $tag
    run_podman --events-backend=file tag $IMAGE $tag
    run_podman --events-backend=file rmi -f $imageID
    run_podman --events-backend=file load -i $tarball

    run_podman --events-backend=file events --stream=false --filter type=image --since $t0
    # Check that all expected events are present (order checked more precisely below)
    assert "$output" =~ "image push $imageID dir:$pushedDir" "push event"
    assert "$output" =~ "image save $imageID $tarball" "save event"
    assert "$output" =~ "image pull.*docker-archive:$tarball" "pull event"
    assert "$output" =~ "image tag $imageID $tag" "tag event"
    assert "$output" =~ "image untag $imageID $tag:latest" "untag event"
    assert "$output" =~ "image untag $imageID $IMAGE" "untag IMAGE event"
    assert "$output" =~ "image remove $imageID $imageID" "remove event"
    # Check for two loadfromarchive events
    run grep -c "image loadfromarchive.*$tarball" <<<"$output"
    assert "$output" = "2" "two loadfromarchive events"

    # With --format we can check the _exact_ output, not just substrings
    # Note: rmi generates remove and untag events that may appear in the file
    # in non-deterministic order due to race conditions during concurrent writes.
    run_podman --events-backend=file events --stream=false --filter type=image --since $t0 --format '{{.Status}}--{{.Name}}'

    # Check deterministic events in order
    assert "${lines[0]}" = "push--dir:$pushedDir" "event 0"
    assert "${lines[1]}" = "save--$tarball" "event 1"
    assert "${lines[2]}" = "loadfromarchive--$tarball" "event 2"
    assert "${lines[3]}" = "pull--docker-archive:$tarball" "event 3"
    assert "${lines[4]}" = "tag--$tag" "event 4"
    assert "${lines[5]}" = "untag--$tag:latest" "event 5"
    assert "${lines[6]}" = "tag--$tag" "event 6"

    # Events 7-9 are from rmi -f and may appear in any order
    # They should be: remove, untag (tag:latest), untag (IMAGE)
    local rmi_events="${lines[7]} ${lines[8]} ${lines[9]}"
    assert "$rmi_events" =~ "remove--$imageID" "remove event present"
    assert "$rmi_events" =~ "untag--$tag:latest" "untag tag:latest event present"
    assert "$rmi_events" =~ "untag--$IMAGE" "untag IMAGE event present"

    # Final event should be the second loadfromarchive
    assert "${lines[10]}" = "loadfromarchive--$tarball" "event 10"
    assert "${#lines[@]}" = "11" "Total lines of output"
}

function _events_disjunctive_filters() {
    local backend=$1

    # Regression test for #10507: make sure that filters with the same key are
    # applied in disjunction.
    t0=$(date --iso-8601=seconds)
    run_podman $backend run --name foo --rm $IMAGE ls
    run_podman $backend run --name bar --rm $IMAGE ls
    run_podman $backend events --stream=false --since=$t0 --filter container=foo --filter container=bar --filter event=start
    is "$output" ".* container start .* name=foo.*
.* container start .* name=bar.*"
}

@test "events with disjunctive filters - file" {
    skip_if_remote "remote does not support --events-backend"
    _events_disjunctive_filters --events-backend=file
}

@test "events with disjunctive filters - journald" {
    skip_if_remote "remote does not support --events-backend"
    skip_if_journald_unavailable "system does not support journald events"
    _events_disjunctive_filters --events-backend=journald
}

@test "events with file backend and journald logdriver with --follow failure" {
    skip_if_remote "remote does not support --events-backend"
    skip_if_journald_unavailable "system does not support journald events"
    run_podman --events-backend=file run --log-driver=journald --name=test $IMAGE echo hi
    is "$output" "hi" "Should support events-backend=file"

    run_podman 125 --events-backend=file logs --follow test
    is "$output" "Error: using --follow with the journald --log-driver but without the journald --events-backend (file) is not supported" \
       "Should fail with reasonable error message when events-backend and events-logger do not match"

}

@test "events with disjunctive filters - default" {
    _events_disjunctive_filters ""
}

@test "events with events_logfile_path in containers.conf" {
    skip_if_remote "remote does not support --events-backend"
    events_file=$PODMAN_TMPDIR/events.log
    containersconf=$PODMAN_TMPDIR/containers.conf
    cat >$containersconf <<EOF
[engine]
events_logfile_path="$events_file"
EOF
    CONTAINERS_CONF="$containersconf" run_podman --events-backend=file pull $IMAGE
    assert "$(< $events_file)" =~ "\"Name\":\"$IMAGE\"" "Image found in events"
}

function _populate_events_file() {
    # Create 100 duplicate entries to populate the events log file.
    local events_file=$1
    truncate --size=0 $events_file
    for i in {0..99}; do
        printf '{"Name":"busybox","Status":"pull","Time":"2022-04-06T11:26:42.7236679%02d+02:00","Type":"image","Attributes":null}\n' $i >> $events_file
    done
}

@test "events log-file rotation" {
    skip_if_remote "setting CONTAINERS_CONF logger options does not affect remote client"

    # Make sure that the events log file is (not) rotated depending on the
    # settings in containers.conf.

    # Config without a limit
    eventsFile=$PODMAN_TMPDIR/events.txt
    containersConf=$PODMAN_TMPDIR/containers.conf
    cat >$containersConf <<EOF
[engine]
events_logger="file"
events_logfile_path="$eventsFile"
EOF

    # Check that a non existing event file does not cause a hang (#15688)
    CONTAINERS_CONF=$containersConf run_podman events --stream=false

    _populate_events_file $eventsFile

    # Create events *without* a limit and make sure that it has not been
    # rotated/truncated.
    contentBefore=$(head -n100 $eventsFile)
    CONTAINERS_CONF=$containersConf run_podman run --rm $IMAGE true
    contentAfter=$(head -n100 $eventsFile)
    is "$contentBefore" "$contentAfter" "events file has not been rotated"

    # Repopulate events file
    rm $eventsFile
    _populate_events_file $eventsFile

    # Config with a limit
    rm $containersConf
    cat >$containersConf <<EOF
[engine]
events_logger="file"
events_logfile_path="$eventsFile"
# The limit of 4750 is the *exact* half of the initial events file.
events_logfile_max_size=4750
EOF

    # Create events *with* a limit and make sure that it has been
    # rotated/truncated.  Once rotated, the events file should only contain the
    # second half of its previous events plus the new ones.
    expectedContentAfterTruncation=$PODMAN_TMPDIR/truncated.txt

    run_podman create $IMAGE
    CONTAINERS_CONF=$containersConf run_podman rm $output
    tail -n52 $eventsFile >> $expectedContentAfterTruncation

    # Make sure the events file looks as expected.
    is "$(cat $eventsFile)" "$(cat $expectedContentAfterTruncation)" "events file has been rotated"

    # Make sure that `podman events` can read the file, and that it returns the
    # same amount of events.  We checked the contents before.
    CONTAINERS_CONF=$containersConf run_podman events --stream=false --since="2022-03-06T11:26:42.723667984+02:00"
    assert "${#lines[@]}" = 51 "Number of events returned"
    is "${lines[-2]}" ".* log-rotation $eventsFile"
}

# Prior to #15633, container labels would not appear in 'die' log events
@test "events - labels included in container die" {
    skip_if_remote "remote does not support --events-backend"
    local cname=c$(random_string 15)
    local lname=l$(random_string 10)
    local lvalue="v$(random_string 10) $(random_string 5)"

    run_podman 17 --events-backend=file run --rm \
               --name=$cname \
               --label=$lname="$lvalue" \
               $IMAGE sh -c 'exit 17'
    run_podman --events-backend=file events \
               --filter=container=$cname \
               --filter=status=died \
               --stream=false \
               --format="{{.Attributes.$lname}}"
    assert "$output" = "$lvalue" "podman-events output includes container label"
}

@test "events - backend none should error" {
    skip_if_remote "remote does not support --events-backend"

    run_podman 125 --events-backend none events
    is "$output" "Error: cannot read events with the \"none\" backend" "correct error message"
    run_podman 125 --events-backend none events --stream=false
    is "$output" "Error: cannot read events with the \"none\" backend" "correct error message"
}

function _events_container_create_inspect_data {
    containersConf=$PODMAN_TMPDIR/containers.conf
    cat >$containersConf <<EOF
[engine]
events_logger="$1"
events_container_create_inspect_data=true
EOF

    local cname=c$(random_string 15)
    t0=$(date --iso-8601=seconds)

    CONTAINERS_CONF=$containersConf run_podman create --name=$cname $IMAGE
    CONTAINERS_CONF=$containersConf run_podman container inspect --size=true $cname
    inspect_json=$(jq -r --tab . <<< "$output")

    CONTAINERS_CONF=$containersConf run_podman --events-backend=$1 events \
        --since="$t0"           \
        --filter=status=$cname  \
        --filter=status=create  \
        --stream=false          \
        --format="{{.ContainerInspectData}}"
    events_json=$(jq -r --tab . <<< "[$output]")
    assert "$events_json" = "$inspect_json" "JSON payload in event attributes is the same as the inspect one"

    # Make sure that the inspect data doesn't show by default in
    # podman-events.
    CONTAINERS_CONF=$containersConf run_podman --events-backend=$1 events \
        --since="$t0"           \
        --filter=status=$cname  \
        --filter=status=create  \
        --stream=false
    assert "$output" != ".*ConmonPidFile.*"
    assert "$output" != ".*EffectiveCaps.*"
}

@test "events - container inspect data - journald" {
    skip_if_remote "remote does not support --events-backend"
    skip_if_journald_unavailable

    _events_container_create_inspect_data journald
}

@test "events - container inspect data - file" {
    skip_if_remote "remote does not support --events-backend"

    _events_container_create_inspect_data file
}

@test "events - docker compat" {
    local cname=c$(random_string 15)
    t0=$(date --iso-8601=seconds)
    run_podman run --name=$cname --rm $IMAGE true
    run_podman events \
        --since="$t0"           \
        --filter=status=$cname  \
        --filter=status=die     \
        --stream=false
    is "${lines[0]}" ".* container died .* (image=$IMAGE, name=$cname, .*)"
}
