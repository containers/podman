#!/usr/bin/env bash

# This script contains helper functions for ensuring that dependencies
# exist on a host system that are required to run Origin scripts.

# os::util::ensure::system_binary_exists ensures that the
# given binary exists on the system in the $PATH.
#
# Globals:
#  None
# Arguments:
#  - 1: binary to search for
# Returns:
#  None
function os::util::ensure::system_binary_exists() {
	local binary="$1"

if ! os::util::find::system_binary "${binary}" >/dev/null 2>&1; then
		os::log::fatal "Required \`${binary}\` binary was not found in \$PATH."
	fi
}
readonly -f os::util::ensure::system_binary_exists