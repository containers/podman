#!/usr/bin/env bats   -*- bats -*-
#
# 410-selinux - podman selinux tests
#

load helpers


function check_label() {
    if [ ! -e /usr/sbin/selinuxenabled ] || ! /usr/sbin/selinuxenabled; then
        skip "selinux disabled or not available"
    fi

    local args="$1"; shift        # command-line args for run

    # FIXME: it'd be nice to specify the command to run, e.g. 'ls -dZ /',
    # but alpine ls (from busybox) doesn't support -Z
    run_podman run --rm $args $IMAGE cat -v /proc/self/attr/current

    # FIXME: on some CI systems, 'run --privileged' emits a spurious
    # warning line about dup devices. Ignore it.
    remove_same_dev_warning
    local context="$output"

    is "$context" ".*_u:system_r:.*" "SELinux role should always be system_r"

    # e.g. system_u:system_r:container_t:s0:c45,c745 -> "container_t"
    type=$(cut -d: -f3 <<<"$context")
    is "$type" "$1" "SELinux type"

    if [ -n "$2" ]; then
        # e.g. from the above example -> "s0:c45,c745"
        range=$(cut -d: -f4,5 <<<"$context")
        is "$range" "$2" "SELinux range"
    fi
}


@test "podman selinux: confined container" {
    check_label "" "container_t"
}

@test "podman selinux: container with label=disable" {
    skip_if_rootless

    check_label "--security-opt label=disable" "spc_t"
}

@test "podman selinux: privileged container" {
    skip_if_rootless

    check_label "--privileged --userns=host" "spc_t"
}

@test "podman selinux: container with overridden range" {
    check_label "--security-opt label=level:s0:c1,c2" "container_t" "s0:c1,c2"
}

# pr #6752
@test "podman selinux: inspect multiple labels" {
    if [ ! -e /usr/sbin/selinuxenabled ] || ! /usr/sbin/selinuxenabled; then
        skip "selinux disabled or not available"
    fi

    run_podman run -d --name myc \
               --security-opt seccomp=unconfined \
               --security-opt label=type:spc_t \
               --security-opt label=level:s0 \
               $IMAGE sh -c 'while test ! -e /stop; do sleep 0.1; done'
    run_podman inspect --format='{{ .HostConfig.SecurityOpt }}' myc
    is "$output" "\[label=type:spc_t,label=level:s0 seccomp=unconfined]" \
      "'podman inspect' preserves all --security-opts"

    run_podman exec myc touch /stop
    run_podman rm -f myc
}

# vim: filetype=sh
