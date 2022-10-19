#!/usr/bin/env bats   -*- bats -*-
#
# Tests that require 'podman system service' but no other systemd aspects

load helpers
load helpers.systemd

SERVICE_NAME="podman-service-$(random_string)"

function teardown() {
    # Ignore exit status: this is just a backup stop in case tests failed
    run systemctl stop "$SERVICE_NAME"

    basic_teardown
}

@test "podman systerm service <bad_scheme_uri> returns error" {
    skip_if_remote "podman system service unavailable over remote"
    run_podman 125 system service localhost:9292
    is "$output" "Error: API Service endpoint scheme \"localhost\" is not supported. Try tcp://localhost:9292 or unix:/localhost:9292"

    run_podman 125 system service myunix.sock
    is "$output" "Error: API Service endpoint scheme \"\" is not supported. Try tcp://myunix.sock or unix:/myunix.sock"
}

@test "podman-system-service containers survive service stop" {
    skip_if_remote "podman system service unavailable over remote"
    local runtime=$(podman_runtime)
    if [[ "$runtime" != "crun" ]]; then
        skip "survival code only implemented in crun; you're using $runtime"
    fi

    port=$(random_free_port)
    URL=tcp://127.0.0.1:$port

    systemd-run --unit=$SERVICE_NAME $PODMAN system service $URL --time=0
    wait_for_port 127.0.0.1 $port

    # Start a long-running container.
    cname=keeps-running
    run_podman --url $URL run -d --name $cname $IMAGE top -d 2

    run_podman container inspect -l --format "{{.State.Running}}"
    is "$output" "true" "This should never fail"

    systemctl stop $SERVICE_NAME

    run_podman container inspect $cname --format "{{.State.Running}}"
    is "$output" "true" "Container is still running after podman server stops"

    run_podman rm -f -t 0 $cname
}

# This doesn't actually test podman system service, but we require it,
# so least-awful choice is to run from this test file.
@test "podman --host / -H options" {
    port=$(random_free_port)
    URL=tcp://127.0.0.1:$port

    # %%-remote makes this run real podman even when testing podman-remote
    systemd-run --unit=$SERVICE_NAME ${PODMAN%%-remote*} system service $URL --time=0
    wait_for_port 127.0.0.1 $port

    for opt in --host -H; do
        run_podman $opt $URL info --format '{{.Host.RemoteSocket.Path}}'
        is "$output" "$URL" "RemoteSocket.Path using $opt"
    done

    systemctl stop $SERVICE_NAME
}
