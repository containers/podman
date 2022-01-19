#!/usr/bin/env bats   -*- bats -*-
#
# Test specific configuration options and overrides
#

load helpers

@test "podman CONTAINERS_CONF - CONTAINERS_CONF in conmon" {
    skip_if_remote "can't check conmon environment over remote"

    # Get the normal runtime for this host
    run_podman info --format '{{ .Host.OCIRuntime.Name }}'
    runtime="$output"
    run_podman info --format "{{ .Host.OCIRuntime.Path }}"
    ocipath="$output"

    # Make an innocuous containers.conf in a non-standard location
    conf_tmp="$PODMAN_TMPDIR/containers.conf"
    cat >$conf_tmp <<EOF
[engine]
runtime="$runtime"
[engine.runtimes]
$runtime = ["$ocipath"]
EOF
    CONTAINERS_CONF="$conf_tmp" run_podman run -d $IMAGE sleep infinity
    cid="$output"

    CONTAINERS_CONF="$conf_tmp" run_podman inspect "$cid" --format "{{ .State.ConmonPid }}"
    conmon="$output"

    output="$(tr '\0' '\n' < /proc/$conmon/environ | grep '^CONTAINERS_CONF=')"
    is "$output" "CONTAINERS_CONF=$conf_tmp"

    # Clean up
    # Oddly, sleep can't be interrupted with SIGTERM, so we need the
    # "-f -t 0" to force a SIGKILL
    CONTAINERS_CONF="$conf_tmp" run_podman rm -f -t 0 "$cid"
}

@test "podman CONTAINERS_CONF - override runtime name" {
    skip_if_remote "Can't set CONTAINERS_CONF over remote"

    # Get the path of the normal runtime
    run_podman info --format "{{ .Host.OCIRuntime.Path }}"
    ocipath="$output"

    export conf_tmp="$PODMAN_TMPDIR/nonstandard_runtime_name.conf"
    cat > $conf_tmp <<EOF
[engine]
runtime = "nonstandard_runtime_name"
[engine.runtimes]
nonstandard_runtime_name = ["$ocipath"]
EOF

    CONTAINERS_CONF="$conf_tmp" run_podman run -d --rm $IMAGE true
    cid="$output"

    # We need to wait for the container to finish before we can check
    # if it was cleaned up properly.  But in the common case that the
    # container completes fast, and the cleanup *did* happen properly
    # the container is now gone.  So, we need to ignore "no such
    # container" errors from podman wait.
    CONTAINERS_CONF="$conf_tmp" run_podman '?' wait "$cid"
    if [[ $status != 0 ]]; then
	is "$output" "Error:.*no such container" "unexpected error from podman wait"
    fi

    # The --rm option means the container should no longer exist.
    # However https://github.com/containers/podman/issues/12917 meant
    # that the container cleanup triggered by conmon's --exit-cmd
    # could fail, leaving the container in place.
    #
    # We verify that the container is indeed gone, by checking that a
    # podman rm *fails* here - and it has the side effect of cleaning
    # up in the case this test fails.
    CONTAINERS_CONF="$conf_tmp" run_podman 1 rm "$cid"
    is "$output" "Error:.*no such container"
}

# vim: filetype=sh
