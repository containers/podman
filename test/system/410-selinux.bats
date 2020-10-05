#!/usr/bin/env bats   -*- bats -*-
#
# 410-selinux - podman selinux tests
#

load helpers


function check_label() {
    skip_if_no_selinux

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

@test "podman selinux: pid=host" {
    # FIXME FIXME FIXME: Remove these lines once all VMs have >= 2.146.0
    # (this is ugly, but better than an unconditional skip)
    skip_if_no_selinux
    if is_rootless; then
        if [ -x /usr/bin/rpm ]; then
            cs_version=$(rpm -q --qf '%{version}' container-selinux)
        else
            # SELinux not enabled on Ubuntu, so we should never get here
            die "WHOA! SELinux enabled, but no /usr/bin/rpm!"
        fi
        if [[ "$cs_version" < "2.146" ]]; then
            skip "FIXME: #7939: requires container-selinux-2.146.0 (currently installed: $cs_version)"
        fi
    fi
    # FIXME FIXME FIXME: delete up to here, leaving just check_label

    check_label "--pid=host" "spc_t"
}

@test "podman selinux: container with overridden range" {
    check_label "--security-opt label=level:s0:c1,c2" "container_t" "s0:c1,c2"
}

# pr #6752
@test "podman selinux: inspect multiple labels" {
    skip_if_no_selinux

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

# Sharing context between two containers not in a pod
# These tests were piggybacked in with #7902, but are not actually related
@test "podman selinux: shared context in (some) namespaces" {
    skip_if_no_selinux

    run_podman run -d --name myctr $IMAGE top
    run_podman exec myctr cat -v /proc/self/attr/current
    context_c1="$output"

    # --ipc container
    run_podman run --name myctr2 --ipc container:myctr $IMAGE cat -v /proc/self/attr/current
    is "$output" "$context_c1" "new container, run with ipc of existing one "

    # --pid container
    run_podman run --rm --pid container:myctr $IMAGE cat -v /proc/self/attr/current
    is "$output" "$context_c1" "new container, run with --pid of existing one "

    # net NS: do not share context
    run_podman run --rm --net container:myctr $IMAGE cat -v /proc/self/attr/current
    if [[ "$output" = "$context_c1" ]]; then
        die "run --net : context ($output) is same as running container (it should not be)"
    fi

    # The 'myctr2' above was not run with --rm, so it still exists, and
    # we can't remove the original container until this one is gone.
    run_podman stop -t 0 myctr
    run_podman 125 rm myctr
    is "$output" "Error: container .* has dependent containers"

    # We have to do this in two steps: even if ordered as 'myctr2 myctr',
    # podman will try the removes in random order, which fails if it
    # tries myctr first.
    run_podman rm myctr2
    run_podman rm myctr
}

# pr #7902 - containers in pods should all run under same context
@test "podman selinux: containers in pods share full context" {
    skip_if_no_selinux

    # We don't need a fullblown pause container; avoid pulling the k8s one
    run_podman pod create --name myselinuxpod \
               --infra-image $IMAGE \
               --infra-command /home/podman/pause

    # Get baseline
    run_podman run --rm --pod myselinuxpod $IMAGE cat -v /proc/self/attr/current
    context_c1="$output"

    # Prior to #7902, the labels (':c123,c456') would be different
    run_podman run --rm --pod myselinuxpod $IMAGE cat -v /proc/self/attr/current
    is "$output" "$context_c1" "SELinux context of 2nd container matches 1st"

    # What the heck. Try a third time just for extra confidence
    run_podman run --rm --pod myselinuxpod $IMAGE cat -v /proc/self/attr/current
    is "$output" "$context_c1" "SELinux context of 3rd container matches 1st"

    run_podman pod rm myselinuxpod
}

# more pr #7902
@test "podman selinux: containers in --no-infra pods do not share context" {
    skip_if_no_selinux

    # We don't need a fullblown pause container; avoid pulling the k8s one
    run_podman pod create --name myselinuxpod --infra=false

    # Get baseline
    run_podman run --rm --pod myselinuxpod $IMAGE cat -v /proc/self/attr/current
    context_c1="$output"

    # Even after #7902, labels (':c123,c456') should be different
    run_podman run --rm --pod myselinuxpod $IMAGE cat -v /proc/self/attr/current
    if [[ "$output" = "$context_c1" ]]; then
        die "context ($output) is the same on two separate containers, it should have been different"
    fi

    run_podman pod rm myselinuxpod
}

# vim: filetype=sh
