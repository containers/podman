#!/usr/bin/env powershell

. $PSScriptRoot\win-lib.ps1

if ($Env:CI -eq "true") {
    Push-Location "$ENV:CIRRUS_WORKING_DIR\repo"
} else {
    Push-Location $PSScriptRoot\..\..
}

Run-Command ".\winmake.ps1 localunit"

Pop-Location
