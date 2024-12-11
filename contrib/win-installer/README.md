# Windows Installer Build

Instructions [have moved here](Build and test the Podman Windows installer](#build-and-test-the-podman-windows-installer)).

## How to run a full tests scenarios

Manual tests to validate changes the wxs files or a WiX upgrade.

## Preparation

- checkout previous release tag (e.g. 5.3.0)
`git fetch --all --tags --prune && git tag --list v5.* && git checkout tags/v5.3.0`
- make the installer
`./winmake podman && ./winmake docs && ./winmake win-gvproxy && ./winmake installer`
- checkout tag `v5.3.1` make the installer
`./winmake podman && ./winmake docs && ./winmake win-gvproxy && ./winmake installer`
- get the `v5.3.1` msi product id (with superorca)
- checkout the main branch and change the product id on `podman.wxs` to match `v5.3.1` product id
- set `$env:V531_SETUP_EXE_PATH` and make current and next installer
`$env:V531_SETUP_EXE_PATH=<path> && ./winmake podman && ./winmake docs && ./winmake win-gvproxy && ./winmake installer && ./winmake installer 9.9.9`
- patch installertest to make sure it doesn't download the setup.exe from internet but uses the one just built

## Run the tests

1. Uninstall the virtualization providers (WSL and Hyper-V) using the "Windows Features" app
2. Run installtest for both `wsl` and `hyperv` (**as an admin**)
```pwsh
.\contrib\win-installer\test-installer.ps1 `
    -scenario all `
    -setupExePath ".\contrib\win-installer\podman-5.4.0-dev-setup.exe" `
    -previousSetupExePath ".\contrib\win-installer\podman-5.3.0-dev-setup.exe" `
    -nextSetupExePath ".\contrib\win-installer\podman-9.9.9-dev-setup.exe" `
    -v531SetupExePath ".\contrib\win-installer\podman-5.3.1-dev-setup.exe" `
    -provider hyperv
```
3. Manually test the upgrade "from v5.3.1 to current to next"
```pwsh
contrib\win-installer\podman-5.3.1-dev-setup.exe /install /log contrib\win-installer\podman-setup-531.log
contrib\win-installer\podman-5.4.0-dev-setup.exe /install /log contrib\win-installer\podman-setup-540.log
contrib\win-installer\podman-9.9.9-dev-setup.exe /install /log contrib\win-installer\podman-setup-999.log
contrib\win-installer\podman-9.9.9-dev-setup.exe /x /log contrib\win-installer\podman-uninstall-999.log
```
4. manually run the current installer with the option to install wsl and confirm it reboots and install both podman and wsl
5. manually run the current installer with the option to install hyperv and confirm it reboots and install both podman and wsl
6. run installtest for both wsl and hyperv
7. manually run the current installer with the option to install wsl and confirm it doesn't reboot
8. manually run the current installer with the option to install hyperv and confirm it doesn't reboot

## retrieve installed podman msi package information

```pwsh
$Installer = New-Object -ComObject WindowsInstaller.Installer;
$InstallerProducts = $Installer.ProductsEx("", "", 7);
$InstalledProducts = ForEach($Product in $InstallerProducts){
    [PSCustomObject]@{ProductCode = $Product.ProductCode();
                      LocalPackage = $Product.InstallProperty("LocalPackage");
                      VersionString = $Product.InstallProperty("VersionString");
                      ProductName = $Product.InstallProperty("ProductName")
                      }
};
$InstalledProducts | Where-Object {$_.ProductName -match "podman"}
```

and uninstall it with `msiexec /x "{<product-code>}"`
