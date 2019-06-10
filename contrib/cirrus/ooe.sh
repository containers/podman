#!/bin/bash

# This script executes a command while logging all output to a temporary
# file.  If the command exits non-zero, then all output is sent to the console,
# before returning the exit code.  If the script itself fails, the exit code 121
# is returned.

set -eo pipefail

SCRIPT_BASEDIR="$(basename $0)"

badusage() {
    echo "Incorrect usage: $SCRIPT_BASEDIR) <command> [options]" > /dev/stderr
    echo "ERROR: $1"
    exit 121
}

COMMAND="$@"
[[ -n "$COMMAND" ]] || badusage "No command specified"

OUTPUT_TMPFILE="$(mktemp -p '' ${SCRIPT_BASEDIR}_output_XXXX)"
output_on_error() {
    RET=$?
    set +e
    if [[ "$RET" -ne "0" ]]
    then
        echo "---------------------------"
        cat "$OUTPUT_TMPFILE"
        echo "[$(date --iso-8601=second)] <exit $RET> $COMMAND"
    fi
    rm -f "$OUTPUT_TMPFILE"
}
trap "output_on_error" EXIT

"$@" 2>&1 | while IFS='' read LINE  # Preserve leading/trailing whitespace
do
    # Every stdout and (copied) stderr line
    echo "[$(date --iso-8601=second)] $LINE"
done >> "$OUTPUT_TMPFILE"
