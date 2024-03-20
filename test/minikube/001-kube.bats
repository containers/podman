#!/usr/bin/env bats
#
# Tests of podman kube commands with minikube
#

load helpers.bash

###############################################################################
# BEGIN tests

@test "minikube - check cluster is up" {
    run_minikube kubectl get nodes
    assert "$output" =~ "Ready"
    run_minikube kubectl get pods
    assert "$output" == "No resources found in default namespace."
}

@test "minikube - deploy generated container yaml to minikube" {
    cname="test-ctr"
    fname="/tmp/minikube_deploy_$(random_string 6).yaml"
    run_podman container create --name $cname $IMAGE top
    run_podman kube generate -f $fname $cname

    # deploy to the minikube cluster
    project="ctr-ns"
    run_minikube kubectl create namespace $project
    run_minikube kubectl -- apply -f $fname
    assert "$output" == "pod/$cname-pod created"
    wait_for_pods_to_start
    run_minikube kubectl delete namespace $project
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
    run_minikube kubectl create namespace $project
    run_minikube kubectl -- apply -f $fname
    assert "$output" == "pod/$pname created"
    wait_for_pods_to_start
    run_minikube kubectl delete namespace $project
}

@test "minikube - apply podman ctr to cluster" {
    cname="test-ctr-apply"
    run_podman container create --name $cname $IMAGE top

    # deploy to minikube cluster with kube apply
    project="ctr-apply"
    run_minikube kubectl create namespace $project
    run_podman kube apply --kubeconfig $KUBECONFIG --ns $project $cname
    assert "$output" =~ "Successfully deployed workloads to cluster!"
    run_minikube kubectl -- get pods --namespace $project
    assert "$output" =~ "$cname-pod"
    wait_for_pods_to_start
    run_minikube kubectl delete namespace $project
}

@test "minikube - apply podman pod to cluster" {
    pname="test-pod-apply"
    run_podman pod create --name $pname
    run_podman container create --pod $pname $IMAGE top

    # deploy to minikube cluster with kube apply
    project="pod-apply"
    run_minikube kubectl create namespace $project
    run_podman kube apply --kubeconfig $KUBECONFIG --ns $project $pname
    assert "$output" =~ "Successfully deployed workloads to cluster!"
    run_minikube kubectl -- get pods --namespace $project
    assert "$output" =~ "$pname"
    wait_for_pods_to_start
    run_minikube kubectl delete namespace $project
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
    run_minikube kubectl create namespace $project
    run_podman kube apply --kubeconfig $KUBECONFIG --ns $project -f $fname
    assert "$output" =~ "Successfully deployed workloads to cluster!"
    run_minikube kubectl -- get pods --namespace $project
    assert "$output" =~ "$pname"
    wait_for_pods_to_start
    run_minikube kubectl delete namespace $project
}

@test "minikube - apply podman ctr with volume to cluster" {
    cname="ctr-vol"
    vname="myvol"
    run_podman container create -v $vname:/myvol --name $cname $IMAGE top

    # deploy to minikube cluster with kube apply
    project="ctr-vol-apply"
    run_minikube kubectl create namespace $project
    run_podman kube apply --kubeconfig $KUBECONFIG --ns $project $cname $vname
    assert "$output" =~ "Successfully deployed workloads to cluster!"
    run_minikube kubectl -- get pods --namespace $project
    assert "$output" =~ "$cname-pod"
    run_minikube kubectl -- get pvc --namespace $project
    assert "$output" =~ "$vname"
    wait_for_pods_to_start
    run_minikube kubectl delete namespace $project
}

@test "minikube - apply podman ctr with service to cluster" {
    cname="ctr-svc"
    run_podman container create -p 3000:4000 --name $cname $IMAGE top

    # deploy to minikube cluster with kube apply
    project="ctr-svc-apply"
    run_minikube kubectl create namespace $project
    run_podman kube apply --kubeconfig $KUBECONFIG -s --ns $project $cname
    assert "$output" =~ "Successfully deployed workloads to cluster!"
    run_minikube kubectl -- get pods --namespace $project
    assert "$output" =~ "$cname-pod"
    run_minikube kubectl -- get svc --namespace $project
    assert "$output" =~ "$cname-pod"
    wait_for_pods_to_start
    run_minikube kubectl delete namespace $project
}

@test "minikube - deploy generated container yaml to minikube --type=deployment" {
    cname="test-ctr"
    fname="/tmp/minikube_deploy_$(random_string 6).yaml"
    run_podman container create --name $cname $IMAGE top
    run_podman kube generate --type deployment -f $fname $cname

    # deploy to the minikube cluster
    project="dep-ctr-ns"
    run_minikube kubectl create namespace $project
    run_minikube kubectl -- apply -f $fname
    assert "$output" == "deployment.apps/$cname-pod-deployment created"
    wait_for_pods_to_start
    run_minikube kubectl delete namespace $project
}

@test "minikube - deploy generated pod yaml to minikube --type=deployment" {
    pname="test-pod"
    cname1="test-ctr1"
    cname2="test-ctr2"
    fname="/tmp/minikube_deploy_$(random_string 6).yaml"

    run_podman pod create --name $pname --publish 9999:8888
    run_podman container create --name $cname1 --pod $pname $IMAGE sleep 1000
    run_podman container create --name $cname2 --pod $pname $IMAGE sleep 2000
    run_podman kube generate --type deployment -f $fname $pname

    # deploy to the minikube cluster
    project="dep-pod-ns"
    run_minikube kubectl create namespace $project
    run_minikube kubectl -- apply -f $fname
    assert "$output" == "deployment.apps/$pname-deployment created"
    wait_for_pods_to_start
    run_minikube kubectl delete namespace $project
}

@test "minikube - deploy generated container yaml to minikube --type=daemonset" {
    cname="test-ctr"
    fname="/tmp/minikube_deploy_$(random_string 6).yaml"
    run_podman container create --name $cname $IMAGE top
    run_podman kube generate --type daemonset -f $fname $cname

    # deploy to the minikube cluster
    project="dep-ctr-ns"
    run_minikube kubectl create namespace $project
    run_minikube kubectl -- apply -f $fname
    assert "$output" == "daemonset.apps/$cname-pod-daemonset created"
    wait_for_pods_to_start
    run_minikube kubectl delete namespace $project
}

@test "minikube - deploy generated pod yaml to minikube --type=daemonset" {
    pname="test-pod"
    cname1="test-ctr1"
    cname2="test-ctr2"
    fname="/tmp/minikube_deploy_$(random_string 6).yaml"

    run_podman pod create --name $pname --publish 9999:8888
    run_podman container create --name $cname1 --pod $pname $IMAGE sleep 1000
    run_podman container create --name $cname2 --pod $pname $IMAGE sleep 2000
    run_podman kube generate --type daemonset -f $fname $pname

    # deploy to the minikube cluster
    project="dep-pod-ns"
    run_minikube kubectl create namespace $project
    run_minikube kubectl -- apply -f $fname
    assert "$output" == "daemonset.apps/$pname-daemonset created"
    wait_for_pods_to_start
    run_minikube kubectl delete namespace $project
}
