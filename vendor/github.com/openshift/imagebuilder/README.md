Docker / OCI Image Builder
==========================

[![Go Report Card](https://goreportcard.com/badge/github.com/openshift/imagebuilder)](https://goreportcard.com/report/github.com/openshift/imagebuilder)
[![GoDoc](https://godoc.org/github.com/openshift/imagebuilder?status.png)](https://godoc.org/github.com/openshift/imagebuilder)
[![Travis](https://travis-ci.org/openshift/imagebuilder.svg?branch=master)](https://travis-ci.org/openshift/imagebuilder)
[![Join the chat at freenode:openshift-dev](https://img.shields.io/badge/irc-freenode%3A%20%23openshift--dev-blue.svg)](http://webchat.freenode.net/?channels=%23openshift-dev)

Note: this library is beta and may contain bugs that prevent images from being identical to Docker build.  Test your images (and add to our conformance suite)!

This library supports using the Dockerfile syntax to build Docker
compatible images, without invoking Docker build. It is intended to give
clients more control over how a Docker build is run, including:

* Instead of building one layer per line, run all instructions in the
  same container
* Set Docker HostConfig settings like network and memory controls that
  are not available when running Docker builds
* Mount external files into the build that are not persisted as part of
  the final image (i.e. "secrets")
* If there are no RUN commands in the Dockerfile, the container is created
  and committed, but never started.

The final image should be 99.9% compatible with regular docker builds,
but bugs are always possible.

Future goals include:

* Output OCI compatible images
* Support other container execution engines, like runc or rkt
* Better conformance testing
* Windows support

## Install and Run

To download and install the library and the binary, set up a Golang build environment and with `GOPATH` set run:

```
$ go get -u github.com/openshift/imagebuilder/cmd/imagebuilder
```

The included command line takes one argument, a path to a directory containing a Dockerfile. The `-t` option
can be used to specify an image to tag as:

```
$ imagebuilder [-t TAG] DIRECTORY
```

To mount a file into the image for build that will not be present in the final output image, run:

```
$ imagebuilder --mount ~/secrets/private.key:/etc/keys/private.key path/to/my/code testimage
```

Any processes in the Dockerfile will have access to `/etc/keys/private.key`, but that file will not be part of the committed image.

Running `--mount` requires Docker 1.10 or newer, as it uses a Docker volume to hold the mounted files and the volume API was not
available in earlier versions.

You can also customize which Dockerfile is run, or run multiple Dockerfiles in sequence (the FROM is ignored on
later files):

```
$ imagebuilder -f Dockerfile:Dockerfile.extra .
```

will build the current directory and combine the first Dockerfile with the second. The FROM in the second image
is ignored.


## Code Example

```
f, err := os.Open("path/to/Dockerfile")
if err != nil {
	return err
}
defer f.Close()

e := builder.NewClientExecutor(o.Client)
e.Out, e.ErrOut = os.Stdout, os.Stderr
e.AllowPull = true
e.Directory = "context/directory"
e.Tag = "name/of-image:and-tag"
e.AuthFn = nil // ... pass a function to retrieve authorization info
e.LogFn = func(format string, args ...interface{}) {
	fmt.Fprintf(e.ErrOut, "--> %s\n", fmt.Sprintf(format, args...))
}

buildErr := e.Build(f, map[string]string{"arg1":"value1"})
if err := e.Cleanup(); err != nil {
	fmt.Fprintf(e.ErrOut, "error: Unable to clean up build: %v\n", err)
}

return buildErr
```

Example of usage from OpenShift's experimental `dockerbuild` [command with mount secrets](https://github.com/openshift/origin/blob/26c9e032ff42f613fe10649cd7c5fa1b4c33501b/pkg/cmd/cli/cmd/dockerbuild/dockerbuild.go)

## Run conformance tests (very slow):

```
go test ./dockerclient/conformance_test.go -tags conformance
```
