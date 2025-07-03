#!/usr/bin/env bats   -*- bats -*-
#
# Tests generated configurations for systemd.
#

load helpers
load helpers.network
load helpers.registry
load helpers.systemd

function setup() {
    skip_if_remote "podman quadlet is not implemented for remote setup yet"
    skip_if_rootless_cgroupsv1 "Can't use --cgroups=split w/ CGv1 (issue 17456, wontfix)"
    skip_if_journald_unavailable "Needed for RHEL. FIXME: we might be able to re-enable a subset of tests."

    test -x "$QUADLET" || die "Cannot run quadlet tests without executable \$QUADLET ($QUADLET)"

    basic_setup
}

function teardown() {
    systemctl daemon-reload
    basic_teardown
}

# Helper function to get the systemd install directory based on rootless/root mode
function get_quadlet_install_dir() {
    if is_rootless; then
        # For rootless: $XDG_CONFIG_HOME/containers/systemd or ~/.config/containers/systemd
        local config_home=${XDG_CONFIG_HOME:-$HOME/.config}
        echo "$config_home/containers/systemd"
    else
        # For root: /etc/containers/systemd
        echo "/etc/containers/systemd"
    fi
}

@test "quadlet verb - install, list, rm" {
    # Determine the install directory path based on rootless/root
    local install_dir=$(get_quadlet_install_dir)
    # Create a test quadlet file
    local quadlet_file=$PODMAN_TMPDIR/alpine-quadlet.container
    cat > $quadlet_file <<EOF
[Container]
Image=$IMAGE
Exec=sh -c "echo STARTED CONTAINER; trap 'exit' SIGTERM; while :; do sleep 0.1; done"
EOF
    # Test quadlet install
    run_podman quadlet install $quadlet_file
    # Verify install output contains the quadlet name on a single line
    assert "$output" =~ "alpine-quadlet.container" "install output should contain quadlet name"

    # Count lines in output (should be 1 line with the quadlet name)
    assert "${#lines[@]}" -eq 1 "install output should contain exactly one line"

    # Test quadlet list
    run_podman quadlet list
    assert "$output" =~ "alpine-quadlet.container" "list should contain alpine-quadlet.container"
    assert "$output" =~ "alpine-quadlet.service" "UNIT NAME should be alpine-quadlet.service"

    # Loaded status should be inactive/dead
    assert "$output" =~ "inactive/dead" "loaded status should be 'inactive/dead'"

    assert "$output" =~ "$install_dir/alpine-quadlet.container" "PATH ON DISK must be set and must belong to"

    # Test quadlet list with filter
    run_podman quadlet list --filter name=something*
    assert "$output" !~ "alpine-quadlet.container" "filtered list should not contain alpine-quadlet.container"

    # Test quadlet list with matching filter
    run_podman quadlet list --filter name=alpine*
    assert "$output" =~ "alpine-quadlet.container" "matching filter should contain alpine-quadlet.container"

    # Test quadlet print
    run_podman quadlet print alpine-quadlet.container
    assert "$output" == "$(<$quadlet_file)" "print output matches quadlet file"

    # Test quadlet rm
    run_podman quadlet rm alpine-quadlet.container
    # Verify remove output contains the quadlet name on a single line
    assert "$output" =~ "alpine-quadlet.container" "remove output should contain quadlet name"

    # Count lines in output (should be 1 line with the quadlet name)
    assert "${#lines[@]}" -eq 1 "remove output should contain exactly one line"

    # Verify removal
    run_podman quadlet list
    assert "$output" !~ "alpine-quadlet.container" "list should not contain removed container"
}

@test "quadlet verb - install multiple files from directory and remove by app name" {
    # Create a directory for multiple quadlet files
    local app_name="test-app-$(safe_name)"
    local quadlet_dir="$PODMAN_TMPDIR/$app_name"
    mkdir -p $quadlet_dir

    # Create multiple quadlet files with different configurations
    cat > $quadlet_dir/alpine1.container <<EOF
[Container]
Image=$IMAGE
Exec=sh -c "echo STARTED CONTAINER 1; trap 'exit' SIGTERM; while :; do sleep 0.1; done"
EOF

    cat > $quadlet_dir/alpine2.container <<EOF
[Container]
Image=$IMAGE
Exec=sh -c "echo STARTED CONTAINER 2; trap 'exit' SIGTERM; while :; do sleep 0.1; done"
EOF

    cat > $quadlet_dir/nginx.container <<EOF
[Container]
Image=$IMAGE
Environment=FOO1=foo1
Exec=sh -c "echo STARTED NGINX; trap 'exit' SIGTERM; while :; do sleep 0.1; done"
EOF
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
    assert "$output" =~ "Environment=FOO1=foo1" "print should contain environment for nginx container"

    # Test quadlet rm for all containers
    run_podman quadlet rm ".$app_name.app"

    # Verify all containers were removed
    run_podman quadlet list
    assert "$output" == "NAME        UNIT NAME   PATH ON DISK  STATUS      APPLICATION" "output should be blank"
}

