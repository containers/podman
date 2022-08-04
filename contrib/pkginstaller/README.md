## How to build

```sh
$ make ARCH=<amd64 | aarch64> NO_CODESIGN=1 pkginstaller

# or to create signed pkg
$ make ARCH=<amd64 | aarch64> CODESIGN_IDENTITY=<ID> PRODUCTSIGN_IDENTITY=<ID> pkginstaller
```

The generated pkg will be written to `out/podman-macos-installer-*.pkg`.
Currently the pkg installs `podman`, `qemu`, `gvproxy` and `podman-mac-helper` to `/Applications/podman`

The `qemu` build it uses is from [containers/podman-machine-qemu](https://github.com/containers/podman-machine-qemu)

## Uninstalling

```sh
$ sudo rm -rf /opt/podman
```

### Screenshot
<img width="626" alt="screenshot-macOS-pkg-podman" src="https://user-images.githubusercontent.com/8885742/157380992-2e3b1573-34a0-4aa0-bdc1-a85f4792a1d2.png">
