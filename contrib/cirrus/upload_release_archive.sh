#!/bin/bash

set -eo pipefail

source $(dirname $0)/lib.sh

req_env_var CI UPLDREL_IMAGE CIRRUS_BUILD_ID GOSRC RELEASE_GCPJSON RELEASE_GCPNAME RELEASE_GCPROJECT TEST_IMAGE_CACHE_DIRPATH

[[ "$CI" == "true" ]] || \
    die 56 "$0 must be run under Cirrus-CI to function"

unset PR_OR_BRANCH BUCKET
if [[ -n "$CIRRUS_PR" ]]
then
    PR_OR_BRANCH="pr$CIRRUS_PR"
    BUCKET="libpod-pr-releases"
elif [[ -n "$CIRRUS_BRANCH" ]]
then
    PR_OR_BRANCH="$CIRRUS_BRANCH"
    BUCKET="libpod-$CIRRUS_BRANCH-releases"
else
    die 1 "Expecting either \$CIRRUS_PR or \$CIRRUS_BRANCH to be non-empty."
fi

# Functional local podman required for uploading a release
cd $GOSRC
[[ -n "$(type -P podman)" ]] || \
    make install || \
    die 57 "$0 requires working podman binary on path to function"

TMPF=$(mktemp -p '' $(basename $0)_XXXX.json)
trap "rm -f $TMPF" EXIT
set +x
echo "$RELEASE_GCPJSON" > "$TMPF"
unset RELEASE_GCPJSON

install_container_image $UPLDREL_IMAGE

cd $GOSRC
for filename in $(ls -1 *.tar.gz *.zip)
do
    echo "Running podman ... $UPLDREL_IMAGE $filename"
    podman run -i --rm \
        -e "GCPNAME=$RELEASE_GCPNAME" \
        -e "GCPPROJECT=$RELEASE_GCPROJECT" \
        -e "GCPJSON_FILEPATH=$TMPF" \
        -e "REL_ARC_FILEPATH=/tmp/$filename" \
        -e "PR_OR_BRANCH=$PR_OR_BRANCH" \
        -e "BUCKET=$BUCKET" \
        --security-opt label=disable \
        -v "$TMPF:$TMPF:ro" \
        -v "$GOSRC/$filename:/tmp/$filename:ro" \
        $UPLDREL_IMAGE
done
