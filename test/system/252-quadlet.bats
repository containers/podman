#!/usr/bin/env bats   -*- bats -*-
#
# Tests generated configurations for systemd.
#

load helpers
load helpers.systemd

UNIT_FILES=()

function start_time() {
    sleep_to_next_second # Ensure we're on a new second with no previous logging
    STARTED_TIME=$(date "+%F %R:%S") # Start time for new log time
}

function setup() {
    skip_if_remote "quadlet tests are meaningless over remote"

    start_time

    basic_setup
}

function teardown() {
    for UNIT_FILE in ${UNIT_FILES[@]}; do
        if [[ -e "$UNIT_FILE" ]]; then
            local service=$(basename "$UNIT_FILE")
            run systemctl stop "$service"
            if [ $status -ne 0 ]; then
               echo "# WARNING: systemctl stop failed in teardown: $output" >&3
            fi
            rm -f "$UNIT_FILE"
        fi
    done
    systemctl daemon-reload

    basic_teardown
}

# Converts the quadlet file and installs the result it in $UNIT_DIR
function run_quadlet() {
    local sourcefile="$1"
    local service=$(quadlet_to_service_name "$sourcefile")

    # quadlet always works on an entire directory, so copy the file
    # to transform to a tmpdir
    local quadlet_tmpdir=$(mktemp -d --tmpdir=$PODMAN_TMPDIR quadlet.XXXXXX)
    cp $sourcefile $quadlet_tmpdir/

    QUADLET_UNIT_DIRS="$quadlet_tmpdir" run $QUADLET $_DASHUSER $UNIT_DIR
    assert $status -eq 0 "Failed to convert quadlet file: $sourcefile"
    is "$output" "" "quadlet should report no errors"

    # Ensure this is teared down
    UNIT_FILES+=("$UNIT_DIR/$service")

    QUADLET_SERVICE_NAME="$service"
    QUADLET_SYSLOG_ID="$(basename $service .service)"
    QUADLET_CONTAINER_NAME="systemd-$QUADLET_SYSLOG_ID"

    cat $UNIT_DIR/$QUADLET_SERVICE_NAME
}

function service_setup() {
    local service="$1"
    local option="$2"

    systemctl daemon-reload

    local startargs=""
    local statusexit=0
    local activestate="active"

    # If option wait, start and wait for service to exist
    if [ "$option" == "wait" ]; then
        startargs="--wait"
        statusexit=3
        local activestate="inactive"
    fi

    run systemctl $startargs start "$service"
    assert $status -eq 0 "Error starting systemd unit $service: $output"

    run systemctl status "$service"
    assert $status -eq $statusexit "systemctl status $service: $output"

    run systemctl show --value --property=ActiveState "$service"
    assert $status -eq 0 "systemctl show $service: $output"
    is "$output" $activestate
}

# Helper to stop a systemd service running a container
function service_cleanup() {
    local service="$1"
    local expected_state="$2"

    run systemctl stop "$service"
    assert $status -eq 0 "Error stopping systemd unit $service: $output"

    # Regression test for #11304: confirm that unit stops into correct state
    if [[ -n "$expected_state" ]]; then
        run systemctl show --property=ActiveState "$service"
        assert "$output" = "ActiveState=$expected_state" \
               "state of service $service after systemctl stop"
    fi

    rm -f "$UNIT_DIR/$service"
    systemctl daemon-reload
}

@test "quadlet - basic" {
    local quadlet_file=$PODMAN_TMPDIR/basic_$(random_string).container
    cat > $quadlet_file <<EOF
[Container]
Image=$IMAGE
Exec=sh -c "echo STARTED CONTAINER; echo "READY=1" | socat -u STDIN unix-sendto:\$NOTIFY_SOCKET; top"
Notify=yes
EOF

    run_quadlet "$quadlet_file"
    service_setup $QUADLET_SERVICE_NAME

    # Ensure we have output. Output is synced via sd-notify (socat in Exec)
    run journalctl "--since=$STARTED_TIME" --unit="$QUADLET_SERVICE_NAME"
    is "$output" '.*STARTED CONTAINER.*'

    run_podman container inspect  --format "{{.State.Status}}" $QUADLET_CONTAINER_NAME
    is "$output" "running" "container should be started by systemd and hence be running"

    service_cleanup $QUADLET_SERVICE_NAME failed
}

