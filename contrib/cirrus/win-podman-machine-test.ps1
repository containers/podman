#!/usr/bin/env powershell

. $PSScriptRoot\win-lib.ps1

Write-Host 'Recovering env. vars.'
Import-CLIXML "$ENV:TEMP\envars.xml" | ForEach-Object {
    Write-Host "    $($_.Name) = $($_.Value)"
    Set-Item "Env:$($_.Name)" "$($_.Value)"
}

if ($Env:TEST_FLAVOR -eq 'machine-wsl') {
    # FIXME: Test-modes should be definitively set and positively asserted.
    # Otherwise if the var. goes out-of-scope, defaults change, or definition
    # fails: Suddenly assumed behavior != actual behaviorr, esp. if/when only
    # quickly glancing at a green status check-mark.
    $Env:CONTAINERS_MACHINE_PROVIDER = ''  # IMPLIES WSL
}
elseif ($Env:TEST_FLAVOR -eq 'machine-hyperv') {
    $Env:CONTAINERS_MACHINE_PROVIDER = 'hyperv'
}
else {
    Write-Host "Unsupported value for `$TEST_FLAVOR '$Env:TEST_FLAVOR'"
    Exit 1
}
# Make sure an observer knows the value of this critical variable (consumed by tests).
Write-Host "    CONTAINERS_MACHINE_PROVIDER = $Env:CONTAINERS_MACHINE_PROVIDER"
Write-Host "`n"

# The repo.tar.zst artifact was extracted here
Set-Location "$ENV:CIRRUS_WORKING_DIR\repo"
# Tests hard-code this location for podman-remote binary, make sure it actually runs.
Run-Command '.\bin\windows\podman.exe --version'

# Add policy.json to filesystem for podman machine pulls
New-Item -ItemType 'directory' -Path "$env:AppData\containers"
Copy-Item -Path pkg\machine\ocipull\policy.json -Destination "$env:AppData\containers"

# Set TMPDIR to fast storage, see cirrus.yml setup_disk_script for setup Z:\
# TMPDIR is used by the machine tests paths, while TMP and TEMP are the normal
# windows temporary dirs. Just to ensure everything uses the fast disk.
$Env:TMPDIR = 'Z:\'
$Env:TMP = 'Z:\'
$Env:TEMP = 'Z:\'

Write-Host "`nRunning podman-machine e2e tests"

if ($Env:TEST_FLAVOR -eq 'machine-wsl') {
    if ($Env:CIRRUS_CI -eq 'true') {
        # Add a WSL configuration file
        # The `kernelBootTimeout` configuration is to prevent CI/CD flakes
        # See
        # https://github.com/microsoft/WSL/issues/13301#issuecomment-3367452109
        Write-Host "`nAdding WSL configuration file"
        $wslConfigPath = "$env:UserProfile\.wslconfig"
        $wslConfigContent = @'
[wsl2]
kernelCommandLine=WSL_DEBUG=hvsocket WSL_SOCKET_LOG=1
kernelBootTimeout=300000 # 5 minutes
'@
        Set-Content -Path $wslConfigPath -Value $wslConfigContent -Encoding utf8
        wsl --shutdown
        Write-Host "`n$wslConfigPath content:"
        Get-Content -Path $wslConfigPath
    }

    # Output info so we know what version we are testing.
    Write-Host "`nUpdating WSL version to pre-release:"
    (New-Object System.Net.WebClient).DownloadFile("https://wslstorestorage.blob.core.windows.net/wslblob/wsl.2.6.2.2.x64.msi", "wsl.msi")
    Start-Process "msiexec.exe" -ArgumentList "/i wsl.msi /quiet"  -Wait
    Write-Host "`nOutputting WSL version:"
    wsl --version
    # Run-Command "$PSScriptRoot\win-collect-wsl-logs-start.ps1"
    Run-Command "$PSScriptRoot\win-collect-wsl-logs-start.ps1 -LogProfile hvsocket"
}

try {
    Run-Command '.\winmake localmachine'
}
finally {
    if ($Env:TEST_FLAVOR -eq 'machine-wsl') {
        Run-Command "$PSScriptRoot\win-collect-wsl-logs-stop.ps1"
    }
}
