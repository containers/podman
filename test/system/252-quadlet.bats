#!/usr/bin/env bats   -*- bats -*-
#
# Tests generated configurations for systemd.
#

load helpers
load helpers.network
load helpers.registry
load helpers.systemd

UNIT_FILES=()

function start_time() {
    sleep_to_next_second # Ensure we're on a new second with no previous logging
    STARTED_TIME=$(date "+%F %R:%S") # Start time for new log time
}

function setup() {
    skip_if_remote "quadlet tests are meaningless over remote"
    skip_if_rootless_cgroupsv1 "Can't use --cgroups=split w/ CGv1 (issue 17456, wontfix)"
    skip_if_journald_unavailable "Needed for RHEL. FIXME: we might be able to re-enable a subset of tests."

    test -x "$QUADLET" || die "Cannot run quadlet tests without executable \$QUADLET ($QUADLET)"

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
    # to transform to the given or newly created tmpdir
    local quadlet_tmpdir="$2"
    if [ -z "$quadlet_tmpdir" ]; then
        quadlet_tmpdir=$(mktemp -d --tmpdir=$PODMAN_TMPDIR quadlet.XXXXXX)
    fi
    cp $sourcefile $quadlet_tmpdir/

    echo "$_LOG_PROMPT $QUADLET $_DASHUSER $UNIT_DIR"
    QUADLET_UNIT_DIRS="$quadlet_tmpdir" run \
                     timeout --foreground -v --kill=10 $PODMAN_TIMEOUT \
                     $QUADLET $_DASHUSER $UNIT_DIR
    echo "$output"
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

    echo "$_LOG_PROMPT systemctl $startargs start $service"
    run systemctl $startargs start "$service"
    echo "$output"
    assert $status -eq 0 "Error starting systemd unit $service"

    # FIXME FIXME FIXME: this is racy with short-lived containers!
    echo "$_LOG_PROMPT systemctl status $service"
    run systemctl status "$service"
    echo "$output"
    assert $status -eq $statusexit "systemctl status $service"

    echo "$_LOG_PROMPT systemctl show --value --property=ActiveState $service"
    run systemctl show --value --property=ActiveState "$service"
    echo "$output"
    assert $status -eq 0 "systemctl show $service"
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

function create_secret() {
    local secret_name=$(random_string)
    local secret_file=$PODMAN_TMPDIR/secret_$(random_string)
    local secret=$(random_string)

    echo $secret > $secret_file
    run_podman secret create $secret_name $secret_file

    SECRET_NAME=$secret_name
    SECRET=$secret
}

function remove_secret() {
    local secret_name="$1"

    run_podman secret rm $secret_name
}

@test "quadlet - basic" {
    local quadlet_file=$PODMAN_TMPDIR/basic_$(random_string).container
    cat > $quadlet_file <<EOF
[Container]
Image=$IMAGE
Exec=sh -c "echo STARTED CONTAINER; echo "READY=1" | socat -u STDIN unix-sendto:\$NOTIFY_SOCKET; sleep inf"
Notify=yes
LogDriver=passthrough
EOF

    run_quadlet "$quadlet_file"
    service_setup $QUADLET_SERVICE_NAME

    # Check that we can read the logs from the container with podman logs even
    # with the `passthrough` driver.  The log may need a short period of time
    # to bubble up into the journal logs, so wait for it.
    wait_for_output "STARTED CONTAINER" $QUADLET_CONTAINER_NAME
    # Make sure it's an *exact* match, not just a substring (i.e. no spurious
    # warnings or other cruft).
    run_podman logs $QUADLET_CONTAINER_NAME
    assert "$output" == "STARTED CONTAINER" "exact/full match when using the 'passthrough' driver"

    # Also look for the logs via `journalctl`.
    run journalctl "--since=$STARTED_TIME" --unit="$QUADLET_SERVICE_NAME"
    is "$output" '.*STARTED CONTAINER.*'

    run_podman container inspect  --format "{{.State.Status}}" $QUADLET_CONTAINER_NAME
    is "$output" "running" "container should be started by systemd and hence be running"

    service_cleanup $QUADLET_SERVICE_NAME failed
}

@test "quadlet conflict names" {
    # If two directories in the search have files with the same name, quadlet should
    # only process the first name
    dir1=$PODMAN_TMPDIR/$(random_string)
    dir2=$PODMAN_TMPDIR/$(random_string)
    local quadlet_file=basic_$(random_string).container
    mkdir -p $dir1 $dir2

    cat > $dir1/$quadlet_file <<EOF
[Container]
Image=$IMAGE
Notify=yes
EOF

    cat > $dir2/$quadlet_file <<EOF
[Container]
Image=$IMAGE
Notify=no
EOF
    QUADLET_UNIT_DIRS="$dir1:$dir2" run \
                    timeout --foreground -v --kill=10 $PODMAN_TIMEOUT \
                    $QUADLET --dryrun
    assert "$output" =~ "Notify=yes" "quadlet should show Notify=yes"
    assert "$output" !~ "Notify=no" "quadlet should not show Notify=no"
}

@test "quadlet - envvar" {
    local quadlet_file=$PODMAN_TMPDIR/envvar_$(random_string).container
    cat > $quadlet_file <<EOF
[Container]
Image=$IMAGE
Exec=sh -c "echo OUTPUT: \"\$FOOBAR\" \"\$BAR\""
Environment="FOOBAR=Foo  Bar" BAR=bar
LogDriver=passthrough
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

# A quadlet container depends on a named quadlet volume
@test "quadlet - named volume dependency" {
    # Save the unit name to use as the volume for the container
    local quadlet_vol_unit=dep_$(random_string).volume
    local quadlet_vol_file=$PODMAN_TMPDIR/${quadlet_vol_unit}
    cat > $quadlet_vol_file <<EOF
[Volume]
VolumeName=foo
EOF

    # Have quadlet create the systemd unit file for the volume unit
    local quadlet_tmpdir=$(mktemp -d --tmpdir=$PODMAN_TMPDIR quadlet.XXXXXX)
    run_quadlet "$quadlet_vol_file" "$quadlet_tmpdir"

    # Save the volume service name since the variable will be overwritten
    local vol_service=$QUADLET_SERVICE_NAME
    local volume_name="foo"

    local quadlet_file=$PODMAN_TMPDIR/user_$(random_string).container
    cat > $quadlet_file <<EOF
[Container]
Image=$IMAGE
Exec=top
Volume=$quadlet_vol_unit:/tmp
EOF

    # Have quadlet create the systemd unit file for the container unit
    run_quadlet "$quadlet_file" "$quadlet_tmpdir"

    # Save the container service name for readability
    local container_service=$QUADLET_SERVICE_NAME
    local container_name=systemd-$(basename $quadlet_file .container)

    # Volume should not exist
    run_podman 1 volume exists ${volume_name}

    # Start the container service which should also trigger the start of the volume service
    service_setup $container_service

    # Volume system unit should be active
    run systemctl show --property=ActiveState "$vol_service"
    assert "$output" = "ActiveState=active" "volume should be active via dependency"

    # Volume should exist
    run_podman volume exists ${volume_name}

    # Container should be attached to defined volume
    run_podman container inspect --format "{{(index .Mounts 0).Name}}" $container_name
    assert "$output" = "$volume_name" "container should be attached to network $volume_name"

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

# A quadlet container depends on a named quadlet network
@test "quadlet - named network dependency" {
    # Save the unit name to use as the network for the container
    local quadlet_network_unit=dep_$(random_string).network
    local quadlet_network_file=$PODMAN_TMPDIR/${quadlet_network_unit}
    cat > $quadlet_network_file <<EOF
[Network]
NetworkName=foo
EOF

    # Have quadlet create the systemd unit file for the network unit
    local quadlet_tmpdir=$(mktemp -d --tmpdir=$PODMAN_TMPDIR quadlet.XXXXXX)
    run_quadlet "$quadlet_network_file" "$quadlet_tmpdir"

    # Save the network service name since the variable will be overwritten
    local network_service=$QUADLET_SERVICE_NAME
    local network_name="foo"

    local quadlet_file=$PODMAN_TMPDIR/user_$(random_string).container
    cat > $quadlet_file <<EOF
[Container]
Image=$IMAGE
Exec=top
Network=$quadlet_network_unit
EOF

    run_quadlet "$quadlet_file" "$quadlet_tmpdir"

    # Save the container service name for readability
    local container_service=$QUADLET_SERVICE_NAME
    local container_name=systemd-$(basename $quadlet_file .container)

    # Network should not exist
    run_podman 1 network exists $network_name

    service_setup $container_service

    # Network system unit should be active
    run systemctl show --property=ActiveState "$network_service"
    assert "$output" = "ActiveState=active" "network should be active via dependency"

    # Network should exist
    run_podman network exists $network_name

    # Container should be attached to defined network
    run_podman container inspect --format "{{index .NetworkSettings.Networks \"$network_name\"}}" $container_name
    assert "$output" != "<nil>" "container should be attached to network $network_name"

    service_cleanup $QUADLET_SERVICE_NAME failed
    run_podman network rm $network_name
}

@test "quadlet kube - basic" {
    # Create the YAMl file
    pod_name="test_pod"
    container_name="test"
    yaml_source="$PODMAN_TMPDIR/basic_$(random_string).yaml"
    cat >$yaml_source <<EOF
apiVersion: v1
kind: Pod
metadata:
  labels:
    app: test
  name: $pod_name
spec:
  containers:
  - command:
    - "sh"
    args:
    - "-c"
    - "echo STARTED CONTAINER; top -b"
    image: $IMAGE
    name: $container_name
EOF

    # Create the Quadlet file
    local quadlet_file=$PODMAN_TMPDIR/basic_$(random_string).kube
    cat > $quadlet_file <<EOF
[Kube]
Yaml=${yaml_source}
EOF

    run_quadlet "$quadlet_file"
    service_setup $QUADLET_SERVICE_NAME

    # Ensure we have output.
    wait_for_output "STARTED CONTAINER" $pod_name-$container_name

    run_podman container inspect  --format "{{.State.Status}}" test_pod-test
    is "$output" "running" "container should be started by systemd and hence be running"

    service_cleanup $QUADLET_SERVICE_NAME inactive
    run_podman rmi $(pause_image)
}

@test "quadlet kube - named network dependency" {
    # Save the unit name to use as the network for the container
    local quadlet_network_unit=dep_$(random_string).network
    local quadlet_network_file=$PODMAN_TMPDIR/${quadlet_network_unit}
    cat > $quadlet_network_file <<EOF
[Network]
NetworkName=foo
EOF

    # Have quadlet create the systemd unit file for the network unit
    local quadlet_tmpdir=$(mktemp -d --tmpdir=$PODMAN_TMPDIR quadlet.XXXXXX)
    run_quadlet "$quadlet_network_file" "$quadlet_tmpdir"

    # Save the network service name since the variable will be overwritten
    local network_service=$QUADLET_SERVICE_NAME
    local network_name="foo"

    # Create the YAMl file
    pod_name="test_pod"
    container_name="test"
    yaml_source="$PODMAN_TMPDIR/basic_$(random_string).yaml"
    cat >$yaml_source <<EOF
apiVersion: v1
kind: Pod
metadata:
  labels:
    app: test
  name: $pod_name
spec:
  containers:
  - command:
    - "sh"
    args:
    - "-c"
    - "echo STARTED CONTAINER; top -b"
    image: $IMAGE
    name: $container_name
EOF

    # Create the Quadlet file
    local quadlet_file=$PODMAN_TMPDIR/basic_$(random_string).kube
    cat > $quadlet_file <<EOF
[Kube]
Yaml=${yaml_source}
Network=$quadlet_network_unit
EOF

    # Network should not exist
    run_podman 1 network exists $network_name

    run_quadlet "$quadlet_file" "$quadlet_tmpdir"
    service_setup $QUADLET_SERVICE_NAME

    # Network system unit should be active
    run systemctl show --property=ActiveState "$network_service"
    assert "$output" = "ActiveState=active" "network should be active via dependency"

    # Network should exist
    run_podman network exists $network_name

    # Ensure we have output.
    wait_for_output "STARTED CONTAINER" $pod_name-$container_name

    run_podman container inspect  --format "{{.State.Status}}" test_pod-test
    assert "$output" =~ "running" "container should be started by systemd and hence be running"

    # Container should be attached to defined network
    run_podman container inspect --format "{{index .NetworkSettings.Networks \"$network_name\"}}" test_pod-test
    assert "$output" != "<nil>" "container should be attached to network $network_name"

    service_cleanup $QUADLET_SERVICE_NAME inactive
    run_podman network rm $network_name
    run_podman rmi $(pause_image)
}

@test "quadlet - rootfs" {
    skip_if_no_selinux
    skip_if_rootless
    local quadlet_file=$PODMAN_TMPDIR/basic_$(random_string).container
    cat > $quadlet_file <<EOF
[Container]
Rootfs=/:O
Exec=sh -c "echo STARTED CONTAINER; echo "READY=1" | socat -u STDIN unix-sendto:\$NOTIFY_SOCKET; top -b"
Notify=yes
EOF

    run_quadlet "$quadlet_file"
    service_setup $QUADLET_SERVICE_NAME

    wait_for_output "STARTED CONTAINER" $QUADLET_CONTAINER_NAME
}

@test "quadlet - selinux disable" {
    skip_if_no_selinux
    local quadlet_file=$PODMAN_TMPDIR/basic_$(random_string).container
    cat > $quadlet_file <<EOF
[Container]
Image=$IMAGE
SecurityLabelDisable=true
Exec=sh -c "echo STARTED CONTAINER; top -b"
EOF

    run_quadlet "$quadlet_file"
    service_setup $QUADLET_SERVICE_NAME

    # Ensure we have output. Output is synced via sd-notify (socat in Exec)
    wait_for_output "STARTED CONTAINER" $QUADLET_CONTAINER_NAME

    run_podman container inspect  --format "{{.ProcessLabel}}" $QUADLET_CONTAINER_NAME
    is "$output" "" "container should be started without specifying a Process Label"

    service_cleanup $QUADLET_SERVICE_NAME failed
}

@test "quadlet - selinux labels" {
    skip_if_no_selinux
    NAME=$(random_string)
    local quadlet_file=$PODMAN_TMPDIR/basic_$(random_string).container
    cat > $quadlet_file <<EOF
[Container]
ContainerName=$NAME
Image=$IMAGE
SecurityLabelType=spc_t
SecurityLabelLevel=s0:c100,c200
SecurityLabelFileType=container_ro_file_t
Exec=sh -c "echo STARTED CONTAINER; top -b"
EOF

    run_quadlet "$quadlet_file"
    service_setup $QUADLET_SERVICE_NAME

    # Ensure we have output. Output is synced via sd-notify (socat in Exec)
    wait_for_output "STARTED CONTAINER" $NAME

    run_podman container ps
    run_podman container inspect  --format "{{.ProcessLabel}}" $NAME
    is "$output" "system_u:system_r:spc_t:s0:c100,c200" "container should be started with correct Process Label"
    run_podman container inspect  --format "{{.MountLabel}}" $NAME
    is "$output" "system_u:object_r:container_ro_file_t:s0:c100,c200" "container should be started with correct Mount Label"

    service_cleanup $QUADLET_SERVICE_NAME failed
}

@test "quadlet - secret as environment variable" {
    create_secret

    local quadlet_file=$PODMAN_TMPDIR/basic_$(random_string).container
    cat > $quadlet_file <<EOF
[Container]
ContainerName=$NAME
Image=$IMAGE
Secret=$SECRET_NAME,type=env,target=MYSECRET
Exec=sh -c "echo STARTED CONTAINER; echo "READY=1" | socat -u STDIN unix-sendto:\$NOTIFY_SOCKET; top -b"
Notify=yes
EOF

    run_quadlet "$quadlet_file"
    service_setup $QUADLET_SERVICE_NAME

    # Ensure we have output. Output is synced via sd-notify (socat in Exec)
    wait_for_output "STARTED CONTAINER" $QUADLET_CONTAINER_NAME

    run_podman exec $QUADLET_CONTAINER_NAME /bin/sh -c "printenv MYSECRET"
    is "$output" $SECRET

    service_cleanup $QUADLET_SERVICE_NAME failed
    remove_secret $SECRET_NAME
}

@test "quadlet - secret as a file" {
    create_secret

    local quadlet_file=$PODMAN_TMPDIR/basic_$(random_string).container
    cat > $quadlet_file <<EOF
[Container]
ContainerName=$NAME
Image=$IMAGE
Secret=$SECRET_NAME,type=mount,target=/root/secret
Exec=sh -c "echo STARTED CONTAINER; echo "READY=1" | socat -u STDIN unix-sendto:\$NOTIFY_SOCKET; top -b"
Notify=yes
EOF

    run_quadlet "$quadlet_file"
    service_setup $QUADLET_SERVICE_NAME

    # Ensure we have output. Output is synced via sd-notify (socat in Exec)
    wait_for_output "STARTED CONTAINER" $QUADLET_CONTAINER_NAME

    run_podman exec $QUADLET_CONTAINER_NAME /bin/sh -c "cat /root/secret"
    is "$output" $SECRET

    service_cleanup $QUADLET_SERVICE_NAME failed
    remove_secret $SECRET_NAME
}

@test "quadlet - volume path using systemd %T specifier" {
    # "specifier" is systemd-speak for "replaceable fields"; see systemd.unit(5)
    #
    # Step 1: determine what systemd is using for %T. There does not
    # seem to be any systemctly way to find this.
    percent_t_file="${PODMAN_TMPDIR}/foo"
    local service=get-percent-t.$(random_string 10).service
    local unitfile=${UNIT_DIR}/$service
    cat >$unitfile <<EOF
[Unit]
Description=Get the value of percent T

[Service]
ExecStart=/bin/bash -c "echo %T >$percent_t_file"
Type=oneshot
EOF
    systemctl daemon-reload
    systemctl --wait start $service
    percent_t=$(< $percent_t_file)
    # Clean up. Don't bother to systemctl-reload, service_setup does that below.
    rm -f $unitfile

    # Sanity check: just make sure it's not "/"
    assert "${#percent_t}" -ge 4 "sanity check: length of %T ($percent_t)"

    # Step 2: Make a subdirectory in %T, and in there, a scratch file
    local tmp_path=$(mktemp -d --tmpdir=${percent_t} quadlet.volume.XXXXXX)
    local tmp_subdir=$(basename $tmp_path)
    local file_name="f$(random_string 10).txt"
    local file_content="data_$(random_string 15)"
    echo $file_content > $tmp_path/$file_name

    local quadlet_file=$PODMAN_TMPDIR/basic_$(random_string).container
    cat > $quadlet_file <<EOF
[Container]
Image=$IMAGE
Volume=%T/$tmp_subdir:/test_content:Z
Exec=sh -c "echo STARTED CONTAINER; echo "READY=1" | socat -u STDIN unix-sendto:\$NOTIFY_SOCKET; top"
Notify=yes
EOF

    run_quadlet "$quadlet_file"
    service_setup $QUADLET_SERVICE_NAME

    run_podman exec $QUADLET_CONTAINER_NAME cat /test_content/$file_name
    is "$output" "$file_content" "contents of testfile in container volume"

    rm -rf $tmp_path
}

@test "quadlet - tmpfs" {
    local quadlet_file=$PODMAN_TMPDIR/basic_$(random_string).container
    cat > $quadlet_file <<EOF
[Container]
Image=$IMAGE
Exec=top
Tmpfs=/tmpfs1
Tmpfs=/tmpfs2:ro
EOF

    run_quadlet "$quadlet_file"
    service_setup $QUADLET_SERVICE_NAME

    run_podman container inspect  --format '{{index .HostConfig.Tmpfs "/tmpfs1"}}' $QUADLET_CONTAINER_NAME
    is "$output" "rw,rprivate,nosuid,nodev,tmpcopyup" "regular tmpfs mount"

    run_podman container inspect  --format '{{index .HostConfig.Tmpfs "/tmpfs2"}}' $QUADLET_CONTAINER_NAME
    is "$output" "ro,rprivate,nosuid,nodev,tmpcopyup" "read-only tmpfs mount"

    run_podman container inspect  --format '{{index .HostConfig.Tmpfs "/tmpfs3"}}' $QUADLET_CONTAINER_NAME
    is "$output" "" "nonexistent tmpfs mount"

    service_cleanup $QUADLET_SERVICE_NAME failed
}

@test "quadlet - userns" {
    local quadlet_file=$PODMAN_TMPDIR/basic_$(random_string).container
    cat > $quadlet_file <<EOF
[Container]
Image=$IMAGE
Exec=top
UserNS=keep-id:uid=200,gid=210
EOF

    run_quadlet "$quadlet_file"
    service_setup $QUADLET_SERVICE_NAME

    run_podman container inspect --format '{{.Config.CreateCommand}}' $QUADLET_CONTAINER_NAME
    is "${output/* --userns keep-id:uid=200,gid=210 */found}" "found"

    service_cleanup $QUADLET_SERVICE_NAME failed
}

@test "quadlet - exit-code propagation" {
   pod_name="test_pod"
   container_name="ctr"
   exit_tests="
all  | true  | 0   | inactive
all  | false | 137 | failed
none | false | 0   | inactive
"
   while read exit_code_prop cmd exit_code service_state; do
      local basename=propagate-${exit_code_prop}-${cmd}-$(random_string)
      local quadlet_file=$PODMAN_TMPDIR/$basename.kube
      local yaml_file=$PODMAN_TMPDIR/$basename.yaml

      cat > $yaml_file <<EOF
apiVersion: v1
kind: Pod
metadata:
  labels:
    app: test
  name: $pod_name
spec:
  restartPolicy: Never
  containers:
    - name: $container_name
      image: $IMAGE
      command:
      - $cmd
EOF
       cat > $quadlet_file <<EOF
[Kube]
Yaml=$yaml_file
ExitCodePropagation=$exit_code_prop
LogDriver=journald
EOF

      run_quadlet "$quadlet_file"
      run systemctl status $QUADLET_SERVICE_NAME

      yaml_sha=$(sha256sum $yaml_file)
      service_container="${yaml_sha:0:12}-service"

      service_setup $QUADLET_SERVICE_NAME

      # Ensure we have output. Output is synced via sd-notify (socat in Exec)
      run journalctl "--since=$STARTED_TIME" --unit="$QUADLET_SERVICE_NAME"
      is "$output" '.*Started.*\.service.*'

      # Opportunistic test: confirm that the Propagation field got set.
      # This is racy, because the container is short-lived and quadlet
      # cleans up on exit (via kube-down in ExecStopPost). So we use '?'
      # and only check output if the inspect succeeds.
      run_podman '?' container inspect --format '{{.KubeExitCodePropagation}}' $service_container
      if [[ $status -eq 0 ]]; then
          is "$output" "$exit_code_prop" \
             "$basename: service container has the expected policy set in its annotations"
      else
          assert "$output" =~ "no such container $service_container" \
                 "$basename: unexpected error from podman container inspect"
      fi

      # Container must stop of its own accord before we call service_cleanup(),
      # otherwise the 'systemctl stop' there may affect the unit's status.
      # Again, use '?' to handle the abovementioned race condition.
      run_podman '?' wait $service_container
      if [[ $status -eq 0 ]]; then
          assert "$output" = "$exit_code" \
                 "$basename: service container reflects expected exit code"
      else
          assert "$output" =~ "no container with name or ID" \
                 "$basename: unexpected error from podman wait"
      fi

      # This is the actual propagation check
      service_cleanup $QUADLET_SERVICE_NAME $service_state
      run_podman ps -aq
      is "$output" "" "all containers are cleaned up even in case of errors"
   done < <(parse_table "$exit_tests")

   run_podman rmi $(pause_image)
}

@test "quadlet kube - Working Directory" {
    yaml_source="$PODMAN_TMPDIR/basic_$(random_string).yaml"
    local_path=local_path$(random_string)
    pod_name=test_pod
    container_name=test

    cat >$yaml_source <<EOF
apiVersion: v1
kind: Pod
metadata:
  labels:
    app: test
  name: $pod_name
spec:
  containers:
  - command:
    - "sh"
    args:
    - "-c"
    - "echo STARTED CONTAINER; top -b"
    image: $IMAGE
    name: $container_name
    volumeMounts:
    - mountPath: /test
      name: test-volume
  volumes:
  - name: test-volume
    hostPath:
      # directory location on host
      path: ./$local_path
      # this field is optional
      type: DirectoryOrCreate
EOF

    # Create the Quadlet file
    local quadlet_file=$PODMAN_TMPDIR/basic_$(random_string).kube
    cat > $quadlet_file <<EOF
[Kube]
Yaml=${yaml_source}
SetWorkingDirectory=yaml
EOF

    run_quadlet "$quadlet_file"
    service_setup $QUADLET_SERVICE_NAME

    # Ensure we have output.
    wait_for_output "STARTED CONTAINER" $pod_name-$container_name

    run_podman container inspect  --format "{{.State.Status}}" $pod_name-$container_name
    is "$output" "running" "container should be started by systemd and hence be running"

    run_podman ps

    run_podman exec $pod_name-$container_name /bin/sh -c "echo hello > /test/test.txt"
    is $(cat $PODMAN_TMPDIR/$local_path/test.txt) "hello"

    service_cleanup $QUADLET_SERVICE_NAME inactive
    run_podman rmi $(pause_image)
}

@test "quadlet - image files" {
    local quadlet_tmpdir=$PODMAN_TMPDIR/quadlets

    local registry=localhost:${PODMAN_LOGIN_REGISTRY_PORT}
    local image_for_test=$registry/quadlet_image_test:$(random_string)
    local authfile=$PODMAN_TMPDIR/authfile.json

    local quadlet_image_unit=image_test_$(random_string).image
    local quadlet_image_file=$PODMAN_TMPDIR/$quadlet_image_unit
    cat > $quadlet_image_file <<EOF
[Image]
Image=$image_for_test
AuthFile=$authfile
TLSVerify=false
EOF

    local quadlet_volume_unit=image_test_$(random_string).volume
    local quadlet_volume_file=$PODMAN_TMPDIR/$quadlet_volume_unit
    local volume_name=systemd-$(basename $quadlet_volume_file .volume)
    cat > $quadlet_volume_file <<EOF
[Volume]
Driver=image
Image=$quadlet_image_unit
EOF

    local quadlet_container_unit=image_test_$(random_string).container
    local quadlet_container_file=$PODMAN_TMPDIR/$quadlet_container_unit
    cat > $quadlet_container_file <<EOF
[Container]
Image=$quadlet_image_unit
Volume=$quadlet_volume_unit:/vol
Exec=sh -c "echo STARTED CONTAINER; echo "READY=1" | socat -u STDIN unix-sendto:\$NOTIFY_SOCKET; sleep inf"
EOF

    # In order to test image pull but without possible Network issues,
    # this test uses an additional registry.
    # Start the registry and populate the authfile that we can use for the test.
    start_registry
    run_podman login --authfile=$authfile \
        --tls-verify=false \
        --username ${PODMAN_LOGIN_USER} \
        --password ${PODMAN_LOGIN_PASS} \
        $registry

    # Push the test image to the registry
    run_podman image tag $IMAGE $image_for_test
    run_podman image push --tls-verify=false --authfile=$authfile $image_for_test

    # Remove the local image to make sure it will be pulled again
    run_podman image rm --ignore $image_for_test

    # Use the same directory for all quadlet files to make sure later steps access previous ones
    mkdir $quadlet_tmpdir

    # Have quadlet create the systemd unit file for the image unit
    run_quadlet "$quadlet_image_file" "$quadlet_tmpdir"
    # Save the image service name since the variable will be overwritten
    local image_service=$QUADLET_SERVICE_NAME

    # Have quadlet create the systemd unit file for the volume unit
    run_quadlet "$quadlet_volume_file" "$quadlet_tmpdir"
    # Save the image service name since the variable will be overwritten
    local volume_service=$QUADLET_SERVICE_NAME

    # Image should not exist
    run_podman 1 image exists ${image_for_test}
    # Volume should not exist
    run_podman 1 volume exists ${volume_name}

    # Have quadlet create the systemd unit file for the image unit
    run_quadlet "$quadlet_container_file" "$quadlet_tmpdir"
    local container_service=$QUADLET_SERVICE_NAME
    local container_name=$QUADLET_CONTAINER_NAME

    service_setup $container_service

    # Image system unit should be active
    run systemctl show --property=ActiveState "$image_service"
    assert "$output" = "ActiveState=active" \
           "quadlet - image files: image should be active via dependency but is not"

    # Volume system unit should be active
    run systemctl show --property=ActiveState "$volume_service"
    assert "$output" = "ActiveState=active" \
           "quadlet - image files: volume should be active via dependency but is not"

    # Image should exist
    run_podman image exists ${image_for_test}

    # Volume should exist
    run_podman volume exists ${volume_name}

    # Verify that the volume was created correctly
    run_podman volume inspect --format "{{ .Driver }}" $volume_name
    assert "$output" = "image" \
           "quadlet - image files: volume driver should be image"

    run_podman volume inspect --format "{{ .Options.image }}" $volume_name
    assert "$output" = "$image_for_test" \
           "quadlet - image files: the image for the volume should be $image_for_test"

    # Verify that the container mounts the volume
    run_podman container inspect --format "{{(index .Mounts 0).Type}}" $container_name
    assert "$output" = "volume" \
           "quadlet - image files: container should be attached to a volume of type volume"

    run_podman container inspect --format "{{(index .Mounts 0).Name}}" $container_name
    assert "$output" = "$volume_name" \
           "quadlet - image files: container should be attached to the volume named $volume_name"

    run_podman exec $QUADLET_CONTAINER_NAME cat /home/podman/testimage-id
    assert "$output" = $PODMAN_TEST_IMAGE_TAG \
           "quadlet - image files: incorrect testimage-id in root"

    run_podman exec $QUADLET_CONTAINER_NAME cat /vol/home/podman/testimage-id
    assert "$output" = $PODMAN_TEST_IMAGE_TAG \
            "quadlet - image files: incorrect testimage-id in bound volume"

    # Shutdown the service and remove the volume
    service_cleanup $container_service failed
    run_podman volume rm $volume_name
    run_podman image rm --ignore $image_for_test
}

@test "quadlet - kube oneshot" {
    local quadlet_tmpdir=$PODMAN_TMPDIR/quadlets
    local test_random_string=$(random_string)

    local quadlet_kube_volume_name=test-volume_$test_random_string
    local quadlet_kube_volume_yaml_file=$PODMAN_TMPDIR/volume_$test_random_string.yaml
    cat > $quadlet_kube_volume_yaml_file <<EOF
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: $quadlet_kube_volume_name
spec:
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
EOF

    local quadlet_kube_volume_unit_file=$PODMAN_TMPDIR/volume_$test_random_string.kube
    cat > $quadlet_kube_volume_unit_file <<EOF
[Kube]
Yaml=$quadlet_kube_volume_yaml_file

[Service]
Type=oneshot
RemainAfterExit=yes
EOF

    local pod_name="test_pod_$test_random_string"
    local container_name="test"
    local quadlet_kube_pod_yaml_file=$PODMAN_TMPDIR/pod_$test_random_string.yaml
    cat > $quadlet_kube_pod_yaml_file <<EOF
apiVersion: v1
kind: Pod
metadata:
  labels:
    app: test
  name: $pod_name
spec:
  containers:
  - command:
    - "sh"
    args:
    - "-c"
    - "echo STARTED CONTAINER; top -b"
    image: $IMAGE
    name: $container_name
    volumeMounts:
    - name: storage
      mountPath: /mnt/storage
  volumes:
  - name: storage
    persistentVolumeClaim:
      claimName: $quadlet_kube_volume_name
EOF

    # Use the same directory for all quadlet files to make sure later steps access previous ones
    mkdir $quadlet_tmpdir

    # Have quadlet create the systemd unit file for the kube based volume unit
    run_quadlet "$quadlet_kube_volume_unit_file" "$quadlet_tmpdir"
    # Save the volume service name since the variable will be overwritten
    local volume_service=$QUADLET_SERVICE_NAME

    # Volume should not exist
    run_podman 1 volume exists ${quadlet_kube_volume_name}

    local quadlet_kube_pod_unit_file=$PODMAN_TMPDIR/pod_$test_random_string.kube
    cat > $quadlet_kube_pod_unit_file <<EOF
[Kube]
Yaml=$quadlet_kube_pod_yaml_file

[Unit]
Requires=$volume_service
After=$volume_service
EOF

    # Have quadlet create the systemd unit file for the pod unit
    run_quadlet "$quadlet_kube_pod_unit_file" "$quadlet_tmpdir"
    local pod_service=$QUADLET_SERVICE_NAME

    service_setup $pod_service

    # Volume system unit should be active
    run systemctl show --property=ActiveState "$volume_service"
    assert "$output" = "ActiveState=active" \
           "quadlet - kube oneshot: volume should be active via dependency but is not"

    # Volume should exist
    run_podman volume exists ${quadlet_kube_volume_name}

    run_podman container inspect --format "{{(index .Mounts 0).Type}}" $pod_name-$container_name
    assert "$output" = "volume" \
           "quadlet - kube oneshot: volume .Type"

    run_podman container inspect --format "{{(index .Mounts 0).Name}}" $pod_name-$container_name
    assert "$output" = "$quadlet_kube_volume_name" \
           "quadlet - kube oneshot: volume .Name"

    # Shutdown the service and remove the volume
    service_cleanup $pod_service inactive
    run_podman volume rm $quadlet_kube_volume_name
    run_podman rmi --ignore $(pause_image)
}

@test "quadlet - kube down force" {
    local test_random_string=$(random_string)

    local quadlet_kube_volume_name=test-volume_$test_random_string
    local pod_name="test_pod_$test_random_string"
    local container_name="test"
    local quadlet_kube_pod_yaml_file=$PODMAN_TMPDIR/pod_$test_random_string.yaml
    cat > $quadlet_kube_pod_yaml_file <<EOF
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: $quadlet_kube_volume_name
spec:
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
---
apiVersion: v1
kind: Pod
metadata:
  labels:
    app: test
  name: $pod_name
spec:
  containers:
  - command:
    - "sh"
    args:
    - "-c"
    - "echo STARTED CONTAINER; top -b"
    image: $IMAGE
    name: $container_name
    volumeMounts:
    - name: storage
      mountPath: /mnt/storage
  volumes:
  - name: storage
    persistentVolumeClaim:
      claimName: $quadlet_kube_volume_name
EOF

    local quadlet_kube_pod_unit_file=$PODMAN_TMPDIR/pod_$test_random_string.kube
    cat > $quadlet_kube_pod_unit_file <<EOF
[Kube]
Yaml=$quadlet_kube_pod_yaml_file
KubeDownForce=true
EOF

    # Have quadlet create the systemd unit file for the pod unit
    run_quadlet "$quadlet_kube_pod_unit_file" "$quadlet_tmpdir"
    local pod_service=$QUADLET_SERVICE_NAME

    # Volume should not exist
    run_podman 1 volume exists ${quadlet_kube_volume_name}

    service_setup $pod_service

    # Volume should exist
    run_podman volume exists ${quadlet_kube_volume_name}

    run_podman container inspect --format "{{(index .Mounts 0).Type}}" $pod_name-$container_name
    assert "$output" = "volume" \
           "quadlet - kube oneshot: volume .Type"

    run_podman container inspect --format "{{(index .Mounts 0).Name}}" $pod_name-$container_name
    assert "$output" = "$quadlet_kube_volume_name" \
           "quadlet - kube oneshot: volume .Name"

    # Shutdown the service
    service_cleanup $pod_service failed

    # Volume should not exist
    run_podman 1 volume exists ${quadlet_kube_volume_name}
    run_podman rmi --ignore $(pause_image)
}

@test "quadlet - image tag" {
    local quadlet_tmpdir=$PODMAN_TMPDIR/quadlets
    local archive_file=$PODMAN_TMPDIR/archive-file.tar
    local image_for_test=localhost/quadlet_image_test:$(random_string)

    local quadlet_image_unit=image_test_$(random_string).image
    local quadlet_image_file=$PODMAN_TMPDIR/$quadlet_image_unit
    cat > $quadlet_image_file <<EOF
[Image]
Image=docker-archive:$archive_file
ImageTag=$image_for_test
EOF

    local quadlet_volume_unit=image_test_$(random_string).volume
    local quadlet_volume_file=$PODMAN_TMPDIR/$quadlet_volume_unit
    local volume_name=systemd-$(basename $quadlet_volume_file .volume)
    cat > $quadlet_volume_file <<EOF
[Volume]
Driver=image
Image=$quadlet_image_unit
EOF

    local quadlet_container_unit=image_test_$(random_string).container
    local quadlet_container_file=$PODMAN_TMPDIR/$quadlet_container_unit
    cat > $quadlet_container_file <<EOF
[Container]
Image=$IMAGE
Exec=sh -c "echo STARTED CONTAINER; echo "READY=1" | socat -u STDIN unix-sendto:\$NOTIFY_SOCKET; sleep inf"
Volume=$quadlet_volume_unit:/vol
EOF

    # Tag the image, save it into a file and remove it
    run_podman image tag $IMAGE $image_for_test
    run_podman image save --format docker-archive --output $archive_file $image_for_test
    run_podman image rm $image_for_test

    # Use the same directory for all quadlet files to make sure later steps access previous ones
    mkdir $quadlet_tmpdir

    # Have quadlet create the systemd unit file for the image unit
    run_quadlet "$quadlet_image_file" "$quadlet_tmpdir"
    # Save the image service name since the variable will be overwritten
    local image_service=$QUADLET_SERVICE_NAME

    # Have quadlet create the systemd unit file for the volume unit
    run_quadlet "$quadlet_volume_file" "$quadlet_tmpdir"
    # Save the image service name since the variable will be overwritten
    local volume_service=$QUADLET_SERVICE_NAME

    # Image should not exist
    run_podman 1 image exists ${image_for_test}
    # Volume should not exist
    run_podman 1 volume exists ${volume_name}

    # Have quadlet create the systemd unit file for the image unit
    run_quadlet "$quadlet_container_file" "$quadlet_tmpdir"
    local container_service=$QUADLET_SERVICE_NAME
    local container_name=$QUADLET_CONTAINER_NAME

    service_setup $container_service

    # Image system unit should be active
    run systemctl show --property=ActiveState "$image_service"
    assert "$output" = "ActiveState=active" \
           "quadlet - image tag: image service ActiveState"

    # Volume system unit should be active
    run systemctl show --property=ActiveState "$volume_service"
    assert "$output" = "ActiveState=active" \
           "quadlet - image tag: volume service ActiveState"

    # Image should exist
    run_podman image exists ${image_for_test}

    # Volume should exist
    run_podman volume exists ${volume_name}

    run_podman exec $QUADLET_CONTAINER_NAME cat /vol/home/podman/testimage-id
    assert "$output" = $PODMAN_TEST_IMAGE_TAG \
            "quadlet - image files: incorrect testimage-id in bound volume"

    # Shutdown the service and remove the image
    service_cleanup $container_service failed
    run_podman image rm --ignore $image_for_test
    run_podman rmi --ignore $(pause_image)
}

@test "quadlet - pod simple" {
    local quadlet_tmpdir=$PODMAN_TMPDIR/quadlets

    local test_pod_name=pod_test_$(random_string)
    local quadlet_pod_unit=$test_pod_name.pod
    local quadlet_pod_file=$PODMAN_TMPDIR/$quadlet_pod_unit
    cat > $quadlet_pod_file <<EOF
[Pod]
PodName=$test_pod_name
EOF

    local quadlet_container_unit=pod_test_$(random_string).container
    local quadlet_container_file=$PODMAN_TMPDIR/$quadlet_container_unit
    cat > $quadlet_container_file <<EOF
[Container]
Image=$IMAGE
Exec=sh -c "echo STARTED CONTAINER; echo "READY=1" | socat -u STDIN unix-sendto:\$NOTIFY_SOCKET; sleep inf"
Pod=$quadlet_pod_unit
EOF

    # Use the same directory for all quadlet files to make sure later steps access previous ones
    mkdir $quadlet_tmpdir

    # Have quadlet create the systemd unit file for the pod unit
    run_quadlet "$quadlet_pod_file" "$quadlet_tmpdir"
    # Save the pod service name since the variable will be overwritten
    local pod_service=$QUADLET_SERVICE_NAME

    # Have quadlet create the systemd unit file for the container unit
    run_quadlet "$quadlet_container_file" "$quadlet_tmpdir"
    local container_service=$QUADLET_SERVICE_NAME
    local container_name=$QUADLET_CONTAINER_NAME

    # Start the pod service
    service_setup $pod_service

    # Pod should exist
    run_podman pod exists ${test_pod_name}

    # Wait for systemd to activate the container service
    wait_for_command_output "systemctl show --property=ActiveState $container_service" "ActiveState=active"

    # Container should exist
    run_podman container exists ${container_name}

    # Shutdown the service
    service_cleanup $pod_service inactive

    # The service of the container should be active
    run systemctl show --property=ActiveState "$container_service"
    assert "ActiveState=failed" \
           "quadlet - pod base: container service ActiveState"

    # Container should not exist
    run_podman 1 container exists ${container_name}

    run_podman rmi $(pause_image)
}
# vim: filetype=sh