@test "quadlet - envvar" {
    local quadlet_file=$PODMAN_TMPDIR/envvar_$(random_string).container
    cat > $quadlet_file <<EOF
[Container]
Image=$IMAGE
Exec=sh -c "echo OUTPUT: \"\$FOOBAR\" \"\$BAR\""
Environment="FOOBAR=Foo  Bar" BAR=bar
EOF

    run_quadlet "$quadlet_file"
    service_setup $QUADLET_SERVICE_NAME wait

    # Ensure we have the right output, sync is done via waiting for service to exit (service_setup wait)

    # Note: Here we have to filter by syslog id instead of unit, because there is a (known) race
    # condition where if the cgroup is cleaned up before journald sees the message, then the journal
    # doesn't know the cgroup  and thus not the unit. (See https://github.com/systemd/systemd/issues/2913)
    run journalctl "--since=$STARTED_TIME"  SYSLOG_IDENTIFIER="$QUADLET_SYSLOG_ID"
    is "$output" '.*OUTPUT: Foo  Bar bar.*'

    service_cleanup $QUADLET_SERVICE_NAME inactive
}

@test "quadlet - ContainerName" {
    local quadlet_file=$PODMAN_TMPDIR/containername_$(random_string).container
    cat > $quadlet_file <<EOF
[Container]
ContainerName=customcontainername
Image=$IMAGE
Exec=top"
EOF

    run_quadlet "$quadlet_file"
    service_setup $QUADLET_SERVICE_NAME

    # Ensure we can access with the custom container name
    run_podman container inspect  --format "{{.State.Status}}" customcontainername
    is "$output" "running" "container should be started by systemd and hence be running"

    service_cleanup $QUADLET_SERVICE_NAME failed
}

@test "quadlet - labels" {
    local quadlet_file=$PODMAN_TMPDIR/labels_$(random_string).container
    cat > $quadlet_file <<EOF
[Container]
Image=$IMAGE
Exec=top
Label="foo=foo bar" "key=val"
Annotation="afoo=afoo bar"
Annotation="akey=aval"
EOF

    run_quadlet "$quadlet_file"
    service_setup $QUADLET_SERVICE_NAME

    run_podman container inspect --format "{{.Config.Labels.foo}}" $QUADLET_CONTAINER_NAME
    is "$output" "foo bar"
    run_podman container inspect --format "{{.Config.Labels.key}}" $QUADLET_CONTAINER_NAME
    is "$output" "val"
    run_podman container inspect --format "{{.Config.Annotations.afoo}}" $QUADLET_CONTAINER_NAME
    is "$output" "afoo bar"
    run_podman container inspect --format "{{.Config.Annotations.akey}}" $QUADLET_CONTAINER_NAME
    is "$output" "aval"

    service_cleanup $QUADLET_SERVICE_NAME failed
}

@test "quadlet - oneshot" {
    local quadlet_file=$PODMAN_TMPDIR/oneshot_$(random_string).container
    cat > $quadlet_file <<EOF
[Container]
Image=$IMAGE
Exec=echo INITIALIZED

[Service]
Type=oneshot
RemainAfterExit=yes
EOF

    run_quadlet "$quadlet_file"

    service_setup $QUADLET_SERVICE_NAME

    # Ensure we have output. Output is synced by oneshot command exit
    run journalctl "--since=$STARTED_TIME"  SYSLOG_IDENTIFIER="$QUADLET_SYSLOG_ID"
    is "$output" '.*INITIALIZED.*'

    service_cleanup $QUADLET_SERVICE_NAME inactive
}

@test "quadlet - volume" {
    local quadlet_file=$PODMAN_TMPDIR/basic_$(random_string).volume
    cat > $quadlet_file <<EOF
[Volume]
Label=foo=bar other="with space"
EOF

    run_quadlet "$quadlet_file"

    service_setup $QUADLET_SERVICE_NAME

    local volume_name=systemd-$(basename $quadlet_file .volume)
    run_podman volume ls
    is "$output" ".*local.*${volume_name}.*"

    run_podman volume inspect  --format "{{.Labels.foo}}" $volume_name
    is "$output" "bar"
    run_podman volume inspect  --format "{{.Labels.other}}" $volume_name
    is "$output" "with space"

    service_cleanup $QUADLET_SERVICE_NAME inactive
}

