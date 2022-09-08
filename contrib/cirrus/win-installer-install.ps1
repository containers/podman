# Update service is required for dotnet 3.5 (dep of wix)
Set-Service -Name wuauserv -StartupType "Manual"
choco install -y wixtoolset mingw golang archiver
if ($LASTEXITCODE -ne 0) {
    Exit 1
}
