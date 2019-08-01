#!/bin/bash

set -e

RED="\e[1;36;41m"
YEL="\e[1;33;44m"
NOR="\e[0m"
SENTINEL="__unknown__"  # default set in dockerfile
# Disable all input prompts
# https://cloud.google.com/sdk/docs/scripting-gcloud
GCLOUD="gcloud --quiet"

die() {
    EXIT=$1
    PFX=$2
    shift 2
    MSG="$@"
    echo -e "${RED}${PFX}:${NOR} ${YEL}$MSG${NOR}"
    [[ "$EXIT" -eq "0" ]] || exit "$EXIT"
}

# Pass in a list of one or more envariable names; exit non-zero with
# helpful error message if any value is empty
req_env_var() {
    for i; do
        if [[ -z "${!i}" ]]
        then
            die 1 FATAL entrypoint.sh requires \$$i to be non-empty.
        elif [[ "${!i}" == "$SENTINEL" ]]
        then
            die 2 FATAL entrypoint.sh requires \$$i to be explicitly set.
        fi
    done
}

gcloud_init() {
    set +xe
    if [[ -n "$1" ]] && [[ -r "$1" ]]
    then
        TMPF="$1"
    else
        TMPF=$(mktemp -p '' .$(uuidgen)_XXXX.json)
        trap "rm -f $TMPF &> /dev/null" EXIT
        echo "$GCPJSON" > $TMPF
    fi
    $GCLOUD auth activate-service-account --project="$GCPPROJECT" --key-file="$TMPF" || \
        die 5 FATAL auth
    rm -f $TMPF &> /dev/null || true  # ignore any read-only error
}
