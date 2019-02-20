#!/usr/bin/env bats
#
# Simplest set of podman tests. If any of these fail, we have serious problems.
#

load helpers

# Override standard setup! We don't yet trust podman-images or podman-rm
function setup() {
    :
}

@test "podman version emits reasonable output" {
    run_podman version

    is "${lines[0]}" "Version:[ ]\+[1-9][0-9.]\+" "Version line 1"

    is "$output" ".*Go Version: \+"               "'Go Version' in output"

    # FIXME: enable for 1.1
#    is "$output" ".*RemoteAPI Version: \+"        "API version in output"
}


@test "podman can pull an image" {
    run_podman pull $PODMAN_TEST_IMAGE_FQN
}

# This is for development only; it's intended to make sure our timeout
# in run_podman continues to work. This test should never run in production
# because it will, by definition, fail.
@test "timeout" {
    if [ -z "$PODMAN_RUN_TIMEOUT_TEST" ]; then
        skip "define \$PODMAN_RUN_TIMEOUT_TEST to enable this test"
    fi
    PODMAN_TIMEOUT=10 run_podman run $PODMAN_TEST_IMAGE_FQN sleep 90
    echo "*** SHOULD NEVER GET HERE"
}
