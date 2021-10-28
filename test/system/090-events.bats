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
    run_podman events --filter type=container --filter container=$cname --filter label=${labelname}=${labelvalue} --filter event=start --stream=false
    is "$output" "$expect" "filtering by container name and label"

    # Same thing, but without the container-name filter
    run_podman events --filter type=container --filter label=${labelname}=${labelvalue} --filter event=start --stream=false
    is "$output" "$expect" "filtering just by label"

    # Now filter just by container name, no label
    run_podman events --filter type=container --filter container=$cname --filter event=start --stream=false
    is "$output" "$expect" "filtering just by label"
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
    run_podman --events-backend=file rmi $tag

    run_podman --events-backend=file events --stream=false --filter type=image --since $t0
    is "$output" ".*image push $imageID dir:$pushedDir
.*image save $imageID $tarball
.*image loadfromarchive *$tarball
.*image pull *docker-archive:$tarball
.*image tag $imageID $tag
.*image untag $imageID $tag:latest
.*image tag $imageID $tag
.*image remove $imageID $tag.*" \
       "podman events"
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
    is "$output" "Error: using --follow with the journald --log-driver but without the journald --events-backend (file) is not supported" "Should fail with reasonable error message when events-backend and events-logger do not match"

}

@test "events with disjunctive filters - default" {
    _events_disjunctive_filters ""
}
