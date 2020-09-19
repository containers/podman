#!/usr/bin/env bash

set -e

die() { echo "${1:-No error message given} (from $(basename $0))"; exit 1; }

if [[ "$(type -t bats)" != "" ]]; then
	# bats is already installed.
	exit 0
fi

buildDir=$(mktemp -d)
git clone https://github.com/bats-core/bats-core $buildDir

pushd $buildDir
pwd
git reset --hard ${VERSION}
echo "Installing bats to /usr/local (requires root)"
sudo ./install.sh /usr/local
popd

rm -rf $buildDir
