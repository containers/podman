$ErrorActionPreference = 'Stop'
function CheckExit {
    if ($LASTEXITCODE -ne 0) {
        throw "Exit code failure = $LASTEXITCODE"
    }
}

# Verify extracted podman binary
Write-Output `n"Starting init...`n"
.\podman machine init; CheckExit
Write-Output "`nStarting podman machine...`n"
.\podman machine start; CheckExit
Write-Output "`nDumping info...`n"
for ($i =0; $i -lt 60; $i++) {
    .\podman info
    if ($LASTEXITCODE -eq 0) {
        break
    }
    Start-Sleep -Seconds 2
}
Write-Output "`nRunning container...`n"
.\podman run ubi8-micro sh -c "exit 123"
if ($LASTEXITCODE -ne 123) {
    throw  "Expected 123, got $LASTEXITCODE"
}
Write-Host "`nMachine verification is successful!`n"
Exit 0
