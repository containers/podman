function ExitOnError() {
    if ($LASTEXITCODE -ne 0) {
        Exit 1
    }
}

function FetchPanel() {
    Remove-Item -Recurse -Force -Path fetch -ErrorAction SilentlyContinue | Out-Null
    New-Item -Force -ItemType Directory fetch | Out-Null
    Push-Location fetch

    $ProgressPreference = 'SilentlyContinue'
    Invoke-WebRequest -UseBasicParsing -OutFile nuget.exe -ErrorAction Stop `
        -Uri https://dist.nuget.org/win-x86-commandline/latest/nuget.exe

    .\nuget.exe install PanelSwWixExtension
    $code = $LASTEXITCODE
    Pop-Location
    if ($code -gt 0) {
        Exit 1
    }
    $loc = Get-ChildItem -Recurse -Path fetch -Name PanelSwWixExtension.dll
    if (!$loc) {
        Write-Host "Could not locate PanelSwWixExtension.dll"
        Exit 1
    }

    Copy-Item -Path fetch/$loc -Destination artifacts/PanelSwWixExtension.dll -ErrorAction Stop
}

function SignItem() {
    param(
        [Parameter(Mandatory)]
        [string[]]$fileNames
    )

    foreach ($val in $ENV:APP_ID, $ENV:TENANT_ID, $ENV:CLIENT_SECRET, $ENV:CERT_NAME) {
        if (!$val) {
            Write-Host "Skipping signing (no config)"
            Return
        }
    }

    CheckCommand AzureSignTool.exe "AzureSignTool"

    AzureSignTool.exe sign -du "https://github.com/containers/podman" `
        -kvu "https://$ENV:VAULT_ID.vault.azure.net" `
        -kvi $ENV:APP_ID `
        -kvt $ENV:TENANT_ID `
        -kvs $ENV:CLIENT_SECRET `
        -kvc $ENV:CERT_NAME `
        -tr http://timestamp.digicert.com $fileNames

    ExitOnError
}

function CheckCommand() {
    param(
        [Parameter(Mandatory)]
        [string] $cmd,
        [Parameter(Mandatory)]
        [string] $description
    )

    if (! (Get-Command $cmd -errorAction SilentlyContinue)) {
        Write-Host "Required dep `"$description`" is not installed"
        Exit 1
    }
}

function CheckRequirements() {
    CheckCommand "gcc" "MingW CC"
    CheckCommand "candle" "WiX Toolset"
    CheckCommand "go" "Golang"
}


if ($args.Count -lt 1 -or $args[0].Length -lt 1) {
    Write-Host "Usage: " $MyInvocation.MyCommand.Name "<version> [dev|prod] [release_dir]"
    Write-Host
    Write-Host 'Uses Env Vars: '
    Write-Host '   $ENV:FETCH_BASE_URL - GitHub Repo Address to locate release on'
    Write-Host 'Env Settings for signing (optional)'
    Write-Host '   $ENV:VAULT_ID'
    Write-Host '   $ENV:APP_ID'
    Write-Host '   $ENV:TENANT_ID'
    Write-Host '   $ENV:CLIENT_SECRET'
    Write-Host '   $ENV:CERT_NAME'
    Write-Host
    Write-Host "Example: Download and build from the official Github release (dev output): "
    Write-Host " .\build.ps1 4.2.0"
    Write-Host
    Write-Host "Example: Build a dev build from a pre-download release "
    Write-Host " .\build.ps1 4.2.0 dev fetchdir"
    Write-Host

    Exit 1
}

# Pre-set to standard locations in-case build env does not refresh paths
$Env:Path="$Env:Path;C:\Program Files (x86)\WiX Toolset v3.11\bin;C:\ProgramData\chocolatey\lib\mingw\tools\install\mingw64\bin;;C:\Program Files\Go\bin"

CheckRequirements

$version = $args[0]

if ($version[0] -eq "v") {
    $version = $version.Substring(1)
}

$suffix = "-dev"
if ($args.Count -gt 1 -and $args[1] -eq "prod") {
    $suffix = ""
}

$releaseDir = ""
if ($args.Count -gt 2) {
    $releaseDir = $args[2]
}

.\process-release.ps1 $version $releaseDir
if ($LASTEXITCODE -eq 2) {
    Write-Host "Skip signaled, relaying skip"
    Exit 2
}
if ($ENV:INSTVER -eq "") {
    Write-Host "process-release did not define an install version!"
    Exit 1
}

FetchPanel

.\build-hooks.bat; ExitOnError
SignItem @("artifacts/win-sshproxy.exe",
          "artifacts/podman.exe",
          "artifacts/podman-msihooks.dll",
          "artifacts/podman-wslkerninst.exe")

.\build-msi.bat $ENV:INSTVER; ExitOnError
SignItem @("podman.msi")

.\build-burn.bat $ENV:INSTVER; ExitOnError
insignia -ib podman-setup.exe -o engine.exe; ExitOnError
SignItem @("engine.exe")

$file = "podman-$version$suffix-setup.exe"
insignia -ab engine.exe podman-setup.exe -o $file; ExitOnError
SignItem @("$file")

if (Test-Path -Path shasums) {
    $hash = (Get-FileHash -Algorithm SHA256 $file).Hash.ToLower()
    Write-Output "$hash  $file" | Out-File -Append -FilePath shasums
}

Write-Host "Complete"
Get-ChildItem "podman-$version$suffix-setup.exe"
