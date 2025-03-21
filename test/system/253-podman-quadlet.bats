#!/usr/bin/env bats   -*- bats -*-
#
# Tests generated configurations for systemd.
#

# bats file_tags=ci:parallel

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
            run systemctl reset-failed "$service"
            rm -f "$UNIT_FILE"
        fi
    done
    systemctl daemon-reload

    basic_teardown
}

@test "quadlet verb - install, list, rm" {
    # Create a test quadlet file
    local quadlet_file=$PODMAN_TMPDIR/alpine-quadlet.container
    cat > $quadlet_file <<EOF
[Container]
Image=$IMAGE
Exec=sh -c "echo STARTED CONTAINER; trap 'exit' SIGTERM; while :; do sleep 0.1; done"
Notify=yes
LogDriver=passthrough
EOF
    # Clean all quadlets
    run_podman quadlet rm --all -f

    # Test quadlet install
    run_podman quadlet install $quadlet_file

    # Test quadlet list
    run_podman quadlet list
    assert "$output" =~ "alpine-quadlet.container" "list should contain alpine-quadlet.container"

    # Test quadlet list with filter
    run_podman quadlet list --filter name=something*
    assert "$output" !~ "alpine-quadlet.container" "filtered list should not contain alpine-quadlet.container"

    # Test quadlet list with matching filter
    run_podman quadlet list --filter name=alpine*
    assert "$output" =~ "alpine-quadlet.container" "matching filter should contain alpine-quadlet.container"

    # Test quadlet print
    run_podman quadlet print alpine-quadlet.container
    assert "$output" =~ "\[Container\]" "print should show container section"
    assert "$output" =~ "Image=$IMAGE" "print should show correct image"

    # Test quadlet rm
    run_podman quadlet rm alpine-quadlet.container

    # Verify removal
    run_podman quadlet list
    assert "$output" !~ "alpine-quadlet.container" "list should not contain removed container"
}

# bats test_tags=distro-integration
@test "quadlet verb - install multiple files from directory" {
    # Create a directory for multiple quadlet files
    local quadlet_dir="$PODMAN_TMPDIR/quadlet-multi-$(safe_name)"
    mkdir -p $quadlet_dir

    # Create multiple quadlet files with different configurations
    cat > $quadlet_dir/alpine1.container <<EOF
[Container]
Image=$IMAGE
Exec=sh -c "echo STARTED CONTAINER 1; trap 'exit' SIGTERM; while :; do sleep 0.1; done"
Notify=yes
LogDriver=passthrough
EOF

    cat > $quadlet_dir/alpine2.container <<EOF
[Container]
Image=$IMAGE
Exec=sh -c "echo STARTED CONTAINER 2; trap 'exit' SIGTERM; while :; do sleep 0.1; done"
Notify=yes
LogDriver=passthrough
EOF

    cat > $quadlet_dir/nginx.container <<EOF
[Container]
Image=quay.io/libpod/nginx:latest
Exec=sh -c "echo STARTED NGINX; trap 'exit' SIGTERM; while :; do sleep 0.1; done"
Notify=yes
LogDriver=passthrough
EOF
    # Clean all quadlets
    run_podman quadlet rm --all -f

    # Test quadlet install with directory
    run_podman quadlet install $quadlet_dir

    # Test quadlet list to verify all containers were installed
    run_podman quadlet list
    assert "$output" =~ "alpine1.container" "list should contain alpine1.container"
    assert "$output" =~ "alpine2.container" "list should contain alpine2.container"
    assert "$output" =~ "nginx.container" "list should contain nginx.container"

    # Test quadlet list with filter for alpine containers
    run_podman quadlet list --filter name=alpine*
    assert "$output" =~ "alpine1.container" "filtered list should contain alpine1.container"
    assert "$output" =~ "alpine2.container" "filtered list should contain alpine2.container"
    assert "$output" !~ "nginx.container" "filtered list should not contain nginx.container"

    # Test quadlet print for each container
    run_podman quadlet print alpine1.container
    assert "$output" =~ "Image=$IMAGE" "print should show correct image for alpine1"

    run_podman quadlet print alpine2.container
    assert "$output" =~ "Image=$IMAGE" "print should show correct image for alpine2"

    run_podman quadlet print nginx.container
    assert "$output" =~ "Image=quay.io/libpod/nginx:latest" "print should show correct image for nginx"

    # Test quadlet rm for all containers
    run_podman quadlet rm alpine1.container alpine2.container nginx.container

    # Verify all containers were removed
    run_podman quadlet list
    assert "$output" !~ "alpine1.container" "list should not contain removed container alpine1"
    assert "$output" !~ "alpine2.container" "list should not contain removed container alpine2"
    assert "$output" !~ "nginx.container" "list should not contain removed container nginx"
}

# bats test_tags=distro-integration
@test "quadlet verb - install from URL" {
    # Clean all quadlets
    run_podman quadlet rm --all -f

    # Test quadlet install from URL and capture the output
    run_podman quadlet install https://raw.githubusercontent.com/containers/podman/main/test/e2e/quadlet/basic.container
    # Extract just the basename from the full path
    quadlet_name=$(basename "$output")

    # Test quadlet list to verify the container was installed
    run_podman quadlet list
    assert "$output" =~ "$quadlet_name" "list should contain $quadlet_name"

    # Test quadlet print to verify the configuration
    run_podman quadlet print "$quadlet_name"
    assert "$output" =~ "\[Container\]" "print should show container section"
    assert "$output" =~ "Image=" "print should show image configuration"

    # Test quadlet rm
    run_podman quadlet rm "$quadlet_name"

    # Verify removal
    run_podman quadlet list
    assert "$output" !~ "$quadlet_name" "list should not contain removed container"
}

