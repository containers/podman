#!/usr/bin/env powershell

. $PSScriptRoot\win-lib.ps1
. $PSScriptRoot\..\win-installer\utils.ps1

if ($Env:CIRRUS_CI -eq "true") {
    $WIN_INST_FOLDER = "$ENV:CIRRUS_WORKING_DIR\repo\contrib\win-installer"
    $RELEASE_DIR = "$ENV:CIRRUS_WORKING_DIR\repo"
} else {
    $WIN_INST_FOLDER = "$PSScriptRoot\..\win-installer"
    $ENV:WIN_INST_VER = "9.9.9"
    $RELEASE_DIR = Resolve-Path -Path "$PSScriptRoot\..\..\contrib\win-installer\current"
    if ($null -eq $ENV:CONTAINERS_MACHINE_PROVIDER) { $ENV:CONTAINERS_MACHINE_PROVIDER = 'wsl' }
}

$ENV:PODMAN_ARCH = Get-Current-Architecture # on Cirrus CI this should always be amd64

# To get the a GitHub release ID run the following command:
#  curl -s -H "Accept: application/vnd.github+json" \
#    https://api.github.com/repos/containers/podman/releases/tags/v5.6.2 | jq '.id'
$ENV:LATEST_GH_RELEASE_ID = "251232431" # v5.6.2
$NEXT_WIN_INST_VER="9.9.10"

# Download the previous installer to test a major update
if (!$env:PREV_SETUP_EXE_PATH) {
    $env:PREV_SETUP_EXE_PATH = Get-Podman-Setup-From-GitHub $ENV:LATEST_GH_RELEASE_ID
}

#########################################################
# Build and test the new windows installer msi package
#########################################################

$installer_folder = Resolve-Path -Path "$WIN_INST_FOLDER"

# Note: consumes podman-remote-release-windows_amd64.zip from repo.tar.zst
Run-Command "${installer_folder}\build.ps1 -Version `"$Env:WIN_INST_VER`" -Architecture `"$ENV:PODMAN_ARCH`" -LocalReleaseDirPath `"$RELEASE_DIR`""

# Build a v9.9.10 installer to test an update from current to next version
Run-Command "${installer_folder}\build.ps1 -Version `"$NEXT_WIN_INST_VER`" -Architecture `"$ENV:PODMAN_ARCH`" -LocalReleaseDirPath `"$RELEASE_DIR`""

# Run the installer silently and WSL/HyperV install options disabled (prevent reboots)
$command = "${installer_folder}\test.ps1 "
$command += "-scenario all "
$command += "-provider $ENV:CONTAINERS_MACHINE_PROVIDER "
$command += "-msiPath `"${installer_folder}\podman-$ENV:WIN_INST_VER.msi`" "
$command += "-nextMsiPath `"${installer_folder}\podman-$NEXT_WIN_INST_VER.msi`" "
$command += "-previousSetupExePath `"$env:PREV_SETUP_EXE_PATH`""
Run-Command "${command}"

#########################################################
# Build and test the legacy windows installer setup bundle
#########################################################

$installer_folder = Resolve-Path -Path "$WIN_INST_FOLDER-legacy"

Push-Location "${installer_folder}"

# Note: consumes podman-remote-release-windows_amd64.zip from repo.tar.zst
Run-Command ".\build.ps1 $Env:WIN_INST_VER dev `"$RELEASE_DIR`""

# Build a v9.9.10 installer to test an update from current to next version
Run-Command ".\build.ps1 `"$NEXT_WIN_INST_VER`" dev `"$RELEASE_DIR`""

Pop-Location

# Run the installer silently and WSL/HyperV install options disabled (prevent reboots)
$command = "${installer_folder}\test-installer.ps1 "
$command += "-scenario all "
$command += "-provider $ENV:CONTAINERS_MACHINE_PROVIDER "
$command += "-setupExePath `"${installer_folder}\podman-$ENV:WIN_INST_VER-dev-setup.exe`" "
$command += "-previousSetupExePath `"$env:PREV_SETUP_EXE_PATH`" "
$command += "-nextSetupExePath `"${installer_folder}\podman-$NEXT_WIN_INST_VER-dev-setup.exe`" "
Run-Command "${command}"
