#!/bin/bash
#
# This script is what runs mac tests. It is invoked from .cirrus.yml
#
# Yep, some of this is adapted from runner.sh. We can't actually
# use that as a library, because Macintosh bash and awk lack
# features we need.

set -euo pipefail


# Name pattern for logformatter output file, derived from environment
function output_name() {
    # .cirrus.yml defines this as a short readable string for web UI
    std_name_fmt=$(sed -ne 's/^.*std_name_fmt \"\(.*\)\"/\1/p' <.cirrus.yml)
    test -n "$std_name_fmt" || die "Could not grep 'std_name_fmt' from .cirrus.yml"

    # Interpolate envariables. 'set -u' throws fatal if any are undefined
    (
        set -u
        eval echo "$std_name_fmt" | tr ' ' '-'
    )
}

function logformatter() {
    # Mac awk barfs on this, syntax error
#    awk --file "${CIRRUS_WORKING_DIR}/${SCRIPT_BASE}/timestamp.awk" \
    # shellcheck disable=SC2154
    "${CIRRUS_WORKING_DIR}/${SCRIPT_BASE}/logformatter" "$(output_name)"
}

# Defined by .cirrus.yml
# shellcheck disable=SC2154
case "$TEST_FLAVOR" in
    machine-applehv)
        export CONTAINERS_MACHINE_PROVIDER="applehv"
        ;;
    machine-libkrun)
        export CONTAINERS_MACHINE_PROVIDER="libkrun"
        ;;
    *)
        echo "Unknown/unsupported \$TEST_FLAVOR value '$TEST_FLAVOR'."
        exit 1
esac

make localmachine 2>&1 | logformatter
