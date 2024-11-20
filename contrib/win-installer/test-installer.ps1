#!/usr/bin/env pwsh

# Example usage:
# rm .\contrib\win-installer\*.log &&
# rm .\contrib\win-installer\*.exe &&
# rm .\contrib\win-installer\*.wixpdb &&
# .\winmake.ps1 installer 9.9.9 &&
# .\contrib\win-installer\test-installer.ps1 `
#     -scenario update-without-user-changes `
#     -setupExePath ".\contrib\win-installer\podman-9.9.9-dev-setup.exe" `
#     -provider hyperv

# The Param statement must be the first statement, except for comments and any #Require statements.
param (
    [Parameter(Mandatory)]
    [ValidateSet("test-objects-exist", "test-objects-exist-not", "installation-green-field", "installation-skip-config-creation-flag", "installation-with-pre-existing-podman-exe", "update-without-user-changes", "update-with-user-changed-config-file", "update-with-user-removed-config-file", "all")]
    [string]$scenario,
    [ValidateScript({Test-Path $_ -PathType Leaf})]
    [string]$setupExePath,
    [ValidateScript({Test-Path $_ -PathType Leaf})]
    [string]$previousSetupExePath,
    [ValidateSet("wsl", "hyperv")]
    [string]$provider="wsl",
    [switch]$installWSL=$false,
    [switch]$installHyperV=$false,
    [switch]$skipWinVersionCheck=$false,
    [switch]$skipConfigFileCreation=$false
)

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
                                /log $PSScriptRoot\podman-setup.log"
    if ($ret.ExitCode -ne 0) {
        Write-Host "Install failed, dumping log"
        Get-Content $PSScriptRoot\podman-setup.log
        throw "Exit code is $($ret.ExitCode)"
    }
    Write-Host "Installation completed successfully!`n"
}

function Install-Previous-Podman {
    Install-Podman -setupExePath $previousSetupExePath
}

function Install-Previous-Podman-With-Defaults {
    Install-Podman-With-Defaults -setupExePath $previousSetupExePath
}

function Install-Current-Podman {
    Install-Podman -setupExePath $setupExePath
}

