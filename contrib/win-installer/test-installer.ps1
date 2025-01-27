#!/usr/bin/env pwsh

# Usage example:
#
# rm .\contrib\win-installer\*.log &&
# rm .\contrib\win-installer\*.exe &&
# rm .\contrib\win-installer\*.wixpdb &&
# .\winmake.ps1 installer &&
# .\winmake.ps1 installer 9.9.9 &&
# .\contrib\win-installer\test-installer.ps1 `
#     -scenario all `
#     -previousSetupExePath ".\contrib\win-installer\podman-5.3.0-dev-setup.exe" `
#     -setupExePath ".\contrib\win-installer\podman-5.4.0-dev-setup.exe" `
#     -nextSetupExePath ".\contrib\win-installer\podman-9.9.9-dev-setup.exe" `
#     -provider hyperv
#

# The Param statement must be the first statement, except for comments and any #Require statements.
param (
    [Parameter(Mandatory)]
    [ValidateSet("test-objects-exist", "test-objects-exist-not", "installation-green-field", "installation-skip-config-creation-flag", "installation-with-pre-existing-podman-exe",
                 "update-without-user-changes", "update-with-user-changed-config-file", "update-with-user-removed-config-file",
                 "update-without-user-changes-to-next", "update-with-user-changed-config-file-to-next", "update-with-user-removed-config-file-to-next",
                 "update-without-user-changes-from-531",
                 "all")]
    [string]$scenario,
    [ValidateScript({Test-Path $_ -PathType Leaf})]
    [string]$setupExePath,
    [ValidateScript({Test-Path $_ -PathType Leaf})]
    [string]$previousSetupExePath,
    [ValidateScript({Test-Path $_ -PathType Leaf})]
    [string]$nextSetupExePath,
    [ValidateSet("wsl", "hyperv")]
    [string]$provider="wsl",
    [switch]$installWSL=$false,
    [switch]$installHyperV=$false,
    [switch]$skipWinVersionCheck=$false,
    [switch]$skipConfigFileCreation=$false
)

. $PSScriptRoot\utils.ps1

$MachineConfPath = "$env:ProgramData\containers\containers.conf.d\99-podman-machine-provider.conf"
$PodmanFolderPath = "$env:ProgramFiles\RedHat\Podman"
$PodmanExePath = "$PodmanFolderPath\podman.exe"
$WindowsPathsToTest = @($PodmanExePath,
                        "$PodmanFolderPath\win-sshproxy.exe",
                        "HKLM:\SOFTWARE\Red Hat\Podman")

function Install-Podman {
    param (
        [Parameter(Mandatory)]
        [ValidateScript({Test-Path $_ -PathType Leaf})]
        [string]$setupExePath
    )
    if ($installWSL) {$wslCheckboxVar = "1"} else {$wslCheckboxVar = "0"}
    if ($installHyperV) {$hypervCheckboxVar = "1"} else {$hypervCheckboxVar = "0"}
    if ($skipWinVersionCheck) {$allowOldWinVar = "1"} else {$allowOldWinVar = "0"}
    if ($skipConfigFileCreation) {$skipConfigFileCreationVar = "1"} else {$skipConfigFileCreationVar = "0"}

    Write-Host "Running the installer ($setupExePath)..."
    Write-Host "(provider=`"$provider`", WSLCheckbox=`"$wslCheckboxVar`", HyperVCheckbox=`"$hypervCheckboxVar`", AllowOldWin=`"$allowOldWinVar`", SkipConfigFileCreation=`"$skipConfigFileCreationVar`")"
    $ret = Start-Process -Wait `
                            -PassThru "$setupExePath" `
                            -ArgumentList "/install /quiet `
                                MachineProvider=${provider} `
                                WSLCheckbox=${wslCheckboxVar} `
                                HyperVCheckbox=${hypervCheckboxVar} `
                                AllowOldWin=${allowOldWinVar} `
                                SkipConfigFileCreation=${skipConfigFileCreationVar} `
                                /log $PSScriptRoot\podman-setup.log"
    if ($ret.ExitCode -ne 0) {
        Write-Host "Install failed, dumping log"
        Get-Content $PSScriptRoot\podman-setup.log
        throw "Exit code is $($ret.ExitCode)"
    }
    Write-Host "Installation completed successfully!`n"
}

