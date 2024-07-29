#!/bin/bash

set -eo pipefail

# Skip this entire script when run under nightly treadmill cron job.
#
# Treadmill vendors in containers/<many>, which may bring in new versions
# of other dependencies such as cobra, which may change the completion
# files, which gives us a failure that we don't care one whit about.
# The purpose of the treadmill is to check for incompatibilities with
# other closely-tied container modules.
# shellcheck disable=SC2154
if [[ "$CIRRUS_CRON" = "treadmill" ]]; then
    echo "[in treadmill; skipping $(basename $0)]"
    exit 0
fi

# This script attempts to confirm all included go modules from
# other sources match what is expected in `vendor/modules.txt`
# vs `go.mod`.  Also make sure that the generated bindings in
# `pkg/bindings/...` are in sync with the code.  It's intended
# for use after successfully building podman, to prevent wasting
# time on tests that might otherwise succeed with bad/ugly/invalid
# code.

source /etc/automation_environment
source $AUTOMATION_LIB_PATH/common_lib.sh

# Defined by the CI system
# shellcheck disable=SC2154
cd $CIRRUS_WORKING_DIR

showrun make .install.goimports
# showrun make vendor
# SUGGESTION="run 'make vendor' and commit all changes" ./hack/tree_status.sh
showrun make generate-bindings
SUGGESTION="run 'make generate-bindings' and commit all changes" ./hack/tree_status.sh
showrun make completions
SUGGESTION="run 'make completions' and commit all changes" ./hack/tree_status.sh

# Defined in Cirrus-CI config.
# shellcheck disable=SC2154
$SCRIPT_BASE/check_go_changes.sh
