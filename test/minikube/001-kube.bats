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

@test "minikube - apply podman ctr to cluster" {
    cname="test-ctr-apply"
    run_podman container create --name $cname $IMAGE top

    # deploy to minikube cluster with kube apply
    project="ctr-apply"
    run minikube kubectl create namespace $project
    assert "$status" -eq 0 "create new namespace $project"
    run_podman kube apply --kubeconfig $KUBECONFIG --ns $project $cname
    assert "$output" =~ "Successfully deployed workloads to cluster!"
    run minikube kubectl -- get pods --namespace $project
    assert "$status" -eq 0 "kube apply $cname to the cluster"
    assert "$output" =~ "$cname-pod"
    wait_for_pods_to_start
    run minikube kubectl delete namespace $project
    assert $status -eq 0 "delete namespace $project"
}

@test "minikube - apply podman pod to cluster" {
    pname="test-pod-apply"
    run_podman pod create --name $pname
    run podman container create --pod $pname $IMAGE top

    # deploy to minikube cluster with kube apply
    project="pod-apply"
    run minikube kubectl create namespace $project
    assert "$status" -eq 0 "create new namespace $project"
    run_podman kube apply --kubeconfig $KUBECONFIG --ns $project $pname
    assert "$output" =~ "Successfully deployed workloads to cluster!"
    run minikube kubectl -- get pods --namespace $project
    assert "$status" -eq 0 "kube apply $pname to the cluster"
    assert "$output" =~ "$pname"
    wait_for_pods_to_start
    run minikube kubectl delete namespace $project
    assert $status -eq 0 "delete namespace $project"
}

@test "minikube - deploy generated kube yaml with podman kube apply to cluster" {
    pname="test-pod"
    cname1="test-ctr1"
    cname2="test-ctr2"
    fname="/tmp/minikube_deploy_$(random_string 6).yaml"

    run_podman pod create --name $pname --publish 9999:8888
    run_podman container create --name $cname1 --pod $pname $IMAGE sleep 1000
    run_podman container create --name $cname2 --pod $pname $IMAGE sleep 2000
    run_podman kube generate -f $fname $pname

    # deploy to minikube cluster with kube apply
    project="yaml-apply"
    run minikube kubectl create namespace $project
    assert "$status" -eq 0 "create new namespace $project"
    run_podman kube apply --kubeconfig $KUBECONFIG --ns $project -f $fname
    assert "$output" =~ "Successfully deployed workloads to cluster!"
    run minikube kubectl -- get pods --namespace $project
    assert "$status" -eq 0 "kube apply $pname to the cluster"
    assert "$output" =~ "$pname"
    wait_for_pods_to_start
    run minikube kubectl delete namespace $project
    assert $status -eq 0 "delete namespace $project"
}

@test "minikube - apply podman ctr with volume to cluster" {
    cname="ctr-vol"
    vname="myvol"
    run_podman container create -v $vname:/myvol --name $cname $IMAGE top

    # deploy to minikube cluster with kube apply
    project="ctr-vol-apply"
    run minikube kubectl create namespace $project
    assert "$status" -eq 0 "create new namespace $project"
    run_podman kube apply --kubeconfig $KUBECONFIG --ns $project $cname $vname
    assert "$output" =~ "Successfully deployed workloads to cluster!"
    run minikube kubectl -- get pods --namespace $project
    assert "$status" -eq 0 "kube apply $cname to the cluster"
    assert "$output" =~ "$cname-pod"
    run minikube kubectl -- get pvc --namespace $project
    assert "$status" -eq 0 "kube apply $vname to the cluster"
    assert "$output" =~ "$vname"
    wait_for_pods_to_start
    run minikube kubectl delete namespace $project
    assert $status -eq 0 "delete namespace $project"
}

@test "minikube - apply podman ctr with service to cluster" {
    cname="ctr-svc"
    run_podman container create -p 3000:4000 --name $cname $IMAGE top

    # deploy to minikube cluster with kube apply
    project="ctr-svc-apply"
    run minikube kubectl create namespace $project
    assert "$status" -eq 0 "create new namespace $project"
    run_podman kube apply --kubeconfig $KUBECONFIG -s --ns $project $cname
    assert "$output" =~ "Successfully deployed workloads to cluster!"
    run minikube kubectl -- get pods --namespace $project
    assert "$status" -eq 0 "kube apply $cname to the cluster"
    assert "$output" =~ "$cname-pod"
    run minikube kubectl -- get svc --namespace $project
    assert "$status" -eq 0 "kube apply service to the cluster"
    assert "$output" =~ "$cname-pod"
    wait_for_pods_to_start
    run minikube kubectl delete namespace $project
    assert $status -eq 0 "delete namespace $project"
}
