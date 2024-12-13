# Building the Podman client and client installer on Windows

The following describes the process for building and testing the Podman Windows
client (`podman.exe`) and the Podman Windows installer (`podman-setup.exe`) on
Windows.

## Topics

- [Requirements](#requirements)
  - [OS requirements](#os-requirements)
  - [Git and go](#git-and-go)
  - [Pandoc](#pandoc)
  - [.NET SDK](#net-sdk)
  - [Visual Studio Build Tools](#visual-studio-build-tools)
  - [Virtualization Provider](#virtualization-provider)
    - [WSL](#wsl)
    - [Hyper-V](#hyper-v)
- [Get the source code](#get-the-source-code)
  - [Allow local PowerShell scripts execution](#allow-local-powershell-scripts-execution)
- [Build and test the Podman client for Windows](#build-and-test-the-podman-client-for-windows)
  - [Build the Podman client](#build-the-podman-client)
  - [Download gvproxy.exe and win-sshproxy.exe](#download-gvproxyexe-and-win-sshproxyexe)
  - [Create a configuration file (optional)](#create-a-configuration-file-optional)
  - [Create and start a podman machine](#create-and-start-a-podman-machine)
  - [Run a container using podman](#run-a-container-using-podman)
- [Build and test the Podman Windows installer](#build-and-test-the-podman-windows-installer)
  - [Build the Windows installer](#build-the-windows-installer)
  - [Test the Windows installer](#test-the-windows-installer)
  - [Build and test the standalone `podman.msi` file](#build-and-test-the-standalone-podmanmsi-file)
  - [Verify the installation](#verify-the-installation)
  - [Uninstall and clean-up](#uninstall-and-clean-up)
- [Validate changes before submitting a PR](#validate-changes-before-submitting-a-pr)
  - [winmake lint](#winmake-lint)
  - [winmake validatepr](#winmake-validatepr)

## Requirements

### OS requirements

This documentation assumes one uses a Windows 10 or 11 development machine and a
PowerShell terminal.

### Git and go

To build Podman, the [git](https://gitforwindows.org/) and [go](https://go.dev)
tools are required. In case they are not yet installed, open a Windows
PowerShell terminal and run the following command (it assumes that
[winget](https://learn.microsoft.com/en-us/windows/package-manager/winget/) is
installed):

```pwsh
winget install -e GoLang.Go Git.Git
```

:information_source: A terminal restart is advised for the `PATH` to be
reloaded. This can also be manually changed by configuring the `PATH`:

```pwsh
$env:Path += ";C:\Program Files\Go\bin\;C:\Program Files\Git\cmd\"
```

### Pandoc

[Pandoc](https://pandoc.org/) is used to generate Podman documentation. It is
required for building the documentation and the
[bundle installer](#build-the-installer). It can be avoided when building and
testing the
[Podman client for Windows](#build-and-test-the-podman-client-for-windows) or
[the standalone `podman.msi` installer](#build-and-test-the-standalone-podmanmsi-file).
Pandoc can be installed from https://pandoc.org/installing.html. When performing
the Pandoc installation one, has to choose the option "Install for all users"
(to put the binaries into "Program Files" directory).

### .NET SDK

[.NET SDK](https://learn.microsoft.com/en-us/dotnet/core/sdk), version 6 or
later, is required to develop and build the Podman Windows installer. It's not
required for the Podman Windows client.

```pwsh
winget install -e Microsoft.DotNet.SDK.8
```

[WiX Toolset](https://wixtoolset.org) **v5**, distributed as a .NET SDK tool, is
used too and can be installed using `dotnet install`:

```pwsh
dotnet tool install --global wix
```

### Visual Studio Build Tools

The installer includes a C program that checks the installation of the
pre-required virtualization providers (WSL or Hyper-V). Building this program
requires the
[Microsoft C/C++ compiler](https://learn.microsoft.com/en-us/cpp/build/building-on-the-command-line?view=msvc-170) and the
[PowerShell Module VSSetup](https://github.com/microsoft/vssetup.powershell):

1. Download the Build Tools for Visual Studio 2022 installer
```pwsh
Invoke-WebRequest -Uri 'https://aka.ms/vs/17/release/vs_BuildTools.exe' -OutFile "$env:TEMP\vs_BuildTools.exe"
```
2. Run the installer with the parameter to include the optional C/C++ Tools
```pwsh
& "$env:TEMP\vs_BuildTools.exe" --passive --wait `
                      --add Microsoft.VisualStudio.Workload.VCTools `
                      --includeRecommended `
                      --remove Microsoft.VisualStudio.Component.VC.CMake.Project
```
3. Install the PowerShell Module VSSetup
```pwsh
Install-Module VSSetup
```

### Virtualization Provider

Running Podman on Windows requires a virtualization provider. The supported
providers are the
[Windows Subsystem for Linux (WSL)](https://learn.microsoft.com/en-us/windows/wsl/)
and
[Hyper-V](https://learn.microsoft.com/en-us/virtualization/hyper-v-on-windows/quick-start/enable-hyper-v).
At least one of those two is required to test podman on a local Windows machine.

#### WSL

WSL can be installed on Windows 10 and Windows 11, including Windows Home, with
the following command, from a PowerShell or Windows Command Prompt terminal in
**administrator mode**:

```pwsh
wsl --install
```

For more information refer to
[the official documentation](https://learn.microsoft.com/en-us/windows/wsl/).

#### Hyper-V

Hyper-V is an optional feature of Windows Enterprise, Pro, or Education (not
Home). It is available on Windows 10 and 11 only and
[has some particular requirements in terms of CPU and memory](https://learn.microsoft.com/en-us/virtualization/hyper-v-on-windows/quick-start/enable-hyper-v#check-requirements).
To enable it on a supported system, enter the following command:

```pwsh
Enable-WindowsOptionalFeature -Online -FeatureName Microsoft-Hyper-V -All
```

After running this command, a restart of the Windows machine is required.

:information_source: Configure the VM provider used by podman (Hyper-V or WSL)
in the file `%PROGRAMDATA%/containers/containers.conf`.
[More on that later](#create-a-configuration-file-optional).

## Get the source code

Open a Windows Terminal and run the following command:

```pwsh
git config --global core.autocrlf false
```

It configures git so that it does **not** automatically convert LF to CRLF. In
the Podman git repository, files are expected to use Unix LF rather than Windows
CRLF.

Then run the command to clone the Podman git repository:

```pwsh
git clone https://github.com/containers/podman
```

It creates the folder `podman` in the current directory and clones the Podman
git repository into it.

### Allow local PowerShell scripts execution

A developer can build the Podman client for Windows and the Windows installer
with the PowerShell script
[winmake.ps1](https://github.com/containers/podman/blob/main/winmake.ps1).

Windows sets the ExecutionPolicy to `Restricted` by default; running scripts is
prohibited. Determine the ExecutionPolicy on the machine with this command:

```pwsh
Get-ExecutionPolicy
```

If the command returns `Restricted`, the ExecutionPolicy should be changed to
`RemoteSigned`:

```pwsh
Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser
```

This policy allows the execution of local PowerShell scripts, such as
`winmake.ps1`, for the current user.

## Build and test the Podman client for Windows

The following steps describe how to build the `podman.exe` binary from sources
and test it.

### Build the Podman client

Open a PowerShell terminal and move to Podman local git repository directory:

```pwsh
Set-Location .\podman
```

Build `podman.exe`

```
.\winmake.ps1 podman-remote
```

:information_source: Verify build's success by checking the content of the
`.\bin\windows` folder. Upon successful completion, the executable `podman.exe`
should be there:

```pwsh
Get-ChildItem .\bin\windows\


    Directory: C:\Users\mario\Git\podman\bin\windows


Mode                 LastWriteTime         Length Name
----                 -------------         ------ ----
-a----         2/27/2024  11:59 AM       45408256 podman.exe
```

### Download gvproxy.exe and win-sshproxy.exe

[gvisor-tap-vsock](https://github.com/containers/gvisor-tap-vsock/) binaries
(`gvproxy-windowsgui.exe` and `win-sshproxy.exe`) are required to run the Podman
client on Windows. The executables are expected to be in the same folder as
`podman.exe`. The following command downloads the latest version in the
`.\bin\windows\` folder:

```pwsh
.\winmake.ps1 win-gvproxy
```

:information_source: To verify that the binaries have been downloaded
successfully, check the content of the .\bin\windows` folder.

```pwsh
Get-ChildItem .\bin\windows\


    Directory: C:\Users\mario\Git\podman\bin\windows


Mode                 LastWriteTime         Length Name
----                 -------------         ------ ----
-a----         2/29/2024  12:10 PM       10946048 gvproxy.exe
-a----         2/27/2024  11:59 AM       45408256 podman.exe
-a----         2/29/2024  12:10 PM        4089856 win-sshproxy.exe
```

### Create a configuration file (optional)

To test some particular configurations of Podman, create a `containers.conf`
file:

```
New-Item -ItemType Directory $env:PROGRAMDATA\containers\
New-Item -ItemType File $env:PROGRAMDATA\containers\containers.conf
notepad $env:PROGRAMDATA\containers\containers.conf
```

For example, to test with Hyper-V as the virtualization provider, use the
following content:

```toml
[machine]
provider="hyperv"
```

Find the complete list of configuration options in the
[documentation](https://github.com/containers/common/blob/main/docs/containers.conf.5.md).

### Create and start a podman machine

Execute the following commands in a terminal to create a Podman machine:

```pwsh
.\bin\windows\podman.exe machine init
```

When `machine init` completes, run `machine start`:

```pwsh
.\bin\windows\podman.exe machine start
```

:information_source: If the virtualization provider is Hyperv-V, execute the
above commands in an administrator terminal.

### Run a container using podman

Use the locally built Podman client for Windows to run containers:

```pwsh
.\bin\windows\podman.exe run hello-world
```

To learn how to use the Podman client, refer to its
[tutorial](https://github.com/containers/podman/blob/main/docs/tutorials/remote_client.md).

## Build and test the Podman Windows installer

The Podman Windows installer (e.g., `podman-5.1.0-dev-setup.exe`) is a bundle
that includes an msi package (`podman.msi`) and installs the WSL kernel
(`podman-wslkerninst.exe`). It's built using the
[WiX Toolset](https://wixtoolset.org/) and the
[PanelSwWixExtension](https://github.com/nirbar/PanelSwWixExtension/tree/master5)
WiX extension. The source code is in the folder `contrib\win-installer`.

### Build the Windows installer

To build the installation bundle, run the following command:

```pwsh
.\winmake.ps1 installer
```

:information_source: making `podman-remote`, `win-gvproxy`, and `docs` is
required before running this command.

Locate the installer in the `contrib\win-installer` folder (relative to checkout
root) with a name like `podman-5.2.0-dev-setup.exe`.

The `installer` target of `winmake.ps1` runs the script
`contrib\win-installer\build.ps1` that, in turns, executes:

- `build-hooks.bat`: builds `podman-wslkerninst.exe` (WSL kernel installer) and
  `podman-msihooks.dll` (helper that checks if WSL and Hyper-V are installed).
- `dotnet build podman.wixproj`: builds `podman.msi` from the WiX source files `podman.wxs`,
  `pages.wxs`, `podman-ui.wxs` and `welcome-install-dlg.wxs`.
- `dotnet build podman-setup.wixproj`: builds `podman-setup.exe` file from
  [WiX Burn bundle](https://wixtoolset.org/docs/tools/burn/) `burn.wxs`.

### Test the Windows installer

Double-click on the Windows installer to run it. To get the installation logs
with debug information, running it via the command line is recommended:

```pwsh
contrib\win-installer\podman-5.1.0-dev-setup.exe /install /log podman-setup.log
```

It generates the files `podman-setup.log` and `podman-setup_000_Setup.log`,
which include detailed installation information, in the current directory.

Run it in `quiet` mode to automate the installation and avoid interacting with
the GUI. Open the terminal **as an administrator**, add the `/quiet` option, and
set the bundle variables `MachineProvider` (`wsl` or `hyperv`), `WSLCheckbox`
(`1` to install WSL as part of the installation, `0` otherwise), and
`HyperVCheckbox` (`1` to install Hyper-V as part of the installation, `0`
otherwise):

```pwsh
contrib\win-installer\podman-5.1.0-dev-setup.exe /install `
                      /log podman-setup.log /quiet `
                      MachineProvider=wsl WSLCheckbox=0 HyperVCheckbox=0
```

:information_source: If uninstallation fails, the installer may end up in an
inconsistent state. Podman results as uninstalled, but some install packages are
still tracked in the Windows registry and will affect further tentative to
re-install Podman. When this is the case, trying to re-install Podman results in
the installer returning zero (success) but no action is executed. The trailing
packages `GID` can be found in installation logs:

```
Detected related package: {<GID>}
```

To fix this problem remove the related packages:

```pwsh
msiexec /x "{<GID>}"
```

#### Run the Windows installer automated tests

The following command executes a number of tests of the windows installer. Running
it requires an administrator terminal.

```pwsh
.\winmake.ps1 installertest [wsl|hyperv]
```

### Build and test the standalone `podman.msi` file

Building and testing the standalone `podman.msi` package during development may
be useful. Even if this package is not published as a standalone file when
Podman is released (it's included in the `podman-setup.exe` bundle), it can be
faster to build and test that rather than the full bundle during the development
phase.

Run the command `dotnet build` to build the standalone `podman.msi` file:

```pwsh
Push-Location .\contrib\win-installer\
dotnet build podman.wixproj /property:DefineConstants="VERSION=9.9.9" -o .
Pop-Location
```

It creates the file `.\contrib\win-installer\en-US\podman.msi`. Test it using the
[Microsoft Standard Installer](https://learn.microsoft.com/en-us/windows/win32/msi/standard-installer-command-line-options)
command line tool:

```pwsh
msiexec /package contrib\win-installer\en-US\podman.msi /l*v podman-msi.log
```

To run it in quiet, non-interactive mode, open the terminal **as an
administrator**, add the `/quiet` option, and set the MSI properties
`MACHINE_PROVIDER` (`wsl` or `hyperv`), `WITH_WSL` (`1` to install WSL as part
of the installation, `0` otherwise) and `WITH_HYPERV` (`1` to install Hyper-V as
part of the installation, `0` otherwise):

```pwsh
msiexec /package contrib\win-installer\en-US\podman.msi /l*v podman-msi.log /quiet MACHINE_PROVIDER=wsl WITH_WSL=0 WITH_HYPERV=0
```

:information_source: `podman.msi` GUI dialogs, defined in the file
`contrib\win-installer\welcome-install-dlg.wxs`, are distinct from the installation bundle
`podman-setup.exe` GUI dialogs, defined in
`contrib\win-installer\podman-theme.xml`.

### Verify the installation

Inspect the msi installation log `podman-msi.log` (or
`podman-setup_000_Setup.log` if testing with the bundle) to verify that the
installation was successful:

```pwsh
Select-String -Path "podman-msi.log" -Pattern "Installation success or error status: 0"
```

These commands too are helpful to check the installation:

```pwsh
# Check the copy of the podman client in the Podman folder
Test-Path -Path "$ENV:PROGRAMFILES\RedHat\Podman\podman.exe"
# Check the generation of the podman configuration file
Test-Path -Path "$ENV:PROGRAMDATA\containers\containers.conf.d\99-podman-machine-provider.conf"
# Check that the installer configured the right provider
Get-Content "$ENV:PROGRAMDATA\containers\containers.conf.d\99-podman-machine-provider.conf" | Select -Skip 1 | ConvertFrom-StringData | % { $_.provider }
# Check the creation of the registry key
Test-Path -Path "HKLM:\SOFTWARE\Red Hat\Podman"
Get-ItemProperty "HKLM:\SOFTWARE\Red Hat\Podman" InstallDir
# Check the podman.exe is in the $PATH
$env:PATH | Select-String -Pattern "Podman"
```

:information_source: Podman CI uses script
`contrib\cirrus\win-installer-main.ps1`. Use it locally, too, to build and test
the installer:

```pwsh
$ENV:CONTAINERS_MACHINE_PROVIDER='wsl'; .\contrib\cirrus\win-installer-main.ps1
$ENV:CONTAINERS_MACHINE_PROVIDER='hyperv'; .\contrib\cirrus\win-installer-main.ps1
```

### Uninstall and clean-up

Podman can be uninstalled from the Windows Control Panel or running the
following command from a terminal **as an administrator**:

```pwsh
contrib\win-installer\podman-5.1.0-dev-setup.exe /uninstall /quiet /log podman-setup-uninstall.log
```

The uninstaller does not delete some folders. Clean them up manually:

```pwsh
$extraFolders = @(
    "$ENV:PROGRAMDATA\containers\"
    "$ENV:LOCALAPPDATA\containers\"
    "$env:USERPROFILE.config\containers\"
    "$env:USERPROFILE.local\share\containers\"
    )
$extraFolders | ForEach-Object {Remove-Item -Recurse -Force $PSItem}
```

The following commands are helpful to verify that the uninstallation was
successful:

```pwsh
# Inspect the uninstallation log for a success message
Select-String -Path "podman-setup-uninstall_000_Setup.log" -Pattern "Removal success or error status: 0"
# Check that the uninstaller removed Podman resources
$foldersToCheck = @(
    "$ENV:PROGRAMFILES\RedHat\Podman\podman.exe"
    "HKLM:\SOFTWARE\Red Hat\Podman"
    "$ENV:PROGRAMDATA\containers\"
    "$env:USERPROFILE.config\containers\"
    "$env:USERPROFILE.local\share\containers\"
    "$ENV:LOCALAPPDATA\containers\"
    "$ENV:PROGRAMDATA\containers\containers.conf.d\99-podman-machine-provider.conf"
)
$foldersToCheck | ForEach-Object {Test-Path -Path $PSItem}
```

## Validate changes before submitting a PR

The script `winmake.ps1` has a couple of targets to check the source code
statically. GitHub Pull request checks execute the same statical analysis. It is
highly recommended that you run them locally before submitting a PR.

### winmake lint

The `lint` target provides a fast validation target. It runs the following
tools:

- `golangci-lint`: runs go-specific linters configured in
  [`.golangci.yml`](.golangci.yml)
- `pre-commit`: runs more linters configured in
  [`.pre-commit-config.yaml`](.pre-commit-config.yaml)

:information_source: Install [golangci-lint](https://golangci-lint.run) and
[pre-commit](https://pre-commit.com) to run `winmake.ps1 lint`.

### winmake validatepr

Target `validatepr` performs a more exhaustive validation but takes
significantly more time to complete. It uses `podman` to run the target
`.validatepr` of the [Linux `Makefile`](Makefile). It builds Podman for Linux,
MacOS and Windows and then performs the same checks as the `lint` target plus
many more.

:information_source: Create and start a Podman machine before running
`winmake.ps1 validatepr`. Configure the Podman machine with at least 4GB of
memory:
`podman machine init -m 4096`.
