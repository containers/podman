#!/usr/bin/env bats   -*- bats -*-
# shellcheck disable=SC2096
#
# Tests for podman build
#

load helpers

@test "podman --ipc=host" {
    run readlink /proc/self/ns/ipc
    hostipc=$output
    run_podman run --rm --ipc=host $IMAGE readlink /proc/self/ns/ipc
    is "$output" "$hostipc" "HostIPC and container IPC should be same"
}

@test "podman --ipc=none" {
    run readlink /proc/self/ns/ipc
    hostipc=$output
    run_podman run --rm --ipc=none $IMAGE readlink /proc/self/ns/ipc
    if [[ $output == "$hostipc" ]]; then
       die "hostipc and containeripc should be different"
    fi
    run_podman 1 run --rm --ipc=none $IMAGE ls /dev/shm
    is "$output" "ls: /dev/shm: No such file or directory" "Should fail with missing /dev/shm"
}

@test "podman --ipc=private" {
    run readlink /proc/self/ns/ipc
    hostipc=$output
    run_podman run -d --ipc=private --name test $IMAGE sleep 100
    if [[ $output == "$hostipc" ]]; then
       die "hostipc and containeripc should be different"
    fi
    run_podman 125 run --ipc=container:test --rm $IMAGE readlink /proc/self/ns/ipc
    is "$output" ".*is not allowed: non-shareable IPC (hint: use IpcMode:shareable for the donor container)" "Containers should not share private ipc namespace"
    run_podman stop -t 0 test
    run_podman rm test
}

@test "podman --ipc=shareable" {
    run readlink /proc/self/ns/ipc
    hostipc=$output
    run_podman run -d --ipc=shareable --name test $IMAGE sleep 100
    if [[ $output == "$hostipc" ]]; then
       die "hostipc and containeripc should be different"
    fi
    run_podman run --ipc=container:test --rm $IMAGE readlink /proc/self/ns/ipc
    if [[ $output == "$hostipc" ]]; then
       die "hostipc and containeripc should be different"
    fi
    run_podman stop -t 0 test
    run_podman rm test
}

@test "podman --ipc=container@test" {
    run readlink /proc/self/ns/ipc
    hostipc=$output
    run_podman run -d --name test $IMAGE sleep 100
    run_podman exec test readlink /proc/self/ns/ipc
    if [[ $output == "$hostipc" ]]; then
       die "hostipc and containeripc should be different"
    fi
    testipc=$output
    run_podman run --ipc=container:test --rm $IMAGE readlink /proc/self/ns/ipc
    is "$output" "$testipc" "Containers should share ipc namespace"
    run_podman stop -t 0 test
    run_podman rm test
}

# vim: filetype=sh
