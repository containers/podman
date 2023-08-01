#!/usr/bin/env bats

load helpers
load helpers.network
load helpers.registry

# Helper function for several of the tests which verifies compression.
#
#  Usage:  validate_instance_compression INDEX MANIFEST ARCH COMPRESSION
#
#     INDEX             instance which needs to be verified in
#                       provided manifest list.
#
#     MANIFEST          OCI manifest specification in json format
#
#     ARCH              instance architecture
#
#     COMPRESSION       compression algorithm name; e.g "zstd".
#
function validate_instance_compression {
  case $4 in

   gzip)
    run jq -r '.manifests['$1'].annotations' <<< $2
    # annotation is `null` for gzip compression
    assert "$output" = "null" ".manifests[$1].annotations (null means gzip)"
    ;;

  zstd)
    # annotation `'"io.github.containers.compression.zstd": "true"'` must be there for zstd compression
    run jq -r '.manifests['$1'].annotations."io.github.containers.compression.zstd"' <<< $2
    assert "$output" = "true" ".manifests[$1].annotations.'io.github.containers.compression.zstd' (io.github.containers.compression.zstd must be set)"
    ;;
  esac

  run jq -r '.manifests['$1'].platform.architecture' <<< $2
  assert "$output" = $3 ".manifests[$1].platform.architecture"
}

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

@test "manifest list --add-compression with zstd" {
    if ! type -p skopeo; then
        skip "skopeo not available"
    fi
    skip_if_remote "running a local registry doesn't work with podman-remote"
    start_registry

    tmpdir=$PODMAN_TMPDIR/build-test
    mkdir -p $tmpdir
    dockerfile=$tmpdir/Dockerfile
    cat >$dockerfile <<EOF
FROM alpine
EOF
    authfile=${PODMAN_LOGIN_WORKDIR}/auth-$(random_string 10).json
    run_podman login --tls-verify=false \
               --username ${PODMAN_LOGIN_USER} \
               --password-stdin \
               --authfile=$authfile \
               localhost:${PODMAN_LOGIN_REGISTRY_PORT} <<<"${PODMAN_LOGIN_PASS}"
    is "$output" "Login Succeeded!" "output from podman login"

  manifest1="localhost:${PODMAN_LOGIN_REGISTRY_PORT}/test:1.0"
  run_podman build -t image1 --platform linux/amd64 -f $dockerfile
  run_podman build -t image2 --platform linux/arm64 -f $dockerfile

  run_podman manifest create foo
  run_podman images -a
  run_podman manifest add foo containers-storage:localhost/image1:latest
  run_podman manifest add foo containers-storage:localhost/image2:latest

  run_podman manifest push --authfile=$authfile --all --add-compression zstd --tls-verify=false foo $manifest1

  run skopeo inspect --authfile=$authfile --tls-verify=false --raw docker://$manifest1
  echo $output
  list="$output"

  validate_instance_compression "0" "$list" "amd64" "gzip"
  validate_instance_compression "1" "$list" "arm64" "gzip"
  validate_instance_compression "2" "$list" "amd64" "zstd"
  validate_instance_compression "3" "$list" "arm64" "zstd"
}

# vim: filetype=sh
