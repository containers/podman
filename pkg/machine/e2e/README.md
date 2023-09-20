# Working README for running the machine tests


## Linux

### QEMU

`make localmachine`

## Microsoft Windows

### HyperV

1. Open a powershell as admin
1. $env:CONTAINERS_MACHINE_PROVIDER="hyperv"
1. $env:MACHINE_IMAGE="https://fedorapeople.org/groups/podman/testing/hyperv/fedora-coreos-38.20230830.dev.0-hyperv.x86_64.vhdx.zip"
1. `./test/tools/build/ginkgo.exe -vv  --tags "remote exclude_graphdriver_btrfs btrfs_noversion exclude_graphdriver_devicemapper containers_image_openpgp remote" -timeout=90m --trace --no-color  pkg/machine/e2e/. `

Note: Add `--focus-file "basic_test.go" ` to only run basic test

### WSL
1. Open a powershell as a regular user
1. Build and copy win-sshproxy into bin/
1. `./test/tools/build/ginkgo.exe -vv  --tags "remote exclude_graphdriver_btrfs btrfs_noversion exclude_graphdriver_devicemapper containers_image_openpgp remote" -timeout=90m --trace --no-color  pkg/machine/e2e/. `

Note: Add `--focus-file "basic_test.go" ` to only run basic test
