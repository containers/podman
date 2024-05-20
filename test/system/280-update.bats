#!/usr/bin/env bats   -*- bats -*-
#
# Tests for podman update
#

load helpers

LOOPDEVICE=

function teardown() {
    if [[ -n "$LOOPDEVICE" ]]; then
        losetup -d $LOOPDEVICE
        LOOPDEVICE=
    fi
    basic_teardown
}


# bats test_tags=distro-integration
@test "podman update - test all options" {
    local cgv=1
    if is_cgroupsv2; then
        cgv=2;
    fi

    # Need a block device for blkio-weight-device testing
    local pass_loop_device=
    if ! is_rootless; then
        if is_cgroupsv2; then
            lofile=${PODMAN_TMPDIR}/disk.img
            fallocate -l 1k  ${lofile}
            LOOPDEVICE=$(losetup --show -f $lofile)
            pass_loop_device="--device $LOOPDEVICE"

            # Get maj:min (tr needed because losetup seems to use %2d)
            lomajmin=$(losetup -l --noheadings --output MAJ:MIN $LOOPDEVICE | tr -d ' ')
        fi
    fi

    # Shortcuts to make the table narrower
    local -a gig=(0 1073741824 2147483648 3221225472)
    local devicemax="1:5 rbps=10485760 wbps=31457280 riops=2000 wiops=4000"
    local mm=memory/memory

    # Format:
    #   --<option> = <value>  | rootless? | check: cgroups v1            | check: cgroups v2
    #
    # Requires very wide window to read. Sorry.
    #
    # FIXMEs:
    #  cpu-rt-period  (cgv1 only, on cpu/cpu.rt_period_us) works on RHEL8 but not on Ubuntu
    #  cpu-rt-runtime (cgv1 only, on cpu/cpu.rt_runtime_us) fails: error setting cgroup config for procHooks ...
    tests="
cpu-shares          =            512 | - | cpu/cpu.shares       = 512              | cpu.weight      = 20
cpus                =              5 | - | cpu/cpu.cfs_quota_us = 500000           | cpu.max         = 500000 100000
cpuset-cpus         =              0 | - | cpuset/cpuset.cpus   = 0                | cpuset.cpus     = 0
cpuset-mems         =              0 | - | cpuset/cpuset.mems   = 0                | cpuset.mems     = 0

memory              =             1G | 2 | $mm.limit_in_bytes       = ${gig[1]}    | memory.max      = ${gig[1]}
memory-swap         =             3G | 2 | $mm.memsw.limit_in_bytes = ${gig[3]}    | memory.swap.max = ${gig[2]}
memory-reservation  =           400M | 2 | $mm.soft_limit_in_bytes  = 419430400    | memory.low      = 419430400

blkio-weight        =            321 | - | -                                       | io.bfq.weight   = default 321 $lomajmin 98
blkio-weight-device = $LOOPDEVICE:98 | - | -                                       | io.bfq.weight   = default 321 $lomajmin 98

device-read-bps     = /dev/zero:10mb | - | -                                       | io.max          = $devicemax
device-read-iops    = /dev/zero:2000 | - | -                                       | io.max          = $devicemax
device-write-bps    = /dev/zero:30mb | - | -                                       | io.max          = $devicemax
device-write-iops   = /dev/zero:4000 | - | -                                       | io.max          = $devicemax
"

    # Run a container
    run_podman run ${pass_loop_device} -d $IMAGE sleep infinity
    cid="$output"

    # Pass 1: read the table above, gather up the options applicable
    # to this test environment (root/rootless, cgroups v1/v2)
    local -a opts
    local -A check
    while read opt works_rootless cgv1 cgv2; do
        if is_rootless; then
            local skipping="skipping --$opt : does not work rootless"
            if [[ $works_rootless = '-' ]]; then
                echo "[ $skipping ]"
                continue
            fi
            if [[ ! $works_rootless =~ $cgv ]]; then
                echo "[ $skipping on cgroups v$cgv ]"
                continue
            fi
        fi

        # Determine the "path = newvalue" string for this cgroup
        tuple=$cgv1
        if is_cgroupsv2; then
            tuple=$cgv2
        fi
        if [[ $tuple = '-' ]]; then
            echo "[ skipping --$opt : N/A on cgroups v$cgv ]"
            continue
        fi

        # Sigh. bfq doesn't exist on Debian (2024-03)
        read path op expect <<<"$tuple"
        if [[ ! -e /sys/fs/cgroup/$path ]]; then
            echo "[ skipping --$opt : /sys/fs/cgroup/$path does not exist ]"
            continue
        fi

        # OK: setting is applicable. Preserve it. (First removing whitespace)
        opt=${opt// /}
        opts+=("--$opt")
        check["--$opt"]=$tuple
    done < <(parse_table "$tests")

    # Now do the update in one fell swoop
    run_podman update "${opts[@]}" $cid

    # ...and check one by one
    defer-assertion-failures
    for opt in "${opts[@]}"; do
        read path op expect <<<"${check[$opt]}"
        run_podman exec $cid cat /sys/fs/cgroup/$path

        # Magic echo of unquoted-output converts newlines to spaces;
        # important for otherwise multiline blkio file.
        updated="$(echo $output)"
        assert "$updated" $op "$expect" "$opt ($path)"
    done
    immediate-assertion-failures

    # Clean up
    run_podman rm -f -t0 $cid
    if [[ -n "$LOOPDEVICE" ]]; then
        losetup -d $LOOPDEVICE
        LOOPDEVICE=
    fi
}

@test "podman update - set restart policy" {
    touch ${PODMAN_TMPDIR}/sentinel
    run_podman run --security-opt label=disable --name testctr -v ${PODMAN_TMPDIR}:/testdir -d $IMAGE sh -c "touch /testdir/alive; while test -e /testdir/sentinel; do sleep 0.1; done;"

    run_podman container inspect testctr --format "{{ .HostConfig.RestartPolicy.Name }}"
    is "$output" "no"

    run_podman update --restart always testctr

    run_podman container inspect testctr --format "{{ .HostConfig.RestartPolicy.Name }}"
    is "$output" "always"

    # Ensure the container is alive
    wait_for_file ${PODMAN_TMPDIR}/alive

    rm -f ${PODMAN_TMPDIR}/alive
    rm -f ${PODMAN_TMPDIR}/sentinel

    # Restart should ensure that the container comes back up and recreates the file
    wait_for_file ${PODMAN_TMPDIR}/alive

    run_podman rm -f -t0 testctr
}

# vim: filetype=sh
