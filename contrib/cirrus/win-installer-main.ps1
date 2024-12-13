#!/usr/bin/env powershell

. $PSScriptRoot\win-lib.ps1
. $PSScriptRoot\..\win-installer\utils.ps1

if ($Env:CI -eq "true") {
    $WIN_INST_FOLDER = "$ENV:CIRRUS_WORKING_DIR\repo\contrib\win-installer"
    $RELEASE_DIR = "$ENV:CIRRUS_WORKING_DIR\repo"
} else {
    $WIN_INST_FOLDER = "$PSScriptRoot\..\win-installer"
    $ENV:WIN_INST_VER = "9.9.9"
    $RELEASE_DIR = "$PSScriptRoot\..\..\contrib\win-installer\current"
    if ($null -eq $ENV:CONTAINERS_MACHINE_PROVIDER) { $ENV:CONTAINERS_MACHINE_PROVIDER = 'wsl' }
}

Push-Location $WIN_INST_FOLDER

# Build and test the windows installer

# Download v5.3.1 installer as `build.ps1` uses it to build the patch
# (podman.msp). `build.ps1` reads `$env:V531_SETUP_EXE_PATH` to get its path.
# The v5.3.1 installer is also used to test the "v5.3.1 -> current" version minor
# update (with patch).
if (!$env:V531_SETUP_EXE_PATH) {
    $env:V531_SETUP_EXE_PATH = Get-Podman-Setup-From-GitHub -version "tags/v5.3.1"
}

# Download the previous installer to test a major update (without patch)
# After v5.3.2 release we should download latest instead of v5.3.0 (i.e.
# `Get-Latest-Podman-Setup-From-GitHub`)
if (!$env:PREV_SETUP_EXE_PATH) {
    $env:PREV_SETUP_EXE_PATH = Get-Podman-Setup-From-GitHub -version "tags/v5.3.0"
}

# Note: consumes podman-remote-release-windows_amd64.zip from repo.tar.zst
Run-Command ".\build.ps1 $Env:WIN_INST_VER dev `"$RELEASE_DIR`""

# Build a v9.9.10 installer to test an update from current to next version
$NEXT_WIN_INST_VER="9.9.10"
Run-Command ".\build.ps1 `"$NEXT_WIN_INST_VER`" dev `"$RELEASE_DIR`""

Pop-Location

# Run the installer silently and WSL/HyperV install options disabled (prevent reboots)
$command = "$WIN_INST_FOLDER\test-installer.ps1 "
$command += "-scenario all "
$command += "-provider $ENV:CONTAINERS_MACHINE_PROVIDER "
$command += "-setupExePath `"$WIN_INST_FOLDER\podman-$ENV:WIN_INST_VER-dev-setup.exe`""
$command += "-previousSetupExePath `"$env:PREV_SETUP_EXE_PATH`""
$command += "-nextSetupExePath `"$WIN_INST_FOLDER\podman-$NEXT_WIN_INST_VER-dev-setup.exe`""
$command += "-v531SetupExePath `"$env:V531_SETUP_EXE_PATH`""
Run-Command "${command}"
