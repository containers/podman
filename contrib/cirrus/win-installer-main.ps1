 # Powershell doesn't exit after
 function CheckExit {
    if ($LASTEXITCODE -ne 0) {
        throw "Exit code = $LASTEXITCODE"
    }
}
function DownloadFile {
    param(
        [Parameter(Mandatory)]
        [string]$url,
        [Parameter(Mandatory)]
        [string]$file,
        [Int]$retries=5,
        [Int]$delay=8
    )
    $ProgressPreference = 'SilentlyContinue';
    Write-Host "Downloading $url to $file"
    For($i = 0;;) {
        Try {
            Invoke-WebRequest -UseBasicParsing -ErrorAction Stop -Uri $url -OutFile $file
            Break
        } Catch {
            if (++$i -gt $retries) {
                throw $_.Exception
            }
            Write-Host "Download failed - retrying:" $_.Exception.Response.StatusCode
            Start-Sleep -Seconds $delay
        }
    }
}
# Drop global envs which have unix paths, defaults are fine
Remove-Item Env:\GOPATH
Remove-Item Env:\GOSRC
Remove-Item Env:\GOCACHE

Set-Location contrib\win-installer

# Download and extract alt_build win release zip
$url = "${ENV:ART_URL}/Windows Cross/repo/repo.tbz"
# Arc requires extension to be "tbz2"
DownloadFile "$url" "repo.tbz2"
arc unarchive repo.tbz2 .; CheckExit

# Build Installer
.\build.ps1 $Env:WIN_INST_VER dev repo; CheckExit

# Run the installer silently and WSL install option disabled (prevent reboots, wsl requirements)
# We need AllowOldWin=1 for server 2019 (cirrus image), can be dropped after server 2022
$ret = Start-Process -Wait -PassThru ".\podman-${ENV:WIN_INST_VER}-dev-setup.exe" -ArgumentList "/install /quiet WSLCheckbox=0 AllowOldWin=1 /log inst.log"
if ($ret.ExitCode -ne 0) {
    Write-Host "Install failed, dumping log"
    Get-Content inst.log
    throw "Exit code is $($ret.ExitCode)"
}
if (! ((Test-Path -Path "C:\Program Files\RedHat\Podman\podman.exe") -and `
       (Test-Path -Path "C:\Program Files\RedHat\Podman\win-sshproxy.exe"))) {
    throw "Expected podman.exe and win-sshproxy.exe, one or both not present after install"
}
Write-Host "Installer verification successful!"
