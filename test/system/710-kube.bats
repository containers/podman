#!/usr/bin/env bats   -*- bats -*-
#
# Test podman kube generate
#

load helpers

@test "podman kube generate - basic" {
    run_podman kube generate --help
    is "$output" ".*podman.* kube generate \[options\] {CONTAINER...|POD...|VOLUME...}"
    run_podman generate kube --help
    is "$output" ".*podman.* generate kube \[options\] {CONTAINER...|POD...|VOLUME...}"
}

# vim: filetype=sh
