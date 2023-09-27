#!/usr/bin/env powershell

# This powershell script is intended to be "dot-sourced" by other scripts.
# It's purpose is identical to that of the `lib.sh` script for Linux environments.

# Behave similar to `set -e` in bash, but ONLY for powershell commandlets!
# For all legacy, program, and script calls use Run-Command() or Check-Exit()
$ErrorActionPreference = 'Stop'

# Any golang compilation needs to know what it's building for.
$Env:GOOS = "windows"
$Env:GOARCH = "amd64"

# Unnecessary and intrusive.  They claim parameter/variable
# values aren't collected, but there could be a bug leading
# to a concern over leaking of some sensitive-value.  Stop this.
$Env:POWERSHELL_TELEMETRY_OPTOUT = "true"

# Unnecessary and potentially disruptive.  Powershell will
# never ever be updated during automation execution.  Stop this.
$Env:POWERSHELL_UPDATECHECK = "off"

# Color in output may confuse tooling and makes logs hard to read.
# TODO: There are probably other places where color needs to be disabled
# in a slightly different way :(
$Env:NO_COLOR = "true"

# Items only relevant in a CI environment.
if ($Env:CI -eq "true") {
    # Defined by .cirrus.yml for use by all the linux tasks.
    # Drop all global envs which have unix paths, defaults are fine.
    Remove-Item Env:\GOPATH -ErrorAction:Ignore
    Remove-Item Env:\GOSRC -ErrorAction:Ignore
    Remove-Item Env:\GOCACHE -ErrorAction:Ignore

    # Defined by Cirrus-CI
    # Drop large known env variables (an env > 32k will break MSI/ICE validation)
    Remove-Item Env:\CIRRUS_COMMIT_MESSAGE -ErrorAction:Ignore
    Remove-Item Env:\CIRRUS_CHANGE_MESSAGE -ErrorAction:Ignore
    Remove-Item Env:\CIRRUS_PR_BODY -ErrorAction:Ignore
}

# Non-powershell commands do not halt execution on error!  This helper
# should be called after every critical operation to check and halt on a
# non-zero exit code.  Be careful not to use this for powershell commandlets
# (builtins)!  They set '$?' to "True" (failed) or "False" success so calling
# this would mask failures.  Rely on $ErrorActionPreference = 'Stop' instead.
function Check-Exit {
    $result = $LASTEXITCODE  # WARNING: might not be a number!
    if ( ($result -ne $null) -and ($result -ne 0) ) {
        # https://learn.microsoft.com/en-us/dotnet/api/system.management.automation.callstackframe
        $caller = (Get-PSCallStack)[1]
        Write-Host "Exit code = '$result' from $($caller.ScriptName):$($caller.ScriptLineNumber)"
        Exit $result
    }
}

# Small helper to avoid needing to write 'Check-Exit' after every
# non-powershell instruction.  It simply prints then executes the _QUOTED_
# argument followed by Check-Exit.
# N/B: Escape any nested quotes with back-tick ("`") characters.
# WARNING: DO NOT use this with powershell builtins! It will not do what you expect!
function Run-Command {
    param (
        [string] $command
    )

    Write-Host $command

    Invoke-Expression $command
    Check-Exit
}