@test "quadlet verb - install from URL" {
    # Create a directory for multiple quadlet files
    echo READY > $PODMAN_TMPDIR/ready
    local quadlet_dir="$PODMAN_TMPDIR/quadlet_diri_$(safe_name)"
    mkdir -p $quadlet_dir

    cat > $quadlet_dir/basic.container <<EOF
[Container]
Image=$Image
EOF

    HOST_PORT=$(random_free_port)
    SERVER=http://127.0.0.1:$HOST_PORT

    serverctr="quadletserver-$(safename)"
    run_podman run -d --name $serverctr -p "$HOST_PORT:80" \
               -v $quadlet_dir/basic.container:/var/www/basic.container:Z \
               -v $PODMAN_TMPDIR/ready:/var/www/ready:Z \
               -w /var/www \
               $IMAGE /bin/busybox-extras httpd -f -p 80

    wait_for_port 127.0.0.1 $HOST_PORT
    wait_for_command_output "curl -s -S $SERVER/ready" "READY"
    # Test quadlet install from URL and capture the output
    run_podman quadlet install $SERVER/basic.container
    # Extract just the basename from the full path
    quadlet_name=$(basename "$output")

    # Test quadlet list to verify the container was installed
    run_podman quadlet list
    assert "$output" =~ "$quadlet_name" "list should contain $quadlet_name"

    # Test quadlet print to verify the configuration
    run_podman quadlet print "$quadlet_name"
    assert "$output" == "$(<$quadlet_dir/basic.container)" "print output matches quadlet file"

    # Test quadlet rm
    run_podman quadlet rm "$quadlet_name"

    # Verify removal
    run_podman quadlet list
    assert "$output" !~ "$quadlet_name" "list should not contain removed container"

    run_podman rm -f -t0 $serverctr
}

@test "quadlet verb - install with external file mount" {
    # Create a test.txt file with content
    local test_file=$PODMAN_TMPDIR/test.txt
    mount_content="This is a test file for quadlet mount testing $(random_string)"
    echo $mount_content > $test_file

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
EOF

    # Determine the install directory path based on rootless/root
    local install_dir=$(get_quadlet_install_dir)

    # Test quadlet install with the directory containing the quadlet and test file
    run_podman quadlet install $quadlet_dir $test_file

    # Verify the content of the installed test.txt file
    run -0 cat "$install_dir/test.txt"
    assert "$output" == "$mount_content" "installed test.txt should have correct content"

    # Test quadlet list to verify the container was installed
    run_podman quadlet list
    assert "$output" =~ "mount-test.container" "list should contain mount-test.container"

    # Test quadlet print to verify the configuration includes the volume mount
    run_podman quadlet print mount-test.container
    assert "$output" == "$(<$quadlet_file)" "print output matches quadlet file"

    # Test quadlet rm
    run_podman quadlet rm mount-test.container

    # Verify the test.txt file should not exists in $install_dir
    if [[ -f "$install_dir/test.txt" ]]; then
            die "test.txt file should not exist in install directory $install_dir after removal"
    fi

    # Verify removal
    run_podman quadlet list
    assert "$output" !~ "mount-test.container" "list should not contain removed container"
}

