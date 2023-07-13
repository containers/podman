#!/usr/bin/env bash

# This script will update the Version field in the spec which is set to 0 by
# default. Useful for local manual rpm builds where the Version needs to be set
# correctly.

set -eox pipefail

PACKAGE=podman
SPEC_FILE=$PACKAGE.spec
GIT_DESCRIBE=$(git describe)
VERSION=$(echo $GIT_DESCRIBE | sed -e 's/^v//' -e 's/-/~/g')

# Update spec file to use local changes
sed -i "s/^Version:.*/Version: $VERSION/" $SPEC_FILE
sed -i "s/^Source0:.*/Source0: $PACKAGE-$GIT_DESCRIBE.tar.gz/" $SPEC_FILE
sed -i "s/^%autosetup.*/%autosetup -Sgit -n %{name}-$GIT_DESCRIBE/" $SPEC_FILE

# Generate Source0 archive from HEAD
(cd .. && git archive --format=tar.gz --prefix=$PACKAGE-$GIT_DESCRIBE/ HEAD -o rpm/$PACKAGE-$GIT_DESCRIBE.tar.gz)
