#!/bin/bash

set -e

RED="\e[1;36;41m"
YEL="\e[1;33;44m"
NOR="\e[0m"

die() {
    echo -e "$2" >&2
    exit "$1"
}

SENTINEL="__unknown__"  # default set in dockerfile

[[ "$GCPJSON" != "$SENTINEL" ]] || \
    die 1 "Must specify service account JSON in \$GCPJSON"
[[ "$GCPNAME" != "$SENTINEL" ]] || \
    die 2 "Must specify service account name in \$GCPNAME"
[[ "$GCPPROJECT" != "$SENTINEL" ]] || \
    die 4 "Must specify GCP Project ID in \$GCPPROJECT"
[[ -n "$GCPPROJECT" ]] || \
    die 5 "Must specify non-empty GCP Project ID in \$GCPPROJECT"
[[ "$IMGNAMES" != "$SENTINEL" ]] || \
    die 6 "Must specify space separated list of GCE image names in \$IMGNAMES"
[[ "$BUILDID" != "$SENTINEL" ]] || \
    die 7 "Must specify the number of current build in \$BUILDID"
[[ "$REPOREF" != "$SENTINEL" ]] || \
    die 8 "Must specify a PR number or Branch name in \$REPOREF"

ARGS="--update-labels=last-used=$(date +%s)"
# optional
[[ -z "$BUILDID" ]] || ARGS="$ARGS --update-labels=build-id=$BUILDID"
[[ -z "$REPOREF" ]] || ARGS="$ARGS --update-labels=repo-ref=$REPOREF"

gcloud config set account "$GCPNAME"
gcloud config set project "$GCPPROJECT"
echo "$GCPJSON" > /tmp/gcp.json
gcloud auth activate-service-account --key-file=/tmp/gcp.json || rm /tmp/gcp.json
for image in $IMGNAMES
do
    gcloud compute images update "$image" $ARGS &
done
set +e  # Actual update failures are only warnings
wait || die 0 "${RED}WARNING:$NOR ${YEL}Failed to update labels on one or more images:$NOR '$IMGNAMES'"
