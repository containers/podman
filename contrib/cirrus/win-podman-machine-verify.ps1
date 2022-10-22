# Powershell doesn't exit after command failures
# Note, due to a bug in cirrus that does not correctly evaluate exit code,
# errors conditions should always be thrown
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
Expand-Archive -Path "podman-remote-release-windows_amd64.zip" -DestinationPath extracted
Set-Location extracted
$x = Get-ChildItem -Path bin -Recurse
Set-Location $x

# Verify extracted podman binary
Write-Output "Starting init..."
.\podman machine init; CheckExit
Write-Output "Starting podman machine..."
.\podman machine start; CheckExit
for ($i =0; $i -lt 60; $i++) {
    .\podman info
    if ($LASTEXITCODE -eq 0) {
        break
    }
    Start-Sleep -Seconds 2
}
Write-Output "Running container..."
.\podman run ubi8-micro sh -c "exit 123"
if ($LASTEXITCODE -ne 123) {
    throw  "Expected 123, got $LASTEXITCODE"
}
Exit 0
