# -*- bash -*-

load ../system/helpers.bash

KUBECONFIG="$HOME/.kube/config"

##################
#  run_minikube  #  Local helper, with instrumentation for debugging failures
##################
function run_minikube() {
    # Number as first argument = expected exit code; default 0
    local expected_rc=0
    case "$1" in
        [0-9])           expected_rc=$1; shift;;
        [1-9][0-9])      expected_rc=$1; shift;;
        [12][0-9][0-9])  expected_rc=$1; shift;;
        '?')             expected_rc=  ; shift;;  # ignore exit code
    esac

    # stdout is only emitted upon error; this printf is to help in debugging
    printf "\n%s %s %s %s\n" "$(timestamp)" "\$" "minikube" "$*"
    run minikube "$@"
    # without "quotes", multiple lines are glommed together into one
    if [[ -n "$output" ]]; then
        echo "$(timestamp) $output"
    fi
    if [[ "$status" -ne 0 ]]; then
        echo -n "$(timestamp) [ rc=$status ";
        if [[ -n "$expected_rc" ]]; then
            if [[ "$status" -eq "$expected_rc" ]]; then
                echo -n "(expected) ";
            else
                echo -n "(** EXPECTED $expected_rc **) ";
            fi
        fi
        echo "]"
    fi

    if [[ -n "$expected_rc" ]]; then
        if [[ "$status" -ne "$expected_rc" ]]; then
            # Further debugging
            echo "\$ minikube logs"
            run minikube logs
            echo "$output"

            die "exit code is $status; expected $expected_rc"
        fi
    fi
}


function setup(){
    # only set up the minikube cluster before the first test
    if [[ "$BATS_TEST_NUMBER" -eq 1 ]]; then
        run_minikube start
        wait_for_default_sa
    fi
    basic_setup
}

function teardown(){
    # only delete the minikube cluster if we are done with the last test
    # the $DEBUG_MINIKUBE env can be set to preserve the cluster to debug if needed
    if [[ "$BATS_TEST_NUMBER" -eq ${#BATS_TEST_NAMES[@]} ]] && [[ "$DEBUG_MINIKUBE" == "" ]]; then
        run_minikube delete
    fi

    # Prevents nasty red warnings in log
    run_podman rmi --ignore $(pause_image)

    basic_teardown
}

function wait_for_default_sa(){
    count=0
    sa_ready=false
    # timeout after 30 seconds
    # if the default service account hasn't been created yet, there is something else wrong
    while [[ $count -lt 30 ]] && [[ $sa_ready == false ]]
    do
        run_minikube kubectl get sa
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
        run_minikube kubectl get pods
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
