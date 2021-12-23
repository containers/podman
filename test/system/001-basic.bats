#!/usr/bin/env bats
#
# Simplest set of podman tests. If any of these fail, we have serious problems.
#

load helpers

# Override standard setup! We don't yet trust podman-images or podman-rm
function setup() {
    :
}

#### DO NOT ADD ANY TESTS HERE! ADD NEW TESTS AT BOTTOM!

@test "podman version emits reasonable output" {
    run_podman version

    # First line of podman version is "Client: *Podman Engine".
    # Just delete it (i.e. remove the first entry from the 'lines' array)
    if expr "${lines[0]}" : "Client: *" >/dev/null; then
        lines=("${lines[@]:1}")
    fi

    is "${lines[0]}" "Version:[ ]\+[1-9][0-9.]\+" "Version line 1"
    is "$output" ".*Go Version: \+"               "'Go Version' in output"
    is "$output" ".*API Version: \+"		  "API version in output"

    # Test that build date is reasonable, e.g. after 2019-01-01
    local built=$(expr "$output" : ".*Built: \+\(.*\)" | head -n1)
    local built_t=$(date --date="$built" +%s)
    if [ $built_t -lt 1546300800 ]; then
        die "Preposterous 'Built' time in podman version: '$built'"
    fi
}


@test "podman --context emits reasonable output" {
    # All we care about here is that the command passes
    run_podman --context=default version

    # This one must fail
    run_podman 125 --context=swarm version
    is "$output" \
       "Error: podman does not support swarm, the only --context value allowed is \"default\"" \
       "--context=default or fail"
}

@test "podman can pull an image" {
    run_podman pull $IMAGE

    # Also make sure that the tag@digest syntax is supported.
    run_podman inspect --format "{{ .Digest }}" $IMAGE
    digest=$output
    run_podman pull $IMAGE@$digest

    # Now untag the digest reference again.
    run_podman untag $IMAGE $IMAGE@$digest

    # Make sure the original image is still present (#11557).
    run_podman image exists $IMAGE
}

# PR #7212: allow --remote anywhere before subcommand, not just as 1st flag
@test "podman-remote : really is remote, works as --remote option" {
    if ! is_remote; then
        skip "only applicable on podman-remote"
    fi

    # First things first: make sure our podman-remote actually is remote!
    run_podman version
    is "$output" ".*Server:" "the given podman path really contacts a server"

    # $PODMAN may be a space-separated string, e.g. if we include a --url.
    # Split it into its components; remove "-remote" from the command path;
    # and preserve any other args if present.
    local -a podman_as_array=($PODMAN)
    local    podman_path=${podman_as_array[0]}
    local    podman_non_remote=${podman_path%%-remote}
    local -a podman_args=("${podman_as_array[@]:1}")

    # This always worked: running "podman --remote ..."
    PODMAN="${podman_non_remote} --remote ${podman_args[@]}" run_podman version
    is "$output" ".*Server:" "podman --remote: contacts server"

    # This was failing: "podman --foo --bar --remote".
    PODMAN="${podman_non_remote} --log-level=error ${podman_args[@]} --remote" run_podman version
    is "$output" ".*Server:" "podman [flags] --remote: contacts server"

    # ...but no matter what, --remote is never allowed after subcommand
    PODMAN="${podman_non_remote} ${podman_args[@]}" run_podman 125 version --remote
    is "$output" "Error: unknown flag: --remote" "podman version --remote"
}

@test "podman-remote: defaults" {
    skip_if_remote "only applicable on a local run"

    # By default, podman should include '--remote' in its help output
    run_podman --help
    is "$output" ".* --remote " "podman --help includes the --remote option"

    # When it detects CONTAINER_HOST or _CONNECTION, --remote is not an option
    CONTAINER_HOST=foobar run_podman --help
    if grep -- " --remote " <<<"$output"; then
        die "podman --help, with CONTAINER_HOST set, is showing --remote"
    fi

    CONTAINER_CONNECTION=foobar run_podman --help
    if grep -- " --remote " <<<"$output"; then
        die "podman --help, with CONTAINER_CONNECTION set, is showing --remote"
    fi
}

# Check that just calling "podman-remote" prints the usage message even
# without a running endpoint. Use "podman --remote" for this as this works the same.
@test "podman-remote: check for command usage message without a running endpoint" {
    if is_remote; then
        skip "only applicable on a local run since this requires no endpoint"
    fi

    run_podman 125 --remote
    is "$output" ".*Usage:" "podman --remote show usage message without running endpoint"
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

@test "podman --log-level recognizes log levels" {
    run_podman 1 --log-level=telepathic info
    is "$output" 'Log Level "telepathic" is not supported.*'
    run_podman --log-level=trace   info
    run_podman --log-level=debug   info
    run_podman --log-level=info    info
    run_podman --log-level=warn    info
    run_podman --log-level=warning info
    run_podman --log-level=error   info
    run_podman --log-level=fatal   info
    run_podman --log-level=panic   info
}

# vim: filetype=sh
