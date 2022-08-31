function Copy-Artifact {
    param(
        [Parameter(Mandatory)]
        [string]$fileName
    )
    $file = Get-ChildItem -Recurse -Path . -Name $fileName
    if (!$file) {
        throw "Could not find $filename"
    }
    Write-Host "file:" $file
    Copy-Item -Path $file -Destination "..\artifacts\$filename" -ErrorAction Stop
}

function DownloadOrSkip {
    param(
        [Parameter(Mandatory)]
        [string]$url,
        [Parameter(Mandatory)]
        [string]$file
    )
    $ProgressPreference = 'SilentlyContinue';
    try {
        Invoke-WebRequest -UseBasicParsing -ErrorAction Stop -Uri $url -OutFile $file
    } Catch {
        if ($_.Exception.Response.StatusCode -eq 404) {
            Write-Host "URL not available, signaling skip:"
            Write-Host "URL: $url"
            Exit 2
        }

        throw $_.Exception
    }
}

function DownloadOptional {
    param(
        [Parameter(Mandatory)]
        [string]$url,
        [Parameter(Mandatory)]
        [string]$file
    )
    $ProgressPreference = 'SilentlyContinue';
    try {
        Invoke-WebRequest -UseBasicParsing -ErrorAction Stop -Uri $url -OutFile $file
    } Catch {
    }

    Return
}


if ($args.Count -lt 1) {
    Write-Host "Usage: " $MyInvocation.MyCommand.Name "<version> [release_dir]"
    Exit 1
}

$releaseDir = ""
if ($args.Count -gt 1 -and $args[1].Length -gt 0) {
    $path = $args[1]
    $releaseDir = (Resolve-Path -Path "$path" -ErrorAction Stop).Path
}


$base_url = "$ENV:FETCH_BASE_URL"
if ($base_url.Length -le 0) {
    $base_url = "https://github.com/containers/podman"
}

$version = $args[0]
if ($version -notmatch '^v?([0-9]+\.[0-9]+\.[0-9]+)(-.*)?$') {
    Write-Host "Invalid version"
    Exit 1
}

# WiX burn requires a QWORD version only, numeric only
$Env:INSTVER=$Matches[1]

if ($version[0] -ne 'v') {
    $version = 'v' + $version
}

$restore = 0
$exitCode = 0

try {
    Write-Host "Cleaning up old artifacts"
    Remove-Item -Force -Recurse -Path .\docs -ErrorAction SilentlyContinue | Out-Null
    Remove-Item -Force -Recurse -Path .\artifacts -ErrorAction SilentlyContinue | Out-Null
    Remove-Item -Force -Recurse -Path .\fetch -ErrorAction SilentlyContinue | Out-Null

    New-Item fetch -ItemType Directory | Out-Null
    New-Item artifacts -ItemType Directory | Out-Null

    Write-Host "Fetching zip release"

    Push-Location fetch -ErrorAction Stop
    $restore = 1
    $ProgressPreference = 'SilentlyContinue';

    if ($releaseDir.Length -gt 0) {
        Copy-Item -Path "$releaseDir/podman-remote-release-windows_amd64.zip" "release.zip"
    } else {
        DownloadOrSkip "$base_url/releases/download/$version/podman-remote-release-windows_amd64.zip"  "release.zip"
        DownloadOptional "$base_url/releases/download/$version/shasums" ..\shasums
    }
    Expand-Archive -Path release.zip
    $loc = Get-ChildItem -Recurse -Path . -Name win-sshproxy.exe
    if (!$loc) {
        if ($releaseDir.Length -gt 0) {
            throw "Release dir only supports zip which includes win-sshproxy.exe"
        }
        Write-Host "Old release, zip does not include win-sshproxy.exe, fetching via msi"
        DownloadOrSkip "$base_url/releases/download/$version/podman-$version.msi" "podman.msi"
        dark -x expand ./podman.msi
        if (!$?) {
            throw "Dark command failed"
        }
        $loc = Get-ChildItem -Recurse -Path expand -Name 4A2AD125-34E7-4BD8-BE28-B2A9A5EDBEB5
        if (!$loc) {
            throw "Could not obtain win-sshproxy.exe"
        }
        Copy-Item -Path "expand\$loc" -Destination "win-sshproxy.exe" -ErrorAction Stop
        Remove-Item -Recurse -Force -Path expand
    }

    Write-Host "Copying artifacts"
    Foreach ($fileName in "win-sshproxy.exe", "podman.exe") {
        Copy-Artifact($fileName)
    }

    $docsloc = Get-ChildItem -Path . -Name docs -Recurse
    $loc = Get-ChildItem -Recurse -Path . -Name podman-for-windows.html
    if (!$loc) {
        Write-Host "Old release did not include welcome page, using podman-machine instead"
        $loc = Get-ChildItem -Recurse -Path . -Name podman-machine.html
        Copy-Item -Path $loc -Destination "$docsloc\podman-for-windows.html"
    }

    Write-Host "Copying docs"
    Copy-Item -Recurse -Path $docsloc -Destination ..\docs -ErrorAction Stop
    Write-Host "Done!"

    if (!$loc) {
        throw "Could not find docs"
    }
}
catch {
    Write-Host $_

    $exitCode = 1
}
finally {
    if ($restore) {
        Pop-Location
    }
}

exit $exitCode
