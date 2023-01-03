#!/usr/bin/env bats   -*- bats -*-
#
# Tests generated configurations for systemd.
#

load helpers
load helpers.systemd

SERVICE_NAME="podman_test_$(random_string)"

UNIT_FILE="$UNIT_DIR/$SERVICE_NAME.service"
TEMPLATE_FILE_PREFIX="$UNIT_DIR/$SERVICE_NAME"

function setup() {
    skip_if_remote "systemd tests are meaningless over remote"

    basic_setup
}

function teardown() {
    if [[ -e "$UNIT_FILE" ]]; then
        run systemctl stop "$SERVICE_NAME"
        if [ $status -ne 0 ]; then
            echo "# WARNING: systemctl stop failed in teardown: $output" >&3
        fi

        rm -f "$UNIT_FILE"
        systemctl daemon-reload
    fi

    basic_teardown
}

# Helper to start a systemd service running a container
function service_setup() {
    run_podman generate systemd \
               -e http_proxy -e https_proxy -e no_proxy \
               -e HTTP_PROXY -e HTTPS_PROXY -e NO_PROXY \
               --new $cname
    echo "$output" > "$UNIT_FILE"
    run_podman rm $cname

    systemctl daemon-reload

    # Also test enabling services (see #12438).
    run systemctl enable "$SERVICE_NAME"
    assert $status -eq 0 "Error enabling systemd unit $SERVICE_NAME: $output"

    run systemctl start "$SERVICE_NAME"
    assert $status -eq 0 "Error starting systemd unit $SERVICE_NAME: $output"

    run systemctl status "$SERVICE_NAME"
    assert $status -eq 0 "systemctl status $SERVICE_NAME: $output"
}

# Helper to stop a systemd service running a container
function service_cleanup() {
    run systemctl stop "$SERVICE_NAME"
    assert $status -eq 0 "Error stopping systemd unit $SERVICE_NAME: $output"

    # Regression test for #11304: confirm that unit stops into correct state
    local expected_state="$1"
    if [[ -n "$expected_state" ]]; then
        run systemctl show --property=ActiveState "$SERVICE_NAME"
        assert "$output" = "ActiveState=$expected_state" \
               "state of service after systemctl stop"
    fi

    run systemctl disable "$SERVICE_NAME"
    assert $status -eq 0 "Error disabling systemd unit $SERVICE_NAME: $output"

    rm -f "$UNIT_FILE"
    systemctl daemon-reload
}

# These tests can fail in dev. environment because of SELinux.
# quick fix: chcon -t container_runtime_exec_t ./bin/podman
@test "podman generate - systemd - basic" {
    # Flakes with "ActiveState=failed (expected =inactive)"
    if is_ubuntu; then
        skip "FIXME: 2022-09-01: requires conmon-2.1.4, ubuntu has 2.1.3"
    fi

    # Warn when a custom restart policy is used without --new (see #15284)
    run_podman create --restart=always $IMAGE
    cid="$output"
    run_podman generate systemd $cid
    is "$output" ".*Container $cid has restart policy .*always.* which can lead to issues on shutdown.*" "generate systemd emits warning"
    run_podman rm -f $cid

    cname=$(random_string)
    # See #7407 for --pull=always.
    run_podman create --pull=always --name $cname --label "io.containers.autoupdate=registry" $IMAGE \
        sh -c "trap 'echo Received SIGTERM, finishing; exit' SIGTERM; echo WAITING; while :; do sleep 0.1; done"

    # Start systemd service to run this container
    service_setup

    # Give container time to start; make sure output looks top-like
    sleep 2
    run_podman logs $cname
    is "$output" ".*WAITING.*" "running is waiting for signal"

    # All good. Stop service, clean up.
    # Also make sure the service is in the `inactive` state (see #11304).
    service_cleanup inactive
}

