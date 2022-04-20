#!/usr/bin/env bats   -*- bats -*-
#
# Tests for automatically update images for containerized services
#

load helpers
load helpers.systemd

SNAME_FILE=$BATS_TMPDIR/services

function setup() {
    skip_if_remote "systemd tests are meaningless over remote"
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
    run_podman ? rmi -f                            \
            quay.io/libpod/alpine:latest           \
            quay.io/libpod/busybox:latest          \
            quay.io/libpod/localtest:latest        \
            quay.io/libpod/autoupdatebroken:latest \
            quay.io/libpod/test:latest             \
            quay.io/libpod/fedora:31

    # The rollback tests may leave some dangling images behind, so let's prune
    # them to leave a clean state.
    run_podman ? image prune -f
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
    local command=$3
    local extraArgs=$4
    local noTag=$5

    # Unless specified, set a default command.
    if [[ -z "$command" ]]; then
        command="top -d 120"
    fi

    # Container name. Include the autoupdate type, to make debugging easier.
    # IMPORTANT: variable 'cname' is passed (out of scope) up to caller!
    cname=c_${autoupdate//\'/}_$(random_string)
    target_img="quay.io/libpod/$target_img_basename:latest"

    if [[ -z "$noTag" ]]; then
        run_podman tag $IMAGE $target_img
    fi

    if [[ -n "$autoupdate" ]]; then
        label="--label io.containers.autoupdate=$autoupdate"
    else
        label=""
    fi
    run_podman create $extraArgs --name $cname $label $target_img $command

    (cd $UNIT_DIR; run_podman generate systemd --new --files --name $cname)
    echo "container-$cname" >> $SNAME_FILE
    run_podman rm -t 0 -f $cname

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

    # Print service status as debug information before failed the case
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
    run_podman auto-update --dry-run --format "{{.Unit}},{{.Image}},{{.Updated}},{{.Policy}}"
    is "$output" ".*container-$cname.service,quay.io/libpod/alpine:latest,pending,registry.*" "Image update is pending."

    run_podman auto-update --format "{{.Unit}},{{.Image}},{{.Updated}},{{.Policy}}"
    is "$output" "Trying to pull.*" "Image is updated."
    is "$output" ".*container-$cname.service,quay.io/libpod/alpine:latest,true,registry.*" "Image is updated."

    _confirm_update $cname $ori_image
}

@test "podman auto-update - label io.containers.autoupdate=image with rollback" {
    # FIXME: this test should exercise the authfile label to have a regression
    # test for #11171.

    # Note: the autoupdatebroken image is empty on purpose so it cannot be
    # executed and force a rollback.  The rollback test for the local policy
    # is exercising the case where the container doesn't send a ready message.
    image=quay.io/libpod/autoupdatebroken

    run_podman tag $IMAGE $image
    generate_service autoupdatebroken image

    _wait_service_ready container-$cname.service
    run_podman auto-update --dry-run --format "{{.Unit}},{{.Image}},{{.Updated}},{{.Policy}}"
    is "$output" ".*container-$cname.service,$image:latest,pending,registry.*" "Image update is pending."

    run_podman container inspect --format "{{.Image}}" $cname
    oldID="$output"

    run_podman inspect --format "{{.ID}}" $cname
    containerID="$output"

    run_podman auto-update --format "{{.Unit}},{{.Image}},{{.Updated}},{{.Policy}}"
    is "$output" "Trying to pull.*" "Image is updated."
    is "$output" ".*container-$cname.service,$image:latest,rolled back,registry.*" "Image has been rolled back."

    run_podman container inspect --format "{{.Image}}" $cname
    is "$output" "$oldID" "container rolled back to previous image"

    run_podman container inspect --format "{{.ID}}" $cname
    assert "$output" != "$containerID" \
           "container has not been restarted during rollback"
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
    image=quay.io/libpod/localtest:latest
    podman commit --change CMD=/bin/bash $cname $image
    podman image inspect --format "{{.ID}}" $image
    imageID="$output"

    _wait_service_ready container-$cname.service
    run_podman auto-update --dry-run --format "{{.Unit}},{{.Image}},{{.Updated}},{{.Policy}}"
    is "$output" ".*container-$cname.service,quay.io/libpod/localtest:latest,pending,local.*" "Image update is pending."

    run_podman auto-update --format "{{.Unit}},{{.Image}},{{.Updated}},{{.Policy}}"
    is "$output" ".*container-$cname.service,quay.io/libpod/localtest:latest,true,local.*" "Image is updated."

    _confirm_update $cname $ori_image
}

@test "podman auto-update - label io.containers.autoupdate=local with rollback" {
    # sdnotify fails with runc 1.0.0-3-dev2 on Ubuntu. Let's just
    # assume that we work only with crun, nothing else.
    # [copied from 260-sdnotify.bats]
    runtime=$(podman_runtime)
    if [[ "$runtime" != "crun" ]]; then
        skip "this test only works with crun, not $runtime"
    fi

    dockerfile1=$PODMAN_TMPDIR/Dockerfile.1
    cat >$dockerfile1 <<EOF
FROM quay.io/libpod/fedora:31
RUN echo -e "#!/bin/sh\n\
printenv NOTIFY_SOCKET; echo READY; systemd-notify --ready;\n\
trap 'echo Received SIGTERM, finishing; exit' SIGTERM; echo WAITING; while :; do sleep 0.1; done" \
>> /runme
RUN chmod +x /runme
EOF

    dockerfile2=$PODMAN_TMPDIR/Dockerfile.2
    cat >$dockerfile2 <<EOF
FROM quay.io/libpod/fedora:31
RUN echo -e "#!/bin/sh\n\
exit 1" >> /runme
RUN chmod +x /runme
EOF
    image=test

    # Generate a healthy image that will run correctly.
    run_podman build -t quay.io/libpod/$image -f $dockerfile1
    podman image inspect --format "{{.ID}}" $image
    oldID="$output"

    generate_service $image local /runme --sdnotify=container noTag
    _wait_service_ready container-$cname.service

    run_podman auto-update --dry-run --format "{{.Unit}},{{.Image}},{{.Updated}},{{.Policy}}"
    is "$output" ".*container-$cname.service,quay.io/libpod/$image:latest,false,local.*" "No update available"

    # Generate an unhealthy image that will fail.
    run_podman build -t quay.io/libpod/$image -f $dockerfile2
    podman image inspect --format "{{.ID}}" $image
    newID="$output"

    run_podman auto-update --dry-run --format "{{.Unit}},{{.Image}},{{.Updated}},{{.Policy}}"
    is "$output" ".*container-$cname.service,quay.io/libpod/$image:latest,pending,local.*" "Image updated is pending"

    # Note: we rollback automatically by default.
    run_podman auto-update --format "{{.Unit}},{{.Image}},{{.Updated}},{{.Policy}}"
    is "$output" ".*container-$cname.service,quay.io/libpod/$image:latest,rolled back,local.*" "Rolled back to old image"

    # Make sure that new container is not using the new image ID anymore.
    _confirm_update $cname $newID
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
            img_base="busybox"
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

    # Only check that the last service is started. Previous services should already be activated.
    _wait_service_ready container-$cname.service
    run_podman commit --change CMD=/bin/bash $local_cname quay.io/libpod/localtest:latest
    # Exit code is expected, due to invalid 'fakevalue'
    run_podman 125 auto-update
    update_log=$output
    is "$update_log" ".*invalid auto-update policy.*" "invalid policy setup"
    is "$update_log" ".*Error: invalid auto-update policy.*" "invalid policy setup"

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
            assert "$output" != "$img_id" "$cname: image ID did not change"
        else
            assert "$output" = "$img_id" "Image ID should not be changed."
        fi
    done
}

@test "podman auto-update using systemd" {
    skip_if_journald_unavailable

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
Wants=network-online.target
After=network-online.target

[Service]
Type=oneshot
ExecStart=/usr/bin/podman auto-update

[Install]
WantedBy=default.target
EOF

    echo "podman-auto-update-$cname" >> $SNAME_FILE
    systemctl enable --now podman-auto-update-$cname.timer
    systemctl list-timers --all

    # systemd       <245 displays 'Started Podman auto-update ...'
    # systemd 245 - <250 displays 'Finished Podman auto-update ...'
    # systemd 250 - ???? displays 'Finished <unit name> - Podman auto-...'
    local expect='(Started|Finished.*) Podman auto-update testing service'
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
        echo "journalctl output:"
        sed -e 's/^/  /' <<<"$output"
        die "Did not find expected string '$expect' in journalctl output for $cname"
    fi

    _confirm_update $cname $ori_image
}

# vim: filetype=sh
