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

## Install WiX Toolset v3 (is preinstalled in Github runner)
The latest release of the WiX Toolset can be obtained from https://wixtoolset.org/docs/wix3/.  Installing it into a clean VM might require
an additional installation of .NET Framework 3.5 in advance
([instructions for adding .NET Framework 3.5 via enabling the Windows feature](https://learn.microsoft.com/en-us/dotnet/framework/install/dotnet-35-windows#enable-the-net-framework-35-in-control-panel))

## Install msys2

Podman requires brew -- a collection of Unix like build tools and libraries adapted for Windows. More details and
installation instructions are available from their [home page](https://www.msys2.org/). There are also premade GitHub
actions for this tool that are available.

## Install build dependencies

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

## Restart shell (important)

One needs to restart the [msys2](https://www.msys2.org/) shell after dependency installation before proceeding with the build.

## Obtain Podman source code

One can obtain the latest source code for Podman from its [GitHub](https://github.com/containers/podman) repository.

```
$ git clone https://github.com/containers/podman.git go/src/github.com/containers/podman
```

## Build client

After completing the preparatory steps of obtaining the Podman source code and installing its dependencies, the client
can now be built.

```
$ cd go/src/github.com/containers/podman
$ make clean podman-remote-release-windows_amd64.zip
```

The complete distribution will be packaged to the `podman-remote-release-windows_amd64.zip` file. It is possible to
unzip it and replace files in the default Podman installation with the built ones to use the custom build.

### Build client only (for faster feedback loop)

Building Podman by following this documentation can take a fair amount of time and effort. Packaging the installer adds even more overhead. If
the only needed artifact is the Podman binary itself, it is possible to build only it with this command:

```
$ make podman-remote
```

The binary will be located in `bin/windows/`. It could be used as drop in replacement for the installed version of
Podman.

It is also possible to cross-build for other platforms by providing GOOS and GOARCH environment variables.

## Build client installer

As Windows requires more effort in comparison to Unix  systems for installation procedures, it is sometimes
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
