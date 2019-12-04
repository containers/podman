#!/bin/bash

# This script produces various bits of metadata needed by Makefile.  Using
# a script allows uniform behavior across multiple environments and
# distributions.  The script expects a single argument, as reflected below.

set -euo pipefail

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
cd "${GOSRC:-${DIR}/../}"

valid_args() {
    REGEX='^\s+[[:upper:]]+\*[)]'
    egrep --text --no-filename --group-separator=' ' --only-matching "$REGEX" "$0" | \
        cut -d '*' -f 1
}

unset OUTPUT
case "$1" in
    # Wild-card suffix needed by valid_args() e.g. possible bad grep of "$(echo $FOO)"
    VERSION*)
        OUTPUT="${CIRRUS_TAG:-$(git fetch --tags && git describe HEAD 2> /dev/null)}"
        ;;
    NUMBER*)
        OUTPUT="$($0 VERSION | sed 's/-.*//')"
        ;;
    DIST_VER*)
        OUTPUT="$(source /etc/os-release; echo $VERSION_ID | cut -d '.' -f 1)"
        ;;
    DIST*)
        OUTPUT="$(source /etc/os-release; echo $ID)"
        ;;
    ARCH*)
        OUTPUT="${GOARCH:-$(go env GOARCH 2> /dev/null)}"
        ;;
    BASENAME*)
        OUTPUT="podman"
        ;;
    REMOTENAME*)
        OUTPUT="$($0 BASENAME)-remote"
        ;;
    *)
        echo "Error, unknown/unsupported argument '$1', valid arguments:"
        valid_args
        exit 1
        ;;
esac

if [[ -n "$OUTPUT" ]]
then
    echo -n "$OUTPUT"
else
    echo "Error, empty output for info: '$1'" > /dev/stderr
    exit 2
fi
