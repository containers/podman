#!/usr/bin/env powershell

. $PSScriptRoot\win-lib.ps1

Set-Location "$ENV:CIRRUS_WORKING_DIR\repo\contrib\win-installer"

# Build Installer
# Note: consumes podman-remote-release-windows_amd64.zip from repo.tbz2
Run-Command ".\build.ps1 $Env:WIN_INST_VER dev `"$ENV:CIRRUS_WORKING_DIR\repo`""

# Run the installer silently and WSL install option disabled (prevent reboots, wsl requirements)
# We need AllowOldWin=1 for server 2019 (cirrus image), can be dropped after server 2022
$ret = Start-Process -Wait -PassThru ".\podman-${ENV:WIN_INST_VER}-dev-setup.exe" -ArgumentList "/install /quiet WSLCheckbox=0 AllowOldWin=1 /log inst.log"
if ($ret.ExitCode -ne 0) {
    Write-Host "Install failed, dumping log"
    Get-Content inst.log
    throw "Exit code is $($ret.ExitCode)"
}
if (! ((Test-Path -Path "C:\Program Files\RedHat\Podman\podman.exe") -and `
       (Test-Path -Path "C:\Program Files\RedHat\Podman\win-sshproxy.exe"))) {
    throw "Expected podman.exe and win-sshproxy.exe, one or both not present after install"
}
Write-Host "Installer verification successful!"
