#!/usr/bin/env bats

load helpers

function teardown() {
    cleanup_test
}

function setup() {
    copy_images
}

@test "run a container based on local image" {
    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} run $BB ls"
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "run a container based on a remote image" {
    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} run ${BB_GLIBC} ls"
    echo "$output"
    [ "$status" -eq 0 ]
}

@test "run selinux test" {
    if [ ! -e /usr/sbin/selinuxenabled ] || [ ! /usr/sbin/selinuxenabled ]; then
        skip "SELinux not enabled"
    fi

    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} run ${ALPINE} cat /proc/self/attr/current"
    echo "$output"
    firstLabel=$output

    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} run ${ALPINE} cat /proc/self/attr/current"
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
    run  bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} run --cap-add all ${ALPINE} cat /proc/self/status"
    echo "$output"
    [ "$status" -eq 0 ]

    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} run --cap-add sys_admin ${ALPINE} cat /proc/self/status"
    echo "$output"
    [ "$status" -eq 0 ]

    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} run --cap-drop all ${ALPINE} cat /proc/self/status"
    echo "$output"
    [ "$status" -eq 0 ]

    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} run --cap-drop setuid ${ALPINE} cat /proc/self/status"
    echo "$output"
    [ "$status" -eq 0 ]

}

@test "run environment test" {

    ${KPOD_BINARY} ${KPOD_OPTIONS} pull ${ALPINE}

    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} run -env FOO=BAR ${ALPINE} printenv FOO | tr -d '\r'"
    echo "$output"
    [ "$status" -eq 0 ]
    [ $output = "BAR" ]

    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} run -env PATH="/bin" ${ALPINE} printenv PATH | tr -d '\r'"
    echo "$output"
    [ "$status" -eq 0 ]
    [ $output = "/bin" ]

    run bash -c "export FOO=BAR; ${KPOD_BINARY} ${KPOD_OPTIONS} run -env FOO ${ALPINE} printenv FOO | tr -d '\r'"
    echo "$output"
    [ "$status" -eq 0 ]
    [ "$output" = "BAR" ]

    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} run -env FOO ${ALPINE} printenv"
    echo "$output"
    [ "$status" -ne 0 ]

#    We don't currently set the hostname in containers, since we are not setting up
#    networking.  As soon as kpod run gets network support we need to uncomment this
#    test.
#    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} run ${ALPINE} sh -c printenv | grep HOSTNAME"
#    echo "$output"
#    [ "$status" -eq 0 ]
}

IMAGE="docker.io/library/fedora:latest"

@test "run limits test" {

    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} pull ${IMAGE}"

    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} run --ulimit rtprio=99 --cap-add=sys_nice ${IMAGE}  cat /proc/self/sched"
    echo $output
    [ "$status" -eq 0 ]

    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} run --ulimit nofile=2048:2048 ${IMAGE} ulimit -n | tr -d '\r'"
    echo $output
    [ "$status" -eq 0 ]
    [ "$output" = 2048 ]

    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} run --oom-kill-disable=true ${IMAGE} echo memory-hog"
    echo $output
    [ "$status" -eq 0 ]

    run bash -c "${KPOD_BINARY} ${KPOD_OPTIONS} run --oom-score-adj=100 ${IMAGE} cat /proc/self/oom_score_adj | tr -d '\r'"
    echo $output
    [ "$status" -eq 0 ]
    [ "$output" = 100 ]

}
