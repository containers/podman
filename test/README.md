# Integration testing

Our primary means of performing integration testing for libpod is with the
[Ginkgo](https://github.com/onsi/ginkgo) BDD testing framework. This allows
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

### Running the integration tests

You can run the entire suite of integration tests with the following command:

```
GOPATH=~/go ginkgo -v tests/e2e/.
```

Note the trailing period on the command above. Also, **-v** invokes verbose mode.  That
switch is optional.

You can run a single file of integration tests using the go test command:

```
GOPATH=~/go go test -v tests/e2e/libpod_suite_test.go tests/e2e/your_test.go
```

#### Run all tests like PAPR
You can closely emulate the PAPR run for Fedora with the following command:

```
make integration.fedora
```

This will run lint, git-validation, and gofmt tests and then execute unit and integration
tests as well.
