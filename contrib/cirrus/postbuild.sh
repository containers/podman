#!/bin/bash

set -eo pipefail

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
showrun make vendor
SUGGESTION="run 'make vendor' and commit all changes" ./hack/tree_status.sh
showrun make generate-bindings
SUGGESTION="run 'make generate-bindings' and commit all changes" ./hack/tree_status.sh
showrun make completions
SUGGESTION="run 'make completions' and commit all changes" ./hack/tree_status.sh

# Defined in Cirrus-CI config.
# shellcheck disable=SC2154
$SCRIPT_BASE/check_go_changes.sh
