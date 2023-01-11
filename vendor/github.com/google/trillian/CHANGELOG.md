# TRILLIAN Changelog

## HEAD

## v.1.5.0

### Storage

* Ephemeral nodes are no-longer written for any tree by default (and have not been read since the v1.4.0 release), the corresponding `--tree_ids_with_no_ephemeral_nodes` flag is now deprecated (and will be removed in a future release).
  
### Cleanup
* Format code according to go1.19rc2 by @mhutchinson in https://github.com/google/trillian/pull/2785
* Delete merkle package, use [github.com/transparency-dev/merkle](https://pkg.go.dev/github.com/transparency-dev/merkle) instead.

### Misc
* Fix order-dependent test by @hickford in https://github.com/google/trillian/pull/2792

### Dependency updates
* Updated golangci-lint to v1.47.3 (developers should update to this version) by @mhutchinson in https://github.com/google/trillian/pull/2791
* Bump google.golang.org/api from 0.87.0 to 0.88.0 by @dependabot in https://github.com/google/trillian/pull/2783
* Bump cloud.google.com/go/spanner from 1.35.0 to 1.36.0 by @dependabot in https://github.com/google/trillian/pull/2784
* Bump google.golang.org/api from 0.88.0 to 0.90.0 by @dependabot in https://github.com/google/trillian/pull/2789
* Bump golang.org/x/tools from 0.1.11 to 0.1.12 by @dependabot in https://github.com/google/trillian/pull/2790
* Bump google.golang.org/protobuf from 1.28.0 to 1.28.1 by @dependabot in https://github.com/google/trillian/pull/2788
* Bump google.golang.org/api from 0.90.0 to 0.91.0 by @dependabot in https://github.com/google/trillian/pull/2796
* Bump github.com/prometheus/client_golang from 1.12.2 to 1.13.0 by @dependabot in https://github.com/google/trillian/pull/2795
* Bump github.com/fullstorydev/grpcurl from 1.8.6 to 1.8.7 by @dependabot in https://github.com/google/trillian/pull/2794
* Bump google.golang.org/api from 0.91.0 to 0.92.0 by @dependabot in https://github.com/google/trillian/pull/2798

## v1.4.2

* #2568: Allow disabling the writes of ephemeral nodes to storage via the
  `--tree_ids_with_no_ephemeral_nodes` flag to the sequencer.
* #2748: `--cloudspanner_max_burst_sessions` deprecated (it hasn't had any
  effect for a while, now it's more explicit)
* #2768: update go.mod to use 1.17 compatibility from 1.13.

### Dependency updates

* Updated golangci-lint to v1.46.1 (developers should update to this version)
* Removed dependency on certificate-transparency-go

### Developer updates

* #2765 copies the required protos from `googleapis` into `third_party` in this
  repository. This simplifies the preconditions in order to compile the proto
  definitions, and removes a big dependency on `$GOPATH/src` which was archaic;
  `$GOPATH/src/github.com/googleapis/googleapis` is no longer required.

## v1.4.1

* `countFromInformationSchema` function to add support for MySQL 8.

### Removals

 * #2710: Unused `storage/tools/dumplib` was removed. The useful storage format
  regression test moved to `integration/format`.
 * #2711: Unused `storage/tools/hasher` removed.
 * #2715: Packages under `merkle` are deprecated and to be removed. Use
   https://github.com/transparency-dev/merkle instead.

### Misc improvements

 * #2712: Fix MySQL world-writable config warning.
 * #2726: Check the tile height invariant stricter. No changes required.

### Dependency updates
 * #2731: Update `protoc` from `v3.12.4` to `v3.20.1`

## v1.4.0

* Recommended go version for development: 1.17
  * This is the version used by the cloudbuild presubmits. Using a
    different version can lead to presubmits failing due to unexpected
    diffs.
* GCP terraform script updated. GKE 1.19 and updated CPU type to E2

### Dependency updates
Many dep updates, including:
 * Upgraded to etcd v3 in order to allow grpc to be upgraded (#2195)
   * etcd was `v0.5.0-alpha.5`, now `v3.5.0`
 * grpc upgraded from `v1.29.1` to `v1.40.0`
 * certificate-transparency-go from `v1.0.21` to
   `v1.1.2-0.20210512142713-bed466244fa6`
 * protobuf upgraded from `v1` to `v2`
 * MySQL driver from `1.5.0` to `1.6.0`

### Cleanup
 * **Removed signatures from LogRoot and EntryTimestamps returned by RPCs** (reflecting that
   there should not be a trust boundary between Trillian and the personality.)
 * Removed the deprecated crypto.NewSHA256Signer function.
 * Finish removing the `LogMetadata.GetUnsequencedCounts()` method.
 * Removed the following APIs:
   - `TrillianLog.GetLeavesByHash`
   - `TrillianLog.GetLeavesByIndex`
   - `TrillianLog.QueueLeaves`
 * Removed the incomplete Postgres storage backend (#1298).
 * Deprecated `LogRootV1.Revision` field.
 * Moved `rfc6962` hasher one directory up to eliminate empty leftover package.
 * Removed unused `log_client` tool.
 * Various tidyups and improvements to merke & proof generation code.
 * Remove some remnants of experimental map.

### Storage refactoring
 * `NodeReader.GetMerkleNodes` does not accept revisions anymore. The
   implementations must use the transaction's `ReadRevision` instead.
 * `TreeStorage` migrated to using `compact.NodeID` type suitable for logs.
 * Removed the tree storage `ReadRevision` and `WriteRevision` methods.
   Revisions are now an implementation detail of the current storages. The
   change allows log implementations which don't need revisions.
 * Removed `Rollback` methods from storage interfaces, as `Close` is enough to
   cover the use-case.
 * Removed the unused `IsOpen` and `IsClosed` methods from transaction
   interfaces.
 * Removed the `ReadOnlyLogTX` interface, and put its only used
   `GetActiveLogIDs` method to `LogStorage`.
 * Inlined the `LogMetadata` interface to `ReadOnlyLogStorage`.
 * Inlined the `TreeStorage` interfaces to `LogStorage`.
 * Removed the need for the storage layer to return ephemeral node hashes. The
   application layer always requests for complete subtree nodes comprising the
   compact ranges corresponding to the requests.
 * Removed the single-tile callback from `SubtreeCache`, it uses only
   `GetSubtreesFunc` now.
 * Removed `SetSubtreesFunc` callback from `SubtreeCache`. The tiles should be
   written by the caller now, i.e. the caller must invoke the callback.

## v1.3.13
[Published 2021-02-16](https://github.com/google/trillian/releases/tag/v1.3.13)

### Cleanup
 * Removed the experimental map API.

## v1.3.12
[Published 2021-02-16](https://github.com/google/trillian/releases/tag/v1.3.12)

### Misc improvements

 * Removed unused `PeekTokens` method from the `quota.Manager` interface.
 * Ensure goroutines never block in the subtree cache (#2272).
 * Breaking unnecessary dependencies for Trillian clients:
   * Moved verifiers from `merkle` into `merkle/{log,map}verifier`sub-pacakges,
     reducing the amount of extra baggage inadvertently pulled in by clients.
  * Concrete hashers have been moved into subpackages, separating them from their
    registration code, allowing clients to directly pull just the hasher they're
    interested in and avoid the Trillian/hasher registry+protobuf deps.
 * Moved some packages intended for internal-only use into `internal` packages:
   * InMemoryMerkleTree (indended to only be used by Trillian tests)
 * Removed wrapper for etcd client (#2288).
 * Moved `--quota_system` and `--storage_system` flags to `main.go` so that they
   are initialised properly. It might break depending builds relying on these
   flags. Suggested fix: add the flags to `main.go`.
 * Made signer tolerate mastership election failures [#1150].
 * `testdb` no longer accepts the `--test_mysql_uri` flag, and instead honours the
   `TEST_MYSQL_URI` ENV var. This makes it easier to blanket configure tests to use a
   specific test DB instance.
 * Removed experimental Skylog folder (#2297).
 * Fixed a race condition in the operation manager that should only affect tests
   (#2302).
 * Run gofumpt formatter on the whole repository (#2315).
 * Refactor signer operation loop (#2294).

### Upgrades
 * Dockerfiles are now based on Go 1.13 image.
 * The etcd is now pinned to v3.4.12.
 * The golangci-lint suite is now at v1.36.0.
 * CI/CD has migrated from Travis to Google Cloud Build.
 * prometheus from 1.7.1 to 1.9.0 (#2239, #2270).
 * go-cmp from 0.5.2 to 0.5.4 (#2262).
 * apache/beam from 2.26.0+incompatible to 2.27.0+incompatible (#2273).
 * lib/pq from 1.8.0 to 1.9.0 (#2264).
 * go-redis from 6.15.8+incompatible to 6.15.9+incompatible (#2215).


### Process
 * Recognise that we do not follow strict semantic versioning practices.

## v1.3.11
[Published 2020-10-06](https://github.com/google/trillian/releases/tag/v1.3.11)

### Documentation

Added docs which describe the Claimant Model of transparency, a useful
framework for reasoning about the design and architecture of transparent
systems.

### Misc improvements

 * Fixed int to string conversion warnings for golang 1.15
 * Metric improvements for fetched leaf counts
 * Move tools.go into its own directory to help with dependencies

### Dependency updates
 * go-grpc-middleware from 1.2.0 to 1.2.2 (#2219, #2229)
 * stackdriver from 0.13.2 to 0.13.4 (#2220, #2223)
 * Google api from 0.28.0 to 0.29.0 (#2193)


## v1.3.10
[Published 2020-07-02](https://github.com/google/trillian/releases/tag/v1.3.10)

### Storage

The StorageProvider type and helpers have been moved from the server package to
storage. Aliases for the old types/functions are created for backward
compatibility, but the new code should not use them as we will remove them with
the next major version bump. The individual storage providers have been moved to
the corresponding packages, and are now required to be imported explicitly by
the main file in order to be registered. We are including only MySQL and
cloudspanner providers by default, since these are the ones that we support.

The cloudspanner storage is supported for logs only, while the Map storage API
is being polished and decoupled from the log storage API. We may return the
support when the new API is tested.

Support for storage of Ed25519 signatures has been added to the mysql and
postgres storage drivers (only applicable in new installations) and bugs
preventing correct usage of that algorithm have been fixed.

#### Storage TX Interfaces
- `QueueLeaves` has been removed from the `LogTreeTX` interface because
  `QueueLeaves` is not transactional.  All callers use the
  `QueueLeaves` function in the `LogStorage` interface.
- `AddSequencedLeaves` has been removed from the `LogTreeTX`.


### Log Changes

#### Monitoring & Metrics

The `queued_leaves` metric is removed, and replaced by `added_leaves` which
covers both `QueueLeaves` and `AddSequencedLeaves`, and is labeled by log ID.

#### MySQL Dequeueing Change #2159
mysql will now remove leaves from the queue inside of `UpdateLeaves` rather
than directly inside of `Dequeue`.
This change brings the behavior of the mysql storage implementation into line
with the spanner implementation and makes consistent testing possible.


### Map Changes

**The verifiable map is still experimental.**
APIs, such as SetLeaves, have been deprecated and will be deleted in the near
future. The semantics of WriteLeaves have become stricter: now it always
requires the caller to specify the write revision. These changes will not
affect the Trillian module semantic version due to the experimental status of
the Map.

Map API has been extended with Layout, GetTiles and SetTiles calls which allow
for more direct processing of sparse Merkle tree tiles in the application layer.
Map storage implementations are simpler, and no longer use the SubtreeCache.

The map client has been updated so that GetAndVerifyMapLeaves and
GetAndVerifyMapLeavesByRevision return the MapRoot for the revision at which the
leaves were fetched. Without this callers of GetAndVerifyMapLeaves in particular
were unable to reason about which map revision they were seeing. The
SetAndVerifyMapLeaves method was deleted.



## v1.3.9
[Published 2020-06-22](https://github.com/google/trillian/releases/tag/v1.3.9)

### Selected Dependency Updates
* etcd from v3.3.18 to 3.4.7 (#2090)
* etcd-operator from v0.9.1 to v0.9.4
* upgraded protoc version to latest (#2088)
* github.com/golang/protobuf to v1.4.1 (#2111)
* google.golang.org/grpc from v1.26 to 1.29.1 (#2108)


## v1.3.8
[Published 2020-05-12](https://github.com/google/trillian/releases/tag/v1.3.8)

### HTTP APIs

The HTTP/JSON APIs have been removed in favor of a pure gRPC intereface.
[grpcurl](https://github.com/fullstorydev/grpcurl) is the recommended way
of interacting with the gRPC API from the commandline.


## v1.3.7
[Published 2020-05-12](https://github.com/google/trillian/releases/tag/v1.3.7)

### Server Binaries

The `trillian_log_server`, `trillian_log_signer` and `trillian_map_server`
binaries have moved from `github.com/google/trillian/server/` to
`github.com/google/trillian/cmd`. A subset of the `server` package has also
moved and has been split into `cmd/internal/serverutil`, `quota/etcd` and
`quota/mysqlqm` packages.


## v1.3.6
[Published 2020-05-12](https://github.com/google/trillian/releases/tag/v1.3.6)

### Deployments

The Kubernetes configs will now provision 5 nodes for Trillian's Etcd cluster,
instead of 3 nodes.
[This makes the Etcd cluster more resilient](https://etcd.io/docs/v3.2.17/faq/#what-is-failure-tolerance)
to nodes becoming temporarily unavailable, such as during updates (it can now
tolerate 2 nodes being unavailable, instead of just 1).

### Monitoring & Metrics

A count of the total number of individual leaves the logserver attempts to
fetch via the GetEntries.* API methods has been added.


## v1.3.5
[Published 2020-05-12](https://github.com/google/trillian/releases/tag/v1.3.5)

### Log Changes

#### Potential sequencer hang fixed
A potential deadlock condition in the log sequencer when the process is
attempting to exit has been addressed.

### Quota

#### New Features

An experimental Redis-based `quota.Manager` implementation has been added.

#### Behaviour Changes

Quota used to be refunded for all failed requests. For uses of quota that were
to protect against abuse or fair utilization, this could allow infinite QPS in
situations that really should have the requests throttled. Refunds are now only
performed for tokens in `Global` buckets, which prevents tokens being leaked if
duplicate leaves are queued.

### Tools

The `licenses` tool has been moved from "scripts/licenses" to [a dedicated
repository](https://github.com/google/go-licenses).

### Bazel Changes

Python support is disabled unless we hear that the community cares about this
being re-enabled. This was broken by a downstream change and without a signal
from the Trillian community to say this is needed, the pragmatic action is to
not spend time investigating this issue.


## v1.3.4 - Invalid release, do not use.
[Published 2020-05-12](https://github.com/google/trillian/releases/tag/v1.3.4)


## v1.3.3 - Module fixes

Published 2019-10-31 17:30:00 +0000 UTC

Patch release to address Go Module issue. Removes `replace` directives in our
go.mod file now that our dependencies have fixed their invalid pseudo-version
issues.

## v1.3.2 - Module fixes

Published 2019-09-05 17:30:00 +0000 UTC

Patch release to address Go Module issue. Some dependencies use invalid pseudo-
versions in their go.mod files that Go 1.13 rejects. We've added `replace`
directives to our go.mod file to fix these invalid pseudo-versions.

## v1.3.1 - Module and Bazel fixes

Published 2019-08-16 15:00:00 +0000 UTC

Patch release primarily to address Go Module issue. v1.3.0 declared a dependency
on github.com/russross/blackfriday/v2 v2.0.1+incompatible which made downstream
dependencies suffer.

## v1.3.0

Published 2019-07-17 15:00:00 +0000 UTC

### Storage APIs GetSignedLogRoot / SetSignedLogRoot now take pointers

This at the storage layer and does not affect the log server API.
This is part of work to fix proto buffer usages where they are passed
by value or compared by generic code like `reflect.DeepEquals()`. Passing
them by value creates shallow copies that can share internal state. As the
generated structs contain additional exported `XXX_` fields generic
comparisons using all fields can produce incorrect results.

### Storage Commit takes context.Context

To support passing a context down to `NodeStorage.SetLeaves`, and remove various `context.TODO()`s,
the following functions have been modified to accept a `context.Context` parameter:

- `storage/cache.NodeStorage.SetLeaves`
- `storage/cache.SetSubtreesFunc`
- `storage/cache.SubtreeCache.Flush`
- `storage.ReadonlyLogTX.Commit`

### Go Module Support

Go Module support has been enabled. Please use GO111MODULE=on to build Trillian.
Updating dependencies no longer requires updating the vendor directory.

### TrillianMapWrite API
New API service for writing to the Trillian Map. This allows APIs such as
GetLeavesByRevisionNoProof to be removed from the read API, and these methods to
be tuned & provisioned differently for read vs write performance.

### GetLeavesByRevisionNoProof API
Allow map clients to forgo fetching inclusion proofs.
This dramatically speeds things up for clients that don't need verifiability.
This situation occurs in some situation where a Trillian personality is
interacting directly with the Trillian Map.

### GetMapLeafByRevision API
New GetMapLeafByRevision API for fetching a single map leaf. This allows there
to be a separate API end point for fetching a single leaf vs. the batch
GetMapLeavesByRevision API which is much slower when many leaves are requested.
This supports separate monitoring and alerting for different traffic patterns.

### Add Profiling Flags to Binaries

The `trillian_log_server`, `trillian_log_signer` and `trillian_map_server`
binaries now have CPU and heap profiling flags. Profiling is off by default.
For more details see the
[Go Blog](https://blog.golang.org/profiling-go-programs).
### Map performance tweaks

The map mode has had some performance tweaks added:
* A workaround for locking issues which affect the map when it's used in
  single-transaction mode.

### Introduce BatchInclusionProof function

Added a batch version of the Merkle Tree InclusionProof function.

Updated the map RPC for getLeaves to use the new batch function to improve
efficiency.

### Google Cloud Spanner support

Google Cloud Spanner is now a supported storage backend for maps.

The admin API calls to list trees backed by Cloud Spanner trees are fixed.

### RPC Server Transaction Leaks Fixed

There were some cases where the Log RPC server could leak storage transactions
in error situations. These have now been fixed. If you have a custom storage
implementation review the fixes made to the MySQL Log storage to see if they
need to be applied to your code (`storage/mysql/log_storage.go`). The Map
server had similar issues but these were fixed without requiring changes to
storage code.

### GetLatestSignedLogRoot With Consistency Proof

`GetLatestSignedLogRoot` in the LogServer will return a consistency proof if
`first_tree_size` > 0. This reduces the number of RPC calls from logClient from
2 to 1 in `client.getAndVerifyLatestRoot`.

### Testing

Support has been added for testing against a locally running mysql docker image,
in addition to a locally running mysql instance.

### Deprecated Fields Removed From SignedLogRoot Proto

*Important Note*: For use in Certificate Transparency this version of the
logserver binary won't work properly with an older CTFE. Make sure to update the
CTFE servers to a current version (built from a git checkout after March 20th
2019) before deploying logservers that include this change or deploy them
together with this release. Failure to do this can result in 5XX errors being
returned to clients when the old handler code tries to access fields in
responses that no longer exist.

All the fields marked as deprecated in this proto have been removed. All the
same fields are available via the TLS marshalled log root in the proto. Updating
affected code is straightforward.

Normally, clients will want to verify that the signed root is correctly signed.
This is the preferred way to interact with the root data.

There is a utility function provided that will verify the signature and unpack
the TLS data. It works well in conjunction with a `LogVerifier`. The public key
of the server is required.

```go
verifier := client.NewLogVerifier(rfc6962.DefaultHasher, pk, crypto.SHA256)
root, err := crypto.VerifySignedLogRoot(verifier.PubKey, verifier.SigHash, resp.SignedLogRoot)
if err != nil {
  // Signature verified and unmarshalled correctly. The struct may now
  // be used.
  if root.TreeSize > 0 {
    // Non empty tree.
  }
}
```

### MySQL changes

#### Configurable number of connections for MySQL

Two new flags have been added that limit connections to MySQL database servers:

-   `--mysql_max_conns` - limits the total number of database connections
-   `--mysql_max_idle_conns` - limits the number of idle database connections

By default, there is no maximum number of database connections. However, the
database server will likely impose limits on the number of connections. The
default limit on idle connections is controlled by
[Go's `sql` package](https://golang.org/pkg/database/sql/#DB.SetMaxIdleConns).

#### Enfored no concurrent use of MySQL tx

Concurrently using a single MySQL transaction can cause the driver to error
out, so we now attempt to prevent this from happening.

### Removal of length limits for a tree's `display_name` and `description`

Previously, these were restricted to 20 bytes and 200 bytes respectively. These
limits have been removed. However, the underlying storage implementation may
still impose its own limitations.

### Server validation of leaf hashes

The log server now checks that leaf hashes are the correct length and returns
an InvalidArgument error if they are not. Previously, GetLeavesByHash would
simply not return any matching leaves for invalid hashes, and
GetInclusionProofByHash would return a NotFound error.

### Map client

A [MapClient](client/map_client.go) has been added to simplify interacting with
the map server.

### Database Schema

This version includes a change to the MySQL and Postgres database schemas to add
an index on the `SequencedLeafData` table. This improves performance for
inclusion proof queries.

### Deployments

The Trillian Docker images now accept GOFLAGS and GO111MODULE arguments
and set them as environment variables inside the Docker container.

The [db\_server Docker image](examples/deployment/docker/db_server/Dockerfile)
is now based on
[the MySQL 5.7 image from the Google Cloud Marketplace](https://console.cloud.google.com/marketplace/details/google/mysql5),
rather than the [official MySQL 5.7 image](https://hub.docker.com/_/mysql).
This Dockerfile supercedes Dockerfile.db, which has been removed.

There is now a [mysql.cnf file](examples/deployment/docker/db_server/mysql.cnf)
alongside the Dockerfile that makes it easy to build the image with a custom
configuration, e.g. to allow MySQL to use more memory.

The `trillian-log-service` and `trillian-log-signer` Kubernetes services will
now have load balancers configured for them that expose those services outside
of the Kubernetes cluster. This makes it easier to access their APIs. When
deployed on Google Cloud, these will be
[Internal Load Balancers](https://cloud.google.com/kubernetes-engine/docs/how-to/internal-load-balancing).
Note that this change **cannot be applied to an existing deployment**; delete
the existing Kubernetes services and redeploy them, otherwise you'll see an
error similar to `The Service "trillian-log-service" is invalid: spec.clusterIP:
Invalid value: "": field is immutable`.

A working [Docker Compose](https://docs.docker.com/compose/) configuration is
now available and can be used to bring up a local Trillian deployment for
testing and experimental purposes:

```shell
docker-compose -f examples/deployment/docker-compose.yml up
```

Docker Compose v3.1 or higher is required.

The Terraform, Kubernetes and Docker configuration files, as well as various
scripts, all now use the same, consistently-named environment variables for
MySQL-related data (e.g. `MYSQL_DATABASE`). The variable names are based on
those for the
[MySQL Docker image](https://hub.docker.com/_/mysql#environment-variables).

Docker images have been upgraded from Go 1.9 to 1.11. They now use ["Distroless"
base images](https://github.com/GoogleContainerTools/distroless).

### Dropped metrics

Quota metrics with specs of the form `users/<user>/read` and
`users/<user>/write` are no longer exported by the Trillian binaries (as they
lead to excessive storage requirements for Trillian metrics).

### Resilience improvements in `log_signer`

#### Add timeout to sequencing loop

Added a timeout to the context in the sequencing loop, with a default of 60s.

#### Fix Operation Loop Hang

Resolved a bug that would hide errors and cause the `OperationLoop` to hang
until process exit if any error occurred.

### Linting toolchain migration

gometalinter has been replaced with golangci-lint for improved performance and
Go module support.

### Compact Merkle tree data structures

`CompactMerkleTree` has been removed from `github.com/google/trillian/merkle`,
and a new package `github.com/google/trillian/merkle/compact` was introduced.  A
new powerful data structure named "compact range" has been added to that
package, and is now used throughout the repository instead of the compact tree.
It is a generalization of the previous structure, as it allows manipulating
arbitrary sub-ranges of leaves rather than only prefixes.

### Storage API changes

The internal storage API is modified so that the ReadOnlyTreeTX.ReadRevision and
TreeWriter.WriteRevision entrypoints take a context.Context parameter and return
an optional error.

The `SubtreeCache.GetNodeHash()` method is no longer exported.

The memory storage provider has been refactored to make it more consistent with
the other storage providers.

The `LogMetadata.GetUnsequencedCounts()` method has been removed.

`NodeReader.GetMerkleNodes` now must return `Node` objects in the same order as
node IDs requested. Storage implementations known to us already adhere to this
requirement.

### Maphammer improvements

The maphammer test tool for the experimental Trillian Map has been enhanced.

### Default values changed for some signer flags

The following flags for the signer have new default values:

-   `--sequencer_interval`: changed from 10 seconds to 100 milliseconds
-   `--batch_size`: changed from 50 to 1000

These changes improve the signer's throughput and latency under typical
conditions.

### Master election refactoring

The `--resign_odds` flag in `logsigner` is removed, in favor of a more generic
`--master_hold_jitter` flag. Operators using this flag are advised to set the
jitter to `master_check_interval * resign_odds * 2` to achieve similar behavior.

The `--master_check_interval` flag is removed from `logsigner`.

`logsigner` switched to using a new master election interface contained in
`util/election2` package. The interfaces in `util/election` are removed.

### `CONIKS_SHA256` hash strategy added

Support has been added for a CONIKS sparse tree hasher with SHA256 as the hash
algorithm. Set a tree's `hash_strategy` to `CONIKS_SHA256` to use it.

### Performance

The performance of `SetLeaves` requests on the Map has been slightly improved.
The performance of `GetConsistencyProof` requests has been improved when using
MySQL.

### Logging

Some warning-level logging has been removed from the sequencer in favour of
returning the same information via the returned error. The caller may still
choose to log this information. This allows storage implementations that retry
transactions to suppress warnings when a transaction initially fails but a retry
succeeds.

Some incorrectly-formatted log messages have been fixed.

### Documentation

[API documentation in Markdown format](docs/api.md) is now available.

### Other

The `TimeSource` type (and other time utils) moved to a separate `util/clock`
package, extended with a new `Timer` interface that allows mocking `time.Timer`.

The `Sequencer.SignRoot()` method has been removed.

## v1.2.1 - Map race fixed. TLS client support. LogClient improvements

Published 2018-08-20 10:31:00 +0000 UTC

### Servers

A race condition was fixed that affected sparse Merkle trees as served by the
map server.

### Utilities / Binaries

The `maphammer` uses a consistent empty check, fixing spurious failures in some
tests.

The `createtree` etc. set of utilities now support TLS via the `-tls-cert-file`
flag. This support is also available as a client module.

### Log Client

`GetAndVerifyInclusionAtIndex` no longer updates the clients root on every
access as this was an unexpected side effect. Clients now have explicit control
of when the root is updated by calling `UpdateRoot`.

A root parameter is now required when log clients are constructed.

The client will now only retry requests that fail with the following errors:

-   Aborted
-   DeadlineExceeded
-   ResourceExhausted
-   Unavailable

There is one exception - it will also retry InitLog/InitMap requests that fail
due to a FailedPrecondition error.

### Other

The Travis build script has been updated for newer versions of MySQL (5.7
through MySQL 8) and will no longer work with 5.6.

Commit
[f3eaa887163bb4d2ea4b4458cb4e7c5c2f346bc6](https://api.github.com/repos/google/trillian/commits/f3eaa887163bb4d2ea4b4458cb4e7c5c2f346bc6)
Download [zip](https://api.github.com/repos/google/trillian/zipball/v1.2.1)

## v1.2.0 - Signer / Quota fixes. Error mapping fix. K8 improvements

Published 2018-06-25 10:42:52 +0000 UTC

The Log Signer now tries to avoid creating roots older than ones that already
exist. This issue has been seen occurring on a test system. Important note: If
running this code in production allowing clocks to drift out of sync between
nodes can cause other problems including for clustering and database
replication.

The Log Signer now publishes metrics for the logs that it is actively signing.
In a clustered environment responsibility can be expected to move around between
signer instances over time.

The Log API now allows personalities to explicitly list a vector of identifiers
which should be charged for `User` quota. This allows a more nuanced application
of request rate limiting across multiple dimensions. Some fixes have also been
made to quota handling e.g. batch requests were not reserving the appropriate
quota. Consult the corresponding PRs for more details.

For the log RPC server APIs `GetLeavesByIndex` and `GetLeavesByRange` MySQL
storage has been modified to return status codes that match CloudSpanner.
Previously some requests with out of range parameters were receiving 5xx error
status rather than 4xx when errors were mapped to the HTTP space by CTFE.

The Kubernetes deployment scripts continue to evolve and improve.

Commit
[aef10347dba1bd86a0fcb152b47989d0b51ba1fa](https://api.github.com/repos/google/trillian/commits/aef10347dba1bd86a0fcb152b47989d0b51ba1fa)
Download [zip](https://api.github.com/repos/google/trillian/zipball/v1.2.0)

## v1.1.1 - CloudSpanner / Tracing / Health Checks

Published 2018-05-08 12:55:34 +0000 UTC

More improvements have been made to the CloudSpanner storage code. CloudSpanner
storage has now been tested up to ~3.1 billion log entries.

Explicit health checks have been added to the gRPC Log and Map servers (and the
log signer). The HTTP endpoint must be enabled and the checks will serve on
`/healthz` where a non 200 response means the server is unhealthy. The example
Kubernetes deployment configuration has been updated to include them. Other
improvements have been made to the Kubernetes deployment scripts and docs.

The gRPC Log and Map servers have been instrumented for tracing with
[OpenCensus](https://opencensus.io/). For GCP it just requires the `--tracing`
flag to be added and results will be available in the GCP console under
StackDriver -> Trace.

Commit
[3a68a845f0febdd36937c15f1d97a3a0f9509440](https://api.github.com/repos/google/trillian/commits/3a68a845f0febdd36937c15f1d97a3a0f9509440)
Download [zip](https://api.github.com/repos/google/trillian/zipball/v1.1.1)

## v1.1.0 - CloudSpanner Improvements & Log Root structure changes etc.

Published 2018-04-17 08:02:50 +0000 UTC

Changes are in progress (e.g. see #1037) to rework the internal signed root
format used by the log RPC server to be more useful / interoperable. Currently
they are mostly internal API changes to the log and map servers. However, the
`signature` and `log_id` fields in SignedLogRoot have been deleted and users
must unpack the serialized structure to access these now. This change is not
backwards compatible.

Changes have been made to log server APIs and CT frontends for when a request
hits a server that has an earlier version of the tree than is needed to satisfy
the request. In these cases the log server used to return an error but now
returns an empty proof along with the current STH it has available. This allows
clients to detect these cases and handle them appropriately.

The CloudSpanner schema has changed. If you have a database instance you'll need
to recreate it with the new schema. Performance has been noticeably improved
since the previous release and we have tested it to approx one billion log
entries. Note: This code is still being developed and further changes are
possible.

Support for `sqlite` in unit tests has been removed because of ongoing issues
with flaky tests. These were caused by concurrent accesses to the same database,
which it doesn't support. The use of `sqlite` in production has never been
supported and it should not be used for this.

Commit
[9a5dc6223bab0e1061b66b49757c2418c47b9f29](https://api.github.com/repos/google/trillian/commits/9a5dc6223bab0e1061b66b49757c2418c47b9f29)
Download [zip](https://api.github.com/repos/google/trillian/zipball/v1.1.0)

## v1.0.8 - Docker Updates / Freezing Logs / CloudSpanner Options

Published 2018-03-08 13:42:11 +0000 UTC

The Docker image files have been updated and the database has been changed to
`MariaDB 10.1`.

A `ReadOnlyStaleness` option has been added to the experimental CloudSpanner
storage. This allows for tuning that might increase performance in some
scenarios by issuing read transactions with the `exact_staleness` option set
rather than `strong_read`. For more details see the
[CloudSpanner TransactionOptions](https://cloud.google.com/spanner/docs/reference/rest/v1/TransactionOptions)
documentation.

The `LogVerifier` interface has been removed from the log client, though the
functionality is still available. It is unlikely that there were implementations
by third-parties.

A new `TreeState DRAINING` has been added for trees with `TreeType LOG`. This is
to support logs being cleanly frozen. A log tree in this state will not accept
new entries via `QueueLeaves` but will continue to integrate any that were
previously queued. When the queue of pending entries has been emptied the tree
can be set to the `FROZEN` state safely. For MySQL storage this requires a
schema update to add `'DRAINING'` to the enum of valid states.

A command line utility `updatetree` has been added to allow tree states to be
changed. This is also to support cleanly freezing logs.

A 'howto' document has been added that explains how to freeze a log tree using
the features added in this release.

Commit
[0e6d950b872d19e42320f4714820f0fe793b9913](https://api.github.com/repos/google/trillian/commits/0e6d950b872d19e42320f4714820f0fe793b9913)
Download [zip](https://api.github.com/repos/google/trillian/zipball/v1.0.8)

## v1.0.7 - Storage API Changes, Schema Tweaks

Published 2018-03-01 11:16:32 +0000 UTC

Note: A large number of storage related API changes have been made in this
release. These will probably only affect developers writing their own storage
implementations.

A new tree type `ORDERED_LOG` has been added for upcoming mirror support. This
requires a schema change before it can be used. This change can be made when
convenient and can be deferred until the functionality is available and needed.
The definition of the `TreeType` column enum should be changed to `ENUM('LOG',
'MAP', 'PREORDERED_LOG') NOT NULL`

Some storage interfaces were removed in #977 as they only had one
implementation. We think this won't cause any impact on third parties and are
willing to reconsider this change if it does.

The gRPC Log and Map server APIs have new methods `InitLog` and `InitMap` which
prepare newly created trees for use. Attempting to use trees that have not been
initialized will return the `FAILED_PRECONDITION` error
`storage.ErrTreeNeedsInit`.

The gRPC Log server API has new methods `AddSequencedLeaf` and
`AddSequencedLeaves`. These are intended to support mirroring applications and
are not yet implemented.

Storage APIs have been added such as `ReadWriteTransaction` which allows the
underlying storage to manage the transaction and optionally retry until success
or timeout. This is a more natural fit for some types of storage API such as
[CloudSpanner](https://cloud.google.com/spanner/docs/transactions) and possibly
other environments with managed transactions.

The older `BeginXXX` methods were removed from the APIs. It should be fairly
easy to convert a custom storage implementation to the new API format as can be
seen from the changes made to the MySQL storage.

The `GetOpts` options are no longer used by storage. This fixed the strange
situation of storage code having to pass manufactured dummy instances to
`GetTree`, which was being called in all the layers involved in request
processing. Various internal APIs were modified to take a `*trillian.Tree`
instead of an `int64`.

A new storage implementation has been added for CloudSpanner. This is currently
experimental and does not yet support Map trees. We have also added Docker
examples for running Trillian in Google Cloud with CloudSpanner.

The maximum size of a `VARBINARY` column in MySQL is too small to properly
support Map storage. The type has been changed in the schema to `MEDIUMBLOB`.
This can be done in place with an `ALTER TABLE` command but this could very be
slow for large databases as it is a change to the physical row layout. Note:
There is no need to make this change to the database if you are only using it
for Log storage e.g. for Certificate Transparency servers.

The obsolete programs `queue_leaves` and `fetch_leaves` have been deleted.

Commit
[7d73671537ca2a4745dc94da3dc93d32d7ce91f1](https://api.github.com/repos/google/trillian/commits/7d73671537ca2a4745dc94da3dc93d32d7ce91f1)
Download [zip](https://api.github.com/repos/google/trillian/zipball/v1.0.7)

## v1.0.6 - GetLeavesByRange. 403 Permission Errors. Signer Metrics.

Published 2018-02-05 16:00:26 +0000 UTC

A new log server RPC API has been added to get leaves in a range. This is a more
natural fit for CT type applications as it more closely follows the CT HTTP API.

The server now returns 403 for permission denied where it used to return 500
errors. This follows the behaviour of the C++ implementation.

The log signer binary now reports metrics for the number it has signed and the
number of errors that have occurred. This is intended to give more insight into
the state of the queue and integration processing.

Commit
[b20b3109af7b68227c83c5d930271eaa4f0be771](https://api.github.com/repos/google/trillian/commits/b20b3109af7b68227c83c5d930271eaa4f0be771)
Download [zip](https://api.github.com/repos/google/trillian/zipball/v1.0.6)

## v1.0.5 - TLS, Merge Delay Metrics, Easier Admin Tests

Published 2018-02-07 09:41:08 +0000 UTC

The API protos have been rebuilt with gRPC 1.3.

Timestamps have been added to the log leaves in the MySQL database. Before
upgrading to this version you **must** make the following schema changes:

*   Add the following column to the `LeafData` table. If you have existing data
    in the queue you might have to remove the NOT NULL clause:
    `QueueTimestampNanos BIGINT NOT NULL`

*   Add the following column to the `SequencedLeafData` table:
    `IntegrateTimestampNanos BIGINT NOT NULL`

The above timestamps are used to export metrics via monitoring that give the
merge delay for each tree that is in use. This is a good metric to use for
alerting on.

The Log and Map RPC servers now support TLS.

AdminServer tests have been improved.

Commit
[dec673baf984c3d22d7b314011d809258ec36821](https://api.github.com/repos/google/trillian/commits/dec673baf984c3d22d7b314011d809258ec36821)
Download [zip](https://api.github.com/repos/google/trillian/zipball/v1.0.5)

## v1.0.4 - Fix election issue. Large vendor updates.

Published 2018-02-05 15:42:25 +0000 UTC

An issue has been fixed where the master for a log could resign from the
election while it was in the process of integrating a batch of leaves. We do not
believe this could cause any issues with data integrity because of the versioned
tree storage.

This release includes a large number of vendor commits merged to catch up with
etcd 3.2.10 and gRPC v1.3.

Commit
[1713865ecca0dc8f7b4a8ed830a48ae250fd943b](https://api.github.com/repos/google/trillian/commits/1713865ecca0dc8f7b4a8ed830a48ae250fd943b)
Download [zip](https://api.github.com/repos/google/trillian/zipball/v1.0.4)

## v1.0.3 - Auth API. Interceptor fixes. Request validation + More

Published 2018-02-05 15:33:08 +0000 UTC

An authorization API has been added to the interceptors. This is intended for
future development and integration.

Issues where the interceptor would not time out on `PutTokens` have been fixed.
This should make the quota system more robust.

A bug has been fixed where the interceptor did not pass the context deadline
through to other requests it made. This would cause some failing requests to do
so after longer than the deadline with a misleading reason in the log. It did
not cause request failures if they would otherwise succeed.

Metalinter has been added and the code has been cleaned up where appropriate.

Docker and Kubernetes scripts have been available and images are now built with
Go 1.9.

Sqlite has been introduced for unit tests where possible. Note that it is not
multi threaded and cannot support all our testing scenarios. We still require
MySQL for integration tests. Please note that Sqlite **must not** be used for
production deployments as RPC servers are multi threaded database clients.

The Log RPC server now applies tighter validation to request parameters than
before. It's possible that some requests will be rejected. This should not
affect valid requests.

The admin server will only create trees for the log type it is hosted in. For
example the admin server running in the Log server will not create Map trees.
This may be reviewed in future as applications can legitimately use both tree
types.

Commit
[9d08b330ab4270a8e984072076c0b3e84eb4601b](https://api.github.com/repos/google/trillian/commits/9d08b330ab4270a8e984072076c0b3e84eb4601b)
Download [zip](https://api.github.com/repos/google/trillian/zipball/v1.0.3)

## v1.0.2 - TreeGC, Go 1.9, Update Private Keys.

Published 2018-02-05 15:18:40 +0000 UTC

Go 1.9 is required.

It is now possible to update private keys via the admin API and this was added
to the available field masks. The key storage format has not changed so we
believe this change is transparent.

Deleted trees are now garbage collected after an interval. This hard deletes
them and they cannot be recovered. Be aware of this before upgrading if you have
any that are in a soft deleted state.

The Admin RPC API has been extended to allow trees to be undeleted - up to the
point where they are hard deleted as set out above.

Commit
[442511ad82108654033c9daa4e72f8a79691dd32](https://api.github.com/repos/google/trillian/commits/442511ad82108654033c9daa4e72f8a79691dd32)
Download [zip](https://api.github.com/repos/google/trillian/zipball/v1.0.2)

## v1.0.1 - Batched Queue Option Added

Published 2018-02-05 14:49:33 +0000 UTC

Apart from fixes this release includes the option for a batched queue. This has
been reported to allow faster sequencing but is not enabled by default.

If you want to switch to this you must build the code with the `--tags
batched_queue` option. You must then also apply a schema change if you are
running with a previous version of the database. Add the following column to the
`Unsequenced` table:

`QueueID VARBINARY(32) DEFAULT NULL`

If you don't plan to switch to the `batched_queue` mode then you don't need to
make the above change.

Commit
[afd178f85c963f56ad2ae7d4721d139b1d6050b4](https://api.github.com/repos/google/trillian/commits/afd178f85c963f56ad2ae7d4721d139b1d6050b4)
Download [zip](https://api.github.com/repos/google/trillian/zipball/v1.0.1)

## v1.0 - First Log version we believe was ready for use. To support CT.

Published 2018-02-05 13:51:55 +0000 UTC

Quota metrics published. Quota admin api + server implemented. Improvements to
local / AWS deployment. Map fixes and further development. ECDSA key handling
improvements. Key factory improvements. Code coverage added. Quota integration
test added. Etcd quota support in log and map connected up. Incompatibility with
C++ code fixed where consistency proof requests for first == second == 0 were
rejected.

Commit
[a6546d092307f6e0d396068066033b434203824d](https://api.github.com/repos/google/trillian/commits/a6546d092307f6e0d396068066033b434203824d)
Download [zip](https://api.github.com/repos/google/trillian/zipball/v1.0)