function Install-Current-Podman-With-Defaults {
    Install-Podman-With-Defaults -setupExePath $setupExePath
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

function Uninstall-Previous-Podman {
    Uninstall-Podman -setupExePath $previousSetupExePath
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

function Get-Latest-Podman-Setup-From-GitHub {
    Write-Host "Downloading the latest Podman windows setup from GitHub..."
    $apiUrl = "https://api.github.com/repos/containers/podman/releases/latest"
    $response = Invoke-RestMethod -Uri $apiUrl -Headers @{"User-Agent"="PowerShell"} -ErrorAction Stop
    $downloadUrl = $response.assets[0].browser_download_url
    Write-Host "Downloading URL: $downloadUrl"
    $latestTag = $response.tag_name
    $destinationPath = "$PSScriptRoot\podman-$latestTag-setup.exe"
    Write-Host "Destination Path: $destinationPath"
    Invoke-WebRequest -Uri $downloadUrl -OutFile $destinationPath
    Write-Host "Command completed successfully!`n"
    return $destinationPath
}

# SCENARIOS
function Start-Scenario-Installation-Green-Field {
    Write-Host "`n==========================================="
    Write-Host " Running scenario: Installation-Green-Field"
    Write-Host "==========================================="
    Install-Current-Podman
    Test-Podman-Objects-Exist
    Test-Podman-Machine-Conf-Exist
    Test-Podman-Machine-Conf-Content
    Uninstall-Current-Podman
    Test-Podman-Objects-Exist-Not
    Test-Podman-Machine-Conf-Exist-Not
}

function Start-Scenario-Installation-Skip-Config-Creation-Flag {
    Write-Host "`n========================================================="
    Write-Host " Running scenario: Installation-Skip-Config-Creation-Flag"
    Write-Host "========================================================="
    $skipConfigFileCreation = $true
    Install-Current-Podman
    Test-Podman-Objects-Exist
    Test-Podman-Machine-Conf-Exist-Not
    Uninstall-Current-Podman
    Test-Podman-Objects-Exist-Not
    Test-Podman-Machine-Conf-Exist-Not
}

function Start-Scenario-Installation-With-Pre-Existing-Podman-Exe {
    Write-Host "`n============================================================"
    Write-Host " Running scenario: Installation-With-Pre-Existing-Podman-Exe"
    Write-Host "============================================================"
    New-Fake-Podman-Exe
    Install-Current-Podman
    Test-Podman-Objects-Exist
    Test-Podman-Machine-Conf-Exist-Not
    Uninstall-Current-Podman
    Test-Podman-Objects-Exist-Not
    Test-Podman-Machine-Conf-Exist-Not
}

function Start-Scenario-Update-Without-User-Changes {
    Write-Host "`n=============================================="
    Write-Host " Running scenario: Update-Without-User-Changes"
    Write-Host "=============================================="
    Install-Previous-Podman
    Test-Podman-Objects-Exist
    Test-Podman-Machine-Conf-Exist
    Test-Podman-Machine-Conf-Content
    Install-Current-Podman-With-Defaults
    Test-Podman-Objects-Exist
    Test-Podman-Machine-Conf-Exist
    Test-Podman-Machine-Conf-Content
    Uninstall-Current-Podman
    Test-Podman-Objects-Exist-Not
    Test-Podman-Machine-Conf-Exist-Not
}

function Start-Scenario-Update-With-User-Changed-Config-File {
    Write-Host "`n======================================================="
    Write-Host " Running scenario: Update-With-User-Changed-Config-File"
    Write-Host "======================================================="
    Install-Previous-Podman
    Test-Podman-Objects-Exist
    Test-Podman-Machine-Conf-Exist
    Test-Podman-Machine-Conf-Content
    $newProvider = Switch-Podman-Machine-Conf-Content
    Install-Current-Podman-With-Defaults
    Test-Podman-Objects-Exist
    Test-Podman-Machine-Conf-Exist
    Test-Podman-Machine-Conf-Content -expected $newProvider
    Uninstall-Current-Podman
    Test-Podman-Objects-Exist-Not
    Test-Podman-Machine-Conf-Exist-Not
}

function Start-Scenario-Update-With-User-Removed-Config-File {
    Write-Host "`n======================================================="
    Write-Host " Running scenario: Update-With-User-Removed-Config-File"
    Write-Host "======================================================="
    Install-Previous-Podman
    Test-Podman-Objects-Exist
    Test-Podman-Machine-Conf-Exist
    Test-Podman-Machine-Conf-Content
    Remove-Podman-Machine-Conf
    Install-Current-Podman-With-Defaults
    Test-Podman-Objects-Exist
    Test-Podman-Machine-Conf-Exist-Not
    Uninstall-Current-Podman
    Test-Podman-Objects-Exist-Not
    Test-Podman-Machine-Conf-Exist-Not
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
    'update-with-user-changed-config-file' {
        if (!$previousSetupExePath) {
            $previousSetupExePath = Get-Latest-Podman-Setup-From-GitHub
        }
        Start-Scenario-Update-With-User-Changed-Config-File
    }
    'update-with-user-removed-config-file' {
        if (!$previousSetupExePath) {
            $previousSetupExePath = Get-Latest-Podman-Setup-From-GitHub
        }
        Start-Scenario-Update-With-User-Removed-Config-File
    }
    'all' {
        if (!$previousSetupExePath) {
            $previousSetupExePath = Get-Latest-Podman-Setup-From-GitHub
        }
        Start-Scenario-Installation-Green-Field
        Start-Scenario-Installation-Skip-Config-Creation-Flag
        Start-Scenario-Installation-With-Pre-Existing-Podman-Exe
        Start-Scenario-Update-Without-User-Changes
        Start-Scenario-Update-With-User-Changed-Config-File
        Start-Scenario-Update-With-User-Removed-Config-File
    }
}
