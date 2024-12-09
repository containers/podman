function Build-WSLKernelInstaller {
    param (
        [string]$wslkerninstFolder,
        [string]$artifactsFolder
        );
    Set-Variable GOARCH=amd64
    go build -ldflags -H=windowsgui -o "$artifactsFolder\podman-wslkerninst.exe" "$wslkerninstFolder"
}

function Build-MSIHooks {
    param (
        [string]$msiHooksFolder,
        [string]$artifactsFolder
        );

    # Build using x86 toolchain, see comments in check.c for rationale and details
    if ( Get-MingW ) {
        Build-MSIHooks-Using-MingW $msiHooksFolder $artifactsFolder
    } elseif ( Get-VSBuildTools ) {
        $vsinstance = Get-VSSetupInstance | Select-VSSetupInstance -Product  Microsoft.VisualStudio.Product.BuildTools -Latest
        Build-MSIHooks-Using-VSBuildTools $msiHooksFolder $artifactsFolder $vsinstance
    } else {
        $msg = "A C/C++ compiler is required to build `"$msiHooksFolder\check.c`". "
        $msg += "Supported compilers are MinGW CC (`"x86_64-w64-mingw32-gcc`") and the "
        $msg += "`"Microsoft.VisualStudio.Product.BuildTools`" with `"VSSetup`" PowerShell extension."
        Write-Error -Message $msg -ErrorAction Stop
    }
}

function Get-MingW {
    return Get-Command "x86_64-w64-mingw32-gcc" -errorAction SilentlyContinue
}

function Get-VSBuildTools {
    return ((Get-Command "Get-VSSetupInstance" -errorAction SilentlyContinue) -and `
            (@(Get-VSSetupInstance | Select-VSSetupInstance -Product "Microsoft.VisualStudio.Product.BuildTools").Count -gt 0))
}

function Build-MSIHooks-Using-MingW {
    param (
        [string]$msiHooksFolder,
        [string]$artifactsFolder
        );
    Set-Variable GOARCH=amd64
    x86_64-w64-mingw32-gcc $msiHooksFolder/check.c -shared -lmsi -mwindows -o $artifactsFolder/podman-msihooks.dll
}

function Build-MSIHooks-Using-VSBuildTools {
    param (
        [string]$msiHooksFolder,
        [string]$artifactsFolder,
        [Microsoft.VisualStudio.Setup.Instance]$vsinstance
        );
    $vspath = $vsinstance.InstallationPath
    $vsinstanceid = $vsinstance.InstanceId

    Import-Module "$vspath\Common7\Tools\Microsoft.VisualStudio.DevShell.dll"
    Enter-VsDevShell $vsinstanceid -DevCmdArguments '-arch=amd64 -host_arch=amd64'
    cl.exe /W4 /Fo$artifactsFolder\ $msiHooksFolder\check.c Advapi32.lib Msi.lib /link /DLL /out:$artifactsFolder\podman-msihooks.dll
}

$wslkerninstFolder="$PSScriptRoot\..\..\cmd\podman-wslkerninst"
$msiHooksFolder="$PSScriptRoot\podman-msihooks"
$artifactsFolder="$PSScriptRoot\artifacts"

Build-WSLKernelInstaller $wslkerninstFolder $artifactsFolder
Build-MSIHooks $msiHooksFolder $artifactsFolder
