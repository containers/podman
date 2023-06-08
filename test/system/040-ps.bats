#!/usr/bin/env bats

load helpers

@test "podman ps - basic tests" {
    rand_name=$(random_string 30)

    run_podman ps --noheading
    is "$output" "" "baseline: empty results from ps --noheading"

    run_podman run -d --name $rand_name $IMAGE sleep 5
    cid=$output
    is "$cid" "[0-9a-f]\{64\}$"

    # Special case: formatted ps
    run_podman ps --no-trunc \
               --format '{{.ID}} {{.Image}} {{.Command}} {{.Names}} {{.State}}'
    is "$output" "$cid $IMAGE sleep 5 $rand_name running" "podman ps"


    # Plain old regular ps
    run_podman ps
    is "${lines[1]}" \
       "${cid:0:12} \+$IMAGE \+sleep [0-9]\+ .*second.* $cname"\
       "output from podman ps"

    # OK. Wait for sleep to finish...
    run_podman wait $cid

    # ...then make sure container shows up as stopped
    run_podman ps -a
    is "${lines[1]}" \
       "${cid:0:12} \+$IMAGE *sleep .* Exited .* $rand_name" \
       "podman ps -a"

    run_podman rm $cid
}

@test "podman ps --filter" {
    local -A cid

    # Create three containers, each of whose CID begins with a different char
    run_podman run -d --name running $IMAGE top
    cid[running]=$output

    cid[stopped]=$output
    while [[ ${cid[stopped]:0:1} == ${cid[running]:0:1} ]]; do
        run_podman rm -f stopped
        run_podman run -d --name stopped $IMAGE true
        cid[stopped]=$output
        run_podman wait stopped
    done

    cid[failed]=${cid[stopped]}
    while [[ ${cid[failed]:0:1} == ${cid[running]:0:1} ]] || [[ ${cid[failed]:0:1} == ${cid[stopped]:0:1} ]]; do
        run_podman rm -f failed
        run_podman run -d --name failed $IMAGE false
        cid[failed]=$output
        run_podman wait failed
    done

    # This one is never tested in the id filter, so its cid can be anything
    run_podman create --name created $IMAGE echo hi
    cid[created]=$output

    # For debugging
    run_podman ps -a

    run_podman ps --filter name=running --format '{{.ID}}'
    is "$output" "${cid[running]:0:12}" "filter: name=running"

    # Stopped container should not appear (because we're not using -a)
    run_podman ps --filter name=stopped --format '{{.ID}}'
    is "$output" "" "filter: name=stopped (without -a)"

    # Again, but with -a
    run_podman ps -a --filter name=stopped --format '{{.ID}}'
    is "$output" "${cid[stopped]:0:12}" "filter: name=stopped (with -a)"

    run_podman ps --filter status=stopped --format '{{.Names}}' --sort names
    is "${lines[0]}" "failed"  "status=stopped: 1 of 2"
    is "${lines[1]}" "stopped" "status=stopped: 2 of 2"

    run_podman ps --filter status=exited --filter exited=0 --format '{{.Names}}'
    is "$output" "stopped" "exited=0"

    run_podman ps --filter status=exited --filter exited=1 --format '{{.Names}}'
    is "$output" "failed" "exited=1"

    run_podman ps --filter status=created --format '{{.Names}}'
    is "$output" "created" "state=created"

    # Multiple statuses allowed; and test sort=created
    run_podman ps -a --filter status=exited --filter status=running \
               --format '{{.Names}}' --sort created
    is "${lines[0]}" "running" "status=stopped: 1 of 3"
    is "${lines[1]}" "stopped" "status=stopped: 2 of 3"
    is "${lines[2]}" "failed"  "status=stopped: 3 of 3"

    # ID filtering: if filter is only hex chars, it's a prefix; if it has
    # anything else, it's a regex
    run_podman rm created
    for state in running stopped failed; do
        local test_cid=${cid[$state]}
        for prefix in ${test_cid:0:1} ${test_cid:0:2} ${test_cid:0:13}; do
            # Test lower-case (default), upper-case, and with '^' anchor
            for uclc in ${prefix,,} ${prefix^^} "^$prefix"; do
                run_podman ps -a --filter id=$uclc --format '{{.Names}}'
                assert "$output" = "$state" "ps --filter id=$uclc"
            done
        done

        # Regex check
        local f="^[^${test_cid:0:1}]"
        run_podman ps -a --filter id="$f" --format '{{.Names}}'
        assert "${#lines[*]}" == "2" "filter id=$f: number of lines"
        assert "$output" !~ $state   "filter id=$f: '$state' not in results"
    done

    # All CIDs will have hex characters
    run_podman ps -a --filter id="[0-9a-f]" --format '{{.Names}}' --sort names
    assert "${lines[0]}" == "failed"  "filter id=[0-9a-f], line 1"
    assert "${lines[1]}" == "running" "filter id=[0-9a-f], line 2"
    assert "${lines[2]}" == "stopped" "filter id=[0-9a-f], line 3"

    run_podman ps -a --filter id="[^0-9a-f]" --noheading
    assert "$output" = "" "id=[^0-9a-f], should match no containers"

    # Finally, multiple filters
    run_podman ps -a --filter id=${cid[running]} --filter id=${cid[failed]} \
               --format '{{.Names}}' --sort names
    assert "${lines[0]}" == "failed"  "filter id=running+failed, line 1"
    assert "${lines[1]}" == "running" "filter id=running+failed, line 2"

    run_podman stop -t 1 running
    run_podman rm -a
}

@test "podman ps --external" {

    # Setup: ensure that we have no hidden storage containers
    run_podman ps --external
    is "${#lines[@]}" "1" "setup check: no storage containers at start of test"

    # Force a buildah timeout; this leaves a buildah container behind
    local t0=$SECONDS
    PODMAN_TIMEOUT=5 run_podman 124 build -t thiswillneverexist - <<EOF
FROM $IMAGE
RUN touch /intermediate.image.to.be.pruned
RUN sleep 30
EOF
    local t1=$SECONDS
    local delta_t=$((t1 - t0))
    assert $delta_t -le 10 \
           "podman build did not get killed within 10 seconds"

    run_podman ps -a
    is "${#lines[@]}" "1" "podman ps -a does not see buildah containers"

    run_podman ps --external
    is "${#lines[@]}" "3" "podman ps -a --external sees buildah containers"
    is "${lines[1]}" \
       "[0-9a-f]\{12\} \+$IMAGE *buildah .* seconds ago .* Storage .* ${PODMAN_TEST_IMAGE_NAME}-working-container" \
       "podman ps --external"

    # 'rm -a' should be a NOP
    run_podman rm -a
    run_podman ps --external
    is "${#lines[@]}" "3" "podman ps -a --external sees buildah containers"

    # Cannot prune intermediate image as it's being used by a buildah
    # container.
    run_podman image prune -f
    is "$output" "" "No image is pruned"

    # --external for removing buildah containers.
    run_podman image prune -f --external
    is "${#lines[@]}" "1" "Image used by build container is pruned"

    # One buildah container has been removed.
    run_podman ps --external
    is "${#lines[@]}" "2" "podman ps -a --external sees buildah containers"

    cid="${lines[1]:0:12}"

    # We can't rm it without -f, but podman should issue a helpful message
    run_podman 2 rm "$cid"
    is "$output" "Error: container .* is mounted and cannot be removed without using force: container state improper" "podman rm <buildah container> without -f"

    # With -f, we can remove it.
    run_podman rm -t 0 -f "$cid"

    run_podman ps --external
    is "${#lines[@]}" "1" "storage container has been removed"
}



# vim: filetype=sh