@test "podman autoupdate local" {
    # Note that the entrypoint may be a JSON string which requires preserving the quotes (see #12477)
    cname=$(random_string)

    # Create a scratch image (copy of our regular one)
    image_copy=base$(random_string | tr A-Z a-z)
    run_podman tag $IMAGE $image_copy

    # Create a container based on that
    run_podman create --name $cname --label "io.containers.autoupdate=local" --entrypoint '["top"]' $image_copy

    # Start systemd service to run this container
    service_setup

    # Give container time to start; make sure output looks top-like
    wait_for_output 'Load average' $cname

    # Run auto-update and check that it restarted the container
    run_podman commit --change "CMD=/bin/bash" $cname $image_copy
    run_podman auto-update
    is "$output" ".*$SERVICE_NAME.*" "autoupdate local restarted container"

    # All good. Stop service, clean up.
    service_cleanup
    run_podman rmi $image_copy
}

# These tests can fail in dev. environment because of SELinux.
# quick fix: chcon -t container_runtime_exec_t ./bin/podman
@test "podman generate systemd - envar" {
    cname=$(random_string)
    FOO=value BAR=%s run_podman create --name $cname --env FOO -e BAR --env MYVAR=myval \
        $IMAGE sh -c 'printenv && sleep 100'

    # Start systemd service to run this container
    service_setup

    # Give container time to start; make sure output looks top-like
    sleep 2
    run_podman logs $cname
    is "$output" ".*FOO=value.*" "FOO environment variable set"
    is "$output" ".*BAR=%s.*" "BAR environment variable set"
    is "$output" ".*MYVAR=myval.*" "MYVAL environment variable set"

    # All good. Stop service, clean up.
    service_cleanup
}

# Regression test for #11438
@test "podman generate systemd - restart policy & timeouts" {
    cname=$(random_string)
    run_podman create --restart=always --name $cname $IMAGE
    run_podman generate systemd --new $cname
    is "$output" ".*Restart=always.*" "Use container's restart policy if set"
    run_podman generate systemd --new --restart-policy=on-failure $cname
    is "$output" ".*Restart=on-failure.*" "Override container's restart policy"

    cname2=$(random_string)
    run_podman create --restart=unless-stopped --name $cname2 $IMAGE
    run_podman generate systemd --new $cname2
    is "$output" ".*Restart=always.*" "unless-stopped translated to always"

    cname3=$(random_string)
    run_podman create --restart=on-failure:42 --name $cname3 $IMAGE
    run_podman generate systemd --new $cname3
    is "$output" ".*Restart=on-failure.*" "on-failure:xx is parsed correctly"
    is "$output" ".*StartLimitBurst=42.*" "on-failure:xx is parsed correctly"

    run_podman rm -t 0 -f $cname $cname2 $cname3
}

function set_listen_env() {
    export LISTEN_PID="100" LISTEN_FDS="1" LISTEN_FDNAMES="listen_fdnames"
}

function unset_listen_env() {
    unset LISTEN_PID LISTEN_FDS LISTEN_FDNAMES
}

