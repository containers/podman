#!/usr/bin/env powershell

. $PSScriptRoot\win-lib.ps1

if ($Env:CI -eq "true") {
    $WIN_INST_FOLDER = "$ENV:CIRRUS_WORKING_DIR\repo\contrib\win-installer"
    $RELEASE_DIR = "$ENV:CIRRUS_WORKING_DIR\repo"
} else {
    $WIN_INST_FOLDER = "$PSScriptRoot\..\win-installer"
    $ENV:WIN_INST_VER = "9.9.9"
    $RELEASE_DIR = "$PSScriptRoot\..\.."
    $ENV:CONTAINERS_MACHINE_PROVIDER = "wsl"
}

$ConfFilePath = "$env:ProgramData\containers\containers.conf.d\99-podman-machine-provider.conf"
$WindowsPathsToTest = @("C:\Program Files\RedHat\Podman\podman.exe",
                  "C:\Program Files\RedHat\Podman\win-sshproxy.exe",
                  "$ConfFilePath",
                  "HKLM:\SOFTWARE\Red Hat\Podman")

Set-Location $WIN_INST_FOLDER

# Build Installer
# Note: consumes podman-remote-release-windows_amd64.zip from repo.tbz2
Run-Command ".\build.ps1 $Env:WIN_INST_VER dev `"$RELEASE_DIR`""

# Run the installer silently and WSL/HyperV install options disabled (prevent reboots)
# We need AllowOldWin=1 for server 2019 (cirrus image), can be dropped after server 2022
$ret = Start-Process -Wait -PassThru ".\podman-${ENV:WIN_INST_VER}-dev-setup.exe" -ArgumentList "/install /quiet MachineProvider=$ENV:CONTAINERS_MACHINE_PROVIDER WSLCheckbox=0 HyperVCheckbox=0 AllowOldWin=1 /log inst.log"
if ($ret.ExitCode -ne 0) {
    Write-Host "Install failed, dumping log"
    Get-Content inst.log
    throw "Exit code is $($ret.ExitCode)"
}
$WindowsPathsToTest | ForEach-Object {
    if (! (Test-Path -Path $_) ) {
        throw "Expected $_ but it's not present after uninstall"
    }
}
$machineProvider = Get-Content $ConfFilePath | Select-Object -Skip 1 | ConvertFrom-StringData | ForEach-Object { $_.provider }
if ( $machineProvider -ne "`"$ENV:CONTAINERS_MACHINE_PROVIDER`"" ) {
    throw "Expected `"$ENV:CONTAINERS_MACHINE_PROVIDER`" as default machine provider but got $machineProvider"
}

Write-Host "Installer verification successful!"

# Run the uninstaller silently to verify that it cleans up properly
$ret = Start-Process -Wait -PassThru ".\podman-${ENV:WIN_INST_VER}-dev-setup.exe" -ArgumentList "/uninstall /quiet /log uninst.log"
if ($ret.ExitCode -ne 0) {
    Write-Host "Uninstall failed, dumping log"
    Get-Content uninst.log
    throw "Exit code is $($ret.ExitCode)"
}
$WindowsPathsToTest | ForEach-Object {
    if ( Test-Path -Path $_ ) {
        throw "Path $_ is still present after uninstall"
    }
}

Write-Host "Uninstaller verification successful!"
