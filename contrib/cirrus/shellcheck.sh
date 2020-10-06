#!/bin/bash

set -eo pipefail

# shellcheck source=./contrib/cirrus/lib.sh
source $(dirname $0)/lib.sh

cd $CIRRUS_WORKING_DIR
shellcheck --color=always --format=tty \
    --shell=bash --external-sources \
    --enable add-default-case,avoid-nullary-conditions,check-unassigned-uppercase \
    --exclude SC2046,SC2034,SC2090,SC2064 \
    --wiki-link-count=0 --severity=warning \
    $SCRIPT_BASE/*.sh hack/get_ci_vm.sh

echo "Shellcheck: PASS"
