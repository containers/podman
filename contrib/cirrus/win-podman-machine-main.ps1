#!/usr/bin/env powershell

. $PSScriptRoot\win-lib.ps1

Set-Location "$ENV:CIRRUS_WORKING_DIR\repo"

$GvTargetDir = "C:\Program Files\Redhat\Podman\"

#Expand-Archive -Path "podman-remote-release-windows_amd64.zip" -DestinationPath $GvTargetDir

New-Item -Path $GvTargetDir -ItemType "directory"
Copy-Item "bin/windows/gvproxy.exe" -Destination $GvTargetDir

Write-Host "Saving selection of CI env. vars."
# Env. vars will not pass through win-sess-launch.ps1
Get-ChildItem -Path "Env:\*" -include @("PATH", "Chocolatey*", "CIRRUS*", "TEST_*", "CI_*") `
  | Export-CLIXML "$ENV:TEMP\envars.xml"

# Recent versions of WSL are packaged as a Windows store app running in
# an appX container, which is incompatible with non-interactive
# session 0 execution (where the cirrus agent runs).
# Run verification under an interactive session instead.
Write-Host "Spawning new session to execute $PSScriptRoot\win-podman-machine-test.ps1"
# Can't use Run-Command(), would need overly-complex nested quoting
powershell.exe -File "$PSScriptRoot\win-sess-launch.ps1" `
                     "$PSScriptRoot\win-podman-machine-test.ps1"
Check-Exit
