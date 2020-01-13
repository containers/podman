#!/bin/bash

source $(dirname $0)/lib.sh

# N/B: This script is expected to wrap $ENTRYPOINT when executing under the
#      'meta' Cirrus task on the libpod repo.
ENTRYPOINT=/usr/local/bin/entrypoint.sh

req_env_var IMGNAMES BUILDID REPOREF GCPJSON GCPNAME GCPPROJECT CIRRUS_CI

[[ -x "$ENTRYPOINT" ]] || \
    die 2 "Expecting to find an installed entrypoint script $ENTRYPOINT."

# A better way of checking isn't compatible with old but functional images
# in-use by other repos.
grep -q 'compute images update' "$ENTRYPOINT" || \
    die 3 "Expecting to be running inside a specific imgts container image"

canonicalize_image_names

# Executing inside a container; proper hand-off for process control
exec $ENTRYPOINT
