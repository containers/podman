#!/usr/bin/env bash

# This script will update the Version field in the spec which is set to 0 by
# default. Useful for local manual rpm builds where the Version needs to be set
# correctly.

set -e

SPEC_FILE=$(pwd)/podman.spec
LATEST_TAG=$(git tag --sort=creatordate | tail -1)
LATEST_VERSION=$(echo $LATEST_TAG | sed -e 's/^v//')

sed -i "s/^Version:.*/Version: $LATEST_VERSION/" $SPEC_FILE
