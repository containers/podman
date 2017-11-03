#!/usr/bin/env bats

load helpers

ALPINE="docker.io/library/alpine:latest"

@test "run a container based on local image" {
    run ${KPOD_BINARY} ${KPOD_OPTIONS} pull docker.io/library/busybox:latest
    echo "$output"
    [ "$status" -eq 0 ]
    run bash -c ${KPOD_BINARY} ${KPOD_OPTIONS} run docker.io/library/busybox:latest ls
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "run a container based on a remote image" {
    run bash -c ${KPOD_BINARY} ${KPOD_OPTIONS} run ${ALPINE} ls
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "run selinux test" {
    if [ ! -e /usr/sbin/selinuxenabled ] || [ ! /usr/sbin/selinuxenabled ]; then
        skip "SELinux not enabled"
    fi

    run ${KPOD_BINARY} ${KPOD_OPTIONS} run ${ALPINE} cat /proc/self/attr/current
    echo "$output"
    firstLabel=$output

    run ${KPOD_BINARY} ${KPOD_OPTIONS} run ${ALPINE} cat /proc/self/attr/current
    echo "$output"
    [ "$output" != "${firstLabel}" ]
}


@test "run selinux grep test" {
    skip "Until console issues worked out"
    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} run -t -i --security-opt label=level:s0:c1,c2 ${ALPINE} cat /proc/self/attr/current | grep s0:c1,c2"
    echo "$output"
    [ "$status" -eq 0 ]

}

@test "run capabilities test" {
    run  bash -c ${KPOD_BINARY} ${KPOD_OPTIONS} run --cap-add all ${ALPINE} cat /proc/self/status
    echo "$output"
    [ "$status" -eq 0 ]

    run bash -c ${KPOD_BINARY} ${KPOD_OPTIONS} run --cap-add sys_admin ${ALPINE} cat /proc/self/status
    echo "$output"
    [ "$status" -eq 0 ]

    run bash -c ${KPOD_BINARY} ${KPOD_OPTIONS} run --cap-drop all ${ALPINE} cat /proc/self/status
    echo "$output"
    [ "$status" -eq 0 ]

    run bash -c ${KPOD_BINARY} ${KPOD_OPTIONS} run --cap-drop setuid ${ALPINE} cat /proc/self/status
    echo "$output"
    [ "$status" -eq 0 ]

}
