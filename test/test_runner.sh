#!/usr/bin/env bash
set -e

cd "$(dirname "$(readlink -f "$BASH_SOURCE")")"

# Load the helpers.
. helpers.bash

function execute() {
	>&2 echo "++ $@"
	eval "$@"
}

# Tests to run. Defaults to all.
if [[ -z "${TESTS}" ]]; then
	TESTS=${@:-.}
else
	TESTS=$TESTS
fi

# Run the tests.
execute time bats --tap $TESTS
