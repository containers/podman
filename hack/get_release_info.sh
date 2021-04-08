#!/usr/bin/env bash

# This script produces various bits of metadata needed by Makefile.  Using
# a script allows uniform behavior across multiple environments and
# distributions.  The script expects a single argument, as reflected below.

set -euo pipefail

cd "${GOSRC:-$(dirname $0)/../}"

valid_args() {
    REGEX='^\s+[[:upper:]]+\*[)]'
    egrep --text --no-filename --group-separator=' ' --only-matching "$REGEX" "$0" | \
        cut -d '*' -f 1
}

# `git describe` will never produce a useful version number under all
# branches.  This is because the podman release process (see `RELEASE_PROCESS.md`)
# tags release versions only on release-branches (i.e. never on master).
# Scraping the version number directly from the source, is the only way
# to reliably obtain the number from all the various contexts supported by
# the `Makefile`.
scrape_version() {
    local v
    # extract the value of 'var Version'
    v=$(sed -ne 's/^var\s\+Version\s\+=\s.*("\(.*\)").*/\1/p' <version/version.go)
    # If it's empty, something has changed in version.go, that would be bad!
    test -n "$v"
    # Value consumed literally, must not have any embedded newlines
    echo -n "$v"
}

unset OUTPUT
case "$1" in
    # Wild-card suffix needed by valid_args() e.g. possible bad grep of "$(echo $FOO)"
    VERSION*)
        OUTPUT="${CIRRUS_TAG:-$(scrape_version)}"
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
