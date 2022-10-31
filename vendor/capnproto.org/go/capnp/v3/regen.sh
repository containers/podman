#!/bin/bash
# regen.sh - update capnpc-go and regenerate schemas
set -euo pipefail

cd "$(dirname "$0")"

echo "** go generate"
go generate

echo "** capnpc-go"
# Run tests so that we don't install a broken capnpc-go.
(cd capnpc-go && go generate && go test && go install)

echo "** schemas"
(cd std/capnp; ../gen.sh compile)
(cd std/capnp; capnp compile --no-standard-import -I.. -o- schema.capnp) | (cd internal/schema && capnpc-go -promises=0 -schemas=0 -structstrings=0)
go generate ./...