# Install-Podman-With-Defaults is used to test updates. That's because when
# using the installer GUI the user can't change the default values.
function Install-Podman-With-Defaults {
    param (
        [Parameter(Mandatory)]
        [ValidateScript({Test-Path $_ -PathType Leaf})]
        [string]$setupExePath
    )

    Write-Host "Running the installer using defaults ($setupExePath)..."
    $ret = Start-Process -Wait `
                            -PassThru "$setupExePath" `
                            -ArgumentList "/install /quiet `
                                /log $PSScriptRoot\podman-setup-default.log"
    if ($ret.ExitCode -ne 0) {
        Write-Host "Install failed, dumping log"
        Get-Content $PSScriptRoot\podman-setup-default.log
        throw "Exit code is $($ret.ExitCode)"
    }
    Write-Host "Installation completed successfully!`n"
}
function Install-Podman-With-Defaults-Expected-Fail {
    param (
        [Parameter(Mandatory)]
        [ValidateScript({Test-Path $_ -PathType Leaf})]
        [string]$setupExePath
    )

    Write-Host "Running the installer using defaults ($setupExePath)..."
    $ret = Start-Process -Wait `
                            -PassThru "$setupExePath" `
                            -ArgumentList "/install /quiet `
                                /log $PSScriptRoot\podman-setup-default.log"
    if ($ret.ExitCode -eq 0) {
        Write-Host "Install completed successfully but a failure was expected, dumping log"
        Get-Content $PSScriptRoot\podman-setup-default.log
        throw "Exit code is $($ret.ExitCode)"
    }
    Write-Host "Installation has failed as expected!`n"
}

function Install-Current-Podman {
    Install-Podman -setupExePath $setupExePath
}

function Test-Podman-Objects-Exist {
    Write-Host "Verifying that podman files, folders and registry entries exist..."
    $WindowsPathsToTest | ForEach-Object {
        if (! (Test-Path -Path $_) ) {
            throw "Expected $_ but doesn't exist"
        }
    }
    Write-Host "Verification was successful!`n"
}

function Test-Podman-Machine-Conf-Exist {
    Write-Host "Verifying that $MachineConfPath exist..."
    if (! (Test-Path -Path $MachineConfPath) ) {
        throw "Expected $MachineConfPath but doesn't exist"
    }
    Write-Host "Verification was successful!`n"
}

function Test-Podman-Machine-Conf-Content {
    param (
        [ValidateSet("wsl", "hyperv")]
        [string]$expected=$provider
    )
    Write-Host "Verifying that the machine provider configuration is correct..."
    $machineProvider = Get-Content $MachineConfPath | Select-Object -Skip 1 | ConvertFrom-StringData | ForEach-Object { $_.provider }
    if ( $machineProvider -ne "`"$expected`"" ) {
        throw "Expected `"$expected`" as default machine provider but got $machineProvider"
    }
    Write-Host "Verification was successful!`n"
}

function Uninstall-Podman {
    param (
        # [Parameter(Mandatory)]
        [ValidateScript({Test-Path $_ -PathType Leaf})]
        [string]$setupExePath
    )
    Write-Host "Running the uninstaller ($setupExePath)..."
    $ret = Start-Process -Wait `
                         -PassThru "$setupExePath" `
                         -ArgumentList "/uninstall `
                         /quiet /log $PSScriptRoot\podman-setup-uninstall.log"
    if ($ret.ExitCode -ne 0) {
        Write-Host "Uninstall failed, dumping log"
        Get-Content $PSScriptRoot\podman-setup-uninstall.log
        throw "Exit code is $($ret.ExitCode)"
    }
    Write-Host "The uninstallation completed successfully!`n"
}

function Uninstall-Current-Podman {
    Uninstall-Podman -setupExePath $setupExePath
}

function Test-Podman-Objects-Exist-Not {
    Write-Host "Verifying that podman files, folders and registry entries don't exist..."
    $WindowsPathsToTest | ForEach-Object {
        if ( Test-Path -Path $_ ) {
            throw "Path $_ is present"
        }
    }
    Write-Host "Verification was successful!`n"
}

function Test-Podman-Machine-Conf-Exist-Not {
    Write-Host "Verifying that $MachineConfPath doesn't exist..."
    if ( Test-Path -Path $MachineConfPath ) {
        throw "Path $MachineConfPath is present"
    }
    Write-Host "Verification was successful!`n"
}

function New-Fake-Podman-Exe {
    Write-Host "Creating a fake $PodmanExePath..."
    New-Item -ItemType Directory -Path $PodmanFolderPath -Force -ErrorAction Stop | out-null
    New-Item -ItemType File -Path $PodmanExePath -ErrorAction Stop | out-null
    Write-Host "Creation successful!`n"
}

function Switch-Podman-Machine-Conf-Content {
    $currentProvider = $provider
    if ( $currentProvider -eq "wsl" ) { $newProvider = "hyperv" } else { $newProvider = "wsl" }
    Write-Host "Editing $MachineConfPath content (was $currentProvider, will be $newProvider)..."
    "[machine]`nprovider=`"$newProvider`"" | Out-File -FilePath $MachineConfPath -ErrorAction Stop
    Write-Host "Edit successful!`n"
    return $newProvider
}

