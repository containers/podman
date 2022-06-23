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
	is "$range" "$2^@" "SELinux range"
    fi
}


@test "podman selinux: confined container" {
    check_label "" "container_t"
}

@test "podman selinux: container with label=disable" {
    check_label "--security-opt label=disable" "spc_t"
}

@test "podman selinux: privileged container" {
    check_label "--privileged --userns=host" "spc_t"
}

@test "podman selinux: init container" {
    check_label "--systemd=always" "container_init_t"
}

@test "podman selinux: init container with --security-opt type" {
    check_label "--systemd=always --security-opt=label=type:spc_t" "spc_t"
}

@test "podman selinux: init container with --security-opt level&type" {
    check_label "--systemd=always --security-opt=label=level:s0:c1,c2 --security-opt=label=type:spc_t" "spc_t" "s0:c1,c2"
}

@test "podman selinux: init container with --security-opt level" {
    check_label "--systemd=always --security-opt=label=level:s0:c1,c2" "container_init_t"  "s0:c1,c2"
}

@test "podman selinux: pid=host" {
    # FIXME this test fails when run rootless with runc:
    #   Error: container_linux.go:367: starting container process caused: process_linux.go:495: container init caused: readonly path /proc/asound: operation not permitted: OCI permission denied
    if is_rootless; then
	runtime=$(podman_runtime)
	test "$runtime" == "crun" \
	    || skip "runtime is $runtime; this test requires crun"
    fi

    check_label "--pid=host" "spc_t"
}

@test "podman selinux: container with overridden range" {
    check_label "--security-opt label=level:s0:c1,c2" "container_t" "s0:c1,c2"
}

@test "podman selinux: inspect kvm labels" {
    skip_if_no_selinux
    skip_if_remote "runtime flag is not passed over remote"

    tmpdir=$PODMAN_TMPDIR/kata-test
    mkdir -p $tmpdir
    KATA=${tmpdir}/kata-runtime
    ln -s /bin/true ${KATA}
    run_podman create --runtime=${KATA} --name myc $IMAGE
    run_podman inspect --format='{{ .ProcessLabel }}' myc
    is "$output" ".*container_kvm_t"
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
    is "$output" "[label=type:spc_t,label=level:s0 seccomp=unconfined]" \
      "'podman inspect' preserves all --security-opts"

    run_podman exec myc touch /stop
    run_podman rm -t 0 -f myc
}

# Sharing context between two containers not in a pod
# These tests were piggybacked in with #7902, but are not actually related
@test "podman selinux: shared context in (some) namespaces" {
    skip_if_no_selinux

    # rootless users have no usable cgroups with cgroupsv1, so containers
    # must use a pid namespace and not join an existing one.
    skip_if_rootless_cgroupsv1

    if [[ $(podman_runtime) == "runc" ]]; then
	skip "some sort of runc bug, not worth fixing (#11784)"
    fi

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
    assert "$output" != "$context_c1" \
	   "run --net : context should != context of running container"

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
    assert "$output" != "$context_c1" \
	   "context of two separate containers should be different"

    run_podman pod rm myselinuxpod
}

# #8946 - better diagnostics for nonexistent attributes
@test "podman with nonexistent labels" {
    skip_if_no_selinux

    # runc and crun emit different diagnostics
    runtime=$(podman_runtime)
    case "$runtime" in
	# crun 0.20.1 changes the error message
	#   from /proc/thread-self/attr/exec`: .* unable to assign
	#   to   /proc/self/attr/keycreate`: .* unable to process
	crun) expect="\`/proc/.*\`: OCI runtime error: unable to \(assign\|process\) security attribute" ;;
	# runc 1.1 changed the error message because of new selinux pkg that uses standard os.PathError, see
	# https://github.com/opencontainers/selinux/pull/148/commits/a5dc47f74c56922d58ead05d1fdcc5f7f52d5f4e
	#   from failed to set /proc/self/attr/keycreate on procfs
	#   to   write /proc/self/attr/keycreate: invalid argument
	runc) expect="OCI runtime error: .*: \(failed to set|write\) /proc/self/attr/keycreate" ;;
	*)    skip "Unknown runtime '$runtime'";;
    esac

    # The '.*' in the error below is for dealing with podman-remote, which
    # includes "error preparing container <sha> for attach" in output.
    run_podman 126 run --security-opt label=type:foo.bar $IMAGE true
    is "$output" "Error.*: $expect" "podman emits useful diagnostic on failure"
}

@test "podman selinux: check relabel" {
    skip_if_no_selinux

    LABEL="system_u:object_r:tmp_t:s0"
    RELABEL="system_u:object_r:container_file_t:s0"
    tmpdir=$PODMAN_TMPDIR/vol
    mkdir -p $tmpdir
    chcon -vR ${LABEL} $tmpdir
    ls -Z $tmpdir

    run_podman run -v $tmpdir:/test $IMAGE cat /proc/self/attr/current
    run ls -dZ ${tmpdir}
    is "$output" "${LABEL} ${tmpdir}" "No Relabel Correctly"

    run_podman run -v $tmpdir:/test:z --security-opt label=disable $IMAGE cat /proc/self/attr/current
    run ls -dZ $tmpdir
    is "$output" "${RELABEL} $tmpdir" "Privileged Relabel Correctly"

    run_podman run -v $tmpdir:/test:z --privileged $IMAGE cat /proc/self/attr/current
    run ls -dZ $tmpdir
    is "$output" "${RELABEL} $tmpdir" "Privileged Relabel Correctly"

    run_podman run --name label -v $tmpdir:/test:Z $IMAGE cat /proc/self/attr/current
    level=$(secon -l $output)
    run ls -dZ $tmpdir
    is "$output" "system_u:object_r:container_file_t:$level $tmpdir" \
       "Confined Relabel Correctly"

    # podman-remote has no 'unshare'
    if is_rootless && ! is_remote; then
       run_podman unshare touch $tmpdir/test1
       # Relabel entire directory
       run_podman unshare chcon system_u:object_r:usr_t:s0 $tmpdir
       run_podman start --attach label
       newlevel=$(secon -l $output)
       is "$level" "$newlevel" "start should relabel with same SELinux labels"
       run ls -dZ $tmpdir
       is "$output" "system_u:object_r:container_file_t:$level $tmpdir" \
	  "Confined Relabel Correctly"
	run ls -dZ $tmpdir/test1
	is "$output" "system_u:object_r:container_file_t:$level $tmpdir/test1" \
	   "Start did not Relabel"

	# Relabel only file in subdir
	run_podman unshare chcon system_u:object_r:usr_t:s0 $tmpdir/test1
	run_podman start --attach label
	newlevel=$(secon -l $output)
	is "$level" "$newlevel" "start should use same SELinux labels"

	run ls -dZ $tmpdir/test1
	is "$output" "system_u:object_r:usr_t:s0 $tmpdir/test1" \
	   "Start did not Relabel"
    fi
    run_podman run -v $tmpdir:/test:z $IMAGE cat /proc/self/attr/current
    run ls -dZ $tmpdir
    is "$output" "${RELABEL} $tmpdir" "Shared Relabel Correctly"
}

# vim: filetype=sh
