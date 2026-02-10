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
    # remove any remaining quadlets from tests
    run_podman quadlet rm --all -f
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

    # Generate random names for parallelism
    local app_name="webapp_$(random_string)"
    local container_name="webserver_$(random_string)"
    local volume_name="appstorage_$(random_string)"
    local network_name="appnetwork_$(random_string)"

    # Create a multi-quadlet file with additional systemd sections
    local multi_quadlet_file=$PODMAN_TMPDIR/${app_name}.quadlets
    cat > $multi_quadlet_file <<EOF
# FileName=$container_name
# Web application stack
[Unit]
Description=Web server container for application
After=network.target
Wants=network.target

[Container]
Image=$IMAGE
ContainerName=web-server-$(random_string)
PublishPort=8080:80
Environment=APP_ENV=production

[Service]
Restart=always
TimeoutStartSec=900

[Install]
WantedBy=multi-user.target

---

# FileName=$volume_name
# Database volume
[Unit]
Description=Database storage volume
Documentation=https://example.com/storage-docs

[Volume]
Label=app=$app_name
Label=component=database
Driver=local

[Install]
WantedBy=multi-user.target

---

# FileName=$network_name
# Application network
[Unit]
Description=Application network for web services

[Network]
Subnet=10.0.0.0/24
Gateway=10.0.0.1
Label=app=$app_name

