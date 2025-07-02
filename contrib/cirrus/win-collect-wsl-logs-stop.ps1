#    MIT License
#
#    Copyright (c) Microsoft Corporation.
#
#    Permission is hereby granted, free of charge, to any person obtaining a copy
#    of this software and associated documentation files (the "Software"), to deal
#    in the Software without restriction, including without limitation the rights
#    to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
#    copies of the Software, and to permit persons to whom the Software is
#    furnished to do so, subject to the following conditions:
#
#    The above copyright notice and this permission notice shall be included in all
#    copies or substantial portions of the Software.
#
#    THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
#    IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
#    FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
#    AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
#    LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
#    OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
#    SOFTWARE

# This script is an adapted version of
# https://github.com/Microsoft/WSL/blob/master/diagnostics/collect-wsl-logs.ps1

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

$fileName = (Get-Item $logArchive).Name
$parentFolder = (Get-Item $logArchive).Directory.Parent.FullName
Move-Item -Path $logArchive -Destination $parentFolder

Write-Host -ForegroundColor Green "Logs saved in: ${parentFolder}/${fileName}"
