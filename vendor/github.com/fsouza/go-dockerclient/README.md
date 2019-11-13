# go-dockerclient

[![Travis Build Status](https://travis-ci.com/fsouza/go-dockerclient.svg?branch=master)](https://travis-ci.com/fsouza/go-dockerclient)
[![AppVeyor Build status](https://ci.appveyor.com/api/projects/status/4yusq1f9dqbicobt?svg=true)](https://ci.appveyor.com/project/fsouza/go-dockerclient)
[![GoDoc](https://img.shields.io/badge/api-Godoc-blue.svg?style=flat-square)](https://godoc.org/github.com/fsouza/go-dockerclient)

This package presents a client for the Docker remote API. It also provides
support for the extensions in the [Swarm API](https://docs.docker.com/swarm/swarm-api/).

This package also provides support for docker's network API, which is a simple
passthrough to the libnetwork remote API.

For more details, check the [remote API
documentation](https://docs.docker.com/engine/api/latest/).

## Difference between go-dockerclient and the official SDK

Link for the official SDK: https://docs.docker.com/develop/sdk/

go-dockerclient was created before Docker had an official Go SDK and is
still maintained and active because it's still used out there. New features in
the Docker API do not get automatically implemented here: it's based on demand,
if someone wants it, they can file an issue or a PR and the feature may get
implemented/merged.

For new projects, using the official SDK is probably more appropriate as
go-dockerclient lags behind the official SDK.

When using the official SDK, keep in mind that because of how the its
dependencies are organized, you may need some extra steps in order to be able
to import it in your projects (see
[#784](https://github.com/fsouza/go-dockerclient/issues/784) and
[moby/moby#28269](https://github.com/moby/moby/issues/28269)).

## Example

```go
package main

import (
	"fmt"

	docker "github.com/fsouza/go-dockerclient"
)

func main() {
	client, err := docker.NewClientFromEnv()
	if err != nil {
		panic(err)
	}
	imgs, err := client.ListImages(docker.ListImagesOptions{All: false})
	if err != nil {
		panic(err)
	}
	for _, img := range imgs {
		fmt.Println("ID: ", img.ID)
		fmt.Println("RepoTags: ", img.RepoTags)
		fmt.Println("Created: ", img.Created)
		fmt.Println("Size: ", img.Size)
		fmt.Println("VirtualSize: ", img.VirtualSize)
		fmt.Println("ParentId: ", img.ParentID)
	}
}
```

## Using with TLS

In order to instantiate the client for a TLS-enabled daemon, you should use
NewTLSClient, passing the endpoint and path for key and certificates as
parameters.

```go
package main

import (
	"fmt"

	docker "github.com/fsouza/go-dockerclient"
)

func main() {
	const endpoint = "tcp://[ip]:[port]"
	path := os.Getenv("DOCKER_CERT_PATH")
	ca := fmt.Sprintf("%s/ca.pem", path)
	cert := fmt.Sprintf("%s/cert.pem", path)
	key := fmt.Sprintf("%s/key.pem", path)
	client, _ := docker.NewTLSClient(endpoint, cert, key, ca)
	// use client
}
```

If using [docker-machine](https://docs.docker.com/machine/), or another
application that exports environment variables `DOCKER_HOST`,
`DOCKER_TLS_VERIFY`, `DOCKER_CERT_PATH`, `DOCKER_API_VERSION`, you can use
NewClientFromEnv.


```go
package main

import (
	"fmt"

	docker "github.com/fsouza/go-dockerclient"
)

func main() {
	client, err := docker.NewClientFromEnv()
	if err != nil {
		// handle err
	}
	// use client
}
```

See the documentation for more details.

## Developing

All development commands can be seen in the [Makefile](Makefile).

Commited code must pass:

* [golangci-lint](https://github.com/golangci/golangci-lint)
* [go test](https://golang.org/cmd/go/#hdr-Test_packages)

Running ``make test`` will run all checks, as well as install any required
dependencies.

## Modules

go-dockerclient supports Go modules.

If you're using dep, you can check the [releases
page](https://github.com/fsouza/go-dockerclient/releases) for the latest
release fully compatible with dep.

With other vendoring tools, users need to specify go-dockerclient's
dependencies manually.

## Using with Docker 1.9 and Go 1.4

There's a tag for using go-dockerclient with Docker 1.9 (which requires
compiling go-dockerclient with Go 1.4), the tag name is ``docker-1.9/go-1.4``.

The instructions below can be used to get a version of go-dockerclient that compiles with Go 1.4:

```
% git clone -b docker-1.9/go-1.4 https://github.com/fsouza/go-dockerclient.git $GOPATH/src/github.com/fsouza/go-dockerclient
% git clone -b v1.9.1 https://github.com/docker/docker.git $GOPATH/src/github.com/docker/docker
% go get github.com/fsouza/go-dockerclient
```
