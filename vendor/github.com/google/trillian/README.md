# Trillian: General Transparency

[![Go Report Card](https://goreportcard.com/badge/github.com/google/trillian)](https://goreportcard.com/report/github.com/google/trillian)
[![codecov](https://codecov.io/gh/google/trillian/branch/master/graph/badge.svg?token=QwofUwmvAs)](https://codecov.io/gh/google/trillian)
[![GoDoc](https://godoc.org/github.com/google/trillian?status.svg)](https://godoc.org/github.com/google/trillian)
[![Slack Status](https://img.shields.io/badge/Slack-Chat-blue.svg)](https://gtrillian.slack.com/)

 - [Overview](#overview)
 - [Support](#support)
 - [Using the Code](#using-the-code)
     - [MySQL Setup](#mysql-setup)
     - [Integration Tests](#integration-tests)
 - [Working on the Code](#working-on-the-code)
     - [Rebuilding Generated Code](#rebuilding-generated-code)
     - [Updating Dependencies](#updating-dependencies)
     - [Running Codebase Checks](#running-codebase-checks)
 - [Design](#design)
     - [Design Overview](#design-overview)
     - [Personalities](#personalities)
     - [Log Mode](#log-mode)
 - [Use Cases](#use-cases)
     - [Certificate Transparency Log](#certificate-transparency-log)


## Overview

Trillian is an implementation of the concepts described in the
[Verifiable Data Structures](docs/papers/VerifiableDataStructures.pdf) white paper,
which in turn is an extension and generalisation of the ideas which underpin
[Certificate Transparency](https://certificate-transparency.org).

Trillian implements a [Merkle tree](https://en.wikipedia.org/wiki/Merkle_tree)
whose contents are served from a data storage layer, to allow scalability to
extremely large trees.  On top of this Merkle tree, Trillian provides the
following:

 - An append-only **Log** mode, analogous to the original
   [Certificate Transparency](https://certificate-transparency.org) logs.  In
   this mode, the Merkle tree is effectively filled up from the left, giving a
   *dense* Merkle tree.

Note that Trillian requires particular applications to provide their own
[personalities](#personalities) on top of the core transparent data store
functionality.

[Certificate Transparency (CT)](https://tools.ietf.org/html/rfc6962)
is the most well-known and widely deployed transparency application, and an implementation of CT as a Trillian personality is available in the
[certificate-transparency-go repo](https://github.com/google/certificate-transparency-go/blob/master/trillian).

Other examples of Trillian personalities are available in the
[trillian-examples](https://github.com/google/trillian-examples) repo.


## Support

- Mailing list: https://groups.google.com/forum/#!forum/trillian-transparency
- Slack: https://gtrillian.slack.com/ ([invitation](https://join.slack.com/t/gtrillian/shared_invite/enQtNDM3NTE3NjA4NDcwLTMwYzVlMDUxMDQ2MGU5MjcyZGIxMmVmZGNlNzdhMzRlOGFjMWJkNzc0MGY1Y2QyNWQyMWM4NzJlOGMxNTZkZGU))


## Using the Code

The Trillian codebase is stable and is used in production by multiple
organizations, including many large-scale 
[Certificate Transparency](https://certificate.transparency.dev) log
operators.

Given this, we do not plan to add any new features to this version of Trillian,
and will try to avoid any further incompatible code and schema changes but
cannot guarantee that they will never be necessary.

The current state of feature implementation is recorded in the
[Feature implementation matrix](docs/Feature_Implementation_Matrix.md).

To build and test Trillian you need:

 - Go 1.19 or later (go 1.19 matches cloudbuild, and is preferred for developers
   that will be submitting PRs to this project).

To run many of the tests (and production deployment) you need:

 - [MySQL](https://www.mysql.com/) or [MariaDB](https://mariadb.org/) to provide
   the data storage layer; see the [MySQL Setup](#mysql-setup) section.

Note that this repository uses Go modules to manage dependencies; Go will fetch
and install them automatically upon build/test.

To fetch the code, dependencies, and build Trillian, run the following:

```bash
git clone https://github.com/google/trillian.git
cd trillian

go build ./...
```

To build and run tests, use:

```bash
go test ./...
```


The repository also includes multi-process integration tests, described in the
[Integration Tests](#integration-tests) section below.


### MySQL Setup

To run Trillian's integration tests you need to have an instance of MySQL
running and configured to:

 - listen on the standard MySQL port 3306 (so `mysql --host=127.0.0.1
   --port=3306` connects OK)
 - not require a password for the `root` user

You can then set up the [expected tables](storage/mysql/schema/storage.sql) in a
`test` database like so:

```bash
./scripts/resetdb.sh
Warning: about to destroy and reset database 'test'
Are you sure? y
> Resetting DB...
> Reset Complete
```

### Integration Tests

Trillian includes an integration test suite to confirm basic end-to-end
functionality, which can be run with:

```bash
./integration/integration_test.sh
```

This runs a multi-process test:

 - A [test](integration/log_integration_test.go) that starts a Trillian server
   in Log mode, together with a signer, logs many leaves, and checks they are
   integrated correctly.
 
### Deployment

You can find instructions on how to deploy Trillian in [deployment](/deployment)
and [examples/deployment](/examples/deployment) directories.

## Working on the Code

Developers who want to make changes to the Trillian codebase need some
additional dependencies and tools, described in the following sections. The
[Cloud Build configuration](cloudbuild.yaml) and the scripts it depends on are
also a useful reference for the required tools and scripts, as it may be more
up-to-date than this document.

### Rebuilding Generated Code

Some of the Trillian Go code is autogenerated from other files:

 - [gRPC](http://www.grpc.io/) message structures are originally provided as
   [protocol buffer](https://developers.google.com/protocol-buffers/) message
   definitions. See also, https://grpc.io/docs/protoc-installation/.
 - Some unit tests use mock implementations of interfaces; these are created
   from the real implementations by [GoMock](https://github.com/golang/mock).
 - Some enums have string-conversion methods (satisfying the `fmt.Stringer`
   interface) created using the
   [stringer](https://godoc.org/golang.org/x/tools/cmd/stringer) tool (`go get
   golang.org/x/tools/cmd/stringer`).

Re-generating mock or protobuffer files is only needed if you're changing
the original files; if you do, you'll need to install the prerequisites:

  - a series of tools, using `go install` to ensure that the versions are
    compatible and tested:

    ```
    cd $(go list -f '{{ .Dir }}' github.com/google/trillian); \
    go install github.com/golang/mock/mockgen; \
    go install google.golang.org/protobuf/proto; \
    go install google.golang.org/protobuf/cmd/protoc-gen-go; \
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc; \
    go install github.com/pseudomuto/protoc-gen-doc/cmd/protoc-gen-doc; \
    go install golang.org/x/tools/cmd/stringer
    ```

and run the following:

```bash
go generate -x ./...  # hunts for //go:generate comments and runs them
```

### Updating Dependencies

The Trillian codebase uses go.mod to declare fixed versions of its dependencies. 
With Go modules, updating a dependency simply involves running `go get`:
```
go get package/path       # Fetch the latest published version
go get package/path@X.Y.Z # Fetch a specific published version
go get package/path@HEAD  # Fetch the latest commit 
```

To update ALL dependencies to the latest version run `go get -u`. 
Be warned however, that this may undo any selected versions that resolve issues in other non-module repos. 

While running `go build` and `go test`, go will add any ambiguous transitive dependencies to `go.mod`
To clean these up run:
```
go mod tidy
```

### Running Codebase Checks

The [`scripts/presubmit.sh`](scripts/presubmit.sh) script runs various tools
and tests over the codebase.

#### Install [golangci-lint](https://github.com/golangci/golangci-lint#local-installation).
```bash
go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.51.1
```

#### Run code generation, build, test and linters
```bash
./scripts/presubmit.sh
```

#### Or just run the linters alone
```bash
golangci-lint run
```


## Design

### Design Overview

Trillian is primarily implemented as a
[gRPC service](http://www.grpc.io/docs/guides/concepts.html#service-definition);
this service receives get/set requests over gRPC and retrieves the corresponding
Merkle tree data from a separate storage layer (currently using MySQL), ensuring
that the cryptographic properties of the tree are preserved along the way.

The Trillian service is multi-tenanted – a single Trillian installation can
support multiple Merkle trees in parallel, distinguished by their `TreeId` – and
each tree operates in one of two modes:

 - **Log** mode: an append-only collection of items; this has two sub-modes:
   - normal Log mode, where the Trillian service assigns sequence numbers to
     new tree entries as they arrive
   - 'preordered' Log mode, where the unique sequence number for entries in
     the Merkle tree is externally specified

In either case, Trillian's key transparency property is that cryptographic
proofs of inclusion/consistency are available for data items added to the
service.


### Personalities

To build a complete transparent application, the Trillian core service needs
to be paired with additional code, known as a *personality*, that provides
functionality that is specific to the particular application.

In particular, the personality is responsible for:

 * **Admission Criteria** – ensuring that submissions comply with the
   overall purpose of the application.
 * **Canonicalization** – ensuring that equivalent versions of the same
   data get the same canonical identifier, so they can be de-duplicated by
   the Trillian core service.
 * **External Interface** – providing an API for external users,
   including any practical constraints (ACLs, load-balancing, DoS protection,
   etc.)

This is
[described in more detail in a separate document](docs/Personalities.md).
General
[design considerations for transparent Log applications](docs/TransparentLogging.md)
are also discussed separately.

### Log Mode

When running in Log mode, Trillian provides a gRPC API whose operations are
similar to those available for Certificate Transparency logs
(cf. [RFC 6962](https://tools.ietf.org/html/6962)). These include:

 - `GetLatestSignedLogRoot` returns information about the current root of the
   Merkle tree for the log, including the tree size, hash value, timestamp and
   signature.
 - `GetLeavesByRange` returns leaf information for particular leaves,
   specified by their index in the log.
 - `QueueLeaf` requests inclusion of the specified item into the log.
     - For a pre-ordered log, `AddSequencedLeaves` requests the inclusion of
       specified items into the log at specified places in the tree.
 - `GetInclusionProof`, `GetInclusionProofByHash` and `GetConsistencyProof`
    return inclusion and consistency proof data.

In Log mode (whether normal or pre-ordered), Trillian includes an additional
Signer component; this component periodically processes pending items and
adds them to the Merkle tree, creating a new signed tree head as a result.

![Log components](docs/images/LogDesign.png)

(Note that each of the components in this diagram can be
[distributed](https://github.com/google/certificate-transparency-go/blob/master/trillian/docs/ManualDeployment.md#distribution),
for scalability and resilience.)


Use Cases
---------

### Certificate Transparency Log

The most obvious application for Trillian in Log mode is to provide a
Certificate Transparency (RFC 6962) Log.  To do this, the CT Log personality
needs to include all of the certificate-specific processing – in particular,
checking that an item that has been suggested for inclusion is indeed a valid
certificate that chains to an accepted root.

