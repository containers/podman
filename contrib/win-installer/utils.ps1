#!/usr/bin/env pwsh
function Get-Latest-Podman-Setup-From-GitHub {
    return Get-Podman-Setup-From-GitHub "latest"
}

function Get-Podman-Setup-From-GitHub {
    param(
        [Parameter(Mandatory)]
        [string] $version
    )

    Write-Host "Downloading the $version Podman windows setup from GitHub..."
    $apiUrl = "https://api.github.com/repos/containers/podman/releases/$version"
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
