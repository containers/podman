#Requires -RunAsAdministrator

Set-StrictMode -Version Latest

# get the directory that has a name that starts with WslLogs
$folder = Get-ChildItem -Directory | Where-Object { $_.Name -like "WslLogs*" } | Select-Object -First 1
$wprOutputLog = "$folder/wpr.txt"

Write-Host "Saving WSL logs..."
wpr.exe -stop $folder/logs.etl 2>&1 >> $wprOutputLog

$logArchive = "$(Resolve-Path $folder).zip"
Compress-Archive -Path $folder -DestinationPath $logArchive
Remove-Item $folder -Recurse

Write-Host -ForegroundColor Green "Logs saved in: $logArchive"
