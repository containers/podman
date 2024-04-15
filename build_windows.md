# Building the Podman client and client installer on Windows

The following describes the process for building the Podman client on Windows.

## OS requirements

Windows OS can behave very differently depending on how it was configured. This documentation assumes that one is using
a [Windows 11 development machine](https://developer.microsoft.com/en-us/windows/downloads/virtual-machines/) or a
configuration close to this one. The Podman Windows client installer bundles several tools, which are unnecessary for Podman builds, but this
set of packages is well aligned with GitHub's `windows-latest` offerings. Some of the tools will still be missing from
this distribution and will have to be manually added after this installation completes.

## Install Pandoc

Pandoc could be installed from https://pandoc.org/installing.html When performing the Pandoc installation one, has to choose the option
"Install for all users" (to put the binaries into "Program Files" directory).

## Install WiX Toolset v3 (is preinstalled in GitHub runner)
The latest release of the WiX Toolset can be obtained from https://wixtoolset.org/docs/wix3/. Installing it into a clean VM might require
an additional installation of .NET Framework 3.5 in advance
([instructions for adding .NET Framework 3.5 via enabling the Windows feature](https://learn.microsoft.com/en-us/dotnet/framework/install/dotnet-35-windows#enable-the-net-framework-35-in-control-panel))

## Building and running

### Install git and go

To build Podman, the [git](https://gitforwindows.org/) and [go](https://go.dev) tools are required. In case they are not yet installed,
open a Windows Terminal and run the following command (it assumes that [winget](https://learn.microsoft.com/en-us/windows/package-manager/winget/) is installed):

```
winget install -e GoLang.Go Git.Git
```

:information_source: A terminal restart is advised for the `PATH` to be reloaded. This can also be manually changed by configuring the `PATH`:

```
$env:Path += ";C:\Program Files\Go\bin\;C:\Program Files\Git\cmd\"
```

### Enable Hyper-V (optional)

Podman on Windows can run on the [Windows Subsystem for Linux (WSL)](https://learn.microsoft.com/en-us/windows/wsl/) or
on [Hyper-V](https://learn.microsoft.com/en-us/virtualization/hyper-v-on-windows/quick-start/enable-hyper-v).

Hyper-V is built into Windows Enterprise, Pro, or Education (not Home) as an optional feature. It is available on Windows 10 and 11 only
and [has some particular requirements in terms of CPU and memory](https://learn.microsoft.com/en-us/virtualization/hyper-v-on-windows/quick-start/enable-hyper-v#check-requirements).
To enable it on a supported system, enter the following command:

```
Enable-WindowsOptionalFeature -Online -FeatureName Microsoft-Hyper-V -All
```

After running this command a restart of the Windows machine is required.

:information_source: The VM provider used by podman (Hyper-V or WSL) can be configured in the file
`%APPDATA%/containers/containers.conf`. [More on that later](#configure-podman).

### Get the source code

Open a Windows Terminal and run the following command:

```
git config --global core.autocrlf false
```

This configures git so that it does **not** automatically convert LF to CRLF. Files are expected to use the Unix LF rather
Windows CRLF in the Podman git repository.

Then run the command to clone the Podman git repository:

```
git clone https://github.com/containers/podman
```

This will create the folder `podman` in the current directory and clone the Podman git repository into it.

### Build the podman client for windows

The Podman client for Windows can be built with the PowerShell script [winmake.ps1](https://github.com/containers/podman/blob/main/winmake.ps1).

The ExecutionPolicy is set to `Restricted` on Windows computers by default: running scripts is not allowed.
The ExecutionPolicy on the machine can be determined with this command:

```
Get-ExecutionPolicy
```

If the command returns `Restricted`, the ExecutionPolicy should be changed to `RemoteSigned`:

```
Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser
```

This policy allows the execution of local PowerShell scripts, such as `winmake.ps1`, for the current user:

```
# Get in the podman git repository directory
cd podman

# Build podman.exe
.\winmake.ps1 podman-remote

# Download gvproxy.exe and win-sshproxy.exe
# that are needed to execute the podman client
.\winmake.ps1 win-gvproxy
```

:information_source: To verify that the build was completed successfully, check the content of the .\bin\windows` folder.
Upon successful completion three executables should be shown:

```
ls .\bin\windows\


    Directory: C:\Users\mario\Git\podman\bin\windows


Mode                 LastWriteTime         Length Name
----                 -------------         ------ ----
-a----         2/29/2024  12:10 PM       10946048 gvproxy.exe
-a----         2/27/2024  11:59 AM       45408256 podman.exe
-a----         2/29/2024  12:10 PM        4089856 win-sshproxy.exe
```

### Configure podman

Create a containers.conf file:

```
mkdir $env:APPDATA\containers\
New-Item -ItemType File $env:APPDATA\containers\containers.conf
notepad $env:APPDATA\containers\containers.conf
```

and add the following lines in it:

```toml
[machine]
# Specify the provider
# Values can be "hyperv" or "wsl"
provider="hyperv"
[engine]
# Specify the path of the helper binaries.
# NOTE: path should use slashes instead of anti-slashes
helper_binaries_dir=["C:/Users/mario/git/podman/bin/windows"]
```

Create a policy.json file:

```
New-Item -ItemType File $env:APPDATA\containers\policy.json
notepad $env:APPDATA\containers\policy.json
````

and add the following lines in it:

```json
{
  "default": [
    {
      "type": "insecureAcceptAnything"
    }
  ],
  "transports": {
    "docker-daemon": {
      "": [{ "type": "insecureAcceptAnything" }]
    }
  }
}
```

### Create and start a podman machine

Run a terminal **as an administrator** and execute the following commands to create a Podman machine:

```
.\bin\windows\podman.exe machine init
```

When `machine init` completes run `machine start`:

```
.\bin\windows\podman.exe machine start
```

### Run a container using podman

The locally built Podman client for Windows can now be used to run containers:

```
.\bin\windows\podman.exe run hello-world
```

:information_source: Unlike the previous machine commands, this one doesn't require administrative privileges.

## Building the Podman client and installer with MSYS2

### Install msys2

Podman requires brew -- a collection of Unix like build tools and libraries adapted for Windows. More details and
installation instructions are available from their [home page](https://www.msys2.org/). There are also premade GitHub
actions for this tool that are available.

### Install build dependencies

Podman requires some software from msys2 to be able to build. This can be done using msys2 shell. One can start it
from the Start menu. This documentation covers only usage of MSYS2 UCRT64 shell (msys2 shells come preconfigured for
different [environments](https://www.msys2.org/docs/environments/)).

```
$ pacman -S git make zip mingw-w64-ucrt-x86_64-gcc mingw-w64-ucrt-x86_64-go mingw-w64-ucrt-x86_64-python
```

The Pandoc tool installed in a prior step is specific, that is the installer doesn't add the tool to any PATH environment
variable known to msys2, so, it has to be linked explicitly to work.

```
$ mkdir -p /usr/local/bin
$ ln -sf "/c/Program Files/Pandoc/pandoc.exe" "/usr/local/bin/pandoc.exe"
```

### Restart shell (important)

One needs to restart the [msys2](https://www.msys2.org/) shell after dependency installation before proceeding with the build.

### Obtain Podman source code

One can obtain the latest source code for Podman from its [GitHub](https://github.com/containers/podman) repository.

```
$ git clone https://github.com/containers/podman.git go/src/github.com/containers/podman
```

### Build client

After completing the preparatory steps of obtaining the Podman source code and installing its dependencies, the client
can now be built.

```
$ cd go/src/github.com/containers/podman
$ make clean podman-remote-release-windows_amd64.zip
```

The complete distribution will be packaged to the `podman-remote-release-windows_amd64.zip` file. It is possible to
unzip it and replace files in the default Podman installation with the built ones to use the custom build.

#### Build client only (for faster feedback loop)

Building Podman by following this documentation can take a fair amount of time and effort. Packaging the installer adds even more overhead. If
the only needed artifact is the Podman binary itself, it is possible to build only it with this command:

```
$ make podman-remote
```

The binary will be located in `bin/windows/`. It could be used as drop in replacement for the installed version of
Podman.

It is also possible to cross-build for other platforms by providing GOOS and GOARCH environment variables.

### Build client installer

As Windows requires more effort in comparison to Unix systems for installation procedures, it is sometimes
easier to pack the changes into a ready-to-use installer. To create the installer, the full client distribution in ZIP
format has to be built beforehand.

```
$ export BUILD_PODMAN_VERSION=$(test/version/version | sed 's/-.*//')
$ mkdir -p contrib/win-installer/current
$ cp podman-remote-release-windows_amd64.zip contrib/win-installer/current/
$ cd contrib/win-installer
$ powershell -ExecutionPolicy Bypass -File build.ps1 $BUILD_PODMAN_VERSION dev current
```

The installer will be located in the `contrib/win-installer` folder (relative to checkout root) and will have a name
like `podman-4.5.0-dev-setup.exe`. This could be installed in a similar manner as the official Podman for Windows installers
(when installing unsigned binaries is allowed on the host).

## Using the client

To learn how to use the Podman client, refer to its
[tutorial](https://github.com/containers/podman/blob/main/docs/tutorials/remote_client.md).
