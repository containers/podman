#!/usr/bin/env powershell

. $PSScriptRoot\win-lib.ps1

Write-Host "Recovering env. vars."
Import-CLIXML "$ENV:TEMP\envars.xml" | % {
    Write-Host "    $($_.Name) = $($_.Value)"
    Set-Item "Env:$($_.Name)" "$($_.Value)"
}

if ($Env:TEST_FLAVOR -eq "machine-wsl") {
    # FIXME: Test-modes should be definitively set and positively asserted.
    # Otherwise if the var. goes out-of-scope, defaults change, or definition
    # fails: Suddenly assumed behavior != actual behaviorr, esp. if/when only
    # quickly glancing at a green status check-mark.
    $Env:CONTAINERS_MACHINE_PROVIDER = ""  # IMPLIES WSL
} elseif ($Env:TEST_FLAVOR -eq "machine-hyperv") {
    $Env:CONTAINERS_MACHINE_PROVIDER = "hyperv"
} else {
    Write-Host "Unsupported value for `$TEST_FLAVOR '$Env:TEST_FLAVOR'"
    Exit 1
}
# Make sure an observer knows the value of this critical variable (consumed by tests).
Write-Host "    CONTAINERS_MACHINE_PROVIDER = $Env:CONTAINERS_MACHINE_PROVIDER"
Write-Host "`n"

# The repo.tar.zst artifact was extracted here
Set-Location "$ENV:CIRRUS_WORKING_DIR\repo"
# Tests hard-code this location for podman-remote binary, make sure it actually runs.
Run-Command ".\bin\windows\podman.exe --version"

# Add policy.json to filesystem for podman machine pulls
New-Item -ItemType "directory" -Path "$env:AppData\containers"
Copy-Item -Path pkg\machine\ocipull\policy.json -Destination "$env:AppData\containers"

Write-Host "`nRunning podman-machine e2e tests"
Run-Command ".\winmake localmachine"
