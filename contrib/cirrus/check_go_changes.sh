#!/bin/bash

set -eo pipefail

# This script is intended to confirm new go code conforms to certain
# conventions and/or does not introduce use of old/deprecated packages
# or functions.  It needs to run in the Cirrus CI environment, on behalf
# of PRs, via runner.sh.  This ensures a consistent and predictable
# environment not easily reproduced by a `Makefile`.

# shellcheck source=contrib/cirrus/lib.sh
source $(dirname $0)/lib.sh

check_msg() {
    msg "#####"  # Cirrus-CI logs automatically squash empty lines
    msg "##### $1"  # Complains if $1 is empty
}

# First arg is check description, second is regex to search $diffs for.
check_diffs() {
    local check regex
    check="$1"
    regex="$2"
    check_msg "Confirming changes have no $check"
    req_env_vars check regex diffs
    if egrep -q "$regex"<<<"$diffs"; then
        # Show 5 context lines before/after as compromise for script simplicity
        die "Found $check:
$(egrep -B 5 -A 5 "$regex"<<<"$diffs")"
    fi
}

# Defined by Cirrus-CI
# shellcheck disable=SC2154
if [[ "$CIRRUS_BRANCH" =~ pull ]]; then
    for var in CIRRUS_CHANGE_IN_REPO CIRRUS_PR DEST_BRANCH; do
        if [[ -z "${!var}" ]]; then
            warn "Skipping: Golang code checks require non-empty '\$$var'"
            exit 0
        fi
    done
else
    warn "Skipping: Golang code checks in tag and branch contexts"
    exit 0
fi

# Defined by/in Cirrus-CI config.
# shellcheck disable=SC2154
base=$(git merge-base $DEST_BRANCH $CIRRUS_CHANGE_IN_REPO)
diffs=$(git diff $base $CIRRUS_CHANGE_IN_REPO -- '*.go' ':^vendor/' ':^test/tools/vendor/')

if [[ -z "$diffs" ]]; then
    check_msg "There are no golang diffs to check between $base...$CIRRUS_CHANGE_IN_REPO"
    exit 0
fi

check_diffs \
    "use of deprecated ioutil vs recommended io or os packages." \
    "^(\\+[^#]+io/ioutil)|(\\+.+ioutil\\..+)"

check_diffs \
    "use of os.IsNotExists(err) vs recommended errors.Is(err, os.ErrNotExist)" \
    "^\\+[^#]*os\\.IsNotExists\\("
