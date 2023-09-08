# Update service is required for dotnet 3.5 (dep of wix)
Set-Service -Name wuauserv -StartupType "Manual"
function retryInstall {
    param([Parameter(ValueFromRemainingArguments)] [string[]] $pkgs)

    foreach ($pkg in $pkgs) {
        for ($retries = 0; ; $retries++) {
            if ($retries -gt 5) {
                throw "Could not install package $pkg"
            }

            if ($pkg -match '(.[^\@]+)@(.+)') {
                $pkg = @("--version", $Matches.2, $Matches.1)
            }

            choco install -y --allow-downgrade $pkg
            if ($LASTEXITCODE -eq 0) {
                break
            }
            Write-Output "Error installing, waiting before retry..."
            Start-Sleep -Seconds 6
        }
    }
}
# Force mingw version 11.2 since later versions are incompatible
# with CGO on some versions of golang
retryInstall wixtoolset mingw@11.2 golang archiver