function check_listen_env() {
    local stdenv="$1"
    local context="$2"
    if is_remote; then
	is "$output" "$stdenv" "LISTEN Environment did not pass: $context"
    else
	out=$(for o in $output; do echo $o; done| sort)
	std=$(echo "$stdenv
LISTEN_PID=1
LISTEN_FDS=1
LISTEN_FDNAMES=listen_fdnames" | sort)
       echo "<$out>"
       echo "<$std>"
       is "$out" "$std" "LISTEN Environment passed: $context"
    fi
}

@test "podman pass LISTEN environment " {
    # Note that `--hostname=host1` makes sure that all containers have the same
    # environment.
    run_podman run --hostname=host1 --rm $IMAGE printenv
    stdenv=$output

    # podman run
    set_listen_env
    run_podman run --hostname=host1 --rm $IMAGE printenv
    unset_listen_env
    check_listen_env "$stdenv" "podman run"

    # podman start
    run_podman create --hostname=host1 --rm $IMAGE printenv
    cid="$output"
    set_listen_env
    run_podman start --attach $cid
    unset_listen_env
    check_listen_env "$stdenv" "podman start"
}

@test "podman generate - systemd template" {
    cname=$(random_string)
    run_podman create --name $cname $IMAGE top

    run_podman generate systemd --template -n $cname
    echo "$output" > "$TEMPLATE_FILE_PREFIX@.service"
    run_podman rm -f $cname

    systemctl daemon-reload

    INSTANCE="$SERVICE_NAME@1.service"
    run systemctl start "$INSTANCE"
    assert $status -eq 0 "Error starting systemd unit $INSTANCE: $output"

    run systemctl status "$INSTANCE"
    assert $status -eq 0 "systemctl status $INSTANCE: $output"

    run systemctl stop "$INSTANCE"
    assert $status -eq 0 "Error stopping systemd unit $INSTANCE: $output"

    rm -f "$TEMPLATE_FILE_PREFIX@.service"
    systemctl daemon-reload
}

@test "podman generate - systemd template no support for pod" {
    cname=$(random_string)
    podname=$(random_string)
    run_podman pod create --name $podname
    run_podman run --pod $podname -dt --name $cname $IMAGE top

    run_podman 125 generate systemd --new --template -n $podname
    is "$output" ".*--template is not supported for pods.*" "Error message contains 'not supported'"

    run_podman rm -f $cname
    run_podman pod rm -f $podname
    run_podman rmi $(pause_image)
}

@test "podman generate - systemd template only used on --new" {
    cname=$(random_string)
    run_podman create --name $cname $IMAGE top
    run_podman 125 generate systemd --new=false --template -n $cname
    is "$output" ".*--template cannot be set" "Error message should be '--template requires --new'"
}

@test "podman --cgroup=cgroupfs doesn't show systemd warning" {
    DBUS_SESSION_BUS_ADDRESS= run_podman --log-level warning --cgroup-manager=cgroupfs info -f ''
    is "$output" "" "output should be empty"
}

@test "podman --systemd sets container_uuid" {
    run_podman run --systemd=always --name test $IMAGE printenv container_uuid
    container_uuid=$output
    run_podman inspect test --format '{{ .ID }}'
    is "${container_uuid}" "${output:0:32}" "UUID should be first 32 chars of Container id"
}

# https://github.com/containers/podman/issues/13153
@test "podman rootless-netns slirp4netns process should be in different cgroup" {
    is_rootless || skip "only meaningful for rootless"

    cname=$(random_string)
    local netname=testnet-$(random_string 10)

    # create network and container with network
    run_podman network create $netname
    run_podman create --name $cname --network $netname $IMAGE top

    # run container in systemd unit
    service_setup

    # run second container with network
    cname2=$(random_string)
    run_podman run -d --name $cname2 --network $netname $IMAGE top

    # stop systemd container
    service_cleanup

    # now check that the rootless netns slirp4netns process is still alive and working
    run_podman unshare --rootless-netns ip addr
    is "$output" ".*tap0.*" "slirp4netns interface exists in the netns"
    run_podman exec $cname2 nslookup google.com

    run_podman rm -f -t0 $cname2
    run_podman network rm -f $netname
}

@test "podman create --health-on-failure=kill" {
    img="healthcheck_i"
    _build_health_check_image $img

    cname=c_$(random_string)
    run_podman create --name $cname      \
               --health-cmd /healthcheck \
               --health-on-failure=kill  \
               --health-retries=1        \
               --restart=on-failure      \
               $img

    # run container in systemd unit
    service_setup

    run_podman container inspect $cname --format "{{.ID}}"
    oldID="$output"

    run_podman healthcheck run $cname

    # Now cause the healthcheck to fail
    run_podman exec $cname touch /uh-oh

    # healthcheck should now fail, with exit status 1 and 'unhealthy' output
    run_podman 1 healthcheck run $cname
    is "$output" "unhealthy" "output from 'podman healthcheck run'"

    # What is expected to happen now:
    #  1) The container gets killed as the health check has failed
    #  2) Systemd restarts the service as the restart policy is set to "on-failure"
    #  3) The /uh-oh file is gone and $cname has another ID

    # Wait at most 10 seconds for the service to be restarted
    local timeout=10
    while [[ $timeout -gt 1 ]]; do
        # Possible outcomes:
        #  - status 0, old container is still terminating: sleep and retry
        #  - status 0, new CID: yay, break
        #  - status 1, container not found: sleep and retry
        run_podman '?' container inspect $cname --format '{{.ID}}'
        if [[ $status == 0 ]]; then
            if [[ "$output" != "$oldID" ]]; then
                break
            fi
        fi
        sleep 1
        let timeout=$timeout-1
    done

    run_podman healthcheck run $cname

    # stop systemd container
    service_cleanup
    run_podman rmi -f $img
}

@test "podman-kube@.service template" {
    install_kube_template
    # Create the YAMl file
    yaml_source="$PODMAN_TMPDIR/test.yaml"
    cat >$yaml_source <<EOF
apiVersion: v1
kind: Pod
metadata:
  annotations:
      io.containers.autoupdate: "local"
      io.containers.autoupdate/b: "registry"
  labels:
    app: test
  name: test_pod
spec:
  containers:
  - command:
    - top
    image: $IMAGE
    name: a
  - command:
    - top
    image: $IMAGE
    name: b
EOF

    # Dispatch the YAML file
    service_name="podman-kube@$(systemd-escape $yaml_source).service"
    systemctl start $service_name
    systemctl is-active $service_name

    # Make sure that Podman is the service's MainPID
    run systemctl show --property=MainPID --value $service_name
    is "$(</proc/$output/comm)" "podman" "podman is the service mainPID"

    # The name of the service container is predictable: the first 12 characters
    # of the hash of the YAML file followed by the "-service" suffix
    yaml_sha=$(sha256sum $yaml_source)
    service_container="${yaml_sha:0:12}-service"

    # Make sure that the service container exists and runs.
    run_podman container inspect $service_container --format "{{.State.Running}}"
    is "$output" "true"

    # Check for an error when trying to remove the service container
    run_podman 125 container rm $service_container
    is "$output" "Error: container .* is the service container of pod(s) .* and cannot be removed without removing the pod(s)"

    # Verify that the log-driver for the Pod's containers is passthrough
    for name in "a" "b"; do
        run_podman container inspect test_pod-${name} --format "{{.HostConfig.LogConfig.Type}}"
        is $output "passthrough"
    done

    # Add a simple `auto-update --dry-run` test here to avoid too much redundancy
    # with 255-auto-update.bats
    run_podman auto-update --dry-run --format "{{.Unit}},{{.Container}},{{.Image}},{{.Updated}},{{.Policy}}"
    is "$output" ".*$service_name,.* (test_pod-a),$IMAGE,false,local.*" "global auto-update policy gets applied"
    is "$output" ".*$service_name,.* (test_pod-b),$IMAGE,false,registry.*" "container-specified auto-update policy gets applied"

    # Kill the pod and make sure the service is not running.
    # The restart policy is set to "never" since there is no
    # design yet for propagating exit codes up to the service
    # container.
    run_podman pod kill test_pod
    for i in {0..5}; do
        run systemctl is-active $service_name
        if [[ $output == "inactive" ]]; then
            break
        fi
        sleep 0.5
    done
    is "$output" "inactive" "systemd service transitioned to 'inactive' state: $service_name"

    # Now stop and start the service again.
    systemctl stop $service_name
    systemctl start $service_name
    systemctl is-active $service_name
    run_podman container inspect $service_container --format "{{.State.Running}}"
    is "$output" "true"

    # Clean up
    systemctl stop $service_name
    run_podman 1 container exists $service_container
    run_podman 1 pod exists test_pod
    run_podman rmi $(pause_image)
    rm -f $UNIT_DIR/$unit_name
}

# vim: filetype=sh
