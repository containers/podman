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

[CmdletBinding()]
Param (
    $LogProfile = $null
   )

Set-StrictMode -Version Latest

$folder = "WslLogs" + (Get-Date -Format "yyyy-MM-dd_HH-mm-ss")
mkdir -p $folder | Out-Null

if ($LogProfile -eq $null -Or ![System.IO.File]::Exists($LogProfile))
{
    if ($LogProfile -eq $null)
    {
        $url = "https://raw.githubusercontent.com/microsoft/WSL/master/diagnostics/wsl.wprp"
    }
    elseif ($LogProfile -eq "storage")
    {
         $url = "https://raw.githubusercontent.com/microsoft/WSL/master/diagnostics/wsl_storage.wprp"
    }
    else
    {
        Write-Error "Unknown log profile: $LogProfile"
        exit 1
    }

    $LogProfile = "$folder/wsl.wprp"
    try {
        Invoke-WebRequest -UseBasicParsing $url -OutFile $LogProfile
    }
    catch {
        throw
    }
}

reg.exe export HKEY_CURRENT_USER\Software\Microsoft\Windows\CurrentVersion\Lxss $folder/HKCU.txt 2>&1 | Out-Null
reg.exe export HKEY_LOCAL_MACHINE\Software\Microsoft\Windows\CurrentVersion\Lxss $folder/HKLM.txt 2>&1 | Out-Null
reg.exe export HKEY_LOCAL_MACHINE\SYSTEM\CurrentControlSet\Services\P9NP $folder/P9NP.txt 2>&1 | Out-Null
reg.exe export HKEY_LOCAL_MACHINE\SYSTEM\CurrentControlSet\Services\WinSock2 $folder/Winsock2.txt 2>&1 | Out-Null
# Skipping the next reg.exe command because it's failing with error
#    "ERROR: The system was unable to find the specified registry key or value"
# on the CI server (Windows Server 2022)
# reg.exe export "HKEY_CLASSES_ROOT\CLSID\{e66b0f30-e7b4-4f8c-acfd-d100c46c6278}" $folder/wslsupport-proxy.txt 2>&1 | Out-Null
reg.exe export "HKEY_CLASSES_ROOT\CLSID\{a9b7a1b9-0671-405c-95f1-e0612cb4ce7e}" $folder/wslsupport-impl.txt 2>&1 | Out-Null
Get-ItemProperty -Path "Registry::HKEY_LOCAL_MACHINE\SOFTWARE\Microsoft\Windows NT\CurrentVersion" > $folder/windows-version.txt

Get-Service wslservice -ErrorAction Ignore | Format-list * -Force  > $folder/wslservice.txt

$wslconfig = "$env:USERPROFILE/.wslconfig"
if (Test-Path $wslconfig)
{
    Copy-Item $wslconfig $folder | Out-Null
}

get-appxpackage MicrosoftCorporationII.WindowsSubsystemforLinux -ErrorAction Ignore > $folder/appxpackage.txt
get-acl "C:\ProgramData\Microsoft\Windows\WindowsApps" -ErrorAction Ignore | Format-List > $folder/acl.txt
Get-WindowsOptionalFeature -Online > $folder/optional-components.txt
bcdedit.exe > $folder/bcdedit.txt

$uninstallLogs = "$env:TEMP/wsl-uninstall-logs.txt"
if (Test-Path $uninstallLogs)
{
    Copy-Item $uninstallLogs $folder | Out-Null
}

$wprOutputLog = "$folder/wpr.txt"

wpr.exe -start $LogProfile -filemode 2>&1 >> $wprOutputLog
if ($LastExitCode -Ne 0)
{
    Write-Host -ForegroundColor Yellow "Log collection failed to start (exit code: $LastExitCode), trying to reset it."
    wpr.exe -cancel 2>&1 >> $wprOutputLog

    wpr.exe -start $LogProfile -filemode 2>&1 >> $wprOutputLog
    if ($LastExitCode -Ne 0)
    {
        Write-Host -ForegroundColor Red "Couldn't start log collection (exitCode: $LastExitCode)"
    }
}

Write-Host "`nWSL Log collection is running."
