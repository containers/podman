$ErrorActionPreference = 'Stop'

# Powershell doesn't exit after command failures
# Note, due to a bug in cirrus that does not correctly evaluate exit
# code, error conditions should always be thrown
function CheckExit {
    if ($LASTEXITCODE -ne 0) {
        throw "Exit code failure = $LASTEXITCODE"
    }
}

# Drop global envs which have unix paths, defaults are fine
Remove-Item Env:\GOPATH
Remove-Item Env:\GOSRC
Remove-Item Env:\GOCACHE

mkdir tmp
Set-Location tmp

# Download and extract alt_build win release zip
$url = "${ENV:ART_URL}/Windows%20Cross/repo/repo.tbz"
Write-Output "URL: $url"
# Arc requires extension to be "tbz2"
curl.exe -L -o repo.tbz2 "$url"; CheckExit
arc unarchive repo.tbz2 .; CheckExit
Set-Location repo
Expand-Archive -Path "podman-remote-release-windows_amd64.zip" `
               -DestinationPath extracted
Set-Location extracted
$x = Get-ChildItem -Path bin -Recurse
Set-Location $x

# Recent versions of WSL are packaged as a Windows store app running in
# an appX container, which is incompatible with non-interactive
# session 0 execution (where the cirrus agent runs).
# Run verification under an interactive session instead.
powershell.exe -File "$PSScriptRoot\wsl-env-launch.ps1" `
                     "$PSScriptRoot\win-podman-machine-verify.ps1"
CheckExit