# bats test_tags=distro-integration
@test "quadlet verb - install with external file mount" {
    # Create a test.txt file with content
    local test_file=$PODMAN_TMPDIR/test.txt
    echo "This is a test file for quadlet mount testing" > $test_file

    # Create a quadlet directory for installation
    local quadlet_dir=$PODMAN_TMPDIR/quadlet-mount-test
    mkdir -p $quadlet_dir

    # Create quadlet file that mounts the external test.txt file
    local quadlet_file=$quadlet_dir/mount-test.container
    cat > $quadlet_file <<EOF
[Container]
Image=$IMAGE
Exec=sh -c "echo STARTED CONTAINER WITH MOUNT; cat /mounted/test.txt; trap 'exit' SIGTERM; while :; do sleep 0.1; done"
Mount=type=bind,source=./test.txt,destination=/test.txt:z
Notify=yes
LogDriver=passthrough
EOF

    # Determine the install directory path based on rootless/root
    local install_dir
    if is_rootless; then
        # For rootless: $XDG_CONFIG_HOME/containers/systemd or ~/.config/containers/systemd
        local config_home=${XDG_CONFIG_HOME:-$HOME/.config}
        install_dir="$config_home/containers/systemd"
    else
        # For root: /etc/containers/systemd
        install_dir="/etc/containers/systemd"
    fi

    # Clean all quadlets
    run_podman quadlet rm --all -f

    # Test quadlet install with the directory containing the quadlet and test file
    run_podman quadlet install $quadlet_dir $test_file

    # Verify the test.txt file exists in the install directory
    [ -e "$install_dir/test.txt" ]
    assert $status -eq 0 "test.txt file should exist in install directory $install_dir"

    # Verify the content of the installed test.txt file
    run cat "$install_dir/test.txt"
    assert $status -eq 0 "should be able to read installed test.txt file"
    assert "$output" == "This is a test file for quadlet mount testing" "installed test.txt should have correct content"

    # Test quadlet list to verify the container was installed
    run_podman quadlet list
    assert "$output" =~ "mount-test.container" "list should contain mount-test.container"

    # Test quadlet print to verify the configuration includes the volume mount
    run_podman quadlet print mount-test.container
    assert "$output" =~ "\[Container\]" "print should show container section"
    assert "$output" =~ "Image=$IMAGE" "print should show correct image"
    assert "$output" =~ "Mount=type=bind,source=./test.txt,destination=/test.txt:z" "print should show volume mount configuration"

    # Test quadlet rm
    run_podman quadlet rm mount-test.container

    # Verify the test.txt file should not exists in $install_dir
    [ ! -e "$install_dir/test.txt" ]
    assert $status -eq 0 "test.txt file should not exist in install directory $install_dir after removal"

    # Verify removal
    run_podman quadlet list
    assert "$output" !~ "mount-test.container" "list should not contain removed container"

    # Clean up test file
    rm -f $test_file
}

# bats test_tags=distro-integration
@test "quadlet verb - install multiple quadlets from directory and remove by application" {
    # Create a temporary directory for multiple quadlet files
    local app_name="test-app-$(safe_name)"
    local quadlet_dir=$PODMAN_TMPDIR/$app_name
    mkdir -p $quadlet_dir

    # Create multiple quadlet files with different configurations
    cat > $quadlet_dir/web-server.container <<EOF
[Container]
Image=$IMAGE
Exec=sh -c "echo STARTED WEB SERVER; trap 'exit' SIGTERM; while :; do sleep 0.1; done"
Notify=yes
LogDriver=passthrough
EOF

    cat > $quadlet_dir/database.container <<EOF
[Container]
Image=$IMAGE
Exec=sh -c "echo STARTED DATABASE; trap 'exit' SIGTERM; while :; do sleep 0.1; done"
Notify=yes
LogDriver=passthrough
EOF

    cat > $quadlet_dir/cache.container <<EOF
[Container]
Image=$IMAGE
Exec=sh -c "echo STARTED CACHE; trap 'exit' SIGTERM; while :; do sleep 0.1; done"
Notify=yes
LogDriver=passthrough
EOF

    # Clean all quadlets first
    run_podman quadlet rm --all -f

    # Install all quadlets from the directory
    run_podman quadlet install $quadlet_dir/

    # Verify all quadlets are listed
    run_podman quadlet list
    assert "$output" =~ "web-server.container" "list should contain web-server.container"
    assert "$output" =~ "database.container" "list should contain database.container"
    assert "$output" =~ "cache.container" "list should contain cache.container"

    # Verify we can print each quadlet configuration
    run_podman quadlet print web-server.container
    assert "$output" =~ "Image=$IMAGE" "print should show correct image for web-server"

    run_podman quadlet print database.container
    assert "$output" =~ "Image=$IMAGE" "print should show correct image for database"

    run_podman quadlet print cache.container
    assert "$output" =~ "Image=$IMAGE" "print should show correct image for cache"

    # Remove all quadlets by application name (using the directory name as app identifier)
    run_podman quadlet rm ".$app_name.app"

    # Verify all quadlets were removed
    run_podman quadlet list
    assert "$output" !~ "web-server.container" "list should not contain web-server.container"
    assert "$output" !~ "database.container" "list should not contain database.container"
    assert "$output" !~ "cache.container" "list should not contain cache.container"

    # Clean up temporary directory
    rm -rf $quadlet_dir
}
# vim: filetype=sh
