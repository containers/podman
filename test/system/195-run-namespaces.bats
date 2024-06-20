#!/usr/bin/env bats   -*- bats -*-
#
# Tests for the namespace options
#
# bats file_tags=distro-integration
#

load helpers

@test "podman test all namespaces" {
    # format is nsname | option name
    tests="
cgroup | cgroupns
ipc    | ipc
net    | network
pid    | pid
uts    | uts
"

    for nstype in private host; do
        while read name option; do
            local cname="c_${name}_$(random_string)"
            # ipc is special, private does not allow joining from another container.
            # Instead we must use "shareable".
            local type=$nstype
            if [ "$name" = "ipc" ] && [ "$type" = "private" ]; then
                type="shareable"
            fi

            run_podman run --name $cname --$option $type -d $IMAGE sh -c \
                "readlink /proc/self/ns/$name; sleep inf"

            run_podman run --rm --$option container:$cname $IMAGE readlink /proc/self/ns/$name
            con2_ns="$output"

            run readlink /proc/self/ns/$name
            host_ns="$output"

            run_podman logs $cname
            con1_ns="$output"

            assert "$con1_ns" == "$con2_ns" "($name) namespace matches (type: $type)"
            local matcher="=="
            if [[ "$type" != "host" ]]; then
                matcher="!="
            fi
            assert "$con1_ns" $matcher "$host_ns" "expected host namespace to ($matcher) (type: $type)"

            run_podman rm -f -t0 $cname
        done < <(parse_table "$tests")
    done
}

@test "podman --ipc=host" {
    local cname="c-host-$(random_string)"
    hostipc="$(readlink /proc/self/ns/ipc)"
    run_podman run --name $cname --ipc=host $IMAGE readlink /proc/self/ns/ipc
    is "$output" "$hostipc" "HostIPC and container IPC should be same"
    run_podman inspect $cname --format '{{ .HostConfig.IpcMode }}'
    is "$output" "host" "host mode should be selected"
    run_podman rm $cname
}

@test "podman --ipc=none" {
    local cname="c-none-$(random_string)"
    hostipc="$(readlink /proc/self/ns/ipc)"
    run_podman run --ipc=none --name $cname $IMAGE readlink /proc/self/ns/ipc
    assert "$output" != "$hostipc" "containeripc should != hostipc"
    run_podman inspect $cname --format '{{ .HostConfig.IpcMode }}'
    is "$output" "none" "none mode should be selected"
    run_podman rm $cname

    run_podman 1 run --rm --ipc=none $IMAGE ls /dev/shm
    is "$output" "ls: /dev/shm: No such file or directory" "Should fail with missing /dev/shm"
}

@test "podman --ipc=private" {
    local cname="c-private-$(random_string)"
    hostipc="$(readlink /proc/self/ns/ipc)"
    run_podman run -d --ipc=private --name $cname $IMAGE sleep 100
    assert "$output" != "$hostipc" "containeripc should != hostipc"
    run_podman inspect $cname --format '{{ .HostConfig.IpcMode }}'
    is "$output" "private" "private mode should be selected"

    run_podman 125 run --ipc=container:$cname --rm $IMAGE readlink /proc/self/ns/ipc
    is "$output" ".*is not allowed: non-shareable IPC (hint: use IpcMode:shareable for the donor container)" "Containers should not share private ipc namespace"
    run_podman rm -f -t 0 $cname
}

@test "podman --ipc=shareable" {
    local cname="c-shareable-$(random_string)"
    hostipc="$(readlink /proc/self/ns/ipc)"
    run_podman run -d --ipc=shareable --name $cname $IMAGE sleep 100
    assert "$output" != "$hostipc" "containeripc(shareable) should != hostipc"
    run_podman inspect $cname --format '{{ .HostConfig.IpcMode }}'
    is "$output" "shareable" "shareable mode should be selected"

    run_podman run --ipc=container:$cname --rm $IMAGE readlink /proc/self/ns/ipc
    assert "$output" != "$hostipc" "containeripc(:$cname) should != hostipc"

    run_podman rm -f -t 0 $cname
}

@test "podman --ipc=container@test" {
    local cname="c-container-$(random_string)"
    hostipc="$(readlink /proc/self/ns/ipc)"
    run_podman run -d --name $cname $IMAGE sleep 100
    containerid=$output
    run_podman inspect $cname --format '{{ .HostConfig.IpcMode }}'
    is "$output" "shareable" "shareable mode should be selected"
    run_podman exec $cname readlink /proc/self/ns/ipc
    assert "$output" != "$hostipc" "containeripc(exec) should != hostipc"
    testipc=$output

    local cname2="c-contest-$(random_string)"
    run_podman run --name $cname2 --ipc=container:$cname $IMAGE readlink /proc/self/ns/ipc
    assert "$output" = "$testipc" "Containers should share ipc namespace"
    run_podman inspect $cname2 --format '{{ .HostConfig.IpcMode }}'
    is "$output" "container:$containerid" "ipc mode should be selected"
    run_podman rm $cname2

    run_podman rm -f -t 0 $cname
}

# vim: filetype=sh
