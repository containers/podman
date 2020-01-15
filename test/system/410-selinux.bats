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
    local context="$output"
    if [ ${#lines[@]} -gt 1 ]; then
        if expr "${lines[0]}" : "WARNING: .* type, major" >/dev/null; then
            echo "# ${lines[0]} [ignored]" >&3
            context="${lines[1]}"
        else
            die "FAILED: too much output, expected one single line"
        fi
    fi

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

# vim: filetype=sh