[Install]
WantedBy=multi-user.target
EOF

    # Test quadlet install with multi-quadlet file
    run_podman quadlet install $multi_quadlet_file

    # Verify install output contains all three quadlet names
    assert "$output" =~ "${container_name}.container" "install output should contain ${container_name}.container"
    assert "$output" =~ "${volume_name}.volume" "install output should contain ${volume_name}.volume"
    assert "$output" =~ "${network_name}.network" "install output should contain ${network_name}.network"

    # Count lines in output (should be 3 lines, one for each quadlet)
    assert "${#lines[@]}" -eq 3 "install output should contain exactly three lines"

    # Test quadlet list to verify all quadlets were installed
    run_podman quadlet list
    assert "$output" =~ "${container_name}.container" "list should contain ${container_name}.container"
    assert "$output" =~ "${volume_name}.volume" "list should contain ${volume_name}.volume"
    assert "$output" =~ "${network_name}.network" "list should contain ${network_name}.network"

    # Verify the files exist on disk
    [[ -f "$install_dir/${container_name}.container" ]] || die "${container_name}.container should exist on disk"
    [[ -f "$install_dir/${volume_name}.volume" ]] || die "${volume_name}.volume should exist on disk"
    [[ -f "$install_dir/${network_name}.network" ]] || die "${network_name}.network should exist on disk"

    # Test quadlet print for each installed quadlet and verify systemd sections are preserved
    run_podman quadlet print ${container_name}.container
    assert "$output" =~ "\\[Unit\\]" "print should show Unit section"
    assert "$output" =~ "Description=Web server container" "print should show Unit description"
    assert "$output" =~ "After=network.target" "print should show After directive"
    assert "$output" =~ "Wants=network.target" "print should show Wants directive"
    assert "$output" =~ "\\[Container\\]" "print should show container section"
    assert "$output" =~ "Image=$IMAGE" "print should show correct image"
    assert "$output" =~ "Environment=APP_ENV=production" "print should show environment variable"
    assert "$output" =~ "\\[Service\\]" "print should show Service section"
    assert "$output" =~ "Restart=always" "print should show Restart directive"
    assert "$output" =~ "TimeoutStartSec=900" "print should show TimeoutStartSec directive"
    assert "$output" =~ "\\[Install\\]" "print should show Install section"
    assert "$output" =~ "WantedBy=multi-user.target" "print should show WantedBy directive"

    run_podman quadlet print ${volume_name}.volume
    assert "$output" =~ "\\[Unit\\]" "print should show Unit section"
    assert "$output" =~ "Description=Database storage volume" "print should show Unit description"
    assert "$output" =~ "Documentation=https://example.com/storage-docs" "print should show Documentation directive"
    assert "$output" =~ "\\[Volume\\]" "print should show volume section"
    assert "$output" =~ "Label=app=$app_name" "print should show app label"
    assert "$output" =~ "Driver=local" "print should show Driver directive"
    assert "$output" =~ "\\[Install\\]" "print should show Install section"
    assert "$output" =~ "WantedBy=multi-user.target" "print should show WantedBy directive"

    run_podman quadlet print ${network_name}.network
    assert "$output" =~ "\\[Unit\\]" "print should show Unit section"
    assert "$output" =~ "Description=Application network" "print should show Unit description"
    assert "$output" =~ "\\[Network\\]" "print should show network section"
    assert "$output" =~ "Subnet=10.0.0.0/24" "print should show subnet"
    assert "$output" =~ "\\[Install\\]" "print should show Install section"
    assert "$output" =~ "WantedBy=multi-user.target" "print should show WantedBy directive"

    # Check that the .app file was created (all quadlets are part of the same application)
    [[ -f "$install_dir/.${app_name}.app" ]] || die ".${app_name}.app file should exist"
    [[ ! -f "$install_dir/.${container_name}.container.asset" ]] || die "individual .asset files should not exist"
    [[ ! -f "$install_dir/.${volume_name}.volume.asset" ]] || die "individual .asset files should not exist"
    [[ ! -f "$install_dir/.${network_name}.network.asset" ]] || die "individual .asset files should not exist"

    # Verify the .app file contains all quadlet names
    run cat "$install_dir/.${app_name}.app"
    assert "$output" =~ "${container_name}.container" ".app file should contain ${container_name}.container"
    assert "$output" =~ "${volume_name}.volume" ".app file should contain ${volume_name}.volume"
    assert "$output" =~ "${network_name}.network" ".app file should contain ${network_name}.network"

    # Test quadlet list to verify all quadlets show the same app name
    run_podman quadlet list
    local webserver_line=$(echo "$output" | grep "${container_name}.container")
    local appstorage_line=$(echo "$output" | grep "${volume_name}.volume")
    local appnetwork_line=$(echo "$output" | grep "${network_name}.network")

    # All lines should contain the same app name (.${app_name}.app)
    assert "$webserver_line" =~ "\\.${app_name}\\.app" "${container_name} should show .${app_name}.app as app"
    assert "$appstorage_line" =~ "\\.${app_name}\\.app" "${volume_name} should show .${app_name}.app as app"
    assert "$appnetwork_line" =~ "\\.${app_name}\\.app" "${network_name} should show .${app_name}.app as app"

    # Test quadlet rm for one of the quadlets - should remove entire application
    run_podman quadlet rm ${container_name}.container
    assert "$output" =~ "${container_name}.container" "remove output should contain ${container_name}.container"

    # Verify all quadlets were removed since they're part of the same app
    run_podman quadlet list
    assert "$output" !~ "${container_name}.container" "list should not contain removed ${container_name}.container"
    assert "$output" !~ "${volume_name}.volume" "list should not contain ${volume_name}.volume as app is removed"
    assert "$output" !~ "${network_name}.network" "list should not contain ${network_name}.network as app is removed"

    # The .app file should also be removed
    [[ ! -f "$install_dir/.${app_name}.app" ]] || die ".${app_name}.app file should be removed"
}

@test "quadlet verb - install multi-quadlet file with empty sections" {
    # Test handling of empty sections between separators
    local container_name="testcontainer_$(random_string)"
    local volume_name="testvolume_$(random_string)"
    local multi_quadlet_file=$PODMAN_TMPDIR/with-empty_$(random_string).quadlets
    cat > $multi_quadlet_file <<EOF
# FileName=$container_name
[Container]
Image=$IMAGE
ContainerName=test-container-$(random_string)

---

---

# FileName=$volume_name
[Volume]
Label=test=value

---

EOF

    # Test quadlet install
    run_podman quadlet install $multi_quadlet_file

    # Should only install 2 quadlets (empty sections should be skipped)
    assert "$output" =~ "${container_name}.container" "install output should contain ${container_name}.container"
    assert "$output" =~ "${volume_name}.volume" "install output should contain ${volume_name}.volume"
    assert "${#lines[@]}" -eq 2 "install output should contain exactly two lines"

    # Clean up
    run_podman quadlet rm ${container_name}.container ${volume_name}.volume
}

