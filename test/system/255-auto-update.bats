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
    local target_img_basename=$1
    local autoupdate=$2

    # Container name. Include the autoupdate type, to make debugging easier.
    # IMPORTANT: variable 'cname' is passed (out of scope) up to caller!
    cname=c_${autoupdate//\'/}_$(random_string)
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

    # Original image ID.
    # IMPORTANT: variable 'ori_image' is passed (out of scope) up to caller!
    run_podman inspect --format "{{.Image}}" $cname
    ori_image=$output
}

function _wait_service_ready() {
    local sname=$1

    local timeout=6
    while [[ $timeout -gt 1 ]]; do
        if systemctl -q is-active $sname; then
            return
        fi
        sleep 1
        let timeout=$timeout-1
    done

    # Print serivce status as debug information before failed the case
    systemctl status $sname
    die "Timed out waiting for $sname to start"
}

# Wait for container to update, as confirmed by its image ID changing
function _confirm_update() {
    local cname=$1
    local old_iid=$2

    # Image has already been pulled, so this shouldn't take too long
    local timeout=5
    while [[ $timeout -gt 0 ]]; do
        run_podman '?' inspect --format "{{.Image}}" $cname
        if [[ $status != 0 ]]; then
            if [[ $output =~ (no such object|does not exist in database): ]]; then
                # this is ok, it just means the container is being restarted
                :
            else
                die "podman inspect $cname failed unexpectedly"
            fi
        elif [[ $output != $old_iid ]]; then
            return
        fi
        sleep 1
    done

    die "Timed out waiting for $cname to update; old IID=$old_iid"
}

# This test can fail in dev. environment because of SELinux.
# quick fix: chcon -t container_runtime_exec_t ./bin/podman
@test "podman auto-update - label io.containers.autoupdate=image" {
    generate_service alpine image

    _wait_service_ready container-$cname.service
    run_podman auto-update
    is "$output" "Trying to pull.*" "Image is updated."
    _confirm_update $cname $ori_image
}

@test "podman auto-update - label io.containers.autoupdate=disabled" {
    generate_service alpine disabled

    _wait_service_ready container-$cname.service
    run_podman auto-update
    is "$output" "" "Image is not updated when autoupdate=disabled."

    run_podman inspect --format "{{.Image}}" $cname
    is "$output" "$ori_image" "Image ID should not change"
}

@test "podman auto-update - label io.containers.autoupdate=fakevalue" {
    fakevalue=fake_$(random_string)
    generate_service alpine $fakevalue

    _wait_service_ready container-$cname.service
    run_podman 125 auto-update
    is "$output" ".*invalid auto-update policy.*" "invalid policy setup"

    run_podman inspect --format "{{.Image}}" $cname
    is "$output" "$ori_image" "Image ID should not change"
}

@test "podman auto-update - label io.containers.autoupdate=local" {
    generate_service localtest local
    podman commit --change CMD=/bin/bash $cname quay.io/libpod/localtest:latest

    _wait_service_ready container-$cname.service
    run_podman auto-update
    _confirm_update $cname $ori_image
}

@test "podman auto-update with multiple services" {
    # Preserve original image ID, to confirm that it changes (or not)
    run_podman inspect --format "{{.Id}}" $IMAGE
    local img_id="$output"

    local cnames=()
    local -A expect_update
    local -A will_update=([image]=1 [registry]=1 [local]=1)

    local fakevalue=fake_$(random_string)
    for auto_update in image registry "" disabled "''" $fakevalue local
    do
        local img_base="alpine"
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
    # Exit code is expected, due to invalid 'fakevalue'
    run_podman 125 auto-update
    update_log=$output
    is "$update_log" ".*invalid auto-update policy.*" "invalid policy setup"
    is "$update_log" ".*1 error occurred.*" "invalid policy setup"

    local n_updated=$(grep -c 'Trying to pull' <<<"$update_log")
    is "$n_updated" "2" "Number of images updated from registry."

    for cname in "${!expect_update[@]}"; do
        is "$update_log" ".*$cname.*" "container with auto-update policy image updated"
        # Just because podman says it fetched, doesn't mean it actually updated
        _confirm_update $cname $img_id
    done

    # Final confirmation that all image IDs have/haven't changed
    for cname in "${cnames[@]}"; do
        run_podman inspect --format "{{.Image}}" $cname
        if [[ -n "${expect_update[$cname]}" ]]; then
            if [[ "$output" == "$img_id" ]]; then
                die "$cname: image ID ($output) did not change"
            fi
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

    local expect='Finished Podman auto-update testing service'
    local failed_start=failed
    local count=0
    while [ $count -lt 120 ]; do
        run journalctl -n 15 -u podman-auto-update-$cname.service
        if [[ "$output" =~ $expect ]]; then
            failed_start=
            break
        fi
        ((count+=1))
        sleep 1
    done

    if [[ -n "$failed_start" ]]; then
       die "Did not find expected string '$expect' in journalctl output for $cname"
    fi

    _confirm_update $cname $ori_image
}

# vim: filetype=sh