@test "quadlet verb - install with --reload-systemd option" {
    # Create a test quadlet file
    local quadlet_file=$PODMAN_TMPDIR/reload-test-install.container
    cat > $quadlet_file <<EOF
[Container]
Image=$IMAGE
Exec=sh -c "echo STARTED CONTAINER FOR RELOAD TEST; trap 'exit' SIGTERM; while :; do sleep 0.1; done"
EOF

    # Get the expected service name for the quadlet
    local service_name=$(quadlet_to_service_name "reload-test-install.container")

    # Test quadlet install with --reload-systemd=true (should succeed)
    run_podman quadlet install --reload-systemd=true $quadlet_file
    assert $status -eq 0 "install with --reload-systemd=true should succeed"

    # Verify the quadlet was installed in podman
    run_podman quadlet list
    assert "$output" =~ "reload-test-install.container" "list should contain reload-test-install.container"

    # Verify the systemd unit exists and is loaded after reload
    run systemctl status "$service_name"
    assert $status -eq 3 "systemd unit should exist (inactive/loaded state)"
    assert "$output" =~ "Loaded: loaded" "systemd unit should be loaded"

    # Verify unit can be listed by systemctl
    run systemctl list-unit-files "$service_name"
    assert $status -eq 0 "systemctl should be able to list the unit file"
    assert "$output" =~ "$service_name" "unit file should be listed by systemctl"

    # Remove the quadlet for next test
    run_podman quadlet rm reload-test-install.container

    # Verify systemd unit is removed after quadlet removal
    run systemctl status "$service_name"
    assert $status -eq 4 "systemd unit should not exist after removal"

    # Test quadlet install with --reload-systemd=false (should succeed but systemd may not see it immediately)
    run_podman quadlet install --reload-systemd=false $quadlet_file
    assert $status -eq 0 "install with --reload-systemd=false should succeed"

    # Verify the quadlet was installed in podman
    run_podman quadlet list
    assert "$output" =~ "reload-test-install.container" "list should contain reload-test-install.container after install with --reload-systemd=false"

    # When --reload-systemd=false, systemd might not see the unit immediately
    # But after manual reload, it should be visible
    run systemctl daemon-reload
    run systemctl status "$service_name"
    assert $status -eq 3 "systemd unit should exist after manual daemon-reload"
    assert "$output" =~ "Loaded: loaded" "systemd unit should be loaded after manual reload"

    # Clean up
    run_podman quadlet rm reload-test-install.container
}


@test "quadlet verb - remove with --reload-systemd option" {
    # Create a test quadlet file
    local quadlet_file=$PODMAN_TMPDIR/reload-test-remove.container
    cat > $quadlet_file <<EOF
[Container]
Image=$IMAGE
Exec=sh -c "echo STARTED CONTAINER FOR RELOAD TEST; trap 'exit' SIGTERM; while :; do sleep 0.1; done"
EOF

    # Get the expected service name for the quadlet
    local service_name=$(quadlet_to_service_name "reload-test-remove.container")

    # Install the quadlet
    run_podman quadlet install $quadlet_file

    # Verify the quadlet was installed in podman
    run_podman quadlet list
    assert "$output" =~ "reload-test-remove.container" "list should contain reload-test-remove.container"

    # Verify the systemd unit exists after installation
    run systemctl status "$service_name"
    assert $status -eq 3 "systemd unit should exist (inactive/loaded state)"
    assert "$output" =~ "Loaded: loaded" "systemd unit should be loaded"

    # Test quadlet remove with --reload-systemd (should succeed and reload systemd)
    run_podman quadlet rm --reload-systemd reload-test-remove.container
    assert $status -eq 0 "remove with --reload-systemd should succeed"

    # Verify the quadlet was removed from podman
    run_podman quadlet list
    assert "$output" !~ "reload-test-remove.container" "list should not contain reload-test-remove.container after removal"

    # Verify the systemd unit is removed after removal with --reload-systemd
    run systemctl status "$service_name"
    assert $status -eq 4 "systemd unit should not exist after removal with --reload-systemd"
}

@test "quadlet verb - list with --format option" {
    # Create a test quadlet file
    local quadlet_file=$PODMAN_TMPDIR/format-test.container
    cat > $quadlet_file <<EOF
[Container]
Image=$IMAGE
Exec=sh -c "echo STARTED CONTAINER FOR FORMAT TEST; trap 'exit' SIGTERM; while :; do sleep 0.1; done"
EOF

    # Install the quadlet
    run_podman quadlet install $quadlet_file

    # Test default format (should show tabular output)
    run_podman quadlet list
    assert $status -eq 0 "quadlet list should succeed"
    assert "$output" =~ "format-test.container" "default format should contain quadlet name"

    # Test JSON format
    run_podman quadlet list --format json
    assert $status -eq 0 "quadlet list --format json should succeed"
    assert "$output" =~ '"Name".*"format-test.container"' "JSON format should contain name field"
    assert "$output" =~ '"Path"' "JSON format should contain path field"
    assert "$output" =~ '"Status"' "JSON format should contain status field"

    # Test custom Go template format - extract only names
    run_podman quadlet list --format '{{range .}}{{.Name}}{{"\n"}}{{end}}'
    assert $status -eq 0 "quadlet list with custom format should succeed"
    assert "$output" = $'NAME\nformat-test.container' "custom format should show only the name"

    # Test custom Go template format - extract multiple fields
    run_podman quadlet list --format '{{range .}}Name:{{.Name}} Status:{{.Status}}{{"\n"}}{{end}}'
    assert $status -eq 0 "quadlet list with multi-field format should succeed"
    assert "$output" =~ "Name:format-test.container" "multi-field format should show name"
    assert "$output" =~ "Status:" "multi-field format should show status"

    # Test format with specific field extraction
    run_podman quadlet list --format '{{.Name}}'
    assert $status -eq 0 "quadlet list with field format should succeed"
    assert "$output" = $'NAME\nformat-test.container' "field format should extract just the name"

    # Clean up
    run_podman quadlet rm format-test.container
}

