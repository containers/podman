#!/bin/bash

# This script is intended to be called by Cirrus-CI on a Mac M1 persistent worker.
# It runs after the preparatory `mac_cleanup.sh` to performs all the user-level
# environment setup required to execute testing.  It assumes whatever system-wide
# setup is required, has already happened and was successful.

set -euo pipefail

# The otherwise standard `/etc/ci_environment` file cannot be used in this
# context, because the system is shared for multiple tasks.  Instead, persist
# env. vars required during /subsequent/ testing steps via a "magic" Cirrus-CI
# mechanism.  These cannot be set in the task YAML because they would interfere
# with repo. cloning and task preparation.
# Ref:
# https://cirrus-ci.org/guide/tips-and-tricks/#setting-environment-variables-from-scripts

# Tests expect to call compiled binaries first, make sure they're found first.
# shellcheck disable=SC2154
echo "PATH=$CIRRUS_WORKING_DIR/bin/darwin:$PATH" >> $CIRRUS_ENV

# Post-task cleanup needs to know the actual user home directory
# shellcheck disable=SC2154
echo "ORIGINAL_HOME=$HOME" >> $CIRRUS_ENV

# Help isolate CI-operations from system-operations and simplify task cleanup.
# shellcheck disable=SC2154
echo "HOME=$HOME/ci" >> $CIRRUS_ENV
# shellcheck disable=SC2154
echo "TMPDIR=/private/tmp/ci" >> $CIRRUS_ENV

# Removed completely during cleanup.
mkdir -p /private/tmp/ci
