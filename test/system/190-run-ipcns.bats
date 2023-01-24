#!/usr/bin/env bats   -*- bats -*-
# shellcheck disable=SC2096
#
# Tests for podman build
#

load helpers

@test "podman --ipc=host" {
    cname=c-$(random_string 10)
    hostipc="$(readlink /proc/self/ns/ipc)"
    run_podman run --name=$cname --ipc=host $IMAGE readlink /proc/self/ns/ipc
    is "$output" "$hostipc" "HostIPC and container IPC should be same"

    run_podman container inspect --format "{{.HostConfig.IpcMode}}" $cname
    assert "host" "inspect should show host ipc"

    run_podman rm -f -t0 $cname
}

@test "podman --ipc=none" {
    cname=c-$(random_string 10)
    hostipc="$(readlink /proc/self/ns/ipc)"
    run_podman run --name=$cname --ipc=none $IMAGE readlink /proc/self/ns/ipc
    assert "$output" != "$hostipc" "containeripc should != hostipc"

    run_podman container inspect --format "{{.HostConfig.IpcMode}}" $cname
    assert "none" "inspect should show none ipc"

    run_podman 1 run --rm --ipc=none $IMAGE ls /dev/shm
    is "$output" "ls: /dev/shm: No such file or directory" "Should fail with missing /dev/shm"

    run_podman rm -f -t0 $cname
}

@test "podman --ipc=private" {
    cname=c-$(random_string 10)
    hostipc="$(readlink /proc/self/ns/ipc)"
    run_podman run -d --ipc=private --name=$cname $IMAGE sleep 100
    assert "$output" != "$hostipc" "containeripc should != hostipc"

    run_podman container inspect --format "{{.HostConfig.IpcMode}}" $cname
    assert "private" "inspect should show private ipc"

    run_podman 125 run --ipc=container:$cname --rm $IMAGE readlink /proc/self/ns/ipc
    is "$output" ".*is not allowed: non-shareable IPC (hint: use IpcMode:shareable for the donor container)" "Containers should not share private ipc namespace"

    run_podman rm -f -t0 $cname
}

@test "podman --ipc=shareable" {
    cname=c-$(random_string 10)
    hostipc="$(readlink /proc/self/ns/ipc)"
    run_podman run -d --ipc=shareable --name=$cname $IMAGE sleep 100
    assert "$output" != "$hostipc" "containeripc(shareable) should != hostipc"

    run_podman container inspect --format "{{.HostConfig.IpcMode}}" $cname
    assert "shareable" "inspect should show shareable ipc"

    run_podman run --ipc=container:$cname --rm $IMAGE readlink /proc/self/ns/ipc
    assert "$output" != "$hostipc" "containeripc(:XXX) should != hostipc"

    run_podman rm -f -t0 $cname
}

@test "podman --ipc=container@test" {
    cname1=c1-$(random_string 10)
    cname2=c2-$(random_string 10)
    hostipc="$(readlink /proc/self/ns/ipc)"
    run_podman run -d --name=$cname1 $IMAGE sleep 100
    run_podman exec $cname1 readlink /proc/self/ns/ipc
    assert "$output" != "$hostipc" "containeripc(exec) should != hostipc"
    testipc=$output

    run_podman container inspect --format "{{.HostConfig.IpcMode}}" $cname1
    assert "shareable" "inspect should show shareable ipc"

    run_podman run --ipc=container:$cname1 --name=$cname2 $IMAGE readlink /proc/self/ns/ipc
    assert "$output" = "$testipc" "Containers should share ipc namespace"

    run_podman container inspect --format "{{.HostConfig.IpcMode}}" $cname2
    assert "$output" =~ "container:[0-9a-f]{64}" "inspect should show container:<FULL_ID> ipc"

    # We cannot remove both at once due "container XXXX has dependent containers
    # which must be removed before it" error.
    run_podman rm -f -t0 $cname2
    run_podman rm -f -t0 $cname1
}

# vim: filetype=sh
