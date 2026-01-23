#!/usr/bin/env bash

set -exo pipefail

pushd "$TMT_TREE"
make
make install
popd
