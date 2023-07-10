#!/usr/bin/env bats

load helpers
load helpers.network
load helpers.registry

# Regression test for #8931
@test "podman images - bare manifest list" {
    # Create an empty manifest list and list images.

    run_podman inspect --format '{{.ID}}' $IMAGE
    iid=$output

    run_podman manifest create test:1.0
    mid=$output
    run_podman manifest inspect --verbose $mid
    is "$output" ".*\"mediaType\": \"application/vnd.docker.distribution.manifest.list.v2+json\"" "--insecure is a noop want to make sure manifest inspect is successful"
    run_podman manifest inspect -v $mid
    is "$output" ".*\"mediaType\": \"application/vnd.docker.distribution.manifest.list.v2+json\"" "--insecure is a noop want to make sure manifest inspect is successful"
    run_podman images --format '{{.ID}}' --no-trunc
    is "$output" ".*sha256:$iid" "Original image ID still shown in podman-images output"
    run_podman rmi test:1.0
}

@test "podman manifest --tls-verify and --authfile" {
    skip_if_remote "running a local registry doesn't work with podman-remote"
    start_registry
    authfile=${PODMAN_LOGIN_WORKDIR}/auth-$(random_string 10).json
    run_podman login --tls-verify=false \
               --username ${PODMAN_LOGIN_USER} \
               --password-stdin \
               --authfile=$authfile \
               localhost:${PODMAN_LOGIN_REGISTRY_PORT} <<<"${PODMAN_LOGIN_PASS}"
    is "$output" "Login Succeeded!" "output from podman login"

    manifest1="localhost:${PODMAN_LOGIN_REGISTRY_PORT}/test:1.0"
    run_podman manifest create $manifest1
    mid=$output
    run_podman manifest push --authfile=$authfile \
        --tls-verify=false $mid \
        $manifest1
    run_podman manifest rm $manifest1

    # Default is to require TLS; also test explicit opts
    for opt in '' '--insecure=false' '--tls-verify=true' "--authfile=$authfile"; do
        run_podman 125 manifest inspect $opt $manifest1
        assert "$output" =~ "Error: reading image \"docker://$manifest1\": pinging container registry localhost:${PODMAN_LOGIN_REGISTRY_PORT}:.*x509" \
               "TLE check: fails (as expected) with ${opt:-default}"
    done

    run_podman manifest inspect --authfile=$authfile --tls-verify=false $manifest1
    is "$output" ".*\"mediaType\": \"application/vnd.docker.distribution.manifest.list.v2+json\"" "Verify --tls-verify=false --authfile works against an insecure registry"
    run_podman manifest inspect --authfile=$authfile --insecure $manifest1
    is "$output" ".*\"mediaType\": \"application/vnd.docker.distribution.manifest.list.v2+json\"" "Verify --insecure --authfile works against an insecure registry"
    REGISTRY_AUTH_FILE=$authfile run_podman manifest inspect --tls-verify=false $manifest1
    is "$output" ".*\"mediaType\": \"application/vnd.docker.distribution.manifest.list.v2+json\"" "Verify --tls-verify=false with REGISTRY_AUTH_FILE works against an insecure registry"
}

# vim: filetype=sh
