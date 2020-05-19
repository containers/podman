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

    # First line of podman-remote is "Client:<blank>".
    # Just delete it (i.e. remove the first entry from the 'lines' array)
    if is_remote; then
        if expr "${lines[0]}" : "Client:" >/dev/null; then
            lines=("${lines[@]:1}")
        fi
    fi

    is "${lines[0]}" "Version:[ ]\+[1-9][0-9.]\+" "Version line 1"
    is "$output" ".*Go Version: \+"               "'Go Version' in output"
    is "$output" ".*API Version: \+"		  "API version in output"
}


@test "podman can pull an image" {
    run_podman pull $IMAGE
}

# This is for development only; it's intended to make sure our timeout
# in run_podman continues to work. This test should never run in production
# because it will, by definition, fail.
@test "timeout" {
    if [ -z "$PODMAN_RUN_TIMEOUT_TEST" ]; then
        skip "define \$PODMAN_RUN_TIMEOUT_TEST to enable this test"
    fi
    PODMAN_TIMEOUT=10 run_podman run $IMAGE sleep 90
    echo "*** SHOULD NEVER GET HERE"
}


# Too many tests rely on jq for parsing JSON.
#
# If absolutely necessary, one could establish a convention such as
# defining PODMAN_TEST_SKIP_JQ=1 and adding a skip_if_no_jq() helper.
# For now, let's assume this is not absolutely necessary.
@test "jq is installed and produces reasonable output" {
    type -path jq >/dev/null || die "FATAL: 'jq' tool not found."

    run jq -r .a.b < <(echo '{ "a": { "b" : "you found me" } }')
    is "$output" "you found me" "sample invocation of 'jq'"
}

# vim: filetype=sh
