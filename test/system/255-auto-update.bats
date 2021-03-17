#!/usr/bin/env bats   -*- bats -*-
#
# Tests for automatically update images for containerized services
#

load helpers

UNIT_DIR="/usr/lib/systemd/system"
SNAME_FILE=$BATS_TMPDIR/services

function setup() {
    skip_if_remote "systemd tests are meaningless over remote"
    skip_if_rootless

    basic_setup
}

function teardown() {
    while read line; do
        if [[ "$line" =~ "podman-auto-update" ]]; then
            echo "Stop timer: $line.timer"
            systemctl stop $line.timer
            systemctl disable $line.timer
        else
            systemctl stop $line
        fi
        rm -f $UNIT_DIR/$line.{service,timer}
    done < $SNAME_FILE

    rm -f $SNAME_FILE
    run_podman ? rmi quay.io/libpod/alpine:latest
    run_podman ? rmi quay.io/libpod/alpine_nginx:latest
    run_podman ? rmi quay.io/libpod/localtest:latest
    basic_teardown
}

# This functions is used for handle the basic step in auto-update related
# tests. Including following steps:
#   1. Generate a random container name and echo it to output.
#   2. Tag the fake image before test
#   3. Start a container with io.containers.autoupdate
#   4. Generate the service file from the container
#   5. Remove the origin container
#   6. Start the container from service
function generate_service() {
    target_img_basename=$1
    autoupdate=$2

    # Please keep variable name for cname and ori_image. The
    # scripts will use them directly in following tests.
    cname=c_$(random_string)
    target_img="quay.io/libpod/$target_img_basename:latest"
    run_podman tag $IMAGE $target_img
    if [[ -n "$autoupdate" ]]; then
        label="--label io.containers.autoupdate=$autoupdate"
    else
        label=""
    fi
    run_podman run -d --name $cname $label $target_img top -d 120

    run_podman generate systemd --new $cname
    echo "$output" > "$UNIT_DIR/container-$cname.service"
    echo "container-$cname" >> $SNAME_FILE
    run_podman rm -f $cname

    systemctl daemon-reload
    systemctl start container-$cname
    systemctl status container-$cname

    run_podman inspect --format "{{.Image}}" $cname
    ori_image=$output
}

function _wait_service_ready() {
    local sname=$1

    local timeout=6
    while [[ $timeout -gt 1 ]]; do
        run systemctl is-active $sname
        if [[ $output == "active" ]]; then
            return
        fi
        sleep 1
        let timeout=$timeout-1
    done

    # Print serivce status as debug information before failed the case
    systemctl status $sname
    die "Timed out waiting for $sname to start"
}

function _confirm_update() {
    local sname=$1

    local timeout=6
    last_log=""
    while [[ $timeout -gt 1 ]]; do
        run journalctl -u $sname -n 10
        if [[ "$output" == "$last_log" ]]; then
            return
        fi
        last_log=$output
        sleep 1
        let timeout=$timeout-1
    done

    die "Timed out waiting for $sname to update"
}

# This test can fail in dev. environment because of SELinux.
# quick fix: chcon -t container_runtime_exec_t ./bin/podman
@test "podman auto-update - label io.containers.autoupdate=image" {
    run_podman images
    generate_service alpine image

    _wait_service_ready container-$cname.service
    run_podman ps -a
    run_podman auto-update
    is "$output" "Trying to pull.*" "Image is updated."
    run_podman ps -a
    _confirm_update container-$cname.service
    run_podman inspect --format "{{.Image}}" $cname
    [[ "$output" != "$ori_image" ]]
}

@test "podman auto-update - label io.containers.autoupdate=disabled" {
    generate_service alpine disabled

    _wait_service_ready container-$cname.service
    run_podman ps -a
    run_podman auto-update
    is "$output" "" "Image is not updated with disabled."
    run_podman ps -a
    _confirm_update container-$cname.service
    run_podman inspect --format "{{.Image}}" $cname
    is "$output" "$ori_image" "Image hash should not changed."
}

