#!/usr/bin/env bats   -*- bats -*-
#
# Tests for podman quadlet install with multi-quadlet files
#

load helpers
load helpers.systemd

function setup() {
    skip_if_remote "podman quadlet is not implemented for remote setup yet"
    skip_if_rootless_cgroupsv1 "Can't use --cgroups=split w/ CGv1 (issue 17456, wontfix)"
    skip_if_journald_unavailable "Needed for RHEL. FIXME: we might be able to re-enable a subset of tests."

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

@test "quadlet verb - install multi-quadlet file" {
    # Determine the install directory path based on rootless/root
    local install_dir=$(get_quadlet_install_dir)

    # Create a multi-quadlet file
    local multi_quadlet_file=$PODMAN_TMPDIR/webapp.quadlets
    cat > $multi_quadlet_file <<EOF
# FileName=webserver
# Web application stack
[Container]
Image=$IMAGE
ContainerName=web-server
PublishPort=8080:80

---

# FileName=appstorage
# Database volume
[Volume]
Label=app=webapp
Label=component=database

---

# FileName=appnetwork
# Application network
[Network]
Subnet=10.0.0.0/24
Gateway=10.0.0.1
Label=app=webapp
EOF

    # Test quadlet install with multi-quadlet file
    run_podman quadlet install $multi_quadlet_file

    # Verify install output contains all three quadlet names
    assert "$output" =~ "webserver.container" "install output should contain webserver.container"
    assert "$output" =~ "appstorage.volume" "install output should contain appstorage.volume"
    assert "$output" =~ "appnetwork.network" "install output should contain appnetwork.network"

    # Count lines in output (should be 3 lines, one for each quadlet)
    assert "${#lines[@]}" -eq 3 "install output should contain exactly three lines"

    # Test quadlet list to verify all quadlets were installed
    run_podman quadlet list
    assert "$output" =~ "webserver.container" "list should contain webserver.container"
    assert "$output" =~ "appstorage.volume" "list should contain appstorage.volume"
    assert "$output" =~ "appnetwork.network" "list should contain appnetwork.network"

    # Verify the files exist on disk
    [[ -f "$install_dir/webserver.container" ]] || die "webserver.container should exist on disk"
    [[ -f "$install_dir/appstorage.volume" ]] || die "appstorage.volume should exist on disk"
    [[ -f "$install_dir/appnetwork.network" ]] || die "appnetwork.network should exist on disk"

    # Verify the content of each installed file
    run cat "$install_dir/webserver.container"
    assert "$output" =~ "\\[Container\\]" "container file should contain [Container] section"
    assert "$output" =~ "Image=$IMAGE" "container file should contain correct image"
    assert "$output" =~ "ContainerName=web-server" "container file should contain container name"

    run cat "$install_dir/appstorage.volume"
    assert "$output" =~ "\\[Volume\\]" "volume file should contain [Volume] section"
    assert "$output" =~ "Label=app=webapp" "volume file should contain app label"
    assert "$output" =~ "Label=component=database" "volume file should contain component label"

    run cat "$install_dir/appnetwork.network"
    assert "$output" =~ "\\[Network\\]" "network file should contain [Network] section"
    assert "$output" =~ "Subnet=10.0.0.0/24" "network file should contain subnet"
    assert "$output" =~ "Gateway=10.0.0.1" "network file should contain gateway"

    # Test quadlet print for each installed quadlet
    run_podman quadlet print webserver.container
    assert "$output" =~ "\\[Container\\]" "print should show container section"
    assert "$output" =~ "Image=$IMAGE" "print should show correct image"

    run_podman quadlet print appstorage.volume
    assert "$output" =~ "\\[Volume\\]" "print should show volume section"
    assert "$output" =~ "Label=app=webapp" "print should show app label"

    run_podman quadlet print appnetwork.network
    assert "$output" =~ "\\[Network\\]" "print should show network section"
    assert "$output" =~ "Subnet=10.0.0.0/24" "print should show subnet"

    # Test quadlet rm for one of the quadlets
    run_podman quadlet rm webserver.container
    assert "$output" =~ "webserver.container" "remove output should contain webserver.container"

    # Verify the container quadlet was removed but others remain
    run_podman quadlet list
    assert "$output" !~ "webserver.container" "list should not contain removed webserver.container"
    assert "$output" =~ "appstorage.volume" "list should still contain appstorage.volume"
    assert "$output" =~ "appnetwork.network" "list should still contain appnetwork.network"

    # Clean up remaining quadlets
    run_podman quadlet rm appstorage.volume appnetwork.network
}

@test "quadlet verb - install multi-quadlet file with empty sections" {
    # Test handling of empty sections between separators
    local multi_quadlet_file=$PODMAN_TMPDIR/with-empty.quadlets
    cat > $multi_quadlet_file <<EOF
# FileName=testcontainer
[Container]
Image=$IMAGE
ContainerName=test-container

---

---

# FileName=testvolume
[Volume]
Label=test=value

---

EOF

    # Test quadlet install
    run_podman quadlet install $multi_quadlet_file

    # Should only install 2 quadlets (empty sections should be skipped)
    assert "$output" =~ "testcontainer.container" "install output should contain testcontainer.container"
    assert "$output" =~ "testvolume.volume" "install output should contain testvolume.volume"
    assert "${#lines[@]}" -eq 2 "install output should contain exactly two lines"

    # Clean up
    run_podman quadlet rm testcontainer.container testvolume.volume
}

@test "quadlet verb - install multi-quadlet file missing FileName" {
    # Test error handling when FileName is missing in multi-quadlet file
    local multi_quadlet_file=$PODMAN_TMPDIR/missing-filename.quadlets
    cat > $multi_quadlet_file <<EOF
[Container]
Image=$IMAGE
ContainerName=test-container

---

# FileName=testvolume
[Volume]
Label=test=value
EOF

    # Test quadlet install should fail
    run_podman 125 quadlet install $multi_quadlet_file
    assert "$output" =~ "missing required.*FileName" "error should mention missing FileName"
}
