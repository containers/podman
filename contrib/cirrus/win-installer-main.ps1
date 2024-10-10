#!/usr/bin/env powershell

. $PSScriptRoot\win-lib.ps1

if ($Env:CI -eq "true") {
    $WIN_INST_FOLDER = "$ENV:CIRRUS_WORKING_DIR\repo\contrib\win-installer"
    $RELEASE_DIR = "$ENV:CIRRUS_WORKING_DIR\repo"
} else {
    $WIN_INST_FOLDER = "$PSScriptRoot\..\win-installer"
    $ENV:WIN_INST_VER = "9.9.9"
    $RELEASE_DIR = "$PSScriptRoot\..\..\contrib\win-installer\current"
    $ENV:CONTAINERS_MACHINE_PROVIDER = "wsl"
}

Push-Location $WIN_INST_FOLDER

# Build Installer
# Note: consumes podman-remote-release-windows_amd64.zip from repo.tar.zst
Run-Command ".\build.ps1 $Env:WIN_INST_VER dev `"$RELEASE_DIR`""

Pop-Location

# Run the installer silently and WSL/HyperV install options disabled (prevent reboots)
# We need -skipWinVersionCheck for server 2019 (cirrus image), can be dropped after server 2022
$command = "$WIN_INST_FOLDER\test-installer.ps1 "
$command += "-scenario all "
$command += "-provider $ENV:CONTAINERS_MACHINE_PROVIDER "
$command += "-setupExePath `"$WIN_INST_FOLDER\podman-$ENV:WIN_INST_VER-dev-setup.exe`""
Run-Command "${command}"
