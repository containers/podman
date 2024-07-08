# Running the machine tests

This document is a quick how-to run machine tests.  Not all dependencies, like
`gvproxy` are documented. You must install `gvproxy` in all cases described below.

## General notes

### Environment must be clean
You must not have any machines defined before running tests.  Consider running `podman machine reset` prior to running tests.

### Scoping tests
You can scope tests in the machine suite by adding various incantations of `FOCUS=`.  For example, add `FOCUS_FILE=basic_test.go` to only run basic test. Or add `FOCUS="simple init with start"` to only run one test case. For windows, the syntax differs slightly.  In windows, executing something like following achieves the same result:

`./winmake localmachine "basic_test.go start_test.go"`

## Linux

### QEMU
1. `make localmachine`


## Microsoft Windows

### Hyper-V
1. Open a powershell as admin
1. `$env:CONTAINERS_MACHINE_PROVIDER="hyperv"`
1. `./winmake localmachine`


### WSL
1. Open a powershell as a regular user
1. Build and copy win-sshproxy into bin/
1. `./winmake localmachine`

## MacOS
Macs now support two different machine providers: `applehv` and `libkrun`.  The `applehv` provider is the default.

Note: On macOS, an error will occur if the path length of `$TMPDIR` is longer than 22 characters. Please set the appropriate path to `$TMPDIR`. Also, if `$TMPDIR` is empty, `/private/tmp` will be set.

### Apple Hypervisor
1. `brew install vfkit`
1. `make podman-remote`
1. `make localmachine`


### [Libkrun](https://github.com/containers/libkrun)
1. `brew install krunkit`
1. `make podman-remote`
1. `export CONTAINERS_MACHINE_PROVIDER="libkrun"`
1. `make localmachine`
