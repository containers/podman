#!/usr/bin/env bash

# This script will update the goimports in the rpm spec for downstream fedora
# packaging, via the `propose-downstream` packit action.
# The goimports don't need to be present upstream.

set -e

SPEC_FILE=$(pwd)/podman.spec

sed -i '/Provides: bundled(golang.*/d' $SPEC_FILE

GO_IMPORTS=$(golist --imported --package-path github.com/containers/podman --skip-self | sort -u | xargs -I{} echo "Provides: bundled(golang({}))")

awk -v r="$GO_IMPORTS" '/^# vendored libraries/ {print; print r; next} 1' $SPEC_FILE > temp && mv temp $SPEC_FILE
