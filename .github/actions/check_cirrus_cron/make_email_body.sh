#!/bin/bash

set -eo pipefail

# Intended to be executed from a github action workflow step.
# Input: File listing space separated failed cron build names and IDs
# Output: $GITHUB_WORKSPACE/artifacts/email_body.txt file

source $(dirname "${BASH_SOURCE[0]}")/lib.sh

_errfmt="Expecting %s value to not be empty"
if [[ -z "$GITHUB_REPOSITORY" ]]; then
    err $(printf "$_errfmt" "\$GITHUB_REPOSITORY")
elif [[ -z "$GITHUB_WORKFLOW" ]]; then
    err $(printf "$_errfmt" "\$GITHUB_WORKFLOW")
elif [[ ! -r "$NAME_ID_FILEPATH" ]]; then
    _errfmt="Expecting %s value to be a readable file"
    err $(printf "$_errfmt" "\$NAME_ID_FILEPATH")
fi

mkdir -p artifacts
(
    echo "Detected one or more Cirrus-CI cron-triggered jobs have failed recently:"
    echo ""

    while read -r NAME BID; do
        echo "Cron build '$NAME' Failed: https://cirrus-ci.com/build/$BID"
    done < "$NAME_ID_FILEPATH"

    echo ""
    echo "# Source: ${GITHUB_WORKFLOW} workflow on ${GITHUB_REPOSITORY}."
    # Separate content from sendgrid.com automatic footer.
    echo ""
    echo ""
) > ./artifacts/email_body.txt
