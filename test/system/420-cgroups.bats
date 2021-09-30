#!/usr/bin/env bats   -*- bats -*-
#
# cgroups-related tests
#

load helpers

@test "podman run, preserves initial --cgroup-manager" {
    skip_if_remote "podman-remote does not support --cgroup-manager"

    skip_if_rootless_cgroupsv1

    # Find out our default cgroup manager, and from that, get the non-default
    run_podman info --format '{{.Host.CgroupManager}}'
    case "$output" in
        systemd)  other="cgroupfs" ;;
        cgroupfs) other="systemd"  ;;
        *)        die "Unknown CgroupManager '$output'" ;;
    esac

    run_podman --cgroup-manager=$other run --name myc $IMAGE true
    run_podman container inspect --format '{{.HostConfig.CgroupManager}}' myc
    is "$output" "$other" "podman preserved .HostConfig.CgroupManager"

    if is_rootless && test $other = cgroupfs ; then
        run_podman container inspect --format '{{.HostConfig.CgroupParent}}' myc
        is "$output" "" "podman didn't set .HostConfig.CgroupParent for cgroupfs and rootless"
    fi

    # Restart the container, without --cgroup-manager option (ie use default)
    # Prior to #7970, this would fail with an OCI runtime error
    run_podman start myc

    run_podman rm myc
}

# vim: filetype=sh