@test "quadlet verb - rm --all and --ignore options" {
    # Create multiple test quadlet files
    local quadlet_file1=$PODMAN_TMPDIR/rm-all-test1.container
    cat > $quadlet_file1 <<EOF
[Container]
Image=$IMAGE
Exec=sh -c "echo STARTED CONTAINER 1; trap 'exit' SIGTERM; while :; do sleep 0.1; done"
EOF

    local quadlet_file2=$PODMAN_TMPDIR/rm-all-test2.container
    cat > $quadlet_file2 <<EOF
[Container]
Image=$IMAGE
Exec=sh -c "echo STARTED CONTAINER 2; trap 'exit' SIGTERM; while :; do sleep 0.1; done"
EOF

    local quadlet_file3=$PODMAN_TMPDIR/rm-all-test3.container
    cat > $quadlet_file3 <<EOF
[Container]
Image=$IMAGE
Exec=sh -c "echo STARTED CONTAINER 3; trap 'exit' SIGTERM; while :; do sleep 0.1; done"
EOF

    # Install all three quadlets
    run_podman quadlet install $quadlet_file1 $quadlet_file2 $quadlet_file3
    assert $status -eq 0 "quadlet install should succeed for all files"

    # Verify all quadlets were installed
    run_podman quadlet list
    assert "$output" =~ "rm-all-test1.container" "list should contain rm-all-test1.container"
    assert "$output" =~ "rm-all-test2.container" "list should contain rm-all-test2.container"
    assert "$output" =~ "rm-all-test3.container" "list should contain rm-all-test3.container"

    # Get the expected service names for systemd verification
    local service_name1=$(quadlet_to_service_name "rm-all-test1.container")
    local service_name2=$(quadlet_to_service_name "rm-all-test2.container")
    local service_name3=$(quadlet_to_service_name "rm-all-test3.container")

    # Verify systemd units exist for all quadlets
    run systemctl status "$service_name1"
    assert $status -eq 3 "systemd unit 1 should exist (inactive/loaded state)"
    assert "$output" =~ "Loaded: loaded" "systemd unit 1 should be loaded"

    run systemctl status "$service_name2"
    assert $status -eq 3 "systemd unit 2 should exist (inactive/loaded state)"
    assert "$output" =~ "Loaded: loaded" "systemd unit 2 should be loaded"

    run systemctl status "$service_name3"
    assert $status -eq 3 "systemd unit 3 should exist (inactive/loaded state)"
    assert "$output" =~ "Loaded: loaded" "systemd unit 3 should be loaded"

    # Test quadlet rm --all (should remove all quadlets)
    run_podman quadlet rm --all
    assert $status -eq 0 "quadlet rm --all should succeed"

    # Verify all quadlets were removed from podman
    run_podman quadlet list
    assert "$output" !~ "rm-all-test1.container" "list should not contain rm-all-test1.container after --all removal"
    assert "$output" !~ "rm-all-test2.container" "list should not contain rm-all-test2.container after --all removal"
    assert "$output" !~ "rm-all-test3.container" "list should not contain rm-all-test3.container after --all removal"

    # Verify all systemd units are removed
    run systemctl status "$service_name1"
    assert $status -eq 4 "systemd unit 1 should not exist after --all removal"

    run systemctl status "$service_name2"
    assert $status -eq 4 "systemd unit 2 should not exist after --all removal"

    run systemctl status "$service_name3"
    assert $status -eq 4 "systemd unit 3 should not exist after --all removal"

    # Test quadlet rm --ignore behavior
    # Try to remove non-existent quadlets without --ignore (should fail)
    run_podman 125 quadlet rm non-existent.container
    assert "$output" =~ "could not locate quadlet" "should fail to remove non-existent quadlet without --ignore"

    # Try to remove non-existent quadlets with --ignore (should succeed)
    run_podman quadlet rm --ignore non-existent1.container non-existent2.container
    assert $status -eq 0 "quadlet rm --ignore should succeed even for non-existent quadlets"
}

# vim: filetype=sh