function Remove-Podman-Machine-Conf {
    Write-Host "Deleting $MachineConfPath..."
    Remove-Item -Path $MachineConfPath -ErrorAction Stop | out-null
    Write-Host "Deletion successful!`n"
}

function Test-Installation {
    param (
        [ValidateSet("wsl", "hyperv")]
        [string]$expectedConf
    )
    Test-Podman-Objects-Exist
    Test-Podman-Machine-Conf-Exist
    if ($expectedConf) {
        Test-Podman-Machine-Conf-Content -expected $expectedConf
    } else {
        Test-Podman-Machine-Conf-Content
    }
}

function Test-Installation-No-Config {
    Test-Podman-Objects-Exist
    Test-Podman-Machine-Conf-Exist-Not
}

function Test-Uninstallation {
    Test-Podman-Objects-Exist-Not
    Test-Podman-Machine-Conf-Exist-Not
}

# SCENARIOS
function Start-Scenario-Installation-Green-Field {
    Write-Host "`n==========================================="
    Write-Host " Running scenario: Installation-Green-Field"
    Write-Host "==========================================="
    Install-Current-Podman
    Test-Installation
    Uninstall-Current-Podman
    Test-Uninstallation
}

function Start-Scenario-Installation-Skip-Config-Creation-Flag {
    Write-Host "`n========================================================="
    Write-Host " Running scenario: Installation-Skip-Config-Creation-Flag"
    Write-Host "========================================================="
    $skipConfigFileCreation = $true
    Install-Current-Podman
    Test-Installation-No-Config
    Uninstall-Current-Podman
    Test-Uninstallation
}

function Start-Scenario-Installation-With-Pre-Existing-Podman-Exe {
    Write-Host "`n============================================================"
    Write-Host " Running scenario: Installation-With-Pre-Existing-Podman-Exe"
    Write-Host "============================================================"
    New-Fake-Podman-Exe
    Install-Current-Podman
    Test-Installation-No-Config
    Uninstall-Current-Podman
    Test-Uninstallation
}

function Start-Scenario-Update-Without-User-Changes {
    param (
        [ValidateSet("From-Previous", "To-Next", "From-v531")]
        [string]$mode="From-Previous"
    )
    Write-Host "`n======================================================"
    Write-Host " Running scenario: Update-Without-User-Changes-$mode"
    Write-Host "======================================================"
    switch ($mode) {
        'From-Previous' {$i = $previousSetupExePath; $u = $setupExePath}
        'To-Next' {$i = $setupExePath; $u = $nextSetupExePath}
        'From-v531' {$i = $v531SetupExePath; $u = $setupExePath}
    }
    Install-Podman -setupExePath $i
    Test-Installation

    # Updates are expected to succeed except when updating from v5.3.1
    # The v5.3.1 installer has a bug that is patched in v5.3.2
    # Upgrading from v5.3.1 requires upgrading to v5.3.2 first
    if ($mode -eq "From-Previous" -or $mode -eq "To-Next") {
        Install-Podman-With-Defaults -setupExePath $u
        Test-Installation
        Uninstall-Podman -setupExePath $u
    } else { # From-v531 is expected to fail
        Install-Podman-With-Defaults-Expected-Fail -setupExePath $u
        Uninstall-Podman -setupExePath $i
    }
    Test-Uninstallation
}

function Start-Scenario-Update-Without-User-Changes-To-Next {
    Start-Scenario-Update-Without-User-Changes -mode "To-Next"
}

function Start-Scenario-Update-Without-User-Changes-From-v531 {
    Start-Scenario-Update-Without-User-Changes -mode "From-v531"
}

function Start-Scenario-Update-With-User-Changed-Config-File {
    param (
        [ValidateSet("From-Previous", "To-Next")]
        [string]$mode="From-Previous"
    )
    Write-Host "`n=============================================================="
    Write-Host " Running scenario: Update-With-User-Changed-Config-File-$mode"
    Write-Host "=============================================================="
    switch ($mode) {
        'From-Previous' {$i = $previousSetupExePath; $u = $setupExePath}
        'To-Next' {$i = $setupExePath; $u = $nextSetupExePath}
    }
    Install-Podman -setupExePath $i
    Test-Installation
    $newProvider = Switch-Podman-Machine-Conf-Content
    Install-Podman-With-Defaults -setupExePath $u
    Test-Installation -expectedConf $newProvider
    Uninstall-Podman -setupExePath $u
    Test-Uninstallation
}

