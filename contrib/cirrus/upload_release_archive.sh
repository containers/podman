#!/bin/bash

set -eo pipefail

source $(dirname $0)/lib.sh

req_env_var CI UPLDREL_IMAGE CIRRUS_BUILD_ID GOSRC RELEASE_GCPJSON RELEASE_GCPNAME RELEASE_GCPROJECT

[[ "$CI" == "true" ]] || \
    die 56 "$0 must be run under Cirrus-CI to function"

# We store "releases" for each PR, mostly to validate the process is functional
unset PR_OR_BRANCH BUCKET
if [[ -n "$CIRRUS_PR" ]]
then
    PR_OR_BRANCH="pr$CIRRUS_PR"
    BUCKET="libpod-pr-releases"
elif [[ -n "$CIRRUS_BRANCH" ]]
then
    # Only release non-development tagged commit ranges
    if is_release
    then
        PR_OR_BRANCH="$CIRRUS_BRANCH"
        BUCKET="libpod-$CIRRUS_BRANCH-releases"
    else
        warn "" "Skipping release processing: Commit range|CIRRUS_TAG is development tagged."
        exit 0
    fi
else
    die 1 "Expecting either \$CIRRUS_PR or \$CIRRUS_BRANCH to be non-empty."
fi

echo "Parsing actual_release.txt contents: $(< actual_release.txt)"
cd $GOSRC
RELEASETXT=$(<actual_release.txt)  # see build_release.sh
[[ -n "$RELEASETXT" ]] || \
    die 3 "Could not obtain metadata from actual_release.txt"
RELEASE_INFO=$(echo "$RELEASETXT" | grep -m 1 'X-RELEASE-INFO:' | sed -r -e 's/X-RELEASE-INFO:\s*(.+)/\1/')
if [[ "$?" -ne "0" ]] || [[ -z "$RELEASE_INFO" ]]
then
    die 4 "Metadata is empty or invalid: '$RELEASETXT'"
fi
# Format specified in Makefile
# e.g. libpod v1.3.1-166-g60df124e fedora 29 amd64
# or   libpod-remote v1.3.1-166-g60df124e windows - amd64
FIELDS="RELEASE_BASENAME RELEASE_VERSION RELEASE_DIST RELEASE_DIST_VER RELEASE_ARCH"
read $FIELDS <<< $RELEASE_INFO
req_env_var $FIELDS

# Functional local podman required for uploading
echo "Verifying a local, functional podman, building one if necessary."
[[ -n "$(type -P podman)" ]] || \
    make install PREFIX=/usr || \
    die 57 "$0 requires working podman binary on path to function"

TMPF=$(mktemp -p '' $(basename $0)_XXXX.json)
trap "rm -f $TMPF" EXIT
set +x
echo "$RELEASE_GCPJSON" > "$TMPF"
[[ "$OS_RELEASE_ID" == "ubuntu" ]] || \
    chcon -t container_file_t "$TMPF"
unset RELEASE_GCPJSON

cd $GOSRC
for filename in $(ls -1 *.tar.gz *.zip *.msi)
do
    unset EXT
    EXT=$(echo "$filename" | sed -r -e 's/.+\.(.+$)/\1/g')
    if [[ -z "$EXT" ]] || [[ "$EXT" == "$filename" ]]
    then
        echo "Warning: Not processing $filename (invalid extension '$EXT')"
        continue
    fi
    if [[ "$EXT" =~ "gz" ]]
    then
        EXT="tar.gz"
    fi

    [[ "$OS_RELEASE_ID" == "ubuntu" ]] || \
        chcon -t container_file_t "$filename"
    # Form the generic "latest" file for this branch or pr
    TO_PREFIX="${RELEASE_BASENAME}-latest-${PR_OR_BRANCH}-${RELEASE_DIST}"
    # Form the fully-versioned filename for historical sake
    ALSO_PREFIX="${RELEASE_BASENAME}-${RELEASE_VERSION}-${PR_OR_BRANCH}-${RELEASE_DIST}"
    TO_SUFFIX="${RELEASE_ARCH}.${EXT}"
    if [[ "$RELEASE_DIST" == "windows" ]] || [[ "$RELEASE_DIST" == "darwin" ]]
    then
        TO_FILENAME="${TO_PREFIX}-${TO_SUFFIX}"
        ALSO_FILENAME="${ALSO_PREFIX}-${TO_SUFFIX}"
    else
        TO_FILENAME="${TO_PREFIX}-${RELEASE_DIST_VER}-${TO_SUFFIX}"
        ALSO_FILENAME="${ALSO_PREFIX}-${TO_SUFFIX}"
    fi

    echo "Running podman ... $UPLDREL_IMAGE for $filename -> $TO_FILENAME"
    echo "Warning: upload failures are completely ignored, avoiding any needless holdup of PRs."
    podman run -i --rm \
        -e "GCPNAME=$RELEASE_GCPNAME" \
        -e "GCPPROJECT=$RELEASE_GCPROJECT" \
        -e "GCPJSON_FILEPATH=$TMPF" \
        -e "FROM_FILEPATH=/tmp/$filename" \
        -e "TO_FILENAME=$TO_FILENAME" \
        -e "ALSO_FILENAME=$ALSO_FILENAME" \
        -e "PR_OR_BRANCH=$PR_OR_BRANCH" \
        -e "BUCKET=$BUCKET" \
        -v "$TMPF:$TMPF:ro" \
        -v "$(realpath $GOSRC/$filename):/tmp/$filename:ro" \
        $UPLDREL_IMAGE || true
done
