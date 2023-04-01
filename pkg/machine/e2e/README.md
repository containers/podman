# Working README for running the machine tests

Note: you must not have any machines defined before running tests
## Linux

### QEMU

`make localmachine`

## Microsoft Windows

### HyperV

1. Open a powershell as admin
1. $env:CONTAINERS_MACHINE_PROVIDER="hyperv"
1. `./winmake localmachine`

Note: To run specific test files, add the test files to the end of the winmake command:

`./winmake localmachine "basic_test.go start_test.go"`

### WSL
1. Open a powershell as a regular user
1. Build and copy win-sshproxy into bin/
1. `./winmake localmachine`

Note: To run specific test files, add the test files to the end of the winmake command:

`./winmake localmachine "basic_test.go start_test.go"`

### QEMU
1. Install QEMU and add it to either user or sysmem PATH variable
1. Install Podman release (needed to have gvproxy binary)
1. Open a powershell as a regular user
1. $env:CONTAINERS_MACHINE_PROVIDER="qemu"
1. `./winmake localmachine`

Note: To run specific test files, add the test files to the end of the winmake command:

`./winmake localmachine "basic_test.go start_test.go"`

## MacOS

### Apple Hypervisor

1. `make podman-remote`
1. `export CONTAINERS_MACHINE_PROVIDER="applehv"`
1. `export MACHINE_IMAGE="https://fedorapeople.org/groups/podman/testing/applehv/arm64/fedora-coreos-38.20230925.dev.0-applehv.aarch64.raw.gz"`
1. `make localmachine` (Add `FOCUS_FILE=basic_test.go` to only run basic test)
