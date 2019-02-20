#!/usr/bin/env bats

load helpers

@test "podman ps - basic tests" {
    rand_name=$(random_string 30)

    run_podman run -d --name $rand_name $PODMAN_TEST_IMAGE_FQN sleep 5
    cid=$output
    is "$cid" "[0-9a-f]\{64\}$"

    # Special case: formatted ps
    run_podman ps --no-trunc \
               --format '{{.ID}} {{.Image}} {{.Command}} {{.Names}}'
    is "$output" "$cid $PODMAN_TEST_IMAGE_FQN sleep 5 $rand_name" "podman ps"


    # Plain old regular ps
    run_podman ps
    is "${lines[1]}" \
       "${cid:0:12} \+$PODMAN_TEST_IMAGE_FQN \+sleep [0-9]\+ .*second.* $cname"\
       "output from podman ps"

    # OK. Wait for sleep to finish...
    run_podman wait $cid

    # ...then make sure container shows up as stopped
    run_podman ps -a
    is "${lines[1]}" \
       "${cid:0:12} \+$PODMAN_TEST_IMAGE_FQN *sleep .* Exited .* $rand_name" \
       "podman ps -a"



    run_podman rm $cid
}