# A quadlet container depends on a quadlet volume
@test "quadlet - volume dependency" {
    # Save the unit name to use as the volume for the container
    local quadlet_vol_unit=dep_$(random_string).volume
    local quadlet_vol_file=$PODMAN_TMPDIR/${quadlet_vol_unit}
    cat > $quadlet_vol_file <<EOF
[Volume]
EOF

    # Have quadlet create the systemd unit file for the volume unit
    run_quadlet "$quadlet_vol_file"

    # Save the volume service name since the variable will be overwritten
    local vol_service=$QUADLET_SERVICE_NAME
    local volume_name=systemd-$(basename $quadlet_vol_file .volume)

    local quadlet_file=$PODMAN_TMPDIR/user_$(random_string).container
    cat > $quadlet_file <<EOF
[Container]
Image=$IMAGE
Exec=top
Volume=$quadlet_vol_unit:/tmp
EOF

    # Have quadlet create the systemd unit file for the container unit
    run_quadlet "$quadlet_file"

    # Save the container service name for readability
    local container_service=$QUADLET_SERVICE_NAME

    # Volume should not exist
    run_podman 1 volume exists ${volume_name}

    # Start the container service which should also trigger the start of the volume service
    service_setup $container_service

    # Volume system unit should be active
    run systemctl show --property=ActiveState "$vol_service"
    assert "$output" = "ActiveState=active" \
           "volume should be active via dependency"

    # Volume should exist
    run_podman volume exists ${volume_name}

    # Shutdown the service and remove the volume
    service_cleanup $container_service failed
    run_podman volume rm $volume_name
}

@test "quadlet - network" {
    local quadlet_file=$PODMAN_TMPDIR/basic_$(random_string).network
    cat > $quadlet_file <<EOF
[Network]
Label=foo=bar other="with space"
EOF

    run_quadlet "$quadlet_file"

    service_setup $QUADLET_SERVICE_NAME

    local network_name=systemd-$(basename $quadlet_file .network)
    run_podman network exists $network_name

    run_podman network inspect  --format "{{.Labels.foo}}" $network_name
    is "$output" "bar"
    run_podman network inspect  --format "{{.Labels.other}}" $network_name
    is "$output" "with space"

    service_cleanup $QUADLET_SERVICE_NAME inactive
}

# A quadlet container depends on a quadlet network
@test "quadlet - network dependency" {
    # Save the unit name to use as the network for the container
    local quadlet_network_unit=dep_$(random_string).network
    local quadlet_network_file=$PODMAN_TMPDIR/${quadlet_network_unit}
    cat > $quadlet_network_file <<EOF
[Network]
EOF

    # Have quadlet create the systemd unit file for the network unit
    run_quadlet "$quadlet_network_file"

    # Save the volume service name since the variable will be overwritten
    local network_service=$QUADLET_SERVICE_NAME
    local network_name=systemd-$(basename $quadlet_network_file .network)

    local quadlet_file=$PODMAN_TMPDIR/user_$(random_string).container
    cat > $quadlet_file <<EOF
[Container]
Image=$IMAGE
Exec=top
Network=$quadlet_network_unit
EOF

    run_quadlet "$quadlet_file"

    # Save the container service name for readability
    local container_service=$QUADLET_SERVICE_NAME

    # Network should not exist
    run_podman 1 network exists $network_name

    service_setup $container_service

    # Network system unit should be active
    run systemctl show --property=ActiveState "$network_service"
    assert "$output" = "ActiveState=active" \
           "network should be active via dependency"

    # Network should exist
    run_podman network exists $network_name

    service_cleanup $QUADLET_SERVICE_NAME failed
    run_podman network rm $network_name
}

@test "quadlet kube - basic" {
    # Create the YAMl file
    yaml_source="$PODMAN_TMPDIR/basic_$(random_string).yaml"
    cat >$yaml_source <<EOF
apiVersion: v1
kind: Pod
metadata:
  labels:
    app: test
  name: test_pod
spec:
  containers:
  - command:
    - top
    image: $IMAGE
    name: test
EOF

    # Create the Quadlet file
    local quadlet_file=$PODMAN_TMPDIR/basic_$(random_string).kube
    cat > $quadlet_file <<EOF
[Kube]
Yaml=${yaml_source}
EOF

    run_quadlet "$quadlet_file"
    service_setup $QUADLET_SERVICE_NAME

    # Ensure we have output. Output is synced via sd-notify (socat in Exec)
    run journalctl "--since=$STARTED_TIME" --unit="$QUADLET_SERVICE_NAME"
    is "$output" '.*Started.*\.service.*'

    run_podman container inspect  --format "{{.State.Status}}" test_pod-test
    is "$output" "running" "container should be started by systemd and hence be running"

    service_cleanup $QUADLET_SERVICE_NAME inactive
    run_podman rmi $(pause_image)
}

# vim: filetype=sh
