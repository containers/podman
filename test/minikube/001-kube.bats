#!/usr/bin/env bats
#
# Tests of podman kube commands with minikube
#

load helpers.bash

###############################################################################
# BEGIN tests

@test "minikube - check cluster is up" {
    run minikube kubectl get nodes
    assert "$status" -eq 0 "get status of nodes"
    assert "$output" =~ "Ready"
    run minikube kubectl get pods
    assert "$status" -eq 0 "get pods in the default namespace"
    assert "$output" == "No resources found in default namespace."
    wait_for_default_sa
}

@test "minikube - deploy generated container yaml to minikube" {
    cname="test-ctr"
    fname="/tmp/minikube_deploy_$(random_string 6).yaml"
    run_podman container create --name $cname $IMAGE top
    run_podman kube generate -f $fname $cname

    # deploy to the minikube cluster
    project="ctr-ns"
    run minikube kubectl create namespace $project
    assert "$status" -eq 0 "create new namespace $project"
    run minikube kubectl -- apply -f $fname
    echo $output >&2
    assert "$status" -eq 0 "deploy $fname to the cluster"
    assert "$output" == "pod/$cname-pod created"
    wait_for_pods_to_start
    run minikube kubectl delete namespace $project
    assert $status -eq 0 "delete namespace $project"
}

@test "minikube - deploy generated pod yaml to minikube" {
    pname="test-pod"
    cname1="test-ctr1"
    cname2="test-ctr2"
    fname="/tmp/minikube_deploy_$(random_string 6).yaml"

    run_podman pod create --name $pname --publish 9999:8888
    run_podman container create --name $cname1 --pod $pname $IMAGE sleep 1000
    run_podman container create --name $cname2 --pod $pname $IMAGE sleep 2000
    run_podman kube generate -f $fname $pname

    # deploy to the minikube cluster
    project="pod-ns"
    run minikube kubectl create namespace $project
    assert "$status" -eq 0 "create new namespace $project"
    run minikube kubectl -- apply -f $fname
    assert "$status" -eq 0 "deploy $fname to the cluster"
    assert "$output" == "pod/$pname created"
    wait_for_pods_to_start
    run minikube kubectl delete namespace $project
    assert $status -eq 0 "delete namespace $project"
}
