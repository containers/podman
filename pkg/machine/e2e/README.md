# Working README for running the machine tests

Note: you must not have any machines defined before running tests
## Linux

### QEMU

`make localmachine`

## Microsoft Windows

### Hyper-V

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

## macOS

### Apple Hypervisor

1. `make podman-remote`
1. `make localmachine` (Add `FOCUS_FILE=basic_test.go` to only run basic test. Or add `FOCUS="simple init with start"` to only run one test case)

Note: On macOS, an error will occur if the path length of `$TMPDIR` is longer than 22 characters. Please set the appropriate path to `$TMPDIR`. Also, if `$TMPDIR` is empty, `/private/tmp` will be set.
