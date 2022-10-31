# Cap'n Proto bindings for Go

![License](https://img.shields.io/badge/license-MIT-brightgreen?style=flat-square)
[![CodeQuality](https://goreportcard.com/badge/capnproto.org/go/capnp)](https://goreportcard.com/report/capnproto.org/go/capnp/v3)
[![Go](https://github.com/capnproto/go-capnproto2/actions/workflows/go.yml/badge.svg)](https://github.com/capnproto/go-capnproto2/actions/workflows/go.yml)
[![GoDoc](https://godoc.org/capnproto.org/go/capnp/v3?status.svg)][godoc]
[![Matrix](https://img.shields.io/matrix/go-capnp:matrix.org?color=lightpink&label=Get%20Help&logo=matrix&style=flat-square)](https://matrix.to/#/#go-capnp:matrix.org)

[Cap’n Proto](https://capnproto.org/) is an insanely fast data interchange format similar to [Protocol Buffers](https://github.com/protocolbuffers/protobuf), but much faster.

It also includes a sophisticated RPC system based on [Object Capabilities](https://en.wikipedia.org/wiki/Object-capability_model), ideal for secure, low-latency applications.

This package provides:
- Go code-generation for Cap'n Proto
- Runtime support for the Go language
- Level 1 support for the [Cap'n Proto RPC](https://capnproto.org/rpc.html) protocol

Support for Level 3 RPC is [planned](https://github.com/capnproto/go-capnproto2/issues/160).

[godoc]: http://pkg.go.dev/capnproto.org/go/capnp/v3
## Installation

#### To interact with pre-compiled schemas

Ensure that Go modules are enabled, then run the following command:
```
$ go get capnproto.org/go/capnp/v3
```

#### To compile Cap'n Proto schema files to Go

Two additional steps are needed to compile `.capnp` files to Go:

1. [Install the Cap'n Proto tools](https://capnproto.org/install.html).
2. Install the Go language bindings by running:
  ```bash
  go install capnproto.org/go/capnp/v3/capnpc-go@latest  # install go compiler plugin
  GO111MODULE=off go get -u capnproto.org/go/capnp/v3/  # install go-capnproto to $GOPATH
  ```

To learn how to compile a simple schema, [click here](docs/Getting-Started.md#compiling-schema-files).

This package has been tested with version `0.8.0` of the `capnp` tool.

## Documentation

### Getting Started

Read the ["Getting Started" guide][getting-started] for a high-level introduction to the package API and workflow.
A minimal working RPC example can be found
[here](docs/Getting-Started.md#remote-calls-using-interfaces).

Browse rest of the [Wiki](https://github.com/capnproto/go-capnproto2/wiki) for in depth explanations of concepts, migration guides, and tutorials.

### Help and Support

You can find us on Matrix:   [Go Cap'n Proto](https://matrix.to/#/!pLcnVUHHRZrUPscloW:matrix.org?via=matrix.org)

### API Reference

Available on [GoDoc](http://pkg.go.dev/capnproto.org/go/capnp/v3).

## API Compatibility

Until the official Cap'n Proto spec is finalized, this repository should be considered <u>beta software</u>.

We use [semantic versioning](https://semver.org) to track compatibility and signal breaking changes.  In the spirit of the [Go 1 compatibility guarantee][gocompat], we will make every effort to avoid making breaking API changes within major version numbers, but nevertheless reserve the right to introduce breaking changes for reasons related to:

- Security.
- Changes in the Cap'n Proto specification.
- Bugs.

An exception to this rule is currently in place for the `pogs` package, which is relatively new and may change over time.  However, its functionality has been well-tested, and breaking changes are relatively unlikely.

Note also we may merge breaking changes to the `main` branch without notice.  Users are encouraged to pin their dependencies to a major version, e.g. using the semver-aware features of `go get`.

[gocompat]: https://golang.org/doc/go1compat
## License

MIT - see [LICENSE][] file

[LICENSE]: https://github.com/capnproto/go-capnproto2/blob/master/LICENSE

[getting-started]: docs/Getting-Started.md
