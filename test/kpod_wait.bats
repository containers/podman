#!/usr/bin/env bats

load helpers

IMAGE="redis:alpine"

# Returns the POD ID
function pod_run_from_template(){
    #1=name, 2=uid, 3=namespace) {
    NAME=$1 CUID=$2 NAMESPACE=$3 envsubst < ${TESTDATA}/template_sandbox_config.json > ${TESTDIR}/pod-${1}.json
    crioctl pod run --config ${TESTDIR}/pod-${1}.json
}

# Returns the container ID
function container_create_from_template() {
    #1=name, 2=image, 3=command, 4=id) {
    NAME=$1 IMAGE=$2 COMMAND=$3 envsubst < ${TESTDATA}/template_container_config.json > ${TESTDIR}/ctr-${1}.json
    crioctl ctr create --config ${TESTDIR}/ctr-${1}.json --pod "$4"
}

function container_start() {
    #1=id
    crioctl ctr start --id "$1"

}
@test "wait on a bogus container" {
    start_crio
    run ${KPOD_BINARY} ${KPOD_OPTIONS} wait 12343
    echo $output
    [ "$status" -eq 1 ]
    stop_crio
}

@test "wait on a stopped container" {
    run ${KPOD_BINARY} ${KPOD_OPTIONS} pull docker.io/library/busybox:latest
    echo $output
    [ "$status" -eq 0 ]
    start_crio
    pod_id=$( pod_run_from_template "test" "test" "test1-1" )
    echo $pod_id
    ctr_id=$(container_create_from_template "test-CTR" "docker.io/library/busybox:latest" '["ls"]' "${pod_id}")
    echo $ctr_id
    container_start $ctr_id
    run ${KPOD_BINARY} ${KPOD_OPTIONS} wait $ctr_id
    [ "$status" -eq 0 ]
    cleanup_ctrs
    cleanup_pods
    stop_crio
}

@test "wait on a sleeping container" {
    run ${KPOD_BINARY} ${KPOD_OPTIONS} pull docker.io/library/busybox:latest
    echo $output
    [ "$status" -eq 0 ]
    start_crio
    pod_id=$( pod_run_from_template "test" "test" "test1-1" )
    echo $pod_id
    ctr_id=$(container_create_from_template "test-CTR" "docker.io/library/busybox:latest" '["sleep", "5"]' "${pod_id}")
    echo $ctr_id
    run container_start $ctr_id
    echo $output
    [ "$status" -eq 0 ]
    run ${KPOD_BINARY} ${KPOD_OPTIONS} wait $ctr_id
    echo $output
    [ "$status" -eq 0 ]
    cleanup_ctrs
    cleanup_pods
    stop_crio
}
