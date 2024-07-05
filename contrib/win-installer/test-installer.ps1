#!/usr/bin/env pwsh

# The Param statement must be the first statement, except for comments and any #Require statements.
param (
    [Parameter(Mandatory)]
    [ValidateSet("install", "uninstall", "all")]
    [string]$operation,
    [Parameter(Mandatory)]
    [ValidateScript({Test-Path $_ -PathType Leaf})]
    [string]$setupExePath,
    [ValidateSet("wsl", "hyperv")]
    [string]$provider="wsl",
    [switch]$installWSL=$false,
    [switch]$installHyperV=$false,
    [switch]$skipWinVersionCheck=$false
)

$ConfFilePath = "$env:ProgramData\containers\containers.conf.d\99-podman-machine-provider.conf"
$WindowsPathsToTest = @("C:\Program Files\RedHat\Podman\podman.exe",
                        "C:\Program Files\RedHat\Podman\win-sshproxy.exe",
                        "$ConfFilePath",
                        "HKLM:\SOFTWARE\Red Hat\Podman")

function Test-Installation {
    if ($installWSL) {$wslCheckboxVar = "1"} else {$wslCheckboxVar = "0"}
    if ($installHyperV) {$hypervCheckboxVar = "1"} else {$hypervCheckboxVar = "0"}
    if ($skipWinVersionCheck) {$allowOldWinVar = "1"} else {$allowOldWinVar = "0"}

    Write-Host "Running the installer (provider=`"$provider`")..."
    $ret = Start-Process -Wait `
                         -PassThru "$setupExePath" `
                         -ArgumentList "/install /quiet `
                                MachineProvider=${provider} `
                                WSLCheckbox=${wslCheckboxVar} `
                                HyperVCheckbox=${hypervCheckboxVar} `
                                AllowOldWin=${allowOldWinVar} `
                                /log $PSScriptRoot\podman-setup.log"
    if ($ret.ExitCode -ne 0) {
        Write-Host "Install failed, dumping log"
        Get-Content $PSScriptRoot\podman-setup.log
        throw "Exit code is $($ret.ExitCode)"
    }

    Write-Host "Verifying that the installer has created the expected files, folders and registry entries..."
    $WindowsPathsToTest | ForEach-Object {
        if (! (Test-Path -Path $_) ) {
            throw "Expected $_ but it's not present after uninstall"
        }
    }

    Write-Host "Verifying that the machine provider configuration is correct..."
    $machineProvider = Get-Content $ConfFilePath | Select-Object -Skip 1 | ConvertFrom-StringData | ForEach-Object { $_.provider }
    if ( $machineProvider -ne "`"$provider`"" ) {
        throw "Expected `"$provider`" as default machine provider but got $machineProvider"
    }

    Write-Host "The installation verification was successful!`n"
}

function Test-Uninstallation {
    Write-Host "Running the uninstaller"
    $ret = Start-Process -Wait `
                         -PassThru "$setupExePath" `
                         -ArgumentList "/uninstall `
                         /quiet /log $PSScriptRoot\podman-setup-uninstall.log"
    if ($ret.ExitCode -ne 0) {
        Write-Host "Uninstall failed, dumping log"
        Get-Content $PSScriptRoot\podman-setup-uninstall.log
        throw "Exit code is $($ret.ExitCode)"
    }

    Write-Host "Verifying that the uninstaller has removed files, folders and registry entries as expected..."
    $WindowsPathsToTest | ForEach-Object {
        if ( Test-Path -Path $_ ) {
            throw "Path $_ is still present after uninstall"
        }
    }

    Write-Host "The uninstallation verification was successful!`n"
}

switch ($operation) {
    'install' {
        Test-Installation
    }
    'uninstall' {
        Test-Uninstallation
    }
    'all' {
        Test-Installation
        Test-Uninstallation
    }
}
