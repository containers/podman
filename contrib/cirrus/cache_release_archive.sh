#!/bin/bash

set -eo pipefail

source $(dirname $0)/lib.sh

req_env_var GOSRC

RELEASE_ARCHIVE_NAMES=""

handle_archive() {  # Assumed to be called with set +e
        TASK_NUMBER=$1
        PR_OR_BRANCH=$2
        CACHE_URL=$3
        ARCHIVE_NAME="$(basename $CACHE_URL)"
        req_env_var TASK_NUMBER PR_OR_BRANCH CACHE_URL ARCHIVE_NAME

        cd /tmp
        curl -sO "$CACHE_URL" || return $(warn 0 "Couldn't download file, skipping.")
        [[ -r "/tmp/$ARCHIVE_NAME" ]] || return $(warn 0 "Unreadable archive '/tmp/$ARCHIVE_NAME', skipping.")

        ZIPCOMMENT=$(unzip -qqz "$ARCHIVE_NAME" 2>/dev/null)  # noisy bugger
        if [[ "$?" -ne "0" ]] || [[ -z "$ZIPCOMMENT" ]]
        then
            return $(warn 0 "Could not unzip metadata from downloaded '/tmp/$ARCHIVE_NAME', skipping.")
        fi

        RELEASE_INFO=$(echo "$ZIPCOMMENT" | grep -m 1 'X-RELEASE-INFO:' | sed -r -e 's/X-RELEASE-INFO:\s*(.+)/\1/')
        if [[ "$?" -ne "0" ]] || [[ -z "$RELEASE_INFO" ]]
        then
            return $(warn 0 "Metadata empty or invalid: '$ZIPCOMMENT', skipping.")
        fi

        # e.g. libpod v1.3.1-166-g60df124e fedora 29 amd64
        # or   libpod v1.3.1-166-g60df124e   amd64
        FIELDS="RELEASE_BASENAME RELEASE_VERSION RELEASE_DIST RELEASE_DIST_VER RELEASE_ARCH"
        read $FIELDS <<< $RELEASE_INFO
        for f in $FIELDS
        do
            [[ -n "${!f}" ]] || return $(warn 0 "Expecting $f to be non-empty in metadata: '$RELEASE_INFO', skipping.")
        done

        echo -n "Preparing $RELEASE_BASENAME archive: "
        # Drop version number to enable "latest" representation
        # (version available w/in zip-file comment)
        RELEASE_ARCHIVE_NAME="${RELEASE_BASENAME}-${PR_OR_BRANCH}-${RELEASE_DIST}-${RELEASE_DIST_VER}-${RELEASE_ARCH}.zip"
        # Allow uploading all gathered files in parallel, later with gsutil.
        mv -v "$ARCHIVE_NAME" "/$RELEASE_ARCHIVE_NAME"
        RELEASE_ARCHIVE_NAMES="$RELEASE_ARCHIVE_NAMES $RELEASE_ARCHIVE_NAME"
}

make_release() {
    ARCHIVE_NAME="$1"
    req_env_var ARCHIVE_NAME

    # There's no actual testing of windows/darwin targets yet
    # but we still want to cross-compile and publish binaries
    if [[ "$SPECIALMODE" == "windows" ]] || [[ "$SPECIALMODE" == "darwin" ]]
    then
        RELFILE="podman-remote-${SPECIALMODE}.zip"
    elif [[ "$SPECIALMODE" == "none" ]]
    then
        RELFILE="podman.zip"
    else
        die 55 "$(basename $0) unable to handle \$SPECIALMODE=$SPECIALMODE for $ARCHIVE_NAME"
    fi
    echo "Calling make $RELFILE"
    cd $GOSRC
    make "$RELFILE"
    echo "Renaming archive so it can be identified/downloaded for publishing"
    mv -v "$RELFILE" "$ARCHIVE_NAME"
    echo "Success!"
}

[[ "$CI" == "true" ]] || \
    die 56 "$0 requires a Cirrus-CI cross-task cache to function"

cd $GOSRC
# Same script re-used for both uploading and downloading to avoid duplication
if [[ "$(basename $0)" == "cache_release_archive.sh" ]]
then
    # ref: https://cirrus-ci.org/guide/writing-tasks/#environment-variables
    req_env_var CI_NODE_INDEX CIRRUS_BUILD_ID
    # Use unique names for uncache_release_archives.sh to find/download them all
    ARCHIVE_NAME="build-${CIRRUS_BUILD_ID}-task-${CI_NODE_INDEX}.zip"
    make_release "$ARCHIVE_NAME"

    # ref: https://cirrus-ci.org/guide/writing-tasks/#http-cache
    URL="http://$CIRRUS_HTTP_CACHE_HOST/${ARCHIVE_NAME}"
    echo "Uploading $ARCHIVE_NAME to Cirrus-CI cache at $URL"
    curl -s -X POST --data-binary "@$ARCHIVE_NAME" "$URL"
elif [[ "$(basename $0)" == "uncache_release_archives.sh" ]]
then
    req_env_var CIRRUS_BUILD_ID CI_NODE_TOTAL GCPJSON GCPNAME GCPROJECT
    [[ "${CI_NODE_INDEX}" -eq  "$[CI_NODE_TOTAL-1]" ]] || \
        die 8 "The release task must be executed last to guarantee archive cache is complete"

    if [[ -n "$CIRRUS_PR" ]]
    then
        PR_OR_BRANCH="pr$CIRRUS_PR"
        BUCKET="libpod-pr-releases"
    elif [[ -n "$CIRRUS_BRANCH" ]]
    then
        PR_OR_BRANCH="$CIRRUS_BRANCH"
        BUCKET="libpod-$CIRRUS_BRANCH-releases"
    else
        die 10 "Expecting either \$CIRRUS_PR or \$CIRRUS_BRANCH to be non-empty."
    fi

    echo "Blindly downloading Cirrus-CI cache files for task (some will fail)."
    set +e  # Don't stop looping until all task's cache is attempted
    for (( task_number = 0 ; task_number < $CI_NODE_TOTAL ; task_number++ ))
    do
        ARCHIVE_NAME="build-${CIRRUS_BUILD_ID}-task-${task_number}.zip"
        URL="http://$CIRRUS_HTTP_CACHE_HOST/${ARCHIVE_NAME}"
        echo "Attempting to download cached archive from $URL"
        handle_archive "$task_number" "$PR_OR_BRANCH" "$URL"
        echo "----------------------------------------"
    done
    set -e

    [[ -n "$RELEASE_ARCHIVE_NAMES" ]] || \
        die 67 "Error: No release archives found in CI cache, expecting at least one."

    echo "Preparing to upload release archives."
    gcloud config set project "$GCPROJECT"
    echo "$GCPJSON" > /tmp/gcp.json
    gcloud auth activate-service-account --key-file=/tmp/gcp.json
    rm /tmp/gcp.json
    # handle_archive() placed all uploadable files under /
    gsutil -m cp /*.zip "gs://$BUCKET"  # Upload in parallel
    echo "Successfully uploaded archives:"
    for ARCHIVE_NAME in $RELEASE_ARCHIVE_NAMES
    do
        echo "    https://storage.cloud.google.com/$BUCKET/$ARCHIVE_NAME"
    done
    echo "These will remain available until automatic pruning by bucket policy."
else
    die 9 "I don't know what to do when called $0"
fi
