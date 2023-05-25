#!/usr/bin/env bash

# This script handles any custom processing of the spec file generated using the `post-upstream-clone`
# action and gets used by the fix-spec-file action in .packit.yaml.

set -eo pipefail

# Set path to rpm spec file
SPEC_FILE=rpm/podman.spec

# Get Version from HEAD
VERSION=$(grep '^const RawVersion' version/rawversion/version.go | cut -d\" -f2)

# RPM Version can't take "-"
RPM_VERSION=$(echo $VERSION | sed -e 's/-/~/')

# Generate source tarball from HEAD
git archive --prefix=podman-$VERSION/ -o podman-$VERSION.tar.gz HEAD

# rpmbuild expects source tarball in the same dir as spec file
mv podman-$VERSION.tar.gz rpm/

# RPM Spec modifications

# Use the Version from HEAD in rpm spec
sed -i "s/^Version:.*/Version: $RPM_VERSION/" $SPEC_FILE

# Use Packit's supplied variable in the Release field in rpm spec.
# podman.spec is generated using `rpkg spec --outdir ./` as mentioned in the
# `post-upstream-clone` action in .packit.yaml.
sed -i "s/^Release:.*/Release: $PACKIT_RPMSPEC_RELEASE%{?dist}/" $SPEC_FILE

# Use above generated tarball as Source in rpm spec
sed -i "s/^Source0:.*.tar.gz/Source0: podman-$VERSION.tar.gz/" $SPEC_FILE

# Update setup macro to use the correct build dir
sed -i "s/^%autosetup.*/%autosetup -Sgit -n %{name}-$VERSION/" $SPEC_FILE
