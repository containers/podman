#!/usr/bin/env bash

# This script handles any custom processing of the spec file generated using the `post-upstream-clone`
# action and gets used by the fix-spec-file action in .packit.yaml.

set -eo pipefail

# Get Version from define/types.go in HEAD
VERSION=$(grep ^$'\tVersion' define/types.go | cut -d\" -f2 | sed -e 's/-/~/')

# Generate source tarball from HEAD
git archive --prefix=buildah-$VERSION/ -o buildah-$VERSION.tar.gz HEAD

# RPM Spec modifications

# Use the Version from define/types.go in rpm spec
sed -i "s/^Version:.*/Version: $VERSION/" buildah.spec

# Use Packit's supplied variable in the Release field in rpm spec.
# buildah.spec is generated using `rpkg spec --outdir ./` as mentioned in the
# `post-upstream-clone` action in .packit.yaml.
sed -i "s/^Release:.*/Release: $PACKIT_RPMSPEC_RELEASE%{?dist}/" buildah.spec

# Use above generated tarball as Source in rpm spec
sed -i "s/^Source:.*.tar.gz/Source: buildah-$VERSION.tar.gz/" buildah.spec

# Use the right build dir for autosetup stage in rpm spec
sed -i "s/^%setup.*/%autosetup -Sgit -n %{name}-$VERSION/" buildah.spec