@test "quadlet verb - install multi-quadlet file missing FileName" {
    # Test error handling when FileName is missing in multi-quadlet file
    local volume_name="testvolume_$(random_string)"
    local multi_quadlet_file=$PODMAN_TMPDIR/missing-filename_$(random_string).quadlets
    cat > $multi_quadlet_file <<EOF
[Container]
Image=$IMAGE
ContainerName=test-container-$(random_string)

---

# FileName=$volume_name
[Volume]
Label=test=value
EOF

    # Test quadlet install should fail
    run_podman 125 quadlet install $multi_quadlet_file
    assert "$output" =~ "missing required.*FileName" "error should mention missing FileName"
}

@test "quadlet verb - install single-section .quadlets file missing FileName" {
    # Test error handling when FileName is missing in a .quadlets file with only one section
    local multi_quadlet_file=$PODMAN_TMPDIR/single-missing-filename_$(random_string).quadlets
    cat > $multi_quadlet_file <<EOF
[Container]
Image=$IMAGE
ContainerName=test-container-$(random_string)
EOF

    # Test quadlet install should fail
    run_podman 125 quadlet install $multi_quadlet_file
    assert "$output" =~ "missing required.*FileName" "error should mention missing FileName"
}

@test "quadlet verb - install directory with mixed individual and .quadlets files" {
    # Test installing from a directory containing both individual quadlet files and .quadlets files
    local install_dir=$(get_quadlet_install_dir)
    local app_name="mixed-app_$(random_string)"
    local app_dir=$PODMAN_TMPDIR/$app_name
    mkdir -p "$app_dir"

    # Generate random names for all components
    local frontend_name="frontend_$(random_string)"
    local data_name="data_$(random_string)"
    local api_name="api-server_$(random_string)"
    local cache_name="cache_$(random_string)"
    local network_name="app-network_$(random_string)"

    # Create an individual container quadlet file
    cat > "$app_dir/${frontend_name}.container" <<EOF
[Container]
Image=$IMAGE
ContainerName=frontend-app-$(random_string)
PublishPort=3000:3000
EOF

    # Create an individual volume quadlet file
    cat > "$app_dir/${data_name}.volume" <<EOF
[Volume]
Label=app=$app_name
Label=component=storage
EOF

    # Create a .quadlets file with multiple quadlets
    cat > "$app_dir/backend_$(random_string).quadlets" <<EOF
# FileName=$api_name
[Container]
Image=$IMAGE
ContainerName=api-server-$(random_string)
PublishPort=8080:8080

---

# FileName=$cache_name
[Volume]
Label=app=$app_name
Label=component=cache

---

# FileName=$network_name
[Network]
Subnet=192.168.1.0/24
Gateway=192.168.1.1
Label=app=$app_name
EOF

    # Create a non-quadlet asset file (config file)
    cat > "$app_dir/app.conf" <<EOF
# Application configuration
debug=true
port=3000
EOF

    # Install the directory
    run_podman quadlet install "$app_dir"

    # Verify all quadlets were installed (2 individual + 3 from .quadlets file = 5 total)
    assert "$output" =~ "${frontend_name}.container" "install output should contain ${frontend_name}.container"
    assert "$output" =~ "${data_name}.volume" "install output should contain ${data_name}.volume"
    assert "$output" =~ "${api_name}.container" "install output should contain ${api_name}.container"
    assert "$output" =~ "${cache_name}.volume" "install output should contain ${cache_name}.volume"
    assert "$output" =~ "${network_name}.network" "install output should contain ${network_name}.network"

    # Count lines in output (should be 6 lines: 5 quadlets + 1 asset file)
    assert "${#lines[@]}" -eq 6 "install output should contain exactly six lines"

    # Verify all files exist on disk
    [[ -f "$install_dir/${frontend_name}.container" ]] || die "${frontend_name}.container should exist on disk"
    [[ -f "$install_dir/${data_name}.volume" ]] || die "${data_name}.volume should exist on disk"
    [[ -f "$install_dir/${api_name}.container" ]] || die "${api_name}.container should exist on disk"
    [[ -f "$install_dir/${cache_name}.volume" ]] || die "${cache_name}.volume should exist on disk"
    [[ -f "$install_dir/${network_name}.network" ]] || die "${network_name}.network should exist on disk"
    [[ -f "$install_dir/app.conf" ]] || die "app.conf should exist on disk"

    # Check that the .app file was created (all files are part of one application)
    [[ -f "$install_dir/.${app_name}.app" ]] || die ".${app_name}.app file should exist"

    # Verify the .app file contains all quadlet names
    run cat "$install_dir/.${app_name}.app"
    assert "$output" =~ "${frontend_name}.container" ".app file should contain ${frontend_name}.container"
    assert "$output" =~ "${data_name}.volume" ".app file should contain ${data_name}.volume"
    assert "$output" =~ "${api_name}.container" ".app file should contain ${api_name}.container"
    assert "$output" =~ "${cache_name}.volume" ".app file should contain ${cache_name}.volume"
    assert "$output" =~ "${network_name}.network" ".app file should contain ${network_name}.network"

    # Test quadlet list to verify all quadlets show the same app name
    run_podman quadlet list
    local frontend_line=$(echo "$output" | grep "${frontend_name}.container")
    local data_line=$(echo "$output" | grep "${data_name}.volume")
    local api_line=$(echo "$output" | grep "${api_name}.container")
    local cache_line=$(echo "$output" | grep "${cache_name}.volume")
    local network_line=$(echo "$output" | grep "${network_name}.network")

    # All lines should contain the same app name (.${app_name}.app)
    assert "$frontend_line" =~ "\\.${app_name}\\.app" "${frontend_name} should show .${app_name}.app as app"
    assert "$data_line" =~ "\\.${app_name}\\.app" "${data_name} should show .${app_name}.app as app"
    assert "$api_line" =~ "\\.${app_name}\\.app" "${api_name} should show .${app_name}.app as app"
    assert "$cache_line" =~ "\\.${app_name}\\.app" "${cache_name} should show .${app_name}.app as app"
    assert "$network_line" =~ "\\.${app_name}\\.app" "${network_name} should show .${app_name}.app as app"

    # Verify content of individual quadlet files
    run cat "$install_dir/${frontend_name}.container"
    assert "$output" =~ "\\[Container\\]" "frontend container file should contain [Container] section"
    assert "$output" =~ "ContainerName=frontend-app-" "frontend container file should contain correct name prefix"

    run cat "$install_dir/${api_name}.container"
    assert "$output" =~ "\\[Container\\]" "api-server container file should contain [Container] section"
    assert "$output" =~ "ContainerName=api-server-" "api-server container file should contain correct name prefix"

    run cat "$install_dir/${network_name}.network"
    assert "$output" =~ "\\[Network\\]" "network file should contain [Network] section"
    assert "$output" =~ "Subnet=192.168.1.0/24" "network file should contain correct subnet"

    # Test that removing one quadlet removes the entire application
    run_podman quadlet rm ${frontend_name}.container

    # All quadlets should be removed since they're part of the same app
    run_podman quadlet list
    assert "$output" !~ "${frontend_name}.container" "${frontend_name}.container should be removed"
    assert "$output" !~ "${data_name}.volume" "${data_name}.volume should also be removed as part of same app"
    assert "$output" !~ "${api_name}.container" "${api_name}.container should also be removed as part of same app"
    assert "$output" !~ "${cache_name}.volume" "${cache_name}.volume should also be removed as part of same app"
    assert "$output" !~ "${network_name}.network" "${network_name}.network should also be removed as part of same app"

    # The .app file should also be removed
    [[ ! -f "$install_dir/.${app_name}.app" ]] || die ".${app_name}.app file should be removed"

    # All individual files should be removed
    [[ ! -f "$install_dir/${frontend_name}.container" ]] || die "${frontend_name}.container should be removed"
    [[ ! -f "$install_dir/${data_name}.volume" ]] || die "${data_name}.volume should be removed"
    [[ ! -f "$install_dir/${api_name}.container" ]] || die "${api_name}.container should be removed"
    [[ ! -f "$install_dir/${cache_name}.volume" ]] || die "${cache_name}.volume should be removed"
    [[ ! -f "$install_dir/${network_name}.network" ]] || die "${network_name}.network should be removed"
    [[ ! -f "$install_dir/app.conf" ]] || die "app.conf should be removed"
}
