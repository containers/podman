function ExitOnError() {
    if ($LASTEXITCODE -ne 0) {
        Exit 1
    }
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
    CheckCommand "wix" "WiX Toolset"
    CheckCommand "go" "Golang"
}

if ($args.Count -lt 1 -or $args[0].Length -lt 1) {
    Write-Host "Usage: " $MyInvocation.MyCommand.Name "<version> [dev|prod] [release_dir]"
    Write-Host
    Write-Host 'Uses Env Vars: '
    Write-Host '   $ENV:FETCH_BASE_URL - GitHub Repo Address to locate release on'
    Write-Host '   $ENV:V531_SETUP_EXE_PATH - Path to v5.3.1 setup.exe used to build the patch'
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
$Env:Path="$Env:Path;C:\Users\micro\mingw64\bin;C:\ProgramData\chocolatey\lib\mingw\tools\install\mingw64\bin;;C:\Program Files\Go\bin;C:\Program Files\dotnet"

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

.\build-hooks.ps1; ExitOnError
SignItem @("artifacts/win-sshproxy.exe",
          "artifacts/podman.exe",
          "artifacts/podman-msihooks.dll",
          "artifacts/podman-wslkerninst.exe")
$gvExists = Test-Path "artifacts/gvproxy.exe"
if ($gvExists) {
    SignItem @("artifacts/gvproxy.exe")
    Remove-Item Env:\UseGVProxy -ErrorAction SilentlyContinue
} else {
    $env:UseGVProxy = "Skip"
}

# Retaining for possible future additions
# $pExists = Test-Path "artifacts/policy.json"
# if ($pExists) {
#     Remove-Item Env:\IncludePolicyJSON -ErrorAction SilentlyContinue
# } else {
#     $env:IncludePolicyJSON = "Skip"
# }
if (Test-Path ./obj) {
    Remove-Item ./obj -Recurse -Force -Confirm:$false
}
dotnet build podman.wixproj /property:DefineConstants="VERSION=$ENV:INSTVER" -o .; ExitOnError
SignItem @("en-US\podman.msi")

dotnet build podman-setup.wixproj /property:DefineConstants="VERSION=$ENV:INSTVER" -o .; ExitOnError
wix burn detach podman-setup.exe -engine engine.exe; ExitOnError
SignItem @("engine.exe")

$file = "podman-$version$suffix-setup.exe"
wix burn reattach -engine engine.exe podman-setup.exe -o $file; ExitOnError
SignItem @("$file")

if (Test-Path -Path shasums) {
    $hash = (Get-FileHash -Algorithm SHA256 $file).Hash.ToLower()
    Write-Output "$hash  $file" | Out-File -Append -FilePath shasums
}

Write-Host "Complete"
Get-ChildItem "podman-$version$suffix-setup.exe"
