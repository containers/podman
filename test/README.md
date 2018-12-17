![PODMAN logo](../logo/podman-logo-source.svg)
# Test utils
Test utils provide common functions and structs for testing. It includes two structs:
* `PodmanTest`: Handle the *podman* command and other global resources like temporary
directory. It provides basic methods, like checking podman image and pod status. Test
suites should create their owner test *struct* as a composite of `PodmanTest`, and their
owner PodmanMakeOptions().

* `PodmanSession`: Store execution session data and related *methods*. Such like get command
output and so on. It can be used directly in the test suite, only embed it to your owner
session struct if you need expend it.

## Unittest for test/utils
To ensure neither *tests* nor *utils* break, There are unit-tests for each *functions* and
*structs* in `test/utils`. When you adding functions or structs to this *package*, please
update both unit-tests for it and this documentation.

### Run unit test for test/utils
Run unit test for test/utils.

```
make localunit
```

## Structure of the test utils and test suites
The test *utils* package is at the same level of test suites. Each test suites also have their
owner common functions and structs stored in `libpod_suite_test.go`.

# Ginkgo test framework
[Ginkgo](https://github.com/onsi/ginkgo) is a BDD testing framework. This allows
us to use native Golang to perform our tests and there is a strong affiliation
between Ginkgo and the Go test framework.

## Installing dependencies
The dependencies for integration really consists of three things:
* ginkgo binary
* ginkgo sources
* gomega sources

The following instructions assume your GOPATH is ~/go. Adjust as needed for your
environment.

### Installing ginkgo
Fetch and build ginkgo with the following command:
```
GOPATH=~/go go get -u github.com/onsi/ginkgo/ginkgo
```
Now install the ginkgo binary into your path:
```
install -D -m 755 "$GOPATH"/bin/ginkgo /usr/bin/
```
You now have a ginkgo binary and its sources in your GOPATH.

### Install gomega sources
The gomega sources can be simply installed with the command:
```
GOPATH=~/go go get github.com/onsi/gomega/...
```

# Integration Tests
Test suite for integration test for podman command line. It has its own structs:
* `PodmanTestIntegration`: Integration test *struct* as a composite of `PodmanTest`. It
set up the global options for *podman* command to ignore the environment influence from
different test system.

* `PodmanSessionIntegration`: This *struct* has it own *methods* for checking command
output with given format JSON by using *structs* defined in inspect package.

## Running the integration tests
You can run the entire suite of integration tests with the following command:

```
GOPATH=~/go ginkgo -v test/e2e/.
```

Note the trailing period on the command above. Also, **-v** invokes verbose mode.  That
switch is optional.

You can run a single file of integration tests using the go test command:

```
GOPATH=~/go go test -v test/e2e/libpod_suite_test.go test/e2e/config.go test/e2e/config_amd64.go test/e2e/your_test.go
```

#### Run all tests like PAPR
You can closely emulate the PAPR run for Fedora with the following command:

```
make integration.fedora
```

This will run lint, git-validation, and gofmt tests and then execute unit and integration
tests as well.

### Run tests in a container
In case you have issue running the tests locally on your machine, you can run
them in a container:
```
make shell
```

This will run a container and give you a shell and you can follow the instructions above.

# System test
System tests are used for testing the *podman* CLI in the context of a complete system. It
requires that *podman*, all dependencies, and configurations are in place.  The intention of
system testing is to match as closely as possible with real-world user/developer use-cases
and environments. The orchestration of the environments and tests is left to external
tooling.

* `PodmanTestSystem`: System test *struct* as a composite of `PodmanTest`. It will not add any
options to the command by default. When you run system test, you can set GLOBALOPTIONS,
PODMAN_SUBCMD_OPTIONS or PODMAN_BINARY in ENV to run the test suite for different test matrices.

## Run system test
You can run the test with following command:

```
make localsystem
```