@test "podman auto-update - label io.containers.autoupdate=fakevalue" {
    fakevalue=$(random_string)
    generate_service alpine $fakevalue

    _wait_service_ready container-$cname.service
    run_podman ps -a
    run_podman ? auto-update
    is "$output" ".*invalid auto-update policy.*" "invalid policy setup"
    run_podman ps -a
    _confirm_update container-$cname.service
    run_podman inspect --format "{{.Image}}" $cname
    is "$output" "$ori_image" "Image hash should not changed."
}

@test "podman auto-update - label io.containers.autoupdate=local" {
    generate_service localtest local
    podman commit --change CMD=/bin/bash $cname quay.io/libpod/localtest:latest

    _wait_service_ready container-$cname.service
    run_podman ps -a
    run_podman auto-update
    run_podman ps -a
    _confirm_update container-$cname.service
    run_podman inspect --format "{{.Image}}" $cname
    [[ "$output" != "$ori_image" ]]
}

@test "podman auto-update with multiple services" {
    fakevalue=$(random_string)
    run_podman inspect --format "{{.Id}}" $IMAGE
    img_id="$output"
    cnames=()
    local -A expect_update
    local -A will_update=([image]=1 [registry]=1 [local]=1)

    for auto_update in image registry "" disabled "''" $fakevalue local
    do
        img_base="alpine"
        if [[ $auto_update == "registry" ]]; then
            img_base="alpine_nginx"
        elif [[ $auto_update == "local" ]]; then
            img_base="localtest"
        fi
        generate_service $img_base $auto_update
        cnames+=($cname)
        if [[ $auto_update == "local" ]]; then
            local_cname=$cname
        fi
        if [[ -n "$auto_update" && -n "${will_update[$auto_update]}" ]]; then
            expect_update[$cname]=1
        fi
    done

    # Only check the last service is started. Previous services should already actived.
    _wait_service_ready container-$cname.service
    run_podman commit --change CMD=/bin/bash $local_cname quay.io/libpod/localtest:latest
    run_podman ? auto-update
    update_log=$output
    for cname in "${cnames[@]}"; do
        _confirm_update container-$cname.service
    done
    count=0
    while read line; do
       if [[ "$line" =~ "Trying to pull" ]]; then
           ((count+=1))
       fi
    done <<< "$update_log"
    is "$update_log" ".*invalid auto-update policy.*" "invalid policy setup"
    is "$update_log" ".*1 error occurred.*" "invalid policy setup"
    is "$count" "2" "There are two images being updated from registry."

    for cname in "${!expect_update[@]}"; do

        is "$update_log" ".*$cname.*" "container with auto-update policy image updated"
    done

    for cname in "${cnames[@]}"; do
        run_podman inspect --format "{{.Image}}" $cname
        if [[ -n "${expect_update[$cname]}" ]]; then
            [[ "$output" != "$img_id" ]]
        else
            is "$output" "$img_id" "Image should not be changed."
        fi
    done
}

@test "podman auto-update using systemd" {
    generate_service alpine image

    cat >$UNIT_DIR/podman-auto-update-$cname.timer <<EOF
[Unit]
Description=Podman auto-update testing timer

[Timer]
OnCalendar=*-*-* *:*:0/2
Persistent=true

[Install]
WantedBy=timers.target
EOF
    cat >$UNIT_DIR/podman-auto-update-$cname.service <<EOF
[Unit]
Description=Podman auto-update testing service
Documentation=man:podman-auto-update(1)
Wants=network.target
After=network-online.target

[Service]
Type=oneshot
ExecStart=/usr/bin/podman auto-update

[Install]
WantedBy=multi-user.target default.target
EOF

    echo "podman-auto-update-$cname" >> $SNAME_FILE
    systemctl enable --now podman-auto-update-$cname.timer
    systemctl list-timers --all

    count=0
    failed_start=1
    while [ $count -lt 120 ]; do
        run journalctl -n 15 -u podman-auto-update-$cname.service
        if [[ "$output" =~ "Finished Podman auto-update testing service" ]]; then
            failed_start=0
            break
        fi
        ((count+=1))
        sleep 1
    done
    echo $output

    _confirm_update container-$cname.service
    run_podman inspect --format "{{.Image}}" $cname
    if [[ $failed_start == 1 ]]; then
       die "Failed to get podman auto-update service finished"
    fi
    [[ "$output" != "$ori_image" ]]
}

# vim: filetype=sh
