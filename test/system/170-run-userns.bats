#!/usr/bin/env bats   -*- bats -*-
# shellcheck disable=SC2096
#
# Tests for podman build
#

load helpers

function _require_crun() {
    runtime=$(podman_runtime)
    if [[ $runtime != "crun" ]]; then
        skip "runtime is $runtime; keep-groups requires crun"
    fi
}

@test "podman --group-add keep-groups while in a userns" {
    skip_if_rootless "chroot is not allowed in rootless mode"
    skip_if_remote "--group-add keep-groups not supported in remote mode"
    _require_crun
    run chroot --groups 1234 / ${PODMAN} run --uidmap 0:200000:5000 --group-add keep-groups $IMAGE id
    is "$output" ".*65534(nobody)" "Check group leaked into user namespace"
}

@test "podman --group-add keep-groups while not in a userns" {
    skip_if_rootless "chroot is not allowed in rootless mode"
    skip_if_remote "--group-add keep-groups not supported in remote mode"
    _require_crun
    run chroot --groups 1234,5678 / ${PODMAN} run --group-add keep-groups $IMAGE id
    is "$output" ".*1234" "Check group leaked into container"
}

@test "podman --group-add without keep-groups while in a userns" {
    skip_if_rootless "chroot is not allowed in rootless mode"
    skip_if_remote "--group-add keep-groups not supported in remote mode"
    run chroot --groups 1234,5678 / ${PODMAN} run --uidmap 0:200000:5000 --group-add 457 $IMAGE id
    is "$output" ".*457" "Check group leaked into container"
}

@test "podman --remote --group-add keep-groups " {
    if is_remote; then
        run_podman 125 run --group-add keep-groups $IMAGE id
        is "$output" ".*not supported in remote mode" "Remote check --group-add keep-groups"
    fi
}

@test "podman --group-add without keep-groups " {
    run_podman run --group-add 457 $IMAGE id
    is "$output" ".*457" "Check group leaked into container"
}

@test "podman --group-add keep-groups plus added groups " {
    run_podman 125 run --group-add keep-groups --group-add 457 $IMAGE id
    is "$output" ".*the '--group-add keep-groups' option is not allowed with any other --group-add options" "Check group leaked into container"
}
