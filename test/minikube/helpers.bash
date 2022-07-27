# -*- bash -*-

load ../system/helpers.bash

KUBECONFIG="$HOME/.kube/config"

function setup(){
    # only set up the minikube cluster before the first test
    if [[ "$BATS_TEST_NUMBER" -eq 1 ]]; then
        minikube start
        wait_for_default_sa
    fi
    basic_setup
}

function teardown(){
    # only delete the minikube cluster if we are done with the last test
    # the $DEBUG_MINIKUBE env can be set to preserve the cluster to debug if needed
    if [[ "$BATS_TEST_NUMBER" -eq ${#BATS_TEST_NAMES[@]} ]] && [[ "$DEBUG_MINIKUBE" == "" ]]; then
        minikube delete
    fi
    basic_teardown
}

function wait_for_default_sa(){
    count=0
    sa_ready=false
    # timeout after 30 seconds
    # if the default service account hasn't been created yet, there is something else wrong
    while [[ $count -lt 30 ]] && [[ $sa_ready == false ]]
    do
        run minikube kubectl get sa
        assert "$status" -eq 0
        if [[ "$output" != "No resources found in default namespace." ]]; then
            sa_ready=true
        fi
        count=$((count + 1))
        sleep 1
    done
    if [[ $sa_ready == false ]]; then
        die "Timed out waiting for default service account to be created"
    fi
}

function wait_for_pods_to_start(){
    count=0
    running=false
    # timeout after 30 seconds
    # if the pod hasn't started running after 30 seconds, there is something else wrong
    while [[ $count -lt 30 ]] && [[ $running == false ]]
    do
        run minikube kubectl get pods
        assert "$status" -eq 0
        if [[ "$output" =~ "Running" ]]; then
            running=true
        fi
        count=$((count + 1))
        sleep 1
    done
    if [[ $running == false ]]; then
        die "Timed out waiting for pod to move to 'Running' state"
    fi
}