function Start-Scenario-Update-With-User-Changed-Config-File-To-Next {
    Start-Scenario-Update-With-User-Changed-Config-File -mode "To-Next"
}

function Start-Scenario-Update-With-User-Removed-Config-File {
    param (
        [ValidateSet("From-Previous", "To-Next")]
        [string]$mode="From-Previous"
    )
    Write-Host "`n=============================================================="
    Write-Host " Running scenario: Update-With-User-Removed-Config-File-$mode"
    Write-Host "=============================================================="
    switch ($mode) {
        'From-Previous' {$i = $previousSetupExePath; $u = $setupExePath}
        'To-Next' {$i = $setupExePath; $u = $nextSetupExePath}
    }
    Install-Podman -setupExePath $i
    Test-Installation
    Remove-Podman-Machine-Conf
    Install-Podman-With-Defaults -setupExePath $u
    Test-Installation-No-Config
    Uninstall-Podman -setupExePath $u
    Test-Uninstallation
}

function Start-Scenario-Update-With-User-Removed-Config-File-To-Next {
    Start-Scenario-Update-With-User-Removed-Config-File -mode "To-Next"
}

switch ($scenario) {
    'test-objects-exist' {
        Test-Podman-Objects-Exist
    }
    'test-objects-exist-not' {
        Test-Podman-Objects-Exist-Not
    }
    'installation-green-field' {
        Start-Scenario-Installation-Green-Field
    }
    'installation-skip-config-creation-flag' {
        Start-Scenario-Installation-Skip-Config-Creation-Flag
    }
    'installation-with-pre-existing-podman-exe' {
        Start-Scenario-Installation-With-Pre-Existing-Podman-Exe
    }
    'update-without-user-changes' {
        if (!$previousSetupExePath) {
            $previousSetupExePath = Get-Latest-Podman-Setup-From-GitHub
        }
        Start-Scenario-Update-Without-User-Changes
    }
    'update-without-user-changes-to-next' {
        if (!$nextSetupExePath) {
            throw "Next version installer path is not defined. Use '-nextSetupExePath <setup-exe-path>' to define it."
        }
        Start-Scenario-Update-Without-User-Changes-To-Next
    }
    'update-without-user-changes-from-531' {
        if (!$v531SetupExePath) {
            $v531SetupExePath = Get-Podman-Setup-From-GitHub -version "tags/v5.3.1"
        }
        Start-Scenario-Update-Without-User-Changes-From-v531
    }
    'update-with-user-changed-config-file' {
        if (!$previousSetupExePath) {
            $previousSetupExePath = Get-Latest-Podman-Setup-From-GitHub
        }
        Start-Scenario-Update-With-User-Changed-Config-File
    }
    'update-with-user-changed-config-file-to-next' {
        if (!$nextSetupExePath) {
            throw "Next version installer path is not defined. Use '-nextSetupExePath <setup-exe-path>' to define it."
        }
        Start-Scenario-Update-With-User-Changed-Config-File-To-Next
    }
    'update-with-user-removed-config-file' {
        if (!$previousSetupExePath) {
            $previousSetupExePath = Get-Latest-Podman-Setup-From-GitHub
        }
        Start-Scenario-Update-With-User-Removed-Config-File
    }
    'update-with-user-removed-config-file-to-next' {
        if (!$nextSetupExePath) {
            throw "Next version installer path is not defined. Use '-nextSetupExePath <setup-exe-path>' to define it."
        }
        Start-Scenario-Update-With-User-Removed-Config-File-To-Next
    }
    'all' {
        if (!$nextSetupExePath) {
            throw "Next version installer path is not defined. Use '-nextSetupExePath <setup-exe-path>' to define it."
        }
        if (!$previousSetupExePath) {
            $previousSetupExePath = Get-Latest-Podman-Setup-From-GitHub
        }
        if (!$v531SetupExePath) {
            $v531SetupExePath = Get-Podman-Setup-From-GitHub -version "tags/v5.3.1"
        }
        Start-Scenario-Installation-Green-Field
        Start-Scenario-Installation-Skip-Config-Creation-Flag
        Start-Scenario-Installation-With-Pre-Existing-Podman-Exe
        Start-Scenario-Update-Without-User-Changes
        Start-Scenario-Update-Without-User-Changes-To-Next
        Start-Scenario-Update-With-User-Changed-Config-File
        Start-Scenario-Update-With-User-Changed-Config-File-To-Next
        Start-Scenario-Update-With-User-Removed-Config-File
        Start-Scenario-Update-With-User-Removed-Config-File-To-Next
        Start-Scenario-Update-Without-User-Changes-From-v531
    }
}
